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
	"errors"
	"fmt"
	"io/ioutil"
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

	output.MarkAsSucceeded(false)

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

//TODO:MF: Needs a second download result... or mock the configuration manager and make this a unit test
func TestInstallComponent_DownloadFailed(t *testing.T) {
	plugin := &Plugin{}
	pluginInformation := createStubPluginInputInstall()

	manifest, _ := parseComponentManifest(logger, "testdata/sampleManifest.json")
	manager := &mockConfigureManager{
		downloadManifestResult: manifest,
		downloadManifestError:  nil,
		downloadPackageResult:  "",
		downloadPackageError:   fmt.Errorf("Cannot download package"),
		validateInputResult:    true,
		validateInputError:     nil,
	}
	configureUtil := &mockConfigureUtility{}
	instanceContext := createStubInstanceContext()

	output := runConfigureComponent(
		plugin,
		logger,
		manager,
		configureUtil,
		instanceContext,
		pluginInformation)

	assert.NotEmpty(t, output.Stderr)
	assert.NotEmpty(t, output.Errors)
}

func TestExecute(t *testing.T) {
	pluginInformation := createStubPluginInputInstall()
	config := contracts.Configuration{}
	p := make([]interface{}, 1)
	p[0] = pluginInformation
	config.Properties = p
	plugin := &Plugin{}

	mockCancelFlag := new(task.MockCancelFlag)
	mockContext := context.NewMockDefault()

	getContextOrig := getContext
	runConfigOrig := runConfig
	getContext = func(log log.T) (context *updateutil.InstanceContext, err error) {
		return createStubInstanceContext(), nil
	}
	runConfig = func(
		p *Plugin,
		log log.T,
		manager pluginHelper,
		configureUtil Util,
		context *updateutil.InstanceContext,
		rawPluginInput interface{}) (out ConfigureComponentPluginOutput) {
		out = ConfigureComponentPluginOutput{}
		out.ExitCode = 1
		out.Stderr = "error"

		return out
	}
	defer func() {
		runConfig = runConfigOrig
		getContext = getContextOrig
	}()

	// TODO:MF Test result code for reboot in cases where that is expected?

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
	pluginInformation := createStubPluginInputInstall()
	context := createStubInstanceContext()

	output := &ConfigureComponentPluginOutput{}
	manifest, _ := parseComponentManifest(logger, "testdata/sampleManifest.json")
	manager := &mockConfigureManager{
		downloadManifestResult: manifest,
		downloadManifestError:  nil,
		downloadPackageResult:  "testdata/sampleManifest.json",
		downloadPackageError:   nil,
		validateInputResult:    true,
		validateInputError:     nil,
	}

	result, _ := ioutil.ReadFile("testdata/sampleManifest.json")
	stubs := &ConfigureComponentStubs{fileSysDepStub: &FileSysDepStub{readResult: result}, networkDepStub: &NetworkDepStub{}, execDepStub: &ExecDepStub{}}
	stubs.Set()
	defer stubs.Clear()

	installCommand := "AWSPVDriverSetup.msi /quiet /install"

	err := runInstallComponent(plugin,
		pluginInformation.Name,
		pluginInformation.Version,
		pluginInformation.Source,
		output,
		manager,
		logger,
		installCommand,
		context)

	assert.NoError(t, err)
}

func TestUninstallComponent(t *testing.T) {
	plugin := &Plugin{}
	pluginInformation := createStubPluginInputUninstall()
	instanceContext := createStubInstanceContext()

	output := &ConfigureComponentPluginOutput{}

	stubs := &ConfigureComponentStubs{fileSysDepStub: &FileSysDepStub{existsResultDefault: true}, networkDepStub: &NetworkDepStub{}, execDepStub: &ExecDepStub{}}
	stubs.Set()
	defer stubs.Clear()

	uninstallCommand := "AWSPVDriverSetup.msi /quiet /uninstall"

	err := runUninstallComponent(plugin,
		pluginInformation.Name,
		pluginInformation.Version,
		pluginInformation.Source,
		output,
		logger,
		uninstallCommand,
		instanceContext)

	assert.NoError(t, err)
}

// TO DO: Uninstall test for exe command

func TestValidateInput(t *testing.T) {
	//pluginInformation := createStubPluginInput()

	input := ConfigureComponentPluginInput{}

	input.Version = "1.0.0"
	input.Name = "PVDriver"
	input.Action = "InvalidAction"
	input.Source = "https://amazon-ssm-us-west-2.s3.amazonaws.com/Components/PVDriver/windows/amd64/9000.0.0/PVDriver-amd64.zip"

	manager := configureManager{}

	result, err := manager.validateInput(&input)

	assert.True(t, result)
	assert.NoError(t, err)
}

func TestValidateInput_Name(t *testing.T) {
	input := ConfigureComponentPluginInput{}

	// Set version to a large number to avoid conflict of the actual component release version
	input.Version = "9000.0.0.0"
	input.Name = ""
	input.Action = "InvalidAction"
	input.Source = "https://amazon-ssm-us-west-2.s3.amazonaws.com/Components/PVDriver/windows/amd64/9000.0.0/PVDriver-amd64.zip"

	manager := configureManager{}
	result, err := manager.validateInput(&input)

	assert.False(t, result)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "empty name field")
}

func TestValidateInput_EmptyVersionWithInstall(t *testing.T) {
	input := ConfigureComponentPluginInput{}

	// Set version to a large number to avoid conflict of the actual component release version
	input.Version = ""
	input.Name = "PVDriver"
	input.Action = "Install"
	input.Source = "https://amazon-ssm-us-west-2.s3.amazonaws.com/Components/PVDriver/windows/amd64/9000.0.0/PVDriver-amd64.zip"

	manager := configureManager{}
	result, err := manager.validateInput(&input)

	assert.True(t, result)
	assert.NoError(t, err)
}

func TestValidateInput_EmptyVersionWithUninstall(t *testing.T) {
	input := ConfigureComponentPluginInput{}

	// Set version to a large number to avoid conflict of the actual component release version
	input.Version = ""
	input.Name = "PVDriver"
	input.Action = "Uninstall"
	input.Source = "https://amazon-ssm-us-west-2.s3.amazonaws.com/Components/PVDriver/windows/amd64/9000.0.0/PVDriver-amd64.zip"

	manager := configureManager{}
	result, err := manager.validateInput(&input)

	assert.True(t, result)
	assert.NoError(t, err)
}

func TestDownloadPackage(t *testing.T) {
	pluginInformation := createStubPluginInputInstall()
	context := createStubInstanceContext()

	output := ConfigureComponentPluginOutput{}
	manager := configureManager{}
	util := mockConfigureUtility{}

	result := artifact.DownloadOutput{}
	result.LocalFilePath = "components/PVDriver/9000.0.0.0/PVDriver.zip"

	stubs := &ConfigureComponentStubs{fileSysDepStub: &FileSysDepStub{}, networkDepStub: &NetworkDepStub{downloadResult: result}}
	stubs.Set()
	defer stubs.Clear()

	fileName, err := manager.downloadPackage(logger, &util, pluginInformation.Name, pluginInformation.Version, pluginInformation.Source, &output, context)

	assert.Equal(t, "components/PVDriver/9000.0.0.0/PVDriver.zip", fileName)
	assert.NoError(t, err)
}

func TestDownloadPackage_Failed(t *testing.T) {
	pluginInformation := createStubPluginInputInstall()
	context := createStubInstanceContext()

	output := ConfigureComponentPluginOutput{}
	manager := configureManager{}
	util := mockConfigureUtility{}

	// file download failed
	result := artifact.DownloadOutput{}
	result.LocalFilePath = ""

	stubs := &ConfigureComponentStubs{fileSysDepStub: &FileSysDepStub{}, networkDepStub: &NetworkDepStub{downloadResult: result, downloadError: errors.New("404")}}
	stubs.Set()
	defer stubs.Clear()

	fileName, err := manager.downloadPackage(logger, &util, pluginInformation.Name, pluginInformation.Version, pluginInformation.Source, &output, context)

	assert.Empty(t, fileName)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to download component installation package reliably")
	assert.Contains(t, err.Error(), "404")
}

func TestComponentLock(t *testing.T) {
	// lock Foo for Install
	err := lockComponent("Foo", "Install")
	assert.Nil(t, err)
	defer unlockComponent("Foo")

	// shouldn't be able to lock Foo, even for a different action
	err = lockComponent("Foo", "Uninstall")
	assert.NotNil(t, err)

	// lock and unlock Bar (with defer)
	err = lockAndUnlock("Bar")
	assert.Nil(t, err)

	// should be able to lock and then unlock Bar
	err = lockComponent("Bar", "Uninstall")
	assert.Nil(t, err)
	unlockComponent("Bar")

	// should be able to lock Bar
	err = lockComponent("Bar", "Uninstall")
	assert.Nil(t, err)
	defer unlockComponent("Bar")

	// lock in a goroutine with a 10ms sleep
	errorChan := make(chan error)
	go lockAndUnlockGo("Foobar", errorChan)
	err = <-errorChan // wait until the goroutine has acquired the lock
	assert.Nil(t, err)
	err = lockComponent("Foobar", "Install")
	errorChan <- err // signal the goroutine to exit
	assert.NotNil(t, err)
}

func lockAndUnlockGo(component string, channel chan error) {
	err := lockComponent(component, "Install")
	channel <- err
	_ = <-channel
	if err == nil {
		defer unlockComponent(component)
	}
	return
}

func lockAndUnlock(component string) (err error) {
	if err = lockComponent(component, "Install"); err != nil {
		return
	}
	defer unlockComponent(component)
	return
}

type mockConfigureManager struct {
	downloadManifestResult *ComponentManifest
	downloadManifestError  error
	downloadPackageResult  string
	downloadPackageError   error
	validateInputResult    bool
	validateInputError     error
	installedVersion       string
}

func (m *mockConfigureManager) downloadManifest(log log.T,
	util Util,
	componentName string,
	version string,
	source string,
	output *ConfigureComponentPluginOutput,
	context *updateutil.InstanceContext) (manifest *ComponentManifest, err error) {

	return m.downloadManifestResult, m.downloadManifestError
}

func (m *mockConfigureManager) downloadPackage(log log.T,
	util Util,
	componentName string,
	version string,
	source string,
	output *ConfigureComponentPluginOutput,
	context *updateutil.InstanceContext) (filePath string, err error) {

	return "", m.downloadPackageError
}

func (m *mockConfigureManager) validateInput(input *ConfigureComponentPluginInput) (valid bool, err error) {
	return m.validateInputResult, m.validateInputError
}

// TODO:MF: mock the dependencies this method has instead, maybe pull this out to a different "class"
func (m *mockConfigureManager) getVersionToInstall(log log.T,
	input *ConfigureComponentPluginInput,
	util Util,
	context *updateutil.InstanceContext) (version string, installedVersion string, err error) {

	if m.downloadManifestResult != nil {
		version = m.downloadManifestResult.Version
	} else {
		version = ""
	}
	return version, m.installedVersion, m.downloadManifestError
}

func (m *mockConfigureManager) getVersionToUninstall(log log.T,
	input *ConfigureComponentPluginInput,
	util Util,
	context *updateutil.InstanceContext) (version string, err error) {

	if m.downloadManifestResult != nil {
		version = m.downloadManifestResult.Version
	} else {
		version = ""
	}
	return version, m.downloadManifestError
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
