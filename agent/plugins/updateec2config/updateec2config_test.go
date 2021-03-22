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

// +build windows

// Package updateec2config implements the UpdateEC2Config plugin.
package updateec2config

import (
	"testing"

	"github.com/aws/amazon-ssm-agent/agent/framework/processor/executer/iohandler"
	"github.com/aws/amazon-ssm-agent/agent/log"
	updateinfomocks "github.com/aws/amazon-ssm-agent/agent/updateutil/updateinfo/mocks"
	"github.com/stretchr/testify/assert"
)

//mock log for testing
var logger = log.NewMockLog()

//TestGenerateUpdateCmd tests the function generateUpdateCmd
func TestGenerateUpdateCmd(t *testing.T) {
	manager := updateManager{}

	result, err := manager.generateUpdateCmd(logger, "path")

	assert.NoError(t, err)
	assert.Contains(t, result, "path")
	assert.Contains(t, result, "history")
}

//TestValidateUpdate tests the function validateUpdate
func TestValidateUpdate(t *testing.T) {
	plugin := createStubPluginInput()
	info := &updateinfomocks.T{}
	manifest := createStubManifest()

	manager := updateManager{}
	out := iohandler.DefaultIOHandler{}
	fakeVersion := "1.0.0"

	result, err := manager.validateUpdate(logger, plugin, info, manifest, &out, fakeVersion)

	assert.False(t, result)
	assert.NoError(t, err)
}

//TestValidateUpdate_GetLatestTargetVersionWhenTargetVersionIsEmpty tests negative case
func TestValidateUpdate_GetLatestTargetVersionWhenTargetVersionIsEmpty(t *testing.T) {
	plugin := createStubPluginInput()
	info := &updateinfomocks.T{}
	manifest := createStubManifest()

	manager := updateManager{}
	// Setup, update target version to empty string
	plugin.TargetVersion = ""
	out := iohandler.DefaultIOHandler{}
	fakeVersion := "1.0.0"

	result, err := manager.validateUpdate(logger, plugin, info, manifest, &out, fakeVersion)

	assert.False(t, result)
	assert.NoError(t, err)
}

//TestValidateUpdate_DowngradeVersion tests negative case
func TestValidateUpdate_DowngradeVersion(t *testing.T) {
	plugin := createStubPluginInput()
	plugin.AllowDowngrade = "false"
	plugin.TargetVersion = "1.0.0"
	fakeVersion := "999.0.0"
	info := &updateinfomocks.T{}
	manifest := createStubManifest()

	manager := updateManager{}
	out := iohandler.DefaultIOHandler{}

	noNeedToUpdate, err := manager.validateUpdate(logger, plugin, info, manifest, &out, fakeVersion)

	assert.True(t, noNeedToUpdate)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "please enable allow downgrade to proceed")
}

//TestValidateUpdate_UnsupportedTargetVersion tests negative case
func TestValidateUpdate_UnsupportedTargetVersion(t *testing.T) {
	plugin := createStubPluginInput()
	plugin.TargetVersion = "1.2.3"
	fakeVersion := "1.0.0"
	info := &updateinfomocks.T{}
	manifest := createStubManifest()

	manager := updateManager{}
	out := iohandler.DefaultIOHandler{}

	result, err := manager.validateUpdate(logger, plugin, info, manifest, &out, fakeVersion)

	assert.True(t, result)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "is unsupported")
}

//TestValidateUpdate_TargetVersionSameAsCurrentVersion tests invalid case
func TestValidateUpdate_TargetVersionSameAsCurrentVersion(t *testing.T) {
	plugin := createStubPluginInput()
	//plugin.TargetVersion = fakeAgentVersion
	info := &updateinfomocks.T{}
	manifest := createStubManifest()

	manager := updateManager{}
	out := iohandler.DefaultIOHandler{}

	noNeedToUpdate, err := manager.validateUpdate(logger, plugin, info, manifest, &out, plugin.TargetVersion)

	assert.True(t, noNeedToUpdate)
	assert.NoError(t, err)
	assert.Contains(t, out.GetStdout(), "already been installed, update skipped")
}

//TestValidateUpdate_UnsupportedCurrentVersion tests negative case
func TestValidateUpdate_UnsupportedCurrentVersion(t *testing.T) {
	plugin := createStubPluginInput()
	info := &updateinfomocks.T{}
	manifest := createStubManifest()

	manager := updateManager{}
	out := iohandler.DefaultIOHandler{}
	result, err := manager.validateUpdate(logger, plugin, info, manifest, &out, "1.2.3.4")

	assert.True(t, result)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "is unsupported on current platform")
}

//createStubPluginInput is a helper function to create a stub plugin for testing
func createStubPluginInput() *UpdatePluginInput {
	input := new(UpdatePluginInput)

	// Set target version to a large number to avoid the conflict of the actual agent release version
	input.TargetVersion = "999.0.0"
	input.AgentName = EC2ConfigAgentName
	input.AllowDowngrade = "false"
	return input
}

//createStubManifest is a helper function to create a stub manifest for testing
func createStubManifest() *Manifest {
	manifest := &Manifest{}
	manifest, _ = ParseManifest(logger, "testData/testManifest.json")
	return manifest
}
