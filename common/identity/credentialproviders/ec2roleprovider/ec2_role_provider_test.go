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
	"sync"
	"testing"
	"time"

	"github.com/aws/amazon-ssm-agent/agent/appconfig"
	"github.com/aws/amazon-ssm-agent/agent/log"
	logmocks "github.com/aws/amazon-ssm-agent/agent/mocks/log"
	"github.com/aws/amazon-ssm-agent/common/identity/credentialproviders/ec2roleprovider/stubs"
	"github.com/aws/amazon-ssm-agent/common/identity/credentialproviders/ssmclient"
	"github.com/aws/amazon-ssm-agent/common/identity/credentialproviders/ssmclient/mocks"
	"github.com/aws/amazon-ssm-agent/common/identity/credentialproviders/ssmec2roleprovider"
	"github.com/aws/amazon-ssm-agent/common/runtimeconfig"
	runtimeConfigMocks "github.com/aws/amazon-ssm-agent/common/runtimeconfig/mocks"
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
	couldNotLoadSharedCredentialsErr   = fmt.Errorf("failed to load credentials file")
)

type testCase struct {
	testName                        string
	iprRetrieveErr                  error
	iprUpdateInstanceInformationErr error
	ssmRetrieveErr                  error
	sharedRetrieveErr               error
	runtimeConfigRetrieveErr        error
	runtimeConfig                   runtimeconfig.IdentityRuntimeConfig
	expectedAwsErr                  string
}

func arrangeUpdateInstanceInformationFromTestCase(testCase testCase) (*mocks.ISSMClient, *EC2RoleProvider) {
	ssmClient := &mocks.ISSMClient{}
	updateInstanceInfoOutput := &ssm.UpdateInstanceInformationOutput{}
	if testCase.iprRetrieveErr != nil {
		ssmClient.On("UpdateInstanceInformationWithContext", mock.Anything, mock.Anything).Return(updateInstanceInfoOutput, testCase.iprRetrieveErr).Once()
	}

	if testCase.iprUpdateInstanceInformationErr != nil {
		ssmClient.On("UpdateInstanceInformationWithContext", mock.Anything, mock.Anything).Return(updateInstanceInfoOutput, testCase.iprUpdateInstanceInformationErr).Once()
	}

	if testCase.ssmRetrieveErr != nil {
		ssmClient.On("UpdateInstanceInformationWithContext", mock.Anything, mock.Anything).Return(updateInstanceInfoOutput, testCase.ssmRetrieveErr).Once()
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
		expirationUpdateLock: &sync.Mutex{},
		RuntimeConfigClient:  &runtimeConfigMocks.IIdentityRuntimeConfigClient{},
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
	ssmClient.On("UpdateInstanceInformationWithContext", mock.Anything, mock.Anything).Return(updateInstanceInfoOutput, err).Repeatability = 1
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
		expirationUpdateLock: &sync.Mutex{},
		RuntimeConfigClient:  &runtimeConfigMocks.IIdentityRuntimeConfigClient{},
	}
	return ssmClient, ec2RoleProvider
}

func TestEC2RoleProvider_UpdateEmptyInstanceInformation_Success(t *testing.T) {
	// Arrange
	_, ec2RoleProvider := arrangeUpdateInstanceInformation(nil)
	defaultEndpoint := "ssm.amazon.com"

	// Act
	err := ec2RoleProvider.updateEmptyInstanceInformation(context.Background(), defaultEndpoint, &credentials.Credentials{})

	// Assert
	assert.NoError(t, err)
}

func TestEC2RoleProvider_IPRCredentials_ReturnsIPRCredentials_With1HrSession(t *testing.T) {
	// Arrange
	_, ec2RoleProvider := arrangeUpdateInstanceInformation(nil)
	defaultEndpoint := "ssm.amazon.com"
	now := time.Now()
	timeNowFunc = func() time.Time {
		return now
	}

	newCredentials = func(provider credentials.Provider) *credentials.Credentials {
		creds := credentials.NewCredentials(provider)
		creds.Get()
		return creds
	}

	expectedExpiry := now.Add(1 * time.Hour)

	innerProvider := &stubs.InnerProvider{
		ProviderName: IPRProviderName,
		Expiry:       now.Add(3 * time.Hour),
	}

	ec2RoleProvider.InnerProviders = &EC2InnerProviders{IPRProvider: innerProvider}

	// Act
	creds, err := ec2RoleProvider.iprCredentials(context.Background(), defaultEndpoint)
	credValue, _ := creds.Get()

	// Assert
	assert.NoError(t, err)
	assert.Equal(t, innerProvider.ProviderName, credValue.ProviderName)
	actualExpiry, err := creds.ExpiresAt()
	assert.NoError(t, err)
	assert.Equal(t, expectedExpiry, actualExpiry)

}

func TestEC2RoleProvider_IPRCredentials_ReturnsIPRCredentials_ExpiresAtBeforeNow(t *testing.T) {
	// Arrange
	_, ec2RoleProvider := arrangeUpdateInstanceInformation(nil)
	defaultEndpoint := "ssm.amazon.com"
	now := time.Now()
	timeNowFunc = func() time.Time {
		return now
	}

	newCredentials = func(provider credentials.Provider) *credentials.Credentials {
		creds := credentials.NewCredentials(provider)
		creds.Get()
		return creds
	}

	expectedExpiry := now.Add(1 * time.Hour)

	innerProvider := &stubs.InnerProvider{
		ProviderName: IPRProviderName,
		Expiry:       now.Add(-20 * time.Minute),
	}

	ec2RoleProvider.InnerProviders = &EC2InnerProviders{IPRProvider: innerProvider}

	// Act
	creds, err := ec2RoleProvider.iprCredentials(context.Background(), defaultEndpoint)
	credValue, _ := creds.Get()

	// Assert
	assert.NoError(t, err)
	assert.Equal(t, innerProvider.ProviderName, credValue.ProviderName)
	actualExpiry, err := creds.ExpiresAt()
	assert.NoError(t, err)
	assert.Equal(t, expectedExpiry.Round(time.Second), actualExpiry.Round(time.Second))

}

func TestEC2RoleProvider_IPRCredentials_ReturnsError(t *testing.T) {
	// Arrange
	_, ec2RoleProvider := arrangeUpdateInstanceInformation(fmt.Errorf("failed to call UpdateInstanceInformation"))
	defaultEndpoint := "ssm.amazon.com"
	innerProvider := &stubs.InnerProvider{ProviderName: IPRProviderName}
	ec2RoleProvider.InnerProviders = &EC2InnerProviders{IPRProvider: innerProvider}

	// Act
	creds, err := ec2RoleProvider.iprCredentials(context.Background(), defaultEndpoint)

	// Assert
	assert.Nil(t, creds)
	assert.Error(t, err)
}

func TestEC2RoleProvider_Retrieve_ReturnsIPRCredentials(t *testing.T) {
	// Arrange
	_, ec2RoleProvider := arrangeUpdateInstanceInformation(nil)
	iprProvider := &stubs.InnerProvider{ProviderName: IPRProviderName}
	sharedProvider := &stubs.InnerProvider{ProviderName: credentials.SharedCredsProviderName}
	ec2RoleProvider.InnerProviders = &EC2InnerProviders{
		IPRProvider:               iprProvider,
		SharedCredentialsProvider: sharedProvider,
	}

	runtimeConfigClient := &runtimeConfigMocks.IIdentityRuntimeConfigClient{}
	runtimeConfigClient.On("GetConfigWithRetry").Return(runtimeconfig.IdentityRuntimeConfig{ShareFile: ""}, nil)
	ec2RoleProvider.RuntimeConfigClient = runtimeConfigClient

	// Act
	flag := false
	timeNowFunc = func() time.Time {
		flag = true
		return time.Now()
	}
	creds, err := ec2RoleProvider.Retrieve()
	expiryMins := time.Now().Sub(ec2RoleProvider.ExpiresAt()).Minutes()
	//Assert
	assert.True(t, flag)
	assert.True(t, 28 >= expiryMins && expiryMins <= 30)
	assert.NoError(t, err)
	assert.Equal(t, iprProvider.ProviderName, creds.ProviderName)
	assert.Equal(t, CredentialSourceEC2, ec2RoleProvider.credentialSource)
}

func TestEC2RoleProvider_Retrieve_ReturnsSharedCredentials(t *testing.T) {
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
			ssmClient.On("UpdateInstanceInformationWithContext", mock.Anything, mock.Anything).Return(&ssm.UpdateInstanceInformationOutput{}, nil).Once()
			iprProvider := &stubs.InnerProvider{ProviderName: IPRProviderName}
			ssmProvider := &stubs.InnerProvider{ProviderName: SsmEc2ProviderName}
			sharedProvider := &stubs.InnerProvider{ProviderName: "SharedCredentialsProvider"}
			ec2RoleProvider.InnerProviders = &EC2InnerProviders{
				IPRProvider:               iprProvider,
				SsmEc2Provider:            ssmProvider,
				SharedCredentialsProvider: sharedProvider,
			}

			ec2RoleProvider.credentialSource = CredentialSourceSSM
			runtimeConfigClient := &runtimeConfigMocks.IIdentityRuntimeConfigClient{}
			runtimeConfigClient.On("GetConfigWithRetry").Return(runtimeconfig.IdentityRuntimeConfig{ShareFile: "/some/file/location"}, nil)
			ec2RoleProvider.RuntimeConfigClient = runtimeConfigClient

			// Act
			creds, err := ec2RoleProvider.Retrieve()

			//Assert
			assert.NoError(t, err)
			assert.Equal(t, sharedProvider.ProviderName, creds.ProviderName)
			assert.Equal(t, CredentialSourceSSM, ec2RoleProvider.credentialSource)
		})
	}

}

func TestEC2RoleProvider_Retrieve_ReturnsEmptyCredentials(t *testing.T) {
	testCases := []testCase{
		{
			testName:                 "WhenRuntimeConfigFetchFails_AndInstanceProfileRoleRetrieveError",
			runtimeConfigRetrieveErr: fmt.Errorf("runtime config does not exist"),
			iprRetrieveErr:           errNoInstanceProfileRole,
		},
		{
			testName:       "WhenRuntimeConfigShareFileEmpty_AndInstanceProfileRoleRetrieveError",
			runtimeConfig:  runtimeconfig.IdentityRuntimeConfig{ShareFile: ""},
			iprRetrieveErr: errNoInstanceProfileRole,
		},
		{
			testName:          "WhenRuntimeConfigShareFileNotEmpty_AndShareCredentialLoadError_AndInstanceProfileRoleRetrieveError",
			runtimeConfig:     runtimeconfig.IdentityRuntimeConfig{ShareFile: "/shared/creds/path"},
			sharedRetrieveErr: couldNotLoadSharedCredentialsErr,
			iprRetrieveErr:    errNoInstanceProfileRole,
		},
	}

	for _, j := range testCases {
		t.Run(j.testName, func(t *testing.T) {
			// Arrange
			ec2RoleProvider := arrangeRetrieveEmptyTest(j)

			// Act
			creds, err := ec2RoleProvider.Retrieve()

			//Assert
			assert.Error(t, err)
			assert.Equal(t, iprEmptyCredential, creds)
			assert.Equal(t, CredentialSourceNone, ec2RoleProvider.credentialSource)
		})
	}
}

func TestEC2RoleProvider_RetrieveRemote_ReturnsEmptyCredentials(t *testing.T) {
	testCases := []testCase{
		{
			testName:       "NoIpr_RetrieveDhmrAccessDenied",
			iprRetrieveErr: errNoInstanceProfileRole,
			ssmRetrieveErr: rmirtAccessDeniedError,
			expectedAwsErr: ErrCodeAccessDeniedException,
		},
		{
			testName:       "IprAssumeRoleErr_RetrieveDhmrAccessDenied",
			iprRetrieveErr: instanceProfileRoleAssumeRoleError,
			ssmRetrieveErr: rmirtAccessDeniedError,
			expectedAwsErr: ErrCodeAccessDeniedException,
		},
		{
			testName:       "NoIpr_RetrieveDhmrInternalServerError",
			iprRetrieveErr: awserr.New(ErrCodeAssumeRoleUnauthorizedAccess, "Failed to assume instance profile role", nil),
			ssmRetrieveErr: &ssm.InternalServerError{},
			expectedAwsErr: ssm.ErrCodeInternalServerError,
		},
		{
			testName:                        "RetrieveIprSuccess_UpdateInstanceInformationThrottle",
			iprUpdateInstanceInformationErr: uiiThrottleError,
			expectedAwsErr:                  "RateExceeded",
		},
		{
			testName:                        "RetrieveIprSuccess_UpdateInstanceInformationInternalServerError",
			iprUpdateInstanceInformationErr: &ssm.InternalServerError{},
			expectedAwsErr:                  ssm.ErrCodeInternalServerError,
		},
		{
			testName:                        "RetrieveIprSuccess_UpdateInstanceInformationThrottle",
			iprUpdateInstanceInformationErr: uiiThrottleError,
			expectedAwsErr:                  "RateExceeded",
		},
		{
			testName:                        "RetrieveIprSuccess_UpdateInstanceInformationClientError",
			iprUpdateInstanceInformationErr: genericAwsClientError,
		},
	}

	for _, j := range testCases {
		t.Run(j.testName, func(t *testing.T) {
			// Arrange
			ec2RoleProvider := arrangeRetrieveEmptyTest(j)

			// Act
			creds, err := ec2RoleProvider.RemoteRetrieve(context.Background())

			//Assert
			if j.expectedAwsErr != "" {
				var awsErr awserr.Error
				isAwsErr := errors.As(err, &awsErr)
				assert.True(t, isAwsErr)
				assert.Equal(t, j.expectedAwsErr, awsErr.Code())
			}
			assert.Equal(t, iprEmptyCredential, creds)
			assert.Equal(t, CredentialSourceNone, ec2RoleProvider.credentialSource)
		})
	}
}

func arrangeRetrieveEmptyTest(j testCase) *EC2RoleProvider {
	ssmClient := &mocks.ISSMClient{}
	updateInstanceInfoOutput := &ssm.UpdateInstanceInformationOutput{}
	runtimeConfigClient := &runtimeConfigMocks.IIdentityRuntimeConfigClient{}

	if j.iprUpdateInstanceInformationErr != nil {
		ssmClient.On("UpdateInstanceInformationWithContext", mock.Anything, mock.Anything).Return(updateInstanceInfoOutput, j.iprUpdateInstanceInformationErr)
	}

	if j.ssmRetrieveErr != nil {
		ssmClient.On("UpdateInstanceInformationWithContext", mock.Anything, mock.Anything).Return(updateInstanceInfoOutput, j.ssmRetrieveErr)
	}

	if j.ssmRetrieveErr != nil {
		runtimeConfigClient.On("GetConfigWithRetry").Return(runtimeconfig.IdentityRuntimeConfig{}, j.ssmRetrieveErr)
	} else {
		runtimeConfigClient.On("GetConfigWithRetry").Return(j.runtimeConfig, j.ssmRetrieveErr)
	}

	newV4ServiceWithCreds = func(log log.T, appConfig *appconfig.SsmagentConfig, credentials *credentials.Credentials, region, defaultEndpoint string) ssmclient.ISSMClient {
		return ssmClient
	}

	log := logmocks.NewMockLog()
	instanceInfo := &ssmec2roleprovider.InstanceInfo{
		InstanceId: "SomeInstanceId",
		Region:     "SomeRegion",
	}
	config := &appconfig.SsmagentConfig{
		Agent: appconfig.AgentInfo{
			Version: "3.1.0.0",
		},
	}

	iprProvider := &stubs.InnerProvider{ProviderName: IPRProviderName, RetrieveErr: j.iprRetrieveErr}
	ssmProvider := &stubs.InnerProvider{ProviderName: SsmEc2ProviderName, RetrieveErr: j.ssmRetrieveErr}
	sharedProvider := &stubs.InnerProvider{ProviderName: "SharedCredentialsProvider", RetrieveErr: j.sharedRetrieveErr}
	innerProviders := &EC2InnerProviders{
		IPRProvider:               iprProvider,
		SsmEc2Provider:            ssmProvider,
		SharedCredentialsProvider: sharedProvider,
	}

	ssmEndpoint := "ssm.amazonaws.com"

	return NewEC2RoleProvider(log, config, innerProviders, instanceInfo, ssmEndpoint, runtimeConfigClient)
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
