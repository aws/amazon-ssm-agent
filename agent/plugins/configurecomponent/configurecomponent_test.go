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

// Package configurecomponents implements the ConfigureComponent plugin.
package configurecomponent

import (
	"fmt"
	"testing"

	"time"

	"github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/aws/amazon-ssm-agent/agent/contracts"
	"github.com/aws/amazon-ssm-agent/agent/fileutil/artifact"
	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/amazon-ssm-agent/agent/task"
	"github.com/aws/amazon-ssm-agent/agent/updateutil"
	"github.com/stretchr/testify/assert"
)

var logger = log.NewMockLog()

func TestMarkAsSucceeded(t *testing.T) {
	output := ConfigureComponentPluginOutput{}

	output.MarkAsSucceeded()

	assert.Equal(t, output.ExitCode, 0)
	assert.Equal(t, output.Status, contracts.ResultStatusSuccess)
}

func TestMarkAsFailed(t *testing.T) {
	output := ConfigureComponentPluginOutput{}

	output.MarkAsFailed(logger, fmt.Errorf("Error message"))

	assert.Equal(t, output.ExitCode, 1)
	assert.Equal(t, output.Status, contracts.ResultStatusFailed)
	assert.Contains(t, output.Stderr, "Error message")
}

func TestAppendInfo(t *testing.T) {
	output := ConfigureComponentPluginOutput{}

	output.AppendInfo(logger, "Info message")

	assert.Contains(t, output.Stdout, "Info message")
}

func TestConfigureComponent(t *testing.T) {
	plugin := &Plugin{}
	pluginInformation := createStubPluginInput()

	manifest, _ := parseComponentManifest(logger, "testdata/sampleManifest.json")

	manager := &mockConfigureManager{
		downloadManifestResult: manifest,
		downloadManifestError:  nil,
		downloadPackageResult:  "testdata/sampleManifest.json",
		downloadPackageError:   nil,
	}
	configureUtil := &mockConfigureUtility{}
	updateUtil := &mockUpdateUtility{}

	fileUncompress = func(src, dest string) error {
		return nil
	}

	fileRename = func(oldpath, newpath string) error {
		return nil
	}

	output := configureComponent(
		plugin,
		logger,
		manager,
		configureUtil,
		updateUtil,
		pluginInformation)

	assert.Empty(t, output.Stderr)
	assert.Empty(t, output.Errors)
}

func TestConfigureComponent_InvalidInput(t *testing.T) {
	plugin := &Plugin{}

	manager := &configureManager{}
	configureUtil := &mockConfigureUtility{}
	updateUtil := &mockUpdateUtility{}

	// string value will fail the Remarshal as it's not ConfigureComponentPluginInput
	rawPluginInput := "invalid value"

	result := configureComponent(plugin,
		logger,
		manager,
		configureUtil,
		updateUtil,
		rawPluginInput)

	assert.Contains(t, result.Stderr, "invalid format in plugin properties")
}

func TestConfigureComponent_FailedDownloadManifest(t *testing.T) {
	plugin := &Plugin{}
	pluginInformation := createStubPluginInput()

	manager := &mockConfigureManager{
		downloadManifestResult: nil,
		downloadManifestError:  fmt.Errorf("Cannot download manifest"),
		downloadPackageResult:  "",
		downloadPackageError:   nil,
	}
	configureUtil := &mockConfigureUtility{}
	updateUtil := &mockUpdateUtility{}

	fileRename = func(oldpath, newpath string) error {
		return nil
	}

	output := configureComponent(
		plugin,
		logger,
		manager,
		configureUtil,
		updateUtil,
		pluginInformation)

	assert.NotEmpty(t, output.Stderr)
	assert.NotEmpty(t, output.Errors)
}

func TestExecute(t *testing.T) {
	pluginInformation := createStubPluginInput()
	config := contracts.Configuration{}
	p := make([]interface{}, 1)
	p[0] = pluginInformation
	config.Properties = p
	plugin := &Plugin{}

	mockCancelFlag := new(task.MockCancelFlag)
	mockContext := context.NewMockDefault()

	// TO DO: How to mock reboot?
	// reboot = func() { return }

	// Create stub
	configureComponent = func(
		p *Plugin,
		log log.T,
		manager pluginHelper,
		configureUtil Util,
		updateUtil updateutil.T,
		rawPluginInput interface{}) (out ConfigureComponentPluginOutput) {
		out = ConfigureComponentPluginOutput{}
		out.ExitCode = 1
		out.Stderr = "error"

		return out
	}

	// Setup mocks
	mockCancelFlag.On("Canceled").Return(false)
	mockCancelFlag.On("ShutDown").Return(false)
	mockCancelFlag.On("Wait").Return(false).After(100 * time.Millisecond)

	result := plugin.Execute(mockContext, config, mockCancelFlag)

	assert.Equal(t, result.Code, 1)
	assert.Contains(t, result.Output, "error")
}

func TestInstallComponent(t *testing.T) {
	plugin := &Plugin{}
	pluginInformation := createStubPluginInput()
	context := createStubInstanceContext()

	output := &ConfigureComponentPluginOutput{}
	manifest, _ := parseComponentManifest(logger, "testdata/sampleManifest.json")
	manager := &mockConfigureManager{
		downloadManifestResult: manifest,
		downloadManifestError:  nil,
		downloadPackageResult:  "testdata/sampleManifest.json",
		downloadPackageError:   nil,
	}
	configureUtil := &mockConfigureUtility{}
	updateUtil := &mockUpdateUtility{}

	fileUncompress = func(src, dest string) error {
		return nil
	}

	installCommand := "AWSPVDriverSetup.msi /quiet /install"

	err := runInstallComponent(plugin,
		pluginInformation,
		output,
		manager,
		logger,
		installCommand,
		configureUtil,
		updateUtil,
		context)

	assert.NoError(t, err)
}

func TestInstallComponent_DownloadFailed(t *testing.T) {
	plugin := &Plugin{}
	pluginInformation := createStubPluginInput()
	context := createStubInstanceContext()

	output := &ConfigureComponentPluginOutput{}
	manifest, _ := parseComponentManifest(logger, "testdata/sampleManifest.json")
	manager := &mockConfigureManager{
		downloadManifestResult: manifest,
		downloadManifestError:  nil,
		downloadPackageResult:  "",
		downloadPackageError:   fmt.Errorf("Cannot download package"),
	}
	configureUtil := &mockConfigureUtility{}
	updateUtil := &mockUpdateUtility{}

	installCommand := "AWSPVDriverSetup.msi /quiet /install"

	err := runInstallComponent(plugin,
		pluginInformation,
		output,
		manager,
		logger,
		installCommand,
		configureUtil,
		updateUtil,
		context)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "Cannot download package")
}

func TestInstallComponent_ExtractFailed(t *testing.T) {
	plugin := &Plugin{}
	pluginInformation := createStubPluginInput()
	context := createStubInstanceContext()

	output := &ConfigureComponentPluginOutput{}
	manifest, _ := parseComponentManifest(logger, "testdata/sampleManifest.json")
	manager := &mockConfigureManager{
		downloadManifestResult: manifest,
		downloadManifestError:  nil,
		downloadPackageResult:  "testdata/sampleManifest.json",
		downloadPackageError:   nil,
	}
	configureUtil := &mockConfigureUtility{}
	updateUtil := &mockUpdateUtility{}

	fileUncompress = func(src, dest string) error {
		return fmt.Errorf("Cannot extract package")
	}

	installCommand := "AWSPVDriverSetup.msi /quiet /install"

	err := runInstallComponent(plugin,
		pluginInformation,
		output,
		manager,
		logger,
		installCommand,
		configureUtil,
		updateUtil,
		context)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "Cannot extract package")
}

func TestInstallComponent_DeleteFailed(t *testing.T) {
	plugin := &Plugin{}
	pluginInformation := createStubPluginInput()
	context := createStubInstanceContext()

	output := &ConfigureComponentPluginOutput{}
	manifest, _ := parseComponentManifest(logger, "testdata/sampleManifest.json")
	manager := &mockConfigureManager{
		downloadManifestResult: manifest,
		downloadManifestError:  nil,
		downloadPackageResult:  "testdata/sampleManifest.json",
		downloadPackageError:   nil,
	}
	configureUtil := &mockConfigureUtility{}
	updateUtil := &mockUpdateUtility{}

	fileUncompress = func(src, dest string) error {
		return nil
	}

	fileRemove = func(path string) error {
		return fmt.Errorf("failed to delete compressed package")
	}

	installCommand := "AWSPVDriverSetup.msi /quiet /install"

	err := runInstallComponent(plugin,
		pluginInformation,
		output,
		manager,
		logger,
		installCommand,
		configureUtil,
		updateUtil,
		context)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to delete compressed package")
}

// TO DO: Install test for exe command

func TestUninstallComponent(t *testing.T) {
	plugin := &Plugin{}
	pluginInformation := createStubPluginInput()
	context := createStubInstanceContext()

	output := &ConfigureComponentPluginOutput{}
	util := &mockUpdateUtility{}

	uninstallCommand := "AWSPVDriverSetup.msi /quiet /uninstall"

	fileRemove = func(path string) error {
		return nil
	}

	err := runUninstallComponent(plugin,
		pluginInformation,
		output,
		logger,
		uninstallCommand,
		util,
		context)

	assert.NoError(t, err)
}

func TestUninstallComponent_RemovalFailed(t *testing.T) {
	plugin := &Plugin{}
	pluginInformation := createStubPluginInput()
	context := createStubInstanceContext()

	output := &ConfigureComponentPluginOutput{}
	util := &mockUpdateUtility{}

	uninstallCommand := "AWSPVDriverSetup.msi /quiet /uninstall"

	fileRemove = func(path string) error {
		return fmt.Errorf("404")
	}

	err := runUninstallComponent(plugin,
		pluginInformation,
		output,
		logger,
		uninstallCommand,
		util,
		context)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to delete directory")
	assert.Contains(t, err.Error(), "404")
}

// TO DO: Uninstall test for exe command

func TestDownloadPackage(t *testing.T) {
	pluginInformation := createStubPluginInput()
	context := createStubInstanceContext()

	output := ConfigureComponentPluginOutput{}
	manager := configureManager{}
	util := mockConfigureUtility{}

	fileDownload = func(log log.T, input artifact.DownloadInput) (output artifact.DownloadOutput, err error) {
		result := artifact.DownloadOutput{}
		result.LocalFilePath = "components/PVDriver/9000.0.0.0/PVDriver-amd64.zip"
		return result, nil
	}

	fileName, err := manager.downloadPackage(logger, &util, pluginInformation, &output, context)

	assert.Equal(t, "components/PVDriver/9000.0.0.0/PVDriver-amd64.zip", fileName)
	assert.NoError(t, err)
}

func TestDownloadPackage_Failed(t *testing.T) {
	pluginInformation := createStubPluginInput()
	context := createStubInstanceContext()

	output := ConfigureComponentPluginOutput{}
	manager := configureManager{}
	util := mockConfigureUtility{}

	// file download failed
	fileDownload = func(log log.T, input artifact.DownloadInput) (output artifact.DownloadOutput, err error) {
		result := artifact.DownloadOutput{}
		result.LocalFilePath = ""
		return result, fmt.Errorf("404")
	}

	fileName, err := manager.downloadPackage(logger, &util, pluginInformation, &output, context)

	assert.Empty(t, fileName)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to download component installation package reliably")
	assert.Contains(t, err.Error(), "404")
}

type mockConfigureManager struct {
	downloadManifestResult *ComponentManifest
	downloadManifestError  error
	downloadPackageResult  string
	downloadPackageError   error
}

func (m *mockConfigureManager) downloadManifest(log log.T,
	util Util,
	input *ConfigureComponentPluginInput,
	output *ConfigureComponentPluginOutput,
	context *updateutil.InstanceContext) (manifest *ComponentManifest, err error) {

	return m.downloadManifestResult, m.downloadManifestError
}

func (m *mockConfigureManager) downloadPackage(log log.T,
	util Util,
	input *ConfigureComponentPluginInput,
	output *ConfigureComponentPluginOutput,
	context *updateutil.InstanceContext) (fileName string, err error) {

	return "", m.downloadPackageError
}

type mockUpdateUtility struct {
	updateutil.Utility
}

func (u *mockUpdateUtility) CreateInstanceContext(log log.T) (context *updateutil.InstanceContext, err error) {
	return createStubInstanceContext(), nil
}

func (u *mockUpdateUtility) IsServiceRunning(log log.T, i *updateutil.InstanceContext) (result bool, err error) {
	return true, nil
}

func (u *mockUpdateUtility) CreateUpdateDownloadFolder() (folder string, err error) {
	return "", nil
}

func (u *mockUpdateUtility) ExeCommand(
	log log.T,
	cmd string,
	updateRoot string,
	workingDir string,
	stdOut string,
	stdErr string,
	isAsync bool) (err error) {
	return nil
}

func (u *mockUpdateUtility) SaveUpdatePluginResult(
	log log.T,
	updateRoot string,
	updateResult *updateutil.UpdatePluginResult) (err error) {
	return nil
}

func (u *mockUpdateUtility) IsDiskSpaceSufficientForUpdate(log log.T) (bool, error) {
	return true, nil
}
