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
	"testing"

	"github.com/aws/amazon-ssm-agent/agent/contracts"
	"github.com/aws/amazon-ssm-agent/agent/plugins/configurepackage/installer/mock"
	"github.com/aws/amazon-ssm-agent/agent/plugins/configurepackage/localpackages"
	"github.com/aws/amazon-ssm-agent/agent/plugins/configurepackage/localpackages/mock"
	"github.com/aws/amazon-ssm-agent/agent/plugins/configurepackage/packageservice"
	"github.com/aws/amazon-ssm-agent/agent/plugins/configurepackage/packageservice/mock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func createStubPluginInputFoo() *ConfigurePackagePluginInput {
	input := ConfigurePackagePluginInput{}

	input.Version = "1.0.0"
	input.Name = "PVDriver"
	input.Action = "Foo"

	return &input
}

func createStubPluginInputInstallLatest() *ConfigurePackagePluginInput {
	input := ConfigurePackagePluginInput{}

	input.Name = "PVDriver"
	input.Action = "Install"

	return &input
}

func createStubPluginInputUninstallLatest() *ConfigurePackagePluginInput {
	input := ConfigurePackagePluginInput{}

	input.Name = "PVDriver"
	input.Action = "Uninstall"

	return &input
}

func createStubInvalidPluginInput() *ConfigurePackagePluginInput {
	input := ConfigurePackagePluginInput{}

	input.Version = "7.2"
	input.Name = ""
	input.Action = "InvalidAction"

	return &input
}

func TestConfigurePackage(t *testing.T) {
	stubs := setSuccessStubs()
	defer stubs.Clear()

	plugin := &Plugin{}
	pluginInformation := createStubPluginInputInstall()

	mockRepo := repository_mock.MockedRepository{}
	mockRepo.On("GetInstalledVersion", mock.Anything, pluginInformation.Name).Return("")
	mockRepo.On("GetInstallState", mock.Anything, pluginInformation.Name).Return(localpackages.None, "")
	mockRepo.On("ValidatePackage", mock.Anything, pluginInformation.Name, pluginInformation.Version).Return(nil)

	mockInst := installer_mock.Mock{}
	mockInst.On("Install", mock.Anything).Return(&contracts.PluginOutput{Status: contracts.ResultStatusSuccess}).Once()
	mockInst.On("Validate", mock.Anything).Return(&contracts.PluginOutput{Status: contracts.ResultStatusSuccess}).Once()
	mockRepo.On("GetInstaller", mock.Anything, mock.Anything, mock.Anything, pluginInformation.Name, pluginInformation.Version).Return(&mockInst)

	mockRepo.On("SetInstallState", mock.Anything, pluginInformation.Name, pluginInformation.Version, mock.Anything).Return(nil)

	manager := createInstanceWithRepoMock(&mockRepo)

	output := runConfigurePackage(
		plugin,
		contextMock,
		manager,
		pluginInformation)

	mockRepo.AssertExpectations(t)
	assert.Empty(t, output.Stderr)
}

func TestConfigurePackage_InvalidAction(t *testing.T) {
	stubs := setSuccessStubs()
	defer stubs.Clear()

	plugin := &Plugin{}
	pluginInformation := createStubPluginInputFoo()

	manager := createInstance()

	result := runConfigurePackage(
		plugin,
		contextMock,
		manager,
		pluginInformation)

	assert.Contains(t, result.Stderr, "unsupported action")
}

func TestConfigurePackage_InvalidRawInput(t *testing.T) {
	// string value will fail the Remarshal as it's not ConfigurePackagePluginInput
	pluginInformation := "invalid value"

	result, err := parseAndValidateInput(pluginInformation)
	assert.Contains(t, err.Error(), "invalid format in plugin properties")
	assert.Nil(t, result)
}

func TestConfigurePackage_InvalidInput(t *testing.T) {
	pluginInformation := createStubInvalidPluginInput()

	result, err := parseAndValidateInput(pluginInformation)
	assert.Contains(t, err.Error(), "invalid input")
	assert.Nil(t, result)
}

func TestInstallPackage_DownloadFailed(t *testing.T) {
	stubs := setSuccessStubs()
	defer stubs.Clear()

	plugin := &Plugin{}
	pluginInformation := createStubPluginInputInstall()

	mockRepo := repository_mock.MockedRepository{}
	mockRepo.On("GetInstalledVersion", mock.Anything, pluginInformation.Name).Return("")
	mockRepo.On("GetInstallState", mock.Anything, pluginInformation.Name).Return(localpackages.None, "")
	mockRepo.On("ValidatePackage", mock.Anything, pluginInformation.Name, pluginInformation.Version).Return(errors.New("not valid")).Once()
	mockRepo.On("RefreshPackage", mock.Anything, pluginInformation.Name, pluginInformation.Version, mock.Anything).Return(errors.New("failed")).Once()

	manager := createInstanceWithRepoMock(&mockRepo)

	output := runConfigurePackage(
		plugin,
		contextMock,
		manager,
		pluginInformation)

	mockRepo.AssertExpectations(t)
	assert.NotEmpty(t, output.Stderr, output.Stdout)
}

func TestInstallPackage_AlreadyInstalled(t *testing.T) {
	stubs := setSuccessStubs()
	defer stubs.Clear()

	plugin := &Plugin{}
	pluginInformation := createStubPluginInputInstall()

	mockRepo := repository_mock.MockedRepository{}
	mockRepo.On("GetInstalledVersion", mock.Anything, pluginInformation.Name).Return(pluginInformation.Version)
	mockRepo.On("GetInstallState", mock.Anything, pluginInformation.Name).Return(localpackages.Installed, pluginInformation.Version)
	mockRepo.On("ValidatePackage", mock.Anything, pluginInformation.Name, pluginInformation.Version).Return(nil)

	mockInst := installer_mock.Mock{}
	mockInst.On("Validate", mock.Anything).Return(&contracts.PluginOutput{Status: contracts.ResultStatusSuccess}).Once()
	mockRepo.On("GetInstaller", mock.Anything, mock.Anything, mock.Anything, pluginInformation.Name, pluginInformation.Version).Return(&mockInst)

	manager := createInstanceWithRepoMock(&mockRepo)

	output := runConfigurePackage(
		plugin,
		contextMock,
		manager,
		pluginInformation)

	mockRepo.AssertExpectations(t)
	assert.Empty(t, output.Stderr)
}

func TestInstallPackage_Repair(t *testing.T) {
	stubs := &ConfigurePackageStubs{
		fileSysDepStub: &FileSysDepStub{},
	}
	stubs.Set()
	defer stubs.Clear()

	plugin := &Plugin{}
	pluginInformation := createStubPluginInputInstall()

	mockRepo := repository_mock.MockedRepository{}
	mockRepo.On("GetInstalledVersion", mock.Anything, pluginInformation.Name).Return(pluginInformation.Version)
	mockRepo.On("GetInstallState", mock.Anything, pluginInformation.Name).Return(localpackages.Installed, pluginInformation.Version)
	mockRepo.On("ValidatePackage", mock.Anything, pluginInformation.Name, pluginInformation.Version).Return(nil)
	mockInst := installer_mock.Mock{}
	mockInst.On("Validate", mock.Anything).Return(&contracts.PluginOutput{Status: contracts.ResultStatusFailed}).Once()
	mockInst.On("Install", mock.Anything).Return(&contracts.PluginOutput{Status: contracts.ResultStatusSuccess}).Once()
	mockInst.On("Validate", mock.Anything).Return(&contracts.PluginOutput{Status: contracts.ResultStatusSuccess}).Once()
	mockRepo.On("GetInstaller", mock.Anything, mock.Anything, mock.Anything, pluginInformation.Name, pluginInformation.Version).Return(&mockInst)
	mockRepo.On("SetInstallState", mock.Anything, pluginInformation.Name, pluginInformation.Version, mock.Anything).Return(nil)

	manager := createInstanceWithRepoMock(&mockRepo)

	output := runConfigurePackage(
		plugin,
		contextMock,
		manager,
		pluginInformation)

	mockRepo.AssertExpectations(t)
	assert.Empty(t, output.Stderr)
}

func TestInstallPackage_RetryFailedLatest(t *testing.T) {
	stubs := &ConfigurePackageStubs{
		fileSysDepStub: &FileSysDepStub{},
	}
	stubs.Set()
	defer stubs.Clear()

	plugin := &Plugin{}
	pluginInformation := createStubPluginInputInstallLatest()
	version := "1.0.0"

	mockRepo := repository_mock.MockedRepository{}
	mockRepo.On("GetInstalledVersion", mock.Anything, pluginInformation.Name).Return(version)
	mockRepo.On("GetInstallState", mock.Anything, pluginInformation.Name).Return(localpackages.Failed, version)
	mockRepo.On("RefreshPackage", mock.Anything, pluginInformation.Name, version, mock.Anything).Return(nil)
	mockRepo.On("ValidatePackage", mock.Anything, pluginInformation.Name, version).Return(nil)
	mockInst := installer_mock.Mock{}
	mockInst.On("Install", mock.Anything).Return(&contracts.PluginOutput{Status: contracts.ResultStatusSuccess}).Once()
	mockInst.On("Validate", mock.Anything).Return(&contracts.PluginOutput{Status: contracts.ResultStatusSuccess}).Once()
	mockRepo.On("GetInstaller", mock.Anything, mock.Anything, mock.Anything, pluginInformation.Name, version).Return(&mockInst)
	mockRepo.On("SetInstallState", mock.Anything, pluginInformation.Name, version, mock.Anything).Return(nil)

	mockDS := packageservice_mock.Mock{}
	mockDS.On("DownloadManifest", mock.Anything, pluginInformation.Name, packageservice.Latest).Return(version, nil)

	manager := createInstanceWithRepoAndDSMock(&mockRepo, &mockDS)

	output := runConfigurePackage(
		plugin,
		contextMock,
		manager,
		pluginInformation)

	mockRepo.AssertExpectations(t)
	assert.Empty(t, output.Stderr)
}

/*
// TODO: requires to change structure: no callback for download is passed to localRepository
func TestInstallPackage_ExtractFailed(t *testing.T) {
	result, _ := ioutil.ReadFile("testdata/sampleManifest.json")
	stubs := &ConfigurePackageStubs{
		fileSysDepStub: &FileSysDepStub{readResult: result, uncompressError: errors.New("Cannot extract package")},
		execDepStub:    execStubSuccess(),
	}
	stubs.Set()
	defer stubs.Clear()

	plugin := &Plugin{}
	pluginInformation := createStubPluginInputInstall()

	manager := createInstance()

	output := runConfigurePackage(
		plugin,
		contextMock,
		manager,
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
		execDepStub: execStubSuccess(),
	}
	stubs.Set()
	defer stubs.Clear()

	plugin := &Plugin{}
	pluginInformation := createStubPluginInputInstall()

	manager := createInstance()

	output := runConfigurePackage(
		plugin,
		contextMock,
		manager,
		pluginInformation)

	assert.NotEmpty(t, output.Stderr)
	assert.Contains(t, output.Stderr, "failed to delete compressed package")
}
*/

func TestInstallPackage_Reboot(t *testing.T) {
	stubs := &ConfigurePackageStubs{
		fileSysDepStub: &FileSysDepStub{},
	}
	stubs.Set()
	defer stubs.Clear()

	plugin := &Plugin{}
	pluginInformation := createStubPluginInputInstall()

	mockRepo := repository_mock.MockedRepository{}
	mockRepo.On("GetInstalledVersion", mock.Anything, pluginInformation.Name).Return("")
	mockRepo.On("GetInstallState", mock.Anything, pluginInformation.Name).Return(localpackages.None, pluginInformation.Version)
	mockRepo.On("ValidatePackage", mock.Anything, pluginInformation.Name, pluginInformation.Version).Return(nil)
	mockInst := installer_mock.Mock{}
	mockInst.On("Install", mock.Anything).Return(&contracts.PluginOutput{Status: contracts.ResultStatusSuccessAndReboot}).Once()
	mockRepo.On("GetInstaller", mock.Anything, mock.Anything, mock.Anything, pluginInformation.Name, pluginInformation.Version).Return(&mockInst)
	mockRepo.On("SetInstallState", mock.Anything, pluginInformation.Name, pluginInformation.Version, mock.Anything).Return(nil)

	manager := createInstanceWithRepoMock(&mockRepo)

	output := runConfigurePackage(
		plugin,
		contextMock,
		manager,
		pluginInformation)

	mockRepo.AssertExpectations(t)
	assert.Empty(t, output.Stderr)
	assert.True(t, output.Status == contracts.ResultStatusSuccessAndReboot)
}

func TestInstallPackage_Failed(t *testing.T) {
	stubs := &ConfigurePackageStubs{
		fileSysDepStub: &FileSysDepStub{},
	}
	stubs.Set()
	defer stubs.Clear()

	plugin := &Plugin{}
	pluginInformation := createStubPluginInputInstall()

	mockRepo := repository_mock.MockedRepository{}
	mockRepo.On("GetInstalledVersion", mock.Anything, pluginInformation.Name).Return("")
	mockRepo.On("GetInstallState", mock.Anything, pluginInformation.Name).Return(localpackages.None, pluginInformation.Version)
	mockRepo.On("ValidatePackage", mock.Anything, pluginInformation.Name, pluginInformation.Version).Return(nil)
	mockInst := installer_mock.Mock{}
	mockInst.On("Install", mock.Anything).Return(&contracts.PluginOutput{Status: contracts.ResultStatusFailed}).Once()
	mockRepo.On("GetInstaller", mock.Anything, mock.Anything, mock.Anything, pluginInformation.Name, pluginInformation.Version).Return(&mockInst)
	mockRepo.On("SetInstallState", mock.Anything, pluginInformation.Name, pluginInformation.Version, mock.Anything).Return(nil)

	manager := createInstanceWithRepoMock(&mockRepo)

	output := runConfigurePackage(
		plugin,
		contextMock,
		manager,
		pluginInformation)

	mockRepo.AssertExpectations(t)
	assert.NotEmpty(t, output.Stderr)
	assert.Contains(t, output.Stderr, "install action was not successful")
}

func TestInstallPackage_Invalid(t *testing.T) {
	stubs := &ConfigurePackageStubs{
		fileSysDepStub: &FileSysDepStub{},
	}
	stubs.Set()
	defer stubs.Clear()

	plugin := &Plugin{}
	pluginInformation := createStubPluginInputInstall()

	mockRepo := repository_mock.MockedRepository{}
	mockRepo.On("GetInstalledVersion", mock.Anything, pluginInformation.Name).Return("")
	mockRepo.On("GetInstallState", mock.Anything, pluginInformation.Name).Return(localpackages.None, pluginInformation.Version)
	mockRepo.On("ValidatePackage", mock.Anything, pluginInformation.Name, pluginInformation.Version).Return(nil)
	mockInst := installer_mock.Mock{}
	mockInst.On("Install", mock.Anything).Return(&contracts.PluginOutput{Status: contracts.ResultStatusSuccess}).Once()
	mockInst.On("Validate", mock.Anything).Return(&contracts.PluginOutput{Status: contracts.ResultStatusFailed}).Once()
	mockRepo.On("GetInstaller", mock.Anything, mock.Anything, mock.Anything, pluginInformation.Name, pluginInformation.Version).Return(&mockInst)
	mockRepo.On("SetInstallState", mock.Anything, pluginInformation.Name, pluginInformation.Version, mock.Anything).Return(nil)

	manager := createInstanceWithRepoMock(&mockRepo)

	output := runConfigurePackage(
		plugin,
		contextMock,
		manager,
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

	mockRepo := repository_mock.MockedRepository{}
	mockRepo.On("GetInstalledVersion", mock.Anything, pluginInformation.Name).Return("someversion")
	mockRepo.On("GetInstallState", mock.Anything, pluginInformation.Name).Return(localpackages.Installed, pluginInformation.Version)
	mockRepo.On("ValidatePackage", mock.Anything, pluginInformation.Name, "someversion").Return(nil)
	mockInst := installer_mock.Mock{}
	mockInst.On("Uninstall", mock.Anything).Return(&contracts.PluginOutput{Status: contracts.ResultStatusSuccess}).Once()
	mockRepo.On("GetInstaller", mock.Anything, mock.Anything, mock.Anything, pluginInformation.Name, "someversion").Return(&mockInst)
	mockRepo.On("SetInstallState", mock.Anything, pluginInformation.Name, "someversion", localpackages.Uninstalling).Return(nil)
	mockRepo.On("RemovePackage", mock.Anything, pluginInformation.Name, "someversion").Return(nil)
	mockRepo.On("SetInstallState", mock.Anything, pluginInformation.Name, "someversion", localpackages.Uninstalled).Return(nil)

	manager := createInstanceWithRepoMock(&mockRepo)

	output := runConfigurePackage(
		plugin,
		contextMock,
		manager,
		pluginInformation)

	mockRepo.AssertExpectations(t)
	assert.True(t, output.Status == contracts.ResultStatusSuccess)
	assert.Empty(t, output.Stderr)
}

func TestUninstallPackage_DoesNotExist(t *testing.T) {
	stubs := setSuccessStubs()
	defer stubs.Clear()

	plugin := &Plugin{}
	pluginInformation := createStubPluginInputUninstallLatest()

	mockRepo := repository_mock.MockedRepository{}
	mockRepo.On("GetInstalledVersion", mock.Anything, pluginInformation.Name).Return("someversion")
	mockRepo.On("GetInstallState", mock.Anything, pluginInformation.Name).Return(localpackages.None, "")
	mockRepo.On("ValidatePackage", mock.Anything, pluginInformation.Name, "someversion").Return(errors.New("invalid"))
	mockRepo.On("RefreshPackage", mock.Anything, pluginInformation.Name, "someversion", mock.Anything).Return(nil)
	mockInst := installer_mock.Mock{}
	mockInst.On("Uninstall", mock.Anything).Return(&contracts.PluginOutput{Status: contracts.ResultStatusSuccess}).Once()
	mockRepo.On("GetInstaller", mock.Anything, mock.Anything, mock.Anything, pluginInformation.Name, "someversion").Return(&mockInst)
	mockRepo.On("SetInstallState", mock.Anything, pluginInformation.Name, "someversion", localpackages.Uninstalling).Return(nil)
	mockRepo.On("RemovePackage", mock.Anything, pluginInformation.Name, "someversion").Return(nil)
	mockRepo.On("SetInstallState", mock.Anything, pluginInformation.Name, "someversion", localpackages.Uninstalled).Return(nil)

	manager := createInstanceWithRepoMock(&mockRepo)

	output := runConfigurePackage(
		plugin,
		contextMock,
		manager,
		pluginInformation)

	mockRepo.AssertExpectations(t)
	assert.Empty(t, output.Stderr)
}

/*
TODO: currently there is no check for skip uninstall if package is not installed
func TestUninstallPackage_DoesNotExistAtAll(t *testing.T) {
	stubs := &ConfigurePackageStubs{fileSysDepStub: &FileSysDepStub{existsResultDefault: false}, execDepStub: execStubSuccess()}
	stubs.Set()
	defer stubs.Clear()

	plugin := &Plugin{}
	pluginInformation := createStubPluginInputUninstallLatest()

	manager := createInstance()

	output := runConfigurePackage(
		plugin,
		contextMock,
		manager,
		pluginInformation)

	assert.NotEmpty(t, output.Stderr)
	assert.Contains(t, output.Stderr, "unable to determine version")
}
*/

func TestUninstallPackage_RemovalFailed(t *testing.T) {
	stubs := setSuccessStubs()
	defer stubs.Clear()

	plugin := &Plugin{}
	pluginInformation := createStubPluginInputUninstall()

	mockRepo := repository_mock.MockedRepository{}
	mockRepo.On("GetInstalledVersion", mock.Anything, pluginInformation.Name).Return(pluginInformation.Version)
	mockRepo.On("GetInstallState", mock.Anything, pluginInformation.Name).Return(localpackages.Installed, pluginInformation.Version)
	mockRepo.On("ValidatePackage", mock.Anything, pluginInformation.Name, pluginInformation.Version).Return(nil)

	mockInst := installer_mock.Mock{}
	mockInst.On("Uninstall", mock.Anything).Return(&contracts.PluginOutput{Status: contracts.ResultStatusSuccess}).Once()
	mockRepo.On("GetInstaller", mock.Anything, mock.Anything, mock.Anything, pluginInformation.Name, pluginInformation.Version).Return(&mockInst)
	mockRepo.On("SetInstallState", mock.Anything, pluginInformation.Name, pluginInformation.Version, localpackages.Uninstalling).Return(nil)
	mockRepo.On("RemovePackage", mock.Anything, pluginInformation.Name, pluginInformation.Version).Return(errors.New("testfail"))
	mockRepo.On("SetInstallState", mock.Anything, pluginInformation.Name, pluginInformation.Version, localpackages.Failed).Return(nil)

	manager := createInstanceWithRepoMock(&mockRepo)

	output := runConfigurePackage(
		plugin,
		contextMock,
		manager,
		pluginInformation)

	mockRepo.AssertExpectations(t)
	assert.NotEmpty(t, output.Stderr)
	assert.Contains(t, output.Stderr, "failed to uninstall package")
	assert.Contains(t, output.Stderr, "testfail")
}
