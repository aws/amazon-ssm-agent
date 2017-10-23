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

package executers

import (
	"io"
	"os"

	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/amazon-ssm-agent/agent/task"
	"github.com/stretchr/testify/mock"
)

// Note: This code is used in the test files. However, this code is not in a _test.go file
// because then we would have to copy it in every test package that needs the mock.

// MockCommandExecuter mocks a command executer.
type MockCommandExecuter struct {
	mock.Mock
}

// Execute is a mocked method that just returns what mock tells it to.
func (m *MockCommandExecuter) Execute(log log.T,
	workingDir string,
	stdoutFilePath string,
	stderrFilePath string,
	cancelFlag task.CancelFlag,
	executionTimeout int,
	commandName string,
	commandArguments []string,
) (stdout io.Reader, stderr io.Reader, exitCode int, errs []error) {
	args := m.Called(log, workingDir, stdoutFilePath, stderrFilePath, cancelFlag, executionTimeout, commandName, commandArguments)
	log.Infof("args are %v", args)
	return args.Get(0).(io.Reader), args.Get(1).(io.Reader), args.Get(2).(int), args.Get(3).([]error)
}

// NewExecute is a mocked method that just returns what mock tells it to.
func (m *MockCommandExecuter) NewExecute(
	log log.T,
	workingDir string,
	stdoutWriter io.Writer,
	stderrWriter io.Writer,
	cancelFlag task.CancelFlag,
	executionTimeout int,
	commandName string,
	commandArguments []string,
) (exitCode int, err error) {
	args := m.Called(log, workingDir, stdoutWriter, stderrWriter, cancelFlag, executionTimeout, commandName, commandArguments)
	log.Infof("args are %v", args)
	return args.Get(0).(int), args.Error(1)
}

// StartExe is a mocked method that just returns what mock tells it to.
func (m *MockCommandExecuter) StartExe(log log.T,
	workingDir string,
	stdoutWriter io.Writer,
	stderrWriter io.Writer,
	cancelFlag task.CancelFlag,
	commandName string,
	commandArguments []string,
) (process *os.Process, exitCode int, errs error) {
	args := m.Called(log, workingDir, stdoutWriter, stderrWriter, cancelFlag, commandName, commandArguments)
	log.Infof("args are %v", args)
	return args.Get(0).(*os.Process), args.Get(1).(int), args.Error(2)
}
