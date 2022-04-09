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

// Package tester is responsible for initiating testing based on the test stage value passed
package tester

import (
	"runtime/debug"
	"time"

	"github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/aws/amazon-ssm-agent/agent/contracts"
	testCommon "github.com/aws/amazon-ssm-agent/agent/update/tester/common"
	"github.com/aws/amazon-ssm-agent/agent/update/tester/testcases"
)

// getUpdateTests fetches the list of tests to run
var getUpdateTests = func(context context.T) []testCommon.ITestCase {
	return []testCommon.ITestCase{
		testcases.NewEc2DetectorTestCase(context),
	}
}

var runTests = func(context context.T, testCases []testCommon.ITestCase, reportResults func(contracts.ResultStatus, string), doneChan chan bool, stopChan chan bool) {
	var currentRunningTest string
	defer func() {
		if msg := recover(); msg != nil {
			context.Log().Errorf("following test case panicked: %s", currentRunningTest)
			context.Log().Errorf("Stacktrace:\n%s", debug.Stack())
		}

		close(doneChan)
	}()

	for _, testCase := range testCases {
		select {
		case <-stopChan:
			return
		default:
		}

		if !testCase.ShouldRunTest() {
			// test should not be executed, pass
			continue
		}

		currentRunningTest = testCase.GetTestCaseName()
		testCase.Initialize()

		testCaseOutput := testCase.ExecuteTestCase()
		if testCaseOutput.Err != nil {
			context.Log().Errorf("error during %s test case execution %v", currentRunningTest, testCaseOutput.Err)
		}

		if testCaseOutput.Result == testCommon.TestCaseFail {
			reportResults(contracts.ResultStatusTestFailure, currentRunningTest+testCaseOutput.AdditionalInfo)
		} else if testCaseOutput.Result == testCommon.TestCasePass {
			reportResults(contracts.ResultStatusTestPass, currentRunningTest+testCaseOutput.AdditionalInfo)
		}

		testCase.CleanupTestCase()
	}
}

var defaultTestTimeOut = 10 * time.Second

// StartTests starts the tests based on TestStage value
// This function times out based on timeOut value passed
// Returns failed or timed out test case
func StartTests(context context.T, reportResults func(contracts.ResultStatus, string)) {
	context = context.With("[UpdaterTests]")
	defer func() {
		if msg := recover(); msg != nil {
			context.Log().Errorf("test framework panicked: %v", msg)
		}
	}()

	// Closed in runTests function, listened to in this function
	testCompleted := make(chan bool, 1)
	// Closed in this function, listened to in runTests function
	shouldStopTests := make(chan bool, 1)

	testCases := getUpdateTests(context)
	go runTests(context, testCases, reportResults, testCompleted, shouldStopTests)

	select {
	case <-time.After(defaultTestTimeOut):
	case <-testCompleted:
	}

	close(shouldStopTests)
}
