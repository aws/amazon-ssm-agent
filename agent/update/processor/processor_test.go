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

func (c *contextMgrStub) uploadOutput(log log.T, updateDetail *UpdateDetail, orchestrationDir string) error {
	return nil
}

func TestStartOrResumeUpdateFromInstalledState(t *testing.T) {
	// setup
	updater := createDefaultUpdaterStub()
	isMethodExecuted := false
	updateDetail := createUpdateDetail(Installed)
	// mock the verify method
	updater.mgr.verify = func(mgr *updateManager, log log.T, updateDetail *UpdateDetail, isRollback bool) error {
		isMethodExecuted = true
		return nil
	}
	// action
	updater.StartOrResumeUpdate(logger, updateDetail)
	// assert
	assert.True(t, isMethodExecuted)
}

func TestStartOrResumeUpdateFromInitializedState(t *testing.T) {
	// setup
	updater := createDefaultUpdaterStub()
	isMethodExecuted := false
	updateDetail := createUpdateDetail(Initialized)
	// mock the verify method
	updater.mgr.prepare = func(mgr *updateManager, log log.T, updateDetail *UpdateDetail) error {
		isMethodExecuted = true
		return nil
	}
	// action
	updater.StartOrResumeUpdate(logger, updateDetail)
	// assert
	assert.True(t, isMethodExecuted)
}

func TestStartOrResumeUpdateFromStagedState(t *testing.T) {
	// setup
	updater := createDefaultUpdaterStub()
	isMethodExecuted := false
	updateDetail := createUpdateDetail(Staged)
	// mock the verify method
	updater.mgr.update = func(mgr *updateManager, log log.T, updateDetail *UpdateDetail) error {
		isMethodExecuted = true
		return nil
	}
	// action
	updater.StartOrResumeUpdate(logger, updateDetail)
	// assert
	assert.True(t, isMethodExecuted)
}

func TestInitializeUpdate(t *testing.T) {
	// setup
	updater := createDefaultUpdaterStub()
	updateDetail := createUpdateDetail("")

	// action
	err := updater.InitializeUpdate(logger, updateDetail)

	// assert
	assert.NotEmpty(t, updateDetail.StandardOut)
	assert.NotEmpty(t, updateDetail.StartDateTime)
	assert.NoError(t, err)
}

func TestPrepareInstallationPackages(t *testing.T) {
	// setup
	updater := createDefaultUpdaterStub()
	updateDetail := createUpdateDetail(Initialized)
	isUpdateCalled := false

	// stub download for updater
	updater.mgr.download = func(mgr *updateManager, log log.T, downloadInput artifact.DownloadInput, updateDetail *UpdateDetail, version string) (err error) {
		return nil
	}
	// stop at the end of prepareInstallationPackages, do not perform update
	updater.mgr.update = func(mgr *updateManager, log log.T, updateDetail *UpdateDetail) (err error) {
		isUpdateCalled = true
		return nil
	}
	versioncheck = func(context context.T, manifestFilePath string, version string) bool {
		// Don't check the version status in this test
		return true
	}
	// action
	err := prepareInstallationPackages(updater.mgr, logger, updateDetail)

	// assert
	assert.NoError(t, err)
	assert.Equal(t, updateDetail.State, Staged)
	assert.NotEmpty(t, updateDetail.StandardOut)
	assert.True(t, isUpdateCalled)
}

func TestPreparePackagesFailCreateInstanceContext(t *testing.T) {
	// setup
	control := &stubControl{failCreateInstanceContext: true}
	updater := createUpdaterStubs(control)
	updateDetail := createUpdateDetail(Initialized)
	versioncheck = func(context context.T, manifestFilePath string, version string) bool {
		// Don't check the version status in this test
		return true
	}

	// action
	err := prepareInstallationPackages(updater.mgr, logger, updateDetail)

	// assert
	assert.NoError(t, err)
	assert.Equal(t, Completed, updateDetail.State)
}

func TestPreparePackagesFailCreateUpdateDownloadFolder(t *testing.T) {
	// setup
	updater := createDefaultUpdaterStub()
	updateDetail := createUpdateDetail(Initialized)

	// stub download for updater
	updater.mgr.download = func(mgr *updateManager, log log.T, downloadInput artifact.DownloadInput, updateDetail *UpdateDetail, version string) (err error) {
		return fmt.Errorf("no access")
	}
	versioncheck = func(context context.T, manifestFilePath string, version string) bool {
		// Don't check the version status in this test
		return true
	}

	// action
	err := prepareInstallationPackages(updater.mgr, logger, updateDetail)

	// assert
	assert.NoError(t, err)
	assert.Equal(t, Completed, updateDetail.State)
}

func TestPreparePackagesFailDownload(t *testing.T) {
	// setup
	control := &stubControl{failCreateUpdateDownloadFolder: true}
	updater := createUpdaterStubs(control)
	updateDetail := createUpdateDetail(Initialized)
	versioncheck = func(context context.T, manifestFilePath string, version string) bool {
		// Don't check the version status in this test
		return true
	}

	// action
	err := prepareInstallationPackages(updater.mgr, logger, updateDetail)

	// assert
	assert.NoError(t, err)
	assert.Equal(t, Completed, updateDetail.State)
}

func TestPreparePackageFailInvalidVersion(t *testing.T) {
	updater := createDefaultUpdaterStub()
	updateDetail := createUpdateDetail(Initialized)
	updateDetail.ManifestPath = "fake-manifest-path"
	isUpdateCalled := false
	isDownloadCalled := false

	// stub download for updater
	updater.mgr.download = func(mgr *updateManager, log log.T, downloadInput artifact.DownloadInput, updateDetail *UpdateDetail, version string) (err error) {
		isDownloadCalled = true
		return nil
	}
	// stop at the end of prepareInstallationPackages, do not perform update
	updater.mgr.update = func(mgr *updateManager, log log.T, updateDetail *UpdateDetail) (err error) {
		isUpdateCalled = true
		return nil
	}

	versioncheck = func(context context.T, manifestFilePath string, version string) bool {
		// test for invalid version
		return false
	}
	// action
	err := prepareInstallationPackages(updater.mgr, logger, updateDetail)

	// assert
	assert.Nil(t, err)
	assert.Equal(t, Completed, updateDetail.State)

	assert.Empty(t, updateDetail.StandardOut)
	assert.Equal(t, isDownloadCalled, false)
	assert.Equal(t, isUpdateCalled, false)
}

func TestPreparePackageFailInvalidVersion_WithNoManifestPath(t *testing.T) {
	updater := createDefaultUpdaterStub()
	updateDetail := createUpdateDetail(Initialized)
	updateDetail.ManifestPath = ""
	isUpdateCalled := false
	isDownloadCalled := false

	// stub download for updater
	updater.mgr.download = func(mgr *updateManager, log log.T, downloadInput artifact.DownloadInput, updateDetail *UpdateDetail, version string) (err error) {
		isDownloadCalled = true
		return nil
	}
	// stop at the end of prepareInstallationPackages, do not perform update
	updater.mgr.update = func(mgr *updateManager, log log.T, updateDetail *UpdateDetail) (err error) {
		isUpdateCalled = true
		return nil
	}

	versioncheck = func(context context.T, manifestFilePath string, version string) bool {
		// test for invalid version

		return false
	}
	// action
	err := prepareInstallationPackages(updater.mgr, logger, updateDetail)

	// assert
	assert.Nil(t, err)
	assert.Equal(t, Completed, updateDetail.State)

	assert.Empty(t, updateDetail.StandardOut)
	assert.Equal(t, isDownloadCalled, false)
	assert.Equal(t, isUpdateCalled, false)
}

func TestValidateUpdateVersion(t *testing.T) {
	updateDetail := createUpdateDetail(Initialized)
	instanceContext := &updateutil.InstanceInfo{
		Region:          "us-east-1",
		Platform:        updateutil.PlatformRedHat,
		PlatformVersion: "6.5",
		InstallerName:   "linux",
		Arch:            "amd64",
		CompressFormat:  "tar.gz",
	}

	err := validateUpdateVersion(logger, updateDetail, instanceContext)

	assert.NoError(t, err)
}

func TestValidateUpdateVersionFailCentOs(t *testing.T) {
	updateDetail := createUpdateDetail(Initialized)
	updateDetail.TargetVersion = "1.0.0.0"
	instanceContext := &updateutil.InstanceInfo{
		Region:          "us-east-1",
		Platform:        updateutil.PlatformCentOS,
		PlatformVersion: "6.5",
		InstallerName:   "linux",
		Arch:            "amd64",
		CompressFormat:  "tar.gz",
	}

	err := validateUpdateVersion(logger, updateDetail, instanceContext)

	assert.Error(t, err)
}

func TestProceedUpdate(t *testing.T) {
	// setup
	updater := createDefaultUpdaterStub()
	updateDetail := createUpdateDetail(Staged)
	isVerifyCalled := false

	// stub install for updater
	updater.mgr.install = func(mgr *updateManager, log log.T, version string, updateDetail *UpdateDetail) (exitCode updateutil.UpdateScriptExitCode, err error) {
		return exitCode, nil
	}

	updater.mgr.verify = func(mgr *updateManager, log log.T, updateDetail *UpdateDetail, isRollback bool) (err error) {
		isVerifyCalled = true
		return nil
	}

	// action
	err := proceedUpdate(updater.mgr, logger, updateDetail)

	// assert
	assert.NoError(t, err)
	assert.Equal(t, updateDetail.State, Installed)
	assert.True(t, isVerifyCalled)
}

func TestProceedUpdateWithDowngrade(t *testing.T) {
	// setup
	updater := createDefaultUpdaterStub()
	updateDetail := createUpdateDetail(Staged)
	updateDetail.RequiresUninstall = true
	isVerifyCalled := false
	isUninstallCalled := false

	// stub install for updater
	updater.mgr.install = func(mgr *updateManager, log log.T, version string, updateDetail *UpdateDetail) (exitCode updateutil.UpdateScriptExitCode, err error) {
		return exitCode, nil
	}
	updater.mgr.uninstall = func(mgr *updateManager, log log.T, version string, updateDetail *UpdateDetail) (exitCode updateutil.UpdateScriptExitCode, err error) {
		isUninstallCalled = true
		return exitCode, nil
	}
	updater.mgr.verify = func(mgr *updateManager, log log.T, updateDetail *UpdateDetail, isRollback bool) (err error) {
		isVerifyCalled = true
		return nil
	}

	// action
	err := proceedUpdate(updater.mgr, logger, updateDetail)

	// assert
	assert.NoError(t, err)
	assert.True(t, isVerifyCalled)
	assert.True(t, isUninstallCalled)
	assert.Equal(t, updateDetail.State, Installed)
}

func TestProceedUpdateWithUnsupportedServiceMgrForUpdateInstall(t *testing.T) {
	// setup
	updater := createDefaultUpdaterStub()
	updateDetail := createUpdateDetail(Staged)
	isInstallCalled := false
	invalidPlatform := "Invalid Platform"
	// stub install for updater
	updater.mgr.install = func(mgr *updateManager, log log.T, version string, updateDetail *UpdateDetail) (exitCode updateutil.UpdateScriptExitCode, err error) {
		isInstallCalled = true
		return updateutil.ExitCodeUnsupportedPlatform, fmt.Errorf(invalidPlatform)
	}
	updater.mgr.finalize = func(mgr *updateManager, updateDetail *UpdateDetail, errorCode string) (err error) {
		return nil
	}

	// action
	err := proceedUpdate(updater.mgr, logger, updateDetail)

	// assert
	assert.NoError(t, err)
	assert.True(t, isInstallCalled)
	assert.Equal(t, updateDetail.State, Completed)
	assert.Equal(t, updateDetail.Result, contracts.ResultStatusFailed)
	assert.True(t, strings.Contains(updateDetail.StandardOut, invalidPlatform))
}

func TestProceedUpdateWithUnsupportedServiceMgrForUpdateUninstall(t *testing.T) {
	// setup
	updater := createDefaultUpdaterStub()
	updateDetail := createUpdateDetail(Staged)
	updateDetail.RequiresUninstall = true
	isUnInstallCalled := false
	invalidPlatform := "Invalid Platform"

	// stub install for updater
	updater.mgr.uninstall = func(mgr *updateManager, log log.T, version string, updateDetail *UpdateDetail) (exitCode updateutil.UpdateScriptExitCode, err error) {
		isUnInstallCalled = true
		return updateutil.ExitCodeUnsupportedPlatform, fmt.Errorf(invalidPlatform)
	}
	updater.mgr.finalize = func(mgr *updateManager, updateDetail *UpdateDetail, errorCode string) (err error) {
		return nil
	}

	// action
	err := proceedUpdate(updater.mgr, logger, updateDetail)

	// assert
	assert.NoError(t, err)
	assert.True(t, isUnInstallCalled)
	assert.True(t, strings.Contains(updateDetail.StandardOut, invalidPlatform))
}

func TestProceedUpdateWithUnsupportedServiceMgrForRollbackUninstall(t *testing.T) {
	// setup
	control := &stubControl{serviceIsRunning: false}
	updater := createUpdaterStubs(control)
	updateDetail := createUpdateDetail(Rollback)

	isVerifyCalled, isInstallCalled, isUninstallCalled := false, false, false
	invalidPlatform := "Invalid Platform"

	// stub install for updater
	updater.mgr.install = func(mgr *updateManager, log log.T, version string, updateDetail *UpdateDetail) (exitCode updateutil.UpdateScriptExitCode, err error) {
		isInstallCalled = true
		return exitCode, nil
	}
	updater.mgr.uninstall = func(mgr *updateManager, log log.T, version string, updateDetail *UpdateDetail) (exitCode updateutil.UpdateScriptExitCode, err error) {
		isUninstallCalled = true
		return updateutil.ExitCodeUnsupportedPlatform, fmt.Errorf(invalidPlatform)
	}
	updater.mgr.verify = func(mgr *updateManager, log log.T, updateDetail *UpdateDetail, isRollback bool) (err error) {
		isVerifyCalled = true
		return nil
	}
	updater.mgr.finalize = func(mgr *updateManager, updateDetail *UpdateDetail, errorCode string) (err error) {
		return nil
	}

	// action
	err := rollbackInstallation(updater.mgr, logger, updateDetail)

	// assert
	assert.NoError(t, err)
	assert.True(t, isUninstallCalled)
	assert.False(t, isVerifyCalled, isInstallCalled)
	assert.True(t, strings.Contains(updateDetail.StandardOut, invalidPlatform))
}

func TestProceedUpdateWithUnsupportedServiceMgrForRollbackInstall(t *testing.T) {
	// setup
	control := &stubControl{serviceIsRunning: false}
	updater := createUpdaterStubs(control)
	updateDetail := createUpdateDetail(Rollback)
	invalidPlatform := "Invalid Platform"

	isVerifyCalled, isInstallCalled, isUninstallCalled := false, false, false

	// stub install for updater
	updater.mgr.install = func(mgr *updateManager, log log.T, version string, updateDetail *UpdateDetail) (exitCode updateutil.UpdateScriptExitCode, err error) {
		isInstallCalled = true
		return updateutil.ExitCodeUnsupportedPlatform, fmt.Errorf(invalidPlatform)
	}
	updater.mgr.uninstall = func(mgr *updateManager, log log.T, version string, updateDetail *UpdateDetail) (exitCode updateutil.UpdateScriptExitCode, err error) {
		isUninstallCalled = true
		return exitCode, nil
	}
	updater.mgr.verify = func(mgr *updateManager, log log.T, updateDetail *UpdateDetail, isRollback bool) (err error) {
		isVerifyCalled = true
		return nil
	}
	updater.mgr.finalize = func(mgr *updateManager, updateDetail *UpdateDetail, errorCode string) (err error) {
		return nil
	}
	// action
	err := rollbackInstallation(updater.mgr, logger, updateDetail)

	// assert
	assert.NoError(t, err)
	assert.True(t, isUninstallCalled, isInstallCalled)
	assert.False(t, isVerifyCalled)
	assert.True(t, strings.Contains(updateDetail.StandardOut, invalidPlatform))
}

func TestProceedUpdateWithDowngradeFailUninstall(t *testing.T) {
	// setup
	updater := createDefaultUpdaterStub()
	updateDetail := createUpdateDetail(Staged)
	updateDetail.RequiresUninstall = true
	isVerifyCalled := false
	isUninstallCalled := false

	// stub install for updater
	updater.mgr.install = func(mgr *updateManager, log log.T, version string, updateDetail *UpdateDetail) (exitCode updateutil.UpdateScriptExitCode, err error) {
		return exitCode, nil
	}
	updater.mgr.uninstall = func(mgr *updateManager, log log.T, version string, updateDetail *UpdateDetail) (exitCode updateutil.UpdateScriptExitCode, err error) {
		isUninstallCalled = true
		return exitCode, fmt.Errorf("cannot uninstall")
	}
	updater.mgr.verify = func(mgr *updateManager, log log.T, updateDetail *UpdateDetail, isRollback bool) (err error) {
		isVerifyCalled = true
		return nil
	}

	// action
	err := proceedUpdate(updater.mgr, logger, updateDetail)

	// assert
	assert.NoError(t, err)
	assert.False(t, isVerifyCalled)
	assert.True(t, isUninstallCalled)
	assert.Equal(t, Completed, updateDetail.State)
}

func TestProceedUpdateFailInstall(t *testing.T) {
	// setup
	updater := createDefaultUpdaterStub()
	updateDetail := createUpdateDetail(Staged)
	isRollbackCalled := false

	// stub install for updater
	updater.mgr.install = func(mgr *updateManager, log log.T, version string, updateDetail *UpdateDetail) (exitCode updateutil.UpdateScriptExitCode, err error) {
		return exitCode, fmt.Errorf("install failed")
	}

	updater.mgr.rollback = func(mgr *updateManager, log log.T, updateDetail *UpdateDetail) (err error) {
		isRollbackCalled = true
		return nil
	}

	// action
	err := proceedUpdate(updater.mgr, logger, updateDetail)

	// assert
	assert.NoError(t, err)
	assert.True(t, isRollbackCalled)
	assert.Equal(t, updateDetail.State, Rollback)
}

func TestVerifyInstallation(t *testing.T) {
	// setup
	control := &stubControl{serviceIsRunning: true}
	updater := createUpdaterStubs(control)
	updateDetail := createUpdateDetail(Installed)

	// action
	err := verifyInstallation(updater.mgr, logger, updateDetail, false)

	// assert
	assert.NoError(t, err)
	assert.Equal(t, Completed, updateDetail.State)
}

func TestVerifyInstallationFailedGetInstanceInfo(t *testing.T) {
	// setup
	control := &stubControl{failCreateInstanceContext: true}
	updater := createUpdaterStubs(control)
	updateDetail := createUpdateDetail(Installed)

	// action
	err := verifyInstallation(updater.mgr, logger, updateDetail, false)

	// assert
	assert.NoError(t, err)
	assert.Equal(t, Completed, updateDetail.State)
}

func TestVerifyInstallationCannotStartAgent(t *testing.T) {
	// setup
	control := &stubControl{serviceIsRunning: false}
	updater := createUpdaterStubs(control)
	updateDetail := createUpdateDetail(Installed)
	expectedVersion := updateDetail.TargetVersion
	isRollbackCalled := false

	updater.mgr.rollback = func(mgr *updateManager, log log.T, updateDetail *UpdateDetail) (err error) {
		isRollbackCalled = true
		return nil
	}

	// action
	err := verifyInstallation(updater.mgr, logger, updateDetail, false)

	// assert
	assert.NoError(t, err)
	assert.True(t, isRollbackCalled)
	assert.Equal(t, expectedVersion, control.getWaitForServiceVersion())
	assert.Equal(t, updateDetail.State, Rollback)
}

func TestVerifyRollback(t *testing.T) {
	// setup
	control := &stubControl{serviceIsRunning: true}
	updater := createUpdaterStubs(control)
	updateDetail := createUpdateDetail(RolledBack)
	expectedVersion := updateDetail.SourceVersion

	// action
	err := verifyInstallation(updater.mgr, logger, updateDetail, true)

	// assert
	assert.NoError(t, err)
	assert.Equal(t, Completed, updateDetail.State)
	assert.Equal(t, expectedVersion, control.getWaitForServiceVersion())
}

func TestVerifyRollbackCannotStartAgent(t *testing.T) {
	// setup
	control := &stubControl{serviceIsRunning: false}
	updater := createUpdaterStubs(control)

	// open network required
	updateDetail := createUpdateDetail(RolledBack)

	// action
	err := verifyInstallation(updater.mgr, logger, updateDetail, true)

	// assert
	assert.NoError(t, err)
	assert.Equal(t, Completed, updateDetail.State)
}

func TestRollbackInstallation(t *testing.T) {
	// setup
	control := &stubControl{serviceIsRunning: false}
	updater := createUpdaterStubs(control)
	updateDetail := createUpdateDetail(Rollback)

	isVerifyCalled, isInstallCalled, isUninstallCalled := false, false, false

	// stub install for updater
	updater.mgr.install = func(mgr *updateManager, log log.T, version string, updateDetail *UpdateDetail) (exitCode updateutil.UpdateScriptExitCode, err error) {
		isInstallCalled = true
		return exitCode, nil
	}
	updater.mgr.uninstall = func(mgr *updateManager, log log.T, version string, updateDetail *UpdateDetail) (exitCode updateutil.UpdateScriptExitCode, err error) {
		isUninstallCalled = true
		return exitCode, nil
	}
	updater.mgr.verify = func(mgr *updateManager, log log.T, updateDetail *UpdateDetail, isRollback bool) (err error) {
		isVerifyCalled = true
		return nil
	}
	// action
	err := rollbackInstallation(updater.mgr, logger, updateDetail)

	// assert
	assert.NoError(t, err)
	assert.True(t, isVerifyCalled, isInstallCalled, isUninstallCalled)
	assert.Equal(t, updateDetail.State, RolledBack)
}

func TestRollbackInstallationFailUninstall(t *testing.T) {
	// setup
	control := &stubControl{serviceIsRunning: false}
	updater := createUpdaterStubs(control)
	updateDetail := createUpdateDetail(Rollback)

	isVerifyCalled, isInstallCalled, isUninstallCalled := false, false, false

	// stub install for updater
	updater.mgr.install = func(mgr *updateManager, log log.T, version string, updateDetail *UpdateDetail) (exitCode updateutil.UpdateScriptExitCode, err error) {
		isInstallCalled = true
		return exitCode, nil
	}
	updater.mgr.uninstall = func(mgr *updateManager, log log.T, version string, updateDetail *UpdateDetail) (exitCode updateutil.UpdateScriptExitCode, err error) {
		isUninstallCalled = true
		return exitCode, fmt.Errorf("cannot uninstall")
	}
	updater.mgr.verify = func(mgr *updateManager, log log.T, updateDetail *UpdateDetail, isRollback bool) (err error) {
		isVerifyCalled = true
		return nil
	}
	// action
	err := rollbackInstallation(updater.mgr, logger, updateDetail)

	// assert
	assert.NoError(t, err)
	assert.True(t, isUninstallCalled)
	assert.False(t, isInstallCalled, isVerifyCalled)
}

func TestRollbackInstallationFailInstall(t *testing.T) {
	// setup
	control := &stubControl{serviceIsRunning: false}
	updater := createUpdaterStubs(control)
	updateDetail := createUpdateDetail(Rollback)

	isVerifyCalled, isInstallCalled, isUninstallCalled := false, false, false

	// stub install for updater
	updater.mgr.install = func(mgr *updateManager, log log.T, version string, updateDetail *UpdateDetail) (exitCode updateutil.UpdateScriptExitCode, err error) {
		isInstallCalled = true
		return exitCode, fmt.Errorf("cannot uninstall")
	}
	updater.mgr.uninstall = func(mgr *updateManager, log log.T, version string, updateDetail *UpdateDetail) (exitCode updateutil.UpdateScriptExitCode, err error) {
		isUninstallCalled = true
		return exitCode, nil
	}
	updater.mgr.verify = func(mgr *updateManager, log log.T, updateDetail *UpdateDetail, isRollback bool) (err error) {
		isVerifyCalled = true
		return nil
	}
	// action
	err := rollbackInstallation(updater.mgr, logger, updateDetail)

	// assert
	assert.NoError(t, err)
	assert.True(t, isUninstallCalled, isInstallCalled)
	assert.False(t, isVerifyCalled)
}

func TestUninstallAgent(t *testing.T) {
	// setup
	control := &stubControl{failExeCommand: false}
	updater := createUpdaterStubs(control)
	updateDetail := createUpdateDetail(Initialized)

	// action
	exitCode, err := uninstallAgent(updater.mgr, logger, updateDetail.TargetVersion, updateDetail)

	// assert
	assert.NoError(t, err)
	assert.Equal(t, 0, int(exitCode))
}

func TestUninstallAgentFailExeCommand(t *testing.T) {
	// setup
	control := &stubControl{failExeCommand: true}
	updater := createUpdaterStubs(control)
	updateDetail := createUpdateDetail(Initialized)

	// action
	exitCode, err := uninstallAgent(updater.mgr, logger, updateDetail.TargetVersion, updateDetail)

	// assert
	assert.Error(t, err)
	assert.Equal(t, 0, int(exitCode))
}

func TestInstallAgent(t *testing.T) {
	// setup
	control := &stubControl{failExeCommand: false}
	updater := createUpdaterStubs(control)
	updateDetail := createUpdateDetail(Initialized)

	// action
	exitCode, err := installAgent(updater.mgr, logger, updateDetail.TargetVersion, updateDetail)

	// assert
	assert.NoError(t, err)
	assert.Equal(t, 0, int(exitCode))
}

func TestInstallAgentFailExeCommand(t *testing.T) {
	// setup
	control := &stubControl{failExeCommand: true}
	updater := createUpdaterStubs(control)
	updateDetail := createUpdateDetail(Initialized)

	// action
	exitCode, err := installAgent(updater.mgr, logger, updateDetail.TargetVersion, updateDetail)

	// assert
	assert.Error(t, err)
	assert.Equal(t, 0, int(exitCode))
}

func TestDownloadAndUnzipArtifact(t *testing.T) {
	// setup
	control := &stubControl{failExeCommand: true}
	updater := createUpdaterStubs(control)
	updateDetail := createUpdateDetail(Initialized)
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
	err := downloadAndUnzipArtifact(updater.mgr, logger, artifact.DownloadInput{}, updateDetail, updateDetail.TargetVersion)

	// assert
	assert.NoError(t, err)
}

func TestDownloadWithError(t *testing.T) {
	// setup
	control := &stubControl{failExeCommand: true}
	updater := createUpdaterStubs(control)
	updateDetail := createUpdateDetail(Initialized)
	downloadOutput := artifact.DownloadOutput{
		IsHashMatched: false,
		LocalFilePath: "",
	}

	downloadArtifact = func(context context.T, input artifact.DownloadInput) (output artifact.DownloadOutput, err error) {
		return downloadOutput, nil
	}

	// action
	err := downloadAndUnzipArtifact(updater.mgr, logger, artifact.DownloadInput{}, updateDetail, updateDetail.TargetVersion)

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
