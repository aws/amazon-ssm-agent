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
	"sync"

	"time"

	"github.com/aws/amazon-ssm-agent/agent/fileutil"
	"github.com/aws/amazon-ssm-agent/agent/fileutil/artifact"
	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/amazon-ssm-agent/agent/updateutil"
)

var minimumSupportedVersions map[string]string
var once sync.Once

var (
	downloadArtifact = artifact.Download
	uncompress       = fileutil.Uncompress
)

// NewUpdater creates an instance of Updater and other services it requires
func NewUpdater() *Updater {
	updater := &Updater{
		mgr: &updateManager{
			util:      &updateutil.Utility{},
			svc:       &svcManager{},
			ctxMgr:    &contextManager{},
			prepare:   prepareInstallationPackages,
			update:    proceedUpdate,
			verify:    verifyInstallation,
			rollback:  rollbackInstallation,
			uninstall: uninstallAgent,
			install:   installAgent,
			download:  downloadAndUnzipArtifact,
		},
	}

	return updater
}

// StartOrResumeUpdate starts/resume update.
func (u *Updater) StartOrResumeUpdate(log log.T, context *UpdateContext) (err error) {
	switch {
	case context.Current.State == Initialized:
		return u.mgr.prepare(u.mgr, log, context)
	case context.Current.State == Staged:
		return u.mgr.update(u.mgr, log, context)
	case context.Current.State == Installed:
		return u.mgr.verify(u.mgr, log, context, false)
	case context.Current.State == Rollback:
		return u.mgr.rollback(u.mgr, log, context)
	case context.Current.State == RolledBack:
		return u.mgr.verify(u.mgr, log, context, true)
	}

	return nil
}

// InitializeUpdate initializes update, creates update context
func (u *Updater) InitializeUpdate(log log.T, detail *UpdateDetail) (context *UpdateContext, err error) {
	var pluginResult *updateutil.UpdatePluginResult

	// load plugin update result
	pluginResult, err = updateutil.LoadUpdatePluginResult(log, detail.UpdateRoot)
	if err != nil {
		return nil, fmt.Errorf("update failed, no rollback needed %v", err.Error())
	}
	detail.StandardOut = pluginResult.StandOut
	// if failed to read time from updateplugin file
	if !pluginResult.StartDateTime.Equal(time.Time{}) {
		detail.StartDateTime = pluginResult.StartDateTime
	}

	// Load UpdateContext from local storage, set current update with the new UpdateDetail
	if context, err = LoadUpdateContext(log, updateutil.UpdateContextFilePath(detail.UpdateRoot)); err != nil {
		return context, fmt.Errorf("update failed, no rollback needed %v", err.Error())
	}

	if context.IsUpdateInProgress(log) {
		return context, fmt.Errorf("another update is in progress, please retry later")
	}

	context.Current = detail
	if err = u.mgr.inProgress(context, log, Initialized); err != nil {
		return
	}

	return context, nil
}

// Failed sets update to failed with error messages
func (u *Updater) Failed(context *UpdateContext, log log.T, code updateutil.ErrorCode, errMessage string, noRollbackMessage bool) (err error) {
	return u.mgr.failed(context, log, code, errMessage, noRollbackMessage)
}

// validateUpdateVersion validates target version number base on the current platform
// to avoid accidentally downgrade agent to the earlier version that doesn't support current platform
func validateUpdateVersion(log log.T, detail *UpdateDetail, instanceContext *updateutil.InstanceContext) (err error) {
	compareResult := 0
	minimumVersions := getMinimumVSupportedVersions()

	// check if current platform has minimum supported version
	if val, ok := (*minimumVersions)[instanceContext.Platform]; ok {
		// compare current agent version with minimum supported version
		if compareResult, err = updateutil.VersionCompare(detail.TargetVersion, val); err != nil {
			return err
		}
		if compareResult < 0 {
			return fmt.Errorf("Agent version %v is unsupported on current platform", detail.TargetVersion)
		}
	}

	return nil
}

// getMinimumVSupportedVersions returns a map of minimum supported version and it's platform
func getMinimumVSupportedVersions() (versions *map[string]string) {
	once.Do(func() {
		minimumSupportedVersions = make(map[string]string)
		minimumSupportedVersions[updateutil.PlatformCentOS] = "1.0.187.0"
	})
	return &minimumSupportedVersions
}

// prepareInstallationPackages downloads artifacts from public s3 storage
func prepareInstallationPackages(mgr *updateManager, log log.T, context *UpdateContext) (err error) {
	log.Infof("Initiating download %v", context.Current.PackageName)
	var instanceContext *updateutil.InstanceContext
	updateDownload := ""

	if instanceContext, err = mgr.util.CreateInstanceContext(log); err != nil {
		return mgr.failed(context, log, updateutil.ErrorEnvironmentIssue, err.Error(), false)
	}
	if err = validateUpdateVersion(log, context.Current, instanceContext); err != nil {
		return mgr.failed(context, log, updateutil.ErrorEnvironmentIssue, err.Error(), true)
	}

	if updateDownload, err = mgr.util.CreateUpdateDownloadFolder(); err != nil {
		message := updateutil.BuildMessage(
			err,
			"failed to create download folder %v %v",
			context.Current.PackageName,
			context.Current.TargetVersion)
		return mgr.failed(context, log, updateutil.ErrorEnvironmentIssue, message, true)
	}

	// Download source
	downloadInput := artifact.DownloadInput{
		SourceURL: context.Current.SourceLocation,
		SourceChecksums: map[string]string{
			updateutil.HashType: context.Current.SourceHash,
		},
		DestinationDirectory: updateDownload,
	}

	if err = mgr.download(mgr, log, downloadInput, context, context.Current.SourceVersion); err != nil {
		return mgr.failed(context, log, updateutil.ErrorInvalidPackage, err.Error(), true)
	}

	// Download target
	downloadInput = artifact.DownloadInput{
		SourceURL: context.Current.TargetLocation,
		SourceChecksums: map[string]string{
			updateutil.HashType: context.Current.TargetHash,
		},
		DestinationDirectory: updateDownload,
	}

	if err = mgr.download(mgr, log, downloadInput, context, context.Current.TargetVersion); err != nil {
		return mgr.failed(context, log, updateutil.ErrorInvalidPackage, err.Error(), true)
	}

	// Update stdout
	context.Current.AppendInfo(
		log,
		"Initiating %v update to %v",
		context.Current.PackageName,
		context.Current.TargetVersion)

	// Update state to Staged
	if err = mgr.inProgress(context, log, Staged); err != nil {
		return err
	}

	// Process update
	return mgr.update(mgr, log, context)
}

// proceedUpdate starts update process
func proceedUpdate(mgr *updateManager, log log.T, context *UpdateContext) (err error) {
	log.Infof(
		"Attemping to upgrade from %v to %v",
		context.Current.SourceVersion,
		context.Current.TargetVersion)

	// Uninstall only when the target version is lower than the source version
	if context.Current.RequiresUninstall {
		if err = mgr.uninstall(mgr, log, context.Current.SourceVersion, context); err != nil {
			message := updateutil.BuildMessage(
				err,
				"failed to uninstall %v %v",
				context.Current.PackageName,
				context.Current.SourceVersion)
			return mgr.failed(context, log, updateutil.ErrorUninstallFailed, message, true)
		}
	}

	if err = mgr.install(mgr, log, context.Current.TargetVersion, context); err != nil {
		// Install target failed with err
		// log the error and initiating rollback to the source version
		message := updateutil.BuildMessage(err,
			"failed to install %v %v",
			context.Current.PackageName,
			context.Current.TargetVersion)
		context.Current.AppendError(log, message)

		context.Current.AppendInfo(
			log,
			"Initiating rollback %v to %v",
			context.Current.PackageName,
			context.Current.SourceVersion)
		// Update state to Rollback to indicate updater has initiated the rollback process
		if err = mgr.inProgress(context, log, Rollback); err != nil {
			return err
		}
		// Rollback
		return mgr.rollback(mgr, log, context)
	}

	// Update state to installed to indicate there is no error occur during installation
	// Updater has installed the new version and started the verify process
	if err = mgr.inProgress(context, log, Installed); err != nil {
		return err
	}

	// verify target version
	return mgr.verify(mgr, log, context, false)
}

// verifyInstallation checks installation result, verifies if agent is running
func verifyInstallation(mgr *updateManager, log log.T, context *UpdateContext, isRollback bool) (err error) {
	// Check if agent is running
	var isRunning = false
	var instanceContext *updateutil.InstanceContext

	if instanceContext, err = mgr.util.CreateInstanceContext(log); err != nil {
		return mgr.failed(context, log, updateutil.ErrorEnvironmentIssue, err.Error(), false)
	}

	log.Infof("Initiating update health check")
	if isRunning, err = mgr.util.WaitForServiceToStart(log, instanceContext); err != nil || !isRunning {
		if !isRollback {
			message := updateutil.BuildMessage(err,
				"failed to update %v to %v, %v",
				context.Current.PackageName,
				context.Current.TargetVersion,
				"failed to start the agent")

			context.Current.AppendError(log, message)
			context.Current.AppendInfo(
				log,
				"Initiating rollback %v to %v",
				context.Current.PackageName,
				context.Current.SourceVersion)
			// Update state to rollback
			if err = mgr.inProgress(context, log, Rollback); err != nil {
				return err
			}
			return mgr.rollback(mgr, log, context)
		}

		message := updateutil.BuildMessage(err,
			"failed to rollback %v to %v, %v",
			context.Current.PackageName,
			context.Current.SourceVersion,
			"failed to start the agent")
		// Rolled back, but service cannot start, Update failed.
		return mgr.failed(context, log, updateutil.ErrorCannotStartService, message, false)
	}

	log.Infof("%v is running", context.Current.PackageName)
	if !isRollback {
		return mgr.succeeded(context, log)
	}

	message := fmt.Sprintf("rolledback %v to %v", context.Current.PackageName, context.Current.SourceVersion)
	log.Infof("message is %v", message)
	return mgr.failed(context, log, updateutil.ErrorCannotStartService, message, false)
}

// rollbackInstallation rollback installation to the source version
func rollbackInstallation(mgr *updateManager, log log.T, context *UpdateContext) (err error) {
	if err = mgr.uninstall(mgr, log, context.Current.TargetVersion, context); err != nil {
		// Fail the rollback process as a result of target version cannot be uninstalled
		message := updateutil.BuildMessage(
			err,
			"failed to uninstall %v %v",
			context.Current.PackageName,
			context.Current.TargetVersion)
		return mgr.failed(context, log, updateutil.ErrorUninstallFailed, message, false)
	}

	if err = mgr.install(mgr, log, context.Current.SourceVersion, context); err != nil {
		// Fail the rollback process as a result of source version cannot be reinstalled
		message := updateutil.BuildMessage(
			err,
			"failed to install %v %v",
			context.Current.PackageName,
			context.Current.SourceVersion)
		return mgr.failed(context, log, updateutil.ErrorInstallFailed, message, false)
	}

	if err = mgr.inProgress(context, log, RolledBack); err != nil {
		return err
	}
	return mgr.verify(mgr, log, context, true)
}

// uninstall executes the uninstall script for the specific version of agent
func uninstallAgent(mgr *updateManager, log log.T, version string, context *UpdateContext) (err error) {
	log.Infof("Initiating %v %v uninstallation", context.Current.PackageName, version)

	// find the path for the uninstall script
	uninstallPath := updateutil.UnInstallerFilePath(
		context.Current.UpdateRoot,
		context.Current.PackageName,
		version)

	// calculate work directory
	workDir := updateutil.UpdateArtifactFolder(
		context.Current.UpdateRoot,
		context.Current.PackageName,
		version)

	// Uninstall version
	if err = mgr.util.ExeCommand(
		log,
		uninstallPath,
		workDir,
		context.Current.UpdateRoot,
		context.Current.StdoutFileName,
		context.Current.StderrFileName,
		false); err != nil {
		return err
	}
	log.Infof("%v %v uninstalled successfully", context.Current.PackageName, version)
	return nil
}

// install executes the install script for the specific version of agent
func installAgent(mgr *updateManager, log log.T, version string, context *UpdateContext) (err error) {
	log.Infof("Initiating %v %v installation", context.Current.PackageName, version)

	// find the path for the install script
	installerPath := updateutil.InstallerFilePath(
		context.Current.UpdateRoot,
		context.Current.PackageName,
		version)
	// calculate work directory
	workDir := updateutil.UpdateArtifactFolder(
		context.Current.UpdateRoot,
		context.Current.PackageName,
		version)

	// Install version
	if err = mgr.util.ExeCommand(
		log,
		installerPath,
		workDir,
		context.Current.UpdateRoot,
		context.Current.StdoutFileName,
		context.Current.StderrFileName,
		false); err != nil {

		return err
	}

	log.Infof("%v %v installed successfully", context.Current.PackageName, version)
	return nil
}

// downloadAndUnzipArtifact downloads installation package and unzips it
func downloadAndUnzipArtifact(
	mgr *updateManager,
	log log.T,
	downloadInput artifact.DownloadInput,
	context *UpdateContext,
	version string) (err error) {

	log.Infof("Preparing source for version %v", version)
	// download installation zip files
	downloadOutput, err := downloadArtifact(log, downloadInput)
	if err != nil ||
		downloadOutput.IsHashMatched == false ||
		downloadOutput.LocalFilePath == "" {
		if err != nil {
			return fmt.Errorf("failed to download file reliably, %v, %v", downloadInput.SourceURL, err.Error())
		}
		return fmt.Errorf("failed to download file reliably, %v", downloadInput.SourceURL)
	}

	// downloaded successfully, append message
	context.Current.AppendInfo(log, "Successfully downloaded %v", downloadInput.SourceURL)

	// uncompress installation package
	if err = uncompress(
		log,
		downloadOutput.LocalFilePath,
		updateutil.UpdateArtifactFolder(context.Current.UpdateRoot, context.Current.PackageName, version)); err != nil {
		return fmt.Errorf("failed to uncompress installation package, %v", err.Error())
	}

	return nil
}
