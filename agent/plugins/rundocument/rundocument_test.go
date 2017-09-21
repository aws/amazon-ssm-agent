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
// permissions and limitations under the License..

// Package rundocument implements the aws:runDocument plugin
package rundocument

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"time"

	"github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/aws/amazon-ssm-agent/agent/contracts"
	"github.com/aws/amazon-ssm-agent/agent/docmanager/model"
	filemock "github.com/aws/amazon-ssm-agent/agent/fileutil/filemanager/mock"
	executermocks "github.com/aws/amazon-ssm-agent/agent/framework/processor/executer/mock"
	"github.com/aws/amazon-ssm-agent/agent/log"
	ssmsvc "github.com/aws/amazon-ssm-agent/agent/ssm"
	"github.com/aws/amazon-ssm-agent/agent/task"
	"github.com/aws/aws-sdk-go/service/ssm"
)

var logMock = log.NewMockLog()
var contextMock = context.NewMockDefault()
var plugin = model.PluginState{}

func TestReadFileContents(t *testing.T) {
	fileMock := filemock.FileSystemMock{}
	destinationDir := "destination"

	fileMock.On("ReadFile", destinationDir).Return("content", nil)

	rawFile, err := readFileContents(logMock, fileMock, destinationDir)

	assert.NoError(t, err)
	assert.Equal(t, []byte("content"), rawFile)
	fileMock.AssertExpectations(t)
}

func TestReadFileContents_Fail(t *testing.T) {
	fileMock := filemock.FileSystemMock{}
	destinationDir := "destination"

	fileMock.On("ReadFile", destinationDir).Return("content", fmt.Errorf("Error"))

	_, err := readFileContents(logMock, fileMock, destinationDir)

	assert.Error(t, err)
	fileMock.AssertExpectations(t)
}

func TestExecCommandImpl_ExecuteDocumentFailure(t *testing.T) {

	//Expected out isFail because plugin output is nil
	output := contracts.PluginOutput{}
	documentId := "documentId"
	var pluginInput []model.PluginState
	pluginInput = append(pluginInput, plugin)
	var pluginRes map[string]*contracts.PluginResult

	execMock := createExecDocTestStub(pluginRes, pluginInput)

	exec := ExecDocumentImpl{
		DocExecutor: execMock,
	}
	exec.ExecuteDocument(contextMock, pluginInput, documentId, "time", &output)

	assert.Equal(t, contracts.ResultStatusFailed, output.Status)
}

func TestExecCommandImpl_ExecuteDocumentSuccess(t *testing.T) {

	output := contracts.PluginOutput{}
	documentId := "documentId"
	var pluginInput []model.PluginState
	pluginInput = append(pluginInput, plugin)
	pluginRes := make(map[string]*contracts.PluginResult)
	pluginResult := contracts.PluginResult{
		PluginName:     "aws:runDocument",
		Status:         contracts.ResultStatusSuccess,
		StandardOutput: "out",
	}
	pluginRes["stringname"] = &pluginResult

	execMock := createExecDocTestStub(pluginRes, pluginInput)

	exec := ExecDocumentImpl{
		DocExecutor: execMock,
	}
	exec.ExecuteDocument(contextMock, pluginInput, documentId, "time", &output)

	assert.Equal(t, contracts.ResultStatusSuccess, output.Status)
	assert.Equal(t, 0, output.ExitCode)
}

func TestExecutePlugin_PrepareDocumentForExecution(t *testing.T) {

	execMock := NewExecMock()
	fileMock := filemock.FileSystemMock{}

	plugin := model.PluginState{}
	plugins := []model.PluginState{plugin}

	parameters := make(map[string]interface{})
	conf := createStubConfiguration("orch", "bucket", "prefix", "1234-1234-1234", "directory")

	fileMock.On("ReadFile", "document/name.json").Return("content", nil)
	execMock.On("ParseDocument", logMock, ".json", []byte("content"), conf.OrchestrationDirectory, conf.OutputS3BucketName, conf.OutputS3KeyPrefix, conf.MessageId, conf.PluginID, conf.DefaultWorkingDirectory, parameters).Return(plugins, nil)

	p := Plugin{
		filesys: fileMock,
		execDoc: execMock,
	}

	_, err := p.prepareDocumentForExecution(logMock, "document/name.json", conf, "")

	assert.NoError(t, err)
	fileMock.AssertExpectations(t)
	execMock.AssertExpectations(t)
}

func TestExecutePlugin_PrepareDocumentForExecutionFail(t *testing.T) {

	execMock := NewExecMock()
	localFileMock := filemock.FileSystemMock{}

	conf := createStubConfiguration("orch", "bucket", "prefix", "1234-1234-1234", "directory")

	localFileMock.On("ReadFile", "document/name.json").Return("", fmt.Errorf("File is empty!"))

	p := Plugin{
		filesys: localFileMock,
		execDoc: execMock,
	}

	_, err := p.prepareDocumentForExecution(logMock, "document/name.json", conf, "")

	assert.Error(t, err)
	assert.Equal(t, fmt.Errorf("File is empty!"), err)
	localFileMock.AssertExpectations(t)
}

func TestExecuteImpl_PrepareDocumentForExecutionParametersJSON(t *testing.T) {
	execMock := NewExecMock()
	fileMock := filemock.FileSystemMock{}

	plugin := model.PluginState{}
	plugins := []model.PluginState{plugin}

	params := `{
		"param1":"hello",
		"param2":"world"
	}`
	parameters := make(map[string]interface{})
	parameters["param1"] = "hello"
	parameters["param2"] = "world"
	conf := createStubConfiguration("orch", "bucket", "prefix", "1234-1234-1234", "directory")

	fileMock.On("ReadFile", "document/doc-name.json").Return("content", nil)
	execMock.On("ParseDocument", logMock, ".json", []byte("content"), conf.OrchestrationDirectory, conf.OutputS3BucketName, conf.OutputS3KeyPrefix, conf.MessageId, conf.PluginID, conf.DefaultWorkingDirectory, parameters).Return(plugins, nil)

	p := Plugin{
		filesys: fileMock,
		execDoc: execMock,
	}

	_, err := p.prepareDocumentForExecution(logMock, "document/doc-name.json", conf, params)

	assert.NoError(t, err)
	fileMock.AssertExpectations(t)
	execMock.AssertExpectations(t)
}

func TestExecuteImpl_PrepareDocumentForExecutionParametersYAML(t *testing.T) {
	execMock := NewExecMock()

	fileMock := filemock.FileSystemMock{}

	plugin := model.PluginState{}
	plugins := []model.PluginState{plugin}

	params := `{
		"param1":"hello",
		"param2":"world"
	}`
	parameters := make(map[string]interface{})
	parameters["param1"] = "hello"
	parameters["param2"] = "world"
	conf := createStubConfiguration("orch", "bucket", "prefix", "1234-1234-1234", "directory")

	fileMock.On("ReadFile", "document/doc-name.yaml").Return("content", nil)
	execMock.On("ParseDocument", logMock, ".yaml", []byte("content"), conf.OrchestrationDirectory, conf.OutputS3BucketName, conf.OutputS3KeyPrefix, conf.MessageId, conf.PluginID, conf.DefaultWorkingDirectory, parameters).Return(plugins, nil)

	p := Plugin{
		filesys: fileMock,
		execDoc: execMock,
	}

	_, err := p.prepareDocumentForExecution(logMock, "document/doc-name.yaml", conf, params)

	assert.NoError(t, err)
	fileMock.AssertExpectations(t)
	execMock.AssertExpectations(t)
}

func TestPlugin_RunDocumentMaxDepthExceeded(t *testing.T) {

	execMock := NewExecMock()
	fileMock := filemock.FileSystemMock{}
	fileMock.On("MakeDirs", "orch").Return(nil)
	fileMock.On("WriteFile", "orch", mock.Anything).Return(nil).Twice()
	mockplugin := MockDefaultPlugin{}
	mockplugin.On("UploadOutputToS3Bucket", contextMock.Log(), mock.Anything, mock.Anything,
		mock.Anything, mock.Anything, false, mock.Anything,
		mock.Anything, mock.Anything).Return([]string{})

	conf := createStubConfiguration("orch", "bucket", "prefix", "1234-1234-1234", "directory")

	var input RunDocumentPluginInput
	input.DocumentType = "LocalPath"
	input.DocumentPath = "/var/tmp/docLocation/docname.json"
	conf.Properties = &input
	var executionDepth interface{}
	executionDepth = createStubExecutionDepth(4)
	conf.Settings = executionDepth

	p := Plugin{
		filesys: fileMock,
		execDoc: execMock,
	}

	p.ExecuteUploadOutputToS3Bucket = mockplugin.UploadOutputToS3Bucket

	result := p.execute(contextMock, conf, createMockCancelFlag())

	execMock.AssertExpectations(t)
	mockplugin.AssertExpectations(t)
	fileMock.AssertExpectations(t)

	assert.Equal(t, 1, result.Code)
	assert.Contains(t, result.Output, "Maximum depth for document execution exceeded. Maximum depth permitted - 3 and current depth - 5")
}

func TestPlugin_RunDocumentFromSSMDocument(t *testing.T) {

	execMock := NewExecMock()
	fileMock := filemock.FileSystemMock{}
	ssmMock := ssmsvc.NewMockDefault()

	content := "content"
	docResponse := ssm.GetDocumentOutput{
		Content: &content,
	}
	conf := createStubConfiguration("orch", "bucket", "prefix", "1234-1234-1234", "directory")

	plugin := model.PluginState{}
	plugins := []model.PluginState{plugin}

	parameters := make(map[string]interface{})
	output := contracts.PluginOutput{}
	ssmMock.On("GetDocument", contextMock.Log(), "AWS-RunShellScript", "1").Return(&docResponse, nil)
	fileMock.On("MakeDirs", mock.Anything).Return(nil)
	fileMock.On("WriteFile", mock.Anything, content).Return(nil)
	fileMock.On("ReadFile", mock.Anything).Return(content, nil)
	execMock.On("ParseDocument", contextMock.Log(), ".json", []byte(content), conf.OrchestrationDirectory, conf.OutputS3BucketName, conf.OutputS3KeyPrefix, conf.MessageId, conf.PluginID, conf.DefaultWorkingDirectory, parameters).Return(plugins, nil)
	execMock.On("ExecuteDocument", contextMock, plugins, conf.BookKeepingFileName, mock.Anything, &output).Return()

	var input RunDocumentPluginInput
	input.DocumentType = "SSMDocument"
	input.DocumentPath = "AWS-RunShellScript:1"
	conf.Properties = &input

	p := Plugin{
		filesys: fileMock,
		ssmSvc:  ssmMock,
		execDoc: execMock,
	}

	p.runDocument(contextMock, &input, conf, &output)

	execMock.AssertExpectations(t)
	fileMock.AssertExpectations(t)
	ssmMock.AssertExpectations(t)

	assert.Equal(t, 0, output.ExitCode)
}

func TestPlugin_RunDocumentFromAbsLocalPath(t *testing.T) {

	execMock := NewExecMock()
	fileMock := filemock.FileSystemMock{}

	content := "content"
	conf := createStubConfiguration("orch", "bucket", "prefix", "1234-1234-1234", "directory")

	plugin := model.PluginState{}
	plugins := []model.PluginState{plugin}

	parameters := make(map[string]interface{})
	output := contracts.PluginOutput{}
	fileMock.On("ReadFile", "/var/tmp/document/docName.json").Return(content, nil)
	execMock.On("ParseDocument", contextMock.Log(), ".json", []byte(content), conf.OrchestrationDirectory, conf.OutputS3BucketName, conf.OutputS3KeyPrefix, conf.MessageId, conf.PluginID, conf.DefaultWorkingDirectory, parameters).Return(plugins, nil)
	execMock.On("ExecuteDocument", contextMock, plugins, conf.BookKeepingFileName, mock.Anything, &output).Return()

	var input RunDocumentPluginInput
	input.DocumentType = "LocalPath"
	input.DocumentPath = "/var/tmp/document/docName.json"
	conf.Properties = &input

	p := Plugin{
		filesys: fileMock,
		execDoc: execMock,
	}

	p.runDocument(contextMock, &input, conf, &output)

	execMock.AssertExpectations(t)
	fileMock.AssertExpectations(t)
	assert.Equal(t, 0, output.ExitCode)
}

func TestName(t *testing.T) {
	assert.Equal(t, "aws:runDocument", Name())
}

func createStubExecutionDepth(depth int) *ExecutePluginDepth {
	currentDepth := ExecutePluginDepth{}
	currentDepth.executeCommandDepth = depth

	return &currentDepth
}

func createExecDocTestStub(pluginRes map[string]*contracts.PluginResult, pluginInput []model.PluginState) *executermocks.MockedExecuter {
	execMock := executermocks.NewMockExecuter()
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

func createStubConfiguration(orch, bucket, prefix, message, dir string) contracts.Configuration {
	return contracts.Configuration{
		OrchestrationDirectory:  orch,
		OutputS3BucketName:      bucket,
		OutputS3KeyPrefix:       prefix,
		MessageId:               message,
		PluginID:                "aws:runDocument",
		DefaultWorkingDirectory: dir,
	}
}

// MockDefaultPlugin mocks the default plugin.
type MockDefaultPlugin struct {
	mock.Mock
}

// UploadOutputToS3Bucket is a mocked method that just returns what mock tells it to.
func (m *MockDefaultPlugin) UploadOutputToS3Bucket(log log.T, pluginID string, orchestrationDir string, outputS3BucketName string, outputS3KeyPrefix string, useTempDirectory bool, tempDir string, Stdout string, Stderr string) []string {
	args := m.Called(log, pluginID, orchestrationDir, outputS3BucketName, outputS3KeyPrefix, useTempDirectory, tempDir, mock.Anything, Stderr)
	log.Infof("args are %v", args)
	return args.Get(0).([]string)
}
func createMockCancelFlag() task.CancelFlag {
	mockCancelFlag := new(task.MockCancelFlag)
	// Setup mocks
	mockCancelFlag.On("Canceled").Return(false)
	mockCancelFlag.On("ShutDown").Return(false)
	mockCancelFlag.On("Wait").Return(false).After(100 * time.Millisecond)

	return mockCancelFlag
}
