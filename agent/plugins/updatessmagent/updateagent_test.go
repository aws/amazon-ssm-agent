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
	"github.com/aws/amazon-ssm-agent/agent/updateutil/updateconstants"
	"github.com/aws/amazon-ssm-agent/agent/version"
	"github.com/aws/amazon-ssm-agent/core/executor"
	executormocks "github.com/aws/amazon-ssm-agent/core/executor/mocks"
	"github.com/nightlyone/lockfile"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

var logger = log.NewMockLog()

func TestGenerateUpdateCmdWithV2(t *testing.T) {
	plugin := createStubPluginInput()
	plugin.Source = "testSource"
	info := createStubInstanceInfo()
	manifest := createStubManifest(plugin, info, true, true)
	manager := updateManager{}

	result, err := manager.generateUpdateCmd(logger, manifest, plugin, info,
		"2.0.0.0", "messageID", "stdout", "stderr", "prefix", "bucket")

	assert.NoError(t, err)
	assert.Contains(t, result, "2.0.0.0")
	assert.Contains(t, result, "messageID")
	assert.Contains(t, result, "stdout")
	assert.Contains(t, result, "stderr")
	assert.Contains(t, result, "prefix")
	assert.Contains(t, result, "bucket")
	assert.NotContains(t, result, "manifest")
}

func TestGenerateUpdateCmdWithV3(t *testing.T) {
	plugin := createStubPluginInput()
	plugin.Source = "testSource"
	info := createStubInstanceInfo()
	manifest := createStubManifest(plugin, info, true, true)
	manager := updateManager{}

	result, err := manager.generateUpdateCmd(logger, manifest, plugin, info,
		"3.0.0.0", "messageID", "stdout", "stderr", "prefix", "bucket")

	assert.NoError(t, err)
	assert.Contains(t, result, "3.0.0.0")
	assert.Contains(t, result, "messageID")
	assert.Contains(t, result, "stdout")
	assert.Contains(t, result, "stderr")
	assert.Contains(t, result, "prefix")
	assert.Contains(t, result, "bucket")
	assert.Contains(t, result, "manifest")
}

func TestDownloadManifest(t *testing.T) {
	plugin := createStubPluginInput()
	info := createStubInstanceInfo()

	manager := updateManager{}
	util := fakeUtility{}
	out := iohandler.DefaultIOHandler{}

	fileDownload = func(context context.T, input artifact.DownloadInput) (output artifact.DownloadOutput, err error) {
		result := artifact.DownloadOutput{}
		result.IsHashMatched = true
		result.LocalFilePath = "testdata/sampleManifest.json"
		return result, nil
	}

	mockContext := context.NewMockDefault()
	manifest, err := manager.downloadManifest(mockContext, &util, plugin, info, &out)

	assert.NoError(t, err)
	assert.NotNil(t, manifest)
}

func TestDownloadUpdater(t *testing.T) {
	plugin := createStubPluginInput()
	info := createStubInstanceInfo()
	manifest := createStubManifest(plugin, info, true, true)

	manager := updateManager{}
	util := fakeUtility{}
	out := iohandler.DefaultIOHandler{}

	fileDownload = func(context context.T, input artifact.DownloadInput) (output artifact.DownloadOutput, err error) {
		result := artifact.DownloadOutput{}
		result.IsHashMatched = true
		result.LocalFilePath = "updater/location"
		return result, nil
	}

	fileUncompress = func(log log.T, src, dest string) error {
		return nil
	}

	mockContext := context.NewMockDefault()
	_, err := manager.downloadUpdater(mockContext, &util, plugin.AgentName, manifest, &out, info)

	assert.NoError(t, err)
}

func TestDownloadUpdater_HashDoesNotMatch(t *testing.T) {
	plugin := createStubPluginInput()
	info := createStubInstanceInfo()
	manifest := createStubManifest(plugin, info, true, true)

	manager := updateManager{}
	util := fakeUtility{}
	out := iohandler.DefaultIOHandler{}

	// file download failed
	fileDownload = func(context context.T, input artifact.DownloadInput) (output artifact.DownloadOutput, err error) {
		result := artifact.DownloadOutput{}
		result.IsHashMatched = false
		result.LocalFilePath = ""
		return result, fmt.Errorf("404")
	}
	mockContext := context.NewMockDefault()

	_, err := manager.downloadUpdater(mockContext, &util, plugin.AgentName, manifest, &out, info)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to download file reliably")
	assert.Contains(t, err.Error(), "404")
}

func TestDownloadUpdater_FailedDuringUnCompress(t *testing.T) {
	plugin := createStubPluginInput()
	info := createStubInstanceInfo()
	manifest := createStubManifest(plugin, info, true, true)

	manager := updateManager{}
	util := fakeUtility{}
	out := iohandler.DefaultIOHandler{}

	fileDownload = func(context context.T, input artifact.DownloadInput) (output artifact.DownloadOutput, err error) {
		result := artifact.DownloadOutput{}
		result.IsHashMatched = true
		result.LocalFilePath = "updater/location"
		return result, nil
	}

	fileUncompress = func(log log.T, src, dest string) error {
		return fmt.Errorf("Failed with uncompress")
	}
	mockContext := context.NewMockDefault()

	_, err := manager.downloadUpdater(mockContext, &util, plugin.AgentName, manifest, &out, info)

	assert.Error(t, err, "Failed with uncompress")
}

func TestValidateUpdate(t *testing.T) {
	plugin := createStubPluginInput()
	info := createStubInstanceInfo()
	manifest := createStubManifest(plugin, info, true, true)

	manager := updateManager{}
	out := iohandler.DefaultIOHandler{}

	result, err := manager.validateUpdate(logger, plugin, info, manifest, &out)

	assert.False(t, result)
	assert.NoError(t, err)
}

func TestValidateUpdate_GetLatestTargetVersionWhenTargetVersionIsEmpty(t *testing.T) {
	plugin := createStubPluginInput()
	info := createStubInstanceInfo()
	manifest := createStubManifest(plugin, info, true, true)

	manager := updateManager{}
	// Setup, update target version to empty string
	plugin.TargetVersion = ""
	out := iohandler.DefaultIOHandler{}

	result, err := manager.validateUpdate(logger, plugin, info, manifest, &out)

	assert.False(t, result)
	assert.NoError(t, err)
}

func TestValidateUpdate_TargetVersionSameAsCurrentVersion(t *testing.T) {
	plugin := createStubPluginInput()
	plugin.TargetVersion = version.Version
	info := createStubInstanceInfo()
	manifest := createStubManifest(plugin, info, true, true)

	manager := updateManager{}
	out := iohandler.DefaultIOHandler{}

	noNeedToUpdate, err := manager.validateUpdate(logger, plugin, info, manifest, &out)

	assert.True(t, noNeedToUpdate)
	assert.NoError(t, err)
	assert.Contains(t, out.GetStdout(), "already been installed, update skipped")
}

func TestValidateUpdate_DowngradeVersion(t *testing.T) {
	plugin := createStubPluginInput()
	plugin.AllowDowngrade = "false"
	plugin.TargetVersion = "0.0.0.1"
	info := createStubInstanceInfo()
	manifest := createStubManifest(plugin, info, true, true)

	manager := updateManager{}
	out := iohandler.DefaultIOHandler{}

	noNeedToUpdate, err := manager.validateUpdate(logger, plugin, info, manifest, &out)

	assert.True(t, noNeedToUpdate)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "please enable allow downgrade to proceed")
}

func TestValidateUpdate_TargetVersionNotSupport(t *testing.T) {
	plugin := createStubPluginInput()
	plugin.TargetVersion = "1.1.1.999"
	info := createStubInstanceInfo()
	manifest := createStubManifest(plugin, info, true, false)

	manager := updateManager{}
	out := iohandler.DefaultIOHandler{}

	result, err := manager.validateUpdate(logger, plugin, info, manifest, &out)

	assert.True(t, result)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "is unsupported")
}

func TestValidateUpdate_UnsupportedCurrentVersion(t *testing.T) {
	plugin := createStubPluginInput()
	info := createStubInstanceInfo()
	manifest := createStubManifest(plugin, info, false, true)

	manager := updateManager{}
	out := iohandler.DefaultIOHandler{}
	result, err := manager.validateUpdate(logger, plugin, info, manifest, &out)

	if version.Version != updateconstants.PipelineTestVersion {
		assert.True(t, result)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "is unsupported on current platform")
	}
}

func TestUpdateAgent_InvalidPluginRaw(t *testing.T) {
	config := contracts.Configuration{}
	plugin := &Plugin{
		Context: context.NewMockDefault(),
	}

	mockCancelFlag := new(task.MockCancelFlag)

	manager := &updateManager{}
	util := &fakeUtility{}
	rawPluginInput := "invalid value" // string value will failed the Remarshal as it's not PluginInput
	out := iohandler.DefaultIOHandler{}
	execMock := &executormocks.IExecutor{}

	updateAgent(plugin, config, logger, manager, util, rawPluginInput, mockCancelFlag, &out, time.Now(), execMock)

	assert.Contains(t, out.GetStderr(), "invalid format in plugin properties")
}

func TestUpdateAgent_UpdaterRetry(t *testing.T) {
	config := contracts.Configuration{}
	plugin := &Plugin{
		Context: context.NewMockDefault(),
	}
	mockCancelFlag := new(task.MockCancelFlag)
	manager := &fakeUpdateManager{}
	util := &fakeUtility{pid: -1, execCommandError: fmt.Errorf("test")}
	out := iohandler.DefaultIOHandler{}
	pluginInput := createStubPluginInput()
	pluginInput.TargetVersion = ""
	execMock := &executormocks.IExecutor{}

	updateAgent(plugin, config, logger, manager, util, pluginInput, mockCancelFlag, &out, time.Now(), execMock)
	assert.Equal(t, util.retryCounter, 2)
}

func TestUpdateAgent(t *testing.T) {
	pluginInput := createStubPluginInput()
	pluginInput.TargetVersion = ""
	info := createStubInstanceInfo()
	manifest := createStubManifest(pluginInput, info, true, true)
	config := contracts.Configuration{}
	plugin := &Plugin{
		Context: context.NewMockDefault(),
	}

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
		execMock := &executormocks.IExecutor{}

		execMock.On("IsPidRunning", mock.Anything).Return(true, nil)
		updateAgent(plugin, config, logger, &manager, &util, pluginInput, mockCancelFlag, &out, time.Now(), execMock)
		assert.Empty(t, out.GetStderr())
	}
}

func TestUpdateAgentUpdaterFailedToStart(t *testing.T) {
	pluginInput := createStubPluginInput()
	pluginInput.TargetVersion = ""
	context := createStubInstanceInfo()
	manifest := createStubManifest(pluginInput, context, true, true)
	config := contracts.Configuration{}
	plugin := &Plugin{}

	manager := fakeUpdateManager{
		generateUpdateCmdResult: "-updater -message id value",
		generateUpdateCmdError:  nil,
		downloadManifestResult:  manifest,
		downloadManifestError:   nil,
		downloadUpdaterResult:   "updater",
		downloadUpdaterError:    nil,
		validateUpdateResult:    false,
		validateUpdateError:     nil,
	}

	pluginInput.TargetVersion = ""
	mockCancelFlag := new(task.MockCancelFlag)
	util := fakeUtility{}

	out := iohandler.DefaultIOHandler{}
	execMock := &executormocks.IExecutor{}

	execMock.On("IsPidRunning", mock.Anything).Return(false, nil)
	execMock.On("Kill", mock.Anything).Return(nil)
	updateAgent(plugin, config, logger, &manager, &util, pluginInput, mockCancelFlag, &out, time.Now(), execMock)
	assert.Equal(t, out.GetStderr(), "Updater died before updating, make sure your system is supported")
}

func TestUpdateAgent_NegativeTestCases(t *testing.T) {
	pluginInput := createStubPluginInput()
	pluginInput.TargetVersion = ""
	info := createStubInstanceInfo()
	manifest := createStubManifest(pluginInput, info, true, true)
	config := contracts.Configuration{}
	plugin := &Plugin{
		Context: context.NewMockDefault(),
	}

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
		execMock := &executormocks.IExecutor{}

		updateAgent(plugin, config, logger, &manager, &util, pluginInput, mockCancelFlag, &out, time.Now(), execMock)
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
	plugin := &Plugin{
		Context: context.NewMockDefault(),
	}

	pluginInput.TargetVersion = ""
	mockCancelFlag := new(task.MockCancelFlag)
	mockLockfile := lockfile.MockLockfile{}
	mockIOHandler := iohandler.DefaultIOHandler{}
	methodCalled := false

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
		startTime time.Time,
		exec executor.IExecutor) int {
		methodCalled = true
		output.MarkAsInProgress()
		return 1
	}

	getLockObj = func(pth string) (lockfile.Lockfile, error) {
		return mockLockfile, nil
	}
	// Setup mocks
	mockCancelFlag.On("Canceled").Return(false)
	mockCancelFlag.On("ShutDown").Return(false)
	mockCancelFlag.On("Wait").Return(false).After(100 * time.Millisecond)

	mockLockfile.On("TryLockExpireWithRetry", int64(60)).Return(nil)
	mockLockfile.On("ShouldRetry", nil).Return(false)
	mockLockfile.On("ChangeOwner", 1).Return(nil)

	updateUtilRef = &fakeUtility{
		downloadErr: false,
	}

	plugin.Execute(config, mockCancelFlag, &mockIOHandler)

	if !methodCalled {
		t.Fail()
		fmt.Println("Error UpdateAgent method never called")
	}
}

func TestExecuteUpdateLocked(t *testing.T) {
	pluginInput := createStubPluginInput()
	pluginInput.TargetVersion = ""
	config := contracts.Configuration{}
	p := make([]interface{}, 1)
	p[0] = pluginInput
	config.Properties = p
	plugin := &Plugin{
		Context: context.NewMockDefault(),
	}
	pluginInput.TargetVersion = ""
	mockCancelFlag := new(task.MockCancelFlag)
	mockLockfile := lockfile.MockLockfile{}
	mockIOHandler := iohandler.DefaultIOHandler{}

	getLockObj = func(pth string) (lockfile.Lockfile, error) {
		return mockLockfile, nil
	}

	// Setup mocks
	mockCancelFlag.On("Canceled").Return(false)
	mockCancelFlag.On("ShutDown").Return(false)
	mockCancelFlag.On("Wait").Return(false).After(100 * time.Millisecond)

	mockLockfile.On("TryLockExpireWithRetry", int64(60)).Return(lockfile.ErrBusy)
	mockLockfile.On("ShouldRetry", lockfile.ErrBusy).Return(false)

	updateUtilRef = &fakeUtility{
		downloadErr: true,
	}
	plugin.Execute(config, mockCancelFlag, &mockIOHandler)
}

func TestExecutePanicDuringUpdate(t *testing.T) {
	pluginInput := createStubPluginInput()
	pluginInput.TargetVersion = ""
	config := contracts.Configuration{}
	p := make([]interface{}, 1)
	p[0] = pluginInput
	config.Properties = p
	plugin := &Plugin{
		Context: context.NewMockDefault(),
	}
	pluginInput.TargetVersion = ""
	mockCancelFlag := new(task.MockCancelFlag)
	mockLockfile := lockfile.MockLockfile{}
	mockIOHandler := iohandler.DefaultIOHandler{}
	methodCalled := false

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
		startTime time.Time,
		exec executor.IExecutor) int {
		methodCalled = true
		panic(fmt.Errorf("Some Random Panic"))
		return 1
	}

	getLockObj = func(pth string) (lockfile.Lockfile, error) {
		return mockLockfile, nil
	}

	// Setup mocks
	mockCancelFlag.On("Canceled").Return(false)
	mockCancelFlag.On("ShutDown").Return(false)
	mockCancelFlag.On("Wait").Return(false).After(100 * time.Millisecond)

	mockLockfile.On("TryLockExpireWithRetry", int64(60)).Return(nil)
	mockLockfile.On("ShouldRetry", nil).Return(false)
	mockLockfile.On("Unlock").Return(nil)

	updateUtilRef = &fakeUtility{
		downloadErr: true,
	}
	plugin.Execute(config, mockCancelFlag, &mockIOHandler)

	if !methodCalled {
		t.Fail()
		fmt.Println("Error UpdateAgent method never called")
	}
}

func TestExecuteFailureDuringUpdate(t *testing.T) {
	pluginInput := createStubPluginInput()
	pluginInput.TargetVersion = ""
	config := contracts.Configuration{}
	p := make([]interface{}, 1)
	p[0] = pluginInput
	config.Properties = p
	plugin := &Plugin{
		Context: context.NewMockDefault(),
	}
	pluginInput.TargetVersion = ""
	mockCancelFlag := new(task.MockCancelFlag)
	mockLockfile := lockfile.MockLockfile{}
	mockIOHandler := iohandler.DefaultIOHandler{}
	methodCalled := false

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
		startTime time.Time,
		exec executor.IExecutor) int {
		methodCalled = true
		output.MarkAsFailed(fmt.Errorf("Some Random Failure"))
		return 1
	}

	getLockObj = func(pth string) (lockfile.Lockfile, error) {
		return mockLockfile, nil
	}
	// Setup mocks
	mockCancelFlag.On("Canceled").Return(false)
	mockCancelFlag.On("ShutDown").Return(false)
	mockCancelFlag.On("Wait").Return(false).After(100 * time.Millisecond)

	mockLockfile.On("TryLockExpireWithRetry", int64(60)).Return(nil)
	mockLockfile.On("ShouldRetry", nil).Return(false)
	mockLockfile.On("Unlock").Return(nil)

	updateUtilRef = &fakeUtility{
		downloadErr: false,
	}
	plugin.Execute(config, mockCancelFlag, &mockIOHandler)

	if !methodCalled {
		t.Fail()
		fmt.Println("Error UpdateAgent method never called")
	}
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
	context *updateutil.InstanceInfo,
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

func createStubInstanceInfo() *updateutil.InstanceInfo {
	info := updateutil.InstanceInfo{}
	info.Arch = "amd64"
	info.CompressFormat = "tar.gz"
	info.InstallerName = updateconstants.PlatformLinux
	info.Platform = updateconstants.PlatformLinux
	info.PlatformVersion = "2015.9"
	return &info
}

type fakeUtility struct {
	retryCounter     int
	pid              int
	execCommandError error
	downloadErr      bool
}

func (u *fakeUtility) CreateInstanceInfo(log log.T) (context *updateutil.InstanceInfo, err error) {
	return createStubInstanceInfo(), nil
}

func (u *fakeUtility) CleanupCommand(log log.T, pid int) error {
	return nil
}

func (u *fakeUtility) IsServiceRunning(log log.T, i *updateutil.InstanceInfo) (result bool, err error) {
	return true, nil
}

func (u *fakeUtility) IsWorkerRunning(log log.T) (result bool, err error) {
	return true, nil
}

func (u *fakeUtility) WaitForServiceToStart(log log.T, i *updateutil.InstanceInfo, targetVersion string) (result bool, err error) {
	return true, nil
}

func (u *fakeUtility) CreateUpdateDownloadFolder() (folder string, err error) {
	if u.downloadErr {
		return "", fmt.Errorf("download error")
	}
	return "", nil
}

func (u *fakeUtility) ExeCommand(
	log log.T,
	cmd string,
	updateRoot string,
	workingDir string,
	stdOut string,
	stdErr string,
	isAsync bool) (pid int, exitCode updateconstants.UpdateScriptExitCode, err error) {
	u.retryCounter++
	return u.pid, exitCode, u.execCommandError
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

func (u *fakeUtility) DownloadManifestFile(log log.T, updateDownloadFolder string, manifestUrl string, region string) (*artifact.DownloadOutput, string, error) {

	return &artifact.DownloadOutput{
		LocalFilePath: "testPath",
		IsUpdated:     true,
		IsHashMatched: true,
	}, "manifestUrl", nil
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
	context *updateutil.InstanceInfo,
	updaterVersion string,
	messageID string,
	stdout string,
	stderr string,
	keyPrefix string,
	bucketName string) (cmd string, err error) {

	return u.generateUpdateCmdResult, u.generateUpdateCmdError
}

func (u *fakeUpdateManager) downloadManifest(context context.T,
	util updateutil.T,
	pluginInput *UpdatePluginInput,
	info *updateutil.InstanceInfo,
	out iohandler.IOHandler) (manifest *Manifest, err error) {
	return u.downloadManifestResult, u.downloadManifestError
}

func (u *fakeUpdateManager) downloadUpdater(context context.T,
	util updateutil.T,
	updaterPackageName string,
	manifest *Manifest,
	out iohandler.IOHandler,
	info *updateutil.InstanceInfo) (version string, err error) {
	return u.downloadUpdaterResult, u.downloadUpdaterError
}

func (u *fakeUpdateManager) validateUpdate(log log.T,
	pluginInput *UpdatePluginInput,
	context *updateutil.InstanceInfo,
	manifest *Manifest,
	out iohandler.IOHandler) (noNeedToUpdate bool, err error) {
	return u.validateUpdateResult, u.validateUpdateError
}
