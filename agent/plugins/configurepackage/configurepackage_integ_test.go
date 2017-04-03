// Copyright 2016 Amazon.com, Inc. or its affiliates. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License"). You may not
// use this file except in compliance with the License. A copy of the
// License is located at
//
// http://aws.amazon.com/apache2.0/
//
// or in the "license" file accompanying this file. This file is distributed
// on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND,
// either express or implied. See the License for the specific language governing
// permissions and limitations under the License.

// Package configurepackage implements the ConfigurePackage plugin.
package configurepackage

import (
	"errors"
	"io/ioutil"
	"testing"

	"github.com/aws/amazon-ssm-agent/agent/contracts"
	"github.com/aws/amazon-ssm-agent/agent/plugins/configurepackage/localpackages"
	"github.com/aws/amazon-ssm-agent/agent/plugins/configurepackage/localpackages/mock"
	"github.com/aws/amazon-ssm-agent/agent/statemanager/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestConfigurePackage(t *testing.T) {
	stubs := setSuccessStubs()
	defer stubs.Clear()

	plugin := &Plugin{}
	pluginInformation := createStubPluginInputInstall()

	manager := createInstance()
	instanceContext := createStubInstanceContext()

	output := runConfigurePackage(
		plugin,
		contextMock,
		manager,
		instanceContext,
		pluginInformation)

	assert.Empty(t, output.Stderr)
}

func TestConfigurePackage_InvalidAction(t *testing.T) {
	stubs := setSuccessStubs()
	defer stubs.Clear()

	plugin := &Plugin{}
	pluginInformation := createStubPluginInputFoo()

	manager := createInstance()
	instanceContext := createStubInstanceContext()

	result := runConfigurePackage(
		plugin,
		contextMock,
		manager,
		instanceContext,
		pluginInformation)

	assert.Contains(t, result.Stderr, "unsupported action")
}

func TestConfigurePackage_InvalidRawInput(t *testing.T) {
	stubs := setSuccessStubs()
	defer stubs.Clear()

	plugin := &Plugin{}
	// string value will fail the Remarshal as it's not ConfigurePackagePluginInput
	pluginInformation := "invalid value"

	manager := createInstance()
	instanceContext := createStubInstanceContext()

	result := runConfigurePackage(
		plugin,
		contextMock,
		manager,
		instanceContext,
		pluginInformation)

	assert.Contains(t, result.Stderr, "invalid format in plugin properties")
}

func TestConfigurePackage_InvalidInput(t *testing.T) {
	stubs := setSuccessStubs()
	defer stubs.Clear()

	plugin := &Plugin{}
	pluginInformation := createStubInvalidPluginInput()

	manager := createInstance()
	instanceContext := createStubInstanceContext()

	result := runConfigurePackage(
		plugin,
		contextMock,
		manager,
		instanceContext,
		pluginInformation)

	assert.Contains(t, result.Stderr, "invalid input")
}

func TestInstallPackage_DownloadFailed(t *testing.T) {
	stubs := &ConfigurePackageStubs{
		fileSysDepStub: &FileSysDepStub{existsResultDefault: false},
		networkDepStub: &NetworkDepStub{downloadErrorDefault: errors.New("Cannot download package")},
		execDepStub:    execStubSuccess(),
	}
	stubs.Set()
	defer stubs.Clear()

	plugin := &Plugin{}
	pluginInformation := createStubPluginInputInstall()

	manager := createInstance()
	instanceContext := createStubInstanceContext()

	output := runConfigurePackage(
		plugin,
		contextMock,
		manager,
		instanceContext,
		pluginInformation)

	assert.NotEmpty(t, output.Stderr, output.Stdout)
}

func TestInstallPackage_AlreadyInstalled(t *testing.T) {
	stubs := setSuccessStubs()
	defer stubs.Clear()

	plugin := &Plugin{}
	pluginInformation := createStubPluginInputInstall()

	action, _ := ioutil.ReadFile("testdata/sampleAction.json")

	mockRepo := repository_mock.MockedRepository{}
	mockRepo.On("GetInstalledVersion", mock.Anything, pluginInformation.Name).Return(pluginInformation.Version)
	mockRepo.On("GetInstallState", mock.Anything, pluginInformation.Name).Return(localpackages.Installed, pluginInformation.Version)
	mockRepo.On("ValidatePackage", mock.Anything, pluginInformation.Name, pluginInformation.Version).Return(nil)
	mockRepo.On("GetAction", mock.Anything, pluginInformation.Name, pluginInformation.Version, "validate").Return(true, action, `foo`, nil)

	manager := createInstanceWithRepoMock(&mockRepo)
	instanceContext := createStubInstanceContext()

	output := runConfigurePackage(
		plugin,
		contextMock,
		manager,
		instanceContext,
		pluginInformation)

	mockRepo.AssertExpectations(t)
	assert.Empty(t, output.Stderr)
}

func TestInstallPackage_Repair(t *testing.T) {
	stubs := &ConfigurePackageStubs{
		fileSysDepStub: fileSysStubSuccessNewPackage(),
		networkDepStub: networkStubSuccess(),
		execDepStub: &ExecDepStub{pluginInput: &model.PluginState{},
			pluginOutputMap: map[string]*contracts.PluginResult{
				"validate1": {Status: contracts.ResultStatusFailed},
				"install":   {Status: contracts.ResultStatusSuccess},
				"validate2": {Status: contracts.ResultStatusSuccess},
			},
		},
	}
	stubs.Set()
	defer stubs.Clear()

	plugin := &Plugin{}
	pluginInformation := createStubPluginInputInstall()

	action, _ := ioutil.ReadFile("testdata/sampleAction.json")

	mockRepo := repository_mock.MockedRepository{}
	mockRepo.On("GetInstalledVersion", mock.Anything, pluginInformation.Name).Return(pluginInformation.Version)
	mockRepo.On("GetInstallState", mock.Anything, pluginInformation.Name).Return(localpackages.Installed, pluginInformation.Version)
	mockRepo.On("ValidatePackage", mock.Anything, pluginInformation.Name, pluginInformation.Version).Return(nil)
	mockRepo.On("GetAction", mock.Anything, pluginInformation.Name, pluginInformation.Version, "validate").Return(true, action, `validate1`, nil).Once()
	mockRepo.On("GetAction", mock.Anything, pluginInformation.Name, pluginInformation.Version, "install").Return(true, action, `install`, nil).Once()
	mockRepo.On("GetAction", mock.Anything, pluginInformation.Name, pluginInformation.Version, "validate").Return(true, action, `validate2`, nil).Once()
	mockRepo.On("SetInstallState", mock.Anything, pluginInformation.Name, pluginInformation.Version, mock.Anything).Return(nil)

	manager := createInstanceWithRepoMock(&mockRepo)
	instanceContext := createStubInstanceContext()

	output := runConfigurePackage(
		plugin,
		contextMock,
		manager,
		instanceContext,
		pluginInformation)

	mockRepo.AssertExpectations(t)
	assert.Empty(t, output.Stderr)
}

func TestInstallPackage_RetryFailedLatest(t *testing.T) {
	stubs := &ConfigurePackageStubs{
		fileSysDepStub: fileSysStubSuccessNewPackage(),
		networkDepStub: networkStubSuccess(),
		execDepStub:    execStubSuccess(),
	}
	stubs.Set()
	defer stubs.Clear()

	plugin := &Plugin{}
	pluginInformation := createStubPluginInputInstallLatest()
	version := "1.0.0"

	action, _ := ioutil.ReadFile("testdata/sampleAction.json")

	mockRepo := repository_mock.MockedRepository{}
	mockRepo.On("GetInstalledVersion", mock.Anything, pluginInformation.Name).Return(version)
	mockRepo.On("GetInstallState", mock.Anything, pluginInformation.Name).Return(localpackages.Failed, version)
	mockRepo.On("RefreshPackage", mock.Anything, pluginInformation.Name, version, mock.Anything).Return(nil)
	mockRepo.On("ValidatePackage", mock.Anything, pluginInformation.Name, version).Return(nil)
	mockRepo.On("GetAction", mock.Anything, pluginInformation.Name, version, "install").Return(true, action, `install`, nil).Once()
	mockRepo.On("GetAction", mock.Anything, pluginInformation.Name, version, "validate").Return(true, action, `validate`, nil).Once()
	mockRepo.On("SetInstallState", mock.Anything, pluginInformation.Name, version, mock.Anything).Return(nil)

	manager := createInstanceWithRepoMock(&mockRepo)
	instanceContext := createStubInstanceContext()

	output := runConfigurePackage(
		plugin,
		contextMock,
		manager,
		instanceContext,
		pluginInformation)

	mockRepo.AssertExpectations(t)
	assert.Empty(t, output.Stderr)
}

func TestInstallPackage_ExtractFailed(t *testing.T) {
	result, _ := ioutil.ReadFile("testdata/sampleManifest.json")
	stubs := &ConfigurePackageStubs{
		fileSysDepStub: &FileSysDepStub{readResult: result, uncompressError: errors.New("Cannot extract package")},
		networkDepStub: networkStubSuccess(),
		execDepStub:    execStubSuccess(),
	}
	stubs.Set()
	defer stubs.Clear()

	plugin := &Plugin{}
	pluginInformation := createStubPluginInputInstall()

	manager := createInstance()
	instanceContext := createStubInstanceContext()

	output := runConfigurePackage(
		plugin,
		contextMock,
		manager,
		instanceContext,
		pluginInformation)

	assert.NotEmpty(t, output.Stderr)
	assert.Contains(t, output.Stderr, "Cannot extract package")
}

func TestInstallPackage_DeleteFailed(t *testing.T) {
	result, _ := ioutil.ReadFile("testdata/sampleManifest.json")
	stubs := &ConfigurePackageStubs{
		fileSysDepStub: &FileSysDepStub{
			readResult:           result,
			existsResultSequence: []bool{false, false},
			existsResultDefault:  true,
			removeError:          errors.New("failed to delete compressed package"),
		},
		networkDepStub: networkStubSuccess(),
		execDepStub:    execStubSuccess(),
	}
	stubs.Set()
	defer stubs.Clear()

	plugin := &Plugin{}
	pluginInformation := createStubPluginInputInstall()

	manager := createInstance()
	instanceContext := createStubInstanceContext()

	output := runConfigurePackage(
		plugin,
		contextMock,
		manager,
		instanceContext,
		pluginInformation)

	assert.NotEmpty(t, output.Stderr)
	assert.Contains(t, output.Stderr, "failed to delete compressed package")
}

func TestInstallPackage_Reboot(t *testing.T) {
	stubs := &ConfigurePackageStubs{
		fileSysDepStub: fileSysStubSuccessNewPackage(),
		networkDepStub: networkStubSuccess(),
		execDepStub: &ExecDepStub{pluginInput: &model.PluginState{},
			pluginOutputMap: map[string]*contracts.PluginResult{
				"install": {Status: contracts.ResultStatusSuccessAndReboot},
			},
		},
	}
	stubs.Set()
	defer stubs.Clear()

	plugin := &Plugin{}
	pluginInformation := createStubPluginInputInstall()

	action, _ := ioutil.ReadFile("testdata/sampleAction.json")

	mockRepo := repository_mock.MockedRepository{}
	mockRepo.On("GetInstalledVersion", mock.Anything, pluginInformation.Name).Return("")
	mockRepo.On("GetInstallState", mock.Anything, pluginInformation.Name).Return(localpackages.None, pluginInformation.Version)
	mockRepo.On("ValidatePackage", mock.Anything, pluginInformation.Name, pluginInformation.Version).Return(nil)
	mockRepo.On("GetAction", mock.Anything, pluginInformation.Name, pluginInformation.Version, "install").Return(true, action, `install`, nil)
	mockRepo.On("SetInstallState", mock.Anything, pluginInformation.Name, pluginInformation.Version, mock.Anything).Return(nil)

	manager := createInstanceWithRepoMock(&mockRepo)
	instanceContext := createStubInstanceContext()

	output := runConfigurePackage(
		plugin,
		contextMock,
		manager,
		instanceContext,
		pluginInformation)

	mockRepo.AssertExpectations(t)
	assert.Empty(t, output.Stderr)
	assert.True(t, output.Status == contracts.ResultStatusSuccessAndReboot)
}

func TestInstallPackage_Failed(t *testing.T) {
	stubs := &ConfigurePackageStubs{
		fileSysDepStub: fileSysStubSuccessNewPackage(),
		networkDepStub: networkStubSuccess(),
		execDepStub: &ExecDepStub{pluginInput: &model.PluginState{},
			pluginOutputMap: map[string]*contracts.PluginResult{
				"install": {Status: contracts.ResultStatusFailed},
			},
		},
	}
	stubs.Set()
	defer stubs.Clear()

	plugin := &Plugin{}
	pluginInformation := createStubPluginInputInstall()

	action, _ := ioutil.ReadFile("testdata/sampleAction.json")

	mockRepo := repository_mock.MockedRepository{}
	mockRepo.On("GetInstalledVersion", mock.Anything, pluginInformation.Name).Return("")
	mockRepo.On("GetInstallState", mock.Anything, pluginInformation.Name).Return(localpackages.None, pluginInformation.Version)
	mockRepo.On("ValidatePackage", mock.Anything, pluginInformation.Name, pluginInformation.Version).Return(nil)
	mockRepo.On("GetAction", mock.Anything, pluginInformation.Name, pluginInformation.Version, "install").Return(true, action, `install`, nil)
	mockRepo.On("SetInstallState", mock.Anything, pluginInformation.Name, pluginInformation.Version, mock.Anything).Return(nil)

	manager := createInstanceWithRepoMock(&mockRepo)
	instanceContext := createStubInstanceContext()

	output := runConfigurePackage(
		plugin,
		contextMock,
		manager,
		instanceContext,
		pluginInformation)

	mockRepo.AssertExpectations(t)
	assert.NotEmpty(t, output.Stderr)
	assert.Contains(t, output.Stderr, "install action state was Failed and not Success")
}

func TestInstallPackage_Invalid(t *testing.T) {
	stubs := &ConfigurePackageStubs{
		fileSysDepStub: fileSysStubSuccessNewPackage(),
		networkDepStub: networkStubSuccess(),
		execDepStub: &ExecDepStub{pluginInput: &model.PluginState{},
			pluginOutputMap: map[string]*contracts.PluginResult{
				"install":  {Status: contracts.ResultStatusSuccess},
				"validate": {Status: contracts.ResultStatusFailed},
			},
		},
	}
	stubs.Set()
	defer stubs.Clear()

	plugin := &Plugin{}
	pluginInformation := createStubPluginInputInstall()

	action, _ := ioutil.ReadFile("testdata/sampleAction.json")

	mockRepo := repository_mock.MockedRepository{}
	mockRepo.On("GetInstalledVersion", mock.Anything, pluginInformation.Name).Return("")
	mockRepo.On("GetInstallState", mock.Anything, pluginInformation.Name).Return(localpackages.None, pluginInformation.Version)
	mockRepo.On("ValidatePackage", mock.Anything, pluginInformation.Name, pluginInformation.Version).Return(nil)
	mockRepo.On("GetAction", mock.Anything, pluginInformation.Name, pluginInformation.Version, "install").Return(true, action, `install`, nil)
	mockRepo.On("GetAction", mock.Anything, pluginInformation.Name, pluginInformation.Version, "validate").Return(true, action, `validate`, nil)
	mockRepo.On("SetInstallState", mock.Anything, pluginInformation.Name, pluginInformation.Version, mock.Anything).Return(nil)

	manager := createInstanceWithRepoMock(&mockRepo)
	instanceContext := createStubInstanceContext()

	output := runConfigurePackage(
		plugin,
		contextMock,
		manager,
		instanceContext,
		pluginInformation)

	mockRepo.AssertExpectations(t)
	assert.NotEmpty(t, output.Stderr)
	assert.Contains(t, output.Stderr, "failed to install package")
	assert.Contains(t, output.Stderr, "Validation error")
}

func TestUninstallPackage_Success(t *testing.T) {
	stubs := setSuccessStubs()
	defer stubs.Clear()

	plugin := &Plugin{}
	pluginInformation := createStubPluginInputUninstallLatest()

	manager := createInstance()
	instanceContext := createStubInstanceContext()

	output := runConfigurePackage(
		plugin,
		contextMock,
		manager,
		instanceContext,
		pluginInformation)

	assert.True(t, output.Status == contracts.ResultStatusSuccess)
	assert.Empty(t, output.Stderr)
}

func TestUninstallPackage_DoesNotExist(t *testing.T) {
	stubs := &ConfigurePackageStubs{fileSysDepStub: &FileSysDepStub{existsResultDefault: false}, networkDepStub: networkStubSuccess(), execDepStub: execStubSuccess()}
	stubs.Set()
	defer stubs.Clear()

	plugin := &Plugin{}
	pluginInformation := createStubPluginInputUninstallLatest()

	manager := createInstance()
	instanceContext := createStubInstanceContext()

	output := runConfigurePackage(
		plugin,
		contextMock,
		manager,
		instanceContext,
		pluginInformation)

	assert.Empty(t, output.Stderr)
}

func TestUninstallPackage_DoesNotExistAtAll(t *testing.T) {
	stubs := &ConfigurePackageStubs{fileSysDepStub: &FileSysDepStub{existsResultDefault: false}, networkDepStub: &NetworkDepStub{foldersResult: []string{}}, execDepStub: execStubSuccess()}
	stubs.Set()
	defer stubs.Clear()

	plugin := &Plugin{}
	pluginInformation := createStubPluginInputUninstallLatest()

	manager := createInstance()
	instanceContext := createStubInstanceContext()

	output := runConfigurePackage(
		plugin,
		contextMock,
		manager,
		instanceContext,
		pluginInformation)

	assert.NotEmpty(t, output.Stderr)
	assert.Contains(t, output.Stderr, "unable to determine version")
}

func TestUninstallPackage_RemovalFailed(t *testing.T) {
	result, _ := ioutil.ReadFile("testdata/sampleManifest.json")
	stubs := &ConfigurePackageStubs{
		fileSysDepStub: &FileSysDepStub{readResult: result, existsResultDefault: true, removeError: errors.New("404"), filesResult: []string{"PVDriver.json", "install.json"}},
		networkDepStub: networkStubSuccess(),
		execDepStub:    execStubSuccess(),
	}
	stubs.Set()
	defer stubs.Clear()

	plugin := &Plugin{}
	pluginInformation := createStubPluginInputUninstall()

	manager := createInstance()
	instanceContext := createStubInstanceContext()

	output := runConfigurePackage(
		plugin,
		contextMock,
		manager,
		instanceContext,
		pluginInformation)

	assert.NotEmpty(t, output.Stderr)
	assert.Contains(t, output.Stderr, "failed to uninstall package")
	assert.Contains(t, output.Stderr, "404")
}

func TestConfigurePackage_ExecuteError(t *testing.T) {
	stubs := &ConfigurePackageStubs{
		fileSysDepStub: fileSysStubSuccessNewPackage(),
		networkDepStub: networkStubSuccess(),
		execDepStub:    &ExecDepStub{pluginInput: &model.PluginState{}, pluginOutput: &contracts.PluginResult{StandardError: "execute error"}},
	}
	stubs.Set()
	defer stubs.Clear()

	plugin := &Plugin{}
	pluginInformation := createStubPluginInputInstall()

	manager := createInstance()
	instanceContext := createStubInstanceContext()

	output := runConfigurePackage(
		plugin,
		contextMock,
		manager,
		instanceContext,
		pluginInformation)

	assert.Empty(t, output.Stderr)
	assert.NotEmpty(t, output.Stdout)
	assert.Contains(t, output.Stdout, "execute error")
}
