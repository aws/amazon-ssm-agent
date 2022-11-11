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
	"testing"
	"time"

	"github.com/aws/amazon-ssm-agent/agent/appconfig"
	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/amazon-ssm-agent/common/identity/credentialproviders/ec2roleprovider/stubs"
	"github.com/aws/amazon-ssm-agent/common/identity/credentialproviders/ssmclient"
	"github.com/aws/amazon-ssm-agent/common/identity/credentialproviders/ssmclient/mocks"
	"github.com/aws/amazon-ssm-agent/common/identity/credentialproviders/ssmec2roleprovider"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/service/ssm"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

const (
	IPRProviderName    = "IMDS"
	SsmEc2ProviderName = "SSM"
)

func arrangeUpdateInstanceInformation(err error) (*mocks.ISSMClient, *EC2RoleProvider) {
	ssmClient := &mocks.ISSMClient{}
	updateInstanceInfoOutput := &ssm.UpdateInstanceInformationOutput{}
	ssmClient.On("UpdateInstanceInformation", mock.Anything).Return(updateInstanceInfoOutput, err).Repeatability = 1
	newV4ServiceWithCreds = func(log log.T, appConfig *appconfig.SsmagentConfig, credentials *credentials.Credentials, region, defaultEndpoint string) ssmclient.ISSMClient {
		return ssmClient
	}

	ec2RoleProvider := &EC2RoleProvider{
		Log: log.NewMockLog(),
		InstanceInfo: &ssmec2roleprovider.InstanceInfo{
			InstanceId: "SomeInstanceId",
			Region:     "SomeRegion",
		},
		Config: &appconfig.SsmagentConfig{
			Agent: appconfig.AgentInfo{
				Version: "3.1.0.0",
			},
		},
	}
	return ssmClient, ec2RoleProvider
}

func TestEC2RoleProvider_UpdateEmptyInstanceInformation_Success(t *testing.T) {
	// Arrange
	_, ec2RoleProvider := arrangeUpdateInstanceInformation(nil)
	defaultEndpoint := "ssm.amazon.com"

	// Act
	err := ec2RoleProvider.updateEmptyInstanceInformation(defaultEndpoint, &credentials.Credentials{})

	// Assert
	assert.NoError(t, err)
}

func TestEC2RoleProvider_IsExpired_DefaultExpirationDate_True(t *testing.T) {
	_, ec2RoleProvider := arrangeUpdateInstanceInformation(nil)
	ec2RoleProvider.credentialSource = CredentialSourceEC2
	isExpired := ec2RoleProvider.IsExpired()
	assert.True(t, isExpired)
	assert.Equal(t, ec2RoleProvider.currentCredentialExpiration, time.Time{})
}

func TestEC2RoleProvider_IsExpired_OldExpirationDate_True(t *testing.T) {
	_, ec2RoleProvider := arrangeUpdateInstanceInformation(nil)
	ec2RoleProvider.credentialSource = CredentialSourceEC2
	oldExpirationDate := time.Now().Add(-2 * time.Hour)
	ec2RoleProvider.currentCredentialExpiration = oldExpirationDate
	isExpired := ec2RoleProvider.IsExpired()
	assert.True(t, isExpired)
	assert.Equal(t, ec2RoleProvider.currentCredentialExpiration, oldExpirationDate)
}

func TestEC2RoleProvider_IsExpired_ExpirationDate_False(t *testing.T) {
	_, ec2RoleProvider := arrangeUpdateInstanceInformation(nil)
	ec2RoleProvider.credentialSource = CredentialSourceEC2
	expirationDate := time.Now().Add(5 * time.Second)
	ec2RoleProvider.currentCredentialExpiration = expirationDate
	isExpired := ec2RoleProvider.IsExpired()
	assert.False(t, isExpired)
	assert.Equal(t, ec2RoleProvider.currentCredentialExpiration, expirationDate)
}

func TestEC2RoleProvider_IPRCredentials_ReturnsIPRCredentials(t *testing.T) {
	// Arrange
	_, ec2RoleProvider := arrangeUpdateInstanceInformation(nil)
	defaultEndpoint := "ssm.amazon.com"
	innerProvider := &stubs.InnerProvider{ProviderName: IPRProviderName}
	ec2RoleProvider.InnerProviders = &EC2InnerProviders{IPRProvider: innerProvider}

	// Act
	creds, err := ec2RoleProvider.iprCredentials(defaultEndpoint)
	credValue, _ := creds.Get()

	// Assert
	assert.NoError(t, err)
	assert.Equal(t, innerProvider.ProviderName, credValue.ProviderName)
}

func TestEC2RoleProvider_IPRCredentials_ReturnsError(t *testing.T) {
	// Arrange
	_, ec2RoleProvider := arrangeUpdateInstanceInformation(fmt.Errorf("failed to call UpdateInstanceInformation"))
	defaultEndpoint := "ssm.amazon.com"
	innerProvider := &stubs.InnerProvider{ProviderName: IPRProviderName}
	ec2RoleProvider.InnerProviders = &EC2InnerProviders{IPRProvider: innerProvider}

	// Act
	creds, err := ec2RoleProvider.iprCredentials(defaultEndpoint)

	// Assert
	assert.Nil(t, creds)
	assert.Error(t, err)
}

func TestEC2RoleProvider_SsmEc2Credentials_ReturnsSsmEc2Credentials(t *testing.T) {
	// Arrange
	_, ec2RoleProvider := arrangeUpdateInstanceInformation(nil)
	defaultEndpoint := "ssm.amazon.com"
	innerProvider := &stubs.InnerProvider{ProviderName: SsmEc2ProviderName}
	ec2RoleProvider.InnerProviders = &EC2InnerProviders{SsmEc2Provider: innerProvider}

	// Act
	creds, err := ec2RoleProvider.ssmEc2Credentials(defaultEndpoint)
	credValue, _ := creds.Get()

	// Assert
	assert.NoError(t, err)
	assert.Equal(t, innerProvider.ProviderName, credValue.ProviderName)
}

func TestEC2RoleProvider_SsmEc2Credentials_ReturnsError(t *testing.T) {
	// Arrange
	_, ec2RoleProvider := arrangeUpdateInstanceInformation(fmt.Errorf("failed to call UpdateInstanceInformation"))
	defaultEndpoint := "ssm.amazon.com"
	innerProvider := &stubs.InnerProvider{ProviderName: SsmEc2ProviderName}
	ec2RoleProvider.InnerProviders = &EC2InnerProviders{SsmEc2Provider: innerProvider}

	// Act
	creds, err := ec2RoleProvider.ssmEc2Credentials(defaultEndpoint)

	// Assert
	assert.Nil(t, creds)
	assert.Error(t, err)
}

func TestEC2RoleProvider_Retrieve_ReturnsIPRCredentials(t *testing.T) {
	// Arrange
	_, ec2RoleProvider := arrangeUpdateInstanceInformation(nil)
	innerProvider := &stubs.InnerProvider{ProviderName: IPRProviderName}
	ec2RoleProvider.InnerProviders = &EC2InnerProviders{IPRProvider: innerProvider}

	// Act
	creds, err := ec2RoleProvider.Retrieve()

	//Assert
	assert.NoError(t, err)
	assert.Equal(t, innerProvider.ProviderName, creds.ProviderName)
	assert.Equal(t, CredentialSourceEC2, ec2RoleProvider.credentialSource)
}

func TestEC2RoleProvider_Retrieve_ReturnsSSMCredentials(t *testing.T) {
	// Arrange
	awsErr := awserr.New("400", "Unauthorized", nil)
	iprErr := awserr.NewRequestFailure(awsErr, 400, "testRequestId")
	ssmClient, ec2RoleProvider := arrangeUpdateInstanceInformation(iprErr)
	ssmClient.On("UpdateInstanceInformation", mock.Anything).Return(&ssm.UpdateInstanceInformationOutput{}, nil)
	iprProvider := &stubs.InnerProvider{ProviderName: IPRProviderName}
	ssmProvider := &stubs.InnerProvider{ProviderName: SsmEc2ProviderName}
	ec2RoleProvider.InnerProviders = &EC2InnerProviders{
		IPRProvider:    iprProvider,
		SsmEc2Provider: ssmProvider,
	}

	// Act
	creds, err := ec2RoleProvider.Retrieve()

	//Assert
	assert.NoError(t, err)
	assert.Equal(t, ssmProvider.ProviderName, creds.ProviderName)
	assert.Equal(t, CredentialSourceSSM, ec2RoleProvider.credentialSource)
}

func TestEC2RoleProvider_Retrieve_ReturnsEmptyCredentials(t *testing.T) {
	// Arrange
	awsErr := awserr.New("400", "Unauthorized", nil)
	updateInstanceInformationErr := awserr.NewRequestFailure(awsErr, 400, "testRequestId")
	ssmClient, ec2RoleProvider := arrangeUpdateInstanceInformation(updateInstanceInformationErr)
	ssmClient.On("UpdateInstanceInformation", mock.Anything).
		Return(nil, updateInstanceInformationErr).
		Repeatability = 1
	iprProvider := &stubs.InnerProvider{ProviderName: IPRProviderName}
	ssmProvider := &stubs.InnerProvider{ProviderName: SsmEc2ProviderName}
	ec2RoleProvider.InnerProviders = &EC2InnerProviders{
		IPRProvider:    iprProvider,
		SsmEc2Provider: ssmProvider,
	}

	// Act
	creds, err := ec2RoleProvider.Retrieve()

	//Assert
	assert.Error(t, err)
	assert.Equal(t, iprEmptyCredential, creds)
	assert.Equal(t, CredentialSourceEC2, ec2RoleProvider.credentialSource)
}

func TestEC2RoleProvider_GetInnerProvider_ReturnsIPRProvider_WhenCredentialSourceEmpty(t *testing.T) {
	iprProvider := &stubs.InnerProvider{ProviderName: IPRProviderName}
	ssmProvider := &stubs.InnerProvider{ProviderName: SsmEc2ProviderName}
	innerProviders := &EC2InnerProviders{
		IPRProvider:    iprProvider,
		SsmEc2Provider: ssmProvider,
	}

	ec2RoleProvider := &EC2RoleProvider{
		InnerProviders: innerProviders,
	}

	assert.Equal(t, iprProvider, ec2RoleProvider.GetInnerProvider())
}

func TestEC2RoleProvider_GetInnerProvider_ReturnsIPRProvider_WhenCredentialSourceIsEC2(t *testing.T) {
	iprProvider := &stubs.InnerProvider{ProviderName: IPRProviderName}
	ssmProvider := &stubs.InnerProvider{ProviderName: SsmEc2ProviderName}
	innerProviders := &EC2InnerProviders{
		IPRProvider:    iprProvider,
		SsmEc2Provider: ssmProvider,
	}

	ec2RoleProvider := &EC2RoleProvider{
		InnerProviders:   innerProviders,
		credentialSource: CredentialSourceEC2,
	}

	assert.Equal(t, iprProvider, ec2RoleProvider.GetInnerProvider())
}

func TestEC2RoleProvider_GetInnerProvider_ReturnsSsmEc2Provider_WhenCredentialSourceIsSSM(t *testing.T) {
	iprProvider := &stubs.InnerProvider{ProviderName: IPRProviderName}
	ssmProvider := &stubs.InnerProvider{ProviderName: SsmEc2ProviderName}
	innerProviders := &EC2InnerProviders{
		IPRProvider:    iprProvider,
		SsmEc2Provider: ssmProvider,
	}

	ec2RoleProvider := &EC2RoleProvider{
		InnerProviders:   innerProviders,
		credentialSource: CredentialSourceSSM,
	}

	assert.Equal(t, ssmProvider, ec2RoleProvider.GetInnerProvider())
}
