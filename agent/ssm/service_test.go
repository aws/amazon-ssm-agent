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
	"testing"

	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/amazon-ssm-agent/agent/platform"
	"github.com/aws/amazon-ssm-agent/agent/times"
	"github.com/aws/aws-sdk-go/aws"
	awsmock "github.com/aws/aws-sdk-go/awstesting/mock"
	"github.com/aws/aws-sdk-go/service/ssm"
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
	sdkMock    *ssm.SSM
}

// Setting up the testing environment for ssm service test.
// Give testing parameters e.g region and instanceId in awsConfig struct.
// Initialize the log mock struct.
func (suite *SsmServiceTestSuite) SetupTest() {
	logMock := log.NewMockLog()
	awsConfig := &aws.Config{}
	region := "us-east-1"
	platform.SetInstanceID("i-12345678")
	awsConfig.Region = &region
	clientMock := awsmock.NewMockClient(awsConfig)
	// This clientMock will connect to an aws mock server which will validate the input variable
	sdkMock := &ssm.SSM{
		Client: clientMock,
	}
	suite.logMock = logMock
	suite.sdkMock = sdkMock
	suite.sdkService = &sdkService{
		sdk: sdkMock,
	}
}

// Testing function for update instance association
// Generate mock time stamp struct for testing. Set the agent mock status as "active"
func (suite *SsmServiceTestSuite) TestUpdateInstanceAssociationStatus() {
	// Prepare the testing variable
	date := times.ParseIso8601UTC("2018-07-05T13:45:23.017Z")
	executionResult := ssm.InstanceAssociationExecutionResult{
		Status:           aws.String("active"),
		ErrorCode:        aws.String("0"),
		ExecutionDate:    aws.Time(date),
		ExecutionSummary: aws.String("TestExecutionSummary"),
	}
	// Test the UpdateInstanceAssociationStatus function, assert the err is nil.
	res, err := suite.sdkService.UpdateInstanceAssociationStatus(suite.logMock, "associationID", "i-1234567", &executionResult)
	assert.Nil(suite.T(), err, "Err should be nil")
	assert.NotNil(suite.T(), res, "response shouldn't be nil")
}

// Test function for update empty instance information.
// This function only update the agent name, but not update agent version and agent status
func (suite *SsmServiceTestSuite) TestUpdateEmptyInstanceInformation() {
	// Test the UpdateEmptyInstanceInformation, assert error is nil
	response, err := suite.sdkService.UpdateEmptyInstanceInformation(suite.logMock, "2.2.3.2", "Amazon-ssm-agent")
	assert.Nil(suite.T(), err, "Err should be nil")
	assert.NotNil(suite.T(), response, "response shouldn't be nil")
}

// Test function for update instance information
// This function update the agent name, agent statuc, and agent version.
func (suite *SsmServiceTestSuite) TestUpdateInstanceInformation() {
	// Give mock value to test UpdateInstanceInformation, assert the error is nil, assert the log.Debug function get called.
	response, err := suite.sdkService.UpdateInstanceInformation(suite.logMock, "2.2.3.2", "active", "Amazon-ssm-agent")
	assert.Nil(suite.T(), err, "Err should be nil")
	assert.NotNil(suite.T(), response, "response shouldn't be nil")
}

// Execute the test suite
func TestSsmServiceTestSuite(t *testing.T) {
	suite.Run(t, new(SsmServiceTestSuite))
}
