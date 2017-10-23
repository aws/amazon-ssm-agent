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
// +build windows

// Package cloudwatch implements cloudwatch plugin and its configuration
package cloudwatch

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/aws/amazon-ssm-agent/agent/executers"
	"github.com/aws/amazon-ssm-agent/agent/framework/processor/executer/iohandler"
	"github.com/aws/amazon-ssm-agent/agent/log"
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
	execMock := new(mock.Mock)
	stdout := strings.NewReader("False")
	stderr := strings.NewReader("")

	execMock.On("Execute", mock.Anything,
		mock.AnythingOfType("string"),
		mock.AnythingOfType("string"),
		mock.AnythingOfType("string"),
		mock.Anything,
		mock.AnythingOfType("int"),
		mock.AnythingOfType("string"),
		mock.AnythingOfType("[]string")).Return(stdout, stderr, 0, []error{})

	execMock.On("StartExe", mock.Anything,
		mock.AnythingOfType("string"),
		mock.AnythingOfType("string"),
		mock.AnythingOfType("string"),
		mock.Anything,
		mock.AnythingOfType("int"),
		mock.AnythingOfType("string"),
		mock.AnythingOfType("[]string")).Return(stdout, stderr, 0, []error{})

	fileExist = func(filePath string) bool {
		return true
	}

	createScript = func(log.T, string, []string) error {
		return nil
	}

	getInstanceId = func() (string, error) {
		return "i-0123456789", nil
	}

	getRegion = func() (string, error) {
		return "us-east-1", nil
	}

	var execVar = executers.MockCommandExecuter{*execMock}
	getProcess = execVar.GetProcess
	startExe = execVar.StartExe
	waitExe = execVar.Execute

	var p, _ = NewPlugin(pluginConfig)
	res := p.Start(context, "", "C:\\abc", cancelFlag)

	assert.Equal(t, nil, res)
}

// TestStartFailFileNotExist tests the Start method, which returns error when system cannot find the executable file.
func TestStartFailFileNotExist(t *testing.T) {
	fileExist = func(filePath string) bool {
		return false
	}
	context := context.NewMockDefault()
	cancelFlag := task.NewMockDefault()
	var p, _ = NewPlugin(pluginConfig)
	res := p.Start(context, "", "", cancelFlag)
	expectErr := errors.New("Unable to locate cloudwatch.exe")
	assert.Equal(t, expectErr, res)
}

// TestStopFail tests the Stop method, which returns false when stops the executable successfully.
func TestStopSuccess(t *testing.T) {
	cancelFlag := task.NewMockDefault()
	context := context.NewMockDefault()
	var p, _ = NewPlugin(pluginConfig)
	var process = os.Process{
		Pid: 1986,
	}

	p.Process = process
	res := p.Stop(context, cancelFlag)
	assert.Equal(t, nil, res)
}

// TestStopFail tests the Stop method, which returns false when cannot stop the executable successfully.
func TestStopFail(t *testing.T) {
	cancelFlag := task.NewMockDefault()
	context := context.NewMockDefault()
	var p, _ = NewPlugin(pluginConfig)
	var process = os.Process{
		Pid: 0,
	}

	execMock := new(mock.Mock)
	stdout := strings.NewReader("Process not found")
	stderr := strings.NewReader("")
	execMock.On("Execute", mock.Anything,
		mock.AnythingOfType("string"),
		mock.AnythingOfType("string"),
		mock.AnythingOfType("string"),
		mock.Anything,
		mock.AnythingOfType("int"),
		mock.AnythingOfType("string"),
		mock.AnythingOfType("[]string")).Return(stdout, stderr, 0, []error{})

	var execVar = executers.MockCommandExecuter{*execMock}
	waitExe = execVar.Execute

	p.Process = process
	res := p.Stop(context, cancelFlag)
	cloudwatchProcessName := "EC2Config.CloudWatch"
	expectErr := errors.New(fmt.Sprintf("%s is not running", cloudwatchProcessName))
	assert.Equal(t, expectErr, res)
}

// TestIsCloudWatchExeRunning tests the IsCloudWatchExeRunning method, which returns true when the cloud watch exe is running.
func TestIsCloudWatchExeRunningTrue(t *testing.T) {
	mocklog := log.NewMockLog()
	cancelFlag := task.NewMockDefault()
	execMock := new(mock.Mock)
	stdout := strings.NewReader("True")
	stderr := strings.NewReader("")

	execMock.On("Execute", mock.Anything,
		mock.AnythingOfType("string"),
		mock.AnythingOfType("string"),
		mock.AnythingOfType("string"),
		mock.Anything,
		mock.AnythingOfType("int"),
		mock.AnythingOfType("string"),
		mock.AnythingOfType("[]string")).Return(stdout, stderr, 0, []error{})

	fileExist = func(filePath string) bool {
		return true
	}

	createScript = func(log.T, string, []string) error {
		return nil
	}

	//readPrefix = func(io.Reader, int, string) (string, error) {
	//	return "True", nil
	//}

	var execVar = executers.MockCommandExecuter{*execMock}
	startExe = execVar.StartExe
	waitExe = execVar.Execute

	var p, _ = NewPlugin(pluginConfig)
	res := p.IsCloudWatchExeRunning(mocklog, "", "", cancelFlag)
	assert.True(t, res)
}

// TestIsCloudWatchExeRunning tests the IsCloudWatchExeRunning method, which returns false when the cloud watch exe is not running.
func TestIsCloudWatchExeRunningFalse(t *testing.T) {
	mocklog := log.NewMockLog()
	cancelFlag := task.NewMockDefault()
	execMock := new(mock.Mock)
	stdout := strings.NewReader("False")
	stderr := strings.NewReader("")

	execMock.On("Execute", mock.Anything,
		mock.AnythingOfType("string"),
		mock.AnythingOfType("string"),
		mock.AnythingOfType("string"),
		mock.Anything,
		mock.AnythingOfType("int"),
		mock.AnythingOfType("string"),
		mock.AnythingOfType("[]string")).Return(stdout, stderr, 0, []error{})

	fileExist = func(filePath string) bool {
		return true
	}

	createScript = func(log.T, string, []string) error {
		return nil
	}

	var execVar = executers.MockCommandExecuter{*execMock}
	startExe = execVar.StartExe
	waitExe = execVar.Execute

	var p, _ = NewPlugin(pluginConfig)
	res := p.IsCloudWatchExeRunning(mocklog, "", "", cancelFlag)
	assert.False(t, res)

}

// TestGetPidOfCloudWatchExe tests the GetPidOfCloudWatchExe method when the result is Process not found.
func TestGetPidOfCloudWatchExeNotProcess(t *testing.T) {
	mocklog := log.NewMockLog()
	cancelFlag := task.NewMockDefault()
	execMock := new(mock.Mock)
	stdout := strings.NewReader("Process not found")
	stderr := strings.NewReader("")
	execMock.On("Execute", mock.Anything,
		mock.AnythingOfType("string"),
		mock.AnythingOfType("string"),
		mock.AnythingOfType("string"),
		mock.Anything,
		mock.AnythingOfType("int"),
		mock.AnythingOfType("string"),
		mock.AnythingOfType("[]string")).Return(stdout, stderr, 0, []error{})

	var execVar = executers.MockCommandExecuter{*execMock}
	startExe = execVar.StartExe
	waitExe = execVar.Execute

	fileExist = func(filePath string) bool {
		return true
	}

	createScript = func(log.T, string, []string) error {
		return nil
	}
	cloudwatchProcessName := "EC2Config.CloudWatch"
	expectErr := errors.New(fmt.Sprintf("%s is not running", cloudwatchProcessName))
	var p, _ = NewPlugin(pluginConfig)
	pid, err := p.GetPidOfCloudWatchExe(mocklog, "", "", cancelFlag)
	assert.Equal(t, 0, pid)
	assert.Equal(t, expectErr, err)
}

// TestGetPidOfCloudWatchExe tests the GetPidOfCloudWatchExe method, which returns if the said plugin is running or not.
func TestGetPidOfCloudWatchExeSuccess(t *testing.T) {
	mocklog := log.NewMockLog()
	cancelFlag := task.NewMockDefault()
	execMock := new(mock.Mock)
	stdout := strings.NewReader("1978")
	stderr := strings.NewReader("")
	execMock.On("Execute", mock.Anything,
		mock.AnythingOfType("string"),
		mock.AnythingOfType("string"),
		mock.AnythingOfType("string"),
		mock.Anything,
		mock.AnythingOfType("int"),
		mock.AnythingOfType("string"),
		mock.AnythingOfType("[]string")).Return(stdout, stderr, 0, []error{})

	var execVar = executers.MockCommandExecuter{*execMock}
	startExe = execVar.StartExe
	waitExe = execVar.Execute

	fileExist = func(filePath string) bool {
		return true
	}

	createScript = func(log.T, string, []string) error {
		return nil
	}

	var p, _ = NewPlugin(pluginConfig)
	pid, _ := p.GetPidOfCloudWatchExe(mocklog, "", "", cancelFlag)
	assert.Equal(t, 1978, pid)
}
