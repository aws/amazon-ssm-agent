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

// Package ssminstaller implements the installer for ssm packages that use documents or scripts to install and uninstall.
package ssminstaller

import (
	"io/ioutil"
	"path"
	"testing"

	"github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/aws/amazon-ssm-agent/agent/contracts"
	"github.com/aws/amazon-ssm-agent/agent/docmanager/model"
	"github.com/aws/amazon-ssm-agent/agent/framework/runpluginutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

const testPackagePath = "testdata"

var contextMock context.T = context.NewMockDefault()

func TestPackageName(t *testing.T) {
	testName := "TestName"
	testVersion := "TestVersion"
	inst := Installer{packageName: testName, version: testVersion}
	assert.Equal(t, testName, inst.packageName)
}

func TestPackageVersion(t *testing.T) {
	testName := "TestName"
	testVersion := "TestVersion"
	inst := Installer{packageName: testName, version: testVersion}
	assert.Equal(t, testVersion, inst.version)
}

func TestReadAction(t *testing.T) {
	// Setup mock with expectations
	mockFileSys := MockedFileSys{}
	mockFileSys.On("Exists", path.Join(testPackagePath, "Foo.json")).Return(true).Once()
	mockFileSys.On("ReadFile", path.Join(testPackagePath, "Foo.json")).Return(loadFile(t, path.Join(testPackagePath, "valid-action.json")), nil).Once()

	// Instantiate installer with mock
	inst := Installer{filesysdep: &mockFileSys, packagePath: testPackagePath}

	// Call and validate mock expectations and return value
	exists, actionDoc, workingDir, err := inst.readAction(contextMock, "Foo")
	mockFileSys.AssertExpectations(t)
	assert.True(t, exists)
	assert.NotEmpty(t, actionDoc)
	assert.Equal(t, workingDir, testPackagePath)
	assert.Nil(t, err)
}

func TestReadActionInvalid(t *testing.T) {
	// Setup mock with expectations
	mockFileSys := MockedFileSys{}
	mockFileSys.On("Exists", path.Join(testPackagePath, "Foo.json")).Return(true).Once()
	mockFileSys.On("ReadFile", path.Join(testPackagePath, "Foo.json")).Return(loadFile(t, path.Join(testPackagePath, "invalid-action.json")), nil).Once()

	// Instantiate installer with mock
	inst := Installer{filesysdep: &mockFileSys, packagePath: testPackagePath}

	// Call and validate mock expectations and return value
	exists, actionDoc, workingDir, err := inst.readAction(contextMock, "Foo")
	mockFileSys.AssertExpectations(t)
	assert.True(t, exists)
	assert.Empty(t, actionDoc)
	assert.Empty(t, workingDir)
	assert.NotNil(t, err)
}

func TestReadActionMissing(t *testing.T) {
	// Setup mock with expectations
	mockFileSys := MockedFileSys{}
	mockFileSys.On("Exists", path.Join(testPackagePath, "Foo.json")).Return(false).Once()

	// Instantiate installer with mock
	inst := Installer{filesysdep: &mockFileSys, packagePath: testPackagePath}

	// Call and validate mock expectations and return value
	exists, actionDoc, workingDir, err := inst.readAction(contextMock, "Foo")
	mockFileSys.AssertExpectations(t)
	assert.False(t, exists)
	assert.Empty(t, actionDoc)
	assert.Empty(t, workingDir)
	assert.Nil(t, err)
}

func TestInstall_ExecuteError(t *testing.T) {
	// Setup mocks with expectations
	mockFileSys := MockedFileSys{}
	mockFileSys.On("Exists", path.Join(testPackagePath, "install.json")).Return(true).Once()
	mockFileSys.On("ReadFile", path.Join(testPackagePath, "install.json")).Return(loadFile(t, path.Join(testPackagePath, "valid-action.json")), nil).Once()

	plugin := model.PluginState{}
	plugins := []model.PluginState{plugin}
	mockExec := MockedExec{}
	mockExec.On("ParseDocument", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(plugins, nil).Once()
	mockExec.On("ExecuteDocument", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(map[string]*contracts.PluginResult{"Foo": {StandardError: "execute error"}}).Once()

	// Instantiate installer with mock
	inst := Installer{filesysdep: &mockFileSys, execdep: &mockExec, packagePath: testPackagePath}

	// Call and validate mock expectations and return value
	output := inst.Install(contextMock)
	mockFileSys.AssertExpectations(t)
	mockExec.AssertExpectations(t)
	assert.NotEmpty(t, output.Stderr)
	assert.Contains(t, output.Stderr, "execute error")
}

func TestValidate_NoAction(t *testing.T) {
	// Setup mocks with expectations
	mockFileSys := MockedFileSys{}
	mockFileSys.On("Exists", path.Join(testPackagePath, "validate.json")).Return(false).Once()
	mockExec := MockedExec{}

	// Instantiate installer with mock
	inst := Installer{filesysdep: &mockFileSys, execdep: &mockExec, packagePath: testPackagePath}

	// Call and validate mock expectations and return value
	output := inst.Validate(contextMock)
	mockFileSys.AssertExpectations(t)
	mockExec.AssertExpectations(t)
	assert.Empty(t, output.Stderr)
	assert.Equal(t, 0, output.ExitCode)
	assert.Equal(t, contracts.ResultStatusSuccess, output.Status)
}

func TestUninstall_Success(t *testing.T) {
	// Setup mocks with expectations
	mockFileSys := MockedFileSys{}
	mockFileSys.On("Exists", path.Join(testPackagePath, "uninstall.json")).Return(true).Once()
	mockFileSys.On("ReadFile", path.Join(testPackagePath, "uninstall.json")).Return(loadFile(t, path.Join(testPackagePath, "valid-action.json")), nil).Once()

	plugin := model.PluginState{}
	plugins := []model.PluginState{plugin}
	mockExec := MockedExec{}
	mockExec.On("ParseDocument", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(plugins, nil).Once()
	mockExec.On("ExecuteDocument", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(map[string]*contracts.PluginResult{"Foo": {Status: contracts.ResultStatusSuccess}}).Once()

	// Instantiate installer with mock
	inst := Installer{filesysdep: &mockFileSys,
		execdep:     &mockExec,
		packagePath: testPackagePath,
		config:      contracts.Configuration{OutputS3BucketName: "foo", OutputS3KeyPrefix: "bar"}}

	// Call and validate mock expectations and return value
	output := inst.Uninstall(contextMock)
	mockFileSys.AssertExpectations(t)
	mockExec.AssertExpectations(t)
	assert.Empty(t, output.Stderr)
	assert.Equal(t, 0, output.ExitCode)
	assert.Equal(t, contracts.ResultStatusSuccess, output.Status)
}

// Load specified file from file system
func loadFile(t *testing.T, fileName string) (result []byte) {
	var err error
	if result, err = ioutil.ReadFile(fileName); err != nil {
		t.Fatal(err)
	}
	return
}

type MockedFileSys struct {
	mock.Mock
}

func (fileMock *MockedFileSys) Exists(filePath string) bool {
	args := fileMock.Called(filePath)
	return args.Bool(0)
}

func (fileMock *MockedFileSys) ReadFile(filename string) ([]byte, error) {
	args := fileMock.Called(filename)
	return args.Get(0).([]byte), args.Error(1)
}

type MockedExec struct {
	mock.Mock
}

func (execMock *MockedExec) ParseDocument(
	context context.T,
	documentRaw []byte,
	orchestrationDir string,
	s3Bucket string,
	s3KeyPrefix string,
	messageID string,
	documentID string,
	defaultWorkingDirectory string) (pluginsInfo []model.PluginState, err error) {
	args := execMock.Called(context, documentRaw, orchestrationDir, s3Bucket, s3KeyPrefix, messageID, documentID, defaultWorkingDirectory)
	return args.Get(0).([]model.PluginState), args.Error(1)
}

func (execMock *MockedExec) ExecuteDocument(
	runner runpluginutil.PluginRunner,
	context context.T,
	pluginInput []model.PluginState,
	documentID string,
	documentCreatedDate string) (pluginOutputs map[string]*contracts.PluginResult) {
	args := execMock.Called(runner, context, pluginInput, documentID, documentCreatedDate)
	return args.Get(0).(map[string]*contracts.PluginResult)
}
