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

// Package updatessmagent implements the UpdateSsmAgent plugin.
package updatessmagent

import (
	"fmt"
	"testing"
	"time"

	"github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/aws/amazon-ssm-agent/agent/contracts"
	"github.com/aws/amazon-ssm-agent/agent/fileutil/artifact"
	"github.com/aws/amazon-ssm-agent/agent/framework/processor/executer/iohandler"
	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/amazon-ssm-agent/agent/task"
	"github.com/aws/amazon-ssm-agent/agent/updateutil"
	"github.com/aws/amazon-ssm-agent/agent/version"
	"github.com/stretchr/testify/assert"
)

var logger = log.NewMockLog()

func TestGenerateUpdateCmd(t *testing.T) {
	plugin := createStubPluginInput()
	context := createStubInstanceContext()
	manifest := createStubManifest(plugin, context, true, true)
	manager := updateManager{}

	result, err := manager.generateUpdateCmd(logger, manifest, plugin, context,
		"path", "messageID", "stdout", "stderr", "prefix", "bucket")

	assert.NoError(t, err)
	assert.Contains(t, result, "path")
	assert.Contains(t, result, "messageID")
	assert.Contains(t, result, "stdout")
	assert.Contains(t, result, "stderr")
	assert.Contains(t, result, "prefix")
	assert.Contains(t, result, "bucket")
}

func TestDownloadManifest(t *testing.T) {
	plugin := createStubPluginInput()
	context := createStubInstanceContext()

	manager := updateManager{}
	util := fakeUtility{}
	out := iohandler.DefaultIOHandler{}

	fileDownload = func(log log.T, input artifact.DownloadInput) (output artifact.DownloadOutput, err error) {
		result := artifact.DownloadOutput{}
		result.IsHashMatched = true
		result.LocalFilePath = "testdata/sampleManifest.json"
		return result, nil
	}

	manifest, err := manager.downloadManifest(logger, &util, plugin, context, &out)

	assert.NoError(t, err)
	assert.NotNil(t, manifest)
}

func TestDownloadUpdater(t *testing.T) {
	plugin := createStubPluginInput()
	context := createStubInstanceContext()
	manifest := createStubManifest(plugin, context, true, true)

	manager := updateManager{}
	util := fakeUtility{}
	out := iohandler.DefaultIOHandler{}

	fileDownload = func(log log.T, input artifact.DownloadInput) (output artifact.DownloadOutput, err error) {
		result := artifact.DownloadOutput{}
		result.IsHashMatched = true
		result.LocalFilePath = "updater/location"
		return result, nil
	}

	fileUncompress = func(log log.T, src, dest string) error {
		return nil
	}

	_, err := manager.downloadUpdater(logger, &util, plugin.AgentName, manifest, &out, context)

	assert.NoError(t, err)
}

func TestDownloadUpdater_HashDoesNotMatch(t *testing.T) {
	plugin := createStubPluginInput()
	context := createStubInstanceContext()
	manifest := createStubManifest(plugin, context, true, true)

	manager := updateManager{}
	util := fakeUtility{}
	out := iohandler.DefaultIOHandler{}

	// file download failed
	fileDownload = func(log log.T, input artifact.DownloadInput) (output artifact.DownloadOutput, err error) {
		result := artifact.DownloadOutput{}
		result.IsHashMatched = false
		result.LocalFilePath = ""
		return result, fmt.Errorf("404")
	}

	_, err := manager.downloadUpdater(logger, &util, plugin.AgentName, manifest, &out, context)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to download file reliably")
	assert.Contains(t, err.Error(), "404")
}

func TestDownloadUpdater_FailedDuringUnCompress(t *testing.T) {
	plugin := createStubPluginInput()
	context := createStubInstanceContext()
	manifest := createStubManifest(plugin, context, true, true)

	manager := updateManager{}
	util := fakeUtility{}
	out := iohandler.DefaultIOHandler{}

	fileDownload = func(log log.T, input artifact.DownloadInput) (output artifact.DownloadOutput, err error) {
		result := artifact.DownloadOutput{}
		result.IsHashMatched = true
		result.LocalFilePath = "updater/location"
		return result, nil
	}

	fileUncompress = func(log log.T, src, dest string) error {
		return fmt.Errorf("Failed with uncompress")
	}

	_, err := manager.downloadUpdater(logger, &util, plugin.AgentName, manifest, &out, context)

	assert.Error(t, err, "Failed with uncompress")
}

func TestValidateUpdate(t *testing.T) {
	plugin := createStubPluginInput()
	context := createStubInstanceContext()
	manifest := createStubManifest(plugin, context, true, true)

	manager := updateManager{}
	out := iohandler.DefaultIOHandler{}

	result, err := manager.validateUpdate(logger, plugin, context, manifest, &out)

	assert.False(t, result)
	assert.NoError(t, err)
}

func TestValidateUpdate_GetLatestTargetVersionWhenTargetVersionIsEmpty(t *testing.T) {
	plugin := createStubPluginInput()
	context := createStubInstanceContext()
	manifest := createStubManifest(plugin, context, true, true)

	manager := updateManager{}
	// Setup, update target version to empty string
	plugin.TargetVersion = ""
	out := iohandler.DefaultIOHandler{}

	result, err := manager.validateUpdate(logger, plugin, context, manifest, &out)

	assert.False(t, result)
	assert.NoError(t, err)
}

func TestValidateUpdate_TargetVersionSameAsCurrentVersion(t *testing.T) {
	plugin := createStubPluginInput()
	plugin.TargetVersion = version.Version
	context := createStubInstanceContext()
	manifest := createStubManifest(plugin, context, true, true)

	manager := updateManager{}
	out := iohandler.DefaultIOHandler{}

	noNeedToUpdate, err := manager.validateUpdate(logger, plugin, context, manifest, &out)

	assert.True(t, noNeedToUpdate)
	assert.NoError(t, err)
	assert.Contains(t, out.GetStdout(), "already been installed, update skipped")
}

func TestValidateUpdate_DowngradeVersion(t *testing.T) {
	plugin := createStubPluginInput()
	plugin.AllowDowngrade = "false"
	plugin.TargetVersion = "0.0.0.1"
	context := createStubInstanceContext()
	manifest := createStubManifest(plugin, context, true, true)

	manager := updateManager{}
	out := iohandler.DefaultIOHandler{}

	noNeedToUpdate, err := manager.validateUpdate(logger, plugin, context, manifest, &out)

	assert.True(t, noNeedToUpdate)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "please enable allow downgrade to proceed")
}

func TestValidateUpdate_TargetVersionNotSupport(t *testing.T) {
	plugin := createStubPluginInput()
	plugin.TargetVersion = "1.1.1.999"
	context := createStubInstanceContext()
	manifest := createStubManifest(plugin, context, true, false)

	manager := updateManager{}
	out := iohandler.DefaultIOHandler{}

	result, err := manager.validateUpdate(logger, plugin, context, manifest, &out)

	assert.True(t, result)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "is unsupported")
}

func TestValidateUpdate_UnsupportedCurrentVersion(t *testing.T) {
	plugin := createStubPluginInput()
	context := createStubInstanceContext()
	manifest := createStubManifest(plugin, context, false, true)

	manager := updateManager{}
	out := iohandler.DefaultIOHandler{}
	result, err := manager.validateUpdate(logger, plugin, context, manifest, &out)

	if version.Version != updateutil.PipelineTestVersion {
		assert.True(t, result)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "is unsupported on current platform")
	}
}

func TestUpdateAgent_InvalidPluginRaw(t *testing.T) {
	config := contracts.Configuration{}
	plugin := &Plugin{}

	mockCancelFlag := new(task.MockCancelFlag)

	manager := &updateManager{}
	util := &fakeUtility{}
	rawPluginInput := "invalid value" // string value will failed the Remarshal as it's not PluginInput
	out := iohandler.DefaultIOHandler{}
	updateAgent(plugin, config, logger, manager, util, rawPluginInput, mockCancelFlag, &out, time.Now())

	assert.Contains(t, out.GetStderr(), "invalid format in plugin properties")
}

func TestUpdateAgent(t *testing.T) {
	pluginInput := createStubPluginInput()
	pluginInput.TargetVersion = ""
	context := createStubInstanceContext()
	manifest := createStubManifest(pluginInput, context, true, true)
	config := contracts.Configuration{}
	plugin := &Plugin{}

	testCases := []fakeUpdateManager{
		{
			generateUpdateCmdResult: "-updater -message id value",
			generateUpdateCmdError:  nil,
			downloadManifestResult:  manifest,
			downloadManifestError:   nil,
			downloadUpdaterResult:   "updater",
			downloadUpdaterError:    nil,
			validateUpdateResult:    false,
			validateUpdateError:     nil,
		},
	}

	pluginInput.TargetVersion = ""
	mockCancelFlag := new(task.MockCancelFlag)
	util := fakeUtility{}

	for _, manager := range testCases {
		out := iohandler.DefaultIOHandler{}
		updateAgent(plugin, config, logger, &manager, &util, pluginInput, mockCancelFlag, &out, time.Now())
		assert.Empty(t, out.GetStderr())
	}
}

func TestUpdateAgent_NegativeTestCases(t *testing.T) {
	pluginInput := createStubPluginInput()
	pluginInput.TargetVersion = ""
	context := createStubInstanceContext()
	manifest := createStubManifest(pluginInput, context, true, true)
	config := contracts.Configuration{}
	plugin := &Plugin{}

	testCases := []fakeUpdateManager{
		{
			generateUpdateCmdResult: "",
			generateUpdateCmdError:  fmt.Errorf("Cannot generate command"),
		},
		{
			generateUpdateCmdResult: "-updater -message id value",
			generateUpdateCmdError:  nil,
			downloadManifestResult:  manifest,
			downloadManifestError:   fmt.Errorf("Cannot generate manifest"),
		},
		{
			generateUpdateCmdResult: "-updater -message id value",
			generateUpdateCmdError:  nil,
			downloadManifestResult:  manifest,
			downloadManifestError:   nil,
			downloadUpdaterResult:   "",
			downloadUpdaterError:    fmt.Errorf("Cannot loadload updater"),
		},
		{
			generateUpdateCmdResult: "-updater -message id value",
			generateUpdateCmdError:  nil,
			downloadManifestResult:  manifest,
			downloadManifestError:   nil,
			downloadUpdaterResult:   "updater",
			downloadUpdaterError:    nil,
			validateUpdateResult:    true,
			validateUpdateError:     fmt.Errorf("Invalid download"),
		},
	}

	pluginInput.TargetVersion = ""
	mockCancelFlag := new(task.MockCancelFlag)
	util := fakeUtility{}

	for _, manager := range testCases {
		out := iohandler.DefaultIOHandler{}
		updateAgent(plugin, config, logger, &manager, &util, pluginInput, mockCancelFlag, &out, time.Now())
		assert.NotEmpty(t, out.GetStderr())
	}
}

func TestExecute(t *testing.T) {
	pluginInput := createStubPluginInput()
	pluginInput.TargetVersion = ""
	config := contracts.Configuration{}
	p := make([]interface{}, 1)
	p[0] = pluginInput
	config.Properties = p
	plugin := &Plugin{}

	pluginInput.TargetVersion = ""
	mockCancelFlag := new(task.MockCancelFlag)
	mockContext := context.NewMockDefault()
	mockIOHandler := iohandler.DefaultIOHandler{}

	// Create stub
	updateAgent = func(
		p *Plugin,
		config contracts.Configuration,
		log log.T,
		manager pluginHelper,
		util updateutil.T,
		rawPluginInput interface{},
		cancelFlag task.CancelFlag,
		output iohandler.IOHandler,
		startTime time.Time) {
		return
	}

	// Setup mocks
	mockCancelFlag.On("Canceled").Return(false)
	mockCancelFlag.On("ShutDown").Return(false)
	mockCancelFlag.On("Wait").Return(false).After(100 * time.Millisecond)

	plugin.Execute(mockContext, config, mockCancelFlag, &mockIOHandler)
}

func createStubPluginInput() *UpdatePluginInput {
	input := UpdatePluginInput{}

	// Set target version to a large number to avoid the conflict of the actual agent release version
	input.TargetVersion = "9000.0.0.0"
	input.AgentName = "amazon-ssm-agent"
	input.AllowDowngrade = "true"
	return &input
}

func createStubManifest(plugin *UpdatePluginInput,
	context *updateutil.InstanceContext,
	addCurrentVersion bool,
	addTargetVersion bool) *Manifest {
	manifest, _ := ParseManifest(logger, "testdata/sampleManifest.json", context, plugin.AgentName)

	for _, p := range manifest.Packages {
		if p.Name == plugin.AgentName {
			for _, f := range p.Files {
				if f.Name == context.FileName(plugin.AgentName) {
					if addCurrentVersion {
						f.AvailableVersions = append(f.AvailableVersions,
							&PackageVersion{Version: version.Version})
					}
					if addTargetVersion {
						f.AvailableVersions = append(f.AvailableVersions,
							&PackageVersion{Version: plugin.TargetVersion})
					}

				}
			}
		}
	}
	return manifest
}

func createStubInstanceContext() *updateutil.InstanceContext {
	context := updateutil.InstanceContext{}
	context.Arch = "amd64"
	context.CompressFormat = "tar.gz"
	context.InstallerName = updateutil.PlatformLinux
	context.Platform = updateutil.PlatformLinux
	context.PlatformVersion = "2015.9"
	return &context
}

type fakeUtility struct{}

func (u *fakeUtility) CreateInstanceContext(log log.T) (context *updateutil.InstanceContext, err error) {
	return createStubInstanceContext(), nil
}

func (u *fakeUtility) IsServiceRunning(log log.T, i *updateutil.InstanceContext) (result bool, err error) {
	return true, nil
}

func (u *fakeUtility) WaitForServiceToStart(log log.T, i *updateutil.InstanceContext) (result bool, err error) {
	return true, nil
}

func (u *fakeUtility) CreateUpdateDownloadFolder() (folder string, err error) {
	return "", nil
}

func (u *fakeUtility) ExeCommand(
	log log.T,
	cmd string,
	updateRoot string,
	workingDir string,
	stdOut string,
	stdErr string,
	isAsync bool) (err error) {
	return nil
}

func (u *fakeUtility) SaveUpdatePluginResult(
	log log.T,
	updateRoot string,
	updateResult *updateutil.UpdatePluginResult) (err error) {
	return nil
}

func (u *fakeUtility) IsDiskSpaceSufficientForUpdate(log log.T) (bool, error) {
	return true, nil
}

type fakeUpdateManager struct {
	generateUpdateCmdResult string
	generateUpdateCmdError  error
	downloadManifestResult  *Manifest
	downloadManifestError   error
	downloadUpdaterResult   string
	downloadUpdaterError    error
	validateUpdateResult    bool
	validateUpdateError     error
}

func (u *fakeUpdateManager) generateUpdateCmd(log log.T,
	manifest *Manifest,
	pluginInput *UpdatePluginInput,
	context *updateutil.InstanceContext,
	updaterPath string,
	messageID string,
	stdout string,
	stderr string,
	keyPrefix string,
	bucketName string) (cmd string, err error) {

	return u.generateUpdateCmdResult, u.generateUpdateCmdError
}

func (u *fakeUpdateManager) downloadManifest(log log.T,
	util updateutil.T,
	pluginInput *UpdatePluginInput,
	context *updateutil.InstanceContext,
	out iohandler.IOHandler) (manifest *Manifest, err error) {
	return u.downloadManifestResult, u.downloadManifestError
}

func (u *fakeUpdateManager) downloadUpdater(log log.T,
	util updateutil.T,
	updaterPackageName string,
	manifest *Manifest,
	out iohandler.IOHandler,
	context *updateutil.InstanceContext) (version string, err error) {
	return u.downloadUpdaterResult, u.downloadUpdaterError
}

func (u *fakeUpdateManager) validateUpdate(log log.T,
	pluginInput *UpdatePluginInput,
	context *updateutil.InstanceContext,
	manifest *Manifest,
	out iohandler.IOHandler) (noNeedToUpdate bool, err error) {
	return u.validateUpdateResult, u.validateUpdateError
}
