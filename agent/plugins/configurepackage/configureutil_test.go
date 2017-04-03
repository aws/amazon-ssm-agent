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

	"github.com/aws/amazon-ssm-agent/agent/appconfig"
	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/stretchr/testify/assert"
)

func TestGetS3Location(t *testing.T) {
	pluginInformation := createStubPluginInputInstall()
	util := NewUtil(createStubInstanceContext(), "")

	packageLocation := "https://s3.us-west-2.amazonaws.com/amazon-ssm-packages-us-west-2/Packages/PVDriver/" + appconfig.PackagePlatform + "/amd64/1.0.0/PVDriver.zip"
	result := util.GetS3Location(pluginInformation.Name, pluginInformation.Version)

	assert.Equal(t, packageLocation, result)
}

func TestGetS3Location_Bjs(t *testing.T) {
	pluginInformation := createStubPluginInputInstall()
	util := NewUtil(createStubInstanceContextBjs(), "")

	packageLocation := "https://s3.cn-north-1.amazonaws.com.cn/amazon-ssm-packages-cn-north-1/Packages/PVDriver/" + appconfig.PackagePlatform + "/amd64/1.0.0/PVDriver.zip"
	result := util.GetS3Location(pluginInformation.Name, pluginInformation.Version)

	assert.Equal(t, packageLocation, result)
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

func createStubPluginInputInstall() *ConfigurePackagePluginInput {
	input := ConfigurePackagePluginInput{}

	input.Version = "1.0.0"
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

func createStubPluginInputUninstall() *ConfigurePackagePluginInput {
	input := ConfigurePackagePluginInput{}

	input.Version = "1.0.0"
	input.Name = "PVDriver"
	input.Action = "Uninstall"

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

func createStubPluginInputFoo() *ConfigurePackagePluginInput {
	input := ConfigurePackagePluginInput{}

	input.Version = "1.0.0"
	input.Name = "PVDriver"
	input.Action = "Foo"

	return &input
}

type mockConfigureUtility struct {
	packageFolder            string
	createPackageFolderError error
	latestVersion            string
	getLatestVersionError    error
	s3Location               string
}

func (u *mockConfigureUtility) GetLatestVersion(log log.T, name string) (latestVersion string, err error) {
	return u.latestVersion, u.getLatestVersionError
}

func (u *mockConfigureUtility) GetS3Location(packageName string, version string) (s3Location string) {
	return u.s3Location
}
