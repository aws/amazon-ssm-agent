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

	"io/ioutil"

	"github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/aws/amazon-ssm-agent/agent/contracts"
	filemock "github.com/aws/amazon-ssm-agent/agent/fileutil/filemanager/mock"
	iohandlermocks "github.com/aws/amazon-ssm-agent/agent/framework/processor/executer/iohandler/mock"
	executermocks "github.com/aws/amazon-ssm-agent/agent/framework/processor/executer/mock"
	"github.com/aws/amazon-ssm-agent/agent/log"
	ssmsvc "github.com/aws/amazon-ssm-agent/agent/ssm"
	"github.com/aws/amazon-ssm-agent/agent/task"
	"github.com/aws/aws-sdk-go/service/ssm"
)

// instanceMock
type InstanceMock struct {
	mock.Mock
}

// InstanceID mocks implementation for InstanceID
func (m *InstanceMock) InstanceID() (string, error) {
	return "", nil
}

var logMock = log.NewMockLog()
var contextMock = context.NewMockDefault()
var plugin = contracts.PluginState{}

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

func TestExecDocumentImpl_ExecuteDocumentFailure(t *testing.T) {

	//Expected out isFail because plugin output is nil
	documentId := "documentId"
	var pluginInput []contracts.PluginState
	pluginInput = append(pluginInput, plugin)

	execMock := executermocks.NewMockExecuter()
	docResultChan := make(chan contracts.DocumentResult)
	mockObj := new(InstanceMock)
	mockObj.On("InstanceID", mock.Anything, mock.Anything).Return("instanceID", nil)

	instance = mockObj

	execMock.On("Run", mock.AnythingOfType("*task.ChanneledCancelFlag"), mock.AnythingOfType("*executer.DocumentFileStore")).Return(docResultChan)

	exec := ExecDocumentImpl{
		DocExecutor: execMock,
	}
	conf := createStubConfiguration("orch", "bucket", "prefix", "1234-1234-1234", "directory")
	_, err := exec.ExecuteDocument(conf, contextMock, pluginInput, documentId, "time")

	assert.NoError(t, err)
}

func TestExecDocumentImpl_ExecuteDocumentSuccess(t *testing.T) {

	documentId := "documentId"
	var pluginInput []contracts.PluginState
	pluginInput = append(pluginInput, plugin)
	pluginRes := make(map[string]*contracts.PluginResult)
	pluginResult := contracts.PluginResult{
		PluginName:     "aws:runDocument",
		Status:         contracts.ResultStatusSuccess,
		StandardOutput: "out",
	}
	pluginRes["aws:runDocument"] = &pluginResult

	execMock := executermocks.NewMockExecuter()
	docResultChan := make(chan contracts.DocumentResult)
	mockObj := new(InstanceMock)
	mockObj.On("InstanceID", mock.Anything, mock.Anything).Return("instanceID", nil)

	instance = mockObj
	execMock.On("Run", mock.AnythingOfType("*task.ChanneledCancelFlag"), mock.AnythingOfType("*executer.DocumentFileStore")).Return(docResultChan)
	exec := ExecDocumentImpl{
		DocExecutor: execMock,
	}
	conf := createStubConfiguration("orch", "bucket", "prefix", "1234-1234-1234", "directory")
	_, err := exec.ExecuteDocument(conf, contextMock, pluginInput, documentId, "time")

	assert.NoError(t, err)
}

func TestExecDocumentImpl_ExecuteDocumentWithMultiplePlugin(t *testing.T) {

	documentId := "documentId"
	conf := contracts.Configuration{
		OrchestrationDirectory:  "orch",
		OutputS3BucketName:      "bucket",
		OutputS3KeyPrefix:       "prefix",
		MessageId:               "1234567890",
		PluginID:                "aws:runShellScript",
		DefaultWorkingDirectory: "directory",
		PluginName:              "aws:runShellScript",
	}
	var pluginInput []contracts.PluginState
	pluginInput = append(pluginInput, plugin)
	execMock := executermocks.NewMockExecuter()
	docResultChan := make(chan contracts.DocumentResult)

	execMock.On("Run", mock.AnythingOfType("*task.ChanneledCancelFlag"), mock.AnythingOfType("*executer.DocumentFileStore")).Return(docResultChan)
	exec := ExecDocumentImpl{
		DocExecutor: execMock,
	}
	_, err := exec.ExecuteDocument(conf, contextMock, pluginInput, documentId, "time")

	assert.NoError(t, err)
}

func TestExecutePlugin_PrepareDocumentForExecution(t *testing.T) {

	execMock := NewExecMock()
	fileMock := filemock.FileSystemMock{}

	plugin := contracts.PluginState{}
	plugins := []contracts.PluginState{plugin}

	parameters := make(map[string]interface{})
	conf := createStubConfiguration("orch", "bucket", "prefix", "1234-1234-1234", "directory")

	content := `{
		"key" : "value"
	}`
	fileMock.On("ReadFile", "document/name.json").Return(content, nil)
	execMock.On("ParseDocument", logMock, []byte(content), conf.OrchestrationDirectory, conf.OutputS3BucketName, conf.OutputS3KeyPrefix, conf.MessageId, conf.PluginID, conf.DefaultWorkingDirectory, parameters).Return(plugins, nil)

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
	execMock.AssertExpectations(t)
}

func TestExecuteImpl_PrepareDocumentForExecutionParametersYAML(t *testing.T) {
	execMock := NewExecMock()
	fileMock := filemock.FileSystemMock{}

	plugin := contracts.PluginState{}
	plugins := []contracts.PluginState{plugin}

	params := `
param1: hello
param2: world`

	parameters := make(map[string]interface{})
	parameters["param1"] = "hello"
	parameters["param2"] = "world"
	conf := createStubConfiguration("orch", "bucket", "prefix", "1234-1234-1234", "directory")

	fileMock.On("ReadFile", "document/doc-name.json").Return("content", nil)
	execMock.On("ParseDocument", logMock, []byte("content"), conf.OrchestrationDirectory, conf.OutputS3BucketName, conf.OutputS3KeyPrefix, conf.MessageId, conf.PluginID, conf.DefaultWorkingDirectory, parameters).Return(plugins, nil)

	p := Plugin{
		filesys: fileMock,
		execDoc: execMock,
	}

	_, err := p.prepareDocumentForExecution(logMock, "document/doc-name.json", conf, params)

	assert.NoError(t, err)
	fileMock.AssertExpectations(t)
	execMock.AssertExpectations(t)
}

func TestExecuteImpl_PrepareDocumentForExecutionParametersJSON(t *testing.T) {
	execMock := NewExecMock()

	fileMock := filemock.FileSystemMock{}

	plugin := contracts.PluginState{}
	plugins := []contracts.PluginState{plugin}

	params := `{
		"param1":"hello",
		"param2":"world"
	}`
	parameters := make(map[string]interface{})
	parameters["param1"] = "hello"
	parameters["param2"] = "world"
	conf := createStubConfiguration("orch", "bucket", "prefix", "1234-1234-1234", "directory")

	fileMock.On("ReadFile", "document/doc-name.yaml").Return("content", nil)
	execMock.On("ParseDocument", logMock, []byte("content"), conf.OrchestrationDirectory, conf.OutputS3BucketName, conf.OutputS3KeyPrefix, conf.MessageId, conf.PluginID, conf.DefaultWorkingDirectory, parameters).Return(plugins, nil)

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

	// Test to check if the max depth code works in the fail case
	execMock := NewExecMock()
	fileMock := filemock.FileSystemMock{}
	mockIOHandler := new(iohandlermocks.MockIOHandler)
	mockplugin := MockDefaultPlugin{}

	conf := createStubConfiguration("orch", "bucket", "prefix", "1234-1234-1234", "directory")

	var input RunDocumentPluginInput
	input.DocumentType = "LocalPath"
	input.DocumentPath = "/var/tmp/docLocation/docname.json"
	conf.Properties = &input
	var executionDepth interface{}
	executionDepth = createStubExecutionDepth(4)
	conf.Settings = executionDepth

	mockIOHandler.On("MarkAsFailed", fmt.Errorf("Maximum depth for document execution exceeded. Maximum depth permitted - 3 and current depth - 5")).Return()

	p := Plugin{
		filesys: fileMock,
		execDoc: execMock,
	}
	p.execute(contextMock, conf, createMockCancelFlag(), mockIOHandler)

	execMock.AssertExpectations(t)
	mockplugin.AssertExpectations(t)
	fileMock.AssertExpectations(t)
	mockIOHandler.AssertExpectations(t)
}

func TestPlugin_RunDocument(t *testing.T) {

	execMock := NewExecMock()
	fileMock := filemock.FileSystemMock{}
	mockIOHandler := new(iohandlermocks.MockIOHandler)
	mockplugin := MockDefaultPlugin{}

	conf := createStubConfiguration("orch", "bucket", "prefix", "1234-1234-1234", "directory")

	var input RunDocumentPluginInput
	input.DocumentType = LocalPathType
	input.DocumentPath = "/var/tmp/docLocation/docname.json"
	conf.Properties = &input

	resChan := make(chan contracts.DocumentResult)
	pluginRes := contracts.PluginResult{
		PluginID:   "aws:runDocument",
		PluginName: "aws:runDocument",
		Status:     contracts.ResultStatusSuccess,
		Code:       0,
	}
	pluginResults := make(map[string]*contracts.PluginResult)
	pluginResults[pluginRes.PluginID] = &pluginRes

	go func() {
		res := contracts.DocumentResult{
			LastPlugin:    "",
			Status:        contracts.ResultStatusSuccess,
			PluginResults: pluginResults,
		}

		resChan <- res
		close(resChan)
	}()
	parameters := make(map[string]interface{})
	content := "content"

	plugin := contracts.PluginState{}
	plugins := []contracts.PluginState{plugin}

	fileMock.On("ReadFile", "/var/tmp/docLocation/docname.json").Return(content, nil)
	execMock.On("ParseDocument", contextMock.Log(), []byte(content), conf.OrchestrationDirectory, conf.OutputS3BucketName, conf.OutputS3KeyPrefix, conf.MessageId, conf.PluginID, conf.DefaultWorkingDirectory, parameters).Return(plugins, nil)
	execMock.On("ExecuteDocument", contextMock, plugins, conf.BookKeepingFileName, mock.Anything).Return(resChan, nil)
	mockIOHandler.On("GetStatus").Return(contracts.ResultStatusSuccess)
	mockIOHandler.On("SetStatus", contracts.ResultStatusSuccess).Return()

	p := Plugin{
		filesys: fileMock,
		execDoc: execMock,
	}

	p.execute(contextMock, conf, createMockCancelFlag(), mockIOHandler)

	execMock.AssertExpectations(t)
	mockplugin.AssertExpectations(t)
	fileMock.AssertExpectations(t)
	mockIOHandler.AssertExpectations(t)
}

func TestPlugin_RunDocumentFromSSMDocument(t *testing.T) {

	execMock := NewExecMock()
	fileMock := filemock.FileSystemMock{}
	mockIOHandler := new(iohandlermocks.MockIOHandler)
	ssmMock := ssmsvc.NewMockDefault()

	content := "content"
	docResponse := ssm.GetDocumentOutput{
		Content: &content,
	}
	conf := createStubConfiguration("orch", "bucket", "prefix", "1234-1234-1234", "directory")

	plugin := contracts.PluginState{}
	plugins := []contracts.PluginState{plugin}

	parameters := make(map[string]interface{})
	resChan := make(chan contracts.DocumentResult)

	pluginRes := contracts.PluginResult{
		PluginID:   "aws:runDocument",
		PluginName: "aws:runDocument",
		Status:     contracts.ResultStatusSuccess,
		Code:       0,
	}
	pluginResults := make(map[string]*contracts.PluginResult)
	pluginResults[pluginRes.PluginID] = &pluginRes
	go func() {
		res := contracts.DocumentResult{
			LastPlugin:    "",
			Status:        contracts.ResultStatusSuccess,
			PluginResults: pluginResults,
		}

		resChan <- res
		close(resChan)
	}()

	ssmMock.On("GetDocument", contextMock.Log(), "RunShellScript", "10").Return(&docResponse, nil)
	fileMock.On("MakeDirs", "orch/downloads").Return(nil)
	fileMock.On("WriteFile", "orch/downloads/RunShellScript.json", content).Return(nil)
	fileMock.On("ReadFile", "orch/downloads/RunShellScript.json").Return(content, nil)
	execMock.On("ParseDocument", contextMock.Log(), []byte(content), conf.OrchestrationDirectory, conf.OutputS3BucketName, conf.OutputS3KeyPrefix, conf.MessageId, conf.PluginID, conf.DefaultWorkingDirectory, parameters).Return(plugins, nil)
	execMock.On("ExecuteDocument", contextMock, plugins, conf.BookKeepingFileName, mock.Anything).Return(resChan, nil)
	mockIOHandler.On("GetStatus").Return(contracts.ResultStatusSuccess)
	mockIOHandler.On("SetStatus", contracts.ResultStatusSuccess).Return()

	var input RunDocumentPluginInput
	input.DocumentType = "SSMDocument"
	input.DocumentPath = "RunShellScript:10"
	conf.Properties = &input

	p := Plugin{
		filesys: fileMock,
		ssmSvc:  ssmMock,
		execDoc: execMock,
	}

	p.runDocument(contextMock, &input, conf, mockIOHandler)

	execMock.AssertExpectations(t)
	fileMock.AssertExpectations(t)
	mockIOHandler.AssertExpectations(t)
	ssmMock.AssertExpectations(t)
}

func TestPlugin_RunDocumentFromAbsLocalPath(t *testing.T) {

	execMock := NewExecMock()
	fileMock := filemock.FileSystemMock{}
	mockIOHandler := new(iohandlermocks.MockIOHandler)

	content := "content"
	conf := createStubConfiguration("orch", "bucket", "prefix", "1234-1234-1234", "directory")

	plugin := contracts.PluginState{}
	plugins := []contracts.PluginState{plugin}
	pluginRes := contracts.PluginResult{
		PluginID:   "aws:runDocument",
		PluginName: "aws:runDocument",
		Status:     contracts.ResultStatusSuccess,
		Code:       0,
	}
	pluginResults := make(map[string]*contracts.PluginResult)
	pluginResults[pluginRes.PluginID] = &pluginRes

	resChan := make(chan contracts.DocumentResult)
	go func() {
		res := contracts.DocumentResult{
			LastPlugin:    "",
			Status:        contracts.ResultStatusSuccess,
			PluginResults: pluginResults,
		}

		resChan <- res
		close(resChan)
	}()
	parameters := make(map[string]interface{})

	fileMock.On("ReadFile", "/var/tmp/document/docName.json").Return(content, nil)
	execMock.On("ParseDocument", contextMock.Log(), []byte(content), conf.OrchestrationDirectory, conf.OutputS3BucketName, conf.OutputS3KeyPrefix, conf.MessageId, conf.PluginID, conf.DefaultWorkingDirectory, parameters).Return(plugins, nil)
	execMock.On("ExecuteDocument", contextMock, plugins, conf.BookKeepingFileName, mock.Anything).Return(resChan, nil)
	mockIOHandler.On("GetStatus").Return(contracts.ResultStatusSuccess)
	mockIOHandler.On("SetStatus", contracts.ResultStatusSuccess).Return()

	var input RunDocumentPluginInput
	input.DocumentType = "LocalPath"
	input.DocumentPath = "/var/tmp/document/docName.json"
	conf.Properties = &input

	p := Plugin{
		filesys: fileMock,
		execDoc: execMock,
	}

	p.runDocument(contextMock, &input, conf, mockIOHandler)

	execMock.AssertExpectations(t)
	fileMock.AssertExpectations(t)
	mockIOHandler.AssertExpectations(t)
}

func TestName(t *testing.T) {
	assert.Equal(t, "aws:runDocument", Name())
}

func TestExecDocumentImpl_ParseDocumentYAML(t *testing.T) {
	yamlDoc := loadFile(t, "testdata/yamldoc.yaml")
	conf := contracts.Configuration{
		OrchestrationDirectory:  "orch",
		OutputS3BucketName:      "bucket",
		OutputS3KeyPrefix:       "prefix",
		MessageId:               "1234-1234-1234",
		PluginID:                "aws:runScript",
		DefaultWorkingDirectory: "directory",
		PluginName:              "aws:runScript",
	}
	var exec ExecDocumentImpl
	var params map[string]interface{}
	pluginsInfo, err := exec.ParseDocument(contextMock.Log(), []byte(yamlDoc), conf.OrchestrationDirectory, conf.OutputS3BucketName, conf.OutputS3KeyPrefix, conf.MessageId, conf.PluginID, conf.DefaultWorkingDirectory, params)

	assert.NoError(t, err)
	for _, plugin := range pluginsInfo {
		assert.NotEqual(t, plugin.Configuration, conf)
		assert.Equal(t, plugin.Id, conf.PluginID)
		assert.Equal(t, plugin.Name, conf.PluginName)
		assert.NotEqual(t, nil, plugin.Configuration.Properties)
	}
}

func TestExecDocumentImpl_ParseDocumentJSON(t *testing.T) {
	jsonDoc := loadFile(t, "testdata/jsondoc.json")
	conf := contracts.Configuration{
		OrchestrationDirectory:  "orch",
		OutputS3BucketName:      "bucket",
		OutputS3KeyPrefix:       "prefix",
		MessageId:               "1234-1234-1234",
		PluginID:                "aws:runScript",
		DefaultWorkingDirectory: "directory",
		PluginName:              "aws:runScript",
	}
	var exec ExecDocumentImpl
	var params map[string]interface{}
	pluginsInfo, err := exec.ParseDocument(contextMock.Log(), []byte(jsonDoc), conf.OrchestrationDirectory, conf.OutputS3BucketName, conf.OutputS3KeyPrefix, conf.MessageId, conf.PluginID, conf.DefaultWorkingDirectory, params)

	assert.NoError(t, err)
	for _, plugin := range pluginsInfo {
		assert.NotEqual(t, plugin.Configuration, conf)
		assert.Equal(t, plugin.Id, conf.PluginID)
		assert.Equal(t, plugin.Name, conf.PluginName)
		assert.NotEqual(t, nil, plugin.Configuration.Properties)
	}
}

func TestValidateInput_NoDocumentType(t *testing.T) {
	input := RunDocumentPluginInput{}

	result, err := validateInput(&input)

	assert.False(t, result)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "Document Type must be specified to either by SSMDocument or LocalPath.")

}
func TestValidateInput_UnknownDocumentType(t *testing.T) {
	input := RunDocumentPluginInput{}
	input.DocumentType = "unknown"

	result, err := validateInput(&input)

	assert.False(t, result)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "Document type specified in invalid")

}

func TestValidateInput_EmptyDocumentPath(t *testing.T) {
	input := RunDocumentPluginInput{}
	input.DocumentType = LocalPathType

	result, err := validateInput(&input)

	assert.False(t, result)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "Document Path must be provided")

}

func TestParseAndValidateInput_NoInput(t *testing.T) {
	rawPluginInput := ""

	_, err := parseAndValidateInput(rawPluginInput)

	assert.Error(t, err)
}

func TestDownloadDocumentFromSSM_ARNName(t *testing.T) {
	conf := createStubConfiguration("orch", "bucket", "prefix", "1234-1234-1234", "directory")
	input := RunDocumentPluginInput{}
	input.DocumentType = SSMDocumentType
	input.DocumentPath = "arn:aws:ssm:us-east-1:1234567890:document/mySharedDocument:10"

	execMock := NewExecMock()
	fileMock := filemock.FileSystemMock{}
	ssmMock := ssmsvc.NewMockDefault()

	content := "content"
	docResponse := ssm.GetDocumentOutput{
		Content: &content,
	}

	ssmMock.On("GetDocument", contextMock.Log(), "arn:aws:ssm:us-east-1:1234567890:document/mySharedDocument", "10").Return(&docResponse, nil)
	fileMock.On("MakeDirs", "orch/downloads").Return(nil)
	fileMock.On("WriteFile", "orch/downloads/mySharedDocument.json", content).Return(nil)
	p := Plugin{
		filesys: fileMock,
		execDoc: execMock,
		ssmSvc:  ssmMock,
	}

	pathToFile, err := p.downloadDocumentFromSSM(contextMock.Log(), conf, &input)

	assert.NoError(t, err)
	ssmMock.AssertExpectations(t)
	execMock.AssertExpectations(t)
	fileMock.AssertExpectations(t)
	assert.Equal(t, pathToFile, "orch/downloads/mySharedDocument.json")

}

func createStubExecutionDepth(depth int) *ExecutePluginDepth {
	currentDepth := ExecutePluginDepth{}
	currentDepth.executeCommandDepth = depth

	return &currentDepth
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

func createMockCancelFlag() task.CancelFlag {
	mockCancelFlag := new(task.MockCancelFlag)
	// Setup mocks
	mockCancelFlag.On("Canceled").Return(false)
	mockCancelFlag.On("ShutDown").Return(false)
	mockCancelFlag.On("Wait").Return(false).After(100 * time.Millisecond)

	return mockCancelFlag
}

func loadFile(t *testing.T, fileName string) (result []byte) {
	result, err := ioutil.ReadFile(fileName)
	if err != nil {
		t.Fatal(err)
	}
	return
}
