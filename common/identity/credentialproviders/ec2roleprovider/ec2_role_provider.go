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
	"time"

	"github.com/aws/amazon-ssm-agent/agent/appconfig"
	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/amazon-ssm-agent/agent/version"
	"github.com/aws/amazon-ssm-agent/common/identity/credentialproviders/ssmec2roleprovider"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/service/ssm"
)

// EC2RoleProvider provides credentials for the agent when on an EC2 instance
type EC2RoleProvider struct {
	InnerProviders         *EC2InnerProviders
	Log                    log.T
	Config                 *appconfig.SsmagentConfig
	InstanceInfo           *ssmec2roleprovider.InstanceInfo
	credentialSource       string
	SsmEndpoint            string
	ShareFileLocation      string
	CredentialProfile      string
	ShouldShareCredentials bool
}

// GetInnerProvider gets the role provider that is currently being used for credentials
func (p *EC2RoleProvider) GetInnerProvider() IInnerProvider {
	if p.credentialSource == CredentialSourceSSM {
		return p.InnerProviders.SsmEc2Provider
	}

	return p.InnerProviders.IPRProvider
}

// Retrieve returns instance profile role credentials if it has sufficient systems manager permissions and
// returns ssm provided credentials otherwise. If neither can be retrieved then empty credentials are returned
func (p *EC2RoleProvider) Retrieve() (credentials.Value, error) {
	p.Log.Debug("attempting to retrieve instance profile role")
	if iprCredentials, err := p.iprCredentials(p.SsmEndpoint); err != nil {
		p.Log.Debugf("failed to connect to Systems Manager with instance profile role credentials. Err: %v", err)
	} else {
		p.credentialSource = CredentialSourceEC2
		return iprCredentials.Get()
	}

	p.Log.Debug("attempting to retrieve role from Systems Manager")
	if ssmCredentials, err := p.ssmEc2Credentials(p.SsmEndpoint); err != nil {
		p.Log.Debugf("failed to connect to Systems Manager with SSM role credentials. v%", err)
		p.credentialSource = CredentialSourceEC2
	} else {
		p.credentialSource = CredentialSourceSSM
		return ssmCredentials.Get()
	}

	return iprEmptyCredential, fmt.Errorf("no valid credentials could be retrieved for ec2 identity")
}

// iprCredentials retrieves instance profile role credentials and returns an error if the returned credentials cannot
// connect to Systems Manager
func (p *EC2RoleProvider) iprCredentials(ssmEndpoint string) (*credentials.Credentials, error) {
	// Setup SSM client with instance profile role credentials
	iprCredentials := credentials.NewCredentials(p.InnerProviders.IPRProvider)
	err := p.updateEmptyInstanceInformation(ssmEndpoint, iprCredentials)
	if err != nil {
		if awsErr, ok := err.(awserr.RequestFailure); ok {
			err = fmt.Errorf("retrieved credentials failed to report to ssm. RequestId: %s Error: %w", awsErr.RequestID(), awsErr)
		} else {
			err = fmt.Errorf("retrieved credentials failed to report to ssm. Error: %w", err)
		}

		return nil, err
	}

	return iprCredentials, nil
}

func (p *EC2RoleProvider) updateEmptyInstanceInformation(ssmEndpoint string, roleCredentials *credentials.Credentials) error {
	ssmClient := newV4ServiceWithCreds(p.Log.WithContext("SSMService"), p.Config, roleCredentials, p.InstanceInfo.Region, ssmEndpoint)

	p.Log.Debugf("calling UpdateInstanceInformation with agent version %s", p.Config.Agent.Version)
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
func (p *EC2RoleProvider) ssmEc2Credentials(ssmEndpoint string) (*credentials.Credentials, error) {
	// Return credentials if retrievable
	ssmEc2Credentials := credentials.NewCredentials(p.InnerProviders.SsmEc2Provider)
	_, err := p.InnerProviders.SsmEc2Provider.Retrieve()
	if err != nil {
		err = fmt.Errorf("failed to get valid credentials from Systems Manager. Error: %w", err)
		return nil, err
	}

	if err = p.updateEmptyInstanceInformation(ssmEndpoint, ssmEc2Credentials); err != nil {
		err = fmt.Errorf("returned SSM credentials unable to call UpdateInstanceInformation API. %w", err)
		return nil, err
	}

	return ssmEc2Credentials, nil
}

// IsExpired wraps the IsExpired method of the current provider
func (p *EC2RoleProvider) IsExpired() bool {
	return p.GetInnerProvider().IsExpired()
}

// ExpiresAt wraps the ExpiresAt method of the current provider
func (p *EC2RoleProvider) ExpiresAt() time.Time {
	return p.GetInnerProvider().ExpiresAt()
}

// ShareFile is the credentials file where the agent should write shared credentials
func (p *EC2RoleProvider) ShareFile() string {
	return p.ShareFileLocation
}

// ShareProfile is the profile where the agent should write shared credentials
func (p *EC2RoleProvider) ShareProfile() string {
	return p.CredentialProfile
}

// SharesCredentials returns true if credentials refresher in core agent should save returned credentials to disk
func (p *EC2RoleProvider) SharesCredentials() bool {
	return p.ShouldShareCredentials
}
