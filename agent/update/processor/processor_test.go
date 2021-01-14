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

// +build e2e

package processor

import (
	"fmt"
	"strings"
	"testing"

	"github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/aws/amazon-ssm-agent/agent/contracts"
	"github.com/aws/amazon-ssm-agent/agent/fileutil/artifact"
	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/amazon-ssm-agent/agent/updateutil"
	"github.com/stretchr/testify/assert"
)

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

type contextMgrStub struct {
	tempStdOut string
}

func (c *contextMgrStub) saveUpdateContext(log log.T, context *UpdateContext, contextLocation string) (err error) {
	if context.Current.StandardOut == "" {
		return nil
	}
	c.tempStdOut = context.Current.StandardOut
	return nil
}

func (c *contextMgrStub) uploadOutput(log log.T, context *UpdateContext, orchestrationDir string) error {
	return nil
}

func TestStartOrResumeUpdateFromInstalledState(t *testing.T) {
	// setup
	updater := createDefaultUpdaterStub()
	isMethodExecuted := false
	updateContext := createUpdateContext(Installed)
	// mock the verify method
	updater.mgr.verify = func(mgr *updateManager, log log.T, context *UpdateContext, isRollback bool) error {
		isMethodExecuted = true
		return nil
	}
	// action
	updater.StartOrResumeUpdate(logger, updateContext)
	// assert
	assert.True(t, isMethodExecuted)
}

func TestStartOrResumeUpdateFromInitializedState(t *testing.T) {
	// setup
	updater := createDefaultUpdaterStub()
	isMethodExecuted := false
	updateContext := createUpdateContext(Initialized)
	// mock the verify method
	updater.mgr.prepare = func(mgr *updateManager, log log.T, context *UpdateContext) error {
		isMethodExecuted = true
		return nil
	}
	// action
	updater.StartOrResumeUpdate(logger, updateContext)
	// assert
	assert.True(t, isMethodExecuted)
}

func TestStartOrResumeUpdateFromStagedState(t *testing.T) {
	// setup
	updater := createDefaultUpdaterStub()
	isMethodExecuted := false
	updateContext := createUpdateContext(Staged)
	// mock the verify method
	updater.mgr.update = func(mgr *updateManager, log log.T, context *UpdateContext) error {
		isMethodExecuted = true
		return nil
	}
	// action
	updater.StartOrResumeUpdate(logger, updateContext)
	// assert
	assert.True(t, isMethodExecuted)
}

func TestInitializeUpdate(t *testing.T) {
	// setup
	updater := createDefaultUpdaterStub()
	updateContext := createUpdateContext("")

	// action
	updateContext, err := updater.InitializeUpdate(logger, updateContext.Current)

	// assert
	assert.NotEmpty(t, updateContext.Current.StandardOut)
	assert.NotEmpty(t, updateContext.Current.StartDateTime)
	assert.NoError(t, err)
}

func TestPrepareInstallationPackages(t *testing.T) {
	// setup
	updater := createDefaultUpdaterStub()
	updateContext := createUpdateContext(Initialized)
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
	versioncheck = func(context context.T, manifestFilePath string, version string) bool {
		// Don't check the version status in this test
		return true
	}
	// action
	err := prepareInstallationPackages(updater.mgr, logger, updateContext)

	// assert
	assert.NoError(t, err)
	assert.Equal(t, updateContext.Current.State, Staged)
	assert.NotEmpty(t, updateContext.Current.StandardOut)
	assert.Empty(t, updateContext.Histories)
	assert.True(t, isUpdateCalled)
}

func TestPreparePackagesFailCreateInstanceContext(t *testing.T) {
	// setup
	control := &stubControl{failCreateInstanceContext: true}
	updater := createUpdaterStubs(control)
	updateContext := createUpdateContext(Initialized)
	versioncheck = func(context context.T, manifestFilePath string, version string) bool {
		// Don't check the version status in this test
		return true
	}

	// action
	err := prepareInstallationPackages(updater.mgr, logger, updateContext)

	// assert
	assert.NoError(t, err)
	assert.Equal(t, string(updateContext.Current.State), "")
	assert.Equal(t, updateContext.Histories[0].State, Completed)
	assert.Equal(t, updateContext.Histories[0].Result, contracts.ResultStatusFailed)
}

func TestPreparePackagesFailCreateUpdateDownloadFolder(t *testing.T) {
	// setup
	updater := createDefaultUpdaterStub()
	updateContext := createUpdateContext(Initialized)

	// stub download for updater
	updater.mgr.download = func(mgr *updateManager, log log.T, downloadInput artifact.DownloadInput, context *UpdateContext, version string) (err error) {
		return fmt.Errorf("no access")
	}
	versioncheck = func(context context.T, manifestFilePath string, version string) bool {
		// Don't check the version status in this test
		return true
	}

	// action
	err := prepareInstallationPackages(updater.mgr, logger, updateContext)

	// assert
	assert.NoError(t, err)
	assert.Equal(t, string(updateContext.Current.State), "")
	assert.Equal(t, updateContext.Histories[0].State, Completed)
	assert.Equal(t, updateContext.Histories[0].Result, contracts.ResultStatusFailed)
}

func TestPreparePackagesFailDownload(t *testing.T) {
	// setup
	control := &stubControl{failCreateUpdateDownloadFolder: true}
	updater := createUpdaterStubs(control)
	updateContext := createUpdateContext(Initialized)
	versioncheck = func(context context.T, manifestFilePath string, version string) bool {
		// Don't check the version status in this test
		return true
	}

	// action
	err := prepareInstallationPackages(updater.mgr, logger, updateContext)

	// assert
	assert.NoError(t, err)
	assert.Equal(t, string(updateContext.Current.State), "")
	assert.Equal(t, updateContext.Histories[0].State, Completed)
	assert.Equal(t, updateContext.Histories[0].Result, contracts.ResultStatusFailed)
}

func TestPreparePackageFailInvalidVersion(t *testing.T) {
	updater := createDefaultUpdaterStub()
	updateContext := createUpdateContext(Initialized)
	updateContext.Current.ManifestPath = "fake-manifest-path"
	isUpdateCalled := false
	isDownloadCalled := false

	// stub download for updater
	updater.mgr.download = func(mgr *updateManager, log log.T, downloadInput artifact.DownloadInput, updateContext *UpdateContext, version string) (err error) {
		isDownloadCalled = true
		return nil
	}
	// stop at the end of prepareInstallationPackages, do not perform update
	updater.mgr.update = func(mgr *updateManager, log log.T, updateContext *UpdateContext) (err error) {
		isUpdateCalled = true
		return nil
	}

	versioncheck = func(context context.T, manifestFilePath string, version string) bool {
		// test for invalid version
		return false
	}
	// action
	err := prepareInstallationPackages(updater.mgr, logger, updateContext)

	// assert
	assert.Nil(t, err)
	assert.Equal(t, string(updateContext.Current.State), "")
	assert.Equal(t, updateContext.Histories[0].State, Completed)
	assert.Equal(t, updateContext.Histories[0].Result, contracts.ResultStatusSuccess)

	assert.Empty(t, updateContext.Current.StandardOut)
	assert.Equal(t, isDownloadCalled, false)
	assert.Equal(t, isUpdateCalled, false)
}

func TestPreparePackageFailInvalidVersion_WithNoManifestPath(t *testing.T) {
	updater := createDefaultUpdaterStub()
	updateContext := createUpdateContext(Initialized)
	updateContext.Current.ManifestPath = ""
	isUpdateCalled := false
	isDownloadCalled := false

	// stub download for updater
	updater.mgr.download = func(mgr *updateManager, log log.T, downloadInput artifact.DownloadInput, updateContext *UpdateContext, version string) (err error) {
		isDownloadCalled = true
		return nil
	}
	// stop at the end of prepareInstallationPackages, do not perform update
	updater.mgr.update = func(mgr *updateManager, log log.T, updateContext *UpdateContext) (err error) {
		isUpdateCalled = true
		return nil
	}

	versioncheck = func(context context.T, manifestFilePath string, version string) bool {
		// test for invalid version

		return false
	}
	// action
	err := prepareInstallationPackages(updater.mgr, logger, updateContext)

	// assert
	assert.Nil(t, err)
	assert.Equal(t, string(updateContext.Current.State), "")
	assert.Equal(t, updateContext.Histories[0].State, Completed)
	assert.Equal(t, updateContext.Histories[0].Result, contracts.ResultStatusSuccess)

	assert.Empty(t, updateContext.Current.StandardOut)
	assert.Equal(t, isDownloadCalled, false)
	assert.Equal(t, isUpdateCalled, false)
}

func TestValidateUpdateVersion(t *testing.T) {
	updateContext := createUpdateContext(Initialized)
	instanceContext := &updateutil.InstanceInfo{
		Region:          "us-east-1",
		Platform:        updateutil.PlatformRedHat,
		PlatformVersion: "6.5",
		InstallerName:   "linux",
		Arch:            "amd64",
		CompressFormat:  "tar.gz",
	}

	err := validateUpdateVersion(logger, updateContext.Current, instanceContext)

	assert.NoError(t, err)
}

func TestValidateUpdateVersionFailCentOs(t *testing.T) {
	updateContext := createUpdateContext(Initialized)
	updateContext.Current.TargetVersion = "1.0.0.0"
	instanceContext := &updateutil.InstanceInfo{
		Region:          "us-east-1",
		Platform:        updateutil.PlatformCentOS,
		PlatformVersion: "6.5",
		InstallerName:   "linux",
		Arch:            "amd64",
		CompressFormat:  "tar.gz",
	}

	err := validateUpdateVersion(logger, updateContext.Current, instanceContext)

	assert.Error(t, err)
}

func TestProceedUpdate(t *testing.T) {
	// setup
	updater := createDefaultUpdaterStub()
	updateContext := createUpdateContext(Staged)
	isVerifyCalled := false

	// stub install for updater
	updater.mgr.install = func(mgr *updateManager, log log.T, version string, updateContext *UpdateContext) (exitCode updateutil.UpdateScriptExitCode, err error) {
		return exitCode, nil
	}

	updater.mgr.verify = func(mgr *updateManager, log log.T, updateContext *UpdateContext, isRollback bool) (err error) {
		isVerifyCalled = true
		return nil
	}

	// action
	err := proceedUpdate(updater.mgr, logger, updateContext)

	// assert
	assert.NoError(t, err)
	assert.Equal(t, updateContext.Current.State, Installed)
	assert.True(t, isVerifyCalled)
	assert.Empty(t, updateContext.Histories)
}

func TestProceedUpdateWithDowngrade(t *testing.T) {
	// setup
	updater := createDefaultUpdaterStub()
	updateContext := createUpdateContext(Staged)
	updateContext.Current.RequiresUninstall = true
	isVerifyCalled := false
	isUninstallCalled := false

	// stub install for updater
	updater.mgr.install = func(mgr *updateManager, log log.T, version string, updateContext *UpdateContext) (exitCode updateutil.UpdateScriptExitCode, err error) {
		return exitCode, nil
	}
	updater.mgr.uninstall = func(mgr *updateManager, log log.T, version string, updateContext *UpdateContext) (exitCode updateutil.UpdateScriptExitCode, err error) {
		isUninstallCalled = true
		return exitCode, nil
	}
	updater.mgr.verify = func(mgr *updateManager, log log.T, updateContext *UpdateContext, isRollback bool) (err error) {
		isVerifyCalled = true
		return nil
	}

	// action
	err := proceedUpdate(updater.mgr, logger, updateContext)

	// assert
	assert.NoError(t, err)
	assert.True(t, isVerifyCalled)
	assert.True(t, isUninstallCalled)
	assert.Equal(t, updateContext.Current.State, Installed)
	assert.Empty(t, updateContext.Histories)
}

func TestProceedUpdateWithUnsupportedServiceMgrForUpdateInstall(t *testing.T) {
	// setup
	updater := createDefaultUpdaterStub()
	updateContext := createUpdateContext(Staged)
	isInstallCalled := false
	invalidPlatform := "Invalid Platform"
	// stub install for updater
	updater.mgr.install = func(mgr *updateManager, log log.T, version string, updateContext *UpdateContext) (exitCode updateutil.UpdateScriptExitCode, err error) {
		isInstallCalled = true
		return updateutil.ExitCodeUnsupportedPlatform, fmt.Errorf(invalidPlatform)
	}

	// action
	err := proceedUpdate(updater.mgr, logger, updateContext)

	// assert
	assert.NoError(t, err)
	assert.True(t, isInstallCalled)
	assert.Equal(t, updateContext.Histories[0].State, Completed)
	assert.Equal(t, updateContext.Histories[0].Result, contracts.ResultStatusFailed)
	assert.True(t, strings.Contains(updater.mgr.ctxMgr.(*contextMgrStub).tempStdOut, invalidPlatform))
}

func TestProceedUpdateWithUnsupportedServiceMgrForUpdateUninstall(t *testing.T) {
	// setup
	updater := createDefaultUpdaterStub()
	updateContext := createUpdateContext(Staged)
	updateContext.Current.RequiresUninstall = true
	isUnInstallCalled := false
	invalidPlatform := "Invalid Platform"

	// stub install for updater
	updater.mgr.uninstall = func(mgr *updateManager, log log.T, version string, updateContext *UpdateContext) (exitCode updateutil.UpdateScriptExitCode, err error) {
		isUnInstallCalled = true
		return updateutil.ExitCodeUnsupportedPlatform, fmt.Errorf(invalidPlatform)
	}

	// action
	err := proceedUpdate(updater.mgr, logger, updateContext)

	// assert
	assert.NoError(t, err)
	assert.True(t, isUnInstallCalled)
	assert.Equal(t, updateContext.Histories[0].State, Completed)
	assert.Equal(t, updateContext.Histories[0].Result, contracts.ResultStatusFailed)
	assert.True(t, strings.Contains(updater.mgr.ctxMgr.(*contextMgrStub).tempStdOut, invalidPlatform))
}

func TestProceedUpdateWithUnsupportedServiceMgrForRollbackUninstall(t *testing.T) {
	// setup
	control := &stubControl{serviceIsRunning: false}
	updater := createUpdaterStubs(control)
	updateContext := createUpdateContext(Rollback)

	isVerifyCalled, isInstallCalled, isUninstallCalled := false, false, false
	invalidPlatform := "Invalid Platform"

	// stub install for updater
	updater.mgr.install = func(mgr *updateManager, log log.T, version string, updateContext *UpdateContext) (exitCode updateutil.UpdateScriptExitCode, err error) {
		isInstallCalled = true
		return exitCode, nil
	}
	updater.mgr.uninstall = func(mgr *updateManager, log log.T, version string, updateContext *UpdateContext) (exitCode updateutil.UpdateScriptExitCode, err error) {
		isUninstallCalled = true
		return updateutil.ExitCodeUnsupportedPlatform, fmt.Errorf(invalidPlatform)
	}
	updater.mgr.verify = func(mgr *updateManager, log log.T, updateContext *UpdateContext, isRollback bool) (err error) {
		isVerifyCalled = true
		return nil
	}
	// action
	err := rollbackInstallation(updater.mgr, logger, updateContext)

	// assert
	assert.NoError(t, err)
	assert.True(t, isUninstallCalled)
	assert.False(t, isVerifyCalled, isInstallCalled)
	assert.Equal(t, updateContext.Histories[0].State, Completed)
	assert.Equal(t, updateContext.Histories[0].Result, contracts.ResultStatusFailed)
	assert.True(t, strings.Contains(updater.mgr.ctxMgr.(*contextMgrStub).tempStdOut, invalidPlatform))
}

func TestProceedUpdateWithUnsupportedServiceMgrForRollbackInstall(t *testing.T) {
	// setup
	control := &stubControl{serviceIsRunning: false}
	updater := createUpdaterStubs(control)
	updateContext := createUpdateContext(Rollback)
	invalidPlatform := "Invalid Platform"

	isVerifyCalled, isInstallCalled, isUninstallCalled := false, false, false

	// stub install for updater
	updater.mgr.install = func(mgr *updateManager, log log.T, version string, updateContext *UpdateContext) (exitCode updateutil.UpdateScriptExitCode, err error) {
		isInstallCalled = true
		return updateutil.ExitCodeUnsupportedPlatform, fmt.Errorf(invalidPlatform)
	}
	updater.mgr.uninstall = func(mgr *updateManager, log log.T, version string, updateContext *UpdateContext) (exitCode updateutil.UpdateScriptExitCode, err error) {
		isUninstallCalled = true
		return exitCode, nil
	}
	updater.mgr.verify = func(mgr *updateManager, log log.T, updateContext *UpdateContext, isRollback bool) (err error) {
		isVerifyCalled = true
		return nil
	}
	// action
	err := rollbackInstallation(updater.mgr, logger, updateContext)

	// assert
	assert.NoError(t, err)
	assert.True(t, isUninstallCalled, isInstallCalled)
	assert.False(t, isVerifyCalled)
	assert.Equal(t, updateContext.Histories[0].State, Completed)
	assert.Equal(t, updateContext.Histories[0].Result, contracts.ResultStatusFailed)
	assert.True(t, strings.Contains(updater.mgr.ctxMgr.(*contextMgrStub).tempStdOut, invalidPlatform))
}

func TestProceedUpdateWithDowngradeFailUninstall(t *testing.T) {
	// setup
	updater := createDefaultUpdaterStub()
	updateContext := createUpdateContext(Staged)
	updateContext.Current.RequiresUninstall = true
	isVerifyCalled := false
	isUninstallCalled := false

	// stub install for updater
	updater.mgr.install = func(mgr *updateManager, log log.T, version string, updateContext *UpdateContext) (exitCode updateutil.UpdateScriptExitCode, err error) {
		return exitCode, nil
	}
	updater.mgr.uninstall = func(mgr *updateManager, log log.T, version string, updateContext *UpdateContext) (exitCode updateutil.UpdateScriptExitCode, err error) {
		isUninstallCalled = true
		return exitCode, fmt.Errorf("cannot uninstall")
	}
	updater.mgr.verify = func(mgr *updateManager, log log.T, updateContext *UpdateContext, isRollback bool) (err error) {
		isVerifyCalled = true
		return nil
	}

	// action
	err := proceedUpdate(updater.mgr, logger, updateContext)

	// assert
	assert.NoError(t, err)
	assert.False(t, isVerifyCalled)
	assert.True(t, isUninstallCalled)
	assert.Equal(t, string(updateContext.Current.State), "")
	assert.Equal(t, updateContext.Histories[0].State, Completed)
	assert.Equal(t, updateContext.Histories[0].Result, contracts.ResultStatusFailed)
}

func TestProceedUpdateFailInstall(t *testing.T) {
	// setup
	updater := createDefaultUpdaterStub()
	updateContext := createUpdateContext(Staged)
	isRollbackCalled := false

	// stub install for updater
	updater.mgr.install = func(mgr *updateManager, log log.T, version string, updateContext *UpdateContext) (exitCode updateutil.UpdateScriptExitCode, err error) {
		return exitCode, fmt.Errorf("install failed")
	}

	updater.mgr.rollback = func(mgr *updateManager, log log.T, updateContext *UpdateContext) (err error) {
		isRollbackCalled = true
		return nil
	}

	// action
	err := proceedUpdate(updater.mgr, logger, updateContext)

	// assert
	assert.NoError(t, err)
	assert.True(t, isRollbackCalled)
	assert.Equal(t, updateContext.Current.State, Rollback)
	assert.Empty(t, updateContext.Histories)
}

func TestVerifyInstallation(t *testing.T) {
	// setup
	control := &stubControl{serviceIsRunning: true}
	updater := createUpdaterStubs(control)
	updateContext := createUpdateContext(Installed)

	// action
	err := verifyInstallation(updater.mgr, logger, updateContext, false)

	// assert
	assert.NoError(t, err)
	assert.Equal(t, string(updateContext.Current.State), "")
	assert.Equal(t, updateContext.Histories[0].State, Completed)
	assert.Equal(t, updateContext.Histories[0].Result, contracts.ResultStatusSuccess)
}

func TestVerifyInstallationFailedGetInstanceInfo(t *testing.T) {
	// setup
	control := &stubControl{failCreateInstanceContext: true}
	updater := createUpdaterStubs(control)
	updateContext := createUpdateContext(Installed)

	// action
	err := verifyInstallation(updater.mgr, logger, updateContext, false)

	// assert
	assert.NoError(t, err)
	assert.Equal(t, string(updateContext.Current.State), "")
	assert.Equal(t, updateContext.Histories[0].State, Completed)
	assert.Equal(t, updateContext.Histories[0].Result, contracts.ResultStatusFailed)
}

func TestVerifyInstallationCannotStartAgent(t *testing.T) {
	// setup
	control := &stubControl{serviceIsRunning: false}
	updater := createUpdaterStubs(control)
	updateContext := createUpdateContext(Installed)
	expectedVersion := updateContext.Current.TargetVersion
	isRollbackCalled := false

	updater.mgr.rollback = func(mgr *updateManager, log log.T, updateContext *UpdateContext) (err error) {
		isRollbackCalled = true
		return nil
	}

	// action
	err := verifyInstallation(updater.mgr, logger, updateContext, false)

	// assert
	assert.NoError(t, err)
	assert.True(t, isRollbackCalled)
	assert.Equal(t, expectedVersion, control.getWaitForServiceVersion())
	assert.Equal(t, updateContext.Current.State, Rollback)
}

func TestVerifyRollback(t *testing.T) {
	// setup
	control := &stubControl{serviceIsRunning: true}
	updater := createUpdaterStubs(control)
	updateContext := createUpdateContext(RolledBack)
	expectedVersion := updateContext.Current.SourceVersion

	// action
	err := verifyInstallation(updater.mgr, logger, updateContext, true)

	// assert
	assert.NoError(t, err)
	assert.Equal(t, string(updateContext.Current.State), "")
	assert.Equal(t, expectedVersion, control.getWaitForServiceVersion())
	assert.Equal(t, updateContext.Histories[0].State, Completed)
	assert.Equal(t, updateContext.Histories[0].Result, contracts.ResultStatusFailed)
}

func TestVerifyRollbackCannotStartAgent(t *testing.T) {
	// setup
	control := &stubControl{serviceIsRunning: false}
	updater := createUpdaterStubs(control)

	// open network required
	updateContext := createUpdateContext(RolledBack)

	// action
	err := verifyInstallation(updater.mgr, logger, updateContext, true)

	// assert
	assert.NoError(t, err)
	assert.Equal(t, string(updateContext.Current.State), "")
	assert.Equal(t, updateContext.Histories[0].State, Completed)
	assert.Equal(t, updateContext.Histories[0].Result, contracts.ResultStatusFailed)
}

func TestRollbackInstallation(t *testing.T) {
	// setup
	control := &stubControl{serviceIsRunning: false}
	updater := createUpdaterStubs(control)
	updateContext := createUpdateContext(Rollback)

	isVerifyCalled, isInstallCalled, isUninstallCalled := false, false, false

	// stub install for updater
	updater.mgr.install = func(mgr *updateManager, log log.T, version string, updateContext *UpdateContext) (exitCode updateutil.UpdateScriptExitCode, err error) {
		isInstallCalled = true
		return exitCode, nil
	}
	updater.mgr.uninstall = func(mgr *updateManager, log log.T, version string, updateContext *UpdateContext) (exitCode updateutil.UpdateScriptExitCode, err error) {
		isUninstallCalled = true
		return exitCode, nil
	}
	updater.mgr.verify = func(mgr *updateManager, log log.T, updateContext *UpdateContext, isRollback bool) (err error) {
		isVerifyCalled = true
		return nil
	}
	// action
	err := rollbackInstallation(updater.mgr, logger, updateContext)

	// assert
	assert.NoError(t, err)
	assert.True(t, isVerifyCalled, isInstallCalled, isUninstallCalled)
	assert.Equal(t, updateContext.Current.State, RolledBack)
}

func TestRollbackInstallationFailUninstall(t *testing.T) {
	// setup
	control := &stubControl{serviceIsRunning: false}
	updater := createUpdaterStubs(control)
	updateContext := createUpdateContext(Rollback)

	isVerifyCalled, isInstallCalled, isUninstallCalled := false, false, false

	// stub install for updater
	updater.mgr.install = func(mgr *updateManager, log log.T, version string, updateContext *UpdateContext) (exitCode updateutil.UpdateScriptExitCode, err error) {
		isInstallCalled = true
		return exitCode, nil
	}
	updater.mgr.uninstall = func(mgr *updateManager, log log.T, version string, updateContext *UpdateContext) (exitCode updateutil.UpdateScriptExitCode, err error) {
		isUninstallCalled = true
		return exitCode, fmt.Errorf("cannot uninstall")
	}
	updater.mgr.verify = func(mgr *updateManager, log log.T, updateContext *UpdateContext, isRollback bool) (err error) {
		isVerifyCalled = true
		return nil
	}
	// action
	err := rollbackInstallation(updater.mgr, logger, updateContext)

	// assert
	assert.NoError(t, err)
	assert.True(t, isUninstallCalled)
	assert.False(t, isInstallCalled, isVerifyCalled)
	assert.Equal(t, updateContext.Histories[0].State, Completed)
	assert.Equal(t, updateContext.Histories[0].Result, contracts.ResultStatusFailed)
}

func TestRollbackInstallationFailInstall(t *testing.T) {
	// setup
	control := &stubControl{serviceIsRunning: false}
	updater := createUpdaterStubs(control)
	updateContext := createUpdateContext(Rollback)

	isVerifyCalled, isInstallCalled, isUninstallCalled := false, false, false

	// stub install for updater
	updater.mgr.install = func(mgr *updateManager, log log.T, version string, updateContext *UpdateContext) (exitCode updateutil.UpdateScriptExitCode, err error) {
		isInstallCalled = true
		return exitCode, fmt.Errorf("cannot uninstall")
	}
	updater.mgr.uninstall = func(mgr *updateManager, log log.T, version string, updateContext *UpdateContext) (exitCode updateutil.UpdateScriptExitCode, err error) {
		isUninstallCalled = true
		return exitCode, nil
	}
	updater.mgr.verify = func(mgr *updateManager, log log.T, updateContext *UpdateContext, isRollback bool) (err error) {
		isVerifyCalled = true
		return nil
	}
	// action
	err := rollbackInstallation(updater.mgr, logger, updateContext)

	// assert
	assert.NoError(t, err)
	assert.True(t, isUninstallCalled, isInstallCalled)
	assert.False(t, isVerifyCalled)
	assert.Equal(t, updateContext.Histories[0].State, Completed)
	assert.Equal(t, updateContext.Histories[0].Result, contracts.ResultStatusFailed)
}

func TestUninstallAgent(t *testing.T) {
	// setup
	control := &stubControl{failExeCommand: false}
	updater := createUpdaterStubs(control)
	updateContext := createUpdateContext(Initialized)

	// action
	exitCode, err := uninstallAgent(updater.mgr, logger, updateContext.Current.TargetVersion, updateContext)

	// assert
	assert.NoError(t, err)
	assert.Equal(t, 0, int(exitCode))
}

func TestUninstallAgentFailExeCommand(t *testing.T) {
	// setup
	control := &stubControl{failExeCommand: true}
	updater := createUpdaterStubs(control)
	updateContext := createUpdateContext(Initialized)

	// action
	exitCode, err := uninstallAgent(updater.mgr, logger, updateContext.Current.TargetVersion, updateContext)

	// assert
	assert.Error(t, err)
	assert.Equal(t, 0, int(exitCode))
}

func TestInstallAgent(t *testing.T) {
	// setup
	control := &stubControl{failExeCommand: false}
	updater := createUpdaterStubs(control)
	updateContext := createUpdateContext(Initialized)

	// action
	exitCode, err := installAgent(updater.mgr, logger, updateContext.Current.TargetVersion, updateContext)

	// assert
	assert.NoError(t, err)
	assert.Equal(t, 0, int(exitCode))
}

func TestInstallAgentFailExeCommand(t *testing.T) {
	// setup
	control := &stubControl{failExeCommand: true}
	updater := createUpdaterStubs(control)
	updateContext := createUpdateContext(Initialized)

	// action
	exitCode, err := installAgent(updater.mgr, logger, updateContext.Current.TargetVersion, updateContext)

	// assert
	assert.Error(t, err)
	assert.Equal(t, 0, int(exitCode))
}

func TestDownloadAndUnzipArtifact(t *testing.T) {
	// setup
	control := &stubControl{failExeCommand: true}
	updater := createUpdaterStubs(control)
	updateContext := createUpdateContext(Initialized)
	downloadOutput := artifact.DownloadOutput{
		IsHashMatched: true,
		LocalFilePath: "filepath",
	}

	downloadArtifact = func(context context.T, input artifact.DownloadInput) (output artifact.DownloadOutput, err error) {
		return downloadOutput, nil
	}
	uncompress = func(log log.T, src, dest string) error {
		return nil
	}

	// action
	err := downloadAndUnzipArtifact(updater.mgr, logger, artifact.DownloadInput{}, updateContext, updateContext.Current.TargetVersion)

	// assert
	assert.NoError(t, err)
}

func TestDownloadWithError(t *testing.T) {
	// setup
	control := &stubControl{failExeCommand: true}
	updater := createUpdaterStubs(control)
	updateContext := createUpdateContext(Initialized)
	downloadOutput := artifact.DownloadOutput{
		IsHashMatched: false,
		LocalFilePath: "",
	}

	downloadArtifact = func(context context.T, input artifact.DownloadInput) (output artifact.DownloadOutput, err error) {
		return downloadOutput, nil
	}

	// action
	err := downloadAndUnzipArtifact(updater.mgr, logger, artifact.DownloadInput{}, updateContext, updateContext.Current.TargetVersion)

	// assert
	assert.Error(t, err)
}

// createUpdaterWithStubs creates stubs updater and it's manager, util and service
func createDefaultUpdaterStub() *Updater {
	return createUpdaterStubs(&stubControl{})
}

func createUpdaterStubs(control *stubControl) *Updater {
	context := context.NewMockDefault()
	updater := NewUpdater(context)
	updater.mgr.svc = &serviceStub{}
	util := &utilityStub{controller: control}
	util.Context = context
	updater.mgr.util = util
	updater.mgr.ctxMgr = &contextMgrStub{}
	updater.mgr.Context = context

	return updater
}

type stubControl struct {
	failCreateInstanceContext      bool
	failCreateUpdateDownloadFolder bool
	serviceIsRunning               bool
	failExeCommand                 bool
	waitForServiceVersion          string
}

func (s *stubControl) getWaitForServiceVersion() string {
	return s.waitForServiceVersion
}

type utilityStub struct {
	updateutil.Utility
	controller *stubControl
}

func (u *utilityStub) CreateInstanceContext(log log.T) (info *updateutil.InstanceInfo, err error) {
	if u.controller.failCreateInstanceContext {
		return nil, fmt.Errorf("failed to load context")
	}
	return &updateutil.InstanceInfo{
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

func (u *utilityStub) ExeCommand(log log.T, cmd string, workingDir string, updaterRoot string, stdOut string, stdErr string, isAsync bool) (pid int, exitCode updateutil.UpdateScriptExitCode, err error) {
	if u.controller.failExeCommand {
		return -1, exitCode, fmt.Errorf("cannot run script")
	}
	return 1, exitCode, nil
}

func (u *utilityStub) SaveUpdatePluginResult(log log.T, updaterRoot string, updateResult *updateutil.UpdatePluginResult) (err error) {
	return nil
}

func (u *utilityStub) IsServiceRunning(log log.T, i *updateutil.InstanceInfo) (result bool, err error) {
	if u.controller.serviceIsRunning {
		return true, nil
	}
	return false, nil
}

func (u *utilityStub) WaitForServiceToStart(log log.T, i *updateutil.InstanceInfo, targetVersion string) (result bool, err error) {
	u.controller.waitForServiceVersion = targetVersion
	if u.controller.serviceIsRunning {
		return true, nil
	}
	return false, nil
}

func (u *utilityStub) DownloadManifestFile(log log.T, updateDownloadFolder string, manifestUrl string, region string) (*artifact.DownloadOutput, string, error) {

	return &artifact.DownloadOutput{
		LocalFilePath: "testPath",
		IsUpdated:     true,
		IsHashMatched: true,
	}, "manifestUrl", nil
}
