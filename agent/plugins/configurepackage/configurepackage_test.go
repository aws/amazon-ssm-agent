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
package configurepackage

import (
	"testing"

	"github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/aws/amazon-ssm-agent/agent/contracts"
	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/amazon-ssm-agent/agent/plugins/configurepackage/localpackages"
	"github.com/aws/amazon-ssm-agent/agent/plugins/configurepackage/trace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

var contextMock context.T = context.NewMockDefault()

func createStubPluginInputInstall() *ConfigurePackagePluginInput {
	input := ConfigurePackagePluginInput{}

	input.Version = "0.0.1"
	input.Name = "PVDriver"
	input.Action = "Install"

	return &input
}

func createStubPluginInputInstallLatest() *ConfigurePackagePluginInput {
	input := ConfigurePackagePluginInput{}

	input.Name = "PVDriver"
	input.Action = "Install"

	return &input
}

func createStubPluginInputUninstall(version string) *ConfigurePackagePluginInput {
	input := ConfigurePackagePluginInput{}

	input.Version = version
	input.Name = "PVDriver"
	input.Action = "Uninstall"

	return &input
}

func createStubPluginInputUninstallCurrent() *ConfigurePackagePluginInput {
	input := ConfigurePackagePluginInput{}

	input.Name = "PVDriver"
	input.Action = "Uninstall"

	return &input
}

func createStubPluginInputFoo() *ConfigurePackagePluginInput {
	input := ConfigurePackagePluginInput{}

	input.Version = "0.0.1"
	input.Name = "PVDriver"
	input.Action = "Foo"

	return &input
}

func TestName(t *testing.T) {
	assert.Equal(t, "aws:configurePackage", Name())
}

func TestPrepareNewInstall(t *testing.T) {
	// file stubs are needed for ensurePackage because it handles the unzip
	stubs := setSuccessStubs()
	defer stubs.Clear()

	pluginInformation := createStubPluginInputInstall()
	installerMock := installerNotCalledMock()
	repoMock := repoInstallMock(pluginInformation, installerMock)
	serviceMock := serviceSuccessMock()
	tracer := trace.NewTracer(log.NewMockLog())
	output := &trace.PluginOutputTrace{Tracer: tracer}

	inst, uninst, installState, installedVersion := prepareConfigurePackage(
		tracer,
		buildConfigSimple(pluginInformation),
		repoMock,
		serviceMock,
		pluginInformation,
		"packageArn",
		"0.0.1",
		output)

	assert.NotNil(t, inst)
	assert.Nil(t, uninst)
	assert.Equal(t, localpackages.None, installState)
	assert.Empty(t, installedVersion)
	assert.Equal(t, 0, output.GetExitCode())
	assert.Empty(t, tracer.ToPluginOutput().GetStderr())

	installerMock.AssertExpectations(t)
}

func TestPrepareUpgrade(t *testing.T) {
	// file stubs are needed for ensurePackage because it handles the unzip
	stubs := setSuccessStubs()
	defer stubs.Clear()

	pluginInformation := createStubPluginInputInstallLatest()
	installerMock := installerNotCalledMock()
	repoMock := repoUpgradeMock(pluginInformation, installerMock)
	serviceMock := serviceUpgradeMock()
	tracer := trace.NewTracer(log.NewMockLog())
	output := &trace.PluginOutputTrace{Tracer: tracer}

	inst, uninst, installState, installedVersion := prepareConfigurePackage(
		tracer,
		buildConfigSimple(pluginInformation),
		repoMock,
		serviceMock,
		pluginInformation,
		"packageArn",
		"0.0.2",
		output)

	assert.NotNil(t, inst)
	assert.NotNil(t, uninst)
	assert.Equal(t, localpackages.Installed, installState)
	assert.NotEmpty(t, installedVersion)
	assert.Equal(t, 0, output.GetExitCode())
	assert.Empty(t, tracer.ToPluginOutput().GetStderr())

	installerMock.AssertExpectations(t)
}

func TestPrepareUninstall(t *testing.T) {
	// file stubs are needed for ensurePackage because it handles the unzip
	stubs := setSuccessStubs()
	defer stubs.Clear()

	pluginInformation := createStubPluginInputUninstall("0.0.1")
	installerMock := installerNotCalledMock()
	repoMock := repoUninstallMock(pluginInformation, installerMock)
	serviceMock := serviceSuccessMock()
	tracer := trace.NewTracer(log.NewMockLog())
	output := &trace.PluginOutputTrace{Tracer: tracer}

	inst, uninst, installState, installedVersion := prepareConfigurePackage(
		tracer,
		buildConfigSimple(pluginInformation),
		repoMock,
		serviceMock,
		pluginInformation,
		"packageArn",
		"0.0.1",
		output)

	assert.Nil(t, inst)
	assert.NotNil(t, uninst)
	assert.Equal(t, localpackages.Installed, installState)
	assert.NotEmpty(t, installedVersion)
	assert.Equal(t, 0, output.GetExitCode())
	assert.Empty(t, tracer.ToPluginOutput().GetStderr())

	installerMock.AssertExpectations(t)
}

func TestPrepareUninstallCurrent(t *testing.T) {
	// file stubs are needed for ensurePackage because it handles the unzip
	stubs := setSuccessStubs()
	defer stubs.Clear()

	pluginInformation := createStubPluginInputUninstallCurrent()
	installerMock := installerNotCalledMock()
	repoMock := repoUninstallMock(pluginInformation, installerMock)
	serviceMock := serviceSuccessMock()
	tracer := trace.NewTracer(log.NewMockLog())
	output := &trace.PluginOutputTrace{Tracer: tracer}

	inst, uninst, installState, installedVersion := prepareConfigurePackage(
		tracer,
		buildConfigSimple(pluginInformation),
		repoMock,
		serviceMock,
		pluginInformation,
		"packageArn",
		"0.0.1",
		output)

	assert.Nil(t, inst)
	assert.NotNil(t, uninst)
	assert.Equal(t, localpackages.Installed, installState)
	assert.NotEmpty(t, installedVersion)
	assert.Equal(t, 0, output.GetExitCode())
	assert.Empty(t, tracer.ToPluginOutput().GetStderr())

	installerMock.AssertExpectations(t)
}

func TestPrepareUninstallWrongVersion(t *testing.T) {
	// file stubs are needed for ensurePackage because it handles the unzip
	stubs := setSuccessStubs()
	defer stubs.Clear()

	pluginInformation := createStubPluginInputUninstall("2.3.4")
	installerMock := installerNotCalledMock()
	repoMock := repoUninstallMock(pluginInformation, installerMock)
	serviceMock := serviceSuccessMock()
	tracer := trace.NewTracer(log.NewMockLog())
	output := &trace.PluginOutputTrace{Tracer: tracer}

	inst, uninst, installState, installedVersion := prepareConfigurePackage(
		tracer,
		buildConfigSimple(pluginInformation),
		repoMock,
		serviceMock,
		pluginInformation,
		"packageArn",
		"0.0.1",
		output)

	assert.Nil(t, inst)
	assert.Nil(t, uninst)
	assert.Equal(t, localpackages.Installed, installState)
	assert.NotEmpty(t, installedVersion)
	assert.Equal(t, 1, output.GetExitCode())
	assert.NotEmpty(t, tracer.ToPluginOutput().GetStderr())

	installerMock.AssertExpectations(t)
}

func TestInstalledValid(t *testing.T) {
	pluginInformation := createStubPluginInputInstall()
	installerMock := installerSuccessMock(pluginInformation.Name, pluginInformation.Version)
	uninstallerMock := installerNotCalledMock()
	repoMock := repoInstallMock(pluginInformation, installerMock)
	tracer := trace.NewTracer(log.NewMockLog())
	output := &trace.PluginOutputTrace{Tracer: tracer}

	alreadyInstalled := checkAlreadyInstalled(
		tracer,
		contextMock,
		repoMock,
		installerMock.Version(),
		localpackages.Installed,
		installerMock,
		uninstallerMock,
		output)
	assert.True(t, alreadyInstalled)
}

func TestNotInstalled(t *testing.T) {
	pluginInformation := createStubPluginInputInstall()
	installerMock := installerSuccessMock(pluginInformation.Name, pluginInformation.Version)
	uninstallerMock := installerNotCalledMock()
	repoMock := repoInstallMock(pluginInformation, installerMock)
	tracer := trace.NewTracer(log.NewMockLog())
	output := &trace.PluginOutputTrace{Tracer: tracer}

	alreadyInstalled := checkAlreadyInstalled(
		tracer,
		contextMock,
		repoMock,
		"",
		localpackages.None,
		installerMock,
		uninstallerMock,
		output)
	assert.False(t, alreadyInstalled)
}

func TestOtherVersionInstalled(t *testing.T) {
	pluginInformation := createStubPluginInputInstall()
	installerMock := installerSuccessMock(pluginInformation.Name, pluginInformation.Version)
	uninstallerMock := installerNotCalledMock()
	repoMock := repoInstallMock(pluginInformation, installerMock)
	tracer := trace.NewTracer(log.NewMockLog())
	output := &trace.PluginOutputTrace{Tracer: tracer}

	alreadyInstalled := checkAlreadyInstalled(
		tracer,
		contextMock,
		repoMock,
		"2.3.4",
		localpackages.Installed,
		installerMock,
		uninstallerMock,
		output)
	assert.False(t, alreadyInstalled)
}

func TestInstallingValid(t *testing.T) {
	pluginInformation := createStubPluginInputInstall()
	installerMock := installerSuccessMock(pluginInformation.Name, pluginInformation.Version)
	uninstallerMock := installerNameVersionOnlyMock(pluginInformation.Name, pluginInformation.Version)
	repoMock := repoInstallMock(pluginInformation, installerMock)
	repoMock.On("RemovePackage", mock.Anything, pluginInformation.Name, pluginInformation.Version).Return(nil)
	tracer := trace.NewTracer(log.NewMockLog())
	output := &trace.PluginOutputTrace{Tracer: tracer}

	alreadyInstalled := checkAlreadyInstalled(
		tracer,
		contextMock,
		repoMock,
		installerMock.Version(),
		localpackages.Installing,
		installerMock,
		uninstallerMock,
		output)
	assert.True(t, alreadyInstalled)
}

func TestRollbackValid(t *testing.T) {
	pluginInformation := createStubPluginInputInstall()
	installerMock := installerNameVersionOnlyMock(pluginInformation.Name, pluginInformation.Version)
	uninstallerMock := installerSuccessMock(pluginInformation.Name, pluginInformation.Version)
	repoMock := repoInstallMock(pluginInformation, installerMock)
	repoMock.On("RemovePackage", mock.Anything, pluginInformation.Name, pluginInformation.Version).Return(nil)
	tracer := trace.NewTracer(log.NewMockLog())
	output := &trace.PluginOutputTrace{Tracer: tracer}

	alreadyInstalled := checkAlreadyInstalled(
		tracer,
		contextMock,
		repoMock,
		installerMock.Version(),
		localpackages.RollbackInstall,
		installerMock,
		uninstallerMock,
		output)
	assert.True(t, alreadyInstalled)
}

func TestInstallingNotValid(t *testing.T) {
	pluginInformation := createStubPluginInputInstall()
	installerMock := installerInvalidMock(pluginInformation.Name, pluginInformation.Version)
	uninstallerMock := installerNotCalledMock()
	repoMock := repoInstallMock(pluginInformation, installerMock)
	repoMock.On("RemovePackage", mock.Anything, pluginInformation.Name, pluginInformation.Version).Return(nil)
	tracer := trace.NewTracer(log.NewMockLog())
	output := &trace.PluginOutputTrace{Tracer: tracer}

	alreadyInstalled := checkAlreadyInstalled(
		tracer,
		contextMock,
		repoMock,
		installerMock.Version(),
		localpackages.Installing,
		installerMock,
		uninstallerMock,
		output)
	assert.False(t, alreadyInstalled)
}

func TestExecute(t *testing.T) {
	// file stubs are needed for ensurePackage because it handles the unzip
	stubs := setSuccessStubs()
	defer stubs.Clear()

	pluginInformation := createStubPluginInputInstall()
	installerMock := installerSuccessMock(pluginInformation.Name, pluginInformation.Version)
	repoMock := repoInstallMock(pluginInformation, installerMock)
	serviceMock := serviceSuccessMock()

	plugin := &Plugin{
		localRepository:        repoMock,
		packageServiceSelector: selectMockService(serviceMock),
	}
	plugin.execute(contextMock, buildConfigSimple(pluginInformation), createMockCancelFlag(), createMockIOHandler())

	repoMock.AssertExpectations(t)
	installerMock.AssertExpectations(t)
	serviceMock.AssertExpectations(t)
}

func TestExecuteArrayInput(t *testing.T) {
	pluginInformation := createStubPluginInputInstall()
	installerMock := installerSuccessMock(pluginInformation.Name, pluginInformation.Version)
	repoMock := repoInstallMock(pluginInformation, installerMock)
	serviceMock := serviceSuccessMock()

	plugin := &Plugin{
		localRepository:        repoMock,
		packageServiceSelector: selectMockService(serviceMock),
	}

	config := contracts.Configuration{}
	var rawPluginInputs []interface{}
	rawPluginInputs = append(rawPluginInputs, pluginInformation)
	rawPluginInputs = append(rawPluginInputs, pluginInformation)
	config.Properties = rawPluginInputs

	plugin.execute(contextMock, config, createMockCancelFlag(), createMockIOHandler())
}

func TestConfigurePackage_InvalidAction(t *testing.T) {
	pluginInformation := createStubPluginInputFoo()
	installerMock := installerNotCalledMock()
	repoMock := repoInstallMock(pluginInformation, installerMock)
	serviceMock := serviceSuccessMock()

	plugin := &Plugin{
		localRepository:        repoMock,
		packageServiceSelector: selectMockService(serviceMock),
	}
	plugin.execute(contextMock, buildConfigSimple(pluginInformation), createMockCancelFlag(), createMockIOHandler())
}

func buildConfigSimple(pluginInformation *ConfigurePackagePluginInput) contracts.Configuration {
	config := contracts.Configuration{}

	var rawPluginInput interface{}
	rawPluginInput = pluginInformation
	config.Properties = rawPluginInput

	return config
}

func buildConfig(pluginInformation *ConfigurePackagePluginInput, orchestrationDir string, bucketName string, prefix string, pluginID string) contracts.Configuration {
	config := contracts.Configuration{}
	config.OrchestrationDirectory = orchestrationDir
	config.OutputS3BucketName = bucketName
	config.OutputS3KeyPrefix = prefix
	config.PluginID = pluginID

	var rawPluginInput interface{}
	rawPluginInput = pluginInformation
	config.Properties = rawPluginInput

	return config
}

func TestValidateInput(t *testing.T) {
	input := ConfigurePackagePluginInput{}

	input.Version = "1.0.0"
	input.Name = "PVDriver"
	input.Action = "InvalidAction"

	result, err := validateInput(&input)

	assert.True(t, result)
	assert.NoError(t, err)
}

func TestValidateInput_Source(t *testing.T) {
	input := ConfigurePackagePluginInput{}

	input.Version = "1.0.0"
	input.Name = "PVDriver"
	input.Action = "Install"
	input.Source = "http://amazon.com"

	result, err := validateInput(&input)

	assert.False(t, result)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "source parameter is not supported")
}

func TestValidateInput_NameEmpty(t *testing.T) {
	input := ConfigurePackagePluginInput{}

	// Set version to a large number to avoid conflict of the actual package release version
	input.Version = "9000.0.0"
	input.Name = ""
	input.Action = "Install"

	result, err := validateInput(&input)

	assert.False(t, result)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "empty name field")
}

func TestValidateInput_NameValid(t *testing.T) {
	input := ConfigurePackagePluginInput{}
	// Set version to a large number to avoid conflict of the actual package release version
	input.Version = "9000.0.0"
	input.Action = "Install"

	validNames := []string{"a0", "_a", "_._._", "_-_", "A", "ABCDEFGHIJKLM-NOPQRSTUVWXYZ.abcdefghijklm-nopqrstuvwxyz.1234567890"}

	for _, name := range validNames {
		input.Name = name

		result, err := validateInput(&input)

		assert.True(t, result)
		assert.NoError(t, err)
	}
}

func TestValidateInput_VersionValid(t *testing.T) {
	input := ConfigurePackagePluginInput{}

	validVersions := []string{"1.0.0", "9000.0.0.0", "0.21.0", "1.2.3.4", "5.4.3.2.1", "1.2.3-a.b.c.10.d.5+beta"}

	for _, version := range validVersions {
		input.Version = version
		input.Name = "PVDriver"
		input.Action = "Install"

		result, err := validateInput(&input)

		assert.True(t, result)
		assert.NoError(t, err)
	}
}

func TestValidateInput_EmptyVersionWithInstall(t *testing.T) {
	input := ConfigurePackagePluginInput{}

	// Set version to a large number to avoid conflict of the actual package release version
	input.Version = ""
	input.Name = "PVDriver"
	input.Action = "Install"

	result, err := validateInput(&input)

	assert.True(t, result)
	assert.NoError(t, err)
}

func TestValidateInput_EmptyVersionWithUninstall(t *testing.T) {
	input := ConfigurePackagePluginInput{}

	// Set version to a large number to avoid conflict of the actual package release version
	input.Version = ""
	input.Name = "PVDriver"
	input.Action = "Uninstall"

	result, err := validateInput(&input)

	assert.True(t, result)
	assert.NoError(t, err)
}

func TestGetShortNameAndNoVersion(t *testing.T) {
	pluginInformation := createStubPluginInputInstallLatest()
	serviceMock := serviceSuccessMock()
	tracer := trace.NewTracer(log.NewMockLog())

	packageArn, version, err := getPackageArnAndVersion(
		tracer,
		serviceMock,
		pluginInformation)

	assert.Equal(t, "packageArn", packageArn)
	assert.Equal(t, "0.0.1", version)
	assert.NoError(t, err)
	assert.Empty(t, tracer.ToPluginOutput().GetStderr())
}

func TestGetShortNameAndLatestVersion(t *testing.T) {
	pluginInformation := createStubPluginInputUninstall("latest")
	serviceMock := serviceUpgradeMock()
	tracer := trace.NewTracer(log.NewMockLog())

	packageArn, version, err := getPackageArnAndVersion(
		tracer,
		serviceMock,
		pluginInformation)

	assert.Equal(t, "packageArn", packageArn)
	assert.Equal(t, "0.0.2", version)
	assert.NoError(t, err)
	assert.Empty(t, tracer.ToPluginOutput().GetStderr())
}

func TestGetShortNameAndVersion(t *testing.T) {
	pluginInformation := createStubPluginInputInstall()
	serviceMock := serviceSuccessMock()
	tracer := trace.NewTracer(log.NewMockLog())

	packageArn, version, err := getPackageArnAndVersion(
		tracer,
		serviceMock,
		pluginInformation)

	assert.Equal(t, "packageArn", packageArn)
	assert.Equal(t, "0.0.1", version)
	assert.NoError(t, err)
	assert.Empty(t, tracer.ToPluginOutput().GetStderr())
}

func TestGetShortArnAndVersionFailed(t *testing.T) {
	pluginInformation := createStubPluginInputInstall()
	serviceMock := serviceFailedMock()
	tracer := trace.NewTracer(log.NewMockLog())

	packageArn, version, err := getPackageArnAndVersion(
		tracer,
		serviceMock,
		pluginInformation)

	assert.Empty(t, packageArn)
	assert.Empty(t, version)
	assert.Error(t, err)
	assert.Equal(t, "testerror\n", tracer.ToPluginOutput().GetStderr())
}
