// Copyright 2020 Amazon.com, Inc. or its affiliates. All Rights Reserved.
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
package ec2

import (
	"fmt"
	"sync"
	"testing"

	"github.com/aws/amazon-ssm-agent/common/runtimeconfig"
	runtimeConfigMocks "github.com/aws/amazon-ssm-agent/common/runtimeconfig/mocks"

	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/amazon-ssm-agent/agent/ssm/authregister"
	authregistermocks "github.com/aws/amazon-ssm-agent/agent/ssm/authregister/mocks"
	"github.com/aws/amazon-ssm-agent/common/identity/availableidentities/ec2/mocks"
	"github.com/aws/amazon-ssm-agent/common/identity/availableidentities/ec2/stubs"
	ec2roleprovidermocks "github.com/aws/amazon-ssm-agent/common/identity/credentialproviders/ec2roleprovider/mocks"
	"github.com/aws/amazon-ssm-agent/common/identity/credentialproviders/ssmec2roleprovider"
	endpointmocks "github.com/aws/amazon-ssm-agent/common/identity/endpoint/mocks"

	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/ec2metadata"
	"github.com/aws/aws-sdk-go/service/ssm"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestEC2IdentityType_InstanceId(t *testing.T) {
	client := &mocks.IEC2MdsSdkClientMock{}

	identity := Identity{
		Log:    log.NewMockLog(),
		Client: client,
	}
	val := "SomeId"
	client.On("GetMetadata", ec2InstanceIDResource).Return(val, nil).Once()

	res, err := identity.InstanceID()
	assert.Equal(t, res, val)
	assert.NoError(t, err)
}

func TestEC2IdentityType_RegionFirstSuccess(t *testing.T) {
	client := &mocks.IEC2MdsSdkClientMock{}

	identity := Identity{
		Log:    log.NewMockLog(),
		Client: client,
	}
	val := "SomeRegion"
	client.On("Region").Return(val, nil).Once()

	res, err := identity.Region()
	assert.Equal(t, res, val)
	assert.NoError(t, err)
}

func TestEC2IdentityType_RegionFailDocumentSuccess(t *testing.T) {
	client := &mocks.IEC2MdsSdkClientMock{}

	identity := Identity{
		Log:    log.NewMockLog(),
		Client: client,
	}
	val := "SomeOtherRegion"
	document := ec2metadata.EC2InstanceIdentityDocument{Region: val}

	client.On("Region").Return("", fmt.Errorf("SomeError")).Once()
	client.On("GetInstanceIdentityDocument").Return(document, nil).Once()

	res, err := identity.Region()
	assert.Equal(t, res, val)
	assert.NoError(t, err)
}

func TestEC2IdentityType_AvailabilityZone(t *testing.T) {
	client := &mocks.IEC2MdsSdkClientMock{}

	identity := Identity{
		Log:    log.NewMockLog(),
		Client: client,
	}
	val := "SomeAZ"
	client.On("GetMetadata", ec2AvailabilityZoneResource).Return(val, nil).Once()

	res, err := identity.AvailabilityZone()
	assert.Equal(t, res, val)
	assert.NoError(t, err)
}

func TestEC2IdentityType_AvailabilityZoneId(t *testing.T) {
	client := &mocks.IEC2MdsSdkClient{}

	identity := Identity{
		Log:    log.NewMockLog(),
		Client: client,
	}
	val := "SomeAZ"
	client.On("GetMetadata", ec2AvailabilityZoneResourceId).Return(val, nil).Once()

	res, err := identity.AvailabilityZoneId()
	assert.Equal(t, res, val)
	assert.NoError(t, err)
}

func TestEC2IdentityType_InstanceType(t *testing.T) {
	client := &mocks.IEC2MdsSdkClientMock{}

	identity := Identity{
		Log:    log.NewMockLog(),
		Client: client,
	}
	val := "SomeInstanceType"
	client.On("GetMetadata", ec2InstanceTypeResource).Return(val, nil).Once()

	res, err := identity.InstanceType()
	assert.Equal(t, res, val)
	assert.NoError(t, err)
}

func TestEC2IdentityType_Credentials_CompatibilityTestRuntimeConfigPresent_Success(t *testing.T) {
	client := &mocks.IEC2MdsSdkClientMock{}
	client.On("GetMetadata", ec2InstanceIDResource).Return("SomeRegion", nil).Once()
	client.On("GetMetadata", ec2InstanceIDResource).Return("SomeInstanceId", nil).Once()
	client.On("GetMetadata", ec2ServiceDomainResource).Return("SomeServiceDomain", nil).Once()
	client.On("Region").Return("SomeRegion", nil).Once()

	runtimeConfigClientMocks := &runtimeConfigMocks.IIdentityRuntimeConfigClient{}
	runtimeConfigClientMocks.On("GetConfig").Return(runtimeconfig.IdentityRuntimeConfig{}, nil)

	ec2RoleProviderMocks := &ec2roleprovidermocks.IEC2RoleProvider{}
	ec2RoleProviderMocks.On("GetInnerProvider").Return(ec2roleprovidermocks.NewIInnerProvider(t), nil)

	identity := Identity{
		Log:                 log.NewMockLog(),
		Client:              client,
		credentialsProvider: ec2RoleProviderMocks,
		shareLock:           &sync.RWMutex{},
		runtimeConfigClient: runtimeConfigClientMocks,
	}
	assert.NotNil(t, identity.Credentials())
	ec2RoleProviderMocks.AssertNumberOfCalls(t, "GetInnerProvider", 1)

	// Shared File is null and Shared File is not null
	runtimeConfigClientMocks = &runtimeConfigMocks.IIdentityRuntimeConfigClient{}
	runtimeConfigVal := runtimeconfig.IdentityRuntimeConfig{ShareFile: "test"}
	runtimeConfigClientMocks.On("GetConfig").Return(runtimeConfigVal, nil)
	identity.runtimeConfigClient = runtimeConfigClientMocks
	assert.NotNil(t, identity.Credentials())
	ec2RoleProviderMocks.AssertNumberOfCalls(t, "GetInnerProvider", 2)

	// Shared Profile is not null and Shared File is null
	runtimeConfigClientMocks = &runtimeConfigMocks.IIdentityRuntimeConfigClient{}
	runtimeConfigVal = runtimeconfig.IdentityRuntimeConfig{ShareProfile: "test"}
	runtimeConfigClientMocks.On("GetConfig").Return(runtimeConfigVal, nil)
	identity.runtimeConfigClient = runtimeConfigClientMocks
	assert.NotNil(t, identity.Credentials())
	ec2RoleProviderMocks.AssertNumberOfCalls(t, "GetInnerProvider", 3)

	// Shared Profile and Shared File both not null
	runtimeConfigClientMocks = &runtimeConfigMocks.IIdentityRuntimeConfigClient{}
	runtimeConfigVal = runtimeconfig.IdentityRuntimeConfig{ShareProfile: "test", ShareFile: "test"}
	runtimeConfigClientMocks.On("GetConfig").Return(runtimeConfigVal, nil)
	identity.runtimeConfigClient = runtimeConfigClientMocks
	assert.NotNil(t, identity.Credentials())
	ec2RoleProviderMocks.AssertNumberOfCalls(t, "GetInnerProvider", 3)
}

func TestEC2IdentityType_Credentials_CompatibilityTestRuntimeConfigNotPresent_Success(t *testing.T) {
	client := &mocks.IEC2MdsSdkClientMock{}
	client.On("GetMetadata", ec2InstanceIDResource).Return("SomeRegion", nil).Once()
	client.On("GetMetadata", ec2InstanceIDResource).Return("SomeInstanceId", nil).Once()
	client.On("GetMetadata", ec2ServiceDomainResource).Return("SomeServiceDomain", nil).Once()
	client.On("Region").Return("SomeRegion", nil).Once()

	runtimeConfigClientMocks := &runtimeConfigMocks.IIdentityRuntimeConfigClient{}
	runtimeConfigClientMocks.On("GetConfig").Return(runtimeconfig.IdentityRuntimeConfig{}, fmt.Errorf("no config found"))

	ec2RoleProviderMocks := &ec2roleprovidermocks.IEC2RoleProvider{}
	ec2RoleProviderMocks.On("GetInnerProvider").Return(ec2roleprovidermocks.NewIInnerProvider(t), nil)
	identity := Identity{
		Log:                 log.NewMockLog(),
		Client:              client,
		credentialsProvider: ec2RoleProviderMocks,
		shareLock:           &sync.RWMutex{},
		runtimeConfigClient: runtimeConfigClientMocks,
	}
	assert.NotNil(t, identity.Credentials())
	ec2RoleProviderMocks.AssertNumberOfCalls(t, "GetInnerProvider", 0)
}

func TestEC2IdentityType_IsIdentityEnvironment(t *testing.T) {
	client := &mocks.IEC2MdsSdkClientMock{}

	identity := Identity{
		Log:    log.NewMockLog(),
		Client: client,
	}

	// Success
	client.On("GetMetadata", ec2InstanceIDResource).Return("", fmt.Errorf("SomeError")).Once()
	assert.False(t, identity.IsIdentityEnvironment())

	client.On("GetMetadata", ec2InstanceIDResource).Return("SomeInstanceId", nil).Once()
	client.On("Region").Return("SomeRegion", nil).Once()
	client.On("GetMetadata", ec2ServiceDomainResource).Return("SomeServiceDomain", nil).Once()
	assert.True(t, identity.IsIdentityEnvironment())

}

func TestEC2IdentityType_IdentityType(t *testing.T) {
	identity := Identity{
		Log: log.NewMockLog(),
	}

	res := identity.IdentityType()
	assert.Equal(t, res, IdentityType)
}

func TestEC2Identity_initSharedCreds_InitsSharedCredentials_WhenSharedProviderSuccessfullyCreated(t *testing.T) {
	// Arrange
	newSharedCredentialsProvider = func(log log.T) (credentials.Provider, error) {
		return &stubs.ProviderStub{
			ProviderName: stubs.SharedProviderName,
		}, nil
	}

	identity := &Identity{
		Log: log.NewMockLog(),
	}

	// Act
	identity.initSharedCreds()
	resultingCreds, _ := identity.credentials.Get()

	// Assert
	assert.Equal(t, stubs.SharedProviderName, resultingCreds.ProviderName)
}

func TestEC2Identity_initSharedCreds_InitsNonSharedCredentials_WhenSharedProviderFailsInit(t *testing.T) {
	// Arrange
	newSharedCredentialsProvider = func(log log.T) (credentials.Provider, error) {
		return nil, fmt.Errorf("failed to initialize SharedCredentialProvider")
	}

	identity := &Identity{
		Log: log.NewMockLog(),
		credentialsProvider: &stubs.ProviderStub{
			ProviderName: stubs.NonSharedProviderName,
		},
	}

	// Act
	identity.initSharedCreds()
	resultingCreds, _ := identity.credentials.Get()

	// Assert
	assert.Equal(t, stubs.NonSharedProviderName, resultingCreds.ProviderName)
}

func TestGetInstanceInfo_ReturnsError_WhenErrorGettingInstanceId(t *testing.T) {
	// Arrange
	client := &mocks.IEC2MdsSdkClientMock{}

	identity := &Identity{
		Log:    log.NewMockLog(),
		Client: client,
	}

	instanceId := "SomeId"
	client.On("GetMetadata", ec2InstanceIDResource).Return(instanceId, fmt.Errorf("failed to get instanceId")).Once()

	// Act
	result, err := getInstanceInfo(identity)

	// Assert
	assert.Error(t, err)
	assert.Nil(t, result)
}

func TestGetInstanceInfo_ReturnsError_WhenErrorGettingRegion(t *testing.T) {
	// Arrange
	client := &mocks.IEC2MdsSdkClientMock{}

	identity := &Identity{
		Log:    log.NewMockLog(),
		Client: client,
	}

	instanceId := "SomeId"
	client.On("GetMetadata", ec2InstanceIDResource).Return(instanceId, nil).Once()
	client.On("Region").Return("", fmt.Errorf("failed to get region")).Once()
	client.On("GetInstanceIdentityDocument").
		Return(ec2metadata.EC2InstanceIdentityDocument{}, fmt.Errorf("failed to get instance identity document")).
		Once()

	// Act
	result, err := getInstanceInfo(identity)

	// Assert
	assert.Error(t, err)
	assert.Nil(t, result)
}

func TestGetInstanceInfo_ReturnsInstanceInfo(t *testing.T) {
	// Arrange
	client := &mocks.IEC2MdsSdkClientMock{}

	identity := &Identity{
		Log:    log.NewMockLog(),
		Client: client,
	}

	instanceId := "SomeId"
	region := "SomeRegion"
	client.On("GetMetadata", ec2InstanceIDResource).Return(instanceId, nil).Once()
	client.On("Region").Return(region, nil).Once()

	// Act
	result, err := getInstanceInfo(identity)

	// Assert
	assert.NoError(t, err)
	assert.Equal(t, instanceId, result.InstanceId)
	assert.Equal(t, region, result.Region)
}

func TestEC2Identity_InitEC2RoleProvider_InitsCredentialProvider(t *testing.T) {
	// Arrange
	endpointHelper := &endpointmocks.IEndpointHelper{}
	serviceEndpoint := "ssm.amazon.com"
	endpointHelper.On("GetServiceEndpoint", mock.Anything, mock.Anything).Return(serviceEndpoint)
	registrationReadyChan := make(chan *authregister.RegistrationInfo, 1)
	instanceInfo := &ssmec2roleprovider.InstanceInfo{
		InstanceId: "SomeInstanceId",
		Region:     "SomeRegion",
	}

	identity := &Identity{
		Log:                   log.NewMockLog(),
		registrationReadyChan: registrationReadyChan,
	}

	// Act
	identity.initEc2RoleProvider(endpointHelper, instanceInfo)

	// Assert
	assert.NotNil(t, identity.credentialsProvider)
}

func TestEC2Identity_Register_RegistersEC2InstanceWithSSM_WhenNotRegistered(t *testing.T) {
	// Arrange
	client := &mocks.IEC2MdsSdkClientMock{}
	region := "SomeRegion"
	instanceId := "i-SomeInstanceId"
	client.On("Region").Return(region, nil).Once()
	authRegisterService := &authregistermocks.IClient{}
	client.On("GetMetadata", ec2InstanceIDResource).Return(instanceId, nil).Once()
	authRegisterService.On("RegisterManagedInstance",
		mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(instanceId, nil)
	getStoredPrivateKey = func(log log.T, manifestFileNamePrefix, vaultKey string) string {
		assert.Equal(t, IdentityType, manifestFileNamePrefix)
		return ""
	}

	getStoredPrivateKeyType = func(log log.T, manifestFileNamePrefix, vaultKey string) string {
		assert.Equal(t, IdentityType, manifestFileNamePrefix)
		return ""
	}

	updateServerInfo = func(instanceID, region, privateKey, privateKeyType, manifestFileNamePrefix, vaultKey string) (err error) {
		assert.Equal(t, IdentityType, manifestFileNamePrefix)
		return nil
	}

	identity := &Identity{
		Log:                   log.NewMockLog(),
		Client:                client,
		authRegisterService:   authRegisterService,
		registrationReadyChan: make(chan *authregister.RegistrationInfo, 1),
	}

	// Act
	err := identity.Register()

	//Assert
	assert.NoError(t, err)
	registrationInfo := <-identity.registrationReadyChan
	assert.NotNil(t, registrationInfo)
}

func TestEC2Identity_Register_ReturnsRegistrationInfo_WhenAlreadyRegistered(t *testing.T) {
	// Arrange
	testPrivateKey := "SomePrivateKey"
	testPrivateKeyType := "SomePrivateKeyType"
	testInstanceId := "i-SomeInstanceId"
	testRegion := "SomeRegion"
	client := &mocks.IEC2MdsSdkClientMock{}
	client.On("Region").Return(testRegion, nil).Once()
	client.On("GetMetadata", ec2InstanceIDResource).Return(testInstanceId, nil).Once()
	getStoredPrivateKey = func(log log.T, manifestFileNamePrefix, vaultKey string) string {
		assert.Equal(t, IdentityType, manifestFileNamePrefix)
		return testPrivateKey
	}

	getStoredPrivateKeyType = func(log log.T, manifestFileNamePrefix, vaultKey string) string {
		assert.Equal(t, IdentityType, manifestFileNamePrefix)
		return testPrivateKeyType
	}

	getStoredInstanceId = func(log log.T, manifestFileNamePrefix, vaultKey string) string {
		assert.Equal(t, IdentityType, manifestFileNamePrefix)
		return testInstanceId
	}

	identity := &Identity{
		Log:                   log.NewMockLog(),
		registrationReadyChan: make(chan *authregister.RegistrationInfo, 1),
	}

	// Act
	err := identity.Register()

	// Assert
	assert.NoError(t, err)
	registrationInfo := <-identity.registrationReadyChan
	assert.Equal(t, testPrivateKey, registrationInfo.PrivateKey)
	assert.Equal(t, testPrivateKeyType, registrationInfo.KeyType)
}

// Mock aws error struct
type awsTestError struct {
	errCode string
}

func (a awsTestError) Error() string   { return "" }
func (a awsTestError) Message() string { return "" }
func (a awsTestError) OrigErr() error  { return fmt.Errorf("SomeErr") }
func (a awsTestError) Code() string    { return a.errCode }

func TestEC2Identity_Register_ReturnsNil_WhenInstanceAlreadyRegistered(t *testing.T) {
	// Arrange
	testPrivateKey := "SomePrivateKey"
	testPrivateKeyType := "SomePrivateKeyType"
	testInstanceId := ""
	testRegion := "SomeRegion"
	client := &mocks.IEC2MdsSdkClientMock{}
	client.On("Region").Return(testRegion, nil).Once()
	authRegisterService := &authregistermocks.IClient{}
	client.On("GetMetadata", ec2InstanceIDResource).Return(testInstanceId, nil).Once()

	authRegisterService.On("RegisterManagedInstance",
		mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return("", &awsTestError{errCode: ssm.ErrCodeInstanceAlreadyRegistered})
	getStoredPrivateKey = func(log log.T, manifestFileNamePrefix, vaultKey string) string {
		assert.Equal(t, IdentityType, manifestFileNamePrefix)
		return testPrivateKey
	}

	getStoredPrivateKeyType = func(log log.T, manifestFileNamePrefix, vaultKey string) string {
		assert.Equal(t, IdentityType, manifestFileNamePrefix)
		return testPrivateKeyType
	}

	getStoredInstanceId = func(log log.T, manifestFileNamePrefix, vaultKey string) string {
		assert.Equal(t, IdentityType, manifestFileNamePrefix)
		return testInstanceId
	}

	updateServerInfo = func(instanceID, region, privateKey, privateKeyType, manifestFileNamePrefix, vaultKey string) (err error) {
		assert.Equal(t, IdentityType, manifestFileNamePrefix)
		return nil
	}

	identity := &Identity{
		Log:                   log.NewMockLog(),
		Client:                client,
		authRegisterService:   authRegisterService,
		registrationReadyChan: make(chan *authregister.RegistrationInfo, 1),
	}

	// Act
	err := identity.Register()

	// Assert
	assert.NoError(t, err)
	registrationInfo, ok := <-identity.registrationReadyChan
	assert.Nil(t, registrationInfo)
	assert.False(t, ok)
}
