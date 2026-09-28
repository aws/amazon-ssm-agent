// Copyright 2021 Amazon.com, Inc. or its affiliates. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License"). You may not
// use this file except in compliance with the License. A copy of the
// License is located at
//
// http://aws.amazon.com/apache2.0/
//
// or in the "license" file accompanying this file. This file is distributed
// on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND,
// either express or implied. See the License for the specific language governing
// permissions and limitations under the License.

package sharedprovider

import (
	"context"
	"fmt"
	"math/rand"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"

	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/amazon-ssm-agent/common/runtimeconfig"
)

const (
	// refreshMinutesBeforeExpiryMinutes defines the amount of time the client will wait until it tries to read credentials from disk
	refreshBeforeExpiryDuration = 10 * time.Minute
	providerName                = "SharedCredentialsProvider"
)

var (
	emptyCredential      = aws.Credentials{Source: providerName}
	newSharedCredentials = NewSharedCredentials
	newRuntimeConfig     = runtimeconfig.NewIdentityRuntimeConfigClient
)

type ISharedCredentialsProvider interface {
	aws.CredentialsProvider
	RetrieveWithContext(ctx context.Context) (aws.Credentials, error)
	ExpiresAt() time.Time
	IsExpired() bool
}

// sharedCredentialsProvider implements the AWS SDK credential provider, and is used to create AWS client.
// It retrieves credentials from the shared credentials on disk, and keeps track if those credentials are expired.
type sharedCredentialsProvider struct {
	aws.Credentials

	log                   log.T
	runtimeConfigClient   runtimeconfig.IIdentityRuntimeConfigClient
	identityRuntimeConfig runtimeconfig.IdentityRuntimeConfig
	getTimeNow            func() time.Time
}

func NewSharedCredentials(sharedCredsFile string, sharedProfile string) (aws.CredentialsProvider, error) {
	// Normalize an empty profile to "default" to match the write side.
	// agent/managedInstances/sharedCredentials.Store maps "" -> "default"
	// before creating the [default] section in the shared credentials file,
	// but LoadSharedConfigProfile treats "" as a literal section name and
	// would return SharedConfigProfileNotExistError even when [default]
	// exists.
	if sharedProfile == "" {
		sharedProfile = "default"
	}

	// Parse the shared credentials file directly, without going through
	// LoadDefaultConfig. LoadDefaultConfig — even when constrained via
	// WithSharedCredentialsFiles — still runs the full credential chain
	// (environment variables in front of the shared file, EC2 IMDS behind
	// it), which would let a caller-supplied AWS_ACCESS_KEY_ID override the
	// file or a missing file resolve to IMDS-role credentials.
	// LoadSharedConfigProfile parses only the credentials/config files with
	// no chain fallback, so it fails cleanly when the profile is absent and
	// isn't affected by env vars or IMDS.
	sc, err := awsconfig.LoadSharedConfigProfile(context.TODO(), sharedProfile, func(o *awsconfig.LoadSharedConfigOptions) {
		o.CredentialsFiles = []string{sharedCredsFile}
		// Also disable ~/.aws/config processing to prevent
		// credential_process/SSO/AssumeRole from executing under the agent's
		// privilege (root/SYSTEM).
		o.ConfigFiles = []string{}
	})
	if err != nil {
		return nil, fmt.Errorf("load shared credentials for profile %q from %q: %w", sharedProfile, sharedCredsFile, err)
	}
	if sc.Credentials.AccessKeyID == "" || sc.Credentials.SecretAccessKey == "" {
		return nil, fmt.Errorf("no credentials found in %q for profile %q", sharedCredsFile, sharedProfile)
	}
	return credentials.StaticCredentialsProvider{Value: sc.Credentials}, nil
}

// NewCredentialsProvider initializes a shared provider that loads credentials that were saved disk
func NewCredentialsProvider(log log.T) ISharedCredentialsProvider {
	log = log.WithContext("[SharedCredentialsProvider]")

	return &sharedCredentialsProvider{
		log:        log,
		getTimeNow: time.Now,
	}
}

// Retrieve retrieves credentials from the shared profile
// Error will be returned if the request fails, or unable to extract
// the desired credentials.
func (s *sharedCredentialsProvider) Retrieve(ctx context.Context) (aws.Credentials, error) {
	return s.RetrieveWithContext(ctx)
}

// RetrieveWithContext retrieves credentials from the shared credentials file
// Error will be returned if the request fails, or unable to extract
// the desired credentials.
func (s *sharedCredentialsProvider) RetrieveWithContext(ctx context.Context) (aws.Credentials, error) {
	runtimeConfigClient := newRuntimeConfig()
	// before sharedCredentialsProvider is initialized, we check if the runtime config exists
	config, err := runtimeConfigClient.GetConfig()
	if err != nil {
		return aws.Credentials{Source: providerName}, fmt.Errorf("unable to read runtime config for ShareFile information. Err: %w", err)
	}

	if config.ShareFile == "" {
		return aws.Credentials{}, fmt.Errorf("runtime config has an empty ShareFile")
	}

	// If credentials are already expired, return error
	if config.CredentialsExpiresAt.Before(s.getTimeNow()) {
		return aws.Credentials{Source: providerName}, fmt.Errorf("shared credentials are already expired, they were retrieved at %v and expired at %v", config.CredentialsRetrievedAt.Format(time.RFC3339), config.CredentialsExpiresAt.Format(time.RFC3339))
	}

	credsProvider, err := newSharedCredentials(config.ShareFile, config.ShareProfile)
	if err != nil {
		return aws.Credentials{Source: providerName}, fmt.Errorf("failed to create shared credentials provider. Err: %w", err)
	}

	s.Credentials, err = credsProvider.Retrieve(ctx)
	if err != nil {
		return aws.Credentials{Source: providerName}, err
	}

	s.Credentials.Source = providerName

	expirationWindow := s.getTimeWindow()
	// If credentials currently saved credentials expire in less than 'refreshBeforeExpiryDuration', no expiry window should be set
	if config.CredentialsExpiresAt.Before(s.getTimeNow().Add(refreshBeforeExpiryDuration)) {
		expirationWindow = time.Duration(0)
	}

	s.Credentials.Expires = config.CredentialsExpiresAt.Round(0).Add(-expirationWindow)
	s.Credentials.CanExpire = true

	return s.Credentials, err
}

func (s *sharedCredentialsProvider) getTimeWindow() time.Duration {
	// Random jitter of up to 1 minute in case multiple workers are running, we want to spread read of the runtime config and credentials file
	randomJitterDuration := time.Second * time.Duration(rand.Intn(60))
	return refreshBeforeExpiryDuration + randomJitterDuration
}

// IsExpired wraps the IsExpired method of the current provider
func (s *sharedCredentialsProvider) IsExpired() bool {

	return s.Credentials.Expired()
}

// ExpiresAt returns the expiry of shared credentials using shared credentials
// and returns instance profile role provider expiry otherwise
func (s *sharedCredentialsProvider) ExpiresAt() time.Time {

	return s.Credentials.Expires
}
