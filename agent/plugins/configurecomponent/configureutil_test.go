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
	"testing"

	"github.com/aws/amazon-ssm-agent/agent/updateutil"
	"github.com/stretchr/testify/assert"
)

func TestCreateManifestName(t *testing.T) {
	pluginInformation := createStubPluginInput()

	manifestName := "PVDriver.json"
	result := createManifestName(pluginInformation.Name)

	assert.Equal(t, manifestName, result)
}

func TestCreatePackageName(t *testing.T) {
	pluginInformation := createStubPluginInput()
	context := createStubInstanceContext()

	packageName := "PVDriver-amd64.zip"
	result := createPackageName(pluginInformation.Name, context)

	assert.Equal(t, packageName, result)
}

func TestCreateS3Location(t *testing.T) {
	pluginInformation := createStubPluginInput()
	context := createStubInstanceContext()
	fileName := "PVDriver-amd64.zip"

	packageLocation := "https://amazon-ssm-us-west-2.s3.amazonaws.com/PVDriver/Windows/9000.0.0.0/PVDriver-amd64.zip"
	result := createS3Location(pluginInformation.Name, pluginInformation.Version, context, fileName)

	assert.Equal(t, packageLocation, result)
}

func TestCreateS3Location_Bjs(t *testing.T) {
	pluginInformation := createStubPluginInput()
	context := createStubInstanceContextBjs()
	fileName := "PVDriver-amd64.zip"

	packageLocation := "https://s3.cn-north-1.amazonaws.com.cn/amazon-ssm-cn-north-1/PVDriver/Windows/9000.0.0.0/PVDriver-amd64.zip"
	result := createS3Location(pluginInformation.Name, pluginInformation.Version, context, fileName)

	assert.Equal(t, packageLocation, result)
}

func TestCreateComponentFolderSucceeded(t *testing.T) {
	pluginInformation := createStubPluginInput()
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
	pluginInformation := createStubPluginInput()
	util := Utility{}

	mkDirAll = func(path string) error {
		return fmt.Errorf("Folder cannot be created")
	}

	_, err := util.CreateComponentFolder(pluginInformation.Name, pluginInformation.Version)
	assert.Error(t, err)
}

func createStubPluginInput() *ConfigureComponentPluginInput {
	input := ConfigureComponentPluginInput{}

	// Set version to a large number to avoid conflict of the actual component release version
	input.Version = "9000.0.0.0"
	input.Name = "PVDriver"
	input.Action = "Install"
	input.Source = ""

	return &input
}

func createStubInvalidPluginInput() *ConfigureComponentPluginInput {
	input := ConfigureComponentPluginInput{}

	// Set version to a large number to avoid conflict of the actual component release version
	input.Version = "9000.0.0.0"
	input.Name = "PVDriver"
	input.Action = "InvalidAction"
	input.Source = "https://amazon-ssm-us-west-2.s3.amazonaws.com/PVDriver/Windows/9000.0.0/PVDriver-amd64.zip"

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

type mockConfigureUtility struct{}

func (u *mockConfigureUtility) CreateComponentFolder(name string, version string) (folder string, err error) {
	return "", nil
}
