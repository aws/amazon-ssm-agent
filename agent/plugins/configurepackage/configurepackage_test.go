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

// Package configurepackages implements the ConfigurePackage plugin.
package configurepackage

import (
	"errors"
	"fmt"
	"io/ioutil"
	"testing"
	"time"

	"github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/aws/amazon-ssm-agent/agent/contracts"
	"github.com/aws/amazon-ssm-agent/agent/fileutil/artifact"
	"github.com/aws/amazon-ssm-agent/agent/framework/runpluginutil"
	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/amazon-ssm-agent/agent/statemanager/model"
	"github.com/aws/amazon-ssm-agent/agent/task"
	"github.com/aws/amazon-ssm-agent/agent/updateutil"
	"github.com/stretchr/testify/assert"
)

var logger = log.NewMockLog()

func fileSysStubSuccess() fileSysDep {
	result, _ := ioutil.ReadFile("testdata/sampleManifest.json")
	return &FileSysDepStub{readResult: result, existsResultDefault: true}
}

func networkStubSuccess() networkDep {
	return &NetworkDepStub{downloadResultDefault: artifact.DownloadOutput{LocalFilePath: "Stub"}}
}

func execStubSuccess() execDep {
	return &ExecDepStub{pluginInput: &model.PluginState{}, pluginOutput: &contracts.PluginResult{Status: contracts.ResultStatusSuccess}}
}

func setSuccessStubs() *ConfigurePackageStubs {
	stubs := &ConfigurePackageStubs{fileSysDepStub: fileSysStubSuccess(), networkDepStub: networkStubSuccess(), execDepStub: execStubSuccess()}
	stubs.Set()
	return stubs
}

func TestMarkAsSucceeded(t *testing.T) {
	output := ConfigurePackagePluginOutput{}

	output.MarkAsSucceeded(false)

	assert.Equal(t, output.ExitCode, 0)
	assert.Equal(t, output.Status, contracts.ResultStatusSuccess)
}

func TestMarkAsFailed(t *testing.T) {
	output := ConfigurePackagePluginOutput{}

	output.MarkAsFailed(logger, fmt.Errorf("Error message"))

	assert.Equal(t, output.ExitCode, 1)
	assert.Equal(t, output.Status, contracts.ResultStatusFailed)
	assert.Contains(t, output.Stderr, "Error message")
}

func TestAppendInfo(t *testing.T) {
	output := ConfigurePackagePluginOutput{}

	output.AppendInfo(logger, "Info message")

	assert.Contains(t, output.Stdout, "Info message")
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
		util configureUtil,
		context *updateutil.InstanceContext,
		rawPluginInput interface{}) (out ConfigurePackagePluginOutput) {
		out = ConfigurePackagePluginOutput{}
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

	result := plugin.Execute(mockContext, config, mockCancelFlag, runpluginutil.PluginRunner{})

	assert.Equal(t, result.Code, 1)
	assert.Contains(t, result.Output, "error")
}

func TestInstallPackage(t *testing.T) {
	plugin := &Plugin{}
	pluginInformation := createStubPluginInputInstall()
	context := createStubInstanceContext()

	output := &ConfigurePackagePluginOutput{}
	manifest, _ := parsePackageManifest(logger, "testdata/sampleManifest.json")
	manager := &mockConfigureManager{
		downloadManifestResult: manifest,
		downloadManifestError:  nil,
		downloadPackageResult:  "testdata/sampleManifest.json",
		downloadPackageError:   nil,
		validateInputResult:    true,
		validateInputError:     nil,
	}

	result, _ := ioutil.ReadFile("testdata/sampleManifest.json")
	stubs := &ConfigurePackageStubs{fileSysDepStub: &FileSysDepStub{readResult: result}, networkDepStub: &NetworkDepStub{}, execDepStub: execStubSuccess()}
	stubs.Set()
	defer stubs.Clear()

	_, err := runInstallPackage(plugin,
		pluginInformation.Name,
		pluginInformation.Version,
		output,
		manager,
		logger,
		context)

	assert.NoError(t, err)
}

func TestUninstallPackage(t *testing.T) {
	plugin := &Plugin{}
	pluginInformation := createStubPluginInputUninstall()
	instanceContext := createStubInstanceContext()

	output := &ConfigurePackagePluginOutput{}

	stubs := &ConfigurePackageStubs{fileSysDepStub: &FileSysDepStub{existsResultDefault: true}, networkDepStub: &NetworkDepStub{}, execDepStub: execStubSuccess()}
	stubs.Set()
	defer stubs.Clear()

	_, err := runUninstallPackage(plugin,
		pluginInformation.Name,
		pluginInformation.Version,
		output,
		logger,
		instanceContext)

	assert.NoError(t, err)
}

// TO DO: Uninstall test for exe command

func TestValidateInput(t *testing.T) {
	//pluginInformation := createStubPluginInput()

	input := ConfigurePackagePluginInput{}

	input.Version = "1.0.0"
	input.Name = "PVDriver"
	input.Action = "InvalidAction"

	manager := configureManager{}

	result, err := manager.validateInput(&input)

	assert.True(t, result)
	assert.NoError(t, err)
}

func TestValidateInput_Source(t *testing.T) {
	//pluginInformation := createStubPluginInput()

	input := ConfigurePackagePluginInput{}

	input.Version = "1.0.0"
	input.Name = "PVDriver"
	input.Action = "Install"
	input.Source = "http://amazon.com"

	manager := configureManager{}

	result, err := manager.validateInput(&input)

	assert.False(t, result)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "source parameter is not supported")
}

func TestValidateInput_Name(t *testing.T) {
	input := ConfigurePackagePluginInput{}

	// Set version to a large number to avoid conflict of the actual package release version
	input.Version = "9000.0.0.0"
	input.Name = ""
	input.Action = "InvalidAction"

	manager := configureManager{}
	result, err := manager.validateInput(&input)

	assert.False(t, result)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "empty name field")
}

func TestValidateInput_EmptyVersionWithInstall(t *testing.T) {
	input := ConfigurePackagePluginInput{}

	// Set version to a large number to avoid conflict of the actual package release version
	input.Version = ""
	input.Name = "PVDriver"
	input.Action = "Install"

	manager := configureManager{}
	result, err := manager.validateInput(&input)

	assert.True(t, result)
	assert.NoError(t, err)
}

func TestValidateInput_EmptyVersionWithUninstall(t *testing.T) {
	input := ConfigurePackagePluginInput{}

	// Set version to a large number to avoid conflict of the actual package release version
	input.Version = ""
	input.Name = "PVDriver"
	input.Action = "Uninstall"

	manager := configureManager{}
	result, err := manager.validateInput(&input)

	assert.True(t, result)
	assert.NoError(t, err)
}

func TestDownloadPackage(t *testing.T) {
	pluginInformation := createStubPluginInputInstall()
	context := createStubInstanceContext()

	output := ConfigurePackagePluginOutput{}
	manager := configureManager{}
	util := mockConfigureUtility{}

	result := artifact.DownloadOutput{}
	result.LocalFilePath = "packages/PVDriver/9000.0.0.0/PVDriver.zip"

	stubs := &ConfigurePackageStubs{fileSysDepStub: &FileSysDepStub{}, networkDepStub: &NetworkDepStub{downloadResultDefault: result}}
	stubs.Set()
	defer stubs.Clear()

	fileName, err := manager.downloadPackage(logger, &util, pluginInformation.Name, pluginInformation.Version, &output, context)

	assert.Equal(t, "packages/PVDriver/9000.0.0.0/PVDriver.zip", fileName)
	assert.NoError(t, err)
}

func TestDownloadPackage_Failed(t *testing.T) {
	pluginInformation := createStubPluginInputInstall()
	context := createStubInstanceContext()

	output := ConfigurePackagePluginOutput{}
	manager := configureManager{}
	util := mockConfigureUtility{}

	// file download failed
	result := artifact.DownloadOutput{}
	result.LocalFilePath = ""

	stubs := &ConfigurePackageStubs{fileSysDepStub: &FileSysDepStub{}, networkDepStub: &NetworkDepStub{downloadResultDefault: result, downloadErrorDefault: errors.New("404")}}
	stubs.Set()
	defer stubs.Clear()

	fileName, err := manager.downloadPackage(logger, &util, pluginInformation.Name, pluginInformation.Version, &output, context)

	assert.Empty(t, fileName)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to download installation package reliably")
	assert.Contains(t, err.Error(), "404")
}

func TestPackageLock(t *testing.T) {
	// lock Foo for Install
	err := lockPackage("Foo", "Install")
	assert.Nil(t, err)
	defer unlockPackage("Foo")

	// shouldn't be able to lock Foo, even for a different action
	err = lockPackage("Foo", "Uninstall")
	assert.NotNil(t, err)

	// lock and unlock Bar (with defer)
	err = lockAndUnlock("Bar")
	assert.Nil(t, err)

	// should be able to lock and then unlock Bar
	err = lockPackage("Bar", "Uninstall")
	assert.Nil(t, err)
	unlockPackage("Bar")

	// should be able to lock Bar
	err = lockPackage("Bar", "Uninstall")
	assert.Nil(t, err)
	defer unlockPackage("Bar")

	// lock in a goroutine with a 10ms sleep
	errorChan := make(chan error)
	go lockAndUnlockGo("Foobar", errorChan)
	err = <-errorChan // wait until the goroutine has acquired the lock
	assert.Nil(t, err)
	err = lockPackage("Foobar", "Install")
	errorChan <- err // signal the goroutine to exit
	assert.NotNil(t, err)
}

func TestPackageMark(t *testing.T) {
	stubs := &ConfigurePackageStubs{fileSysDepStub: &FileSysDepStub{existsResultDefault: false}}
	stubs.Set()
	defer stubs.Clear()

	err := markInstallingPackage("Foo", "999.999.999")
	assert.Nil(t, err)
}

func TestPackageInstalling(t *testing.T) {
	stubs := &ConfigurePackageStubs{fileSysDepStub: &FileSysDepStub{existsResultDefault: true, readResult: []byte("999.999.999")}}
	stubs.Set()
	defer stubs.Clear()

	assert.Equal(t, "999.999.999", getInstallingPackageVersion("Foo"))
}

func TestPackageNotInstalling(t *testing.T) {
	stubs := &ConfigurePackageStubs{fileSysDepStub: &FileSysDepStub{existsResultDefault: false}}
	stubs.Set()
	defer stubs.Clear()

	assert.Equal(t, "", getInstallingPackageVersion("Foo"))
}

func TestPackageUnreadableInstalling(t *testing.T) {
	stubs := &ConfigurePackageStubs{fileSysDepStub: &FileSysDepStub{existsResultDefault: false, readResult: []byte(""), readError: errors.New("Foo")}}
	stubs.Set()
	defer stubs.Clear()

	assert.Equal(t, "", getInstallingPackageVersion("Foo"))
}

func TestUnmarkPackage(t *testing.T) {
	stubs := &ConfigurePackageStubs{fileSysDepStub: &FileSysDepStub{existsResultDefault: true}}
	stubs.Set()
	defer stubs.Clear()

	assert.Nil(t, unmarkInstallingPackage("Foo"))
}

func lockAndUnlockGo(packageName string, channel chan error) {
	err := lockPackage(packageName, "Install")
	channel <- err
	_ = <-channel
	if err == nil {
		defer unlockPackage(packageName)
	}
	return
}

func lockAndUnlock(packageName string) (err error) {
	if err = lockPackage(packageName, "Install"); err != nil {
		return
	}
	defer unlockPackage(packageName)
	return
}

type mockConfigureManager struct {
	downloadManifestResult *PackageManifest
	downloadManifestError  error
	downloadPackageResult  string
	downloadPackageError   error
	validateInputResult    bool
	validateInputError     error
	installedVersion       string
}

func (m *mockConfigureManager) downloadPackage(log log.T,
	util configureUtil,
	packageName string,
	version string,
	output *ConfigurePackagePluginOutput,
	context *updateutil.InstanceContext) (filePath string, err error) {

	return "", m.downloadPackageError
}

func (m *mockConfigureManager) validateInput(input *ConfigurePackagePluginInput) (valid bool, err error) {
	return m.validateInputResult, m.validateInputError
}

func (m *mockConfigureManager) getVersionToInstall(log log.T,
	input *ConfigurePackagePluginInput,
	util configureUtil,
	context *updateutil.InstanceContext) (version string, installedVersion string, err error) {

	if m.downloadManifestResult != nil {
		version = m.downloadManifestResult.Version
	} else {
		version = ""
	}
	return version, m.installedVersion, m.downloadManifestError
}

func (m *mockConfigureManager) getVersionToUninstall(log log.T,
	input *ConfigurePackagePluginInput,
	util configureUtil,
	context *updateutil.InstanceContext) (version string, err error) {

	if m.downloadManifestResult != nil {
		version = m.downloadManifestResult.Version
	} else {
		version = ""
	}
	return version, m.downloadManifestError
}
