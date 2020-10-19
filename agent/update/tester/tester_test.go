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
// Package tester is responsible for initiating testing based on the test stage value passed
package tester

import (
	"testing"
	"time"

	"github.com/aws/amazon-ssm-agent/agent/log"
	testCommon "github.com/aws/amazon-ssm-agent/agent/update/tester/common"
	testerMock "github.com/aws/amazon-ssm-agent/agent/update/tester/stages/mocks"

	"github.com/stretchr/testify/assert"

	"github.com/stretchr/testify/suite"
)

// ITesterSuite tests main tester package
type ITesterSuite struct {
	suite.Suite
}

// TestTesterSuite executes test suite
func TestTesterSuite(t *testing.T) {
	suite.Run(t, new(ITesterSuite))
}

// SetupTest initializes Setup
func (suite *ITesterSuite) SetupTest() {
	getPreInstallTestObj = func(log log.T) testCommon.ITestStage {
		return getPreInstallMock("")
	}
}

// TestBasicTest tests basic functionality
func (suite *ITesterSuite) TestBasicTest() {
	output := StartTests(log.NewMockLog(), testCommon.PreInstallTest, 1)
	assert.Empty(suite.T(), output)
}

// TestUnknownTestStage tests StartTests function with unknown stage
func (suite *ITesterSuite) TestUnknownTestStage() {
	output := StartTests(log.NewMockLog(), testCommon.PostInstallTest, 1)
	assert.Empty(suite.T(), output)
}

// TestTimeout tests StartTests function with timeout
func (suite *ITesterSuite) TestTimeout() {
	runTests = func() bool {
		time.Sleep(4 * time.Second)
		return true
	}
	getPreInstallTestObj = func(log log.T) testCommon.ITestStage {
		return getPreInstallMock("listenPipeFailure")
	}
	output := StartTests(log.NewMockLog(), testCommon.PreInstallTest, 2)
	assert.Equal(suite.T(), output, "listenPipeFailure")
}

// TestFailedTest tests StartTests function with failed test case within PreInstallTest test stage
func (suite *ITesterSuite) TestFailedTest() {
	getPreInstallTestObj = func(log log.T) testCommon.ITestStage {
		return getPreInstallMock("listenPipeFailure")
	}
	output := StartTests(log.NewMockLog(), testCommon.PreInstallTest, 1)
	assert.Equal(suite.T(), output, "listenPipeFailure")
}

// getPreInstallMock returns preInstall test stage mock
func getPreInstallMock(runningTestCase string) *testerMock.ITestStage {
	channel := make(chan bool, 1)
	channel <- true
	tester := &testerMock.ITestStage{}
	tester.On("Initialize").Return()
	tester.On("RunTests").Return()
	tester.On("GetCurrentRunningTest").Return(runningTestCase)
	tester.On("CleanUpTests").Return()
	return tester
}
