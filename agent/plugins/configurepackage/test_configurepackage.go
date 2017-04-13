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
	"github.com/aws/amazon-ssm-agent/agent/docmanager/model"
	"github.com/aws/amazon-ssm-agent/agent/framework/runpluginutil"
	"github.com/aws/amazon-ssm-agent/agent/plugins/configurepackage/localpackages"
	"github.com/aws/amazon-ssm-agent/agent/task"
	"github.com/stretchr/testify/mock"
)

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
	execDepStub    execDep
	execDepOrig    execDep
	stubsSet       bool
}

// Set replaces dependencies with stub versions and saves the original version.
// it should always be followed by defer Clear()
func (m *ConfigurePackageStubs) Set() {
	if m.stubsSet {
		m.Clear() // This protects us from double-setting stubs (we don't have a stack so we must only have one layer of stubs set at a time)
	}
	if m.fileSysDepStub != nil {
		m.fileSysDepOrig = filesysdep
		filesysdep = m.fileSysDepStub
	}
	if m.execDepStub != nil {
		m.execDepOrig = execdep
		execdep = m.execDepStub
	}
	m.stubsSet = true
}

// Clear resets dependencies to their original values.
func (m *ConfigurePackageStubs) Clear() {
	if !m.stubsSet {
		return // This protects us from resetting to nil values if stubs were never set
	}
	if m.fileSysDepStub != nil {
		filesysdep = m.fileSysDepOrig
	}
	if m.execDepStub != nil {
		execdep = m.execDepOrig
	}
	m.stubsSet = false
}

func execStubSuccess() execDep {
	return &ExecDepStub{pluginInput: &model.PluginState{}, pluginOutput: &contracts.PluginResult{Status: contracts.ResultStatusSuccess}}
}

func setSuccessStubs() *ConfigurePackageStubs {
	stubs := &ConfigurePackageStubs{fileSysDepStub: &FileSysDepStub{}, execDepStub: execStubSuccess()}
	stubs.Set()
	return stubs
}

type FileSysDepStub struct {
	makeFileError error
	writeError    error
}

func (m *FileSysDepStub) MakeDirExecute(destinationDir string) (err error) {
	return m.makeFileError
}

func (m *FileSysDepStub) WriteFile(filename string, content string) error {
	return m.writeError
}

type ExecDepStub struct {
	pluginInput     *model.PluginState
	parseError      error
	pluginOutput    *contracts.PluginResult
	pluginOutputMap map[string]*contracts.PluginResult
}

func (m *ExecDepStub) ParseDocument(context context.T, documentRaw []byte, orchestrationDir string, s3Bucket string, s3KeyPrefix string, messageID string, documentID string, defaultWorkingDirectory string) (pluginsInfo []model.PluginState, err error) {
	pluginsInfo = make([]model.PluginState, 0, 1)
	if m.pluginInput != nil {
		m.pluginInput.Configuration.DefaultWorkingDirectory = defaultWorkingDirectory
		pluginsInfo = append(pluginsInfo, *m.pluginInput)
	}
	return pluginsInfo, m.parseError
}

func (m *ExecDepStub) ExecuteDocument(runner runpluginutil.PluginRunner, context context.T, pluginInput []model.PluginState, documentID string, documentCreatedDate string) (pluginOutputs map[string]*contracts.PluginResult) {
	pluginOutputs = make(map[string]*contracts.PluginResult)
	// TODO:MF: We're using the working directory as an index into the stub results.  We should convert all of this to testify mocks.
	if output, ok := m.pluginOutputMap[pluginInput[0].Configuration.DefaultWorkingDirectory]; ok {
		pluginOutputs["test"] = output
	} else if m.pluginOutput != nil {
		pluginOutputs["test"] = m.pluginOutput
	}
	return
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
