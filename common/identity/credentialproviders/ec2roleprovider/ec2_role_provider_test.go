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
	logmocks "github.com/aws/amazon-ssm-agent/agent/mocks/log"
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
	IPRProviderName                     = "IMDS"
	SsmEc2ProviderName                  = "SSM"
	ErrCodeAccessDeniedException        = "AccessDeniedException"
	ErrCodeEC2RoleRequestError          = "EC2RoleRequestError"
	ErrCodeAssumeRoleUnauthorizedAccess = "AssumeRoleUnauthorizedAccess"
)

var (
	errNoInstanceProfileRole           = awserr.New(ErrCodeEC2RoleRequestError, "Instance profile role not found", nil)
	instanceProfileRoleAssumeRoleError = awserr.New(ErrCodeAssumeRoleUnauthorizedAccess, "Failed to assume instance profile role", nil)
	rmirtAccessDeniedError             = awserr.New(ErrCodeAccessDeniedException, "No default host management role", nil)
	uiiThrottleError                   = awserr.New("RateExceeded", "UpdateInstanceInformation requests throttled", nil)
	uiiAccessDeniedError               = awserr.New(ErrCodeAccessDeniedException, "Role does not have ssm:UpdateInstanceInformation permission", nil)
	genericAwsClientError              = fmt.Errorf("generic aws client error")
)

type testCase struct {
	testName                         string
	iprRetrieveErr                   error
	iprUpdateInstanceInformationErr  error
	ssmRetrieveErr                   error
	dhmrUpdateInstanceInformationErr error
}

func arrangeUpdateInstanceInformationFromTestCase(testCase testCase) (*mocks.ISSMClient, *EC2RoleProvider) {
	ssmClient := &mocks.ISSMClient{}
	updateInstanceInfoOutput := &ssm.UpdateInstanceInformationOutput{}
	if testCase.iprRetrieveErr != nil {
		ssmClient.On("UpdateInstanceInformation", mock.Anything).Return(updateInstanceInfoOutput, testCase.iprRetrieveErr).Once()
	}

	if testCase.iprUpdateInstanceInformationErr != nil {
		ssmClient.On("UpdateInstanceInformation", mock.Anything).Return(updateInstanceInfoOutput, testCase.iprUpdateInstanceInformationErr).Once()
	}

	if testCase.ssmRetrieveErr != nil {
		ssmClient.On("UpdateInstanceInformation", mock.Anything).Return(updateInstanceInfoOutput, testCase.ssmRetrieveErr).Once()
	}

	if testCase.dhmrUpdateInstanceInformationErr != nil {
		ssmClient.On("UpdateInstanceInformation", mock.Anything).Return(updateInstanceInfoOutput, testCase.dhmrUpdateInstanceInformationErr).Once()
	}

	newV4ServiceWithCreds = func(log log.T, appConfig *appconfig.SsmagentConfig, credentials *credentials.Credentials, region, defaultEndpoint string) ssmclient.ISSMClient {
		return ssmClient
	}

	ec2RoleProvider := &EC2RoleProvider{
		Log: logmocks.NewMockLog(),
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

	iprProvider := &stubs.InnerProvider{ProviderName: IPRProviderName, RetrieveErr: testCase.iprRetrieveErr}
	ssmProvider := &stubs.InnerProvider{ProviderName: SsmEc2ProviderName, RetrieveErr: testCase.ssmRetrieveErr}
	ec2RoleProvider.InnerProviders = &EC2InnerProviders{
		IPRProvider:    iprProvider,
		SsmEc2Provider: ssmProvider,
	}

	return ssmClient, ec2RoleProvider
}

func arrangeUpdateInstanceInformation(err error) (*mocks.ISSMClient, *EC2RoleProvider) {
	ssmClient := &mocks.ISSMClient{}
	updateInstanceInfoOutput := &ssm.UpdateInstanceInformationOutput{}
	ssmClient.On("UpdateInstanceInformation", mock.Anything).Return(updateInstanceInfoOutput, err).Repeatability = 1
	newV4ServiceWithCreds = func(log log.T, appConfig *appconfig.SsmagentConfig, credentials *credentials.Credentials, region, defaultEndpoint string) ssmclient.ISSMClient {
		return ssmClient
	}

	ec2RoleProvider := &EC2RoleProvider{
		Log: logmocks.NewMockLog(),
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
	testCases := []testCase{
		{
			testName:       "NoInstanceProfileRole",
			iprRetrieveErr: errNoInstanceProfileRole,
		},
		{
			testName:                        "RetrieveIprSuccess_UpdateInstanceInformationAccessDenied",
			iprUpdateInstanceInformationErr: uiiAccessDeniedError,
		},
		{
			testName:       "RetrieveIprAssumeRoleException",
			iprRetrieveErr: instanceProfileRoleAssumeRoleError,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.testName, func(t *testing.T) {
			// Arrange
			ssmClient, ec2RoleProvider := arrangeUpdateInstanceInformationFromTestCase(tc)
			ssmClient.On("UpdateInstanceInformation", mock.Anything).Return(&ssm.UpdateInstanceInformationOutput{}, nil).Once()
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
		})
	}

}

func TestEC2RoleProvider_Retrieve_ReturnsEmptyCredentials(t *testing.T) {
	testCases := []testCase{
		{
			testName:       "NoIpr_RetrieveDhmrAccessDenied",
			iprRetrieveErr: errNoInstanceProfileRole,
			ssmRetrieveErr: rmirtAccessDeniedError,
		},
		{
			testName:       "IprAssumeRoleErr_RetrieveDhmrAccessDenied",
			iprRetrieveErr: instanceProfileRoleAssumeRoleError,
			ssmRetrieveErr: rmirtAccessDeniedError,
		},
		{
			testName:       "NoIpr_RetrieveDhmrInternalServerError",
			iprRetrieveErr: awserr.New(ErrCodeAssumeRoleUnauthorizedAccess, "Failed to assume instance profile role", nil),
			ssmRetrieveErr: &ssm.InternalServerError{},
		},
		{
			testName:                        "RetrieveIprSuccess_UpdateInstanceInformationThrottle",
			iprUpdateInstanceInformationErr: uiiThrottleError,
		},
		{
			testName:                        "NoIpr_RetrieveDhmrSuccess_UpdateInstanceInformationThrottle",
			iprRetrieveErr:                  errNoInstanceProfileRole,
			iprUpdateInstanceInformationErr: uiiThrottleError,
		},
		{
			testName:                         "NoIpr_RetrieveDhmrSuccess_UpdateInstanceInformationInternalServerError",
			iprRetrieveErr:                   errNoInstanceProfileRole,
			dhmrUpdateInstanceInformationErr: &ssm.InternalServerError{},
		},
		{
			testName:                         "NoIpr_RetrieveDhmrSuccess_UpdateInstanceInformationAccessDenied",
			iprRetrieveErr:                   errNoInstanceProfileRole,
			dhmrUpdateInstanceInformationErr: uiiAccessDeniedError,
		},
		{
			testName:                         "NoIpr_RetrieveDhmrSuccess_UpdateInstanceInformationClientError",
			iprRetrieveErr:                   errNoInstanceProfileRole,
			dhmrUpdateInstanceInformationErr: genericAwsClientError,
		},
		{
			testName:                        "RetrieveIprSuccess_UpdateInstanceInformationInternalServerError",
			iprUpdateInstanceInformationErr: &ssm.InternalServerError{},
		},
		{
			testName:                        "RetrieveIprSuccess_UpdateInstanceInformationThrottle",
			iprUpdateInstanceInformationErr: uiiThrottleError,
		},
		{
			testName:                        "RetrieveIprSuccess_UpdateInstanceInformationClientError",
			iprUpdateInstanceInformationErr: genericAwsClientError,
		},
		{
			testName:                         "RetrieveIprSuccess_UpdateInstanceInformationAccessDenied_RetrieveDhmrSuccess_UpdateInstanceInformationAccessDenied",
			iprUpdateInstanceInformationErr:  uiiAccessDeniedError,
			dhmrUpdateInstanceInformationErr: uiiAccessDeniedError,
		},
		{
			testName:                         "RetrieveIprSuccess_UpdateInstanceInformationAccessDenied_RetrieveDhmrSuccess_UpdateInstanceInformationThrottle",
			iprUpdateInstanceInformationErr:  uiiAccessDeniedError,
			dhmrUpdateInstanceInformationErr: uiiThrottleError,
		},
		{
			testName:                         "RetrieveIprSuccess_UpdateInstanceInformationAccessDenied_RetrieveDhmrSuccess_UpdateInstanceInformationInternalServerError",
			iprUpdateInstanceInformationErr:  uiiAccessDeniedError,
			dhmrUpdateInstanceInformationErr: &ssm.InternalServerError{},
		},
		{
			testName:                         "RetrieveIprSuccess_UpdateInstanceInformationAccessDenied_RetrieveDhmrSuccess_UpdateInstanceInformationAwsClientError",
			iprUpdateInstanceInformationErr:  uiiAccessDeniedError,
			dhmrUpdateInstanceInformationErr: genericAwsClientError,
		},
	}

	for _, j := range testCases {
		t.Run(j.testName, func(t *testing.T) {
			// Arrange
			ssmClient := &mocks.ISSMClient{}
			updateInstanceInfoOutput := &ssm.UpdateInstanceInformationOutput{}
			if j.iprRetrieveErr != nil {
				ssmClient.On("UpdateInstanceInformation", mock.Anything).Return(updateInstanceInfoOutput, j.iprRetrieveErr)
			}

			if j.iprUpdateInstanceInformationErr != nil {
				ssmClient.On("UpdateInstanceInformation", mock.Anything).Return(updateInstanceInfoOutput, j.iprUpdateInstanceInformationErr)
			}

			if j.ssmRetrieveErr != nil {
				ssmClient.On("UpdateInstanceInformation", mock.Anything).Return(updateInstanceInfoOutput, j.ssmRetrieveErr)
			}

			if j.dhmrUpdateInstanceInformationErr != nil {
				ssmClient.On("UpdateInstanceInformation", mock.Anything).Return(updateInstanceInfoOutput, j.dhmrUpdateInstanceInformationErr)
			}

			newV4ServiceWithCreds = func(log log.T, appConfig *appconfig.SsmagentConfig, credentials *credentials.Credentials, region, defaultEndpoint string) ssmclient.ISSMClient {
				return ssmClient
			}

			ec2RoleProvider := &EC2RoleProvider{
				Log: logmocks.NewMockLog(),
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

			iprProvider := &stubs.InnerProvider{ProviderName: IPRProviderName, RetrieveErr: j.iprRetrieveErr}
			ssmProvider := &stubs.InnerProvider{ProviderName: SsmEc2ProviderName, RetrieveErr: j.ssmRetrieveErr}
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
		})
	}

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
