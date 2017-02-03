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
	"github.com/aws/amazon-ssm-agent/agent/fileutil/artifact"
	"github.com/aws/amazon-ssm-agent/agent/framework/runpluginutil"
	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/amazon-ssm-agent/agent/s3util"
	"github.com/aws/amazon-ssm-agent/agent/statemanager/model"
	"github.com/aws/amazon-ssm-agent/agent/task"
	"github.com/aws/amazon-ssm-agent/agent/updateutil"
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

func createStubInstanceContext() *updateutil.InstanceContext {
	context := updateutil.InstanceContext{}

	context.Region = "us-west-2"
	context.Platform = "windows"
	context.PlatformVersion = "2015.9"
	context.InstallerName = "Windows"
	context.Arch = "amd64"
	context.CompressFormat = "zip"

	return &context
}

func createStubInstanceContextBjs() *updateutil.InstanceContext {
	context := updateutil.InstanceContext{}

	context.Region = "cn-north-1"
	context.Platform = "windows"
	context.PlatformVersion = "2015.9"
	context.InstallerName = "Windows"
	context.Arch = "amd64"
	context.CompressFormat = "zip"

	return &context
}

type ConfigurePackageStubs struct {
	// individual stub functions or interfaces go here with a temp variable for the original version
	fileSysDepStub fileSysDep
	fileSysDepOrig fileSysDep
	networkDepStub networkDep
	networkDepOrig networkDep
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
	if m.networkDepStub != nil {
		m.networkDepOrig = networkdep
		networkdep = m.networkDepStub
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
	if m.networkDepStub != nil {
		networkdep = m.networkDepOrig
	}
	if m.execDepStub != nil {
		execdep = m.execDepOrig
	}
	m.stubsSet = false
}

func fileSysStubSuccess() fileSysDep {
	result := `{
  "name": "PVDriver",
  "platform": "Windows",
  "architecture": "amd64",
  "version": "1.0.0"
}`
	return &FileSysDepStub{readResult: []byte(result), existsResultDefault: true}
}

func networkStubSuccess() networkDep {
	return &NetworkDepStub{downloadResultDefault: artifact.DownloadOutput{LocalFilePath: "Stub"}}
}

func execStubSuccess() execDep {
	return &ExecDepStub{pluginInput: &model.PluginState{}, pluginOutput: &contracts.PluginResult{Status: contracts.ResultStatusSuccess}}
}

func SetStubs() *ConfigurePackageStubs {
	getContext = func(log log.T) (instanceContext *updateutil.InstanceContext, err error) {
		return createStubInstanceContext(), nil
	}
	return setSuccessStubs()
}

func setSuccessStubs() *ConfigurePackageStubs {
	stubs := &ConfigurePackageStubs{fileSysDepStub: fileSysStubSuccess(), networkDepStub: networkStubSuccess(), execDepStub: execStubSuccess()}
	stubs.Set()
	return stubs
}

type FileSysDepStub struct {
	makeFileError        error
	directoriesResult    []string
	directoriesError     error
	filesResult          []string
	filesError           error
	existsResultDefault  bool
	existsResultSequence []bool
	uncompressError      error
	removeError          error
	renameError          error
	readResult           []byte
	readError            error
	writeError           error
}

func (m *FileSysDepStub) MakeDirExecute(destinationDir string) (err error) {
	return m.makeFileError
}

func (m *FileSysDepStub) GetDirectoryNames(srcPath string) (directories []string, err error) {
	return m.directoriesResult, m.directoriesError
}

func (m *FileSysDepStub) GetFileNames(srcPath string) (files []string, err error) {
	return m.filesResult, m.filesError
}

func (m *FileSysDepStub) Exists(filePath string) bool {
	if len(m.existsResultSequence) > 0 {
		result := m.existsResultSequence[0]
		if len(m.existsResultSequence) > 1 {
			m.existsResultSequence = append(m.existsResultSequence[:0], m.existsResultSequence[1:]...)
		} else {
			m.existsResultSequence = nil
		}
		return result
	}
	return m.existsResultDefault
}

func (m *FileSysDepStub) Uncompress(src, dest string) error {
	return m.uncompressError
}

func (m *FileSysDepStub) RemoveAll(path string) error {
	return m.removeError
}

func (m *FileSysDepStub) Rename(oldpath, newpath string) error {
	return m.renameError
}

func (m *FileSysDepStub) ReadFile(filename string) ([]byte, error) {
	return m.readResult, m.readError
}

func (m *FileSysDepStub) WriteFile(filename string, content string) error {
	return m.writeError
}

type NetworkDepStub struct {
	foldersResult          []string
	foldersError           error
	downloadResultDefault  artifact.DownloadOutput
	downloadErrorDefault   error
	downloadResultSequence []artifact.DownloadOutput
	downloadErrorSequence  []error
}

func (m *NetworkDepStub) ListS3Folders(log log.T, amazonS3URL s3util.AmazonS3URL) (folderNames []string, err error) {
	return m.foldersResult, m.foldersError
}

func (m *NetworkDepStub) Download(log log.T, input artifact.DownloadInput) (output artifact.DownloadOutput, err error) {
	if len(m.downloadResultSequence) > 0 {
		result := m.downloadResultSequence[0]
		error := m.downloadErrorSequence[0]
		if len(m.downloadResultSequence) > 1 {
			m.downloadResultSequence = append(m.downloadResultSequence[:0], m.downloadResultSequence[1:]...)
			m.downloadErrorSequence = append(m.downloadErrorSequence[:0], m.downloadErrorSequence[1:]...)
		} else {
			m.downloadResultSequence = nil
			m.downloadErrorSequence = nil
		}
		return result, error
	}
	return m.downloadResultDefault, m.downloadErrorDefault
}

type ExecDepStub struct {
	pluginInput  *model.PluginState
	parseError   error
	pluginOutput *contracts.PluginResult
}

func (m *ExecDepStub) ParseDocument(context context.T, documentRaw []byte, orchestrationDir string, s3Bucket string, s3KeyPrefix string, messageID string, documentID string, defaultWorkingDirectory string) (pluginsInfo []model.PluginState, err error) {
	pluginsInfo = make([]model.PluginState, 0, 1)
	if m.pluginInput != nil {
		pluginsInfo = append(pluginsInfo, *m.pluginInput)
	}
	return pluginsInfo, m.parseError
}

func (m *ExecDepStub) ExecuteDocument(runner runpluginutil.PluginRunner, context context.T, pluginInput []model.PluginState, documentID string, documentCreatedDate string) (pluginOutputs map[string]*contracts.PluginResult) {
	pluginOutputs = make(map[string]*contracts.PluginResult)
	if m.pluginOutput != nil {
		pluginOutputs["test"] = m.pluginOutput
	}
	return
}

type MockedConfigurePackageManager struct {
	mock.Mock
	waitChan chan bool
}

func (configMock *MockedConfigurePackageManager) downloadPackage(context context.T,
	util configureUtil,
	packageName string,
	version string,
	output *contracts.PluginOutput) (filePath string, err error) {
	args := configMock.Called(util, packageName, version, output)
	return args.String(0), args.Error(1)
}

func (configMock *MockedConfigurePackageManager) validateInput(context context.T,
	input *ConfigurePackagePluginInput) (valid bool, err error) {
	args := configMock.Called(input)
	return args.Bool(0), args.Error(1)
}

func (configMock *MockedConfigurePackageManager) getVersionToInstall(context context.T,
	input *ConfigurePackagePluginInput,
	util configureUtil) (version string, installedVersion string, err error) {
	args := configMock.Called(input, util)
	ver := args.String(0)
	if strings.HasPrefix(ver, "Wait") {
		configMock.waitChan <- true
		_ = <-configMock.waitChan
		ver = strings.TrimLeft(ver, "Wait")
	}
	return ver, args.String(1), args.Error(2)
}

func (configMock *MockedConfigurePackageManager) getVersionToUninstall(context context.T,
	input *ConfigurePackagePluginInput,
	util configureUtil) (version string, err error) {
	args := configMock.Called(input, util)
	ver := args.String(0)
	if strings.HasPrefix(ver, "Wait") {
		configMock.waitChan <- true
		_ = <-configMock.waitChan
		ver = strings.TrimLeft(ver, "Wait")
	}
	return ver, args.Error(1)
}

func (configMock *MockedConfigurePackageManager) setMark(context context.T, packageName string, version string) error {
	args := configMock.Called(packageName, version)
	return args.Error(0)
}

func (configMock *MockedConfigurePackageManager) clearMark(context context.T, packageName string) {
	configMock.Called(packageName)
}

func (configMock *MockedConfigurePackageManager) ensurePackage(context context.T,
	util configureUtil,
	packageName string,
	version string,
	output *contracts.PluginOutput) (manifest *PackageManifest, err error) {
	args := configMock.Called(util, packageName, version, output)
	return args.Get(0).(*PackageManifest), args.Error(1)
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

func ConfigPackageSuccessMock(downloadFilePath string,
	versionToActOn string,
	versionCurrentlyInstalled string,
	packageManifest *PackageManifest,
	installResult contracts.ResultStatus,
	uninstallPreResult contracts.ResultStatus,
	uninstallPostResult contracts.ResultStatus) *MockedConfigurePackageManager {
	mockConfig := MockedConfigurePackageManager{}
	mockConfig.On("downloadPackage", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(downloadFilePath, nil)
	mockConfig.On("validateInput", mock.Anything, mock.Anything).Return(true, nil)
	mockConfig.On("getVersionToInstall", mock.Anything, mock.Anything, mock.Anything).Return(versionToActOn, versionCurrentlyInstalled, nil)
	mockConfig.On("getVersionToUninstall", mock.Anything, mock.Anything, mock.Anything).Return(versionToActOn, nil)
	mockConfig.On("setMark", mock.Anything, mock.Anything, mock.Anything).Return(nil)
	mockConfig.On("clearMark", mock.Anything, mock.Anything)
	mockConfig.On("ensurePackage", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(packageManifest, nil)
	mockConfig.On("runUninstallPackagePre", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(uninstallPreResult, nil)
	mockConfig.On("runInstallPackage", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(installResult, nil)
	mockConfig.On("runUninstallPackagePost", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(uninstallPostResult, nil)
	mockConfig.waitChan = make(chan bool)
	return &mockConfig
}
