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

// Package iohandlermocks implements the mock iohandler
package iohandlermocks

import (
	"github.com/aws/amazon-ssm-agent/agent/contracts"
	"github.com/aws/amazon-ssm-agent/agent/framework/processor/executer/iohandler/iomodule"
	"github.com/aws/amazon-ssm-agent/agent/framework/processor/executer/iohandler/multiwriter"
	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/stretchr/testify/mock"
)

// MockIOHandler mocks the IOHandler.
type MockIOHandler struct {
	mock.Mock
}

// Init is a mocked method that acknowledges that the function has been called.
func (m *MockIOHandler) Init(log log.T, filePath ...string) {
	m.Called(log, filePath)
}

// RegisterOutputSource is a mocked method that acknowledges that the function has been called.
func (m *MockIOHandler) RegisterOutputSource(log log.T, multiWriter multiwriter.DocumentIOMultiWriter, IOModules ...iomodule.IOModule) {
	m.Called(log, multiWriter, IOModules)
}

// Close is a mocked method that acknowledges that the function has been called.
func (m *MockIOHandler) Close(log log.T) {
	m.Called(log)
}

// String is a mocked method that just returns what mock tells it to.
func (m *MockIOHandler) String() string {
	args := m.Called()
	return args.String(0)
}

// MarkAsFailed is a mocked method that acknowledges that the function has been called.
func (m *MockIOHandler) MarkAsFailed(err error) {
	m.Called(err)
}

// MarkAsSucceeded is a mocked method that acknowledges that the function has been called.
func (m *MockIOHandler) MarkAsSucceeded() {
	m.Called()
}

// MarkAsInProgress is a mocked method that acknowledges that the function has been called.
func (m *MockIOHandler) MarkAsInProgress() {
	m.Called()
}

// MarkAsSuccessWithReboot is a mocked method that acknowledges that the function has been called.
func (m *MockIOHandler) MarkAsSuccessWithReboot() {
	m.Called()
}

// MarkAsCancelled is a mocked method that acknowledges that the function has been called.
func (m *MockIOHandler) MarkAsCancelled() {
	m.Called()
}

// MarkAsShutdown is a mocked method that acknowledges that the function has been called.
func (m *MockIOHandler) MarkAsShutdown() {
	m.Called()
}

// AppendInfo is a mocked method that acknowledges that the function has been called.
func (m *MockIOHandler) AppendInfo(message string) {
	m.Called(message)
}

// AppendInfof is a mocked method that acknowledges that the function has been called.
func (m *MockIOHandler) AppendInfof(format string, params ...interface{}) {
	m.Called(format, params)
}

// AppendError is a mocked method that acknowledges that the function has been called.
func (m *MockIOHandler) AppendError(message string) {
	m.Called(message)
}

// AppendErrorf is a mocked method that acknowledges that the function has been called.
func (m *MockIOHandler) AppendErrorf(format string, params ...interface{}) {
	m.Called(format, params)
}

// GetStatus is a mocked method that just returns what mock tells it to.
func (m *MockIOHandler) GetStatus() contracts.ResultStatus {
	args := m.Called()
	return args.Get(0).(contracts.ResultStatus)
}

// GetStdout is a mocked method that just returns what mock tells it to.
func (m *MockIOHandler) GetStdout() string {
	args := m.Called()
	return args.String(0)
}

// GetStderr is a mocked method that just returns what mock tells it to.
func (m *MockIOHandler) GetStderr() string {
	args := m.Called()
	return args.String(0)
}

// GetExitCode is a mocked method that just returns what mock tells it to.
func (m *MockIOHandler) GetExitCode() int {
	args := m.Called()
	return args.Int(0)
}

// GetStdoutWriter is a mocked method that just returns what mock tells it to.
func (m *MockIOHandler) GetStdoutWriter() multiwriter.DocumentIOMultiWriter {
	args := m.Called()
	return args.Get(0).(multiwriter.DocumentIOMultiWriter)
}

// GetStderrWriter is a mocked method that just returns what mock tells it to.
func (m *MockIOHandler) GetStderrWriter() multiwriter.DocumentIOMultiWriter {
	args := m.Called()
	return args.Get(0).(multiwriter.DocumentIOMultiWriter)
}

// GetIOConfig is a mocked method that just returns what mock tells it to.
func (m *MockIOHandler) GetIOConfig() contracts.IOConfiguration {
	args := m.Called()
	return args.Get(0).(contracts.IOConfiguration)
}

// SetStatus is a mocked method that acknowledges that the function has been called.
func (m *MockIOHandler) SetStatus(status contracts.ResultStatus) {
	m.Called(status)
}

// SetExitCode is a mocked method that acknowledges that the function has been called.
func (m *MockIOHandler) SetExitCode(code int) {
	m.Called(code)
}

// SetOutput is a mocked method that acknowledges that the function has been called.
func (m *MockIOHandler) SetOutput(out interface{}) {
	m.Called(out)
}

// SetStdout is a mocked method that acknowledges that the function has been called.
func (m *MockIOHandler) SetStdout(stdout string) {
	m.Called(stdout)
}

// SetStderr is a mocked method that acknowledges that the function has been called.
func (m *MockIOHandler) SetStderr(stderr string) {
	m.Called(stderr)
}
