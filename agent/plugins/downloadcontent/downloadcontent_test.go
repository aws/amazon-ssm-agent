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
	"testing"

	"time"

	"errors"

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
	remoteresource, err := newRemoteResource(logger, "invalid", mockLocationInfo)

	assert.Nil(t, remoteresource)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "Invalid SourceType")

}

func TestNewRemoteResource_Github(t *testing.T) {

	locationInfo := `{
		"owner" : "test-owner",
		"repository" :	 "test-repo"
		}`
	remoteresource, err := newRemoteResource(logger, "GitHub", locationInfo)

	assert.NotNil(t, remoteresource)
	assert.NoError(t, err)

}

func TestNewRemoteResource_S3(t *testing.T) {

	locationInfo := `{
		"path" : "https://s3.amazonaws.com/test-bucket/fake-key/"
		}`
	remoteresource, err := newRemoteResource(logger, "S3", locationInfo)

	assert.NotNil(t, remoteresource)
	assert.NoError(t, err)

}

func TestNewRemoteResource_SSMDocument(t *testing.T) {

	locationInfo := `{
		"name" : "doc-name",
		"version" : "1"
		}`
	remoteresource, err := newRemoteResource(logger, "SSMDocument", locationInfo)

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
		remoteResourceCreator: fakeRemoteResource,
		filesys:               fileMock,
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
		DestinationPath: "/var/temp/fake-dir",
	}
	config := createStubConfiguration("orch", "bucket", "prefix", "1234-1234-1234", "directory")

	p := Plugin{
		remoteResourceCreator: absoluteDestinationDirRemoteResource,
		filesys:               fileMock,
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
		DestinationPath: "temp/fake-dir/",
	}
	config := createStubConfiguration("orch/aws-copyContent", "bucket", "prefix", "1234-1234-1234", "directory")

	p := Plugin{
		remoteResourceCreator: relativeDestinationDirRemoteResource,
		filesys:               fileMock,
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
		remoteResourceCreator: newRemoteResource,
		filesys:               fileMock,
	}
	mockIOHandler.On("MarkAsFailed", mock.Anything).Return()

	p.runCopyContent(logger, &input, config, mockIOHandler)

	fileMock.AssertExpectations(t)
	mockIOHandler.AssertExpectations(t)
}

func TestPlugin_ExecuteGitHubFile(t *testing.T) {

	mockplugin := MockDefaultPlugin{}
	mockIOHandler := new(iohandlermocks.MockIOHandler)

	input := DownloadContentPlugin{}

	input.SourceType = "GitHub"
	input.SourceInfo = `{
		"owner" : "test-owner",
		"repository" :	 "test-repo",
		"path" : "path"
		}`
	input.DestinationPath = "destination"
	conf := createSimpleConfigWithProperties(&input)

	var githubcopyContentResourceMock = resourcemock.RemoteResourceMock{}

	var githubCopyContentFileMock = filemock.FileSystemMock{}

	mockIOHandler.On("AppendInfof", mock.Anything, mock.Anything).Return()
	mockIOHandler.On("MarkAsSucceeded").Return()

	githubRemoteresourceMock := func(log log.T, locationtype, locationInfo string) (remoteresource.RemoteResource, error) {

		githubcopyContentResourceMock.On("ValidateLocationInfo").Return(true, nil).Once()
		githubcopyContentResourceMock.On("DownloadRemoteResource", contextMock.Log(), githubCopyContentFileMock, "orch/downloads/destination").Return(nil, resourcemock.NewEmptyDownloadResult()).Once()
		return githubcopyContentResourceMock, nil
	}

	p := &Plugin{
		remoteResourceCreator: githubRemoteresourceMock,
		filesys:               githubCopyContentFileMock,
	}
	SetPermission = stubChmod
	p.execute(contextMock, conf, createMockCancelFlag(), mockIOHandler)

	githubCopyContentFileMock.AssertExpectations(t)
	githubcopyContentResourceMock.AssertExpectations(t)
	mockplugin.AssertExpectations(t)
	mockIOHandler.AssertExpectations(t)
}

func TestPlugin_ExecuteS3File(t *testing.T) {

	mockplugin := MockDefaultPlugin{}
	mockIOHandler := new(iohandlermocks.MockIOHandler)

	input := DownloadContentPlugin{}

	input.SourceType = "S3"
	input.SourceInfo = `{
		"path" : "https://s3.amazonaws.com/fake-bucket/fake-key/filename.ps"
		}`
	input.DestinationPath = "/var/tmp/destination"
	conf := createSimpleConfigWithProperties(&input)
	cancelFlag := createMockCancelFlag()
	var s3copyContentResourceMock = resourcemock.RemoteResourceMock{}

	var s3CopyContentFileMock = filemock.FileSystemMock{}
	mockIOHandler.On("AppendInfof", mock.Anything, mock.Anything).Return()
	mockIOHandler.On("MarkAsSucceeded").Return()

	s3MockRemoteResource := func(log log.T, locationtype, locationInfo string) (remoteresource.RemoteResource, error) {

		s3copyContentResourceMock.On("ValidateLocationInfo").Return(true, nil).Once()
		s3copyContentResourceMock.On("DownloadRemoteResource", contextMock.Log(), s3CopyContentFileMock, "/var/tmp/destination").Return(nil, resourcemock.NewEmptyDownloadResult()).Once()
		return s3copyContentResourceMock, nil
	}
	p := &Plugin{
		remoteResourceCreator: s3MockRemoteResource,
		filesys:               s3CopyContentFileMock,
	}
	SetPermission = stubChmod
	p.execute(contextMock, conf, cancelFlag, mockIOHandler)

	s3CopyContentFileMock.AssertExpectations(t)
	s3copyContentResourceMock.AssertExpectations(t)
	mockplugin.AssertExpectations(t)
	mockIOHandler.AssertExpectations(t)
}

func TestPlugin_ExecuteSSMDoc(t *testing.T) {

	mockplugin := MockDefaultPlugin{}
	mockIOHandler := new(iohandlermocks.MockIOHandler)

	input := DownloadContentPlugin{}

	input.SourceType = "SSMDocument"
	input.SourceInfo = `{
		"name" : "arn:aws:ssm:us-east-1:1234567890:document/mySharedDocument:10"
		}`
	input.DestinationPath = "/var/tmp/destination/"
	conf := createSimpleConfigWithProperties(&input)
	cancelFlag := createMockCancelFlag()

	var ssmDocCopyContentResourceMock = resourcemock.RemoteResourceMock{}
	var ssmDocCopyContentFileMock = filemock.FileSystemMock{}
	mockIOHandler.On("AppendInfof", mock.Anything, mock.Anything).Return()
	mockIOHandler.On("MarkAsSucceeded").Return()

	ssmDocMockRemoteResource := func(log log.T, locationtype, locationInfo string) (remoteresource.RemoteResource, error) {
		ssmDocCopyContentResourceMock.On("ValidateLocationInfo").Return(true, nil).Once()
		ssmDocCopyContentResourceMock.On("DownloadRemoteResource", contextMock.Log(), ssmDocCopyContentFileMock, "/var/tmp/destination/").Return(nil, resourcemock.NewEmptyDownloadResult()).Once()
		return ssmDocCopyContentResourceMock, nil
	}
	p := &Plugin{
		remoteResourceCreator: ssmDocMockRemoteResource,
		filesys:               ssmDocCopyContentFileMock,
	}
	SetPermission = stubChmod
	p.execute(contextMock, conf, cancelFlag, mockIOHandler)

	ssmDocCopyContentFileMock.AssertExpectations(t)
	ssmDocCopyContentResourceMock.AssertExpectations(t)
	mockplugin.AssertExpectations(t)
	mockIOHandler.AssertExpectations(t)
}

func TestPlugin_ExecuteSSMDocError(t *testing.T) {

	mockplugin := MockDefaultPlugin{}
	mockIOHandler := new(iohandlermocks.MockIOHandler)

	input := DownloadContentPlugin{}

	input.SourceType = "SSMDocument"
	input.SourceInfo = `{
	"name" : ":10"
		}`
	input.DestinationPath = "/var/tmp/destination/"
	conf := createSimpleConfigWithProperties(&input)
	cancelFlag := createMockCancelFlag()

	var ssmDoccopyContentResourceMock = resourcemock.RemoteResourceMock{}
	var ssmDocCopyContentFileMock = filemock.FileSystemMock{}
	mockIOHandler.On("MarkAsFailed", mock.Anything).Return()

	ssmDocMockRemoteResource := func(log log.T, locationtype, locationInfo string) (remoteresource.RemoteResource, error) {
		ssmDoccopyContentResourceMock.On("DownloadRemoteResource", contextMock.Log(), ssmDocCopyContentFileMock, "/var/tmp/destination/").Return(errors.New("Document name must be specified"), (*remoteresource.DownloadResult)(nil)).Once()
		ssmDoccopyContentResourceMock.On("ValidateLocationInfo").Return(true, nil).Once()
		return ssmDoccopyContentResourceMock, nil
	}
	p := &Plugin{
		remoteResourceCreator: ssmDocMockRemoteResource,
		filesys:               ssmDocCopyContentFileMock,
	}
	SetPermission = stubChmod
	p.execute(contextMock, conf, cancelFlag, mockIOHandler)

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

// Mock and stub functions
func fakeRemoteResource(log log.T, locationType string, locationInfo string) (remoteresource.RemoteResource, error) {

	copyContentResourceMock.On("ValidateLocationInfo").Return(true, nil).Once()
	copyContentResourceMock.On("DownloadRemoteResource", logger, copyContentFileMock, mock.Anything).Return(nil, resourcemock.NewEmptyDownloadResult()).Once()
	return copyContentResourceMock, nil
}

func absoluteDestinationDirRemoteResource(log log.T, locationType string, locationInfo string) (remoteresource.RemoteResource, error) {

	copyContentResourceMock.On("ValidateLocationInfo").Return(true, nil).Once()
	copyContentResourceMock.On("DownloadRemoteResource", logger, copyContentFileMock, "/var/temp/fake-dir").Return(nil, resourcemock.NewEmptyDownloadResult()).Once()
	return copyContentResourceMock, nil
}

func relativeDestinationDirRemoteResource(log log.T, locationType string, locationInfo string) (remoteresource.RemoteResource, error) {
	copyContentResourceMock.On("ValidateLocationInfo").Return(true, nil).Once()
	copyContentResourceMock.On("DownloadRemoteResource", logger, copyContentFileMock, "orch/downloads/temp/fake-dir/").Return(nil, resourcemock.NewEmptyDownloadResult()).Once()
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
