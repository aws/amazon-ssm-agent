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

//go:build e2e
// +build e2e

package configurepackage

import (
	"errors"
	"io/ioutil"
	"testing"

	"github.com/aws/amazon-ssm-agent/agent/appconfig"
	"github.com/aws/amazon-ssm-agent/agent/contracts"
	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/amazon-ssm-agent/agent/plugins/configurepackage/birdwatcher/facade"
	facadeMock "github.com/aws/amazon-ssm-agent/agent/plugins/configurepackage/birdwatcher/facade/mocks"
	"github.com/aws/amazon-ssm-agent/agent/plugins/configurepackage/envdetect/ec2infradetect"
	"github.com/aws/amazon-ssm-agent/agent/plugins/configurepackage/localpackages"
	"github.com/aws/amazon-ssm-agent/agent/plugins/configurepackage/packageservice"
	"github.com/aws/amazon-ssm-agent/agent/plugins/configurepackage/trace"
	"github.com/aws/amazon-ssm-agent/common/identity"
	"github.com/aws/aws-sdk-go/service/ssm"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// getConfig mock
func init() {
	getConfig = func(reload bool) (appconfig.SsmagentConfig, error) {
		return appconfig.DefaultConfig(), nil
	}
}

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

func createStubPluginInputInstallWithAdditionalArguments() *ConfigurePackagePluginInput {
	input := ConfigurePackagePluginInput{}

	input.Version = "0.0.1"
	input.Name = "PVDriver"
	input.Action = "Install"
	input.AdditionalArguments = "{\"customArg1\":\"customVal1\", \"customArg2\":\"customVal2\"}"

	return &input
}

func createStubPluginInputUninstallLatest() *ConfigurePackagePluginInput {
	input := ConfigurePackagePluginInput{}

	input.Name = "PVDriver"
	input.Action = "Uninstall"

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

func createStubPluginInputUninstallWithAdditionalArguments() *ConfigurePackagePluginInput {
	input := ConfigurePackagePluginInput{}

	input.Name = "PVDriver"
	input.Action = "Uninstall"
	input.AdditionalArguments = "{\"customArg1\":\"customVal1\", \"customArg2\":\"customVal2\"}"

	return &input
}

func createStubPluginInputUpgrade() *ConfigurePackagePluginInput {
	input := ConfigurePackagePluginInput{}

	input.Version = "0.0.2"
	input.Name = "PVDriver"
	input.Action = "Install"
	input.InstallationType = InstallationTypeLegacy

	return &input
}

func createStubPluginInputUpdate() *ConfigurePackagePluginInput {
	input := ConfigurePackagePluginInput{}

	input.Version = "0.0.2"
	input.Name = "packageArn"
	input.Action = "Install"
	input.InstallationType = InstallationTypeInPlace

	return &input
}

func createStubPluginInputUpdateWithAdditionalArguments() *ConfigurePackagePluginInput {
	input := ConfigurePackagePluginInput{}

	input.Version = "0.0.2"
	input.Name = "packageArn"
	input.Action = "Install"
	input.InstallationType = InstallationTypeInPlace
	input.AdditionalArguments = "{\"customArg1\":\"customVal1\", \"customArg2\":\"customVal2\"}"

	return &input
}

func createStubPluginInputFoo() *ConfigurePackagePluginInput {
	input := ConfigurePackagePluginInput{}

	input.Version = "0.0.1"
	input.Name = "PVDriver"
	input.Action = "Foo"

	return &input
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

	inst, uninst, isUpdateInPlace, installState, installedVersion := prepareConfigurePackage(
		tracer,
		buildConfigSimple(pluginInformation),
		repoMock,
		serviceMock,
		pluginInformation,
		"packageArn",
		"0.0.1",
		false,
		output)

	assert.NotNil(t, inst)
	assert.Nil(t, uninst)
	assert.False(t, isUpdateInPlace)
	assert.Equal(t, localpackages.None, installState)
	assert.Empty(t, installedVersion)
	assert.Equal(t, 0, output.GetExitCode())
	assert.Empty(t, tracer.ToPluginOutput().GetStderr())

	installerMock.AssertExpectations(t)
	repoMock.AssertExpectations(t)
}

func TestPrepareNewInstallWithAdditionalArguments(t *testing.T) {
	// file stubs are needed for ensurePackage because it handles the unzip
	stubs := setSuccessStubs()
	defer stubs.Clear()

	pluginInformation := createStubPluginInputInstallWithAdditionalArguments()
	installerMock := installerNotCalledMock()
	repoMock := repoInstallMock(pluginInformation, installerMock)
	serviceMock := serviceSuccessMock()
	tracer := trace.NewTracer(log.NewMockLog())
	output := &trace.PluginOutputTrace{Tracer: tracer}

	prepareConfigurePackage(
		tracer,
		buildConfigSimple(pluginInformation),
		repoMock,
		serviceMock,
		pluginInformation,
		"packageArn",
		"0.0.1",
		false,
		output)

	// verify repoMock called GetInstaller with additional arguments
	repoMock.AssertExpectations(t)
}

func TestAlreadyInstalled(t *testing.T) {
	// file stubs are needed for ensurePackage because it handles the unzip
	stubs := setSuccessStubs()
	defer stubs.Clear()

	pluginInformation := createStubPluginInputInstall()
	installerMock := installerNotCalledMock()
	repoMock := repoAlreadyInstalledMock(pluginInformation, installerMock)
	serviceMock := serviceSameManifestCacheMock()
	tracer := trace.NewTracer(log.NewMockLog())
	output := &trace.PluginOutputTrace{Tracer: tracer}

	inst, uninst, isUpdateInPlace, installState, installedVersion := prepareConfigurePackage(
		tracer,
		buildConfigSimple(pluginInformation),
		repoMock,
		serviceMock,
		pluginInformation,
		"packageArn",
		"0.0.1",
		true,
		output)

	assert.NotNil(t, inst)
	assert.Nil(t, uninst)
	assert.False(t, isUpdateInPlace)
	assert.Equal(t, localpackages.Installed, installState)
	assert.Equal(t, "0.0.1", installedVersion)
	assert.Equal(t, 0, output.GetExitCode())
	assert.Empty(t, tracer.ToPluginOutput().GetStderr())

	installerMock.AssertExpectations(t)
	repoMock.AssertExpectations(t)
}

func TestPrepareUpgrade(t *testing.T) {
	// file stubs are needed for ensurePackage because it handles the unzip
	stubs := setSuccessStubs()
	defer stubs.Clear()

	pluginInformation := createStubPluginInputInstallLatest()
	installerMock := installerNotCalledMock()
	repoMock := repoUpgradeMock(installerMock)
	serviceMock := serviceUpgradeMock()
	tracer := trace.NewTracer(log.NewMockLog())
	output := &trace.PluginOutputTrace{Tracer: tracer}

	inst, uninst, isUpdateInPlace, installState, installedVersion := prepareConfigurePackage(
		tracer,
		buildConfigSimple(pluginInformation),
		repoMock,
		serviceMock,
		pluginInformation,
		"packageArn",
		"0.0.2",
		false,
		output)

	assert.NotNil(t, inst)
	assert.NotNil(t, uninst)
	assert.False(t, isUpdateInPlace)
	assert.Equal(t, localpackages.Installed, installState)
	assert.NotEmpty(t, installedVersion)
	assert.Equal(t, 0, output.GetExitCode())
	assert.Empty(t, tracer.ToPluginOutput().GetStderr())

	installerMock.AssertExpectations(t)
	repoMock.AssertExpectations(t)
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

	inst, uninst, isUpdateInPlace, installState, installedVersion := prepareConfigurePackage(
		tracer,
		buildConfigSimple(pluginInformation),
		repoMock,
		serviceMock,
		pluginInformation,
		"packageArn",
		"0.0.1",
		false,
		output)

	assert.Nil(t, inst)
	assert.NotNil(t, uninst)
	assert.False(t, isUpdateInPlace)
	assert.Equal(t, localpackages.Installed, installState)
	assert.NotEmpty(t, installedVersion)
	assert.Equal(t, 0, output.GetExitCode())
	assert.Empty(t, tracer.ToPluginOutput().GetStderr())

	installerMock.AssertExpectations(t)
	repoMock.AssertExpectations(t)
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

	inst, uninst, isUpdateInPlace, installState, installedVersion := prepareConfigurePackage(
		tracer,
		buildConfigSimple(pluginInformation),
		repoMock,
		serviceMock,
		pluginInformation,
		"packageArn",
		"0.0.1",
		false,
		output)

	assert.Nil(t, inst)
	assert.NotNil(t, uninst)
	assert.False(t, isUpdateInPlace)
	assert.Equal(t, localpackages.Installed, installState)
	assert.NotEmpty(t, installedVersion)
	assert.Equal(t, 0, output.GetExitCode())
	assert.Empty(t, tracer.ToPluginOutput().GetStderr())

	installerMock.AssertExpectations(t)
	repoMock.AssertExpectations(t)
}

func TestPrepareUninstallCurrentWithLatest(t *testing.T) {
	// file stubs are needed for ensurePackage because it handles the unzip
	stubs := setSuccessStubs()
	defer stubs.Clear()

	pluginInformation := createStubPluginInputUninstall("latest")
	installerMock := installerNotCalledMock()
	repoMock := repoUninstallMock(pluginInformation, installerMock)
	serviceMock := serviceSuccessMock()
	tracer := trace.NewTracer(log.NewMockLog())
	output := &trace.PluginOutputTrace{Tracer: tracer}

	inst, uninst, isUpdateInPlace, installState, installedVersion := prepareConfigurePackage(
		tracer,
		buildConfigSimple(pluginInformation),
		repoMock,
		serviceMock,
		pluginInformation,
		"packageArn",
		"0.0.1",
		false,
		output)

	assert.Nil(t, inst)
	assert.NotNil(t, uninst)
	assert.False(t, isUpdateInPlace)
	assert.Equal(t, localpackages.Installed, installState)
	assert.NotEmpty(t, installedVersion)
	assert.Equal(t, 0, output.GetExitCode())
	assert.Empty(t, tracer.ToPluginOutput().GetStderr())

	installerMock.AssertExpectations(t)
	repoMock.AssertExpectations(t)
}

func TestPrepareUninstallWithAdditionalArguments(t *testing.T) {
	// file stubs are needed for ensurePackage because it handles the unzip
	stubs := setSuccessStubs()
	defer stubs.Clear()

	pluginInformation := createStubPluginInputUninstallWithAdditionalArguments()
	installerMock := installerNotCalledMock()
	repoMock := repoUninstallMock(pluginInformation, installerMock)
	serviceMock := serviceSuccessMock()
	tracer := trace.NewTracer(log.NewMockLog())
	output := &trace.PluginOutputTrace{Tracer: tracer}

	prepareConfigurePackage(
		tracer,
		buildConfigSimple(pluginInformation),
		repoMock,
		serviceMock,
		pluginInformation,
		"packageArn",
		"0.0.1",
		false,
		output)

	// verify repoMock called GetInstaller with additional arguments
	repoMock.AssertExpectations(t)
}

func TestPrepareUninstallWrongVersion(t *testing.T) {
	// file stubs are needed for ensurePackage because it handles the unzip
	stubs := setSuccessStubs()
	defer stubs.Clear()

	pluginInformation := createStubPluginInputUninstall("2.3.4")
	installerMock := installerNotCalledMock()
	repoMock := repoUninstallMockWrongVersion(pluginInformation, installerMock)
	serviceMock := serviceSuccessMock()
	tracer := trace.NewTracer(log.NewMockLog())
	output := &trace.PluginOutputTrace{Tracer: tracer}

	inst, uninst, isUpdateInPlace, installState, installedVersion := prepareConfigurePackage(
		tracer,
		buildConfigSimple(pluginInformation),
		repoMock,
		serviceMock,
		pluginInformation,
		"packageArn",
		"2.3.4",
		false,
		output)

	assert.Nil(t, inst)
	assert.Nil(t, uninst)
	assert.False(t, isUpdateInPlace)
	assert.Equal(t, localpackages.None, installState)
	assert.NotEmpty(t, installedVersion)
	assert.Equal(t, 0, output.GetExitCode())
	assert.Empty(t, tracer.ToPluginOutput().GetStderr())

	installerMock.AssertExpectations(t)
	repoMock.AssertExpectations(t)
}

func TestPrepareUpdate(t *testing.T) {
	// file stubs are needed for ensurePackage because it handles the unzip
	stubs := setSuccessStubs()
	defer stubs.Clear()

	pluginInformation := createStubPluginInputUpdate()
	installerMock := installerNotCalledMock()
	repoMock := repoUpdateMock(pluginInformation, installerMock)
	serviceMock := serviceUpdateMock()
	tracer := trace.NewTracer(log.NewMockLog())
	output := &trace.PluginOutputTrace{Tracer: tracer}

	inst, uninst, isUpdateInPlace, installState, installedVersion := prepareConfigurePackage(
		tracer,
		buildConfigSimple(pluginInformation),
		repoMock,
		serviceMock,
		pluginInformation,
		"packageArn",
		"0.0.2",
		false,
		output)

	assert.NotNil(t, inst)
	assert.NotNil(t, uninst)
	assert.True(t, isUpdateInPlace)
	assert.Equal(t, localpackages.Installed, installState)
	assert.NotEmpty(t, installedVersion)
	assert.Equal(t, 0, output.GetExitCode())
	assert.Empty(t, tracer.ToPluginOutput().GetStderr())

	installerMock.AssertExpectations(t)
	repoMock.AssertExpectations(t)
}

func TestPrepareLegacyUpgrade(t *testing.T) {
	// file stubs are needed for ensurePackage because it handles the unzip
	stubs := setSuccessStubs()
	defer stubs.Clear()

	pluginInformation := createStubPluginInputUpgrade()
	installerMock := installerNotCalledMock()
	repoMock := repoUpgradeMock(installerMock)
	serviceMock := serviceUpgradeMock()
	tracer := trace.NewTracer(log.NewMockLog())
	output := &trace.PluginOutputTrace{Tracer: tracer}

	inst, uninst, isUpdateInPlace, installState, installedVersion := prepareConfigurePackage(
		tracer,
		buildConfigSimple(pluginInformation),
		repoMock,
		serviceMock,
		pluginInformation,
		"packageArn",
		"0.0.2",
		false,
		output)

	assert.NotNil(t, inst)
	assert.NotNil(t, uninst)
	assert.False(t, isUpdateInPlace)
	assert.Equal(t, localpackages.Installed, installState)
	assert.NotEmpty(t, installedVersion)
	assert.Equal(t, 0, output.GetExitCode())
	assert.Empty(t, tracer.ToPluginOutput().GetStderr())

	installerMock.AssertExpectations(t)
	repoMock.AssertExpectations(t)
}

func TestPrepareUpdateWithAdditionalArguments(t *testing.T) {
	// file stubs are needed for ensurePackage because it handles the unzip
	stubs := setSuccessStubs()
	defer stubs.Clear()

	pluginInformation := createStubPluginInputUpdateWithAdditionalArguments()
	installerMock := installerNotCalledMock()
	repoMock := repoUpdateMock(pluginInformation, installerMock)
	serviceMock := serviceUpdateMock()
	tracer := trace.NewTracer(log.NewMockLog())
	output := &trace.PluginOutputTrace{Tracer: tracer}

	prepareConfigurePackage(
		tracer,
		buildConfigSimple(pluginInformation),
		repoMock,
		serviceMock,
		pluginInformation,
		"packageArn",
		"0.0.2",
		false,
		output)

	// verify repoMock called GetInstaller with additional arguments
	repoMock.AssertExpectations(t)
}

func TestPrepareUpgrade_BirdwatcherService_InvalidInPlaceInstallationType(t *testing.T) {
	// file stubs are needed for ensurePackage because it handles the unzip
	stubs := setSuccessStubs()
	defer stubs.Clear()

	pluginInformation := createStubPluginInputUpdate()
	installerMock := installerNotCalledMock()
	repoMock := repoMockNotInvoked()
	serviceMock := birdwatcherServiceMockForPrepareUpdate()
	tracer := trace.NewTracer(log.NewMockLog())
	output := &trace.PluginOutputTrace{Tracer: tracer}

	prepareConfigurePackage(
		tracer,
		buildConfigSimple(pluginInformation),
		repoMock,
		serviceMock,
		pluginInformation,
		"packageArn",
		"0.0.2",
		false,
		output)

	installerMock.AssertExpectations(t)
	repoMock.AssertExpectations(t)
	serviceMock.AssertExpectations(t)
}

// Test that if Update is triggered but the requested package does not exist, use the install script to install
func TestPrepareUpdate_PackageNotInstalled(t *testing.T) {
	// file stubs are needed for ensurePackage because it handles the unzip
	stubs := setSuccessStubs()
	defer stubs.Clear()

	pluginInformation := createStubPluginInputUpdate()
	installerMock := installerNotCalledMock()
	repoMock := repoUpdateMock_PackageNotInstalled(pluginInformation, installerMock)
	serviceMock := serviceUpdateMock()
	tracer := trace.NewTracer(log.NewMockLog())
	output := &trace.PluginOutputTrace{Tracer: tracer}

	inst, uninst, isUpdateInPlace, installState, installedVersion := prepareConfigurePackage(
		tracer,
		buildConfigSimple(pluginInformation),
		repoMock,
		serviceMock,
		pluginInformation,
		"packageArn",
		"0.0.2",
		false,
		output)

	assert.NotNil(t, inst)
	assert.Nil(t, uninst)
	assert.True(t, isUpdateInPlace)
	assert.Equal(t, localpackages.None, installState)
	assert.Empty(t, installedVersion)
	assert.Equal(t, 0, output.GetExitCode())
	assert.Empty(t, tracer.ToPluginOutput().GetStderr())

	installerMock.AssertExpectations(t)
	repoMock.AssertExpectations(t)
}

// Test that if Update is triggered and the requested version already exists but with different manifest,
// update to the requested version with support to roll back to current version/manifest
func TestPrepareUpdate_VersionAlreadyInstalled_ManifestDifferent(t *testing.T) {
	// file stubs are needed for ensurePackage because it handles the unzip
	stubs := setSuccessStubs()
	defer stubs.Clear()

	pluginInformation := createStubPluginInputUpdate()
	installerMock := installerNotCalledMock()
	repoMock := repoUpdateMock_VersionAlreadyInstalledWithDifferentManifest(pluginInformation, installerMock)
	serviceMock := serviceUpdateMock()
	tracer := trace.NewTracer(log.NewMockLog())
	output := &trace.PluginOutputTrace{Tracer: tracer}

	inst, uninst, isUpdateInPlace, installState, installedVersion := prepareConfigurePackage(
		tracer,
		buildConfigSimple(pluginInformation),
		repoMock,
		serviceMock,
		pluginInformation,
		"packageArn",
		"0.0.2",
		false,
		output)

	assert.NotNil(t, inst)
	assert.NotNil(t, uninst)
	assert.True(t, isUpdateInPlace)
	assert.Equal(t, localpackages.Installed, installState)
	assert.NotEmpty(t, installedVersion)
	assert.Equal(t, 0, output.GetExitCode())
	assert.Empty(t, tracer.ToPluginOutput().GetStderr())

	installerMock.AssertExpectations(t)
	repoMock.AssertExpectations(t)
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
	repoMock.On("SetInstallState", mock.Anything, mock.Anything, pluginInformation.Version, mock.Anything).Return(nil)
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

func TestUpdatingValid(t *testing.T) {
	pluginInformation := createStubPluginInputInstall()
	installerMock := trueUpdateInstallerMock(pluginInformation.Name, pluginInformation.Version)
	uninstallerMock := installerNameVersionOnlyMock(pluginInformation.Name, pluginInformation.Version)
	repoMock := repoInstallMock(pluginInformation, installerMock)
	repoMock.On("SetInstallState", mock.Anything, mock.Anything, pluginInformation.Version, mock.Anything).Return(nil)
	repoMock.On("RemovePackage", mock.Anything, pluginInformation.Name, pluginInformation.Version).Return(nil)
	tracer := trace.NewTracer(log.NewMockLog())
	output := &trace.PluginOutputTrace{Tracer: tracer}

	alreadyInstalled := checkAlreadyInstalled(
		tracer,
		contextMock,
		repoMock,
		installerMock.Version(),
		localpackages.Updating,
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
	repoMock.On("SetInstallState", mock.Anything, mock.Anything, pluginInformation.Version, mock.Anything).Return(nil)
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

func TestUnknownValid(t *testing.T) {
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
		localpackages.Unknown,
		installerMock,
		uninstallerMock,
		output)
	assert.True(t, alreadyInstalled)
}

func TestUnknownNotValid(t *testing.T) {
	pluginInformation := createStubPluginInputInstall()
	installerMock := installerSuccessMock(pluginInformation.Name, pluginInformation.Version)
	uninstallerMock := installerNotCalledMock()
	repoMock := repoInstallMock_WithValidatePackageError(pluginInformation, installerMock)
	tracer := trace.NewTracer(log.NewMockLog())
	output := &trace.PluginOutputTrace{Tracer: tracer}

	alreadyInstalled := checkAlreadyInstalled(
		tracer,
		contextMock,
		repoMock,
		installerMock.Version(),
		localpackages.Unknown,
		installerMock,
		uninstallerMock,
		output)
	assert.False(t, alreadyInstalled)
}

// Testing Execute module unit tests
func TestExecute(t *testing.T) {
	// file stubs are needed for ensurePackage because it handles the unzip
	stubs := setSuccessStubs()
	defer stubs.Clear()

	pluginInformation := createStubPluginInputInstall()
	installerMock := installerSuccessMock(pluginInformation.Name, pluginInformation.Version)
	repoMock := repoInstallMock(pluginInformation, installerMock)
	repoMock.On("SetInstallState", mock.Anything, mock.Anything, pluginInformation.Version, mock.Anything).Return(nil)
	repoMock.On("LockPackage", mock.Anything, mock.Anything, mock.Anything).Return(nil)
	repoMock.On("UnlockPackage", mock.Anything, mock.Anything).Return()
	repoMock.On("LoadTraces", mock.Anything, mock.Anything).Return(nil)
	serviceMock := serviceSuccessMock()

	plugin := &Plugin{
		context:                contextMock,
		localRepository:        repoMock,
		packageServiceSelector: selectMockService(serviceMock),
	}
	plugin.execute(buildConfigSimple(pluginInformation), createMockCancelFlag(), createMockIOHandler())

	repoMock.AssertExpectations(t)
	installerMock.AssertExpectations(t)
	serviceMock.AssertExpectations(t)
}

// Test that if Update is triggered but the requested package does not exist, use the install script and install action
func TestExecuteUpdate_PackageNotInstalled_TreatAsInstall(t *testing.T) {
	// file stubs are needed for ensurePackage because it handles the unzip
	stubs := setSuccessStubs()
	defer stubs.Clear()

	pluginInformation := createStubPluginInputUpdate()
	installerMock := installerSuccessMock(pluginInformation.Name, pluginInformation.Version)
	repoMock := repoUpdateMock_PackageNotInstalled(pluginInformation, installerMock)
	repoMock.On("SetInstallState", mock.Anything, mock.Anything, pluginInformation.Version, mock.Anything).Return(nil)
	repoMock.On("LockPackage", mock.Anything, mock.Anything, mock.Anything).Return(nil)
	repoMock.On("UnlockPackage", mock.Anything, mock.Anything).Return()
	repoMock.On("LoadTraces", mock.Anything, mock.Anything).Return(nil)
	serviceMock := serviceUpdateMock()

	plugin := &Plugin{
		context:                contextMock,
		localRepository:        repoMock,
		packageServiceSelector: selectMockService(serviceMock),
	}
	plugin.execute(buildConfigSimple(pluginInformation), createMockCancelFlag(), createMockIOHandler())

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
		context:                contextMock,
		localRepository:        repoMock,
		packageServiceSelector: selectMockService(serviceMock),
	}

	config := contracts.Configuration{}
	var rawPluginInputs []interface{}
	rawPluginInputs = append(rawPluginInputs, pluginInformation)
	rawPluginInputs = append(rawPluginInputs, pluginInformation)
	config.Properties = rawPluginInputs

	plugin.execute(config, createMockCancelFlag(), createMockIOHandler())
}

func TestConfigurePackage_InvalidAction(t *testing.T) {
	pluginInformation := createStubPluginInputFoo()
	installerMock := installerNotCalledMock()
	repoMock := repoInstallMock(pluginInformation, installerMock)
	repoMock.On("SetInstallState", mock.Anything, mock.Anything, pluginInformation.Version, mock.Anything).Return(nil)
	repoMock.On("LockPackage", mock.Anything, mock.Anything, mock.Anything).Return(nil)
	repoMock.On("UnlockPackage", mock.Anything, mock.Anything).Return()
	repoMock.On("LoadTraces", mock.Anything, mock.Anything).Return(nil)
	serviceMock := serviceSuccessMock()

	plugin := &Plugin{
		context:                contextMock,
		localRepository:        repoMock,
		packageServiceSelector: selectMockService(serviceMock),
	}
	plugin.execute(buildConfigSimple(pluginInformation), createMockCancelFlag(), createMockIOHandler())
}

func TestConfigurePackageForUpdate_BirdwatcherService_InvalidInPlaceInstallationType(t *testing.T) {
	pluginInformation := createStubPluginInputUpdate()
	installerMock := installerNotCalledMock()
	repoMock := repoUpdateMock_BirdwatcherNotAllowed()
	serviceMock := birdwatcherServiceMock()

	plugin := &Plugin{
		context:                contextMock,
		localRepository:        repoMock,
		packageServiceSelector: selectMockService(serviceMock),
	}
	plugin.execute(buildConfigSimple(pluginInformation), createMockCancelFlag(), createMockIOHandler())

	installerMock.AssertExpectations(t)
	repoMock.AssertExpectations(t)
	serviceMock.AssertExpectations(t)
}

func TestParseAndValidateInputWithAdditionalArgument(t *testing.T) {
	var inputMap = map[string]interface{}{"name": "PVDriver", "action": "Install", "additionalArguments": "{\"SSM_var1\":\"customVal1\",\"SSM_var2\":\"customVal2\"}"}
	var rawPluginInput = interface{}(inputMap)
	input, err := parseAndValidateInput(rawPluginInput)

	assert.NoError(t, err)
	assert.Equal(t, "PVDriver", input.Name)
	assert.Equal(t, "Install", input.Action)
	assert.Empty(t, input.Version)
	assert.NotEmpty(t, input.AdditionalArguments)
	assert.Equal(t, "{\"SSM_var1\":\"customVal1\",\"SSM_var2\":\"customVal2\"}", input.AdditionalArguments)
}

func TestParseAndValidateInputWithEmptyAdditionalArgumentsMap(t *testing.T) {
	var inputMap = map[string]interface{}{"name": "PVDriver", "action": "Install", "additionalArguments": make(map[string]interface{})}
	var rawPluginInput = interface{}(inputMap)
	input, err := parseAndValidateInput(rawPluginInput)

	assert.NoError(t, err)
	assert.Equal(t, "PVDriver", input.Name)
	assert.Equal(t, "Install", input.Action)
	assert.Empty(t, input.Version)
	assert.Empty(t, input.AdditionalArguments)
}

func TestParseAndValidateInputWithEmptyAdditionalArgumentsString(t *testing.T) {
	var inputMap = map[string]interface{}{"name": "PVDriver", "action": "Install", "additionalArguments": ""}
	var rawPluginInput = interface{}(inputMap)
	input, err := parseAndValidateInput(rawPluginInput)

	assert.NoError(t, err)
	assert.Equal(t, "PVDriver", input.Name)
	assert.Equal(t, "Install", input.Action)
	assert.Empty(t, input.Version)
	assert.Empty(t, input.AdditionalArguments)
}

func TestParseAndValidateInputWithoutAdditionalArguments(t *testing.T) {
	var inputMap = map[string]interface{}{"name": "PVDriver", "action": "Install"}
	var rawPluginInput = interface{}(inputMap)
	input, err := parseAndValidateInput(rawPluginInput)

	assert.NoError(t, err)
	assert.Equal(t, "PVDriver", input.Name)
	assert.Equal(t, "Install", input.Action)
	assert.Empty(t, input.Version)
	assert.Empty(t, input.AdditionalArguments)
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

func TestValidateInputWithValidAdditionalArgumentsWithSSMPrefix(t *testing.T) {
	input := ConfigurePackagePluginInput{}
	input.Version = "1.0.0"
	input.Name = "PVDriver"
	input.Action = "InvalidAction"
	input.AdditionalArguments = "{\"SSM_var1\":\"customVal1\",\"SSM_var2\":\"customVal2\"}"

	result, err := validateInput(&input)

	assert.True(t, result)
	assert.NoError(t, err)
	assert.Equal(t, "{\"SSM_var1\":\"customVal1\",\"SSM_var2\":\"customVal2\"}", input.AdditionalArguments)
}

func TestValidateInputWithEmptyAdditionalArguments(t *testing.T) {
	input := ConfigurePackagePluginInput{}
	input.Version = "1.0.0"
	input.Name = "PVDriver"
	input.Action = "InvalidAction"
	input.AdditionalArguments = ""

	result, err := validateInput(&input)

	assert.True(t, result)
	assert.NoError(t, err)
}

func TestValidateInputWithAdditionalArgumentsIncludingEmptyKey(t *testing.T) {
	input := ConfigurePackagePluginInput{}
	input.Version = "1.0.0"
	input.Name = "PVDriver"
	input.Action = "InvalidAction"
	input.AdditionalArguments = "{\"SSM_var1\":\"customVal1\", \"\":\"customVal2\"}"

	result, err := validateInput(&input)

	assert.False(t, result)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "empty key is not allowed in additional arguments from input")
}

func TestValidateInputWithAdditionalArgumentsWithoutSSMPrefix(t *testing.T) {
	input := ConfigurePackagePluginInput{}
	input.Version = "1.0.0"
	input.Name = "PVDriver"
	input.Action = "InvalidAction"
	input.AdditionalArguments = "{\"var1\":\"customVal1\", \"var2\":\"customVal2\"}"

	result, err := validateInput(&input)

	assert.False(t, result)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "keys need to start with \"SSM_\" prefix in additional arguments from input")
}

func TestValidateInputWithInvalidAdditionalArguments(t *testing.T) {
	input := ConfigurePackagePluginInput{}
	input.Version = "1.0.0"
	input.Name = "PVDriver"
	input.Action = "InvalidAction"
	input.AdditionalArguments = "invalidArguments"

	result, err := validateInput(&input)

	assert.False(t, result)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unable to unmarshal additional arguments from input")
}

func TestSelectService(t *testing.T) {
	isDocumentArchive := false
	manifest := "manifest"
	data := []struct {
		name           string
		bwfacade       facade.BirdwatcherFacade
		expectedType   string
		packageName    string
		packageVersion string
		errorExpected  bool
	}{
		{
			"get manifest works",
			&facade.FacadeStub{
				GetManifestOutput: &ssm.GetManifestOutput{
					Manifest: &manifest,
				},
			},
			packageservice.PackageServiceName_birdwatcher,
			"package",
			"1.2.3.4",
			false,
		},
		{
			"no getManifest",
			&facade.FacadeStub{
				GetManifestError: errors.New(resourceNotFoundException),
			},
			packageservice.PackageServiceName_document,
			"package",
			"1.2.3.4",
			false,
		},
		{
			"documentArn type packaget",
			&facade.FacadeStub{},
			packageservice.PackageServiceName_document,
			"arn:aws:ssm:us-west-1:1234567890:document/package",
			"1.2.3.4",
			false,
		},
		{
			"incorrect version type document package",
			&facade.FacadeStub{},
			packageservice.PackageServiceName_document,
			"package",
			"package_latest",
			false,
		},
		{
			"correct version type birdwatcher package doing getManifest",
			&facade.FacadeStub{
				GetManifestOutput: &ssm.GetManifestOutput{
					Manifest: &manifest,
				},
			},
			packageservice.PackageServiceName_birdwatcher,
			"package",
			"packageLatest1.2",
			false,
		},
		{
			"error in getManifest",
			&facade.FacadeStub{
				GetManifestError: errors.New("testError"),
			},
			packageservice.PackageServiceName_birdwatcher,
			"package",
			"1.2.3.4",
			true,
		},
	}

	for _, testdata := range data {
		t.Run(testdata.name, func(t *testing.T) {
			tracer := trace.NewTracer(contextMock.Log())
			defer tracer.BeginSection("test").End()

			appConfig := appconfig.SsmagentConfig{
				Birdwatcher: appconfig.BirdwatcherCfg{
					ForceEnable: true,
				},
			}
			localRepo := localpackages.NewRepository()
			input := &ConfigurePackagePluginInput{
				Name:       testdata.packageName,
				Version:    testdata.packageVersion,
				Repository: "",
			}

			result, err := selectService(contextMock, tracer, input, localRepo, &appConfig, testdata.bwfacade, &isDocumentArchive)

			if !testdata.errorExpected {
				assert.Equal(t, testdata.expectedType, result.PackageServiceName())
				assert.NoError(t, err)
			} else {
				assert.Error(t, err)
			}

		})
	}
}

// Integration tests
func loadFile(t *testing.T, fileName string) (result []byte) {
	result, err := ioutil.ReadFile(fileName)
	if err != nil {
		t.Fatal(err)
	}
	return
}

// Test that checks the agent for calls made to GetManifest
func TestExecuteConfigurePackagePlugin_BirdwatcherService(t *testing.T) {

	// file stubs are needed for ensurePackage because it handles the unzip
	stubs := setSuccessStubs()
	defer stubs.Clear()
	manifest := string(loadFile(t, "testdata/sampleManifest.json"))

	pluginInformation := createStubPluginInputInstall()
	installerMock := installerSuccessMock(pluginInformation.Name, pluginInformation.Version)
	repoMock := repoInstallMock_ReadWriteManifest(pluginInformation, installerMock, pluginInformation.Version, InstallAction)
	bwFacade := facadeMock.BirdwatcherFacade{}
	getManifestInput := &ssm.GetManifestInput{
		PackageName:    &pluginInformation.Name,
		PackageVersion: &pluginInformation.Version,
	}
	getManifestOutput := &ssm.GetManifestOutput{
		Manifest: &manifest,
	}
	bwFacade.On("GetManifest", getManifestInput).Return(getManifestOutput, nil).Once()
	bwFacade.On("PutConfigurePackageResult", mock.Anything).Return(&ssm.PutConfigurePackageResultOutput{}, nil).Once()
	repoMock.On("LoadTraces", mock.Anything, mock.Anything).Return(nil)

	plugin := &Plugin{
		context:                contextMock,
		birdwatcherfacade:      &bwFacade,
		localRepository:        repoMock,
		packageServiceSelector: selectService,
	}

	// open network required
	plugin.execute(buildConfigSimple(pluginInformation), createMockCancelFlag(), createMockIOHandler())

	repoMock.AssertExpectations(t)
	installerMock.AssertExpectations(t)
	bwFacade.AssertExpectations(t)
	assert.Equal(t, false, plugin.isDocumentArchive)

}

// Test that checks the agent for calls made to GetDocument
func TestExecuteConfigurePackagePlugin_DocumentService(t *testing.T) {

	// file stubs are needed for ensurePackage because it handles the unzip
	stubs := setSuccessStubs()
	defer stubs.Clear()
	manifest := string(loadFile(t, "testdata/sampleManifest.json"))
	documentFormat := ssm.DocumentFormatJson
	documentType := ssm.DocumentTypePackage
	documentStatus := ssm.DocumentStatusActive
	packageUrl_linux64bit := "https://s3.amazon.com/testPackage/testAgent-amd64-linux-rpm.zip"
	packageUrl_linux32bit := "https://s3.amazon.com/testPackage/testAgent-386-linux-rpm.zip"
	packageUrl_windows := "https://s3.amazon.com/testPackage/testAgent-windows.zip"
	sha256 := "sha256"
	fakeHash_linux64bit := "76edf2d951825650dc0960e9e5df7c9c16d570e380248b68ac19d4cf3013ff7d"
	fakeHash_linux32bit := "7b8818d4db10a6b01ec261afe4a0b0c8178e97c33976f9aba34ac7529655e350"
	fakeHash_windows := "d05804e5065ea5286ae4a1a45ff6eef299cddd1a78f7430672655c4c75a2fe9b"
	manifestVersion := "0.0.1"
	var getDocumentOutput *ssm.GetDocumentOutput
	var getDocumentError error
	docVersion := "1"
	getDocument_DocVersion := "2"
	fakeHash := "djfhsfdse3498234bbar8821344bncdklsr023445fskdsgg"

	ec2infradetect.CollectEc2Infrastructure = func(identity.IAgentIdentity) (*ec2infradetect.Ec2Infrastructure, error) {
		return &ec2infradetect.Ec2Infrastructure{
			InstanceID:       "i-1234",
			Region:           "us-east-1",
			AvailabilityZone: "us-east-1a",
			InstanceType:     "c3.4xlarge",
		}, nil
	}

	data := []struct {
		name                    string
		mockVersion             string
		getDocumentReturnsError bool
		pluginInformation       *ConfigurePackagePluginInput
		errorResponse           string
		action                  string
	}{
		{
			"install package no version provided",
			"",
			false,
			createStubPluginInputInstallLatest(),
			"",
			InstallAction,
		},
		{
			"install package version provided",
			"0.0.1",
			false,
			createStubPluginInputInstall(),
			"",
			InstallAction,
		},
		{
			"install package not found in documents",
			"",
			true,
			createStubPluginInputInstallLatest(),
			"failed to download manifest - failed to retrieve package document: ResourceNotFoundException\n",
			InstallAction,
		},
		{
			"uninstall package not installed",
			"",
			false,
			createStubPluginInputUninstallLatest(),
			"",
			UninstallAction,
		},
	}
	for _, testdata := range data {
		t.Run(testdata.name, func(t *testing.T) {
			pluginInformation := testdata.pluginInformation
			version := pluginInformation.Version
			if packageservice.IsLatest(version) {
				version = packageservice.Latest
			}
			docDescription := ssm.DocumentDescription{
				Name:            &pluginInformation.Name,
				DocumentVersion: &docVersion,
				VersionName:     &pluginInformation.Version,
				Hash:            &fakeHash,
				Status:          &documentStatus,
			}

			installerMock := installerSuccessMock_Install(pluginInformation.Name, manifestVersion, testdata.action)
			repoMock := repoInstallMock_ReadWriteManifestHash(pluginInformation, installerMock, manifestVersion, docVersion, getDocument_DocVersion, testdata.action)
			bwFacade := facadeMock.BirdwatcherFacade{}
			mockIOHandler := createMockIOHandlerStruct(testdata.errorResponse)
			getManifestInput := &ssm.GetManifestInput{
				PackageName:    &pluginInformation.Name,
				PackageVersion: &version,
			}
			versionName := &testdata.mockVersion
			if testdata.mockVersion == "" {
				versionName = nil
			}
			describeDocumentInput := &ssm.DescribeDocumentInput{
				Name:        &pluginInformation.Name,
				VersionName: versionName,
			}
			describeDocumentOutput := &ssm.DescribeDocumentOutput{
				Document: &docDescription,
			}
			getDocumentInput := &ssm.GetDocumentInput{
				Name:        &pluginInformation.Name,
				VersionName: versionName,
			}
			if !testdata.getDocumentReturnsError {
				getDocumentOutput = &ssm.GetDocumentOutput{
					Content: &manifest,
					AttachmentsContent: []*ssm.AttachmentContent{
						{
							Name:     &pluginInformation.Name,
							Url:      &packageUrl_linux32bit,
							HashType: &sha256,
							Hash:     &fakeHash_linux32bit,
						},
						{
							Name:     &pluginInformation.Name,
							Url:      &packageUrl_linux64bit,
							HashType: &sha256,
							Hash:     &fakeHash_linux64bit,
						},
						{
							Name:     &pluginInformation.Name,
							Url:      &packageUrl_windows,
							HashType: &sha256,
							Hash:     &fakeHash_windows,
						},
					},
					DocumentFormat:  &documentFormat,
					DocumentType:    &documentType,
					DocumentVersion: &getDocument_DocVersion,
					Name:            &pluginInformation.Name,
					Status:          &documentStatus,
					VersionName:     &pluginInformation.Version,
				}
				getDocumentError = nil
			} else {
				getDocumentOutput = nil
				getDocumentError = errors.New(resourceNotFoundException)
			}
			bwFacade.On("GetManifest", getManifestInput).Return(nil, errors.New(resourceNotFoundException)).Once()
			bwFacade.On("DescribeDocument", describeDocumentInput).Return(describeDocumentOutput, nil)
			bwFacade.On("GetDocument", getDocumentInput).Return(getDocumentOutput, getDocumentError).Once()
			bwFacade.On("PutConfigurePackageResult", mock.Anything).Return(&ssm.PutConfigurePackageResultOutput{}, nil).Once()

			plugin := &Plugin{
				context:                contextMock,
				birdwatcherfacade:      &bwFacade,
				localRepository:        repoMock,
				packageServiceSelector: selectService,
			}

			plugin.execute(buildConfigSimple(pluginInformation), createMockCancelFlag(), mockIOHandler)

			if !testdata.getDocumentReturnsError {
				repoMock.AssertExpectations(t)
				installerMock.AssertExpectations(t)
				bwFacade.AssertExpectations(t)
				mockIOHandler.AssertExpectations(t)
			}
			assert.Equal(t, true, plugin.isDocumentArchive)
		})
	}
}
