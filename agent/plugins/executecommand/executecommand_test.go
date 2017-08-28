// Copyright 2016 Amazon.com, Inc. or its affiliates. All Rights Reserved.
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

// Package execute implements the aws:execute plugin
// test_execute contains stub implementations
package executecommand

import (
	"fmt"
	"testing"
	"time"

	"github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/aws/amazon-ssm-agent/agent/contracts"
	"github.com/aws/amazon-ssm-agent/agent/docmanager/model"
	"github.com/aws/amazon-ssm-agent/agent/log"
	executemock "github.com/aws/amazon-ssm-agent/agent/plugins/executecommand/executor/mock"
	"github.com/aws/amazon-ssm-agent/agent/plugins/executecommand/filemanager"
	filemock "github.com/aws/amazon-ssm-agent/agent/plugins/executecommand/filemanager/mock"
	"github.com/aws/amazon-ssm-agent/agent/plugins/executecommand/remoteresource"
	resourcemock "github.com/aws/amazon-ssm-agent/agent/plugins/executecommand/remoteresource/mock"
	"github.com/aws/amazon-ssm-agent/agent/task"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

var logger = log.NewMockLog()
var contextMock context.T = context.NewMockDefault()
var executeFileMock = filemock.FileSystemMock{}
var executeResourceMock = resourcemock.RemoteResourceMock{}

func TestNewRemoteResource_InvalidLocationType(t *testing.T) {

	var mockLocationInfo string
	remoteresource, err := newRemoteResource(logger, "invalid", mockLocationInfo)

	assert.Nil(t, remoteresource)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "Invalid Location type")

}

func TestNewRemoteResource_Github(t *testing.T) {

	locationInfo := `{
		"owner" : "test-owner",
		"repository" :	 "test-repo"
		}`
	remoteresource, err := newRemoteResource(logger, "Github", locationInfo)

	assert.NotNil(t, remoteresource)
	assert.NoError(t, err)

}

func TestExecutePlugin_GetResource(t *testing.T) {

	remoteResourceMock := resourcemock.RemoteResourceMock{}
	fileMock := filemock.FileSystemMock{}

	resource := createStubResourceInfoScript()

	input := ExecutePluginInput{
		EntireDirectory: "false",
	}
	config := createStubConfiguration("orch", "bucket", "prefix", "1234-1234-1234", "directory")

	remoteResourceMock.On("ValidateLocationInfo").Return(true, nil)
	remoteResourceMock.On("Download", logger, fileMock, false, mock.Anything).Return(nil).Once()
	remoteResourceMock.On("PopulateResourceInfo", logger, mock.Anything, mock.Anything).Return(resource)

	p := executeImpl{
		filesys: fileMock,
	}

	outResource, err := p.GetResource(logger, &input, config, remoteResourceMock)

	assert.NoError(t, err)
	assert.Equal(t, resource, outResource)
	remoteResourceMock.AssertExpectations(t)
}

func TestExecutePlugin_GetResourceBadDirectory(t *testing.T) {

	remoteResourceMock := resourcemock.RemoteResourceMock{}
	fileMock := filemock.FileSystemMock{}

	resource := createStubResourceInfoScript()
	config := createStubConfiguration("orch", "bucket", "prefix", "1234-1234-1234", "directory")

	input := ExecutePluginInput{
		EntireDirectory: "",
	}

	remoteResourceMock.On("ValidateLocationInfo").Return(true, nil)
	remoteResourceMock.On("Download", logger, fileMock, false, mock.Anything).Return(nil).Once()
	remoteResourceMock.On("PopulateResourceInfo", logger, mock.Anything, mock.Anything).Return(resource)

	p := executeImpl{
		filesys: fileMock,
	}

	_, err := p.GetResource(logger, &input, config, remoteResourceMock)

	assert.Error(t, err)
}

func TestExecutePlugin_GetResourceBadLocationInfo(t *testing.T) {

	fileMock := filemock.FileSystemMock{}
	locationInfo := `{
		"owner" = "test-owner",
		"repository" = "test-repo"
		}`
	remoteResource, err := newRemoteResource(logger, "Github", locationInfo)
	config := createStubConfiguration("orch", "bucket", "prefix", "1234-1234-1234", "directory")

	input := ExecutePluginInput{}

	p := executeImpl{
		filesys: fileMock,
	}

	_, err = p.GetResource(logger, &input, config, remoteResource)

	assert.Error(t, err)
}

func TestValidateInput_UnsupportedLocationType(t *testing.T) {

	input := ExecutePluginInput{}
	input.LocationType = "unknown"

	validateInput(&input)

	result, err := validateInput(&input)

	assert.False(t, result)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "Unsupported location type")
}

func TestValidateParameters_DocumentWithScriptArguments(t *testing.T) {
	var scriptArgs []string
	scriptArgs = append(scriptArgs, "arg1")
	input := ExecutePluginInput{}
	input.ScriptArguments = scriptArgs

	resourceInfo := remoteresource.ResourceInfo{}
	resourceInfo.TypeOfResource = remoteresource.Document

	result, err := validateParameters(&input, resourceInfo)

	assert.False(t, result)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "Document type of resource cannot specify script type parameters")
}

func TestValidateParameters_ScriptWithDocumentParameters(t *testing.T) {

	input := ExecutePluginInput{}
	input.DocumentParameters = `{"hello" = "world"}`

	resourceInfo := remoteresource.ResourceInfo{}
	resourceInfo.TypeOfResource = remoteresource.Script

	result, err := validateParameters(&input, resourceInfo)

	assert.False(t, result)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "Script type of resource cannot have document parameters specified")
}

//TODO Add a test for DocumentWithDocumentParameters and ScriptWithScriptArguments
func TestValidateParameters_Document(t *testing.T) {

	input := ExecutePluginInput{}

	resourceInfo := remoteresource.ResourceInfo{}
	resourceInfo.TypeOfResource = remoteresource.Document

	result, err := validateParameters(&input, resourceInfo)

	assert.True(t, result)
	assert.NoError(t, err)
}

func TestValidateParameters_Script(t *testing.T) {
	input := ExecutePluginInput{}

	resourceInfo := remoteresource.ResourceInfo{}
	resourceInfo.TypeOfResource = remoteresource.Script

	result, err := validateParameters(&input, resourceInfo)

	assert.True(t, result)
	assert.NoError(t, err)
}

func TestValidateInput_UnknownLocationType(t *testing.T) {

	input := ExecutePluginInput{}

	validateInput(&input)

	result, err := validateInput(&input)

	assert.False(t, result)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "Location Type must be specified")
}

func TestValidateInput_NoLocationInfo(t *testing.T) {

	input := ExecutePluginInput{}
	input.LocationType = "S3"

	validateInput(&input)

	result, err := validateInput(&input)

	assert.False(t, result)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "Location Information must be specified")
}

func TestName(t *testing.T) {
	assert.Equal(t, "aws:executeCommand", Name())
}

func TestExecutePlugin_PrepareDocumentForExecution(t *testing.T) {

	execMock := executemock.NewExecMock()
	resource := createStubResourceInfoDocument()
	fileMock := filemock.FileSystemMock{}

	plugin := model.PluginState{}
	plugins := []model.PluginState{plugin}

	parameters := make(map[string]interface{})
	conf := createStubConfiguration("orch", "bucket", "prefix", "1234-1234-1234", "directory")

	fileMock.On("ReadFile", mock.Anything).Return("content", nil)
	execMock.On("ParseDocument", logger, ".json", []byte("content"), conf.OrchestrationDirectory, conf.OutputS3BucketName, conf.OutputS3KeyPrefix, conf.MessageId, conf.PluginID, conf.DefaultWorkingDirectory, parameters).Return(plugins, nil)

	p := executeImpl{
		filesys: fileMock,
		exec:    execMock,
	}

	_, err := p.PrepareDocumentForExecution(logger, resource, conf, "")

	assert.NoError(t, err)
	fileMock.AssertExpectations(t)
	execMock.AssertExpectations(t)
}

func TestExecutePlugin_PrepareDocumentForExecutionFail(t *testing.T) {

	execMock := executemock.NewExecMock()
	resource := createStubResourceInfoDocument()
	localFileMock := filemock.FileSystemMock{}

	conf := createStubConfiguration("orch", "bucket", "prefix", "1234-1234-1234", "directory")

	localFileMock.On("ReadFile", mock.Anything).Return("", fmt.Errorf("File is empty!"))

	p := executeImpl{
		filesys: localFileMock,
		exec:    execMock,
	}

	_, err := p.PrepareDocumentForExecution(logger, resource, conf, "")

	assert.Error(t, err)
	assert.Equal(t, fmt.Errorf("File is empty!"), err)
	localFileMock.AssertExpectations(t)
}

func TestExecuteImpl_PrepareDocumentForExecutionParametersJSON(t *testing.T) {
	execMock := executemock.NewExecMock()
	resource := createStubResourceInfoDocument()
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

	fileMock.On("ReadFile", mock.Anything).Return("content", nil)
	execMock.On("ParseDocument", logger, ".json", []byte("content"), conf.OrchestrationDirectory, conf.OutputS3BucketName, conf.OutputS3KeyPrefix, conf.MessageId, conf.PluginID, conf.DefaultWorkingDirectory, parameters).Return(plugins, nil)

	p := executeImpl{
		filesys: fileMock,
		exec:    execMock,
	}

	_, err := p.PrepareDocumentForExecution(logger, resource, conf, params)

	assert.NoError(t, err)
	fileMock.AssertExpectations(t)
	execMock.AssertExpectations(t)
}

func TestExecuteImpl_PrepareDocumentForExecutionParametersYAML(t *testing.T) {
	execMock := executemock.NewExecMock()
	resource := remoteresource.ResourceInfo{
		LocalDestinationPath: "destination",
		EntireDir:            false,
		TypeOfResource:       remoteresource.Document,
		StarterFile:          "file",
		ResourceExtension:    ".yaml",
	}
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

	fileMock.On("ReadFile", mock.Anything).Return("content", nil)
	execMock.On("ParseDocument", logger, ".yaml", []byte("content"), conf.OrchestrationDirectory, conf.OutputS3BucketName, conf.OutputS3KeyPrefix, conf.MessageId, conf.PluginID, conf.DefaultWorkingDirectory, parameters).Return(plugins, nil)

	p := executeImpl{
		filesys: fileMock,
		exec:    execMock,
	}

	_, err := p.PrepareDocumentForExecution(logger, resource, conf, params)

	assert.NoError(t, err)
	fileMock.AssertExpectations(t)
	execMock.AssertExpectations(t)
}

func TestParseAndValidateInput_NoInput(t *testing.T) {
	rawPluginInput := ""

	_, err := parseAndValidateInput(rawPluginInput)

	assert.Error(t, err)
}

func TestPlugin_ExecuteScript(t *testing.T) {

	filesysmock := filemock.FileSystemMock{}
	mockplugin := MockDefaultPlugin{}
	execMock := executemock.NewExecMock()
	pluginResult := contracts.PluginOutput{ExitCode: 0, Status: "", Stdout: "", Stderr: ""}
	resourceInfo := remoteresource.ResourceInfo{
		LocalDestinationPath: "destination",
		StarterFile:          "file",
		ResourceExtension:    ".ps",
		TypeOfResource:       remoteresource.Script,
	}
	var args []string
	conf := createSimpleConfigWithProperties(createStubPluginInputGithub())
	execMock.On("ExecuteScript", contextMock.Log(), resourceInfo.LocalDestinationPath, args, 3600, &pluginResult).Return()
	filesysmock.On("MakeDirs", mock.Anything).Return(nil)
	filesysmock.On("WriteFile", mock.Anything, mock.Anything).Return(nil).Twice()
	filesysmock.On("Walk", mock.Anything, mock.AnythingOfType("filepath.WalkFunc")).Return(nil)

	mockplugin.On("UploadOutputToS3Bucket", contextMock.Log(), conf.PluginID, conf.OrchestrationDirectory,
		conf.OutputS3BucketName, conf.OutputS3KeyPrefix, false, conf.OrchestrationDirectory,
		pluginResult.Stdout, pluginResult.Stderr).Return([]string{})
	manager := executeImpl{
		filesys: executeFileMock,
		exec:    execMock,
	}
	p := &Plugin{
		remoteResourceCreator: NewRemoteResourceMockScript,
		pluginManager:         manager,
	}
	p.ExecuteUploadOutputToS3Bucket = mockplugin.UploadOutputToS3Bucket

	result := p.execute(contextMock, conf, createMockCancelFlag(), filesysmock)

	assert.Equal(t, 0, result.Code)
	executeResourceMock.AssertExpectations(t)
	execMock.AssertExpectations(t)
	filesysmock.AssertExpectations(t)
	mockplugin.AssertExpectations(t)
}

func TestPlugin_ExecuteDocument(t *testing.T) {

	docMock := executemock.NewExecMock()
	mockplugin := MockDefaultPlugin{}
	plugin := model.PluginState{}
	plugins := []model.PluginState{plugin}
	filesysmock := filemock.FileSystemMock{}
	pluginResult := contracts.PluginOutput{ExitCode: 0, Status: "", Stdout: "", Stderr: ""}
	conf := createSimpleConfigWithProperties(createStubPluginInputGithub())
	parameters := make(map[string]interface{})
	executeFileMock.On("ReadFile", mock.Anything).Return("content", nil).Once()
	cancelFlag := createMockCancelFlag()
	docMock.On("ParseDocument", contextMock.Log(), ".json", []byte("content"), mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, parameters).Return(plugins, nil)
	docMock.On("ExecuteDocument", contextMock, mock.AnythingOfType("[]model.PluginState"), mock.Anything, mock.Anything, &pluginResult).Return()

	filesysmock.On("MakeDirs", mock.Anything).Return(nil)
	filesysmock.On("WriteFile", mock.Anything, mock.Anything).Return(nil).Twice()
	filesysmock.On("Walk", mock.Anything, mock.AnythingOfType("filepath.WalkFunc")).Return(nil)

	mockplugin.On("UploadOutputToS3Bucket", contextMock.Log(), conf.PluginID, conf.OrchestrationDirectory,
		conf.OutputS3BucketName, conf.OutputS3KeyPrefix, false, conf.OrchestrationDirectory,
		pluginResult.Stdout, pluginResult.Stderr).Return([]string{})
	manager := executeImpl{
		filesys: executeFileMock,
		exec:    docMock,
	}
	p := &Plugin{
		remoteResourceCreator: NewRemoteResourceMockDoc,
		pluginManager:         manager,
	}
	p.ExecuteUploadOutputToS3Bucket = mockplugin.UploadOutputToS3Bucket
	result := p.execute(contextMock, conf, cancelFlag, filesysmock)

	executeFileMock.AssertExpectations(t)
	executeResourceMock.AssertExpectations(t)
	docMock.AssertExpectations(t)
	filesysmock.AssertExpectations(t)
	mockplugin.AssertExpectations(t)

	assert.Equal(t, 0, result.Code)
	assert.Equal(t, 0, pluginResult.ExitCode)
}

func TestPlugin_ExecuteMaxDepthExceeded(t *testing.T) {

	docMock := executemock.NewExecMock()
	mockplugin := MockDefaultPlugin{}
	mockplugin.On("UploadOutputToS3Bucket", contextMock.Log(), mock.Anything, mock.Anything,
		mock.Anything, mock.Anything, false, mock.Anything,
		mock.Anything, mock.Anything).Return([]string{})

	config := createSimpleConfigWithProperties(createStubPluginInputGithub())
	var executionDepth interface{}
	executionDepth = createStubExecutionDepth(4)
	config.Settings = executionDepth

	filesysdep := filemanager.FileSystemImpl{}
	manager := executeImpl{
		filesys: executeFileMock,
		exec:    docMock,
	}
	p := &Plugin{
		remoteResourceCreator: NewRemoteResourceMockDoc,
		pluginManager:         manager,
	}
	p.ExecuteUploadOutputToS3Bucket = mockplugin.UploadOutputToS3Bucket

	result := p.execute(contextMock, config, createMockCancelFlag(), filesysdep)

	executeResourceMock.AssertExpectations(t)
	mockplugin.AssertExpectations(t)

	assert.Equal(t, 1, result.Code)
	assert.Contains(t, result.Output, "Maximum depth for document execution exceeded.")
}

// Mock and stub functions
func createStubResourceInfoScript() remoteresource.ResourceInfo {
	return remoteresource.ResourceInfo{
		LocalDestinationPath: "destination",
		EntireDir:            false,
		TypeOfResource:       remoteresource.Script,
		StarterFile:          "file",
		ResourceExtension:    ".ps",
	}
}

func createStubResourceInfoDocument() remoteresource.ResourceInfo {
	return remoteresource.ResourceInfo{
		LocalDestinationPath: "destination",
		EntireDir:            false,
		TypeOfResource:       remoteresource.Document,
		StarterFile:          "file",
		ResourceExtension:    ".json",
	}
}

func createStubConfiguration(orch, bucket, prefix, message, dir string) contracts.Configuration {
	return contracts.Configuration{
		OrchestrationDirectory:  orch,
		OutputS3BucketName:      bucket,
		OutputS3KeyPrefix:       prefix,
		MessageId:               message,
		PluginID:                "aws-executecmmand",
		DefaultWorkingDirectory: dir,
	}
}

func createSimpleConfigWithProperties(info *ExecutePluginInput) contracts.Configuration {
	config := contracts.Configuration{}

	var rawPluginInput interface{}
	rawPluginInput = info
	config.Properties = rawPluginInput
	config.OrchestrationDirectory = "orch"
	config.PluginID = "plugin"
	config.OutputS3KeyPrefix = ""
	config.OutputS3BucketName = ""

	return config
}

func createMockCancelFlag() task.CancelFlag {
	mockCancelFlag := new(task.MockCancelFlag)
	// Setup mocks
	mockCancelFlag.On("Canceled").Return(false)
	mockCancelFlag.On("ShutDown").Return(false)
	mockCancelFlag.On("Wait").Return(false).After(100 * time.Millisecond)

	return mockCancelFlag
}

func createStubPluginInputGithub() *ExecutePluginInput {
	input := ExecutePluginInput{}

	input.LocationType = "Github"
	input.LocationInfo = `{
		"owner" : "test-owner",
		"repository" :	 "test-repo",
		"path" : "path"
		}`
	input.EntireDirectory = "false"
	input.ScriptArguments = nil

	return &input
}

func createStubExecutionDepth(depth int) *ExecutePluginDepth {
	currentDepth := ExecutePluginDepth{}
	currentDepth.executeCommandDepth = depth

	return &currentDepth
}

func NewRemoteResourceMockScript(log log.T, locationtype, locationInfo string) (remoteresource.RemoteResource, error) {
	resource := createStubResourceInfoScript()

	executeResourceMock.On("ValidateLocationInfo").Return(true, nil).Once()
	executeResourceMock.On("Download", contextMock.Log(), executeFileMock, false, mock.Anything).Return(nil).Once()
	executeResourceMock.On("PopulateResourceInfo", contextMock.Log(), mock.Anything, mock.Anything).Return(resource).Once()

	return executeResourceMock, nil
}

func NewRemoteResourceMockDoc(log log.T, locationtype, locationInfo string) (remoteresource.RemoteResource, error) {
	resource := createStubResourceInfoDocument()

	executeResourceMock.On("ValidateLocationInfo").Return(true, nil).Once()
	executeResourceMock.On("Download", contextMock.Log(), executeFileMock, false, mock.Anything).Return(nil).Once()
	executeResourceMock.On("PopulateResourceInfo", contextMock.Log(), mock.Anything, mock.Anything).Return(resource).Once()

	return executeResourceMock, nil
}

// MockDefaultPlugin mocks the default plugin.
type MockDefaultPlugin struct {
	mock.Mock
}

// UploadOutputToS3Bucket is a mocked method that just returns what mock tells it to.
func (m *MockDefaultPlugin) UploadOutputToS3Bucket(log log.T, pluginID string, orchestrationDir string, outputS3BucketName string, outputS3KeyPrefix string, useTempDirectory bool, tempDir string, Stdout string, Stderr string) []string {
	args := m.Called(log, pluginID, orchestrationDir, outputS3BucketName, outputS3KeyPrefix, useTempDirectory, tempDir, Stdout, Stderr)
	log.Infof("args are %v", args)
	return args.Get(0).([]string)
}
