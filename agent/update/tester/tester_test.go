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
	"fmt"
	"testing"
	"time"

	"github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/aws/amazon-ssm-agent/agent/contracts"
	testCommon "github.com/aws/amazon-ssm-agent/agent/update/tester/common"
	"github.com/aws/amazon-ssm-agent/agent/update/tester/common/mocks"
	"github.com/stretchr/testify/assert"
)

func TestStartTests_Timeout(t *testing.T) {
	oldRunTests := runTests
	oldDefaultTimeout := defaultTestTimeOut
	defer func() {
		runTests = oldRunTests
		defaultTestTimeOut = oldDefaultTimeout
	}()

	runTestDoneChan := make(chan bool, 1)
	var doneChanHook, stopChanHook chan bool
	runTests = func(_ context.T, _ []testCommon.ITestCase, _ func(contracts.ResultStatus, string), doneChan chan bool, stopChan chan bool) {
		doneChanHook = doneChan
		stopChanHook = stopChan
		time.Sleep(time.Millisecond * 500)
		select {
		case <-stopChan:
		default:
			assert.Fail(t, "Expected stopChan to be closed")
		}

		close(doneChan)
		close(runTestDoneChan)
	}

	defaultTestTimeOut = time.Millisecond
	StartTests(context.NewMockDefault(), func(contracts.ResultStatus, string) {})
	select {
	case <-runTestDoneChan:
	case <-time.After(time.Second):
		assert.Fail(t, "Expected runTestDoneChan to be closed sooner")
	}

	select {
	case <-doneChanHook:
	default:
		assert.Fail(t, "Expected doneChan to be closed")
	}

	select {
	case <-stopChanHook:
	default:
		assert.Fail(t, "Expected stopChan to be closed")
	}
}

func TestStartTests_PanicRecovered(t *testing.T) {
	oldGetUpdateTests := getUpdateTests
	defer func() {
		getUpdateTests = oldGetUpdateTests
	}()

	getUpdateTests = func(context context.T) []testCommon.ITestCase {
		panic("Something Happened")
	}

	StartTests(context.NewMockDefault(), func(contracts.ResultStatus, string) {})
}

func TestStartTests_Success(t *testing.T) {
	oldRunTests := runTests
	oldDefaultTimeout := defaultTestTimeOut
	defer func() {
		runTests = oldRunTests
		defaultTestTimeOut = oldDefaultTimeout
	}()

	var doneChanHook, stopChanHook chan bool
	runTests = func(_ context.T, _ []testCommon.ITestCase, _ func(contracts.ResultStatus, string), doneChan chan bool, stopChan chan bool) {
		doneChanHook = doneChan
		stopChanHook = stopChan
		close(doneChan)
	}

	start := time.Now()
	defaultTestTimeOut = time.Second

	StartTests(context.NewMockDefault(), func(contracts.ResultStatus, string) {})
	assert.Less(t, time.Since(start), time.Millisecond*500)

	select {
	case <-doneChanHook:
	default:
		assert.Fail(t, "Expected doneChan to be closed")
	}

	select {
	case <-stopChanHook:
	default:
		assert.Fail(t, "Expected stopChan to be closed")
	}
}

func TestRunTests_NoTestToRun(t *testing.T) {
	stopChan := make(chan bool, 1)
	doneChan := make(chan bool, 1)

	runTests(context.NewMockDefault(), []testCommon.ITestCase{}, func(contracts.ResultStatus, string) {}, doneChan, stopChan)

	select {
	case <-stopChan:
		assert.Fail(t, "expected stopChan to still be open")
	default:
	}

	select {
	case <-doneChan:
	default:
		assert.Fail(t, "expected doneChan to be closed")
	}
}

func TestRunTests_StopBeforeAnyTestRuns(t *testing.T) {
	testMock := &mocks.ITestCase{}

	testCases := []testCommon.ITestCase{
		testMock,
	}

	stopChan := make(chan bool, 1)
	close(stopChan)

	doneChan := make(chan bool, 1)

	runTests(context.NewMockDefault(), testCases, func(contracts.ResultStatus, string) {}, doneChan, stopChan)

	select {
	case <-stopChan:
	default:
		assert.Fail(t, "expected stopChan to be closed")
	}

	select {
	case <-doneChan:
	default:
		assert.Fail(t, "expected doneChan to be closed")
	}

	testMock.AssertExpectations(t)
}

func TestRunTests_StopAfterFirstTest(t *testing.T) {
	testMock := &mocks.ITestCase{}
	testCases := []testCommon.ITestCase{
		testMock, testMock,
	}

	stopChan := make(chan bool, 1)
	doneChan := make(chan bool, 1)

	testMock.On("ShouldRunTest").Return(func() bool {
		close(stopChan)
		return false
	}).Once()

	runTests(context.NewMockDefault(), testCases, func(contracts.ResultStatus, string) {}, doneChan, stopChan)

	select {
	case <-stopChan:
	default:
		assert.Fail(t, "expected stopChan to be closed")
	}

	select {
	case <-doneChan:
	default:
		assert.Fail(t, "expected doneChan to be closed")
	}

	testMock.AssertExpectations(t)
}

func TestRunTests_RunAllTests(t *testing.T) {
	testMock := &mocks.ITestCase{}
	testCases := []testCommon.ITestCase{
		testMock, testMock, testMock, testMock,
	}

	reportResults := func(testResult contracts.ResultStatus, testName string) {
		switch testName {
		case "TestName1":
			assert.Equal(t, contracts.ResultStatusTestFailure, testResult)
		case "TestName2":
			assert.Equal(t, contracts.ResultStatusTestPass, testResult)
		case "TestName3":
			assert.Fail(t, "TestName3 should not report results because of unsupported result")
		default:
			assert.Fail(t, "Unexpected test name: "+testName)
		}
	}

	stopChan := make(chan bool, 1)
	doneChan := make(chan bool, 1)
	// First test skip
	testMock.On("ShouldRunTest").Return(false).Once()

	// Second test pass
	testMock.On("ShouldRunTest").Return(true).Once()
	testMock.On("GetTestCaseName").Return("TestName1").Once()
	testMock.On("Initialize").Return().Once()
	testMock.On("ExecuteTestCase").Return(testCommon.TestOutput{Err: nil, Result: testCommon.TestCaseFail}).Once()
	testMock.On("CleanupTestCase").Return().Once()

	// Third test fail
	testMock.On("ShouldRunTest").Return(true).Once()
	testMock.On("GetTestCaseName").Return("TestName2").Once()
	testMock.On("Initialize").Return().Once()
	testMock.On("ExecuteTestCase").Return(testCommon.TestOutput{Err: nil, Result: testCommon.TestCasePass}).Once()
	testMock.On("CleanupTestCase").Return().Once()

	// Fourth pass unexpected result
	testMock.On("ShouldRunTest").Return(true).Once()
	testMock.On("GetTestCaseName").Return("TestName3").Once()
	testMock.On("Initialize").Return().Once()
	testMock.On("ExecuteTestCase").Return(testCommon.TestOutput{Err: fmt.Errorf("SomeError"), Result: "SomeRandomResult"}).Once()
	testMock.On("CleanupTestCase").Return().Once()

	runTests(context.NewMockDefault(), testCases, reportResults, doneChan, stopChan)

	select {
	case <-stopChan:
		assert.Fail(t, "expected stopChan to still be open")
	default:
	}

	select {
	case <-doneChan:
	default:
		assert.Fail(t, "expected doneChan to be closed")
	}

	testMock.AssertExpectations(t)
}

func TestRunTests_PanicRecovery(t *testing.T) {
	testMock := &mocks.ITestCase{}
	testCases := []testCommon.ITestCase{
		testMock, testMock,
	}

	reportResults := func(testResult contracts.ResultStatus, testName string) {
		assert.Fail(t, "No test should be reported when first one fails")
	}

	stopChan := make(chan bool, 1)
	doneChan := make(chan bool, 1)

	// Panic recove
	testMock.On("ShouldRunTest").Return(func() bool {
		panic("SomeRandomPanicInTest")
	}).Once()

	runTests(context.NewMockDefault(), testCases, reportResults, doneChan, stopChan)

	select {
	case <-stopChan:
		assert.Fail(t, "expected stopChan to still be open")
	default:
	}

	select {
	case <-doneChan:
	default:
		assert.Fail(t, "expected doneChan to be closed")
	}

	testMock.AssertExpectations(t)
}
