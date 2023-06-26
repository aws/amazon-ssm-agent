// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License").
// You may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package ec2roleprovider

import (
	"context"
	"fmt"
	"runtime"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/credentials/ec2rolecreds"
	"github.com/aws/aws-sdk-go/service/ssm"

	"github.com/aws/amazon-ssm-agent/agent/appconfig"
	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/amazon-ssm-agent/agent/sdkutil"
	"github.com/aws/amazon-ssm-agent/agent/version"
	"github.com/aws/amazon-ssm-agent/common/identity/credentialproviders/ssmec2roleprovider"
	"github.com/aws/amazon-ssm-agent/common/runtimeconfig"
)

// EC2RoleProvider provides credentials for the agent when on an EC2 instance
type EC2RoleProvider struct {
	credentials.Expiry
	InnerProviders         *EC2InnerProviders
	Log                    log.T
	Config                 *appconfig.SsmagentConfig
	InstanceInfo           *ssmec2roleprovider.InstanceInfo
	expirationUpdateLock   *sync.Mutex
	credentialSource       string
	SsmEndpoint            string
	shouldShareCredentials bool
	RuntimeConfigClient    runtimeconfig.IIdentityRuntimeConfigClient
}

// NewEC2RoleProvider initializes a new EC2RoleProvider using runtime config values
func NewEC2RoleProvider(log log.T, config *appconfig.SsmagentConfig, innerProviders *EC2InnerProviders, instanceInfo *ssmec2roleprovider.InstanceInfo, ssmEndpoint string, runtimeConfigClient runtimeconfig.IIdentityRuntimeConfigClient) *EC2RoleProvider {
	runtimeConfig, err := runtimeConfigClient.GetConfigWithRetry()
	if err != nil {
		log.Warnf("Failed to get credential source from runtime config. Err: %v", err)
	}

	var credentialSource string
	if runtimeConfig.CredentialSource == CredentialSourceEC2 && runtimeConfig.IdentityType == IdentityTypeEC2 {
		credentialSource = CredentialSourceEC2
	} else if runtimeConfig.CredentialSource == CredentialSourceSSM && runtimeConfig.IdentityType == IdentityTypeEC2 {
		credentialSource = CredentialSourceSSM
	} else {
		credentialSource = CredentialSourceNone
	}

	return &EC2RoleProvider{
		InnerProviders:         innerProviders,
		Log:                    log.WithContext(ec2rolecreds.ProviderName),
		Config:                 config,
		InstanceInfo:           instanceInfo,
		SsmEndpoint:            ssmEndpoint,
		RuntimeConfigClient:    runtimeConfigClient,
		credentialSource:       credentialSource,
		shouldShareCredentials: true,
		expirationUpdateLock:   &sync.Mutex{},
	}
}

// GetInnerProvider gets the remote role provider that is currently being used for credentials
func (p *EC2RoleProvider) GetInnerProvider() IInnerProvider {
	if p.credentialSource == CredentialSourceSSM {
		return p.InnerProviders.SsmEc2Provider
	}

	return p.InnerProviders.IPRProvider
}

// RetrieveWithContext returns shared credentials if specified in runtime config
// and returns instance profile role credentials otherwise.
// If neither can be retrieved then empty credentials are returned
// This function is intended for use by agent workers that require credentials
func (p *EC2RoleProvider) RetrieveWithContext(ctx context.Context) (credentials.Value, error) {
	if runtimeConfig, err := p.RuntimeConfigClient.GetConfigWithRetry(); err != nil {
		p.Log.Errorf("Failed to read runtime config for ShareFile information. Err: %v", err)
	} else if runtimeConfig.ShareFile != "" {
		sharedCreds, err := p.InnerProviders.SharedCredentialsProvider.RetrieveWithContext(ctx)
		if err != nil {
			err = fmt.Errorf("unable to load shared credentials. Err: %w", err)
			p.Log.Error(err)
			return iprEmptyCredential, err
		}

		p.credentialSource = CredentialSourceSSM
		return sharedCreds, nil
	}

	iprCredentials, err := p.InnerProviders.IPRProvider.RetrieveWithContext(ctx)
	if err != nil {
		err = fmt.Errorf("failed to retrieve instance profile role credentials. Err: %w", err)
		p.Log.Error(err)
		return iprEmptyCredential, err
	}
	p.credentialSource = CredentialSourceEC2

	return iprCredentials, nil
}

// RemoteRetrieve uses network calls to retrieve credentials for EC2 instances
// This function is intended for use by the core module's credential refresher routine
// When an error is returned, credential source is updated to CredentialSourceNone
func (p *EC2RoleProvider) RemoteRetrieve(ctx context.Context) (credentials.Value, error) {
	p.Log.Debug("Attempting to retrieve instance profile role")
	if iprCredentials, err := p.iprCredentials(ctx, p.SsmEndpoint); err != nil {
		errCode := sdkutil.GetAwsErrorCode(err)
		if _, ok := exceptionsForDefaultHostMgmt[errCode]; ok {
			p.Log.Warnf("Failed to connect to Systems Manager with instance profile role credentials. Err: %v", err)
		} else {
			p.credentialSource = CredentialSourceNone
			return iprEmptyCredential, fmt.Errorf("unexpected error getting instance profile role credentials or calling UpdateInstanceInformation. Skipping default host management fallback: %w", err)
		}
	} else {
		p.Log.Info("Successfully connected with instance profile role credentials")
		p.credentialSource = CredentialSourceEC2
		return iprCredentials.Get()
	}

	p.Log.Debug("Attempting to retrieve credentials from Systems Manager")
	if ssmCredentials, err := p.InnerProviders.SsmEc2Provider.RetrieveWithContext(ctx); err != nil {
		p.Log.Errorf("Failed to connect to Systems Manager with SSM role credentials. %v", err)
	} else {
		p.Log.Info("Successfully connected with Systems Manager role credentials")
		p.credentialSource = CredentialSourceSSM
		return ssmCredentials, nil
	}

	p.credentialSource = CredentialSourceNone
	return iprEmptyCredential, fmt.Errorf("no valid credentials could be retrieved for ec2 identity")
}

// Retrieve returns instance profile role credentials if it has sufficient systems manager permissions and
// returns ssm provided credentials otherwise. If neither can be retrieved then empty credentials are returned
// This function is intended for use by agent workers that require credentials
func (p *EC2RoleProvider) Retrieve() (credentials.Value, error) {
	return p.RetrieveWithContext(context.Background())
}

// iprCredentials retrieves instance profile role credentials and returns an error if the returned credentials cannot
// connect to Systems Manager
func (p *EC2RoleProvider) iprCredentials(ctx context.Context, ssmEndpoint string) (*credentials.Credentials, error) {
	// Setup SSM client with instance profile role credentials
	iprCredentials := newCredentials(p.InnerProviders.IPRProvider)
	err := p.updateEmptyInstanceInformation(ctx, ssmEndpoint, iprCredentials)
	if err != nil {
		if awsErr, ok := err.(awserr.RequestFailure); ok {
			err = fmt.Errorf("retrieved credentials failed to report to ssm. RequestId: %s Error: %w", awsErr.RequestID(), awsErr)
		} else {
			err = fmt.Errorf("retrieved credentials failed to report to ssm. Error: %w", err)
		}

		return nil, err
	}

	// Trusting instance profile role credentials are valid for at least 1 hour when retrieved
	p.InnerProviders.IPRProvider.SetExpiration(timeNowFunc().Add(1*time.Hour), 0)

	return iprCredentials, nil
}

// updateEmptyInstanceInformation calls UpdateInstanceInformation with minimal parameters
func (p *EC2RoleProvider) updateEmptyInstanceInformation(ctx context.Context, ssmEndpoint string, roleCredentials *credentials.Credentials) error {
	ssmClient := newV4ServiceWithCreds(p.Log.WithContext("SSMService"), p.Config, roleCredentials, p.InstanceInfo.Region, ssmEndpoint)

	p.Log.Debugf("Calling UpdateInstanceInformation with agent version %s", p.Config.Agent.Version)
	// Call update instance information with instance profile role
	input := &ssm.UpdateInstanceInformationInput{
		AgentName:    aws.String(agentName),
		AgentVersion: aws.String(version.Version),
		InstanceId:   aws.String(p.InstanceInfo.InstanceId),
	}

	goOS := runtime.GOOS
	switch goOS {
	case "windows":
		input.PlatformType = aws.String(ssm.PlatformTypeWindows)
	case "linux", "freebsd":
		input.PlatformType = aws.String(ssm.PlatformTypeLinux)
	case "darwin":
		input.PlatformType = aws.String(ssm.PlatformTypeMacOs)
	}

	_, err := ssmClient.UpdateInstanceInformationWithContext(ctx, input)
	return err
}

// ShareFile is the credentials file where the agent should write shared credentials
// Only default host management role credentials are shared across workers
func (p *EC2RoleProvider) ShareFile() string {
	switch p.credentialSource {
	case CredentialSourceSSM:
		return appconfig.DefaultEC2SharedCredentialsFilePath
	default:
		return ""
	}
}

// ShareProfile is the profile where the agent should write shared credentials
func (p *EC2RoleProvider) ShareProfile() string {
	return ""
}

// SharesCredentials returns true if credentials may be saved to disk
func (p *EC2RoleProvider) SharesCredentials() bool {
	return true
}

// IsExpired wraps the IsExpired method of the current provider
func (p *EC2RoleProvider) IsExpired() bool {
	if p.credentialSource == CredentialSourceSSM {
		return p.InnerProviders.SharedCredentialsProvider.IsExpired()
	}

	return p.InnerProviders.IPRProvider.IsExpired()
}

// ExpiresAt returns the expiry of shared credentials using shared credentials
// and returns instance profile role provider expiry otherwise
func (p *EC2RoleProvider) ExpiresAt() time.Time {
	if p.credentialSource == CredentialSourceSSM {
		return p.InnerProviders.SharedCredentialsProvider.ExpiresAt()
	}

	return p.InnerProviders.IPRProvider.ExpiresAt()
}

// RemoteExpiresAt returns the expiry of the remote inner provider currently in use
// This function is intended for use by the core module's credential refresher routine
func (p *EC2RoleProvider) RemoteExpiresAt() time.Time {
	return p.GetInnerProvider().ExpiresAt()
}

// CredentialSource returns the name of the current provider being used
func (p *EC2RoleProvider) CredentialSource() string {
	return p.credentialSource
}
