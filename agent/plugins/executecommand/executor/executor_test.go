// Copyright 2017 Amazon.com, Inc. or its affiliates. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License"). You may not
// use this file except in compliance with the License. A copy of the
// License is located at
//
// http://aws.amazon.com/apache2.0/
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or
// implied. See the License for the specific language governing
// permissions and limitations under the License.

// Package executor implements the document and script related functionality for executecommand
// This file has unit tests to test the ExecCommand interface
package executor

import (
	"github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/aws/amazon-ssm-agent/agent/contracts"
	"github.com/aws/amazon-ssm-agent/agent/docmanager/model"
	"github.com/aws/amazon-ssm-agent/agent/framework/processor/executer"
	exec_mock "github.com/aws/amazon-ssm-agent/agent/framework/processor/executer/mock"
	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/amazon-ssm-agent/agent/plugins/executecommand/filemanager"
	"github.com/aws/amazon-ssm-agent/agent/plugins/executecommand/remoteresource"
	"github.com/aws/amazon-ssm-agent/agent/task"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"bytes"
	"io"
	"os"
	"testing"
)

var contextMock = context.NewMockDefault()
var plugin = model.PluginState{}

func TestExecCommandImpl_ExecuteDocumentFailure(t *testing.T) {

	output := contracts.PluginOutput{}
	documentId := "documentId"
	var pluginInput []model.PluginState
	pluginInput = append(pluginInput, plugin)
	var pluginRes map[string]*contracts.PluginResult

	execMock := createExecDocTestStub(pluginRes, pluginInput)

	doc := ExecCommandImpl{
		DocExecutor: func(context context.T) executer.Executer {
			return execMock
		},
	}
	doc.ExecuteDocument(contextMock, pluginInput, documentId, "time", &output)

	assert.Equal(t, contracts.ResultStatusFailed, output.Status)
}

func TestExecCommandImpl_ExecuteDocumentSuccess(t *testing.T) {

	output := contracts.PluginOutput{}
	documentId := "documentId"
	var pluginInput []model.PluginState
	pluginInput = append(pluginInput, plugin)
	pluginRes := make(map[string]*contracts.PluginResult)
	pluginResult := contracts.PluginResult{
		PluginName:     "aws:executeCommand",
		Status:         contracts.ResultStatusSuccess,
		StandardOutput: "out",
	}
	pluginRes["stringname"] = &pluginResult

	execMock := createExecDocTestStub(pluginRes, pluginInput)

	doc := ExecCommandImpl{
		DocExecutor: func(context context.T) executer.Executer {
			return execMock
		},
	}
	doc.ExecuteDocument(contextMock, pluginInput, documentId, "time", &output)

	assert.Equal(t, contracts.ResultStatusSuccess, output.Status)
	assert.Equal(t, 0, output.ExitCode)
}

func TestExecCommandImpl_ExecuteScript(t *testing.T) {
	log := log.NewMockLog()
	output := contracts.PluginOutput{
		Status: contracts.ResultStatusSuccess,
	}
	filemanager.SetPermission = fakeChmod
	shell_exec := shellExecMock{}
	args := []string{}
	stdout := bytes.NewBuffer([]byte("stdout"))
	stderr := bytes.NewBuffer([]byte(""))
	errs := []error{}

	shell_exec.On("Execute", log, mock.Anything, mock.Anything, mock.Anything, mock.AnythingOfType("*task.ChanneledCancelFlag"),
		mock.AnythingOfType("int"), mock.Anything, mock.AnythingOfType("[]string")).Return(stdout, stderr, 0, errs)

	doc := ExecCommandImpl{
		ScriptExecutor: shell_exec,
	}

	resourceInfo := remoteresource.ResourceInfo{
		LocalDestinationPath: "path",
		StarterFile:          "filename",
	}
	doc.ExecuteScript(log, resourceInfo.LocalDestinationPath, args, 2500, &output)

	assert.Equal(t, 0, output.ExitCode)
	assert.Equal(t, contracts.ResultStatusSuccess, output.Status)

	shell_exec.AssertExpectations(t)
}

type instanceInfoStub struct{}

// InstanceID wraps platform InstanceID
func (m instanceInfoStub) InstanceID() (string, error) {
	return "instanceId", nil
}

// Region wraps platform Region
func (m instanceInfoStub) Region() (string, error) {
	return "region", nil
}

func createExecDocTestStub(pluginRes map[string]*contracts.PluginResult, pluginInput []model.PluginState) executer.Executer {
	execMock := exec_mock.NewMockExecuter()
	instance = &instanceInfoStub{}
	docResultChan := make(chan contracts.DocumentResult)

	go func() {
		res := contracts.DocumentResult{
			LastPlugin:    "",
			Status:        contracts.ResultStatusSuccess,
			PluginResults: pluginRes,
		}
		docResultChan <- res
		close(docResultChan)
	}()
	execMock.On("Run", mock.AnythingOfType("*task.ChanneledCancelFlag"), mock.AnythingOfType("*executer.DocumentFileStore")).Return(docResultChan)

	return execMock
}

type shellExecMock struct {
	mock.Mock
}

func (s shellExecMock) Execute(log log.T,
	workingDir string,
	stdoutFilePath string,
	stderrFilePath string,
	cancelFlag task.CancelFlag,
	executionTimeout int,
	commandName string,
	commandArguments []string,
) (io.Reader, io.Reader, int, []error) {
	args := s.Called(log, workingDir, stdoutFilePath, stderrFilePath, cancelFlag, executionTimeout, commandName, commandArguments)
	return args.Get(0).(io.Reader), args.Get(1).(io.Reader), args.Int(2), args.Get(3).([]error)
}

func (s shellExecMock) StartExe(log log.T,
	workingDir string,
	stdoutFilePath string,
	stderrFilePath string,
	cancelFlag task.CancelFlag,
	commandName string,
	commandArgs []string) (*os.Process, int, []error) {
	args := s.Called(log, workingDir, stdoutFilePath, stderrFilePath, cancelFlag, commandName, commandArgs)
	return args.Get(0).(*os.Process), args.Int(1), args.Get(2).([]error)
}

func fakeChmod(name string, mode os.FileMode) error {
	return nil
}
