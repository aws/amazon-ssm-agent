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

// TODO:MF: flag these as integration tests

// Package configurepackage implements the ConfigurePackage plugin.
package configurepackage

import (
	"errors"
	"io/ioutil"
	"testing"

	"github.com/aws/amazon-ssm-agent/agent/contracts"
	"github.com/aws/amazon-ssm-agent/agent/fileutil/artifact"
	"github.com/aws/amazon-ssm-agent/agent/statemanager/model"
	"github.com/stretchr/testify/assert"
)

func fileSysStubSuccess() fileSysDep {
	result, _ := ioutil.ReadFile("testdata/sampleManifest.json")
	return &FileSysDepStub{readResult: result, existsResultDefault: true}
}

func networkStubSuccess() networkDep {
	return &NetworkDepStub{downloadResultDefault: artifact.DownloadOutput{LocalFilePath: "Stub"}}
}

func execStubSuccess() execDep {
	return &ExecDepStub{pluginInput: &model.PluginState{}, pluginOutput: &contracts.PluginResult{}}
}

func setSuccessStubs() *ConfigurePackageStubs {
	stubs := &ConfigurePackageStubs{fileSysDepStub: fileSysStubSuccess(), networkDepStub: networkStubSuccess(), execDepStub: execStubSuccess()}
	stubs.Set()
	return stubs
}

func TestConfigurePackage(t *testing.T) {
	stubs := setSuccessStubs()
	defer stubs.Clear()

	plugin := &Plugin{}
	pluginInformation := createStubPluginInputInstall()

	manager := &configureManager{}
	configureUtil := &Utility{}
	instanceContext := createStubInstanceContext()

	output := runConfigurePackage(
		plugin,
		logger,
		manager,
		configureUtil,
		instanceContext,
		pluginInformation)

	assert.Empty(t, output.Stderr)
	assert.Empty(t, output.Errors)
}

func TestConfigurePackage_InvalidRawInput(t *testing.T) {
	stubs := setSuccessStubs()
	defer stubs.Clear()

	plugin := &Plugin{}
	// string value will fail the Remarshal as it's not ConfigurePackagePluginInput
	pluginInformation := "invalid value"

	manager := &configureManager{}
	configureUtil := &Utility{}
	instanceContext := createStubInstanceContext()

	result := runConfigurePackage(
		plugin,
		logger,
		manager,
		configureUtil,
		instanceContext,
		pluginInformation)

	assert.Contains(t, result.Stderr, "invalid format in plugin properties")
}

func TestConfigurePackage_InvalidInput(t *testing.T) {
	stubs := setSuccessStubs()
	defer stubs.Clear()

	plugin := &Plugin{}
	pluginInformation := createStubInvalidPluginInput()

	manager := &configureManager{}
	configureUtil := &Utility{}
	instanceContext := createStubInstanceContext()

	result := runConfigurePackage(
		plugin,
		logger,
		manager,
		configureUtil,
		instanceContext,
		pluginInformation)

	assert.Contains(t, result.Stderr, "invalid input")
}

func TestConfigurePackage_DownloadFailed(t *testing.T) {
	stubs := &ConfigurePackageStubs{
		fileSysDepStub: &FileSysDepStub{existsResultDefault: false},
		networkDepStub: &NetworkDepStub{downloadErrorDefault: errors.New("Cannot download package")},
		execDepStub:    execStubSuccess(),
	}
	stubs.Set()
	defer stubs.Clear()

	plugin := &Plugin{}
	pluginInformation := createStubPluginInputInstall()

	manager := &configureManager{}
	configureUtil := &Utility{}
	instanceContext := createStubInstanceContext()

	output := runConfigurePackage(
		plugin,
		logger,
		manager,
		configureUtil,
		instanceContext,
		pluginInformation)

	assert.NotEmpty(t, output.Stderr, output.Stdout)
	assert.NotEmpty(t, output.Errors)
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

	manager := &configureManager{}
	configureUtil := &Utility{}
	instanceContext := createStubInstanceContext()

	output := runConfigurePackage(
		plugin,
		logger,
		manager,
		configureUtil,
		instanceContext,
		pluginInformation)

	assert.NotEmpty(t, output.Stderr)
	assert.NotEmpty(t, output.Errors)
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

	manager := &configureManager{}
	configureUtil := &Utility{}
	instanceContext := createStubInstanceContext()

	output := runConfigurePackage(
		plugin,
		logger,
		manager,
		configureUtil,
		instanceContext,
		pluginInformation)

	assert.NotEmpty(t, output.Stderr)
	assert.NotEmpty(t, output.Errors)
	assert.Contains(t, output.Stderr, "failed to delete compressed package")
}

func TestUninstallPackage_DoesNotExist(t *testing.T) {
	stubs := &ConfigurePackageStubs{fileSysDepStub: &FileSysDepStub{existsResultDefault: false}, networkDepStub: networkStubSuccess(), execDepStub: execStubSuccess()}
	stubs.Set()
	defer stubs.Clear()

	plugin := &Plugin{}
	pluginInformation := createStubPluginInputUninstallLatest()

	manager := &configureManager{}
	configureUtil := &Utility{}
	instanceContext := createStubInstanceContext()

	output := runConfigurePackage(
		plugin,
		logger,
		manager,
		configureUtil,
		instanceContext,
		pluginInformation)

	assert.NotEmpty(t, output.Stderr)
	assert.NotEmpty(t, output.Errors)
	assert.Contains(t, output.Stderr, "unable to determine version")
}

func TestUninstallPackage_RemovalFailed(t *testing.T) {
	result, _ := ioutil.ReadFile("testdata/sampleManifest.json")
	stubs := &ConfigurePackageStubs{
		fileSysDepStub: &FileSysDepStub{readResult: result, existsResultDefault: true, removeError: errors.New("404")},
		networkDepStub: networkStubSuccess(),
		execDepStub:    execStubSuccess(),
	}
	stubs.Set()
	defer stubs.Clear()

	plugin := &Plugin{}
	pluginInformation := createStubPluginInputUninstall()

	manager := &configureManager{}
	configureUtil := &Utility{}
	instanceContext := createStubInstanceContext()

	output := runConfigurePackage(
		plugin,
		logger,
		manager,
		configureUtil,
		instanceContext,
		pluginInformation)

	assert.NotEmpty(t, output.Stderr)
	assert.NotEmpty(t, output.Errors)
	assert.Contains(t, output.Stderr, "failed to delete directory")
	assert.Contains(t, output.Stderr, "404")
}
