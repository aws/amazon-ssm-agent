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
// permissions and limitations under the License
//
// package stages holds testStages and testStages are responsible for executing test cases
package stages

import (
	"errors"
	"testing"

	"github.com/aws/amazon-ssm-agent/agent/log"
	testCommon "github.com/aws/amazon-ssm-agent/agent/update/tester/common"
	testerMock "github.com/aws/amazon-ssm-agent/agent/update/tester/stages/mocks"

	"github.com/stretchr/testify/assert"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
)

// PreInstallTestSuite tests pre-install test stage
type PreInstallTestSuite struct {
	suite.Suite
	preInstallTestObj testCommon.ITestStage
}

//Execute the test suite
func TestPreInstallTestSuite(t *testing.T) {
	suite.Run(t, new(PreInstallTestSuite))
}

// SetupTest initializes Setup
func (suite *PreInstallTestSuite) SetupTest() {
	suite.preInstallTestObj = GetPreInstallTestObj(log.NewMockLog())
	suite.preInstallTestObj.Initialize()
}

// TestBasicTests tests the success scenario
func (suite *PreInstallTestSuite) TestBasicTests() {
	output := testCommon.TestOutput{
		Result: testCommon.TestCasePass,
		Err:    nil,
	}
	namedPipeTestCaseMock := getTestCaseMock()
	namedPipeTestCaseMock.On("ExecuteTestCase", mock.Anything).Return(output)
	namedPipeTestCaseMock.On("Initialize", mock.Anything).Return()
	suite.preInstallTestObj.RegisterTestCase(testCommon.NamedPipeTestCaseName, namedPipeTestCaseMock)
	completed := suite.preInstallTestObj.RunTests()
	assert.Equal(suite.T(), true, completed)
}

// TestBasicTests tests the failure scenario
func (suite *PreInstallTestSuite) TestFailTests() {
	output := testCommon.TestOutput{
		Result: testCommon.TestCaseFail,
		Err:    errors.New("error"),
	}
	namedPipeTestCaseMock := getTestCaseMock()
	namedPipeTestCaseMock.On("ExecuteTestCase", mock.Anything).Return(output)
	suite.preInstallTestObj.RegisterTestCase(testCommon.NamedPipeTestCaseName, namedPipeTestCaseMock)
	completed := suite.preInstallTestObj.RunTests()
	assert.Equal(suite.T(), false, completed)
	assert.Equal(suite.T(), testCommon.NamedPipeTestCaseName, suite.preInstallTestObj.GetCurrentRunningTest())
}

// getTestCaseMock returns namedPipeTestCase mock
func getTestCaseMock() *testerMock.ITestCase {
	testCaseMock := &testerMock.ITestCase{}
	testCaseMock.On("Initialize", mock.Anything).Return()
	testCaseMock.On("CleanupTestCase").Return()
	testCaseMock.On("GetTestCaseName").Return("")
	return testCaseMock
}
