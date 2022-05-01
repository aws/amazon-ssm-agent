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

// Package downloadcontent implements the aws:downloadContent plugin
package downloadcontent

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/aws/amazon-ssm-agent/agent/contracts"
	filemock "github.com/aws/amazon-ssm-agent/agent/fileutil/filemanager/mock"
	iohandlermocks "github.com/aws/amazon-ssm-agent/agent/framework/processor/executer/iohandler/mock"
	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/amazon-ssm-agent/agent/plugins/downloadcontent/remoteresource"
	resourcemock "github.com/aws/amazon-ssm-agent/agent/plugins/downloadcontent/remoteresource/mock"
	"github.com/aws/amazon-ssm-agent/agent/task"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

var logger = log.NewMockLog()
var copyContentFileMock = filemock.FileSystemMock{}
var copyContentResourceMock = resourcemock.RemoteResourceMock{}
var contextMock = context.NewMockDefault()

func TestNewRemoteResource_InvalidLocationType(t *testing.T) {

	var mockLocationInfo string
	remoteresource, err := newRemoteResource(contextMock, "invalid", mockLocationInfo)

	assert.Nil(t, remoteresource)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "Invalid SourceType")

}

func TestNewRemoteResource_Git(t *testing.T) {
	locationInfo := `{
		"repository" :	 "test-repo"
	}`

	remoteResource, err := newRemoteResource(contextMock, "Git", locationInfo)
	assert.NotNil(t, remoteResource)
	assert.NoError(t, err)
}

func TestNewRemoteResource_HTTP(t *testing.T) {
	locationInfo := `{
		"url" :	 "http://"
	}`

	remoteResource, err := newRemoteResource(contextMock, "HTTP", locationInfo)
	assert.NotNil(t, remoteResource)
	assert.NoError(t, err)
}

func TestNewRemoteResource_Github(t *testing.T) {
	locationInfo := `{
		"owner" : "test-owner",
		"repository" :	 "test-repo"
		}`
	remoteresource, err := newRemoteResource(contextMock, "GitHub", locationInfo)

	assert.NotNil(t, remoteresource)
	assert.NoError(t, err)
}

func TestNewRemoteResource_S3(t *testing.T) {

	locationInfo := `{
		"path" : "https://s3.amazonaws.com/test-bucket/fake-key/"
		}`
	remoteresource, err := newRemoteResource(contextMock, "S3", locationInfo)

	assert.NotNil(t, remoteresource)
	assert.NoError(t, err)

}

func TestNewRemoteResource_SSMDocument(t *testing.T) {

	locationInfo := `{
		"name" : "doc-name",
		"version" : "1"
		}`
	remoteresource, err := newRemoteResource(contextMock, "SSMDocument", locationInfo)

	assert.NotNil(t, remoteresource)
	assert.NoError(t, err)

}

func TestNewPlugin_RunCopyContent(t *testing.T) {

	fileMock := filemock.FileSystemMock{}
	mockIOHandler := new(iohandlermocks.MockIOHandler)

	input := DownloadContentPlugin{
		SourceType:      "Github",
		DestinationPath: "destination",
	}
	config := createStubConfiguration("orch", "bucket", "prefix", "1234-1234-1234", "directory")

	p := Plugin{
		context:               context.NewMockDefault(),
		remoteResourceCreator: fakeRemoteResource,
		filesys:               &fileMock,
	}
	mockIOHandler.On("AppendInfof", mock.Anything, mock.Anything).Return()
	mockIOHandler.On("MarkAsSucceeded").Return()

	SetPermission = stubChmod
	p.runCopyContent(logger, &input, config, mockIOHandler)

	copyContentResourceMock.AssertExpectations(t)
	fileMock.AssertExpectations(t)
	mockIOHandler.AssertExpectations(t)
}

func TestNewPlugin_RunCopyContent_absPathDestinationDir(t *testing.T) {
	fileMock := filemock.FileSystemMock{}
	mockIOHandler := new(iohandlermocks.MockIOHandler)

	input := DownloadContentPlugin{
		SourceType:      "Github",
		DestinationPath: filepath.Join("var", "temp", "fake-dir"),
	}
	config := createStubConfiguration("orch", "bucket", "prefix", "1234-1234-1234", "directory")

	p := Plugin{
		context:               context.NewMockDefault(),
		remoteResourceCreator: absoluteDestinationDirRemoteResource,
		filesys:               &fileMock,
	}
	mockIOHandler.On("AppendInfof", mock.Anything, mock.Anything).Return()
	mockIOHandler.On("MarkAsSucceeded").Return()

	SetPermission = stubChmod
	p.runCopyContent(logger, &input, config, mockIOHandler)

	copyContentResourceMock.AssertExpectations(t)
	fileMock.AssertExpectations(t)
}

func TestNewPlugin_RunCopyContent_relativeDirDestinationPath(t *testing.T) {

	fileMock := filemock.FileSystemMock{}
	mockIOHandler := new(iohandlermocks.MockIOHandler)

	input := DownloadContentPlugin{
		SourceType:      "Github",
		DestinationPath: filepath.Join("temp", "fake-dir"),
	}
	config := createStubConfiguration(filepath.Join("orch", "aws-copyContent"), "bucket", "prefix", "1234-1234-1234", "directory")

	p := Plugin{
		context:               context.NewMockDefault(),
		remoteResourceCreator: relativeDestinationDirRemoteResource,
		filesys:               &fileMock,
	}
	mockIOHandler.On("AppendInfof", mock.Anything, mock.Anything).Return()
	mockIOHandler.On("MarkAsSucceeded").Return()

	SetPermission = stubChmod
	p.runCopyContent(logger, &input, config, mockIOHandler)

	copyContentResourceMock.AssertExpectations(t)
	fileMock.AssertExpectations(t)
	mockIOHandler.AssertExpectations(t)
}

func Test_RunCopyContentBadLocationInfo(t *testing.T) {

	fileMock := filemock.FileSystemMock{}
	mockIOHandler := new(iohandlermocks.MockIOHandler)

	locationInfo := `{
		"owner" = "test-owner",
		"repository" = "test-repo"
		}`

	config := createStubConfiguration("orch", "bucket", "prefix", "1234-1234-1234", "directory")

	input := DownloadContentPlugin{
		SourceType:      "GitHub",
		SourceInfo:      locationInfo,
		DestinationPath: "",
	}
	p := Plugin{
		context:               context.NewMockDefault(),
		remoteResourceCreator: newRemoteResource,
		filesys:               &fileMock,
	}
	mockIOHandler.On("MarkAsFailed", mock.Anything).Return()

	p.runCopyContent(logger, &input, config, mockIOHandler)

	fileMock.AssertExpectations(t)
	mockIOHandler.AssertExpectations(t)
}

func executePlugin(t *testing.T, input *DownloadContentPlugin, destPath string) {
	mockplugin := MockDefaultPlugin{}
	mockIOHandler := new(iohandlermocks.MockIOHandler)

	conf := createSimpleConfigWithProperties(input)
	cancelFlag := createMockCancelFlag()

	var copyContentResourceMock = resourcemock.RemoteResourceMock{}
	var copyContentFileMock = filemock.FileSystemMock{}

	mockIOHandler.On("AppendInfof", mock.Anything, mock.Anything).Return()
	mockIOHandler.On("MarkAsSucceeded").Return()

	mockRemoteResource := func(context context.T, locationtype, locationInfo string) (remoteresource.RemoteResource, error) {
		copyContentResourceMock.On("ValidateLocationInfo").Return(true, nil).Once()
		copyContentResourceMock.On("DownloadRemoteResource", copyContentFileMock, destPath).Return(nil, resourcemock.NewEmptyDownloadResult()).Once()
		return copyContentResourceMock, nil
	}

	p := &Plugin{
		context:               context.NewMockDefault(),
		remoteResourceCreator: mockRemoteResource,
		filesys:               copyContentFileMock,
	}

	SetPermission = stubChmod
	p.execute(conf, cancelFlag, mockIOHandler)

	copyContentFileMock.AssertExpectations(t)
	copyContentResourceMock.AssertExpectations(t)
	mockplugin.AssertExpectations(t)
	mockIOHandler.AssertExpectations(t)
}

func TestPlugin_ExecuteGitFile(t *testing.T) {
	input := DownloadContentPlugin{}
	input.SourceType = "Git"
	input.SourceInfo = `{
		"repository" : "test-repo"
	}`
	input.DestinationPath = os.TempDir()

	executePlugin(t, &input, os.TempDir())
}

func TestPlugin_ExecuteGitHubFile(t *testing.T) {
	input := DownloadContentPlugin{}
	input.SourceType = "GitHub"
	input.SourceInfo = `{
		"owner" : "test-owner",
		"repository" :	 "test-repo",
		"path" : "path"
		}`
	input.DestinationPath = "destination"

	executePlugin(t, &input, filepath.Join("orch", "downloads", "destination"))
}

func TestPlugin_ExecuteS3File(t *testing.T) {
	input := DownloadContentPlugin{}
	input.SourceType = "S3"
	input.SourceInfo = `{
		"path" : "https://s3.amazonaws.com/fake-bucket/fake-key/filename.ps"
	}`
	input.DestinationPath = filepath.Join(rootAbsPath, "tmp", "destination")

	executePlugin(t, &input, filepath.Join(rootAbsPath, "tmp", "destination"))
}

func TestPlugin_ExecuteSSMDoc(t *testing.T) {
	input := DownloadContentPlugin{}
	input.SourceType = "SSMDocument"
	input.SourceInfo = `{
		"name" : "arn:aws:ssm:us-east-1:1234567890:document/mySharedDocument:10"
	}`
	input.DestinationPath = filepath.Join(rootAbsPath, "tmp", "destination")

	executePlugin(t, &input, filepath.Join(rootAbsPath, "tmp", "destination"))
}

func TestPlugin_ExecuteSSMDocError(t *testing.T) {

	mockplugin := MockDefaultPlugin{}
	mockIOHandler := new(iohandlermocks.MockIOHandler)

	input := DownloadContentPlugin{}

	input.SourceType = "SSMDocument"
	input.SourceInfo = `{
	"name" : ":10"
		}`
	input.DestinationPath = filepath.Join("var", "tmp", "destination")
	conf := createSimpleConfigWithProperties(&input)
	cancelFlag := createMockCancelFlag()

	var ssmDoccopyContentResourceMock = resourcemock.RemoteResourceMock{}
	var ssmDocCopyContentFileMock = filemock.FileSystemMock{}
	mockIOHandler.On("MarkAsFailed", mock.Anything).Return()

	ssmDocMockRemoteResource := func(context context.T, locationtype, locationInfo string) (remoteresource.RemoteResource, error) {
		ssmDoccopyContentResourceMock.On("DownloadRemoteResource", ssmDocCopyContentFileMock, filepath.Join("orch", "downloads", "var", "tmp", "destination")).Return(errors.New("Document name must be specified"), (*remoteresource.DownloadResult)(nil)).Once()
		ssmDoccopyContentResourceMock.On("ValidateLocationInfo").Return(true, nil).Once()
		return ssmDoccopyContentResourceMock, nil
	}
	p := &Plugin{
		context:               context.NewMockDefault(),
		remoteResourceCreator: ssmDocMockRemoteResource,
		filesys:               ssmDocCopyContentFileMock,
	}
	SetPermission = stubChmod
	p.execute(conf, cancelFlag, mockIOHandler)

	ssmDocCopyContentFileMock.AssertExpectations(t)
	ssmDoccopyContentResourceMock.AssertExpectations(t)
	mockplugin.AssertExpectations(t)
	mockIOHandler.AssertExpectations(t)
}

func TestValidateInput_UnsupportedLocationType(t *testing.T) {

	input := DownloadContentPlugin{}
	input.SourceType = "unknown"

	validateInput(&input)

	result, err := validateInput(&input)

	assert.False(t, result)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "Unsupported source type")
}

func TestValidateInput_UnknownSourceType(t *testing.T) {

	input := DownloadContentPlugin{}

	validateInput(&input)

	result, err := validateInput(&input)

	assert.False(t, result)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "SourceType must be specified")
}

func TestValidateInput_NoLocationInfo(t *testing.T) {

	input := DownloadContentPlugin{}
	input.SourceType = "S3"

	validateInput(&input)

	result, err := validateInput(&input)

	assert.False(t, result)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "SourceInfo must be specified")
}

func TestName(t *testing.T) {
	assert.Equal(t, "aws:downloadContent", Name())
}

func TestParseAndValidateInput_NoInput(t *testing.T) {
	rawPluginInput := ""

	_, err := parseAndValidateInput(rawPluginInput)

	assert.Error(t, err)
}

func TestParseAndValidateInput_SourceInfoStringInput(t *testing.T) {
	sourceInfoOutput := "{'Path': 'test://test.com'}"
	sourceTypeTest := "S3"
	destinationPathTest := "destinationPathTest"
	var rawPluginInput interface{}
	rawPluginInputBytes := "{\"SourceType\":\"" + sourceTypeTest + "\", \"DestinationPath\" : \"" + destinationPathTest + "\", \"SourceInfo\": \"" + sourceInfoOutput + "\"}"
	_ = json.Unmarshal([]byte(rawPluginInputBytes), &rawPluginInput)
	downloadContent, err := parseAndValidateInput(rawPluginInput)
	assert.NoError(t, err)
	assert.Equal(t, downloadContent.SourceInfo, sourceInfoOutput)
	assert.Equal(t, downloadContent.DestinationPath, destinationPathTest)
	assert.Equal(t, downloadContent.SourceType, sourceTypeTest)
}

func TestParseAndValidateInput_SourceInfoJsonInput(t *testing.T) {
	sourceInfoOutput := "{\"Path\":\"test://test.com\"}"
	sourceTypeTest := "S3"
	destinationPathTest := "destinationPathTest"
	var rawPluginInput interface{}
	rawPluginInputBytes := "{\"SourceType\":\"" + sourceTypeTest + "\", \"DestinationPath\" : \"" + destinationPathTest + "\", \"SourceInfo\": " + sourceInfoOutput + "}"
	_ = json.Unmarshal([]byte(rawPluginInputBytes), &rawPluginInput)
	downloadContent, err := parseAndValidateInput(rawPluginInput)
	assert.NoError(t, err)
	assert.Equal(t, downloadContent.SourceInfo, sourceInfoOutput)
	assert.Equal(t, downloadContent.DestinationPath, destinationPathTest)
	assert.Equal(t, downloadContent.SourceType, sourceTypeTest)
}

func TestParseAndValidateInput_SourceInfoJsonInput_CamelCase_Success(t *testing.T) {
	sourceInfoOutput := "{\"Path\":\"test://test.com\"}"
	sourceTypeTest := "S3"
	destinationPathTest := "destinationPathTest"
	var rawPluginInput interface{}
	rawPluginInputBytes := "{\"SourceType\":\"" + sourceTypeTest + "\", \"DestinationPath\" : \"" + destinationPathTest + "\", \"sourceInfo\": " + sourceInfoOutput + "}"
	_ = json.Unmarshal([]byte(rawPluginInputBytes), &rawPluginInput)
	downloadContent, err := parseAndValidateInput(rawPluginInput)
	assert.NoError(t, err)
	assert.Equal(t, downloadContent.SourceInfo, sourceInfoOutput)
	assert.Equal(t, downloadContent.DestinationPath, destinationPathTest)
	assert.Equal(t, downloadContent.SourceType, sourceTypeTest)
}

// Mock and stub functions
func fakeRemoteResource(context context.T, locationType string, locationInfo string) (remoteresource.RemoteResource, error) {

	copyContentResourceMock.On("ValidateLocationInfo").Return(true, nil).Once()
	copyContentResourceMock.On("DownloadRemoteResource", &copyContentFileMock, mock.Anything).Return(nil, resourcemock.NewEmptyDownloadResult()).Once()
	return copyContentResourceMock, nil
}

func absoluteDestinationDirRemoteResource(context context.T, locationType string, locationInfo string) (remoteresource.RemoteResource, error) {
	copyContentResourceMock.On("ValidateLocationInfo").Return(true, nil).Once()
	copyContentResourceMock.On("DownloadRemoteResource", &copyContentFileMock, filepath.Join("orch", "downloads", "var", "temp", "fake-dir")).Return(nil, resourcemock.NewEmptyDownloadResult()).Once()
	return copyContentResourceMock, nil
}

func relativeDestinationDirRemoteResource(context context.T, locationType string, locationInfo string) (remoteresource.RemoteResource, error) {
	copyContentResourceMock.On("ValidateLocationInfo").Return(true, nil).Once()
	copyContentResourceMock.On("DownloadRemoteResource", &copyContentFileMock, filepath.Join("orch", "downloads", "temp", "fake-dir")).Return(nil, resourcemock.NewEmptyDownloadResult()).Once()
	return copyContentResourceMock, nil
}

func createStubConfiguration(orch, bucket, prefix, message, dir string) contracts.Configuration {
	return contracts.Configuration{
		OrchestrationDirectory:  orch,
		OutputS3BucketName:      bucket,
		OutputS3KeyPrefix:       prefix,
		MessageId:               message,
		PluginID:                "aws-copyContent",
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

func createSimpleConfigWithProperties(info *DownloadContentPlugin) contracts.Configuration {
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

func stubChmod(log log.T, workingDir string) error {
	return nil
}
