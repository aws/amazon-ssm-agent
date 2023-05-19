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
	"time"

	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/amazon-ssm-agent/agent/task"
	"github.com/stretchr/testify/mock"
)

// Note: This code is used in the test files. However, this code is not in a _test.go file
// because then we would have to copy it in every test package that needs the mock.

// MockedPool stands for a mock pool.
type MockedPool struct {
	mock.Mock
}

// Submit mocks the method with the same name.
func (mockPool *MockedPool) Submit(log log.T, jobID string, job task.Job) error {
	return mockPool.Called(log, jobID, job).Error(0)
}

// Cancel mocks the method with the same name.
func (mockPool *MockedPool) Cancel(jobID string) bool {
	return mockPool.Called(jobID).Bool(0)
}

// Shutdown mocks the method with the same name.
func (mockPool *MockedPool) Shutdown() {
	mockPool.Called()
}

// ShutdownAndWait mocks the method with the same name.
func (mockPool *MockedPool) ShutdownAndWait(timeout time.Duration) (finished bool) {
	args := mockPool.Called(timeout)
	return args.Bool(0)
}

// HasJob mocks the method with the same name.
func (mockPool *MockedPool) HasJob(jobID string) bool {
	args := mockPool.Called(jobID)
	return args.Bool(0)
}

// BufferTokensIssued mocks the method with the same name.
func (mockPool *MockedPool) BufferTokensIssued() int {
	args := mockPool.Called()
	return args.Int(0)
}

// AcquireBufferToken provides a mock function with given fields:
func (mockPool *MockedPool) AcquireBufferToken(jobId string) task.PoolErrorCode {
	ret := mockPool.Called(jobId)

	var r0 task.PoolErrorCode
	if rf, ok := ret.Get(0).(func(jobId string) task.PoolErrorCode); ok {
		r0 = rf(jobId)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(task.PoolErrorCode)
		}
	}

	return r0
}

// ReleaseBufferToken provides a mock function with given fields:
func (mockPool *MockedPool) ReleaseBufferToken(jobId string) task.PoolErrorCode {
	ret := mockPool.Called(jobId)

	var r0 task.PoolErrorCode
	if rf, ok := ret.Get(0).(func(jobId string) task.PoolErrorCode); ok {
		r0 = rf(jobId)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(task.PoolErrorCode)
		}
	}

	return r0
}

// MockCancelFlag mocks a cancel flag.
type MockCancelFlag struct {
	mock.Mock
}

// NewMockDefault returns a mocked cancel flag.
func NewMockDefault() *MockCancelFlag {
	cancelFlag := new(MockCancelFlag)
	return cancelFlag
}

// Canceled mocks the method with the same name.
func (flag *MockCancelFlag) Canceled() bool {
	return flag.Called().Bool(0)
}

// ShutDown mocks the method with the same name.
func (flag *MockCancelFlag) ShutDown() bool {
	return flag.Called().Bool(0)
}

// Wait mocks the method with the same name.
func (flag *MockCancelFlag) Wait() (state task.State) {
	return flag.Called().Get(0).(task.State)
}

func (flag *MockCancelFlag) Set(state task.State) {
	flag.Called(state)
}

func (flag *MockCancelFlag) State() task.State {
	return flag.Called().Get(0).(task.State)
}
