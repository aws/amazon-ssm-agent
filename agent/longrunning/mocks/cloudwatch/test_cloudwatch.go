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

// Package cloudwatch implements cloudwatch plugin and its configuration
package cloudwatch

import (
	"github.com/aws/amazon-ssm-agent/agent/framework/processor/executer/iohandler"
	"github.com/aws/amazon-ssm-agent/agent/task"
	"github.com/stretchr/testify/mock"
)

// Note: This code is used in the test files. However, this code is not in a _test.go file
// because then we would have to copy it in every test package that needs the mock.

// Mock stands for a mocked cloudwatch.
type Mock struct {
	mock.Mock
}

// NewMockDefault returns an instance of Mock with default expectations set.
func NewMockDefault() *Mock {
	cw := new(Mock)

	cw.On("IsRunning").Return(true)
	cw.On("Start", mock.AnythingOfType("string"), mock.AnythingOfType("string"), mock.AnythingOfType("task.CancelFlag")).Return(nil)
	cw.On("Stop", mock.AnythingOfType("task.CancelFlag")).Return(nil)
	cw.On("IsCloudWatchExeRunning", mock.AnythingOfType("string"), mock.AnythingOfType("string"), mock.AnythingOfType("task.CancelFlag")).Return(nil)
	cw.On("GetPidOfCloudWatchExe", mock.AnythingOfType("string"), mock.AnythingOfType("string"), mock.AnythingOfType("task.CancelFlag")).Return(1234, nil)
	return cw
}

// IsRunning returns if the said plugin is running or not - returns true for testing
func (m *Mock) IsRunning() bool {
	args := m.Called()
	return args.Get(0).(bool)
}

// Start starts the executable file and returns encountered errors - returns nil for testing
func (m *Mock) Start(configuration string, orchestrationDir string, cancelFlag task.CancelFlag, out iohandler.IOHandler) (err error) {
	args := m.Called(configuration, orchestrationDir, cancelFlag, out)
	return args.Get(0).(error)
}

// StopCloudWatchExe returns true if it successfully killed the cloudwatch exe or else it returns false - returns nil for testing
func (m *Mock) Stop(cancelFlag task.CancelFlag) (err error) {
	args := m.Called(cancelFlag)
	return args.Get(0).(error)
}

// IsCloudWatchExeRunning runs a powershell script to determine if the given process is running - returns nil for testing
func (m *Mock) IsCloudWatchExeRunning(workingDirectory, orchestrationDir string, cancelFlag task.CancelFlag) (err error) {
	args := m.Called(workingDirectory, orchestrationDir, cancelFlag)
	return args.Get(0).(error)
}

// IsCloudWatchExeRunning runs a powershell script to determine if the given process is running - returns nil for testing
func (m *Mock) GetPidOfCloudWatchExe(orchestrationDir, workingDirectory string, cancelFlag task.CancelFlag) (int, error) {
	args := m.Called(orchestrationDir, workingDirectory, cancelFlag)
	return args.Get(0).(int), args.Get(1).(error)
}
