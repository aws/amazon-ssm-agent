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
	testCommon "github.com/aws/amazon-ssm-agent/agent/update/tester/common"
	testStage "github.com/aws/amazon-ssm-agent/agent/update/tester/stages"
)

var (
	// getPreInstallTestObj fetches pre-install tester object
	getPreInstallTestObj = testStage.GetPreInstallTestObj

	// runTests references test execution function based on test stage value
	runTests func() bool
)

// StartTests starts the tests based on TestStage value
// This function times out based on timeOut value passed
// Returns failed or timed out test case
func StartTests(context context.T, stage testCommon.TestStage, timeOutSeconds int) (testOutput string) {
	context = context.With("[Preinstall Tests]")
	defer func() {
		if msg := recover(); msg != nil {
			context.Log().Errorf("test framework panicked: %v", msg)
		}
	}()
	var timeOutDuration time.Duration
	var testerObj testCommon.ITestStage

	testCompleted := make(chan bool, 1)
	// sets the timeout
	if timeOutSeconds <= testCommon.DefaultTestTimeOutSeconds {
		timeOutDuration = time.Duration(testCommon.DefaultTestTimeOutSeconds) * time.Second
	} else {
		timeOutDuration = time.Duration(timeOutSeconds) * time.Second
	}

	if stage == testCommon.PreInstallTest {
		testerObj = getPreInstallTestObj(context)
	} else {
		return
	}

	if runTests == nil {
		runTests = testerObj.RunTests
	}
	testerObj.Initialize()
	go func() {
		defer func() {
			if msg := recover(); msg != nil {
				context.Log().Errorf("following test case panicked: %s", testerObj.GetCurrentRunningTest())
				context.Log().Errorf("Stacktrace:\n%s", debug.Stack())
			}
		}()
		testCompleted <- runTests()
	}()

	select {
	case <-time.After(timeOutDuration):
	case <-testCompleted:
	}

	testOutput = testerObj.GetCurrentRunningTest() // better to fetch value immediately
	testerObj.CleanUpTests()
	return testOutput
}
