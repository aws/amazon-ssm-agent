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
	"fmt"
	"runtime"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go/aws/credentials/ec2rolecreds"

	"github.com/aws/amazon-ssm-agent/common/runtimeconfig"

	"github.com/aws/amazon-ssm-agent/agent/appconfig"
	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/amazon-ssm-agent/agent/sdkutil"
	"github.com/aws/amazon-ssm-agent/agent/version"
	"github.com/aws/amazon-ssm-agent/common/identity/credentialproviders/ssmec2roleprovider"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/service/ssm"
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

func NewEC2RoleProvider(log log.T, config *appconfig.SsmagentConfig, innerProviders *EC2InnerProviders, instanceInfo *ssmec2roleprovider.InstanceInfo, ssmEndpoint string, runtimeConfigClient runtimeconfig.IIdentityRuntimeConfigClient) *EC2RoleProvider {
	return &EC2RoleProvider{
		InnerProviders:         innerProviders,
		Log:                    log.WithContext(ec2rolecreds.ProviderName),
		Config:                 config,
		InstanceInfo:           instanceInfo,
		SsmEndpoint:            ssmEndpoint,
		RuntimeConfigClient:    runtimeConfigClient,
		credentialSource:       CredentialSourceEC2,
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

// Retrieve returns shared credentials if specified in runtime config
// and returns instance profile role credentials otherwise.
// If neither can be retrieved then empty credentials are returned
func (p *EC2RoleProvider) Retrieve() (credentials.Value, error) {
	if runtimeConfig, err := p.RuntimeConfigClient.GetConfig(); err != nil {
		p.Log.Errorf("Failed to read runtime config for ShareFile information")
	} else if runtimeConfig.ShareFile != "" {
		sharedCreds, err := p.InnerProviders.SharedCredentialsProvider.Retrieve()
		if err != nil {
			err = fmt.Errorf("unable to load shared credentials. Err: %w", err)
			p.Log.Error(err)
			return iprEmptyCredential, err
		}

		p.credentialSource = CredentialSourceSSM
		return sharedCreds, nil
	}

	p.credentialSource = CredentialSourceEC2
	iprCredentials, err := p.InnerProviders.IPRProvider.Retrieve()
	if err != nil {
		err = fmt.Errorf("failed to retrieve instance profile role credentials. Err: %w", err)
		p.Log.Error(err)
		return iprEmptyCredential, err
	}

	return iprCredentials, nil
}

// RemoteRetrieve uses network calls to retrieve credentials for EC2 instances
func (p *EC2RoleProvider) RemoteRetrieve() (credentials.Value, error) {
	p.Log.Debug("Attempting to retrieve instance profile role")
	if iprCredentials, err := p.iprCredentials(p.SsmEndpoint); err != nil {
		errCode := sdkutil.GetAwsErrorCode(err)
		if _, ok := exceptionsForDefaultHostMgmt[errCode]; ok {
			p.Log.Warnf("Failed to connect to Systems Manager with instance profile role credentials. Err: %v", err)
		} else {
			p.credentialSource = CredentialSourceEC2
			return iprEmptyCredential, fmt.Errorf("failed to call ssm:UpdateInstanceInformation with instance profile role. Err: %w", err)
		}
	} else {
		p.Log.Info("Successfully connected with instance profile role credentials")
		p.credentialSource = CredentialSourceEC2
		return iprCredentials.Get()
	}

	p.Log.Debug("Attempting to retrieve credentials from Systems Manager")
	if ssmCredentials, err := p.ssmEc2Credentials(); err != nil {
		p.Log.Warnf("Failed to connect to Systems Manager with SSM role credentials. %v", err)
		p.credentialSource = CredentialSourceEC2
	} else {
		p.Log.Info("Successfully connected with Systems Manager role credentials")
		p.credentialSource = CredentialSourceSSM
		return ssmCredentials.Get()
	}

	return iprEmptyCredential, fmt.Errorf("no valid credentials could be retrieved for ec2 identity")
}

// iprCredentials retrieves instance profile role credentials and returns an error if the returned credentials cannot
// connect to Systems Manager
func (p *EC2RoleProvider) iprCredentials(ssmEndpoint string) (*credentials.Credentials, error) {
	// Setup SSM client with instance profile role credentials
	iprCredentials := newCredentials(p.InnerProviders.IPRProvider)
	err := p.updateEmptyInstanceInformation(ssmEndpoint, iprCredentials)
	if err != nil {
		if awsErr, ok := err.(awserr.RequestFailure); ok {
			err = fmt.Errorf("retrieved credentials failed to report to ssm. RequestId: %s Error: %w", awsErr.RequestID(), awsErr)
		} else {
			err = fmt.Errorf("retrieved credentials failed to report to ssm. Error: %w", err)
		}

		return nil, err
	}

	err = p.updateIprExpiry(iprCredentials)
	if err != nil {
		p.Log.Errorf("failed to update role provider expiry")
	}

	return iprCredentials, nil
}

// updateIprExpiry updates the expiry of the EC2RoleProvider
// If the token life is greater than 30 minutes then the EC2RoleProvider expiry is set to 30 min
func (p *EC2RoleProvider) updateIprExpiry(iprCredentials *credentials.Credentials) error {
	expiresAt, err := iprCredentials.ExpiresAt()
	if err != nil {
		return fmt.Errorf("unable to get expiration for instance profile role credentials. Err: %w", err)
	}

	durationUntilExpiration := expiresAt.Sub(timeNowFunc())
	if durationUntilExpiration > 30*time.Minute {
		p.Log.Debugf("Reducing instance profile role session duration to 30 minutes")
		p.expirationUpdateLock.Lock()
		defer p.expirationUpdateLock.Unlock()
		p.InnerProviders.IPRProvider.SetExpiration(expiresAt, durationUntilExpiration-30*time.Minute)
	}

	return nil
}

// updateEmptyInstanceInformation calls UpdateInstanceInformation with minimal parameters
func (p *EC2RoleProvider) updateEmptyInstanceInformation(ssmEndpoint string, roleCredentials *credentials.Credentials) error {
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

	_, err := ssmClient.UpdateInstanceInformation(input)
	return err
}

// ssmEc2Credentials sends an instance identity role signed request for an instance role token to Systems Manager
func (p *EC2RoleProvider) ssmEc2Credentials() (*credentials.Credentials, error) {
	// Return credentials if retrievable
	ssmEc2Credentials := credentials.NewCredentials(p.InnerProviders.SsmEc2Provider)
	_, err := p.InnerProviders.SsmEc2Provider.Retrieve()
	if err != nil {
		err = fmt.Errorf("failed to get valid credentials from Systems Manager. Error: %w", err)
		return nil, err
	}

	return ssmEc2Credentials, nil
}

// ShareFile is the credentials file where the agent should write shared credentials
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
func (p *EC2RoleProvider) RemoteExpiresAt() time.Time {
	return p.GetInnerProvider().ExpiresAt()
}

// CredentialSource returns the name of the current provider being used
func (p *EC2RoleProvider) CredentialSource() string {
	return p.credentialSource
}
