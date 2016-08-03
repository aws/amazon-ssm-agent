// Package configurecomponent implements the ConfigureComponent plugin.
package configurecomponent

import (
	"fmt"
	"os"
	"testing"

	"github.com/aws/amazon-ssm-agent/agent/updateutil"
	"github.com/stretchr/testify/assert"
)

func TestCreatePackageName(t *testing.T) {
	pluginInformation := createStubPluginInput()

	packageName := "PVDriver-amd64.zip"
	result := CreatePackageName(pluginInformation)

	assert.Equal(t, packageName, result)
}

func TestCreatePackageLocation(t *testing.T) {
	pluginInformation := createStubPluginInput()
	context := createStubInstanceContext()
	packageName := "PVDriver-amd64.zip"

	packageLocation := "https://amazon-ssm-us-west-2.s3.amazonaws.com/PVDriver/Windows/9000.0.0/PVDriver-amd64.zip"
	result := CreatePackageLocation(pluginInformation, context, packageName)

	assert.Equal(t, packageLocation, result)
}

func TestCreateComponentFolderSucceeded(t *testing.T) {
	pluginInformation := createStubPluginInput()
	util := Utility{}

	mkDirAll = func(path string, perm os.FileMode) error {
		return nil
	}

	result, _ := util.CreateComponentFolder(pluginInformation)

	assert.Contains(t, result, "components")
	assert.Contains(t, result, pluginInformation.Name)
	assert.Contains(t, result, pluginInformation.Version)
}

func TestCreateComponentFolderFailed(t *testing.T) {
	pluginInformation := createStubPluginInput()
	util := Utility{}

	mkDirAll = func(path string, perm os.FileMode) error {
		return fmt.Errorf("Folder cannot be created")
	}

	_, err := util.CreateComponentFolder(pluginInformation)
	assert.Error(t, err)
}

func createStubPluginInput() *ConfigureComponentPluginInput {
	input := ConfigureComponentPluginInput{}

	// Set version to a large number to avoid conflict of the actual component release version
	input.Version = "9000.0.0"
	input.Name = "PVDriver"
	input.Platform = "Windows"
	input.Architecture = "amd64"
	input.Action = "Install"

	return &input
}

func createStubInvalidPluginInput() *ConfigureComponentPluginInput {
	input := ConfigureComponentPluginInput{}

	// Set version to a large number to avoid conflict of the actual component release version
	input.Version = "9000.0.0"
	input.Name = "PVDriver"
	input.Platform = "InvalidPlatform"
	input.Architecture = "InvalidArchitecture"
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

func (u *mockUtility) CreateComponentFolder(input *ConfigureComponentPluginInput) (folder string, err error) {
	return "", nil
}
