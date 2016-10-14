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

// Package configurecomponent implements the ConfigureComponent plugin.
package configurecomponent

import (
	"fmt"
	"strings"
	"testing"

	"github.com/aws/amazon-ssm-agent/agent/updateutil"
	"github.com/stretchr/testify/assert"
)

func TestCreateManifestName(t *testing.T) {
	pluginInformation := createStubPluginInputInstall()

	manifestName := "PVDriver.json"
	result := getManifestName(pluginInformation.Name)

	assert.Equal(t, manifestName, result)
}

func TestCreatePackageName(t *testing.T) {
	pluginInformation := createStubPluginInputInstall()
	context := createStubInstanceContext()

	packageName := "PVDriver.zip"
	result := getPackageName(pluginInformation.Name, context)

	assert.Equal(t, packageName, result)
}

func TestCreateS3Location(t *testing.T) {
	pluginInformation := createStubPluginInputInstall()
	context := createStubInstanceContext()
	fileName := "PVDriver.zip"

	packageLocation := fmt.Sprintf("%v/PVDriver/Windows/amd64/9000.0.0/PVDriver.zip", strings.Replace(ComponentUrl, updateutil.RegionHolder, "us-west-2", -1))
	result := getS3Location(pluginInformation.Name, pluginInformation.Version, context, fileName)

	assert.Equal(t, packageLocation, result)
}

func TestCreateS3Location_Bjs(t *testing.T) {
	pluginInformation := createStubPluginInputInstall()
	context := createStubInstanceContextBjs()
	fileName := "PVDriver.zip"

	packageLocation := "https://s3.cn-north-1.amazonaws.com.cn/amazon-ssm-cn-north-1/Components/PVDriver/Windows/amd64/9000.0.0/PVDriver.zip"
	result := getS3Location(pluginInformation.Name, pluginInformation.Version, context, fileName)

	assert.Equal(t, packageLocation, result)
}

func TestCreateComponentFolderSucceeded(t *testing.T) {
	pluginInformation := createStubPluginInputInstall()
	util := Utility{}

	mkDirAll = func(path string) error {
		return nil
	}

	result, _ := util.CreateComponentFolder(pluginInformation.Name, pluginInformation.Version)

	assert.Contains(t, result, "components")
	assert.Contains(t, result, pluginInformation.Name)
	assert.Contains(t, result, pluginInformation.Version)
}

func TestCreateComponentFolderFailed(t *testing.T) {
	pluginInformation := createStubPluginInputInstall()
	util := Utility{}

	mkDirAll = func(path string) error {
		return fmt.Errorf("Folder cannot be created")
	}

	_, err := util.CreateComponentFolder(pluginInformation.Name, pluginInformation.Version)
	assert.Error(t, err)
}

func TestNeedUpdate(t *testing.T) {
	pluginInformation := createStubPluginInputInstall()
	util := Utility{}

	componentExists = func(filepath string) bool {
		return true
	}

	versionExists = func(filepath string) bool {
		return false
	}

	result := util.NeedUpdate(pluginInformation.Name, pluginInformation.Version)
	assert.Equal(t, true, result)
}

func TestNeedUpdate_NoComponentExists(t *testing.T) {
	pluginInformation := createStubPluginInputInstall()
	util := Utility{}

	componentExists = func(filepath string) bool {
		return false
	}

	result := util.NeedUpdate(pluginInformation.Name, pluginInformation.Version)
	assert.Equal(t, false, result)
}

func TestNeedUpdate_VersionExists(t *testing.T) {
	pluginInformation := createStubPluginInputInstall()
	util := Utility{}

	versionExists = func(filepath string) bool {
		return true
	}

	result := util.NeedUpdate(pluginInformation.Name, pluginInformation.Version)
	assert.Equal(t, false, result)
}

func TestGetLatestVersion_NumericSort(t *testing.T) {
	versions := [3]string{"1.0.0", "2.0.0", "10.0.0"}
	latest := getLatestVersion(versions[:], "")
	assert.Equal(t, "10.0.0", latest)
}

func TestGetLatestVersion_OnlyOneValid(t *testing.T) {
	versions := [3]string{"0.0.0", "1.0", "1.0.0.0"}
	latest := getLatestVersion(versions[:], "")
	assert.Equal(t, "0.0.0", latest)
}

func TestGetLatestVersion_NoneValid(t *testing.T) {
	versions := [3]string{"Foo", "1.0", "1.0.0.0"}
	latest := getLatestVersion(versions[:], "")
	assert.Equal(t, "", latest)
}

func TestGetLatestVersion_None(t *testing.T) {
	versions := make([]string, 0)
	latest := getLatestVersion(versions[:], "")
	assert.Equal(t, "", latest)
}

func createStubPluginInputInstall() *ConfigureComponentPluginInput {
	input := ConfigureComponentPluginInput{}

	// Set version to a large number to avoid conflict of the actual component release version
	input.Version = "9000.0.0"
	input.Name = "PVDriver"
	input.Action = "Install"
	input.Source = ""

	return &input
}

func createStubPluginInputInstallLatest() *ConfigureComponentPluginInput {
	input := ConfigureComponentPluginInput{}

	// Set version to a large number to avoid conflict of the actual component release version
	input.Name = "PVDriver"
	input.Action = "Install"
	input.Source = ""

	return &input
}

func createStubPluginInputUninstall() *ConfigureComponentPluginInput {
	input := ConfigureComponentPluginInput{}

	// Set version to a large number to avoid conflict of the actual component release version
	input.Version = "9000.0.0"
	input.Name = "PVDriver"
	input.Action = "Uninstall"
	input.Source = ""

	return &input
}

func createStubPluginInputUninstallLatest() *ConfigureComponentPluginInput {
	input := ConfigureComponentPluginInput{}

	// Set version to a large number to avoid conflict of the actual component release version
	input.Name = "PVDriver"
	input.Action = "Uninstall"
	input.Source = ""

	return &input
}

func createStubInvalidPluginInput() *ConfigureComponentPluginInput {
	input := ConfigureComponentPluginInput{}

	// Set version to a large number to avoid conflict of the actual component release version
	input.Version = "7.2"
	input.Name = ""
	input.Action = "InvalidAction"
	input.Source = "https://amazon-ssm-us-west-2.s3.amazonaws.com/Components/PVDriver/Windows/amd64/9000.0.0/PVDriver.zip"

	return &input
}

func createStubInstanceContext() *updateutil.InstanceContext {
	context := updateutil.InstanceContext{}

	context.Region = "us-west-2"
	context.Platform = "Windows"
	context.PlatformVersion = "2015.9"
	context.InstallerName = "Windows"
	context.Arch = "amd64"
	context.CompressFormat = "zip"

	return &context
}

func createStubInstanceContextBjs() *updateutil.InstanceContext {
	context := updateutil.InstanceContext{}

	context.Region = "cn-north-1"
	context.Platform = "Windows"
	context.PlatformVersion = "2015.9"
	context.InstallerName = "Windows"
	context.Arch = "amd64"
	context.CompressFormat = "zip"

	return &context
}

type mockConfigureUtility struct {
	componentFolder            string
	createComponentFolderError error
	currentVersion             string
	latestVersion              string
	getLatestVersionError      error
}

func (u *mockConfigureUtility) CreateComponentFolder(name string, version string) (folder string, err error) {
	return u.componentFolder, u.createComponentFolderError
}

func (u *mockConfigureUtility) NeedUpdate(name string, requestedVersion string) (update bool) {
	return false
}

func (u *mockConfigureUtility) HasValidPackage(name string, version string) bool {
	return true
}

func (u *mockConfigureUtility) GetCurrentVersion(name string) (installedVersion string) {
	return u.currentVersion
}

func (u *mockConfigureUtility) GetLatestVersion(name string, source string, context *updateutil.InstanceContext) (latestVersion string, err error) {
	return u.latestVersion, u.getLatestVersionError
}
