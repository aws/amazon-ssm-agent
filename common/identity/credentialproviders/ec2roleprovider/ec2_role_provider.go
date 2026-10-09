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
	"errors"
	"fmt"
	"runtime"
	"sync"
	"time"

	awshttp "github.com/aws/aws-sdk-go-v2/aws/transport/http"

	"github.com/aws/amazon-ssm-agent/agent/appconfig"
	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/amazon-ssm-agent/agent/sdkutil"
	awserr "github.com/aws/amazon-ssm-agent/agent/sdkutil"
	"github.com/aws/amazon-ssm-agent/agent/version"
	"github.com/aws/amazon-ssm-agent/common/identity/credentialproviders/ssmec2roleprovider"
	"github.com/aws/amazon-ssm-agent/common/runtimeconfig"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials/ec2rolecreds"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
	ssmtypes "github.com/aws/aws-sdk-go-v2/service/ssm/types"
)

// EC2RoleProvider provides credentials for the agent when on an EC2 instance
type EC2RoleProvider struct {
	aws.Credentials
	InnerProviders         *EC2InnerProviders
	Log                    log.T
	InstanceInfo           *ssmec2roleprovider.InstanceInfo
	mu                     sync.Mutex // guards Credentials and credentialSource
	credentialSource       string
	SsmEndpoint            string
	shouldShareCredentials bool
	RuntimeConfigClient    runtimeconfig.IIdentityRuntimeConfigClient
}

// NewEC2RoleProvider initializes a new EC2RoleProvider using runtime config values
func NewEC2RoleProvider(log log.T, innerProviders *EC2InnerProviders, instanceInfo *ssmec2roleprovider.InstanceInfo, ssmEndpoint string, runtimeConfigClient runtimeconfig.IIdentityRuntimeConfigClient) *EC2RoleProvider {
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
		InstanceInfo:           instanceInfo,
		SsmEndpoint:            ssmEndpoint,
		RuntimeConfigClient:    runtimeConfigClient,
		credentialSource:       credentialSource,
		shouldShareCredentials: true,
	}
}

// GetInnerProvider gets the remote role provider that is currently being used for credentials
func (p *EC2RoleProvider) GetInnerProvider() IInnerProvider {
	if p.credentialSource == CredentialSourceSSM {
		return p.InnerProviders.SsmEc2Provider
	}

	return p.InnerProviders.IPRProvider
}

// GetInstanceRegion gets the region of the instance for this provider.
func (p *EC2RoleProvider) GetInstanceRegion() string {
	if p.InstanceInfo == nil {
		return "" // Should never happen with proper initialization
	}
	return p.InstanceInfo.Region
}

// RetrieveWithContext returns shared credentials if specified in runtime config
// and returns instance profile role credentials otherwise.
// If neither can be retrieved then empty credentials are returned
// This function is intended for use by agent workers that require credentials
func (p *EC2RoleProvider) RetrieveWithContext(ctx context.Context) (aws.Credentials, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if runtimeConfig, err := p.RuntimeConfigClient.GetConfigWithRetry(); err != nil {
		p.Log.Errorf("Failed to read runtime config for ShareFile information. Err: %v", err)
	} else if runtimeConfig.ShareFile != "" {
		sharedCreds, err := p.InnerProviders.SharedCredentialsProvider.Retrieve(ctx)
		if err != nil {
			err = fmt.Errorf("unable to load shared credentials. Err: %w", err)
			p.Log.Error(err)
			return iprEmptyCredential, err
		}

		p.credentialSource = CredentialSourceSSM
		return sharedCreds, nil
	}

	iprCredentials, err := p.InnerProviders.IPRProvider.Retrieve(ctx)
	if err != nil {
		if awsErr := sdkutil.GetAwsError(err); awsErr != nil {

			err = awserr.New(awsErr.Code(), awsErr.Message(), nil)
		}
		err = fmt.Errorf("failed to retrieve instance profile role credentials. Err: %w", err)
		p.Log.Error(err)
		return iprEmptyCredential, err
	}

	p.Credentials = iprCredentials
	p.credentialSource = CredentialSourceEC2

	// Propagate expiry to the inner provider so IsExpired() is consistent
	if expirySetter, ok := p.InnerProviders.IPRProvider.(interface{ SetExpiry(time.Time) }); ok {
		expirySetter.SetExpiry(iprCredentials.Expires)
	}

	return iprCredentials, nil
}

// RemoteRetrieve uses network calls to retrieve credentials for EC2 instances
// This function is intended for use by the core module's credential refresher routine
// When an error is returned, credential source is updated to CredentialSourceNone
func (p *EC2RoleProvider) RemoteRetrieve(ctx context.Context) (aws.Credentials, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
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
		iprCredentials.Expires = timeNowFunc().Add(1 * time.Hour)
		return iprCredentials, nil
	}

	p.Log.Debug("Attempting to retrieve credentials from Systems Manager")
	if ssmCredentials, err := p.InnerProviders.SsmEc2Provider.Retrieve(ctx); err != nil {
		p.Log.Errorf("Failed to connect to Systems Manager with SSM role credentials. %v", err)
		p.credentialSource = CredentialSourceNone
		return iprEmptyCredential, fmt.Errorf("no valid credentials could be retrieved for ec2 identity. Default Host Management Err: %w", err)
	} else {
		p.Log.Info("Successfully connected with Systems Manager role credentials")
		p.credentialSource = CredentialSourceSSM
		return ssmCredentials, nil
	}
}

// Retrieve returns instance profile role credentials if it has sufficient systems manager permissions and
// returns ssm provided credentials otherwise. If neither can be retrieved then empty credentials are returned
// This function is intended for use by agent workers that require credentials
func (p *EC2RoleProvider) Retrieve(ctx context.Context) (aws.Credentials, error) {
	return p.RetrieveWithContext(ctx)
}

// iprCredentials retrieves instance profile role credentials and returns an error if the returned credentials cannot
// connect to Systems Manager
func (p *EC2RoleProvider) iprCredentials(ctx context.Context, ssmEndpoint string) (aws.Credentials, error) {
	// Setup SSM client with instance profile role credentials

	iprCredentials, _ := p.InnerProviders.IPRProvider.Retrieve(ctx)
	err := p.updateEmptyInstanceInformation(ctx, ssmEndpoint, p.InnerProviders.IPRProvider)
	if err != nil {
		var awsRequestFailure *awshttp.ResponseError
		if isRequestFailure := errors.As(err, &awsRequestFailure); isRequestFailure {
			err = fmt.Errorf("retrieved credentials failed to report to ssm. RequestId: %s Error: %w", awsRequestFailure.ServiceRequestID(), awsRequestFailure)
		} else {
			err = fmt.Errorf("retrieved credentials failed to report to ssm. Error: %w", err)
		}

		return iprEmptyCredential, err
	}

	// Trusting instance profile role credentials are valid for at least 1 hour when retrieved
	iprCredentials.Expires = timeNowFunc().Add(1 * time.Hour)

	return iprCredentials, nil
}

// updateEmptyInstanceInformation calls UpdateInstanceInformation with minimal parameters
func (p *EC2RoleProvider) updateEmptyInstanceInformation(ctx context.Context, ssmEndpoint string, roleCredentialsProvider aws.CredentialsProvider) error {
	ssmClient := newV4ServiceWithCreds(p.Log.WithContext("SSMService"), roleCredentialsProvider, p.GetInstanceRegion(), ssmEndpoint)

	p.Log.Debugf("Calling UpdateInstanceInformation")
	// Call update instance information with instance profile role
	input := &ssm.UpdateInstanceInformationInput{
		AgentName:    aws.String(agentName),
		AgentVersion: aws.String(version.Version),
		InstanceId:   aws.String(p.InstanceInfo.InstanceId),
	}

	goOS := runtime.GOOS
	switch goOS {
	case "windows":
		input.PlatformType = aws.String(string(ssmtypes.PlatformTypeWindows))
	case "linux", "freebsd":
		input.PlatformType = aws.String(string(ssmtypes.PlatformTypeLinux))
	case "darwin":
		input.PlatformType = aws.String(string(ssmtypes.PlatformTypeMacos))
	}

	_, err := ssmClient.UpdateInstanceInformation(ctx, input)
	if err != nil {
		if awsErr := sdkutil.GetAwsError(err); awsErr != nil {
			err = awserr.New(awsErr.Code(), awsErr.Message(), awsErr.OrigErr())
		}
	}
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

	return p.Credentials.Expires
}

// RemoteExpiresAt returns the expiry of the remote inner provider currently in use
// This function is intended for use by the core module's credential refresher routine
func (p *EC2RoleProvider) RemoteExpiresAt() time.Time {
	return p.GetInnerProvider().ExpiresAt()
}

// CredentialSource returns the name of the current provider being used
func (p *EC2RoleProvider) CredentialSource() string {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.credentialSource
}
