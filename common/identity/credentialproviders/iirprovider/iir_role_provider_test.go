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

package iirprovider

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/aws/amazon-ssm-agent/agent/appconfig"
	"github.com/aws/amazon-ssm-agent/agent/mocks/log"
	iirprovidermocks "github.com/aws/amazon-ssm-agent/common/identity/credentialproviders/iirprovider/mocks"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/ec2/imds"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

const (
	testAccessKeyId     = "SomeAccessKeyId"
	testSecretAccessKey = "SomeSecretAccessKey"
	testSessionToken    = "SomeSessionToken"
)

func TestRetrieve_ReturnsCredentials(t *testing.T) {
	logger := log.NewMockLog()
	ssmConfig, _ := appconfig.Config(true)

	respCreds := Ec2RoleCreds{
		AccessKeyID:     testAccessKeyId,
		SecretAccessKey: testSecretAccessKey,
		Token:           testSessionToken,
		Expiration:      time.Now().Add(time.Hour * 6),
		Code:            "Success",
	}

	mdInput := imds.GetMetadataInput{Path: iirCredentialsPath}

	respJSONBytes, _ := json.Marshal(respCreds)

	readCloser := io.NopCloser(strings.NewReader(string(respJSONBytes)))
	mdOutput := imds.GetMetadataOutput{Content: readCloser}

	mockIMDSClient := &iirprovidermocks.IEC2MdsSdkClient{}
	mockIMDSClient.On("GetMetadata", mock.Anything, &mdInput).Return(&mdOutput, nil)

	roleProvider := &IIRRoleProvider{
		IMDSClient: mockIMDSClient,
		Config:     &ssmConfig,
		Log:        logger,
	}

	result, err := roleProvider.Retrieve(context.TODO())

	expectedResult := aws.Credentials{
		AccessKeyID:     respCreds.AccessKeyID,
		SecretAccessKey: respCreds.SecretAccessKey,
		SessionToken:    respCreds.Token,
		Source:          ProviderName,
		CanExpire:       true,
		Expires:         result.Expires,
	}

	assert.NotNil(t, result)
	assert.Nil(t, err)
	assert.Equal(t, expectedResult, result)
}

func TestEmptyCredentials_StructureValidation(t *testing.T) {
	// Test that EmptyCredentials returns a credentials.Value with expected properties
	emptyCredentials := EmptyCredentials()

	// Verify provider name is set correctly
	assert.Equal(t, ProviderName, emptyCredentials.Source, "Provider name should match")

	// Verify other credential fields are empty
	assert.Empty(t, emptyCredentials.AccessKeyID, "AccessKeyID should be empty")
	assert.Empty(t, emptyCredentials.SecretAccessKey, "SecretAccessKey should be empty")
	assert.Empty(t, emptyCredentials.SessionToken, "SessionToken should be empty")
}

func TestRetrieve_IMDSClientFailure(t *testing.T) {
	// Setup
	logger := log.NewMockLog()
	ssmConfig, _ := appconfig.Config(true)

	// Mock IMDS client to return an error
	mockIMDSClient := &iirprovidermocks.IEC2MdsSdkClient{}

	mdInput := imds.GetMetadataInput{Path: iirCredentialsPath}

	readCloser := io.NopCloser(strings.NewReader(string("")))
	mdOutput := imds.GetMetadataOutput{Content: readCloser}

	mockIMDSClient.On("GetMetadata", mock.Anything, &mdInput).Return(&mdOutput, fmt.Errorf("IMDS retrieval failed"))

	roleProvider := &IIRRoleProvider{
		IMDSClient: mockIMDSClient,
		Config:     &ssmConfig,
		Log:        logger,
	}

	// Execute
	result, err := roleProvider.Retrieve(context.TODO())

	// Assertions
	assert.Error(t, err)
	assert.Equal(t, aws.Credentials{Source: ProviderName}, result)

	// Verify log error was called
	logger.AssertCalled(t, "Errorf", mock.AnythingOfType("string"), mock.Anything)
}

func TestRetrieve_JSONDecodingFailure(t *testing.T) {
	// Setup
	logger := log.NewMockLog()
	ssmConfig, _ := appconfig.Config(true)

	// Invalid JSON response
	invalidJSONResponse := `{invalid json}`

	mdInput := imds.GetMetadataInput{Path: iirCredentialsPath}

	readCloser := io.NopCloser(strings.NewReader(invalidJSONResponse))
	mdOutput := imds.GetMetadataOutput{Content: readCloser}

	mockIMDSClient := &iirprovidermocks.IEC2MdsSdkClient{}
	mockIMDSClient.On("GetMetadata", mock.Anything, &mdInput).Return(&mdOutput, nil)

	roleProvider := &IIRRoleProvider{
		IMDSClient: mockIMDSClient,
		Config:     &ssmConfig,
		Log:        logger,
	}

	// Execute
	result, err := roleProvider.Retrieve(context.TODO())

	// Assertions
	assert.Error(t, err)
	assert.Equal(t, aws.Credentials{Source: ProviderName}, result)

	// Verify log error was called
	logger.AssertCalled(t, "Errorf", mock.AnythingOfType("string"), mock.Anything)
}

func TestRetrieve_ExpiryWindowCalculation(t *testing.T) {
	// Setup
	logger := log.NewMockLog()
	ssmConfig, _ := appconfig.Config(true)

	expirationTime := time.Now().Add(time.Hour * 6)
	respCreds := Ec2RoleCreds{
		AccessKeyID:     testAccessKeyId,
		SecretAccessKey: testSecretAccessKey,
		Token:           testSessionToken,
		Expiration:      expirationTime,
		Code:            "Success",
	}

	respJSONBytes, _ := json.Marshal(respCreds)

	mdInput := imds.GetMetadataInput{Path: iirCredentialsPath}

	readCloser := io.NopCloser(strings.NewReader(string(respJSONBytes)))
	mdOutput := imds.GetMetadataOutput{Content: readCloser}

	mockIMDSClient := &iirprovidermocks.IEC2MdsSdkClient{}
	mockIMDSClient.On("GetMetadata", mock.Anything, &mdInput).Return(&mdOutput, nil)

	roleProvider := &IIRRoleProvider{
		IMDSClient: mockIMDSClient,
		Config:     &ssmConfig,
		Log:        logger,
	}

	// Execute
	result, err := roleProvider.Retrieve(context.TODO())

	// Assertions
	assert.NoError(t, err)
	assert.NotNil(t, result)

	// Check expiry window calculation (should be half of token lifetime)
	expectedExpiryWindow := time.Until(expirationTime) / 2
	assert.InDelta(t, expectedExpiryWindow.Seconds(), roleProvider.ExpiryWindow.Seconds(), 1.0)
}

func TestRetrieve_UnsuccessfulResponseCode(t *testing.T) {
	// Setup
	logger := log.NewMockLog()
	ssmConfig, _ := appconfig.Config(true)

	respCreds := Ec2RoleCreds{
		Code:       "Failure",
		Expiration: time.Now().Add(time.Hour * 6),
	}

	respJSONBytes, _ := json.Marshal(respCreds)

	mdInput := imds.GetMetadataInput{Path: iirCredentialsPath}

	readCloser := io.NopCloser(strings.NewReader(string(respJSONBytes)))
	mdOutput := imds.GetMetadataOutput{Content: readCloser}

	mockIMDSClient := &iirprovidermocks.IEC2MdsSdkClient{}
	mockIMDSClient.On("GetMetadata", mock.Anything, &mdInput).Return(&mdOutput, nil)

	roleProvider := &IIRRoleProvider{
		IMDSClient: mockIMDSClient,
		Config:     &ssmConfig,
		Log:        logger,
	}

	// Execute
	result, err := roleProvider.Retrieve(context.TODO())

	// Assertions
	assert.Error(t, err)
	assert.Equal(t, aws.Credentials{Source: ProviderName}, result)
	assert.Contains(t, err.Error(), "invalid")
}

func TestEmptyCredentials_Immutability(t *testing.T) {
	// Test that modifying returned credentials does not affect original
	originalCredentials := EmptyCredentials()
	modifiedCredentials := originalCredentials
	modifiedCredentials.AccessKeyID = "SomeKey"

	assert.NotEqual(t, originalCredentials, modifiedCredentials, "Original credentials should remain unchanged")
	assert.Equal(t, ProviderName, originalCredentials.Source, "Original provider name should be preserved")
}
