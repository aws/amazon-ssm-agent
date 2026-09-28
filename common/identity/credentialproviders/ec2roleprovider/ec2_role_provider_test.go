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
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/credentials/ec2rolecreds"
	"github.com/aws/smithy-go"

	"github.com/aws/amazon-ssm-agent/agent/appconfig"
	"github.com/aws/amazon-ssm-agent/agent/log"

	logmocks "github.com/aws/amazon-ssm-agent/agent/mocks/log"
	awserr "github.com/aws/amazon-ssm-agent/agent/sdkutil"
	"github.com/aws/amazon-ssm-agent/common/identity/credentialproviders/ec2roleprovider/stubs"
	"github.com/aws/amazon-ssm-agent/common/identity/credentialproviders/ssmclient"
	"github.com/aws/amazon-ssm-agent/common/identity/credentialproviders/ssmclient/mocks"
	"github.com/aws/amazon-ssm-agent/common/identity/credentialproviders/ssmec2roleprovider"
	"github.com/aws/amazon-ssm-agent/common/runtimeconfig"
	runtimeConfigMocks "github.com/aws/amazon-ssm-agent/common/runtimeconfig/mocks"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
	ssmtypes "github.com/aws/aws-sdk-go-v2/service/ssm/types"
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
	expectedErr                     string
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

	newV4ServiceWithCreds = func(log log.T, credentialsProvider aws.CredentialsProvider, region, defaultEndpoint string) ssmclient.ISSMClient {
		return ssmClient
	}

	ec2RoleProvider := &EC2RoleProvider{
		Log: logmocks.NewMockLog(),
		InstanceInfo: &ssmec2roleprovider.InstanceInfo{
			InstanceId: "SomeInstanceId",
			Region:     "SomeRegion",
		},

		RuntimeConfigClient: &runtimeConfigMocks.IIdentityRuntimeConfigClient{},
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
	ssmClient.On("UpdateInstanceInformation", mock.Anything, mock.Anything).Return(updateInstanceInfoOutput, err).Repeatability = 1
	newV4ServiceWithCreds = func(log log.T, credentialsProvider aws.CredentialsProvider, region string, defaultEndpoint string) ssmclient.ISSMClient {
		return ssmClient
	}

	ec2RoleProvider := &EC2RoleProvider{
		Log: logmocks.NewMockLog(),
		InstanceInfo: &ssmec2roleprovider.InstanceInfo{
			InstanceId: "SomeInstanceId",
			Region:     "SomeRegion",
		},

		RuntimeConfigClient: &runtimeConfigMocks.IIdentityRuntimeConfigClient{},
	}
	return ssmClient, ec2RoleProvider
}

func TestEC2RoleProvider_UpdateEmptyInstanceInformation_Success(t *testing.T) {
	// Arrange
	_, ec2RoleProvider := arrangeUpdateInstanceInformation(nil)
	defaultEndpoint := "ssm.amazon.com"

	innerProvider := &stubs.InnerProvider{
		ProviderName: IPRProviderName,
	}

	ec2RoleProvider.InnerProviders = &EC2InnerProviders{IPRProvider: innerProvider}

	// Act
	err := ec2RoleProvider.updateEmptyInstanceInformation(context.Background(), defaultEndpoint, ec2RoleProvider.GetInnerProvider())

	// Assert
	assert.NoError(t, err)
}

func TestEC2RoleProvider_UpdateEmptyInstanceInformation_ReturnsNestedError(t *testing.T) {
	// Arrange
	_, ec2RoleProvider := arrangeUpdateInstanceInformation(awserr.New(ErrCodeEC2RoleRequestError, "Instance profile role not found", genericAwsClientError))
	defaultEndpoint := "ssm.amazon.com"

	innerProvider := &stubs.InnerProvider{
		ProviderName: IPRProviderName,
	}

	ec2RoleProvider.InnerProviders = &EC2InnerProviders{IPRProvider: innerProvider}

	// Act
	err := ec2RoleProvider.updateEmptyInstanceInformation(context.Background(), defaultEndpoint, ec2RoleProvider.GetInnerProvider())

	// Assert
	assert.Error(t, err)
	assert.Contains(t, err.Error(), ErrCodeEC2RoleRequestError)
	assert.Contains(t, err.Error(), genericAwsClientError.Error())
}

func TestEC2RoleProvider_IPRCredentials_ReturnsIPRCredentials_With1HrSession(t *testing.T) {
	// Arrange
	_, ec2RoleProvider := arrangeUpdateInstanceInformation(nil)
	defaultEndpoint := "ssm.amazon.com"
	now := time.Now()
	timeNowFunc = func() time.Time {
		return now
	}

	newCredentials = func(optFns ...func(*ec2rolecreds.Options)) *ec2rolecreds.Provider {
		return &ec2rolecreds.Provider{}
	}

	expectedExpiry := now.Add(1 * time.Hour)

	innerProvider := &stubs.InnerProvider{
		ProviderName: IPRProviderName,
		Expiry:       now.Add(3 * time.Hour),
	}

	ec2RoleProvider.InnerProviders = &EC2InnerProviders{IPRProvider: innerProvider}

	// Act
	creds, err := ec2RoleProvider.iprCredentials(context.Background(), defaultEndpoint)
	//credValue, _ := creds.Retrieve(context.TODO())

	// Assert
	assert.NoError(t, err)
	assert.Equal(t, innerProvider.ProviderName, creds.Source)
	actualExpiry := creds.Expires
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

	newCredentials = func(optFns ...func(*ec2rolecreds.Options)) *ec2rolecreds.Provider {
		return &ec2rolecreds.Provider{}
	}

	expectedExpiry := now.Add(1 * time.Hour)

	innerProvider := &stubs.InnerProvider{
		ProviderName: IPRProviderName,
		Expiry:       expectedExpiry,
	}

	ec2RoleProvider.InnerProviders = &EC2InnerProviders{IPRProvider: innerProvider}

	// Act
	creds, err := ec2RoleProvider.iprCredentials(context.Background(), defaultEndpoint)
	//credValue, _ := creds.Retrieve(context.TODO())

	// Assert
	assert.NoError(t, err)
	assert.Equal(t, innerProvider.ProviderName, creds.Source)
	actualExpiry := creds.Expires
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
	assert.Equal(t, iprEmptyCredential, creds)
	assert.Error(t, err)
}

func TestEC2RoleProvider_Retrieve_ReturnsIPRCredentials(t *testing.T) {
	// Arrange
	_, ec2RoleProvider := arrangeUpdateInstanceInformation(nil)
	iprProvider := &stubs.InnerProvider{ProviderName: IPRProviderName}
	sharedProvider := &stubs.InnerProvider{ProviderName: "SharedCredentialsProvider"}
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
	creds, err := ec2RoleProvider.Retrieve(context.TODO())
	expiryMins := time.Now().Sub(ec2RoleProvider.ExpiresAt()).Minutes()
	//Assert
	assert.True(t, flag)
	assert.True(t, 28 >= expiryMins && expiryMins <= 30)
	assert.NoError(t, err)
	assert.Equal(t, iprProvider.ProviderName, creds.Source)
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
			creds, err := ec2RoleProvider.Retrieve(context.TODO())

			//Assert
			assert.NoError(t, err)
			assert.Equal(t, sharedProvider.ProviderName, creds.Source)
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
			creds, err := ec2RoleProvider.Retrieve(context.TODO())

			//Assert
			assert.Error(t, err)
			assert.Equal(t, iprEmptyCredential, creds)
			assert.Equal(t, CredentialSourceNone, ec2RoleProvider.credentialSource)
		})
	}
}

func TestEC2RoleProvider_Retrieve_ReturnsNestedErr(t *testing.T) {
	testCases := []testCase{
		{
			testName:       "InstanceProfileRoleRetrieveErrorWithOrigError_ReturnsNoOrigError",
			iprRetrieveErr: awserr.New(ErrCodeEC2RoleRequestError, "Instance profile role not found", genericAwsClientError),
		},
	}

	for _, j := range testCases {
		t.Run(j.testName, func(t *testing.T) {
			// Arrange
			ec2RoleProvider := arrangeRetrieveEmptyTest(j)

			// Act
			_, err := ec2RoleProvider.Retrieve(context.TODO())

			//Assert
			assert.Error(t, err)
			assert.Contains(t, err.Error(), ErrCodeEC2RoleRequestError)
			assert.NotContains(t, err.Error(), genericAwsClientError.Error())
		})
	}
}

func TestEC2RoleProvider_RetrieveRemote_ReturnsEmptyCredentials(t *testing.T) {
	testCases := []testCase{
		{
			testName:       "NoIpr_RetrieveDhmrAccessDenied",
			iprRetrieveErr: errNoInstanceProfileRole,
			ssmRetrieveErr: rmirtAccessDeniedError,
			expectedErr:    ErrCodeAccessDeniedException,
		},
		{
			testName:       "IprAssumeRoleErr_RetrieveDhmrAccessDenied",
			iprRetrieveErr: instanceProfileRoleAssumeRoleError,
			ssmRetrieveErr: rmirtAccessDeniedError,
			expectedErr:    ErrCodeAccessDeniedException,
		},
		{
			testName:       "NoIpr_RetrieveDhmrInternalServerError",
			iprRetrieveErr: awserr.New(ErrCodeAssumeRoleUnauthorizedAccess, "Failed to assume instance profile role", nil),
			ssmRetrieveErr: &ssmtypes.InternalServerError{},
			expectedErr:    "InternalServerError",
		},
		{
			testName:                        "RetrieveIprSuccess_UpdateInstanceInformationThrottle",
			iprUpdateInstanceInformationErr: uiiThrottleError,
			expectedErr:                     "RateExceeded",
		},
		{
			testName:                        "RetrieveIprSuccess_UpdateInstanceInformationInternalServerError",
			iprUpdateInstanceInformationErr: &ssmtypes.InternalServerError{},
			expectedErr:                     "InternalServerError",
		},
		{
			testName:                        "RetrieveIprSuccess_UpdateInstanceInformationThrottle",
			iprUpdateInstanceInformationErr: uiiThrottleError,
			expectedErr:                     "RateExceeded",
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
			if j.expectedErr != "" {
				var apiErr smithy.APIError
				if errors.As(err, &apiErr) {
					assert.Equal(t, j.expectedErr, apiErr.ErrorCode())
				} else {
					// Handle case where it's not a smithy.APIError
					var awsErr awserr.Error
					isAwsErr := errors.As(err, &awsErr)
					assert.True(t, isAwsErr)
					assert.Equal(t, j.expectedErr, awsErr.Code())
				}
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
		ssmClient.On("UpdateInstanceInformation", mock.Anything, mock.Anything).Return(updateInstanceInfoOutput, j.iprUpdateInstanceInformationErr)
	}

	if j.ssmRetrieveErr != nil {
		ssmClient.On("UpdateInstanceInformation", mock.Anything, mock.Anything).Return(updateInstanceInfoOutput, j.ssmRetrieveErr)
	}

	if j.ssmRetrieveErr != nil {
		runtimeConfigClient.On("GetConfigWithRetry").Return(runtimeconfig.IdentityRuntimeConfig{}, j.ssmRetrieveErr)
	} else {
		runtimeConfigClient.On("GetConfigWithRetry").Return(j.runtimeConfig, j.ssmRetrieveErr)
	}

	newV4ServiceWithCreds = func(log log.T, credentialsProvider aws.CredentialsProvider, region string, defaultEndpoint string) ssmclient.ISSMClient {
		return ssmClient
	}

	log := logmocks.NewMockLog()
	instanceInfo := &ssmec2roleprovider.InstanceInfo{
		InstanceId: "SomeInstanceId",
		Region:     "SomeRegion",
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

	return NewEC2RoleProvider(log, innerProviders, instanceInfo, ssmEndpoint, runtimeConfigClient)
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

func TestEC2RoleProvider_ShareFile_ReturnsDefaultPathWhenSSMCredentialSource(t *testing.T) {
	// Arrange
	ec2RoleProvider := &EC2RoleProvider{
		credentialSource: CredentialSourceSSM,
	}

	// Act
	shareFilePath := ec2RoleProvider.ShareFile()

	// Assert
	assert.Equal(t, appconfig.DefaultEC2SharedCredentialsFilePath, shareFilePath)
}

func TestEC2RoleProvider_ShareFile_ReturnsEmptyStringForNonSSMSources(t *testing.T) {
	// Arrange
	ec2RoleProvider := &EC2RoleProvider{
		credentialSource: CredentialSourceEC2,
	}

	// Act
	shareFilePath := ec2RoleProvider.ShareFile()

	// Assert
	assert.Equal(t, "", shareFilePath)
}

func TestEC2RoleProvider_ShareProfile(t *testing.T) {
	// Arrange
	ec2RoleProvider := &EC2RoleProvider{}
	// Act
	shareProfile := ec2RoleProvider.ShareProfile()
	// Assert
	assert.Equal(t, "", shareProfile, "ShareProfile should return an empty string even with SSM credential source")
}

func TestEC2RoleProvider_SharesCredentials_DefaultBehavior(t *testing.T) {
	// Explanation: Verify that SharesCredentials returns true by default
	// This test ensures the default implementation always returns true

	// Arrange
	ec2RoleProvider := &EC2RoleProvider{}

	// Act
	result := ec2RoleProvider.SharesCredentials()

	// Assert
	assert.True(t, result, "SharesCredentials should return true by default")
}

func TestEC2RoleProvider_ExpiresAt_ReturnsSharedCredentialsExpiry(t *testing.T) {
	// Arrange
	now := time.Now()
	expectedExpiry := now.Add(1 * time.Hour)

	sharedProvider := &stubs.InnerProvider{
		ProviderName: "SharedCredentialsProvider",
		Expiry:       expectedExpiry,
	}

	iprProvider := &stubs.InnerProvider{
		ProviderName: IPRProviderName,
		Expiry:       now.Add(2 * time.Hour),
	}

	innerProviders := &EC2InnerProviders{
		IPRProvider:               iprProvider,
		SharedCredentialsProvider: sharedProvider,
	}

	ec2RoleProvider := &EC2RoleProvider{
		InnerProviders:   innerProviders,
		credentialSource: CredentialSourceSSM,
	}

	// Act
	actualExpiry := ec2RoleProvider.ExpiresAt()

	// Assert
	assert.Equal(t, expectedExpiry, actualExpiry)
}

func TestEC2RoleProvider_IsExpired_True(t *testing.T) {
	//Arrange
	sharedProvider := &stubs.InnerProvider{
		ProviderName: "SharedCredentialsProvider",
		RetrieveErr:  errors.New("retrieval failed"),
	}
	iprProvider := &stubs.InnerProvider{
		ProviderName: IPRProviderName,
		RetrieveErr:  errors.New("retrieval failed"),
	}
	innerProviders := &EC2InnerProviders{
		SharedCredentialsProvider: sharedProvider,
		IPRProvider:               iprProvider,
	}

	ec2RoleProvider := &EC2RoleProvider{
		InnerProviders:   innerProviders,
		credentialSource: CredentialSourceSSM,
	}

	ec2RoleProviderEc2 := &EC2RoleProvider{
		InnerProviders:   innerProviders,
		credentialSource: CredentialSourceEC2,
	}

	// Act
	isExpired := ec2RoleProvider.IsExpired()
	isExpiredEc2 := ec2RoleProviderEc2.IsExpired()
	// Assert
	assert.True(t, isExpired)
	assert.True(t, isExpiredEc2)

}

func TestEc2RoleProvider_CredentialSource(t *testing.T) {
	// Arrange
	ec2RoleProvider := &EC2RoleProvider{
		credentialSource: CredentialSourceEC2,
	}

	// Act
	credentialSource := ec2RoleProvider.CredentialSource()

	// Assert
	assert.Equal(t, CredentialSourceEC2, credentialSource)
}
