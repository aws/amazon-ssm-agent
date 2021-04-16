// Copyright 2016 Amazon.com, Inc. or its affiliates. All Rights Reserved.
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

package task

import (
	"testing"
	"time"

	"github.com/aws/amazon-ssm-agent/agent/times"
	"github.com/stretchr/testify/assert"
)

type InnerFunc func()

// TestCase is a structure for testing functions that run other functions.
// The outer function is the function under test and we want to make sure
// that the inner function is called in the right sequence
type TestCase struct {
	Clock               *times.MockedClock
	CancelFlag          *ChanneledCancelFlag
	CancelWaitMillis    time.Duration
	InnerFunctionPanics bool
	innerFunction       InnerFunc
	testFunctionWrapper func()
	innerFunctionState  chan bool
	testFunctionState   chan bool
	t                   *testing.T
}

// TestProcessNoCancel tests process on a job that exits before cancel.
func TestProcessNoCancel(t *testing.T) {
	testCase := createTestCaseForProcess(t)

	// job ends then "process" ends
	testCase.assertInnerFunctionCompletes()
	testCase.assertTestMethodCompletes()

	testCase.assertExpectations()
}

// TestProcessCancelGraceful tests process on a job that gets canceled and exits before cancel timeout.
func TestProcessCancelGraceful(t *testing.T) {
	testCase := createTestCaseForProcess(t)

	testCase.setupClock()

	// start job then send cancel signal
	testCase.assertInnerFunctionStarts()

	testCase.cancelJob()

	// job terminates before timeout
	testCase.assertInnerFunctionCompletes()

	// "process" exits normally even though the timeout hasn't happened
	testCase.assertTestMethodCompletes()

	// "timeout" the clock even though everything has already exited
	testCase.timeoutCancelWait()

	testCase.assertExpectations()
}

// TestProcessCancelGraceful tests process on a job that gets canceled and fails to exit before cancel timeout.
func TestProcessCancelAbandon(t *testing.T) {
	testCase := createTestCaseForProcess(t)

	testCase.setupClock()

	// start job then send cancel signal
	testCase.assertInnerFunctionStarts()
	testCase.cancelJob()

	// timeout the "clock" before the job ends
	testCase.timeoutCancelWait()

	// check that we have exited from "process" with job still running
	testCase.assertTestMethodCompletes()

	// check that the job is still running (abandoned)
	testCase.assertInnerFunctionCompletes()

	testCase.assertExpectations()
}

// TestRunJob tests the runJob method.
func TestRunJob(t *testing.T) {
	testRunJob(t, false)
	testRunJob(t, true)
}

func testRunJob(t *testing.T, innerFunctionPanics bool) {
	// create buffered channel for job done notification
	doneChannel := make(chan struct{}, 1)
	testCase := createTestCaseForRunJob(t, doneChannel, innerFunctionPanics)

	// see that job starts
	testCase.assertInnerFunctionStarts()

	if !innerFunctionPanics {
		testCase.assertInnerFunctionCompletes()
	}

	// check that runJob sends signal and exits
	assert.NotNil(t, <-doneChannel)
	testCase.assertTestMethodCompletes()

	testCase.assertExpectations()
}

func createTestCase(t *testing.T, innerFunctionPanics bool) TestCase {
	// create job
	jobState := make(chan bool)
	job := func() {
		jobState <- true
		if innerFunctionPanics {
			panic("panic")
		}
		jobState <- false
	}

	return TestCase{
		Clock:               times.NewMockedClock(),
		CancelFlag:          NewChanneledCancelFlag(),
		CancelWaitMillis:    time.Duration(100 * time.Millisecond),
		InnerFunctionPanics: innerFunctionPanics,
		innerFunction:       job,
		innerFunctionState:  jobState,
		testFunctionState:   make(chan bool),
		t:                   t,
	}
}

func createTestCaseForProcess(t *testing.T) TestCase {
	testCase := createTestCase(t, false)
	testMethod := func() {
		job := func(CancelFlag) {
			testCase.innerFunction()
		}
		process(logger, job, testCase.CancelFlag, testCase.CancelWaitMillis, testCase.Clock)
	}
	testCase.startTestMethod(testMethod)
	return testCase
}

func createTestCaseForRunJob(t *testing.T, doneChannel chan struct{}, innerFunctionPanics bool) TestCase {
	testCase := createTestCase(t, innerFunctionPanics)
	testMethod := func() {
		runJob(logger, testCase.innerFunction, doneChannel)
	}
	testCase.startTestMethod(testMethod)
	return testCase
}

func (testCase *TestCase) startTestMethod(testMethod func()) {
	go func() {
		testCase.testFunctionState <- true
		testMethod()
		testCase.testFunctionState <- false
	}()
	assert.True(testCase.t, <-testCase.testFunctionState)
}

func (testCase *TestCase) assertInnerFunctionStarts() {
	assert.True(testCase.t, <-testCase.innerFunctionState)
}

func (testCase *TestCase) assertInnerFunctionCompletes() {
	res := <-testCase.innerFunctionState
	if res {
		assert.False(testCase.t, <-testCase.innerFunctionState)
	}
}

func (testCase *TestCase) assertTestMethodCompletes() {
	assert.False(testCase.t, <-testCase.testFunctionState)
}

func (testCase *TestCase) cancelJob() {
	testCase.CancelFlag.Set(Canceled)
}

func (testCase *TestCase) setupClock() {
	testCase.Clock.On("After", testCase.CancelWaitMillis).Return(testCase.Clock.AfterChannel)
}

func (testCase *TestCase) timeoutCancelWait() {
	testCase.Clock.AfterChannel <- time.Now()
}

func (testCase *TestCase) assertExpectations() {
	testCase.Clock.AssertExpectations(testCase.t)
}
