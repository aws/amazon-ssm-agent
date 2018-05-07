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
	"github.com/aws/amazon-ssm-agent/agent/fileutil"
	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/amazon-ssm-agent/agent/plugins/configurepackage/envdetect"
	"github.com/aws/amazon-ssm-agent/agent/plugins/configurepackage/envdetect/ec2infradetect"
	"github.com/aws/amazon-ssm-agent/agent/plugins/configurepackage/envdetect/osdetect"
	"github.com/aws/amazon-ssm-agent/agent/plugins/configurepackage/trace"
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

func mockReadAction(t *testing.T, mockFileSys *MockedFileSys, actionPathNoExt string, contentSh []byte, contentPs1 []byte, expectReads bool) {
	// Setup mock with expectations
	mockFileSys.On("Exists", actionPathNoExt+".sh").Return(len(contentSh) != 0).Once()
	mockFileSys.On("Exists", actionPathNoExt+".ps1").Return(len(contentPs1) != 0).Once()

	if expectReads {
		if len(contentSh) != 0 {
			mockFileSys.On("ReadFile", actionPathNoExt+".sh").Return(contentSh, nil).Once()
		}
		if len(contentPs1) != 0 {
			mockFileSys.On("ReadFile", actionPathNoExt+".ps1").Return(contentPs1, nil).Once()
		}
	}
}

var environmentStub = envdetect.Environment{
	&osdetect.OperatingSystem{"abc", "567", "", "xyz", "", ""},
	&ec2infradetect.Ec2Infrastructure{"instanceIDX", "Reg1", "", "AZ1", "instanceTypeZ"},
}

func testReadAction(t *testing.T, actionPathNoExt string, contentSh []byte, contentPs1 []byte, expectReads bool) {
	mockFileSys := MockedFileSys{}
	mockReadAction(t, &mockFileSys, actionPathNoExt, contentSh, contentPs1, expectReads)

	mockEnvdetectCollector := &envdetect.CollectorMock{}
	mockEnvdetectCollector.On("CollectData", mock.Anything).Return(&environmentStub, nil).Once()

	tracer := trace.NewTracer(log.NewMockLog())
	tracer.BeginSection("test segment root")

	// Instantiate installer with mock
	inst := Installer{filesysdep: &mockFileSys, packagePath: testPackagePath, envdetectCollector: mockEnvdetectCollector}

	// Call and validate mock expectations and return value
	exists, actionDoc, workingDir, orchestrationDir, err := inst.readAction(tracer, contextMock, "Foo")
	mockFileSys.AssertExpectations(t)
	assert.True(t, exists)
	assert.NotEmpty(t, actionDoc)
	assert.Equal(t, workingDir, testPackagePath)
	assert.Equal(t, orchestrationDir, "Foo")
	assert.Nil(t, err)
}

func testReadActionInvalid(t *testing.T, actionPathNoExt string, contentSh []byte, contentPs1 []byte, expectReads bool) {
	mockFileSys := MockedFileSys{}
	mockReadAction(t, &mockFileSys, actionPathNoExt, contentSh, contentPs1, expectReads)

	mockEnvdetectCollector := &envdetect.CollectorMock{}
	mockEnvdetectCollector.On("CollectData", mock.Anything).Return(&environmentStub, nil).Once()

	tracer := trace.NewTracer(log.NewMockLog())
	tracer.BeginSection("test segment root")

	// Instantiate installer with mock
	inst := Installer{filesysdep: &mockFileSys, packagePath: testPackagePath, envdetectCollector: mockEnvdetectCollector}

	// Call and validate mock expectations and return value
	exists, actionDoc, workingDir, orchestrationDir, err := inst.readAction(tracer, contextMock, "Foo")
	mockFileSys.AssertExpectations(t)
	assert.True(t, exists)
	assert.Empty(t, actionDoc)
	assert.Empty(t, workingDir)
	assert.Empty(t, orchestrationDir)
	assert.NotNil(t, err)
}

func TestReadAction(t *testing.T) {
	actionPathNoExt := path.Join(testPackagePath, "Foo")
	testReadAction(t, actionPathNoExt, []byte("echo sh"), []byte{}, false)
	testReadAction(t, actionPathNoExt, []byte{}, append(fileutil.CreateUTF8ByteOrderMark(), []byte("Write-Host ps1 with BOM")...), false)
}

func TestReadActionMissing(t *testing.T) {
	mockFileSys := MockedFileSys{}
	actionPathNoExt := path.Join(testPackagePath, "Foo")
	mockReadAction(t, &mockFileSys, actionPathNoExt, []byte{}, []byte{}, false)

	mockEnvdetectCollector := &envdetect.CollectorMock{}
	mockEnvdetectCollector.On("CollectData", mock.Anything).Return(&environmentStub, nil).Once()

	tracer := trace.NewTracer(log.NewMockLog())
	tracer.BeginSection("test segment root")

	// Instantiate repository with mock
	repo := Installer{filesysdep: &mockFileSys, packagePath: testPackagePath, envdetectCollector: mockEnvdetectCollector}

	// Call and validate mock expectations and return value
	exists, actionDoc, workingDir, orchestrationDir, err := repo.readAction(tracer, contextMock, "Foo")
	mockFileSys.AssertExpectations(t)
	assert.False(t, exists)
	assert.Empty(t, actionDoc)
	assert.Empty(t, workingDir)
	assert.Empty(t, orchestrationDir)
	assert.Nil(t, err)
}

func testReadActionTooManyActionImplementations(t *testing.T, existSh bool, existPs1 bool) {
	mockFileSys := MockedFileSys{}
	actionPathNoExt := path.Join(testPackagePath, "Foo")
	mockFileSys.On("Exists", actionPathNoExt+".sh").Return(existSh).Once()
	mockFileSys.On("Exists", actionPathNoExt+".ps1").Return(existPs1).Once()

	mockEnvdetectCollector := &envdetect.CollectorMock{}
	mockEnvdetectCollector.On("CollectData", mock.Anything).Return(&environmentStub, nil).Once()

	tracer := trace.NewTracer(log.NewMockLog())
	tracer.BeginSection("test segment root")

	// Instantiate repository with mock
	repo := Installer{filesysdep: &mockFileSys, packagePath: testPackagePath, envdetectCollector: mockEnvdetectCollector}

	// Call and validate mock expectations and return value
	exists, actionDoc, workingDir, orchestrationDir, err := repo.readAction(tracer, contextMock, "Foo")
	mockFileSys.AssertExpectations(t)
	assert.True(t, exists)
	assert.Empty(t, actionDoc)
	assert.Empty(t, workingDir)
	assert.Empty(t, orchestrationDir)
	assert.NotNil(t, err)
}

func TestReadActionTooManyActionImplementations(t *testing.T) {
	testReadActionTooManyActionImplementations(t, true, true)
}

func TestInstall_ExecuteError(t *testing.T) {
	// Setup mocks with expectations
	mockFileSys := MockedFileSys{}
	actionPathNoExt := path.Join(testPackagePath, "install")
	mockReadAction(t, &mockFileSys, actionPathNoExt, []byte("echo sh"), []byte{}, false)

	mockExec := MockedExec{}
	mockExec.On("ExecuteDocument", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(map[string]*contracts.PluginResult{"Foo": {StandardError: "execute error"}}).Once()

	mockEnvdetectCollector := &envdetect.CollectorMock{}
	mockEnvdetectCollector.On("CollectData", mock.Anything).Return(&environmentStub, nil).Once()

	tracer := trace.NewTracer(log.NewMockLog())

	// Instantiate installer with mock
	inst := Installer{filesysdep: &mockFileSys, execdep: &mockExec, packagePath: testPackagePath, envdetectCollector: mockEnvdetectCollector}

	// Call and validate mock expectations and return value
	output := inst.Install(tracer, contextMock)
	mockFileSys.AssertExpectations(t)
	mockExec.AssertExpectations(t)
	assert.NotEmpty(t, output.GetStderr())
	assert.Contains(t, output.GetStderr(), "execute error")
}

func TestValidate_NoAction(t *testing.T) {
	// Setup mocks with expectations
	mockFileSys := MockedFileSys{}
	actionPathNoExt := path.Join(testPackagePath, "validate")
	mockReadAction(t, &mockFileSys, actionPathNoExt, []byte{}, []byte{}, false)
	mockExec := MockedExec{}

	mockEnvdetectCollector := &envdetect.CollectorMock{}
	mockEnvdetectCollector.On("CollectData", mock.Anything).Return(&environmentStub, nil).Once()

	tracer := trace.NewTracer(log.NewMockLog())

	// Instantiate installer with mock
	inst := Installer{filesysdep: &mockFileSys, execdep: &mockExec, packagePath: testPackagePath, envdetectCollector: mockEnvdetectCollector}

	// Call and validate mock expectations and return value
	output := inst.Validate(tracer, contextMock)
	mockFileSys.AssertExpectations(t)
	mockExec.AssertExpectations(t)
	assert.Empty(t, output.GetStderr())
	assert.Equal(t, 0, output.GetExitCode())
	assert.Equal(t, contracts.ResultStatusSuccess, output.GetStatus())
}

func TestUninstall_Success(t *testing.T) {
	// Setup mocks with expectations
	mockFileSys := MockedFileSys{}
	actionPathNoExt := path.Join(testPackagePath, "uninstall")
	mockReadAction(t, &mockFileSys, actionPathNoExt, []byte("echo sh"), []byte{}, false)

	mockExec := MockedExec{}
	mockExec.On("ExecuteDocument", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(map[string]*contracts.PluginResult{"Foo": {Status: contracts.ResultStatusSuccess}}).Once()

	mockEnvdetectCollector := &envdetect.CollectorMock{}
	mockEnvdetectCollector.On("CollectData", mock.Anything).Return(&environmentStub, nil).Once()

	tracer := trace.NewTracer(log.NewMockLog())

	// Instantiate installer with mock
	inst := Installer{filesysdep: &mockFileSys,
		execdep:            &mockExec,
		packagePath:        testPackagePath,
		config:             contracts.Configuration{OutputS3BucketName: "foo", OutputS3KeyPrefix: "bar"},
		envdetectCollector: mockEnvdetectCollector}

	// Call and validate mock expectations and return value
	output := inst.Uninstall(tracer, contextMock)
	mockFileSys.AssertExpectations(t)
	mockExec.AssertExpectations(t)
	assert.Empty(t, output.GetStderr())
	assert.Equal(t, 0, output.GetExitCode())
	assert.Equal(t, contracts.ResultStatusSuccess, output.GetStatus())
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
	defaultWorkingDirectory string) (pluginsInfo []contracts.PluginState, err error) {
	args := execMock.Called(context, documentRaw, orchestrationDir, s3Bucket, s3KeyPrefix, messageID, documentID, defaultWorkingDirectory)
	return args.Get(0).([]contracts.PluginState), args.Error(1)
}

func (execMock *MockedExec) ExecuteDocument(
	context context.T,
	pluginInput []contracts.PluginState,
	documentID string,
	documentCreatedDate string,
	orchestrationDirectory string) (pluginOutputs map[string]*contracts.PluginResult) {
	args := execMock.Called(context, pluginInput, documentID, documentCreatedDate)
	return args.Get(0).(map[string]*contracts.PluginResult)
}
