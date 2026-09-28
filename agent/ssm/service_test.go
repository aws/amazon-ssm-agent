// Copyright 2018 Amazon.com, Inc. or its affiliates. All Rights Reserved.
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
package ssm

import (
	cont "context"
	"testing"

	"encoding/json"
	"io"
	"net/http"
	"strings"

	ssmtypes "github.com/aws/aws-sdk-go-v2/service/ssm/types"
	"github.com/stretchr/testify/mock"

	"github.com/aws/amazon-ssm-agent/agent/mocks/context"
	"github.com/aws/amazon-ssm-agent/agent/mocks/log"
	awsmock "github.com/aws/amazon-ssm-agent/agent/ssm/mocks"
	"github.com/aws/amazon-ssm-agent/agent/times"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ssm"

	//awsmock "github.com/aws/aws-sdk-go/awstesting/mock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
)

// Define the ssm service test suite. Add the log mock, external sdkmock and sdkService variable
// sdkMock use aws-sdk-go client mock object, sdkService is the struct define in service.go file
// Suite is the testify framework struct
type SsmServiceTestSuite struct {
	suite.Suite
	logMock    *log.Mock
	sdkService Service
	sdkMock    *awsmock.Service
}

// MockHTTPClient implements the HTTPClient interface for testing
type MockHTTPClient struct {
	DoFunc func(req *http.Request) (*http.Response, error)
}

func (m *MockHTTPClient) Do(req *http.Request) (*http.Response, error) {
	return m.DoFunc(req)
}

// CreateMockSSMClient creates a mock SSM client that returns successful responses
func CreateMockSSMClient() *ssm.Client {
	mockClient := &MockHTTPClient{
		DoFunc: func(req *http.Request) (*http.Response, error) {
			// Mock successful UpdateInstanceInformation response
			responseBody := `{
				"InstanceInformationList": []
			}`

			return &http.Response{
				StatusCode: 200,
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader(responseBody)),
			}, nil
		},
	}

	awsConfig := aws.Config{
		Region: "us-east-1",
		Credentials: aws.NewCredentialsCache(aws.CredentialsProviderFunc(func(ctx cont.Context) (aws.Credentials, error) {
			return aws.Credentials{
				AccessKeyID:     "mock-key",
				SecretAccessKey: "mock-secret",
			}, nil
		})),
		HTTPClient: mockClient,
	}

	return ssm.NewFromConfig(awsConfig)
}

// createCapturingMockSSMClient builds an *ssm.Client backed by a mock HTTP client
// that records the serialized request body into capturedBody, letting tests assert
// on exactly which fields the SDK put on the wire.
func createCapturingMockSSMClient(capturedBody *string) *ssm.Client {
	mockClient := &MockHTTPClient{
		DoFunc: func(req *http.Request) (*http.Response, error) {
			if req.Body != nil {
				bodyBytes, err := io.ReadAll(req.Body)
				if err == nil {
					*capturedBody = string(bodyBytes)
				}
			}

			responseBody := `{
				"InstanceInformationList": []
			}`

			return &http.Response{
				StatusCode: 200,
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader(responseBody)),
			}, nil
		},
	}

	awsConfig := aws.Config{
		Region: "us-east-1",
		Credentials: aws.NewCredentialsCache(aws.CredentialsProviderFunc(func(ctx cont.Context) (aws.Credentials, error) {
			return aws.Credentials{
				AccessKeyID:     "mock-key",
				SecretAccessKey: "mock-secret",
			}, nil
		})),
		HTTPClient: mockClient,
	}

	return ssm.NewFromConfig(awsConfig)
}

// Give testing parameters e.g region and instanceId in awsConfig struct.
// Initialize the log mock struct.
func (suite *SsmServiceTestSuite) SetupTest() {
	logMock := log.NewMockLog()
	awsConfig := aws.Config{}
	region := "us-east-1"
	awsConfig.Region = region
	sdkMock := &awsmock.Service{}
	//ssmClient := ssm.NewFromConfig(awsConfig)
	// This clientMock will connect to an aws mock server which will validate the input variable
	suite.logMock = logMock
	suite.sdkMock = sdkMock
	suite.sdkService = &sdkService{
		context: context.NewMockDefault(),
		sdk:     CreateMockSSMClient(),
	}
}

// Testing function for update instance association
// Generate mock time stamp struct for testing. Set the agent mock status as "active"
func (suite *SsmServiceTestSuite) TestUpdateInstanceAssociationStatus() {
	// Prepare the testing variable
	date := times.ParseIso8601UTC("2018-07-05T13:45:23.017Z")
	executionResult := ssmtypes.InstanceAssociationExecutionResult{
		Status:           aws.String("active"),
		ErrorCode:        aws.String("0"),
		ExecutionDate:    aws.Time(date),
		ExecutionSummary: aws.String("TestExecutionSummary"),
	}
	suite.sdkMock.On("UpdateInstanceAssociationStatus", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(&ssm.UpdateInstanceAssociationStatusOutput{}, nil)
	// Test the UpdateInstanceAssociationStatus function, assert the err is nil.
	res, err := suite.sdkMock.UpdateInstanceAssociationStatus(suite.logMock, "associationID", "i-12345678", &executionResult)
	assert.Nil(suite.T(), err, "Err should be nil")
	assert.NotNil(suite.T(), res, "response shouldn't be nil")
}

/// TODO: Look into moving below tests to intergration tests as actual creds are neeeded to make calls to real ssm client.

// Test function for update empty instance information.
// This function only update the agent name, but not update agent version and agent status
func (suite *SsmServiceTestSuite) TestUpdateEmptyInstanceInformation() {
	// Test the UpdateEmptyInstanceInformation, assert error is nil
	response, err := suite.sdkService.UpdateEmptyInstanceInformation(suite.logMock, "2.2.3.2", "Amazon-ssm-agent")

	assert.Nil(suite.T(), err, "Err should be nil")
	assert.NotNil(suite.T(), response, "response shouldn't be nil")
}

// Test function for update instance information
// This function update the agent name, agent status, and agent version.
func (suite *SsmServiceTestSuite) TestUpdateInstanceInformation() {
	// Give mock value to test UpdateInstanceInformation, assert the error is nil, assert the log.Debug function get called.
	response, err := suite.sdkService.UpdateInstanceInformation(
		suite.logMock,
		"2.2.3.2",
		"active",
		"Amazon-ssm-agent",
		"us-east-1b",
		"use1-az2",
		"ssmmessages",
		"1724afd8-9092-429e-8b04-0708130c38f7",
		"Microsoft.Compute/virtualMachines",
		"centralus",
		"test-computer")
	assert.Nil(suite.T(), err, "Err should be nil")
	assert.NotNil(suite.T(), response, "response shouldn't be nil")
}

// Execute the test suite
func TestSsmServiceTestSuite(t *testing.T) {
	suite.Run(t, new(SsmServiceTestSuite))
}

// TestUpdateInstanceInformation_WithSourceFields verifies that sourceId, sourceType,
// and sourceLocation are included in the API call when they are non-empty.
func TestUpdateInstanceInformation_WithSourceFields(t *testing.T) {
	t.Skip("SourceId/SourceType/SourceLocation are not yet in the vendored aws-sdk-go-v2 ssm fork; " +
		"UpdateInstanceInformation intentionally no-ops these (see TODOs in service.go). " +
		"Unskip when the SDK fork adds the fields and the TODOs are uncommented.")

	logMock := log.NewMockLog()

	var capturedBody string
	mockClient := createCapturingMockSSMClient(&capturedBody)

	svc := &sdkService{
		context: context.NewMockDefault(),
		sdk:     mockClient,
	}

	_, err := svc.UpdateInstanceInformation(
		logMock,
		"3.0.0",
		"Active",
		"amazon-ssm-agent",
		"us-east-1a",
		"use1-az1",
		"ssmmessages",
		"test-source-id",
		"Microsoft.Compute/virtualMachines",
		"centralus",
		"test-computer",
	)

	assert.Nil(t, err)
	assert.NotEmpty(t, capturedBody)

	var requestBody map[string]interface{}
	err = json.Unmarshal([]byte(capturedBody), &requestBody)
	assert.Nil(t, err)
	assert.Equal(t, "test-source-id", requestBody["SourceId"])
	assert.Equal(t, "Microsoft.Compute/virtualMachines", requestBody["SourceType"])
	assert.Equal(t, "centralus", requestBody["SourceLocation"])
	assert.Equal(t, "test-computer", requestBody["ComputerName"])
}

// TestUpdateInstanceInformation_EmptySourceFields verifies that sourceId, sourceType,
// and sourceLocation are not set when provided as empty strings.
func TestUpdateInstanceInformation_EmptySourceFields(t *testing.T) {
	logMock := log.NewMockLog()

	var capturedBody string
	mockClient := createCapturingMockSSMClient(&capturedBody)

	svc := &sdkService{
		context: context.NewMockDefault(),
		sdk:     mockClient,
	}

	_, err := svc.UpdateInstanceInformation(
		logMock,
		"3.0.0",
		"Active",
		"amazon-ssm-agent",
		"us-east-1a",
		"use1-az1",
		"ssmmessages",
		"",
		"",
		"",
		"",
	)

	assert.Nil(t, err)
	assert.NotEmpty(t, capturedBody)

	var requestBody map[string]interface{}
	err = json.Unmarshal([]byte(capturedBody), &requestBody)
	assert.Nil(t, err)
	// When sourceId/sourceType/sourceLocation are empty, they should not be in the request body
	_, hasSourceId := requestBody["SourceId"]
	_, hasSourceType := requestBody["SourceType"]
	_, hasSourceLocation := requestBody["SourceLocation"]
	assert.False(t, hasSourceId, "SourceId should not be set when empty")
	assert.False(t, hasSourceType, "SourceType should not be set when empty")
	assert.False(t, hasSourceLocation, "SourceLocation should not be set when empty")
	// ComputerName should still be set (falls back to platform.Hostname)
	_, hasComputerName := requestBody["ComputerName"]
	assert.True(t, hasComputerName, "ComputerName should be set")
	assert.NotEqual(t, "", requestBody["ComputerName"])
}

// TestUpdateInstanceInformation_ComputerNameFallback verifies that when computerName
// is empty, the function falls back to platform.Hostname instead of using the empty value.
func TestUpdateInstanceInformation_ComputerNameFallback(t *testing.T) {
	logMock := log.NewMockLog()

	var capturedBody string
	mockClient := createCapturingMockSSMClient(&capturedBody)

	svc := &sdkService{
		context: context.NewMockDefault(),
		sdk:     mockClient,
	}

	_, err := svc.UpdateInstanceInformation(
		logMock,
		"3.0.0",
		"Active",
		"amazon-ssm-agent",
		"us-east-1a",
		"use1-az1",
		"ssmmessages",
		"source-123",
		"AWS::EC2::Instance",
		"us-east-1",
		"", // empty computerName should trigger fallback
	)

	assert.Nil(t, err)
	assert.NotEmpty(t, capturedBody)

	var requestBody map[string]interface{}
	err = json.Unmarshal([]byte(capturedBody), &requestBody)
	assert.Nil(t, err)
	// ComputerName should not be empty - it should have fallen back to platform.Hostname
	_, hasComputerName := requestBody["ComputerName"]
	assert.True(t, hasComputerName, "ComputerName should be set")
	assert.NotEqual(t, "", requestBody["ComputerName"])
}

// TestUpdateInstanceInformation_ComputerNameProvided verifies that when computerName
// is non-empty, it is used directly without calling platform.Hostname.
func TestUpdateInstanceInformation_ComputerNameProvided(t *testing.T) {
	logMock := log.NewMockLog()

	var capturedBody string
	mockClient := createCapturingMockSSMClient(&capturedBody)

	svc := &sdkService{
		context: context.NewMockDefault(),
		sdk:     mockClient,
	}

	_, err := svc.UpdateInstanceInformation(
		logMock,
		"3.0.0",
		"Active",
		"amazon-ssm-agent",
		"us-east-1a",
		"use1-az1",
		"ssmmessages",
		"source-123",
		"AWS::EC2::Instance",
		"us-east-1",
		"my-custom-hostname",
	)

	assert.Nil(t, err)
	assert.NotEmpty(t, capturedBody)

	var requestBody map[string]interface{}
	err = json.Unmarshal([]byte(capturedBody), &requestBody)
	assert.Nil(t, err)
	assert.Equal(t, "my-custom-hostname", requestBody["ComputerName"])
}

// TestUpdateInstanceInformation_PartialSourceFields verifies behavior when only some
// source fields are provided (e.g., sourceId is set but sourceType and sourceLocation are empty).
func TestUpdateInstanceInformation_PartialSourceFields(t *testing.T) {
	t.Skip("SourceId/SourceType/SourceLocation are not yet in the vendored aws-sdk-go-v2 ssm fork; " +
		"UpdateInstanceInformation intentionally no-ops these (see TODOs in service.go). " +
		"Unskip when the SDK fork adds the fields and the TODOs are uncommented.")

	logMock := log.NewMockLog()

	var capturedBody string
	mockClient := createCapturingMockSSMClient(&capturedBody)

	svc := &sdkService{
		context: context.NewMockDefault(),
		sdk:     mockClient,
	}

	_, err := svc.UpdateInstanceInformation(
		logMock,
		"3.0.0",
		"Active",
		"amazon-ssm-agent",
		"us-east-1a",
		"use1-az1",
		"ssmmessages",
		"source-123", // sourceId provided
		"",           // sourceType empty
		"",           // sourceLocation empty
		"my-host",
	)

	assert.Nil(t, err)
	assert.NotEmpty(t, capturedBody)

	var requestBody map[string]interface{}
	err = json.Unmarshal([]byte(capturedBody), &requestBody)
	assert.Nil(t, err)
	// sourceId should be set since it's non-empty
	assert.Equal(t, "source-123", requestBody["SourceId"])
	// sourceType and sourceLocation should not be in the request body since they are empty
	_, hasSourceType := requestBody["SourceType"]
	_, hasSourceLocation := requestBody["SourceLocation"]
	assert.False(t, hasSourceType, "SourceType should not be set when empty")
	assert.False(t, hasSourceLocation, "SourceLocation should not be set when empty")
}
