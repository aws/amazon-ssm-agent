// Copyright 2023 Amazon.com, Inc. or its affiliates. All Rights Reserved.
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

//go:build freebsd || linux || netbsd || openbsd || darwin
// +build freebsd linux netbsd openbsd darwin

// Package common contains common constants and functions needed to be accessed across ssm-setup-cli
package common

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	logmocks "github.com/aws/amazon-ssm-agent/agent/mocks/log"
	"github.com/aws/amazon-ssm-agent/agent/updateutil/updateconstants"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
)

// Define ExecHelper TestSuite struct
type ExecHelperTestSuite struct {
	suite.Suite
	logMock *logmocks.Mock
}

// Initialize the ExecHelperWindows test suite struct
func (suite *ExecHelperTestSuite) SetupTest() {
	logMock := logmocks.NewMockLog()
	suite.logMock = logMock

}

func (suite *ExecHelperTestSuite) TestExecHelper_GetCommandId_Success() {
	expectedOutput := 20
	commandProcess := exec.Cmd{Process: &os.Process{Pid: expectedOutput}}

	pid := getProcessPid(&commandProcess)
	assert.Equal(suite.T(), expectedOutput, pid, "pid mismatch")
}

func (suite *ExecHelperTestSuite) TestExecHelper_GetCommandId_Failure() {
	expectedOutput := -1
	commandProcess := exec.Cmd{}

	pid := getProcessPid(&commandProcess)
	assert.Equal(suite.T(), expectedOutput, pid, "pid mismatch")
}

func (suite *ExecHelperTestSuite) TestExecHelper_UpdateStdOutPath_Success_NonDefault() {
	updateRoot := "root1"
	fileName := "file1"

	actualOutput := updateStdOutPath(updateRoot, fileName)
	expectedOut := filepath.Join(updateRoot, fileName)
	assert.Equal(suite.T(), expectedOut, actualOutput, "file path mismatch")
}

func (suite *ExecHelperTestSuite) TestExecHelper_UpdateStdOutPath_Success_Default() {
	updateRoot := "root1"

	actualOutput := updateStdOutPath(updateRoot, "")
	expectedOut := filepath.Join(updateRoot, updateconstants.DefaultStandOut)
	assert.Equal(suite.T(), expectedOut, actualOutput, "file path mismatch")
}

func (suite *ExecHelperTestSuite) TestExecHelper_UpdateStdErrPath_Success_Default() {
	updateRoot := "root1"

	actualOutput := updateStdErrPath(updateRoot, "")
	expectedOut := filepath.Join(updateRoot, updateconstants.DefaultStandErr)
	assert.Equal(suite.T(), expectedOut, actualOutput, "file path mismatch")
}

func (suite *ExecHelperTestSuite) TestExecHelper_UpdateStdErrPath_Success_NonDefault() {
	updateRoot := "root1"
	fileName := "file1"

	actualOutput := updateStdErrPath(updateRoot, fileName)
	expectedOut := filepath.Join(updateRoot, fileName)
	assert.Equal(suite.T(), expectedOut, actualOutput, "file path mismatch")
}

func (suite *ExecHelperTestSuite) TestExecHelper_IsTimedOut_Success() {
	managerHelper := &ManagerHelper{Timeout: defaultTimeout}
	err := context.DeadlineExceeded
	actualOutput := managerHelper.IsTimeoutError(err)
	expectedOutput := true
	assert.Equal(suite.T(), expectedOutput, actualOutput, "Timed out error is expected")
}

func (suite *ExecHelperTestSuite) TestExecHelper_IsTimedOut_Failure() {
	managerHelper := &ManagerHelper{Timeout: defaultTimeout}
	err := errors.New("test")
	actualOutput := managerHelper.IsTimeoutError(err)
	expectedOutput := false
	assert.Equal(suite.T(), expectedOutput, actualOutput, "Timed out error is expected")
}

func (suite *ExecHelperTestSuite) TestExecHelper_GetExitCode_NegativeCase() {
	managerHelper := &ManagerHelper{Timeout: defaultTimeout}
	err := errors.New("test")
	actualOutput := managerHelper.GetExitCode(err)
	expectedOutput := -1
	assert.Equal(suite.T(), expectedOutput, actualOutput, "Incorrect exit code")
}

func (suite *ExecHelperTestSuite) TestExecHelper_IsCommandAvailable_Success() {
	managerHelper := &ManagerHelper{Timeout: defaultTimeout}
	cmd := "cmd"
	execLookPath = func(file string) (string, error) {
		return "path1", nil
	}
	actualOutput := managerHelper.IsCommandAvailable(cmd)
	expectedOutput := true
	assert.Equal(suite.T(), expectedOutput, actualOutput, "Command unavailable")
}

func (suite *ExecHelperTestSuite) TestExecHelper_IsCommandAvailable_Failure() {
	managerHelper := &ManagerHelper{Timeout: defaultTimeout}
	cmd := "cmd"
	execLookPath = func(file string) (string, error) {
		return "path1", fmt.Errorf("test1")
	}
	actualOutput := managerHelper.IsCommandAvailable(cmd)
	expectedOutput := false
	assert.Equal(suite.T(), expectedOutput, actualOutput, "Command available")
}

func TestExecHelperTestSuite(t *testing.T) {
	suite.Run(t, new(ExecHelperTestSuite))
}
