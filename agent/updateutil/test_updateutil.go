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

// Package updateutil contains updater specific utilities.
package updateutil

import (
	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/amazon-ssm-agent/agent/updateutil/updateconstants"
	"github.com/stretchr/testify/mock"
)

// Note: This code is used in the test files. However, this code is not in a _test.go file
// because then we would have to copy it in every test package that needs the mock.

// Mock stands for a mocked updateutil.
type Mock struct {
	mock.Mock
}

// NewMockDefault returns an instance of Mock with default expectations set.
func NewMockDefault() *Mock {
	return new(Mock)
}

// CreateUpdateDownloadFolder mocks the CreateUpdateDownloadFolder function.
func (m *Mock) CreateUpdateDownloadFolder() (folder string, err error) {
	args := m.Called()
	return args.String(0), args.Error(1)
}

// ExeCommand mocks the ExeCommand function.
func (m *Mock) ExeCommand(log log.T, cmd string, workingDir string, updaterRoot string, stdOut string, stdErr string, isAsync bool) (pid int, exitCode updateconstants.UpdateScriptExitCode, err error) {
	args := m.Called(log, cmd, workingDir, updaterRoot, stdOut, stdErr, isAsync)
	return args.Int(0), args.Get(1).(updateconstants.UpdateScriptExitCode), args.Error(0)
}

// SaveUpdatePluginResult mocks the SaveUpdatePluginResult function.
func (m *Mock) SaveUpdatePluginResult(log log.T, updaterRoot string, updateResult *UpdatePluginResult) (err error) {
	args := m.Called(log, updaterRoot, updateResult)
	return args.Error(0)
}
