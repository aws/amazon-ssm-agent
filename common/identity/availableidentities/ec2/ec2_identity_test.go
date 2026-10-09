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
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	ssmtypes "github.com/aws/aws-sdk-go-v2/service/ssm/types"

	"github.com/aws/amazon-ssm-agent/agent/log"
	logmocks "github.com/aws/amazon-ssm-agent/agent/mocks/log"
	authregistermocks "github.com/aws/amazon-ssm-agent/agent/ssm/authregister/mocks"
	ec2detectormocks "github.com/aws/amazon-ssm-agent/common/identity/availableidentities/ec2/ec2detector/mocks"
	"github.com/aws/amazon-ssm-agent/common/identity/availableidentities/ec2/mocks"
	ec2roleprovidermocks "github.com/aws/amazon-ssm-agent/common/identity/credentialproviders/ec2roleprovider/mocks"
	"github.com/aws/amazon-ssm-agent/common/identity/credentialproviders/ssmec2roleprovider"
	endpointmocks "github.com/aws/amazon-ssm-agent/common/identity/endpoint/mocks"
	"github.com/aws/amazon-ssm-agent/common/runtimeconfig"
	runtimeConfigMocks "github.com/aws/amazon-ssm-agent/common/runtimeconfig/mocks"

	"github.com/aws/aws-sdk-go-v2/feature/ec2/imds"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

const (
	testAccessKeyId     = "SomeAccessKeyId"
	testSecretAccessKey = "SomeSecretAccessKey"
	testSessionToken    = "SomeSessionToken"
)

func TestEC2IdentityType_InstanceId(t *testing.T) {
	client := &mocks.IEC2MdsSdkClient{}

	identity := Identity{
		Log:    logmocks.NewMockLog(),
		Client: client,
	}
	val := "SomeId"

	mdInput := imds.GetMetadataInput{Path: ec2InstanceIDResource}

	readCloser := io.NopCloser(strings.NewReader(val))
	mdOutput := imds.GetMetadataOutput{Content: readCloser}

	client.On("GetMetadata", mock.Anything, &mdInput).Return(&mdOutput, nil).Once()

	res, err := identity.InstanceID()
	assert.Equal(t, res, val)
	assert.NoError(t, err)
}

func TestEC2IdentityType_RegionFirstSuccess(t *testing.T) {
	client := &mocks.IEC2MdsSdkClient{}

	identity := Identity{
		Log:    logmocks.NewMockLog(),
		Client: client,
	}
	val := "SomeRegion"
	client.On("GetRegion", mock.Anything, mock.Anything).Return(&imds.GetRegionOutput{Region: val}, nil).Once()

	res, err := identity.Region()
	assert.Equal(t, res, val)
	assert.NoError(t, err)
}

func TestEC2IdentityType_RegionFailDocumentSuccess(t *testing.T) {
	client := &mocks.IEC2MdsSdkClient{}

	identity := Identity{
		Log:    logmocks.NewMockLog(),
		Client: client,
	}
	val := "SomeOtherRegion"
	doc := imds.InstanceIdentityDocument{Region: val}
	document_output := imds.GetInstanceIdentityDocumentOutput{InstanceIdentityDocument: doc}

	//client.On("RegionWithContext", mock.Anything).Return("", fmt.Errorf("SomeError")).Once()
	client.On("GetRegion", mock.Anything, mock.Anything).Return(&imds.GetRegionOutput{}, fmt.Errorf("SomeError")).Once()
	client.On("GetInstanceIdentityDocument", mock.Anything, mock.Anything).Return(&document_output, nil).Once()

	res, err := identity.Region()
	assert.Equal(t, res, val)
	assert.NoError(t, err)
}

func TestEC2IdentityType_AvailabilityZone(t *testing.T) {
	client := &mocks.IEC2MdsSdkClient{}

	identity := Identity{
		Log:    logmocks.NewMockLog(),
		Client: client,
	}
	val := "SomeAZ"

	mdInput := imds.GetMetadataInput{Path: ec2AvailabilityZoneResource}

	readCloser := io.NopCloser(strings.NewReader(val))
	mdOutput := imds.GetMetadataOutput{Content: readCloser}

	client.On("GetMetadata", mock.Anything, &mdInput).Return(&mdOutput, nil).Once()

	res, err := identity.AvailabilityZone()
	assert.Equal(t, res, val)
	assert.NoError(t, err)
}

func TestEC2IdentityType_AvailabilityZoneId(t *testing.T) {
	client := &mocks.IEC2MdsSdkClient{}

	identity := Identity{
		Log:    logmocks.NewMockLog(),
		Client: client,
	}
	val := "SomeAZ"
	mdInput := imds.GetMetadataInput{Path: ec2AvailabilityZoneResourceId}

	readCloser := io.NopCloser(strings.NewReader(val))
	mdOutput := imds.GetMetadataOutput{Content: readCloser}

	client.On("GetMetadata", mock.Anything, &mdInput).Return(&mdOutput, nil).Once()

	res, err := identity.AvailabilityZoneId()
	assert.Equal(t, res, val)
	assert.NoError(t, err)
}

func TestEC2IdentityType_InstanceType(t *testing.T) {
	client := &mocks.IEC2MdsSdkClient{}

	identity := Identity{
		Log:    logmocks.NewMockLog(),
		Client: client,
	}
	val := "SomeInstanceType"
	mdInput := imds.GetMetadataInput{Path: ec2InstanceTypeResource}

	readCloser := io.NopCloser(strings.NewReader(val))
	mdOutput := imds.GetMetadataOutput{Content: readCloser}

	client.On("GetMetadata", mock.Anything, &mdInput).Return(&mdOutput, nil).Once()

	res, err := identity.InstanceType()
	assert.Equal(t, res, val)
	assert.NoError(t, err)
}

func TestEC2IdentityType_Credentials_CompatibilityTestRuntimeConfigPresent_Success(t *testing.T) {
	client := &mocks.IEC2MdsSdkClient{}

	runtimeConfigClientMocks := &runtimeConfigMocks.IIdentityRuntimeConfigClient{}
	runtimeConfigClientMocks.On("GetConfig").Return(runtimeconfig.IdentityRuntimeConfig{}, nil)

	ec2RoleProviderMocks := &ec2roleprovidermocks.IEC2RoleProvider{}
	ec2RoleProviderMocks.On("GetInnerProvider").Return(ec2roleprovidermocks.NewIInnerProvider(t), nil)

	testCreds := aws.Credentials{
		AccessKeyID:     testAccessKeyId,
		SecretAccessKey: testSecretAccessKey,
		SessionToken:    testSessionToken,
		Source:          ssmec2roleprovider.ProviderName,
	}

	ec2RoleProviderMocks.On("Retrieve", mock.Anything).Return(testCreds, nil)

	identity := Identity{
		Log:                 logmocks.NewMockLog(),
		Client:              client,
		credentialsProvider: ec2RoleProviderMocks,
		shareLock:           &sync.RWMutex{},
		runtimeConfigClient: runtimeConfigClientMocks,
	}
	assert.NotNil(t, identity.CredentialsProvider())

	// Shared Profile is null and Shared File is not null
	runtimeConfigClientMocks = &runtimeConfigMocks.IIdentityRuntimeConfigClient{}
	runtimeConfigVal := runtimeconfig.IdentityRuntimeConfig{ShareFile: "test"}
	runtimeConfigClientMocks.On("GetConfig").Return(runtimeConfigVal, nil)
	identity.runtimeConfigClient = runtimeConfigClientMocks
	assert.NotNil(t, identity.CredentialsProvider())

	// Shared Profile is not null and Shared File is null
	runtimeConfigClientMocks = &runtimeConfigMocks.IIdentityRuntimeConfigClient{}
	runtimeConfigVal = runtimeconfig.IdentityRuntimeConfig{ShareProfile: "test"}
	runtimeConfigClientMocks.On("GetConfig").Return(runtimeConfigVal, nil)
	identity.runtimeConfigClient = runtimeConfigClientMocks
	assert.NotNil(t, identity.CredentialsProvider())

	// Shared Profile and Shared File both not null
	runtimeConfigClientMocks = &runtimeConfigMocks.IIdentityRuntimeConfigClient{}
	runtimeConfigVal = runtimeconfig.IdentityRuntimeConfig{ShareProfile: "test", ShareFile: "test"}
	runtimeConfigClientMocks.On("GetConfig").Return(runtimeConfigVal, nil)
	identity.runtimeConfigClient = runtimeConfigClientMocks
	assert.NotNil(t, identity.CredentialsProvider())
}

func TestEC2IdentityType_Credentials_CompatibilityTestRuntimeConfigNotPresent_Success(t *testing.T) {
	client := &mocks.IEC2MdsSdkClient{}

	runtimeConfigClientMocks := &runtimeConfigMocks.IIdentityRuntimeConfigClient{}
	runtimeConfigClientMocks.On("GetConfig").Return(runtimeconfig.IdentityRuntimeConfig{}, fmt.Errorf("no config found"))

	ec2RoleProviderMocks := &ec2roleprovidermocks.IEC2RoleProvider{}
	ec2RoleProviderMocks.On("GetInnerProvider").Return(ec2roleprovidermocks.NewIInnerProvider(t), nil)
	identity := Identity{
		Log:                 logmocks.NewMockLog(),
		Client:              client,
		credentialsProvider: ec2RoleProviderMocks,
		shareLock:           &sync.RWMutex{},
		runtimeConfigClient: runtimeConfigClientMocks,
	}

	testCreds := aws.Credentials{
		AccessKeyID:     testAccessKeyId,
		SecretAccessKey: testSecretAccessKey,
		SessionToken:    testSessionToken,
		Source:          ssmec2roleprovider.ProviderName,
	}

	ec2RoleProviderMocks.On("Retrieve", mock.Anything).Return(testCreds, nil)

	assert.NotNil(t, identity.CredentialsProvider())
	ec2RoleProviderMocks.AssertNumberOfCalls(t, "GetInnerProvider", 0)
}

func TestEC2IdentityType_IsIdentityEnvironment(t *testing.T) {
	ec2DetectorMocks := &ec2detectormocks.Ec2Detector{}
	client := &mocks.IEC2MdsSdkClient{}
	identity := Identity{
		Log:         logmocks.NewMockLog(),
		Client:      client,
		ec2Detector: ec2DetectorMocks,
	}

	mdInput := imds.GetMetadataInput{Path: ec2InstanceIDResource}

	readCloser := io.NopCloser(strings.NewReader(string("")))
	mdOutput := imds.GetMetadataOutput{Content: readCloser}

	client.On("GetMetadata", mock.Anything, &mdInput).Return(&mdOutput, fmt.Errorf("SomeError")).Once()
	assert.False(t, identity.IsIdentityEnvironment())

	client.On("GetMetadata", mock.Anything, &mdInput).Return(&mdOutput, nil).Once()
	//client.On("RegionWithContext", mock.Anything).Return("SomeRegion", nil).Once()
	assert.True(t, identity.IsIdentityEnvironment())

}

func TestEC2IdentityType_IdentityType(t *testing.T) {
	identity := Identity{
		Log: logmocks.NewMockLog(),
	}

	res := identity.IdentityType()
	assert.Equal(t, res, IdentityType)
}

func TestEC2IdentityType_SourceType(t *testing.T) {
	identity := Identity{}

	res := identity.SourceType()
	assert.Equal(t, SourceType, res)
}

func TestGetInstanceInfo_ReturnsError_WhenErrorGettingInstanceId(t *testing.T) {
	// Arrange
	client := &mocks.IEC2MdsSdkClient{}

	identity := &Identity{
		Log:    logmocks.NewMockLog(),
		Client: client,
	}

	mdInput := imds.GetMetadataInput{Path: ec2InstanceIDResource}

	instanceId := io.NopCloser(strings.NewReader(string("SomeId")))
	mdOutput := imds.GetMetadataOutput{Content: instanceId}
	client.On("GetMetadata", mock.Anything, &mdInput).Return(&mdOutput, fmt.Errorf("failed to get instanceId")).Once()

	// Act
	result, err := getInstanceInfo(context.Background(), identity)

	// Assert
	assert.Error(t, err)
	assert.Nil(t, result)
}

func TestGetInstanceInfo_ReturnsError_WhenErrorGettingRegion(t *testing.T) {
	// Arrange
	client := &mocks.IEC2MdsSdkClient{}

	identity := &Identity{
		Log:    logmocks.NewMockLog(),
		Client: client,
	}

	doc := imds.InstanceIdentityDocument{}
	document_output := imds.GetInstanceIdentityDocumentOutput{InstanceIdentityDocument: doc}
	md_input := imds.GetMetadataInput{ec2InstanceIDResource}

	instanceId := "SomeId"

	readCloser := io.NopCloser(strings.NewReader(instanceId))
	md_output := imds.GetMetadataOutput{Content: readCloser}

	client.On("GetMetadata", mock.Anything, &md_input).Return(&md_output, nil).Once()
	client.On("GetRegion", mock.Anything, mock.Anything).Return(&imds.GetRegionOutput{}, fmt.Errorf("failed to get region")).Once()
	client.On("GetInstanceIdentityDocument", mock.Anything, mock.Anything).
		Return(&document_output, fmt.Errorf("failed to get instance identity document")).
		Once()

	// Act
	result, err := getInstanceInfo(context.Background(), identity)

	// Assert
	assert.Error(t, err)
	assert.Nil(t, result)
}

func TestGetInstanceInfo_ReturnsInstanceInfo(t *testing.T) {
	// Arrange
	client := &mocks.IEC2MdsSdkClient{}

	identity := &Identity{
		Log:    logmocks.NewMockLog(),
		Client: client,
	}

	instanceId := "SomeId"

	mdInput := imds.GetMetadataInput{Path: ec2InstanceIDResource}
	instanceIdCloser := io.NopCloser(strings.NewReader(instanceId))
	mdOutput := imds.GetMetadataOutput{Content: instanceIdCloser}
	client.On("GetMetadata", mock.Anything, &mdInput).Return(&mdOutput, nil).Once()

	regionInput := imds.GetRegionInput{}
	region := "SomeRegion"
	regionOutput := imds.GetRegionOutput{Region: region}
	client.On("GetRegion", mock.Anything, &regionInput).Return(&regionOutput, nil).Once()

	// Act
	result, err := getInstanceInfo(context.Background(), identity)

	// Assert
	assert.NoError(t, err)
	assert.Equal(t, "SomeId", result.InstanceId)
	assert.Equal(t, region, result.Region)
}

func TestEC2Identity_InitEC2RoleProvider_InitsCredentialProvider(t *testing.T) {
	// Arrange
	endpointHelper := &endpointmocks.IEndpointHelper{}
	serviceEndpoint := "ssm.amazon.com"
	endpointHelper.On("GetServiceEndpoint", mock.Anything, mock.Anything).Return(serviceEndpoint)
	instanceInfo := &ssmec2roleprovider.InstanceInfo{
		InstanceId: "SomeInstanceId",
		Region:     "SomeRegion",
	}

	identity := &Identity{
		Log: logmocks.NewMockLog(),
	}

	// Act
	identity.initEc2RoleProvider(endpointHelper, instanceInfo)

	// Assert
	assert.NotNil(t, identity.credentialsProvider)
}

func TestEC2Identity_Register_RegistersEC2InstanceWithSSM_WhenNotRegistered(t *testing.T) {
	// Arrange
	client := &mocks.IEC2MdsSdkClient{}

	region := "SomeRegion"
	instanceId := "i-SomeInstanceId"

	regionInput := imds.GetRegionInput{}
	regionOutput := imds.GetRegionOutput{Region: region}
	client.On("GetRegion", mock.Anything, &regionInput).Return(&regionOutput, nil).Once()

	mdInput := imds.GetMetadataInput{Path: ec2InstanceIDResource}
	instanceIdCloser := io.NopCloser(strings.NewReader(instanceId))
	mdOutput := imds.GetMetadataOutput{Content: instanceIdCloser}
	client.On("GetMetadata", mock.Anything, &mdInput).Return(&mdOutput, nil).Twice()

	authRegisterService := &authregistermocks.IClient{}

	authRegisterService.On("RegisterManagedInstance",
		mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(instanceId, nil)
	getStoredPrivateKey = func(log log.T, manifestFileNamePrefix, vaultKey string) string {
		assert.Equal(t, IdentityType, manifestFileNamePrefix)
		return ""
	}

	getStoredPrivateKeyType = func(log log.T, manifestFileNamePrefix, vaultKey string) string {
		assert.Equal(t, IdentityType, manifestFileNamePrefix)
		return ""
	}

	updateServerInfo = func(instanceID, region, publicKey, privateKey, privateKeyType, manifestFileNamePrefix, vaultKey, provider string) (err error) {
		assert.Equal(t, IdentityType, manifestFileNamePrefix)
		return nil
	}

	identity := &Identity{
		Log:                 logmocks.NewMockLog(),
		Client:              client,
		AuthRegisterService: authRegisterService,
	}

	// Act
	err := identity.Register(context.Background())

	//Assert
	assert.NoError(t, err)
}

func TestEC2Identity_Register_New_WhenAlreadyRegisteredWithOldInstanceId(t *testing.T) {
	// Arrange
	region := "SomeRegion"
	testPrivateKey := "SomePrivateKey"
	testPrivateKeyType := "SomePrivateKeyType"
	liveInstanceId := "i-liveInstanceId"
	client := &mocks.IEC2MdsSdkClient{}

	regionInput := imds.GetRegionInput{}
	regionOutput := imds.GetRegionOutput{Region: region}
	client.On("GetRegion", mock.Anything, &regionInput).Return(&regionOutput, nil).Once()

	mdInput := imds.GetMetadataInput{Path: ec2InstanceIDResource}
	instanceIdCloser := io.NopCloser(strings.NewReader(liveInstanceId))
	mdOutput := imds.GetMetadataOutput{Content: instanceIdCloser}
	client.On("GetMetadata", mock.Anything, &mdInput).Return(&mdOutput, nil).Twice()

	authRegisterService := &authregistermocks.IClient{}
	// One in Register() function and the other call in loadRegistrationInfo function
	authRegisterService.On("RegisterManagedInstance",
		mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(liveInstanceId, nil)
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
		return liveInstanceId
	}

	// Fully-confirmed registration: storedRegion is non-empty, so loadRegistrationInfo
	// treats the registration as complete and Register returns early without calling
	// updateServerInfo or RegisterManagedInstance.
	getStoredRegion = func(log log.T, manifestFileNamePrefix, vaultKey string) string {
		assert.Equal(t, IdentityType, manifestFileNamePrefix)
		return region
	}

	updateServerInfo = func(instanceID, region, publicKey, privateKey, privateKeyType, manifestFileNamePrefix, vaultKey, provider string) (err error) {
		assert.Equal(t, privateKeyType, testPrivateKeyType)
		assert.Equal(t, privateKey, testPrivateKey)
		assert.Equal(t, IdentityType, manifestFileNamePrefix)
		return nil
	}

	identity := &Identity{
		Log:                 logmocks.NewMockLog(),
		Client:              client,
		AuthRegisterService: authRegisterService,
	}

	// Act
	err := identity.Register(context.Background())

	// Assert
	assert.NoError(t, err)
}

func TestEC2Identity_ReRegister_InfoPublicKey_NotBlank(t *testing.T) {
	// Arrange
	region := "SomeRegion"
	testPrivateKey := "SomePrivateKey"
	testPrivateKeyType := "SomePrivateKeyType"
	testPublicKey := "SomePublicKey"
	liveInstanceId := "i-liveInstanceId"
	client := &mocks.IEC2MdsSdkClient{}

	regionInput := imds.GetRegionInput{}
	regionOutput := imds.GetRegionOutput{Region: region}
	client.On("GetRegion", mock.Anything, &regionInput).Return(&regionOutput, nil).Once()

	mdInput := imds.GetMetadataInput{Path: ec2InstanceIDResource}
	instanceIdCloser := io.NopCloser(strings.NewReader(liveInstanceId))
	mdOutput := imds.GetMetadataOutput{Content: instanceIdCloser}

	client.On("GetMetadata", mock.Anything, &mdInput).Return(&mdOutput, nil).Twice()

	authRegisterService := &authregistermocks.IClient{}
	// One in Register() function and the other call in loadRegistrationInfo function
	authRegisterService.On("RegisterManagedInstance",
		mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(liveInstanceId, nil)
	getStoredPrivateKey = func(log log.T, manifestFileNamePrefix, vaultKey string) string {
		assert.Equal(t, IdentityType, manifestFileNamePrefix)
		return testPrivateKey
	}

	getStoredPrivateKeyType = func(log log.T, manifestFileNamePrefix, vaultKey string) string {
		assert.Equal(t, IdentityType, manifestFileNamePrefix)
		return testPrivateKeyType
	}

	getStoredPublicKey = func(log log.T, manifestFileNamePrefix, vaultKey string) string {
		assert.Equal(t, IdentityType, manifestFileNamePrefix)
		return testPublicKey
	}

	// Stored instance id matches the live one so loadRegistrationInfo takes the
	// instanceIdMatch=true branch and keys are preserved.
	getStoredInstanceId = func(log log.T, manifestFileNamePrefix, vaultKey string) string {
		assert.Equal(t, IdentityType, manifestFileNamePrefix)
		return liveInstanceId
	}

	// Interrupted registration: stored region is empty so registrationConfirmed=false,
	// InstanceId is cleared and the existing keypair is reused on re-registration.
	getStoredRegion = func(log log.T, manifestFileNamePrefix, vaultKey string) string {
		assert.Equal(t, IdentityType, manifestFileNamePrefix)
		return ""
	}

	updateServerInfo = func(instanceID, region, publicKey, privateKey, privateKeyType, manifestFileNamePrefix, vaultKey, provider string) (err error) {
		assert.Equal(t, privateKeyType, testPrivateKeyType)
		assert.Equal(t, privateKey, testPrivateKey)
		assert.Equal(t, publicKey, testPublicKey)
		assert.Equal(t, IdentityType, manifestFileNamePrefix)
		return nil
	}

	identity := &Identity{
		Log:                 logmocks.NewMockLog(),
		Client:              client,
		AuthRegisterService: authRegisterService,
	}

	// Act
	err := identity.Register(context.Background())

	// Assert
	assert.NoError(t, err)
}

func TestEC2Identity_ReRegister_InfoPublicKey_Blank(t *testing.T) {
	// Arrange
	region := "SomeRegion"
	testPrivateKey := "SomePrivateKey"
	testPrivateKeyType := "SomePrivateKeyType"
	liveInstanceId := "i-liveInstanceId"
	client := &mocks.IEC2MdsSdkClient{}

	regionInput := imds.GetRegionInput{}
	regionOutput := imds.GetRegionOutput{Region: region}
	client.On("GetRegion", mock.Anything, &regionInput).Return(&regionOutput, nil).Once()

	mdInput := imds.GetMetadataInput{Path: ec2InstanceIDResource}
	instanceIdCloser := io.NopCloser(strings.NewReader(liveInstanceId))
	mdOutput := imds.GetMetadataOutput{Content: instanceIdCloser}
	client.On("GetMetadata", mock.Anything, &mdInput).Return(&mdOutput, nil).Twice()

	authRegisterService := &authregistermocks.IClient{}
	// One in Register() function and the other call in loadRegistrationInfo function
	authRegisterService.On("RegisterManagedInstance",
		mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(liveInstanceId, nil)
	getStoredPrivateKey = func(log log.T, manifestFileNamePrefix, vaultKey string) string {
		assert.Equal(t, IdentityType, manifestFileNamePrefix)
		return testPrivateKey
	}

	getStoredPrivateKeyType = func(log log.T, manifestFileNamePrefix, vaultKey string) string {
		assert.Equal(t, IdentityType, manifestFileNamePrefix)
		return testPrivateKeyType
	}

	getStoredPublicKey = func(log log.T, manifestFileNamePrefix, vaultKey string) string {
		assert.Equal(t, IdentityType, manifestFileNamePrefix)
		return ""
	}

	getStoredInstanceId = func(log log.T, manifestFileNamePrefix, vaultKey string) string {
		assert.Equal(t, IdentityType, manifestFileNamePrefix)
		return ""
	}

	updateServerInfo = func(instanceID, region, publicKey, privateKey, privateKeyType, manifestFileNamePrefix, vaultKey, provider string) (err error) {
		assert.Equal(t, IdentityType, manifestFileNamePrefix)
		return nil
	}

	identity := &Identity{
		Log:                 logmocks.NewMockLog(),
		Client:              client,
		AuthRegisterService: authRegisterService,
	}

	// Act
	err := identity.Register(context.Background())

	// Assert
	assert.NoError(t, err)
}

func TestEC2Identity_Register_ReturnsRegistrationInfo_WhenAlreadyRegistered(t *testing.T) {
	// Arrange
	testPrivateKey := "SomePrivateKey"
	testPrivateKeyType := "SomePrivateKeyType"
	testInstanceId := "i-SomeInstanceId"
	testRegion := "SomeRegion"
	client := &mocks.IEC2MdsSdkClient{}

	regionInput := imds.GetRegionInput{}
	regionOutput := imds.GetRegionOutput{Region: testRegion}
	client.On("GetRegion", mock.Anything, &regionInput).Return(&regionOutput, nil).Once()

	mdInput := imds.GetMetadataInput{Path: ec2InstanceIDResource}
	instanceIdCloser := io.NopCloser(strings.NewReader(testInstanceId))
	mdOutput := imds.GetMetadataOutput{Content: instanceIdCloser}
	client.On("GetMetadata", mock.Anything, &mdInput).Return(&mdOutput, nil).Once()

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

	// Fully-confirmed registration: non-empty stored region keeps
	// registrationInfo.InstanceId populated so Register() returns early without
	// calling updateServerInfo or RegisterManagedInstance.
	getStoredRegion = func(log log.T, manifestFileNamePrefix, vaultKey string) string {
		assert.Equal(t, IdentityType, manifestFileNamePrefix)
		return testRegion
	}

	identity := &Identity{
		Log:    logmocks.NewMockLog(),
		Client: client,
	}

	// Act
	err := identity.Register(context.Background())

	// Assert
	assert.NoError(t, err)
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
	client := &mocks.IEC2MdsSdkClient{}

	regionInput := imds.GetRegionInput{}
	regionOutput := imds.GetRegionOutput{Region: testRegion}
	client.On("GetRegion", mock.Anything, &regionInput).Return(&regionOutput, nil).Once()

	mdInput := imds.GetMetadataInput{Path: ec2InstanceIDResource}
	instanceIdCloser := io.NopCloser(strings.NewReader(testInstanceId))
	mdOutput := imds.GetMetadataOutput{Content: instanceIdCloser}
	client.On("GetMetadata", mock.Anything, &mdInput).Return(&mdOutput, nil).Times(3)

	authRegisterService := &authregistermocks.IClient{}

	//authRegisterService.On("RegisterManagedInstance",
	//	mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return("", &awsTestError{errCode: ssmtypes.InstanceAlreadyRegisteredException})

	IARException := &ssmtypes.InstanceAlreadyRegisteredException{
		Message: aws.String("Instance already registered"),
	}
	authRegisterService.On("RegisterManagedInstance", mock.Anything, mock.Anything,
		mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return("", IARException)
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

	updateServerInfo = func(instanceID, region, publicKey, privateKey, privateKeyType, manifestFileNamePrefix, vaultKey, provider string) (err error) {
		assert.Equal(t, IdentityType, manifestFileNamePrefix)
		return nil
	}

	identity := &Identity{
		Log:                 logmocks.NewMockLog(),
		Client:              client,
		AuthRegisterService: authRegisterService,
	}

	// Act
	err := identity.Register(context.Background())

	// Assert
	assert.NoError(t, err)
}

func TestInitIMDSClient_DoesNotFallBackToIMDSv1(t *testing.T) {
	imdsV1Requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPut {
			w.WriteHeader(http.StatusForbidden)
			return
		}
		if r.Header.Get("X-aws-ec2-metadata-token") == "" {
			imdsV1Requests++
		}
		w.Write([]byte("i-123"))
	}))
	defer server.Close()
	t.Setenv("AWS_EC2_METADATA_SERVICE_ENDPOINT", server.URL)
	t.Setenv("AWS_EC2_METADATA_DISABLED", "")

	identity := &Identity{}
	identity.initIMDSClient(aws.Config{})
	_, err := identity.Client.GetMetadata(context.Background(), &imds.GetMetadataInput{Path: "instance-id"})

	assert.Error(t, err)
	assert.Zero(t, imdsV1Requests)
}
