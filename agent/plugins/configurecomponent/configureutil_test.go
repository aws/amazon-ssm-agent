// Copyright 2016 Amazon.com, Inc. or its affiliates. All Rights Reserved.
//
// Licensed under the Amazon Software License (the "License"). You may not
// use this file except in compliance with the License. A copy of the
// License is located at
//
// http://aws.amazon.com/asl/
//
// or in the "license" file accompanying this file. This file is distributed
// on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND,
// express or implied. See the License for the specific language governing
// permissions and limitations under the License.

// Package configurecomponent implements the ConfigureComponent plugin.
package configurecomponent

import (
	"fmt"
	"testing"

	"github.com/aws/amazon-ssm-agent/agent/updateutil"
	"github.com/stretchr/testify/assert"
)

func TestCreatePackageName(t *testing.T) {
	pluginInformation := createStubPluginInput()
	context := createStubInstanceContext()

	packageName := "PVDriver-amd64.zip"
	result := createPackageName(pluginInformation.Name, context)

	assert.Equal(t, packageName, result)
}

func TestCreatePackageLocation(t *testing.T) {
	pluginInformation := createStubPluginInput()
	context := createStubInstanceContext()
	packageName := "PVDriver-amd64.zip"

	packageLocation := "https://amazon-ssm-us-west-2.s3.amazonaws.com/PVDriver/Windows/9000.0.0/PVDriver-amd64.zip"
	result := createPackageLocation(pluginInformation.Name, pluginInformation.Version, context, packageName)

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
	input.Version = "9000.0.0"
	input.Name = "PVDriver"
	input.Action = "Install"

	return &input
}

func createStubInvalidPluginInput() *ConfigureComponentPluginInput {
	input := ConfigureComponentPluginInput{}

	// Set version to a large number to avoid conflict of the actual component release version
	input.Version = "9000.0.0"
	input.Name = "PVDriver"
	input.Action = "InvalidAction"

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

type mockUtility struct{}

func (u *mockUtility) CreateComponentFolder(name string, version string) (folder string, err error) {
	return "", nil
}
