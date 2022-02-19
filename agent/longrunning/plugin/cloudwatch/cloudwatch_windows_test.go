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
//
//go:build windows
// +build windows

// Package cloudwatch implements cloudwatch plugin and its configuration
package cloudwatch

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/aws/amazon-ssm-agent/agent/executers"
	iohandlermocks "github.com/aws/amazon-ssm-agent/agent/framework/processor/executer/iohandler/mock"
	multiwritermock "github.com/aws/amazon-ssm-agent/agent/framework/processor/executer/iohandler/multiwriter/mock"

	"github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/aws/amazon-ssm-agent/agent/framework/processor/executer/iohandler"
	"github.com/aws/amazon-ssm-agent/agent/task"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

var pluginConfig = iohandler.PluginConfig{
	StdoutFileName:        "stdout",
	StderrFileName:        "stderr",
	MaxStdoutLength:       2500,
	MaxStderrLength:       2500,
	OutputTruncatedSuffix: "cw",
}

// TestStartFailFileNotExist tests the Start method, which returns nil when start the executable file successfully.
func TestStartSuccess(t *testing.T) {
	context := context.NewMockDefault()
	cancelFlag := task.NewMockDefault()
	execMock := &executers.MockCommandExecuter{}
	stdout := strings.NewReader("False")
	stderr := strings.NewReader("")
	ioHandler := &iohandlermocks.MockIOHandler{}
	testPid := 1986
	findProcessCalled := false
	killProcessCalled := false
	process := &os.Process{
		Pid: testPid,
	}

	cancelFlag.On("Wait").Return(task.Completed)
	cancelFlag.On("Canceled").Return(false)
	ioHandler.On("GetStdoutWriter").Return(&multiwritermock.MockDocumentIOMultiWriter{})
	ioHandler.On("GetStderrWriter").Return(&multiwritermock.MockDocumentIOMultiWriter{})
	execMock.On("Execute", mock.Anything,
		mock.AnythingOfType("string"),
		mock.AnythingOfType("string"),
		mock.AnythingOfType("string"),
		mock.Anything,
		mock.AnythingOfType("int"),
		mock.AnythingOfType("string"),
		mock.AnythingOfType("[]string"),
		mock.AnythingOfType("map[string]string")).Return(stdout, stderr, 0, []error{})

	execMock.On("StartExe", mock.Anything,
		mock.AnythingOfType("string"),
		mock.Anything,
		mock.Anything,
		mock.Anything,
		mock.AnythingOfType("string"),
		mock.AnythingOfType("[]string")).Return(process, 0, nil)

	fileExist = func(filePath string) bool {
		return true
	}

	findProcess = func(pid int) (*os.Process, error) {
		findProcessCalled = true
		assert.Equal(t, testPid, pid)
		return process, nil
	}

	killProcess = func(p *os.Process) error {
		killProcessCalled = true
		assert.Equal(t, testPid, p.Pid)
		return nil
	}

	p, _ := NewPlugin(context, pluginConfig)
	p.CommandExecuter = execMock
	res := p.Start("", "C:\\abc", cancelFlag, ioHandler)

	assert.Equal(t, nil, res)
	assert.False(t, findProcessCalled)
	assert.False(t, killProcessCalled)
}

// TestStartFailFileNotExist tests the Start method, which returns error when system cannot find the executable file.
func TestStartFailFileNotExist(t *testing.T) {
	fileExist = func(filePath string) bool {
		return false
	}
	ioHandler := &iohandlermocks.MockIOHandler{}
	context := context.NewMockDefault()
	cancelFlag := task.NewMockDefault()

	p, _ := NewPlugin(context, pluginConfig)
	res := p.Start("", "", cancelFlag, ioHandler)
	expectErr := errors.New("unable to locate cloudwatch.exe")
	assert.Equal(t, expectErr, res)
}

func TestStopSuccess(t *testing.T) {
	cancelFlag := task.NewMockDefault()
	context := context.NewMockDefault()
	execMock := &executers.MockCommandExecuter{}

	testPid := 1986
	findProcessCalled := false
	killProcessCalled := false
	cwProcInfo := CloudwatchProcessInfo{
		PId: testPid,
	}

	procInfoJSON, _ := json.Marshal(cwProcInfo)
	stdout := strings.NewReader(string(procInfoJSON))
	stderr := strings.NewReader("")

	p, _ := NewPlugin(context, pluginConfig)
	process := &os.Process{
		Pid: testPid,
	}

	findProcess = func(pid int) (*os.Process, error) {
		findProcessCalled = true
		assert.Equal(t, testPid, pid)
		return process, nil
	}

	killProcess = func(p *os.Process) error {
		killProcessCalled = true
		assert.Equal(t, testPid, p.Pid)
		return nil
	}

	execMock.On("Execute", mock.Anything,
		mock.AnythingOfType("string"),
		mock.AnythingOfType("string"),
		mock.AnythingOfType("string"),
		mock.Anything,
		mock.AnythingOfType("int"),
		mock.AnythingOfType("string"),
		mock.AnythingOfType("[]string"),
		mock.AnythingOfType("map[string]string")).Return(stdout, stderr, 0, []error{})

	p.CommandExecuter = execMock
	p.Process = process
	res := p.Stop(cancelFlag)
	assert.Equal(t, nil, res)
	assert.True(t, findProcessCalled)
	assert.True(t, killProcessCalled)
}

func TestStopFail_FailedToFindCloudWatchProcess(t *testing.T) {
	cancelFlag := task.NewMockDefault()
	context := context.NewMockDefault()
	execMock := &executers.MockCommandExecuter{}

	testPid := 1986
	findProcessCalled := false
	killProcessCalled := false
	cwProcInfo := CloudwatchProcessInfo{
		PId: testPid,
	}

	procInfoJSON, _ := json.Marshal(cwProcInfo)
	stdout := strings.NewReader(string(procInfoJSON))
	stderr := strings.NewReader("")

	p, _ := NewPlugin(context, pluginConfig)
	process := &os.Process{
		Pid: testPid,
	}

	findProcess = func(pid int) (*os.Process, error) {
		findProcessCalled = true
		assert.Equal(t, testPid, pid)
		return nil, fmt.Errorf("failed to find process with pid %v", pid)
	}

	killProcess = func(p *os.Process) error {
		killProcessCalled = true
		return nil
	}

	execMock.On("Execute", mock.Anything,
		mock.AnythingOfType("string"),
		mock.AnythingOfType("string"),
		mock.AnythingOfType("string"),
		mock.Anything,
		mock.AnythingOfType("int"),
		mock.AnythingOfType("string"),
		mock.AnythingOfType("[]string"),
		mock.AnythingOfType("map[string]string")).Return(stdout, stderr, 0, []error{})

	p.CommandExecuter = execMock
	p.Process = process
	res := p.Stop(cancelFlag)
	assert.NotNil(t, res)
	assert.Contains(t, res.Error(), "failed to find process CloudWatch process")
	assert.True(t, findProcessCalled)
	assert.False(t, killProcessCalled)
}

func TestStopFail_FailedToKillProcess(t *testing.T) {
	cancelFlag := task.NewMockDefault()
	context := context.NewMockDefault()
	execMock := &executers.MockCommandExecuter{}
	expProcessKillError := errors.New("failed to kill process")

	testPid := 1986
	findProcessCalled := false
	killProcessCalled := false
	cwProcInfo := CloudwatchProcessInfo{
		PId: testPid,
	}

	procInfoJSON, _ := json.Marshal(cwProcInfo)
	stdout := strings.NewReader(string(procInfoJSON))
	stderr := strings.NewReader("")

	p, _ := NewPlugin(context, pluginConfig)
	process := &os.Process{
		Pid: testPid,
	}

	findProcess = func(pid int) (*os.Process, error) {
		findProcessCalled = true
		assert.Equal(t, testPid, pid)
		return process, nil
	}

	killProcess = func(p *os.Process) error {
		killProcessCalled = true
		assert.Equal(t, testPid, p.Pid)
		return expProcessKillError
	}

	execMock.On("Execute", mock.Anything,
		mock.AnythingOfType("string"),
		mock.AnythingOfType("string"),
		mock.AnythingOfType("string"),
		mock.Anything,
		mock.AnythingOfType("int"),
		mock.AnythingOfType("string"),
		mock.AnythingOfType("[]string"),
		mock.AnythingOfType("map[string]string")).Return(stdout, stderr, 0, []error{})

	p.CommandExecuter = execMock
	p.Process = process
	res := p.Stop(cancelFlag)
	assert.NotNil(t, res)
	assert.Equal(t, expProcessKillError, res)
	assert.True(t, findProcessCalled)
	assert.True(t, killProcessCalled)
}

// TestIsCloudWatchExeRunning tests the IsCloudWatchExeRunning method, which returns true when the cloud watch exe is running.
func TestIsCloudWatchExeRunningTrue(t *testing.T) {
	context := context.NewMockDefault()
	cancelFlag := task.NewMockDefault()
	cancelFlag.On("Wait").Return(task.Completed)
	cancelFlag.On("Canceled").Return(false)
	execMock := &executers.MockCommandExecuter{}
	stdout := strings.NewReader("True")
	stderr := strings.NewReader("")

	execMock.On("Execute", mock.Anything,
		mock.AnythingOfType("string"),
		mock.AnythingOfType("string"),
		mock.AnythingOfType("string"),
		mock.Anything,
		mock.AnythingOfType("int"),
		mock.AnythingOfType("string"),
		mock.AnythingOfType("[]string"),
		mock.AnythingOfType("map[string]string")).Return(stdout, stderr, 0, []error{})

	fileExist = func(filePath string) bool {
		return true
	}

	var p, _ = NewPlugin(context, pluginConfig)
	p.CommandExecuter = execMock
	res := p.IsCloudWatchExeRunning("", "", cancelFlag)
	assert.True(t, res)
}

// TestIsCloudWatchExeRunning tests the IsCloudWatchExeRunning method, which returns false when the cloud watch exe is not running.
func TestIsCloudWatchExeRunningFalse(t *testing.T) {
	cancelFlag := task.NewMockDefault()
	cancelFlag.On("Wait").Return(task.Completed)
	cancelFlag.On("Canceled").Return(false)
	execMock := &executers.MockCommandExecuter{}
	stdout := strings.NewReader("False")
	stderr := strings.NewReader("")

	execMock.On("Execute", mock.Anything,
		mock.AnythingOfType("string"),
		mock.AnythingOfType("string"),
		mock.AnythingOfType("string"),
		mock.Anything,
		mock.AnythingOfType("int"),
		mock.AnythingOfType("string"),
		mock.AnythingOfType("[]string"),
		mock.AnythingOfType("map[string]string")).Return(stdout, stderr, 0, []error{})

	fileExist = func(filePath string) bool {
		return true
	}

	var p, _ = NewPlugin(context.NewMockDefault(), pluginConfig)
	res := p.IsCloudWatchExeRunning("", "", cancelFlag)
	assert.False(t, res)

}

// TestGetPidOfCloudWatchExe tests the GetPidOfCloudWatchExe method, which returns if the said plugin is running or not.
func TestGetPidOfCloudWatchExeSuccess(t *testing.T) {
	context := context.NewMockDefault()
	cancelFlag := task.NewMockDefault()
	cancelFlag.On("Wait").Return(task.Completed)
	cancelFlag.On("Canceled").Return(false)
	execMock := &executers.MockCommandExecuter{}
	testPid := 1978
	cwProcInfo := CloudwatchProcessInfo{
		PId: testPid,
	}

	procInfoJSON, _ := json.Marshal(cwProcInfo)
	stdout := strings.NewReader(string(procInfoJSON))
	stderr := strings.NewReader("")
	execMock.On("Execute", mock.Anything,
		mock.AnythingOfType("string"),
		mock.AnythingOfType("string"),
		mock.AnythingOfType("string"),
		mock.Anything,
		mock.AnythingOfType("int"),
		mock.AnythingOfType("string"),
		mock.AnythingOfType("[]string"),
		mock.AnythingOfType("map[string]string")).Return(stdout, stderr, 0, []error{})

	fileExist = func(filePath string) bool {
		return true
	}

	var p, _ = NewPlugin(context, pluginConfig)
	p.CommandExecuter = execMock
	procInfos, _ := p.GetProcInfoOfCloudWatchExe("", "", cancelFlag)
	assert.NotNil(t, procInfos)
	assert.Equal(t, 1, len(procInfos))
	assert.Equal(t, 1978, procInfos[0].PId)
}
