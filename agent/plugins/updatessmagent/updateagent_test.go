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
	"github.com/aws/amazon-ssm-agent/agent/framework/processor/executer/iohandler"
	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/amazon-ssm-agent/agent/task"
	"github.com/aws/amazon-ssm-agent/agent/updateutil"
	"github.com/aws/amazon-ssm-agent/agent/updateutil/updateconstants"
	"github.com/aws/amazon-ssm-agent/agent/updateutil/updateinfo"
	"github.com/aws/amazon-ssm-agent/agent/updateutil/updatemanifest"
	updatemanifestmocks "github.com/aws/amazon-ssm-agent/agent/updateutil/updatemanifest/mocks"
	"github.com/aws/amazon-ssm-agent/agent/updateutil/updates3util"
	updates3utilmocks "github.com/aws/amazon-ssm-agent/agent/updateutil/updates3util/mocks"
	"github.com/aws/amazon-ssm-agent/core/executor"
	executormocks "github.com/aws/amazon-ssm-agent/core/executor/mocks"
	"github.com/nightlyone/lockfile"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

var mockContext = context.NewMockDefault()

func TestGenerateUpdateCmd(t *testing.T) {
	pluginInput := createStubPluginInput()

	result, err := generateUpdateCmd(pluginInput,
		"3.0.0.0", "messageID", "stdout", "stderr", "prefix", "bucket")

	assert.NoError(t, err)
	assert.Contains(t, result, "3.0.0.0")
	assert.Contains(t, result, "messageID")
	assert.Contains(t, result, "stdout")
	assert.Contains(t, result, "stderr")
	assert.Contains(t, result, "prefix")
	assert.Contains(t, result, "bucket")
	assert.Contains(t, result, "manifest")
	assert.NotContains(t, result, "-"+updateconstants.DisableDowngradeCmd)
}

func TestGenerateUpdateCmdNoDowngrade(t *testing.T) {
	pluginInput := createStubPluginInput()
	pluginInput.AllowDowngrade = "false"

	result, err := generateUpdateCmd(pluginInput,
		"3.0.0.0", "messageID", "stdout", "stderr", "prefix", "bucket")

	assert.NoError(t, err)
	assert.Contains(t, result, "3.0.0.0")
	assert.Contains(t, result, "messageID")
	assert.Contains(t, result, "stdout")
	assert.Contains(t, result, "stderr")
	assert.Contains(t, result, "prefix")
	assert.Contains(t, result, "bucket")
	assert.Contains(t, result, "manifest")
	assert.Contains(t, result, "-"+updateconstants.DisableDowngradeCmd)
}

func TestGenerateUpdateCmdInvalidDowngrade(t *testing.T) {
	pluginInput := createStubPluginInput()
	pluginInput.AllowDowngrade = "somerandomstring"

	_, err := generateUpdateCmd(pluginInput,
		"3.0.0.0", "messageID", "stdout", "stderr", "prefix", "bucket")

	assert.Error(t, err)
}

func TestUpdateAgent_InvalidPluginRaw(t *testing.T) {
	config := contracts.Configuration{}
	util := &fakeUtility{}

	manifest := &updatemanifestmocks.T{}

	s3Util := &updates3utilmocks.T{}
	rawPluginInput := "invalid value" // string value will failed the Remarshal as it's not PluginInput
	out := iohandler.DefaultIOHandler{}
	execMock := &executormocks.IExecutor{}
	downloadfolder := "somefolder"

	updateAgent(config, mockContext, util, s3Util, manifest, rawPluginInput, &out, time.Now(), execMock, downloadfolder)

	assert.Contains(t, out.GetStderr(), "invalid format in plugin properties")
}

func TestUpdateAgent_FailedDownloadManifest(t *testing.T) {
	pluginInput := createStubPluginInput()
	config := contracts.Configuration{}
	util := &fakeUtility{}

	manifest := &updatemanifestmocks.T{}

	s3Util := &updates3utilmocks.T{}
	s3Util.On("DownloadManifest", mock.Anything, pluginInput.Source).Return(fmt.Errorf("SomeDownloadManifestError"))

	out := iohandler.DefaultIOHandler{}
	execMock := &executormocks.IExecutor{}
	downloadfolder := "somefolder"

	updateAgent(config, mockContext, util, s3Util, manifest, pluginInput, &out, time.Now(), execMock, downloadfolder)

	assert.Equal(t, contracts.ResultStatusFailed, out.Status)
	assert.Contains(t, out.GetStderr(), "SomeDownloadManifestError")
}

func TestUpdateAgent_FailedDownloadUpdater(t *testing.T) {
	pluginInput := createStubPluginInput()
	config := contracts.Configuration{}
	util := &fakeUtility{}

	manifest := createStubManifest(pluginInput, true, true)
	s3Util := &updates3utilmocks.T{}

	out := iohandler.DefaultIOHandler{}
	execMock := &executormocks.IExecutor{}
	downloadfolder := "somefolder"

	// Define behavior
	s3Util.On("DownloadManifest", mock.Anything, pluginInput.Source).Return(nil)
	s3Util.On("DownloadUpdater", mock.Anything, pluginInput.UpdaterName, downloadfolder).Return("", fmt.Errorf("SomeDownloadError"))

	updateAgent(config, mockContext, util, s3Util, manifest, pluginInput, &out, time.Now(), execMock, downloadfolder)

	assert.Equal(t, contracts.ResultStatusFailed, out.Status)
	assert.Equal(t, "SomeDownloadError", out.GetStderr())
}

func TestUpdateAgent_FailedGenerateUpdateCmd(t *testing.T) {
	pluginInput := createStubPluginInput()
	pluginInput.AllowDowngrade = "FailParseBool"
	config := contracts.Configuration{}
	util := &fakeUtility{}

	manifest := createStubManifest(pluginInput, true, true)

	s3Util := &updates3utilmocks.T{}

	out := iohandler.DefaultIOHandler{}
	execMock := &executormocks.IExecutor{}
	downloadfolder := "somefolder"

	// Define behavior
	s3Util.On("DownloadManifest", mock.Anything, pluginInput.Source).Return(nil)
	s3Util.On("DownloadUpdater", mock.Anything, pluginInput.UpdaterName, downloadfolder).Return("", nil)

	updateAgent(config, mockContext, util, s3Util, manifest, pluginInput, &out, time.Now(), execMock, downloadfolder)

	assert.Equal(t, contracts.ResultStatusFailed, out.Status)
	assert.Contains(t, out.GetStderr(), "FailParseBool")
}

func TestUpdateAgent_FailedSaveUpdatePlugin(t *testing.T) {
	pluginInput := createStubPluginInput()
	config := contracts.Configuration{}
	util := &fakeUtility{savePluginErr: fmt.Errorf("SomeSaveError")}

	manifest := createStubManifest(pluginInput, true, true)

	s3Util := &updates3utilmocks.T{}

	out := iohandler.DefaultIOHandler{}
	execMock := &executormocks.IExecutor{}
	downloadfolder := "somefolder"

	// Define behavior
	s3Util.On("DownloadManifest", mock.Anything, pluginInput.Source).Return(nil)
	s3Util.On("DownloadUpdater", mock.Anything, pluginInput.UpdaterName, downloadfolder).Return("", nil)

	updateAgent(config, mockContext, util, s3Util, manifest, pluginInput, &out, time.Now(), execMock, downloadfolder)

	assert.Equal(t, contracts.ResultStatusFailed, out.Status)
	assert.Equal(t, "SomeSaveError", out.GetStderr())
}

func TestUpdateAgent_FailedNoDiskSpaceFalse(t *testing.T) {
	pluginInput := createStubPluginInput()
	config := contracts.Configuration{}
	util := &fakeUtility{noDiskSpace: true}

	manifest := createStubManifest(pluginInput, true, true)

	s3Util := &updates3utilmocks.T{}

	out := iohandler.DefaultIOHandler{}
	execMock := &executormocks.IExecutor{}
	downloadfolder := "somefolder"

	// Define behavior
	s3Util.On("DownloadManifest", mock.Anything, pluginInput.Source).Return(nil)
	s3Util.On("DownloadUpdater", mock.Anything, pluginInput.UpdaterName, downloadfolder).Return("", nil)

	updateAgent(config, mockContext, util, s3Util, manifest, pluginInput, &out, time.Now(), execMock, downloadfolder)

	assert.Equal(t, contracts.ResultStatusFailed, out.Status)
	assert.Equal(t, "Insufficient available disk space", out.GetStderr())
}

func TestUpdateAgent_FailedNoDiskSpaceError(t *testing.T) {
	pluginInput := createStubPluginInput()
	config := contracts.Configuration{}
	util := &fakeUtility{isDiskSpaceErr: fmt.Errorf("SomeDiskError")}

	manifest := createStubManifest(pluginInput, true, true)

	s3Util := &updates3utilmocks.T{}

	out := iohandler.DefaultIOHandler{}
	execMock := &executormocks.IExecutor{}
	downloadfolder := "somefolder"

	// Define behavior
	s3Util.On("DownloadManifest", mock.Anything, pluginInput.Source).Return(nil)
	s3Util.On("DownloadUpdater", mock.Anything, pluginInput.UpdaterName, downloadfolder).Return("", nil)

	updateAgent(config, mockContext, util, s3Util, manifest, pluginInput, &out, time.Now(), execMock, downloadfolder)

	assert.Equal(t, contracts.ResultStatusFailed, out.Status)
	assert.Equal(t, "SomeDiskError", out.GetStderr())
}

func TestUpdateAgent_FailedExecUpdater(t *testing.T) {
	pluginInput := createStubPluginInput()
	config := contracts.Configuration{}
	pid := -5
	util := &fakeUtility{pid: pid, execCommandError: fmt.Errorf("SomeCmdError")}

	manifest := createStubManifest(pluginInput, true, true)

	s3Util := &updates3utilmocks.T{}

	out := iohandler.DefaultIOHandler{}
	execMock := &executormocks.IExecutor{}
	downloadfolder := "somefolder"

	// Define behavior
	s3Util.On("DownloadManifest", mock.Anything, pluginInput.Source).Return(nil)
	s3Util.On("DownloadUpdater", mock.Anything, pluginInput.UpdaterName, downloadfolder).Return("", nil)

	out_pid := updateAgent(config, mockContext, util, s3Util, manifest, pluginInput, &out, time.Now(), execMock, downloadfolder)

	assert.Equal(t, util.retryCounter, noOfRetries)
	assert.Equal(t, contracts.ResultStatusFailed, out.Status)
	assert.Contains(t, out.GetStderr(), "SomeCmdError")

	assert.Equal(t, pid, out_pid)
}

func TestUpdateAgent_FailedIsPidRunningError(t *testing.T) {
	pluginInput := createStubPluginInput()
	config := contracts.Configuration{}
	pid := 5
	util := &fakeUtility{pid: pid}

	manifest := createStubManifest(pluginInput, true, true)

	s3Util := &updates3utilmocks.T{}

	out := iohandler.DefaultIOHandler{}
	execMock := &executormocks.IExecutor{}
	downloadfolder := "somefolder"

	// Define behavior
	s3Util.On("DownloadManifest", mock.Anything, pluginInput.Source).Return(nil)
	s3Util.On("DownloadUpdater", mock.Anything, pluginInput.UpdaterName, downloadfolder).Return("", nil)

	execMock.On("IsPidRunning", mock.Anything).Return(false, fmt.Errorf("SomeIsPidRunningError"))

	out_pid := updateAgent(config, mockContext, util, s3Util, manifest, pluginInput, &out, time.Now(), execMock, downloadfolder)

	// We don't want to fail if we fail to get if the process is running or not. Updater could still be running
	assert.Equal(t, contracts.ResultStatusInProgress, out.Status)
	assert.Equal(t, "", out.GetStderr())
	assert.Equal(t, pid, out_pid)
}

func TestUpdateAgent_FailedIsPidRunningFalse(t *testing.T) {
	pluginInput := createStubPluginInput()
	config := contracts.Configuration{}
	pid := 5
	util := &fakeUtility{pid: pid}

	manifest := createStubManifest(pluginInput, true, true)

	s3Util := &updates3utilmocks.T{}

	out := iohandler.DefaultIOHandler{}
	execMock := &executormocks.IExecutor{}
	downloadfolder := "somefolder"

	// Define behavior
	s3Util.On("DownloadManifest", mock.Anything, pluginInput.Source).Return(nil)
	s3Util.On("DownloadUpdater", mock.Anything, pluginInput.UpdaterName, downloadfolder).Return("", nil)

	execMock.On("IsPidRunning", pid).Return(false, nil)
	execMock.On("Kill", pid).Return(nil)

	out_pid := updateAgent(config, mockContext, util, s3Util, manifest, pluginInput, &out, time.Now(), execMock, downloadfolder)

	// We don't want to fail if we fail to get if the process is running or not. Updater could still be running
	assert.Equal(t, contracts.ResultStatusFailed, out.Status)
	assert.Contains(t, out.GetStderr(), "")
	assert.Equal(t, pid, out_pid)
}

func TestUpdateAgent(t *testing.T) {
	pluginInput := createStubPluginInput()
	config := contracts.Configuration{}
	pid := 5
	util := &fakeUtility{pid: pid}

	manifest := createStubManifest(pluginInput, true, true)

	s3Util := &updates3utilmocks.T{}

	out := iohandler.DefaultIOHandler{}
	execMock := &executormocks.IExecutor{}
	downloadfolder := "somefolder"

	// Define behavior
	s3Util.On("DownloadManifest", mock.Anything, pluginInput.Source).Return(nil)
	s3Util.On("DownloadUpdater", mock.Anything, pluginInput.UpdaterName, downloadfolder).Return("", nil)

	execMock.On("IsPidRunning", mock.Anything).Return(true, nil)

	out_pid := updateAgent(config, mockContext, util, s3Util, manifest, pluginInput, &out, time.Now(), execMock, downloadfolder)

	assert.Equal(t, contracts.ResultStatusInProgress, out.Status)
	assert.Equal(t, "", out.GetStderr())
	assert.Equal(t, pid, out_pid)
}

func TestExecute(t *testing.T) {
	pluginInput := createStubPluginInput()
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
		config contracts.Configuration,
		context context.T,
		util updateutil.T,
		s3util updates3util.T,
		manifest updatemanifest.T,
		rawPluginInput interface{},
		output iohandler.IOHandler,
		startTime time.Time,
		exec executor.IExecutor,
		downloadFolder string) int {
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
		config contracts.Configuration,
		context context.T,
		util updateutil.T,
		s3util updates3util.T,
		manifest updatemanifest.T,
		rawPluginInput interface{},
		output iohandler.IOHandler,
		startTime time.Time,
		exec executor.IExecutor,
		downloadFolder string) int {
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
		config contracts.Configuration,
		context context.T,
		util updateutil.T,
		s3util updates3util.T,
		manifest updatemanifest.T,
		rawPluginInput interface{},
		output iohandler.IOHandler,
		startTime time.Time,
		exec executor.IExecutor,
		downloadFolder string) int {
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
	return &UpdatePluginInput{
		TargetVersion:  "9000.0.0.0",
		AgentName:      "amazon-ssm-agent",
		UpdaterName:    "amazon-ssm-agent-updater",
		AllowDowngrade: "true",
		Source:         "testSource",
	}
}

func createStubManifest(
	plugin *UpdatePluginInput,
	addCurrentVersion bool,
	addTargetVersion bool) *updatemanifestmocks.T {

	manifest := &updatemanifestmocks.T{}

	if addCurrentVersion {
		manifest.On("HasVersion", mock.Anything, currentAgentVersion).Return(true)
	}

	if addTargetVersion {
		manifest.On("HasVersion", mock.Anything, plugin.TargetVersion).Return(true)

	}

	return manifest
}

type fakeUtility struct {
	retryCounter     int
	pid              int
	execCommandError error
	downloadErr      bool
	savePluginErr    error
	noDiskSpace      bool
	isDiskSpaceErr   error
}

func (u *fakeUtility) CleanupCommand(log log.T, pid int) error {
	return nil
}

func (u *fakeUtility) IsServiceRunning(log log.T, i updateinfo.T) (result bool, err error) {
	return true, nil
}

func (u *fakeUtility) IsWorkerRunning(log log.T) (result bool, err error) {
	return true, nil
}

func (u *fakeUtility) WaitForServiceToStart(log log.T, i updateinfo.T, targetVersion string) (result bool, err error) {
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
	return u.savePluginErr
}

func (u *fakeUtility) IsDiskSpaceSufficientForUpdate(log log.T) (bool, error) {
	return !u.noDiskSpace, u.isDiskSpaceErr
}
