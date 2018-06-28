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

// Package processor contains the methods for update ssm agent.
// It also provides methods for sendReply and updateInstanceInfo
package processor

import (
	"fmt"
	"testing"

	"github.com/aws/amazon-ssm-agent/agent/contracts"
	"github.com/aws/amazon-ssm-agent/agent/fileutil/artifact"
	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/amazon-ssm-agent/agent/updateutil"
	"github.com/stretchr/testify/assert"
)

var logger = log.NewMockLog()

type serviceStub struct {
	Service
}

func (s *serviceStub) SendReply(log log.T, update *UpdateDetail) error {
	return nil
}

func (s *serviceStub) DeleteMessage(log log.T, update *UpdateDetail) error {
	return nil
}

func (s *serviceStub) UpdateHealthCheck(log log.T, update *UpdateDetail, errorCode string) error {
	return nil
}

type contextMgrStub struct{}

func (c *contextMgrStub) saveUpdateContext(log log.T, context *UpdateContext, contextLocation string) (err error) {
	return nil
}

func (c *contextMgrStub) uploadOutput(log log.T, context *UpdateContext, orchestrationDir string) error {
	return nil
}

func TestStartOrResumeUpdateFromInstalledState(t *testing.T) {
	// setup
	updater := createDefaultUpdaterStub()
	isMethodExecuted := false
	context := createUpdateContext(Installed)
	// mock the verify method
	updater.mgr.verify = func(mgr *updateManager, log log.T, context *UpdateContext, isRollback bool) error {
		isMethodExecuted = true
		return nil
	}
	// action
	updater.StartOrResumeUpdate(logger, context)
	// assert
	assert.True(t, isMethodExecuted)
}

func TestStartOrResumeUpdateFromInitializedState(t *testing.T) {
	// setup
	updater := createDefaultUpdaterStub()
	isMethodExecuted := false
	context := createUpdateContext(Initialized)
	// mock the verify method
	updater.mgr.prepare = func(mgr *updateManager, log log.T, context *UpdateContext) error {
		isMethodExecuted = true
		return nil
	}
	// action
	updater.StartOrResumeUpdate(logger, context)
	// assert
	assert.True(t, isMethodExecuted)
}

func TestStartOrResumeUpdateFromStagedState(t *testing.T) {
	// setup
	updater := createDefaultUpdaterStub()
	isMethodExecuted := false
	context := createUpdateContext(Staged)
	// mock the verify method
	updater.mgr.update = func(mgr *updateManager, log log.T, context *UpdateContext) error {
		isMethodExecuted = true
		return nil
	}
	// action
	updater.StartOrResumeUpdate(logger, context)
	// assert
	assert.True(t, isMethodExecuted)
}

func TestInitializeUpdate(t *testing.T) {
	// setup
	updater := createDefaultUpdaterStub()
	context := createUpdateContext("")

	// action
	context, err := updater.InitializeUpdate(logger, context.Current)

	// assert
	assert.NotEmpty(t, context.Current.StandardOut)
	assert.NotEmpty(t, context.Current.StartDateTime)
	assert.NoError(t, err)
}

func TestPrepareInstallationPackages(t *testing.T) {
	// setup
	updater := createDefaultUpdaterStub()
	context := createUpdateContext(Initialized)
	isUpdateCalled := false

	// stub download for updater
	updater.mgr.download = func(mgr *updateManager, log log.T, downloadInput artifact.DownloadInput, context *UpdateContext, version string) (err error) {
		return nil
	}
	// stop at the end of prepareInstallationPackages, do not perform update
	updater.mgr.update = func(mgr *updateManager, log log.T, context *UpdateContext) (err error) {
		isUpdateCalled = true
		return nil
	}
	// action
	err := prepareInstallationPackages(updater.mgr, logger, context)

	// assert
	assert.NoError(t, err)
	assert.Equal(t, context.Current.State, Staged)
	assert.NotEmpty(t, context.Current.StandardOut)
	assert.Empty(t, context.Histories)
	assert.True(t, isUpdateCalled)
}

func TestPreparePackagesFailCreateInstanceContext(t *testing.T) {
	// setup
	control := &stubControl{failCreateInstanceContext: true}
	updater := createUpdaterStubs(control)
	context := createUpdateContext(Initialized)

	// action
	err := prepareInstallationPackages(updater.mgr, logger, context)

	// assert
	assert.NoError(t, err)
	assert.Equal(t, string(context.Current.State), "")
	assert.Equal(t, context.Histories[0].State, Completed)
	assert.Equal(t, context.Histories[0].Result, contracts.ResultStatusFailed)
}

func TestPreparePackagesFailCreateUpdateDownloadFolder(t *testing.T) {
	// setup
	updater := createDefaultUpdaterStub()
	context := createUpdateContext(Initialized)

	// stub download for updater
	updater.mgr.download = func(mgr *updateManager, log log.T, downloadInput artifact.DownloadInput, context *UpdateContext, version string) (err error) {
		return fmt.Errorf("no access")
	}

	// action
	err := prepareInstallationPackages(updater.mgr, logger, context)

	// assert
	assert.NoError(t, err)
	assert.Equal(t, string(context.Current.State), "")
	assert.Equal(t, context.Histories[0].State, Completed)
	assert.Equal(t, context.Histories[0].Result, contracts.ResultStatusFailed)
}

func TestPreparePackagesFailDownload(t *testing.T) {
	// setup
	control := &stubControl{failCreateUpdateDownloadFolder: true}
	updater := createUpdaterStubs(control)
	context := createUpdateContext(Initialized)

	// action
	err := prepareInstallationPackages(updater.mgr, logger, context)

	// assert
	assert.NoError(t, err)
	assert.Equal(t, string(context.Current.State), "")
	assert.Equal(t, context.Histories[0].State, Completed)
	assert.Equal(t, context.Histories[0].Result, contracts.ResultStatusFailed)
}

func TestValidateUpdateVersion(t *testing.T) {
	context := createUpdateContext(Initialized)
	instanceContext := &updateutil.InstanceContext{
		Region:          "us-east-1",
		Platform:        updateutil.PlatformRedHat,
		PlatformVersion: "6.5",
		InstallerName:   "linux",
		Arch:            "amd64",
		CompressFormat:  "tar.gz",
	}

	err := validateUpdateVersion(logger, context.Current, instanceContext)

	assert.NoError(t, err)
}

func TestValidateUpdateVersionFailCentOs(t *testing.T) {
	context := createUpdateContext(Initialized)
	context.Current.TargetVersion = "1.0.0.0"
	instanceContext := &updateutil.InstanceContext{
		Region:          "us-east-1",
		Platform:        updateutil.PlatformCentOS,
		PlatformVersion: "6.5",
		InstallerName:   "linux",
		Arch:            "amd64",
		CompressFormat:  "tar.gz",
	}

	err := validateUpdateVersion(logger, context.Current, instanceContext)

	assert.Error(t, err)
}

func TestProceedUpdate(t *testing.T) {
	// setup
	updater := createDefaultUpdaterStub()
	context := createUpdateContext(Staged)
	isVerifyCalled := false

	// stub install for updater
	updater.mgr.install = func(mgr *updateManager, log log.T, version string, context *UpdateContext) (err error) {
		return nil
	}

	updater.mgr.verify = func(mgr *updateManager, log log.T, context *UpdateContext, isRollback bool) (err error) {
		isVerifyCalled = true
		return nil
	}

	// action
	err := proceedUpdate(updater.mgr, logger, context)

	// assert
	assert.NoError(t, err)
	assert.Equal(t, context.Current.State, Installed)
	assert.True(t, isVerifyCalled)
	assert.Empty(t, context.Histories)
}

func TestProceedUpdateWithDowngrade(t *testing.T) {
	// setup
	updater := createDefaultUpdaterStub()
	context := createUpdateContext(Staged)
	context.Current.RequiresUninstall = true
	isVerifyCalled := false
	isUninstallCalled := false

	// stub install for updater
	updater.mgr.install = func(mgr *updateManager, log log.T, version string, context *UpdateContext) (err error) {
		return nil
	}
	updater.mgr.uninstall = func(mgr *updateManager, log log.T, version string, context *UpdateContext) (err error) {
		isUninstallCalled = true
		return nil
	}
	updater.mgr.verify = func(mgr *updateManager, log log.T, context *UpdateContext, isRollback bool) (err error) {
		isVerifyCalled = true
		return nil
	}

	// action
	err := proceedUpdate(updater.mgr, logger, context)

	// assert
	assert.NoError(t, err)
	assert.True(t, isVerifyCalled)
	assert.True(t, isUninstallCalled)
	assert.Equal(t, context.Current.State, Installed)
	assert.Empty(t, context.Histories)
}

func TestProceedUpdateWithDowngradeFailUninstall(t *testing.T) {
	// setup
	updater := createDefaultUpdaterStub()
	context := createUpdateContext(Staged)
	context.Current.RequiresUninstall = true
	isVerifyCalled := false
	isUninstallCalled := false

	// stub install for updater
	updater.mgr.install = func(mgr *updateManager, log log.T, version string, context *UpdateContext) (err error) {
		return nil
	}
	updater.mgr.uninstall = func(mgr *updateManager, log log.T, version string, context *UpdateContext) (err error) {
		isUninstallCalled = true
		return fmt.Errorf("cannot uninstall")
	}
	updater.mgr.verify = func(mgr *updateManager, log log.T, context *UpdateContext, isRollback bool) (err error) {
		isVerifyCalled = true
		return nil
	}

	// action
	err := proceedUpdate(updater.mgr, logger, context)

	// assert
	assert.NoError(t, err)
	assert.False(t, isVerifyCalled)
	assert.True(t, isUninstallCalled)
	assert.Equal(t, string(context.Current.State), "")
	assert.Equal(t, context.Histories[0].State, Completed)
	assert.Equal(t, context.Histories[0].Result, contracts.ResultStatusFailed)
}

func TestProceedUpdateFailInstall(t *testing.T) {
	// setup
	updater := createDefaultUpdaterStub()
	context := createUpdateContext(Staged)
	isRollbackCalled := false

	// stub install for updater
	updater.mgr.install = func(mgr *updateManager, log log.T, version string, context *UpdateContext) (err error) {
		return fmt.Errorf("install failed")
	}

	updater.mgr.rollback = func(mgr *updateManager, log log.T, context *UpdateContext) (err error) {
		isRollbackCalled = true
		return nil
	}

	// action
	err := proceedUpdate(updater.mgr, logger, context)

	// assert
	assert.NoError(t, err)
	assert.True(t, isRollbackCalled)
	assert.Equal(t, context.Current.State, Rollback)
	assert.Empty(t, context.Histories)
}

func TestVerifyInstallation(t *testing.T) {
	// setup
	control := &stubControl{serviceIsRunning: true}
	updater := createUpdaterStubs(control)
	context := createUpdateContext(Installed)

	// action
	err := verifyInstallation(updater.mgr, logger, context, false)

	// assert
	assert.NoError(t, err)
	assert.Equal(t, string(context.Current.State), "")
	assert.Equal(t, context.Histories[0].State, Completed)
	assert.Equal(t, context.Histories[0].Result, contracts.ResultStatusSuccess)
}

func TestVerifyInstallationFailedGetInstanceContext(t *testing.T) {
	// setup
	control := &stubControl{failCreateInstanceContext: true}
	updater := createUpdaterStubs(control)
	context := createUpdateContext(Installed)

	// action
	err := verifyInstallation(updater.mgr, logger, context, false)

	// assert
	assert.NoError(t, err)
	assert.Equal(t, string(context.Current.State), "")
	assert.Equal(t, context.Histories[0].State, Completed)
	assert.Equal(t, context.Histories[0].Result, contracts.ResultStatusFailed)
}

func TestVerifyInstallationCannotStartAgent(t *testing.T) {
	// setup
	control := &stubControl{serviceIsRunning: false}
	updater := createUpdaterStubs(control)
	context := createUpdateContext(Installed)
	isRollbackCalled := false

	updater.mgr.rollback = func(mgr *updateManager, log log.T, context *UpdateContext) (err error) {
		isRollbackCalled = true
		return nil
	}

	// action
	err := verifyInstallation(updater.mgr, logger, context, false)

	// assert
	assert.NoError(t, err)
	assert.True(t, isRollbackCalled)
	assert.Equal(t, context.Current.State, Rollback)
}

func TestVerifyRollback(t *testing.T) {
	// setup
	control := &stubControl{serviceIsRunning: true}
	updater := createUpdaterStubs(control)
	context := createUpdateContext(RolledBack)

	// action
	err := verifyInstallation(updater.mgr, logger, context, true)

	// assert
	assert.NoError(t, err)
	assert.Equal(t, string(context.Current.State), "")
	assert.Equal(t, context.Histories[0].State, Completed)
	assert.Equal(t, context.Histories[0].Result, contracts.ResultStatusFailed)
}

func TestVerifyRollbackCannotStartAgent(t *testing.T) {
	// setup
	control := &stubControl{serviceIsRunning: false}
	updater := createUpdaterStubs(control)
	context := createUpdateContext(RolledBack)

	// action
	err := verifyInstallation(updater.mgr, logger, context, true)

	// assert
	assert.NoError(t, err)
	assert.Equal(t, string(context.Current.State), "")
	assert.Equal(t, context.Histories[0].State, Completed)
	assert.Equal(t, context.Histories[0].Result, contracts.ResultStatusFailed)
}

func TestRollbackInstallation(t *testing.T) {
	// setup
	control := &stubControl{serviceIsRunning: false}
	updater := createUpdaterStubs(control)
	context := createUpdateContext(Rollback)

	isVerifyCalled, isInstallCalled, isUninstallCalled := false, false, false

	// stub install for updater
	updater.mgr.install = func(mgr *updateManager, log log.T, version string, context *UpdateContext) (err error) {
		isInstallCalled = true
		return nil
	}
	updater.mgr.uninstall = func(mgr *updateManager, log log.T, version string, context *UpdateContext) (err error) {
		isUninstallCalled = true
		return nil
	}
	updater.mgr.verify = func(mgr *updateManager, log log.T, context *UpdateContext, isRollback bool) (err error) {
		isVerifyCalled = true
		return nil
	}
	// action
	err := rollbackInstallation(updater.mgr, logger, context)

	// assert
	assert.NoError(t, err)
	assert.True(t, isVerifyCalled, isInstallCalled, isUninstallCalled)
	assert.Equal(t, context.Current.State, RolledBack)
}

func TestRollbackInstallationFailUninstall(t *testing.T) {
	// setup
	control := &stubControl{serviceIsRunning: false}
	updater := createUpdaterStubs(control)
	context := createUpdateContext(Rollback)

	isVerifyCalled, isInstallCalled, isUninstallCalled := false, false, false

	// stub install for updater
	updater.mgr.install = func(mgr *updateManager, log log.T, version string, context *UpdateContext) (err error) {
		isInstallCalled = true
		return nil
	}
	updater.mgr.uninstall = func(mgr *updateManager, log log.T, version string, context *UpdateContext) (err error) {
		isUninstallCalled = true
		return fmt.Errorf("cannot uninstall")
	}
	updater.mgr.verify = func(mgr *updateManager, log log.T, context *UpdateContext, isRollback bool) (err error) {
		isVerifyCalled = true
		return nil
	}
	// action
	err := rollbackInstallation(updater.mgr, logger, context)

	// assert
	assert.NoError(t, err)
	assert.True(t, isUninstallCalled)
	assert.False(t, isInstallCalled, isVerifyCalled)
	assert.Equal(t, context.Histories[0].State, Completed)
	assert.Equal(t, context.Histories[0].Result, contracts.ResultStatusFailed)
}

func TestRollbackInstallationFailInstall(t *testing.T) {
	// setup
	control := &stubControl{serviceIsRunning: false}
	updater := createUpdaterStubs(control)
	context := createUpdateContext(Rollback)

	isVerifyCalled, isInstallCalled, isUninstallCalled := false, false, false

	// stub install for updater
	updater.mgr.install = func(mgr *updateManager, log log.T, version string, context *UpdateContext) (err error) {
		isInstallCalled = true
		return fmt.Errorf("cannot uninstall")
	}
	updater.mgr.uninstall = func(mgr *updateManager, log log.T, version string, context *UpdateContext) (err error) {
		isUninstallCalled = true
		return nil
	}
	updater.mgr.verify = func(mgr *updateManager, log log.T, context *UpdateContext, isRollback bool) (err error) {
		isVerifyCalled = true
		return nil
	}
	// action
	err := rollbackInstallation(updater.mgr, logger, context)

	// assert
	assert.NoError(t, err)
	assert.True(t, isUninstallCalled, isInstallCalled)
	assert.False(t, isVerifyCalled)
	assert.Equal(t, context.Histories[0].State, Completed)
	assert.Equal(t, context.Histories[0].Result, contracts.ResultStatusFailed)
}

func TestUninstallAgent(t *testing.T) {
	// setup
	control := &stubControl{failExeCommand: false}
	updater := createUpdaterStubs(control)
	context := createUpdateContext(Initialized)

	// action
	err := uninstallAgent(updater.mgr, logger, context.Current.TargetVersion, context)

	// assert
	assert.NoError(t, err)
}

func TestUninstallAgentFailExeCommand(t *testing.T) {
	// setup
	control := &stubControl{failExeCommand: true}
	updater := createUpdaterStubs(control)
	context := createUpdateContext(Initialized)

	// action
	err := uninstallAgent(updater.mgr, logger, context.Current.TargetVersion, context)

	// assert
	assert.Error(t, err)
}

func TestInstallAgent(t *testing.T) {
	// setup
	control := &stubControl{failExeCommand: false}
	updater := createUpdaterStubs(control)
	context := createUpdateContext(Initialized)

	// action
	err := installAgent(updater.mgr, logger, context.Current.TargetVersion, context)

	// assert
	assert.NoError(t, err)
}

func TestInstallAgentFailExeCommand(t *testing.T) {
	// setup
	control := &stubControl{failExeCommand: true}
	updater := createUpdaterStubs(control)
	context := createUpdateContext(Initialized)

	// action
	err := installAgent(updater.mgr, logger, context.Current.TargetVersion, context)

	// assert
	assert.Error(t, err)
}

func TestDownloadAndUnzipArtifact(t *testing.T) {
	// setup
	control := &stubControl{failExeCommand: true}
	updater := createUpdaterStubs(control)
	context := createUpdateContext(Initialized)
	downloadOutput := artifact.DownloadOutput{
		IsHashMatched: true,
		LocalFilePath: "filepath",
	}

	downloadArtifact = func(log log.T, input artifact.DownloadInput) (output artifact.DownloadOutput, err error) {
		return downloadOutput, nil
	}
	uncompress = func(log log.T, src, dest string) error {
		return nil
	}

	// action
	err := downloadAndUnzipArtifact(updater.mgr, logger, artifact.DownloadInput{}, context, context.Current.TargetVersion)

	// assert
	assert.NoError(t, err)
}

func TestDownloadWithError(t *testing.T) {
	// setup
	control := &stubControl{failExeCommand: true}
	updater := createUpdaterStubs(control)
	context := createUpdateContext(Initialized)
	downloadOutput := artifact.DownloadOutput{
		IsHashMatched: false,
		LocalFilePath: "",
	}

	downloadArtifact = func(log log.T, input artifact.DownloadInput) (output artifact.DownloadOutput, err error) {
		return downloadOutput, nil
	}

	// action
	err := downloadAndUnzipArtifact(updater.mgr, logger, artifact.DownloadInput{}, context, context.Current.TargetVersion)

	// assert
	assert.Error(t, err)
}

// createUpdaterWithStubs creates stubs updater and it's manager, util and service
func createDefaultUpdaterStub() *Updater {
	return createUpdaterStubs(&stubControl{})
}

func createUpdaterStubs(control *stubControl) *Updater {
	updater := NewUpdater()
	updater.mgr.svc = &serviceStub{}
	updater.mgr.util = &utilityStub{controller: control}
	updater.mgr.ctxMgr = &contextMgrStub{}

	return updater
}

type stubControl struct {
	failCreateInstanceContext      bool
	failCreateUpdateDownloadFolder bool
	serviceIsRunning               bool
	failExeCommand                 bool
}

type utilityStub struct {
	updateutil.Utility
	controller *stubControl
}

func (u *utilityStub) CreateInstanceContext(log log.T) (context *updateutil.InstanceContext, err error) {
	if u.controller.failCreateInstanceContext {
		return nil, fmt.Errorf("failed to load context")
	}
	return &updateutil.InstanceContext{
		Region:          "us-east-1",
		Platform:        updateutil.PlatformRedHat,
		PlatformVersion: "6.5",
		InstallerName:   "linux",
		Arch:            "amd64",
		CompressFormat:  "tar.gz",
	}, nil
}

func (u *utilityStub) CreateUpdateDownloadFolder() (folder string, err error) {
	if u.controller.failCreateUpdateDownloadFolder {
		return "", fmt.Errorf("failed to create update download folder")
	}
	return "rootfolder", nil
}

func (u *utilityStub) ExeCommand(log log.T, cmd string, workingDir string, updaterRoot string, stdOut string, stdErr string, isAsync bool) (err error) {
	if u.controller.failExeCommand {
		return fmt.Errorf("cannot run script")
	}
	return nil
}

func (u *utilityStub) SaveUpdatePluginResult(log log.T, updaterRoot string, updateResult *updateutil.UpdatePluginResult) (err error) {
	return nil
}

func (u *utilityStub) IsServiceRunning(log log.T, i *updateutil.InstanceContext) (result bool, err error) {
	if u.controller.serviceIsRunning {
		return true, nil
	}
	return false, nil
}

func (u *utilityStub) WaitForServiceToStart(log log.T, i *updateutil.InstanceContext) (result bool, err error) {
	if u.controller.serviceIsRunning {
		return true, nil
	}
	return false, nil
}
