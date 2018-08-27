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
	"github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/aws/amazon-ssm-agent/agent/framework/processor/executer/iohandler"
	"github.com/aws/amazon-ssm-agent/agent/log"
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
	context := context.NewMockDefault()

	cw.On("IsRunning", context).Return(true)
	cw.On("Start", mock.AnythingOfType("context.T"), mock.AnythingOfType("string"), mock.AnythingOfType("string"), mock.AnythingOfType("task.CancelFlag")).Return(nil)
	cw.On("Stop", mock.AnythingOfType("context.T"), mock.AnythingOfType("task.CancelFlag")).Return(nil)
	cw.On("IsCloudWatchExeRunning", mock.AnythingOfType("log.T"), mock.AnythingOfType("string"), mock.AnythingOfType("string"), mock.AnythingOfType("task.CancelFlag")).Return(nil)
	cw.On("GetPidOfCloudWatchExe", mock.AnythingOfType("log.T"), mock.AnythingOfType("string"), mock.AnythingOfType("string"), mock.AnythingOfType("task.CancelFlag")).Return(1234, nil)
	return cw
}

// IsRunning returns if the said plugin is running or not - returns true for testing
func (m *Mock) IsRunning(context context.T) bool {
	args := m.Called(context)
	return args.Get(0).(bool)
}

// Start starts the executable file and returns encountered errors - returns nil for testing
func (m *Mock) Start(context context.T, configuration string, orchestrationDir string, cancelFlag task.CancelFlag, out iohandler.IOHandler) (err error) {
	args := m.Called(context, configuration, orchestrationDir, cancelFlag, out)
	return args.Get(0).(error)
}

// StopCloudWatchExe returns true if it successfully killed the cloudwatch exe or else it returns false - returns nil for testing
func (m *Mock) Stop(context context.T, cancelFlag task.CancelFlag) (err error) {
	args := m.Called(context, cancelFlag)
	return args.Get(0).(error)
}

// IsCloudWatchExeRunning runs a powershell script to determine if the given process is running - returns nil for testing
func (m *Mock) IsCloudWatchExeRunning(log log.T, workingDirectory, orchestrationDir string, cancelFlag task.CancelFlag) (err error) {
	args := m.Called(log, workingDirectory, orchestrationDir, cancelFlag)
	return args.Get(0).(error)
}

// IsCloudWatchExeRunning runs a powershell script to determine if the given process is running - returns nil for testing
func (m *Mock) GetPidOfCloudWatchExe(log log.T, orchestrationDir, workingDirectory string, cancelFlag task.CancelFlag) (int, error) {
	args := m.Called(log, orchestrationDir, workingDirectory, cancelFlag)
	return args.Get(0).(int), args.Get(1).(error)
}
