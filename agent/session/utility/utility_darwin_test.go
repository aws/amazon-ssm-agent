// Copyright 2022 Amazon.com, Inc. or its affiliates. All Rights Reserved.
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

//go:build darwin
// +build darwin

// Package platform contains platform specific utilities.
package utility

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"testing"

	logger "github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/stretchr/testify/assert"
)

func TestCreateLocalAdminUser_ExistingUser_Success(t *testing.T) {
	var sessionUtil SessionUtil
	logObj := logger.NewMockLog()
	execCommand = fakeExecCommand("test")
	newPswd, err := sessionUtil.CreateLocalAdminUser(logObj)
	assert.Nil(t, err)
	assert.Equal(t, newPswd, "")
}

func TestCreateLocalAdminUser_ExistingUser_UserShell_Success(t *testing.T) {
	var sessionUtil SessionUtil
	logObj := logger.NewMockLog()
	errorPathCount := 0
	errorCommands := map[string]struct{}{}
	osStat = func(name string) (os.FileInfo, error) {
		var fileInfo os.FileInfo
		return fileInfo, nil
	}
	execCommand = fakeExecCommandWithError(errorCommands, &errorPathCount)
	newPswd, err := sessionUtil.CreateLocalAdminUser(logObj)
	assert.Nil(t, err)
	assert.Equal(t, errorPathCount, 0)
	assert.Equal(t, newPswd, "")
}

func TestCreateLocalAdminUser_NewUser_Success(t *testing.T) {
	var sessionUtil SessionUtil
	logObj := logger.NewMockLog()
	errorPathCount := 0
	errorCommands := map[string]struct{}{
		"-c id ssm-user": {},
	}
	osStat = func(name string) (os.FileInfo, error) {
		var fileInfo os.FileInfo
		return fileInfo, nil
	}
	execCommand = fakeExecCommandWithError(errorCommands, &errorPathCount) // fail only exist user check
	newPswd, err := sessionUtil.CreateLocalAdminUser(logObj)
	assert.Nil(t, err)
	assert.Equal(t, errorPathCount, 1)
	assert.Equal(t, newPswd, "")
}

func TestCreateLocalAdminUser_NewUser_UserShell_Success(t *testing.T) {
	var sessionUtil SessionUtil
	logObj := logger.NewMockLog()
	errorPathCount := 0
	errorCommands := map[string]struct{}{
		"-c id ssm-user": {},
		"-c /usr/bin/dscl . -create /Users/ssm-user UserShell /usr/bin/false": {}, // this is not applicable for new users
	}
	osStat = func(name string) (os.FileInfo, error) {
		var fileInfo os.FileInfo
		return fileInfo, nil
	}
	execCommand = fakeExecCommandWithError(errorCommands, &errorPathCount)
	newPswd, err := sessionUtil.CreateLocalAdminUser(logObj)
	assert.Nil(t, err)
	assert.Equal(t, errorPathCount, 1)
	assert.Equal(t, newPswd, "")
}

func fakeExecCommand(stdOut string) func(string, ...string) *exec.Cmd {
	return func(string, ...string) *exec.Cmd {
		return exec.Command("echo", stdOut)
	}
}

func fakeExecCommandWithError(errorCommands map[string]struct{}, errorPathCount *int) func(string, ...string) *exec.Cmd {
	cs := []string{"-test.run=TestCreateLocalAdminUser", "-test.error", "--", "echo"}
	cs = append(cs, "test")
	return func(command string, args ...string) *exec.Cmd {
		fmt.Println(args)
		if _, ok := errorCommands[strings.Join(args, " ")]; !ok {
			return exec.Command("echo", "test")
		}
		*errorPathCount = *errorPathCount + 1
		cmd := exec.Command(os.Args[0], cs...)
		cmd.Env = []string{"GO_WANT_HELPER_PROCESS=1"}
		return cmd
	}
}
