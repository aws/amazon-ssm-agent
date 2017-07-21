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
	"github.com/aws/amazon-ssm-agent/agent/framework/runpluginutil"
	"github.com/aws/amazon-ssm-agent/agent/log"
	docmock "github.com/aws/amazon-ssm-agent/agent/plugins/executecommand/document/mock"
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
	assert.Equal(t, "Invalid Location type.", err.Error())

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

	p := executePlugin{
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

	p := executePlugin{
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

	p := executePlugin{
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

	execMock := docmock.NewExecMock()
	resource := createStubResourceInfoScript()
	fileMock := filemock.FileSystemMock{}

	plugin := model.PluginState{}
	plugins := []model.PluginState{plugin}

	parameters := make(map[string]interface{})
	conf := createStubConfiguration("orch", "bucket", "prefix", "1234-1234-1234", "directory")

	fileMock.On("ReadFile", mock.Anything).Return("content", nil)
	execMock.On("ParseDocument", logger, []byte("content"), conf.OrchestrationDirectory, conf.OutputS3BucketName, conf.OutputS3KeyPrefix, conf.MessageId, conf.PluginID, conf.DefaultWorkingDirectory, parameters).Return(plugins, nil)

	p := executePlugin{
		filesys: fileMock,
		doc:     execMock,
	}

	_, err := p.PrepareDocumentForExecution(logger, resource, conf, "")

	assert.NoError(t, err)
	fileMock.AssertExpectations(t)
	execMock.AssertExpectations(t)
}

func TestExecutePlugin_PrepareDocumentForExecutionFail(t *testing.T) {

	execMock := docmock.NewExecMock()
	resource := createStubResourceInfoScript()
	localFileMock := filemock.FileSystemMock{}

	conf := createStubConfiguration("orch", "bucket", "prefix", "1234-1234-1234", "directory")

	localFileMock.On("ReadFile", mock.Anything).Return("", fmt.Errorf("File is empty!"))

	p := executePlugin{
		filesys: localFileMock,
		doc:     execMock,
	}

	_, err := p.PrepareDocumentForExecution(logger, resource, conf, "")

	assert.Error(t, err)
	assert.Equal(t, fmt.Errorf("File is empty!"), err)
	localFileMock.AssertExpectations(t)
}

func TestParseAndValidateInput_NoInput(t *testing.T) {
	rawPluginInput := ""

	_, err := parseAndValidateInput(rawPluginInput)

	assert.Error(t, err)
}

func TestPlugin_ExecuteScript(t *testing.T) {

	filesysdep := filemanager.FileSystemImpl{}
	manager := executePlugin{
		filesys: executeFileMock,
		doc:     docmock.NewExecMock(),
	}
	plugin := &Plugin{
		executeCommandDepth:   3,
		remoteResourceCreator: NewRemoteResourceMockScript,
		pluginManager:         manager,
	}
	result := plugin.execute(contextMock, createSimpleConfigWithProperties(createStubPluginInputGithub()), createMockCancelFlag(), runpluginutil.PluginRunner{}, filesysdep)

	assert.Equal(t, 0, result.Code)
	executeResourceMock.AssertExpectations(t)
}

func TestPlugin_ExecuteDocument(t *testing.T) {

	docMock := docmock.NewExecMock()
	plugin := model.PluginState{}
	plugins := []model.PluginState{plugin}
	filesysdep := filemanager.FileSystemImpl{}
	conf := createSimpleConfigWithProperties(createStubPluginInputGithub())
	parameters := make(map[string]interface{})
	executeFileMock.On("ReadFile", mock.Anything).Return("content", nil).Once()
	docMock.On("ParseDocument", contextMock.Log(), []byte("content"), mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, parameters).Return(plugins, nil)

	manager := executePlugin{
		filesys: executeFileMock,
		doc:     docMock,
	}
	p := &Plugin{
		executeCommandDepth:   3,
		remoteResourceCreator: NewRemoteResourceMockDoc,
		pluginManager:         manager,
	}
	result := p.execute(contextMock, conf, createMockCancelFlag(), runpluginutil.PluginRunner{}, filesysdep)

	executeFileMock.AssertExpectations(t)
	executeResourceMock.AssertExpectations(t)

	assert.Equal(t, 0, result.Code)
}

func TestPlugin_ExecuteMaxDepthExceeded(t *testing.T) {

	plugin := &Plugin{
		executeCommandDepth: 4,
	}
	filesysdep := filemanager.FileSystemImpl{}
	result := plugin.execute(contextMock, createSimpleConfigWithProperties(createStubPluginInputGithub()), createMockCancelFlag(), runpluginutil.PluginRunner{}, filesysdep)

	assert.Equal(t, 1, result.Code)
	assert.Contains(t, result.Output, "Maximum depth for document execution exceeded.")
}

func createStubResourceInfoScript() remoteresource.ResourceInfo {
	return remoteresource.ResourceInfo{
		LocalDestinationPath: "destination",
		EntireDir:            false,
		TypeOfResource:       remoteresource.Script,
		StarterFile:          "file",
	}
}

func createStubResourceInfoDocument() remoteresource.ResourceInfo {
	return remoteresource.ResourceInfo{
		LocalDestinationPath: "destination",
		EntireDir:            false,
		TypeOfResource:       remoteresource.Document,
		StarterFile:          "file",
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

	return &input
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
