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

// Package configurepackage implements the ConfigurePackage plugin.
// test_configurepackage contains stub implementations
package configurepackage

import (
	"strings"
	"time"

	"github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/aws/amazon-ssm-agent/agent/contracts"
	"github.com/aws/amazon-ssm-agent/agent/plugins/configurepackage/localpackages"
	"github.com/aws/amazon-ssm-agent/agent/task"
	"github.com/stretchr/testify/mock"
)

// TODO:MF: This whole file is now used only by tests and should be merged into an existing _test file or renamed to have the _test suffix.

func createMockCancelFlag() task.CancelFlag {
	mockCancelFlag := new(task.MockCancelFlag)
	// Setup mocks
	mockCancelFlag.On("Canceled").Return(false)
	mockCancelFlag.On("ShutDown").Return(false)
	mockCancelFlag.On("Wait").Return(false).After(100 * time.Millisecond)

	return mockCancelFlag
}

type ConfigurePackageStubs struct {
	// individual stub functions or interfaces go here with a temp variable for the original version
	fileSysDepStub fileSysDep
	fileSysDepOrig fileSysDep
	stubsSet       bool
}

// Set replaces dependencies with stub versions and saves the original version.
// it should always be followed by defer Clear()
func (m *ConfigurePackageStubs) Set() {
	if m.fileSysDepStub != nil {
		m.fileSysDepOrig = filesysdep
		filesysdep = m.fileSysDepStub
	}
	m.stubsSet = true
}

// Clear resets dependencies to their original values.
func (m *ConfigurePackageStubs) Clear() {
	if m.fileSysDepStub != nil {
		filesysdep = m.fileSysDepOrig
	}
	m.stubsSet = false
}

func setSuccessStubs() *ConfigurePackageStubs {
	stubs := &ConfigurePackageStubs{fileSysDepStub: &FileSysDepStub{}}
	stubs.Set()
	return stubs
}

type FileSysDepStub struct {
	makeFileError   error
	uncompressError error
	removeError     error
	writeError      error
}

func (m *FileSysDepStub) MakeDirExecute(destinationDir string) (err error) {
	return m.makeFileError
}

func (m *FileSysDepStub) Uncompress(src, dest string) error {
	return m.uncompressError
}

func (m *FileSysDepStub) RemoveAll(path string) error {
	return m.removeError
}

func (m *FileSysDepStub) WriteFile(filename string, content string) error {
	return m.writeError
}

type MockedConfigurePackageManager struct {
	mock.Mock
	waitChan chan bool
}

func (configMock *MockedConfigurePackageManager) validateInput(context context.T,
	input *ConfigurePackagePluginInput) (valid bool, err error) {
	args := configMock.Called(input)
	return args.Bool(0), args.Error(1)
}

func (configMock *MockedConfigurePackageManager) getVersionToInstall(context context.T,
	input *ConfigurePackagePluginInput) (version string, installedVersion string, installState localpackages.InstallState, err error) {
	args := configMock.Called(input)
	ver := args.String(0)
	if strings.HasPrefix(ver, "Wait") {
		configMock.waitChan <- true
		_ = <-configMock.waitChan
		ver = strings.TrimLeft(ver, "Wait")
	}
	return ver, args.String(1), args.Get(2).(localpackages.InstallState), args.Error(3)
}

func (configMock *MockedConfigurePackageManager) getVersionToUninstall(context context.T,
	input *ConfigurePackagePluginInput) (version string, err error) {
	args := configMock.Called(input)
	ver := args.String(0)
	if strings.HasPrefix(ver, "Wait") {
		configMock.waitChan <- true
		_ = <-configMock.waitChan
		ver = strings.TrimLeft(ver, "Wait")
	}
	return ver, args.Error(1)
}

func (configMock *MockedConfigurePackageManager) ensurePackage(context context.T,
	packageName string,
	version string,
	output *contracts.PluginOutput) error {
	args := configMock.Called(packageName, version, output)
	return args.Error(0)
}

func (configMock *MockedConfigurePackageManager) runUninstallPackagePre(context context.T,
	packageName string,
	version string,
	output *contracts.PluginOutput) (status contracts.ResultStatus, err error) {
	args := configMock.Called(packageName, version, output)
	return args.Get(0).(contracts.ResultStatus), args.Error(1)
}

func (configMock *MockedConfigurePackageManager) runInstallPackage(context context.T,
	packageName string,
	version string,
	output *contracts.PluginOutput) (status contracts.ResultStatus, err error) {
	args := configMock.Called(packageName, version, output)
	return args.Get(0).(contracts.ResultStatus), args.Error(1)
}

func (configMock *MockedConfigurePackageManager) runUninstallPackagePost(context context.T,
	packageName string,
	version string,
	output *contracts.PluginOutput) (status contracts.ResultStatus, err error) {
	args := configMock.Called(packageName, version, output)
	return args.Get(0).(contracts.ResultStatus), args.Error(1)
}

func (configMock *MockedConfigurePackageManager) runValidatePackage(context context.T,
	packageName string,
	version string,
	output *contracts.PluginOutput) (status contracts.ResultStatus, err error) {
	args := configMock.Called(packageName, version, output)
	return args.Get(0).(contracts.ResultStatus), args.Error(1)
}

func (configMock *MockedConfigurePackageManager) setInstallState(context context.T,
	packageName string,
	version string,
	state localpackages.InstallState) error {
	args := configMock.Called(packageName, version, state)
	return args.Error(0)
}

func ConfigPackageSuccessMock(downloadFilePath string,
	versionToActOn string,
	versionCurrentlyInstalled string,
	installResult contracts.ResultStatus,
	uninstallPreResult contracts.ResultStatus,
	uninstallPostResult contracts.ResultStatus) *MockedConfigurePackageManager {
	mockConfig := MockedConfigurePackageManager{}
	mockConfig.On("downloadPackage", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(downloadFilePath, nil)
	mockConfig.On("validateInput", mock.Anything, mock.Anything).Return(true, nil)
	mockConfig.On("getVersionToInstall", mock.Anything, mock.Anything, mock.Anything).Return(versionToActOn, versionCurrentlyInstalled, localpackages.None, nil)
	mockConfig.On("getVersionToUninstall", mock.Anything, mock.Anything, mock.Anything).Return(versionToActOn, nil)
	mockConfig.On("setMark", mock.Anything, mock.Anything, mock.Anything).Return(nil)
	mockConfig.On("clearMark", mock.Anything, mock.Anything)
	mockConfig.On("ensurePackage", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil)
	mockConfig.On("runUninstallPackagePre", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(uninstallPreResult, nil)
	mockConfig.On("runInstallPackage", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(installResult, nil)
	mockConfig.On("runUninstallPackagePost", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(uninstallPostResult, nil)
	mockConfig.On("runValidatePackage", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(contracts.ResultStatusSuccess, nil)
	mockConfig.On("setInstallState", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil)
	mockConfig.waitChan = make(chan bool)
	return &mockConfig
}

// -- form configureutil_test.go
func createStubPluginInputInstall() *ConfigurePackagePluginInput {
	input := ConfigurePackagePluginInput{}

	input.Version = "1.0.0"
	input.Name = "PVDriver"
	input.Action = "Install"

	return &input
}

func createStubPluginInputUninstall() *ConfigurePackagePluginInput {
	input := ConfigurePackagePluginInput{}

	input.Version = "1.0.0"
	input.Name = "PVDriver"
	input.Action = "Uninstall"

	return &input
}
