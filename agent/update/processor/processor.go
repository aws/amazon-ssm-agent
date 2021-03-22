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
	"math/rand"
	"sync"
	"time"

	"github.com/aws/amazon-ssm-agent/agent/appconfig"
	"github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/aws/amazon-ssm-agent/agent/contracts"
	"github.com/aws/amazon-ssm-agent/agent/fileutil"
	"github.com/aws/amazon-ssm-agent/agent/fileutil/artifact"
	"github.com/aws/amazon-ssm-agent/agent/log"
	testerPkg "github.com/aws/amazon-ssm-agent/agent/update/tester"
	testerCommon "github.com/aws/amazon-ssm-agent/agent/update/tester/common"
	"github.com/aws/amazon-ssm-agent/agent/updateutil"
	"github.com/aws/amazon-ssm-agent/agent/updateutil/updateconstants"
	"github.com/aws/amazon-ssm-agent/agent/updateutil/updateinfo"
	"github.com/aws/amazon-ssm-agent/agent/versionutil"
)

var minimumSupportedVersions map[string]string
var once sync.Once

var (
	downloadArtifact = artifact.Download
	uncompress       = fileutil.Uncompress
	versioncheck     = updateutil.ValidateVersion
)

// NewUpdater creates an instance of Updater and other services it requires
func NewUpdater(context context.T, info updateinfo.T) *Updater {
	updater := &Updater{
		mgr: &updateManager{
			Context: context,
			Info:    info,
			util: &updateutil.Utility{
				Context: context,
			},
			svc: &svcManager{
				context: context,
			},
			ctxMgr: &contextManager{
				context: context,
			},
			prepare:   prepareInstallationPackages,
			update:    proceedUpdate,
			verify:    verifyInstallation,
			rollback:  rollbackInstallation,
			uninstall: uninstallAgent,
			install:   installAgent,
			download:  downloadAndUnzipArtifact,
			clean:     cleanUninstalledVersions,
			runTests:  testerPkg.StartTests,
			finalize:  finalizeUpdateAndSendReply,
		},
	}

	return updater
}

// StartOrResumeUpdate starts/resume update.
func (u *Updater) StartOrResumeUpdate(log log.T, updateDetail *UpdateDetail) (err error) {
	switch {
	case updateDetail.State == Initialized:
		return u.mgr.prepare(u.mgr, log, updateDetail)
	case updateDetail.State == Staged:
		return u.mgr.update(u.mgr, log, updateDetail)
	case updateDetail.State == Installed:
		return u.mgr.verify(u.mgr, log, updateDetail, false)
	case updateDetail.State == Rollback:
		return u.mgr.rollback(u.mgr, log, updateDetail)
	case updateDetail.State == RolledBack:
		return u.mgr.verify(u.mgr, log, updateDetail, true)
	}

	return nil
}

// InitializeUpdate initializes update, populates update detail
func (u *Updater) InitializeUpdate(log log.T, updateDetail *UpdateDetail) (err error) {
	var pluginResult *updateutil.UpdatePluginResult

	// load plugin update result
	pluginResult, err = updateutil.LoadUpdatePluginResult(log, updateDetail.UpdateRoot)
	if err != nil {
		// TODO: Check old path
		return fmt.Errorf("update failed, no rollback needed %v", err.Error())
	}
	updateDetail.StandardOut = pluginResult.StandOut
	// if failed to read time from updateplugin file
	if !pluginResult.StartDateTime.Equal(time.Time{}) {
		updateDetail.StartDateTime = pluginResult.StartDateTime
	}

	if err = u.mgr.inProgress(updateDetail, log, Initialized); err != nil {
		return
	}

	return nil
}

// Failed sets update to failed with error messages
func (u *Updater) Failed(updateDetail *UpdateDetail, log log.T, code updateconstants.ErrorCode, errMessage string, noRollbackMessage bool) (err error) {
	return u.mgr.failed(updateDetail, log, code, errMessage, noRollbackMessage)
}

// validateUpdateVersion validates target version number base on the current platform
// to avoid accidentally downgrade agent to the earlier version that doesn't support current platform
func validateUpdateVersion(log log.T, detail *UpdateDetail, info updateinfo.T) (err error) {
	compareResult := 0
	minimumVersions := getMinimumVSupportedVersions()

	// check if current platform has minimum supported version
	if val, ok := (*minimumVersions)[info.GetPlatform()]; ok {
		// compare current agent version with minimum supported version
		if compareResult, err = versionutil.VersionCompare(detail.TargetVersion, val); err != nil {
			return err
		}
		if compareResult < 0 {
			return fmt.Errorf("agent version %v is unsupported on current platform", detail.TargetVersion)
		}
	}

	return nil
}

func validateInactiveVersion(context context.T, info updateinfo.T, detail *UpdateDetail) (err error) {
	context.Log().Info("Validating inactive version for amazon ssm agent")
	if !versioncheck(context, info, detail.ManifestPath, detail.TargetVersion) {
		err := fmt.Errorf("agent version %v is inactive", detail.TargetVersion)
		return err
	}

	if detail.TargetVersion == "2.3.772.0" {
		err := fmt.Errorf("agent version %v is inactive", detail.TargetVersion)
		return err
	}

	return nil
}

// getMinimumVSupportedVersions returns a map of minimum supported version and it's platform
func getMinimumVSupportedVersions() (versions *map[string]string) {
	once.Do(func() {
		minimumSupportedVersions = make(map[string]string)
		minimumSupportedVersions[updateconstants.PlatformCentOS] = "1.0.187.0"
	})
	return &minimumSupportedVersions
}

// prepareInstallationPackages downloads artifacts from public s3 storage
func prepareInstallationPackages(mgr *updateManager, log log.T, updateDetail *UpdateDetail) (err error) {
	log.Infof("Initiating download %v", updateDetail.PackageName)
	var region string

	var manifestDownloadOutput *artifact.DownloadOutput
	updateDownloadFolder := ""

	updateDownload := ""

	if region, err = mgr.Context.Identity().Region(); err != nil {
		return mgr.failed(updateDetail, log, updateconstants.ErrorEnvironmentIssue, err.Error(), false)
	}

	if err = validateUpdateVersion(log, updateDetail, mgr.Info); err != nil {
		return mgr.failed(updateDetail, log, updateconstants.ErrorUnsupportedVersion, err.Error(), true)
	}

	if updateDownload, err = mgr.util.CreateUpdateDownloadFolder(); err != nil {
		message := updateutil.BuildMessage(
			err,
			"failed to create download folder %v %v",
			updateDetail.PackageName,
			updateDetail.TargetVersion)
		return mgr.failed(updateDetail, log, updateconstants.ErrorCreateUpdateFolder, message, true)
	}

	if updateDetail.ManifestPath == "" {
		if manifestDownloadOutput, updateDetail.ManifestUrl, err =
			mgr.util.DownloadManifestFile(log, updateDownloadFolder, updateDetail.ManifestUrl, region); err != nil {

			message := updateutil.BuildMessage(err, "failed to download manifest file")
			return mgr.failed(updateDetail, log, updateconstants.ErrorInvalidManifest, message, true)
		}
		updateDetail.ManifestPath = manifestDownloadOutput.LocalFilePath
	}

	if err = validateInactiveVersion(mgr.Context, mgr.Info, updateDetail); err != nil {
		return mgr.inactive(updateDetail, log, updateconstants.WarnInactiveVersion)
	}

	// Download source
	downloadInput := artifact.DownloadInput{
		SourceURL: updateDetail.SourceLocation,
		SourceChecksums: map[string]string{
			updateconstants.HashType: updateDetail.SourceHash,
		},
		DestinationDirectory: updateDownload,
	}
	if err = mgr.download(mgr, log, downloadInput, updateDetail, updateDetail.SourceVersion); err != nil {
		return mgr.failed(updateDetail, log, updateconstants.ErrorSourcePkgDownload, err.Error(), true)
	}
	// Download target
	downloadInput = artifact.DownloadInput{
		SourceURL: updateDetail.TargetLocation,
		SourceChecksums: map[string]string{
			updateconstants.HashType: updateDetail.TargetHash,
		},
		DestinationDirectory: updateDownload,
	}
	if err = mgr.download(mgr, log, downloadInput, updateDetail, updateDetail.TargetVersion); err != nil {
		return mgr.failed(updateDetail, log, updateconstants.ErrorTargetPkgDownload, err.Error(), true)
	}
	// Update stdout
	updateDetail.AppendInfo(
		log,
		"Initiating %v update to %v",
		updateDetail.PackageName,
		updateDetail.TargetVersion)

	// Update state to Staged
	if err = mgr.inProgress(updateDetail, log, Staged); err != nil {
		return err
	}

	// Process update
	return mgr.update(mgr, log, updateDetail)
}

// proceedUpdate starts update process
func proceedUpdate(mgr *updateManager, log log.T, updateDetail *UpdateDetail) (err error) {
	log.Infof(
		"Attemping to upgrade from %v to %v",
		updateDetail.SourceVersion,
		updateDetail.TargetVersion)

	// Uninstall only when the target version is lower than the source version
	if updateDetail.RequiresUninstall {
		if exitCode, err := mgr.uninstall(mgr, log, updateDetail.SourceVersion, updateDetail); err != nil {
			message := updateutil.BuildMessage(
				err,
				"failed to uninstall %v %v",
				updateDetail.PackageName,
				updateDetail.SourceVersion)
			mgr.subStatus = updateconstants.Downgrade
			if exitCode == updateconstants.ExitCodeUnsupportedPlatform {
				return mgr.failed(updateDetail, log, updateconstants.ErrorUnsupportedServiceManager, message, true)
			}
			return mgr.failed(updateDetail, log, updateconstants.ErrorUninstallFailed, message, true)
		}
	}
	preInstallTestTimeoutSeconds := 7
	if testResult := mgr.runTests(mgr.Context, testerCommon.PreInstallTest, preInstallTestTimeoutSeconds); testResult != "" {
		mgr.reportTestFailure(updateDetail, log, testResult)
	}
	if exitCode, err := mgr.install(mgr, log, updateDetail.TargetVersion, updateDetail); err != nil {
		// Install target failed with err
		// log the error and initiating rollback to the source version
		message := updateutil.BuildMessage(err,
			"failed to install %v %v",
			updateDetail.PackageName,
			updateDetail.TargetVersion)
		updateDetail.AppendError(log, message)

		if exitCode == updateconstants.ExitCodeUnsupportedPlatform {
			return mgr.failed(updateDetail, log, updateconstants.ErrorUnsupportedServiceManager, message, true)
		}

		updateDetail.AppendInfo(
			log,
			"Initiating rollback %v to %v",
			updateDetail.PackageName,
			updateDetail.SourceVersion)
		mgr.subStatus = updateconstants.InstallRollback
		// Update state to Rollback to indicate updater has initiated the rollback process
		if err = mgr.inProgress(updateDetail, log, Rollback); err != nil {
			return err
		}
		// Rollback
		return mgr.rollback(mgr, log, updateDetail)
	}

	// Update state to installed to indicate there is no error occur during installation
	// Updater has installed the new version and started the verify process
	if err = mgr.inProgress(updateDetail, log, Installed); err != nil {
		return err
	}

	// verify target version
	return mgr.verify(mgr, log, updateDetail, false)
}

// verifyInstallation checks installation result, verifies if agent is running
func verifyInstallation(mgr *updateManager, log log.T, updateDetail *UpdateDetail, isRollback bool) (err error) {
	// Check if agent is running
	var isRunning = false

	version := updateDetail.TargetVersion
	if isRollback {
		version = updateDetail.SourceVersion
	}
	log.Infof("Initiating update health check")
	if isRunning, err = mgr.util.WaitForServiceToStart(log, mgr.Info, version); err != nil || !isRunning {
		if !isRollback {
			message := updateutil.BuildMessage(err,
				"failed to update %v to %v, %v",
				updateDetail.PackageName,
				updateDetail.TargetVersion,
				"failed to start the agent")

			updateDetail.AppendError(log, message)
			updateDetail.AppendInfo(
				log,
				"Initiating rollback %v to %v",
				updateDetail.PackageName,
				updateDetail.SourceVersion)
			mgr.subStatus = updateconstants.VerificationRollback
			// Update state to rollback
			if err = mgr.inProgress(updateDetail, log, Rollback); err != nil {
				return err
			}
			return mgr.rollback(mgr, log, updateDetail)
		}

		message := updateutil.BuildMessage(err,
			"failed to rollback %v to %v, %v",
			updateDetail.PackageName,
			updateDetail.SourceVersion,
			"failed to start the agent")
		// Rolled back, but service cannot start, Update failed.
		return mgr.failed(updateDetail, log, updateconstants.ErrorCannotStartService, message, false)
	}

	log.Infof("%v is running", updateDetail.PackageName)
	if !isRollback {
		return mgr.succeeded(updateDetail, log)
	}

	message := fmt.Sprintf("rolledback %v to %v", updateDetail.PackageName, updateDetail.SourceVersion)
	log.Infof("message is %v", message)
	return mgr.failed(updateDetail, log, updateconstants.ErrorUpdateFailRollbackSuccess, message, false)
}

// rollbackInstallation rollback installation to the source version
func rollbackInstallation(mgr *updateManager, log log.T, updateDetail *UpdateDetail) (err error) {
	if exitCode, err := mgr.uninstall(mgr, log, updateDetail.TargetVersion, updateDetail); err != nil {
		// Fail the rollback process as a result of target version cannot be uninstalled
		message := updateutil.BuildMessage(
			err,
			"failed to uninstall %v %v",
			updateDetail.PackageName,
			updateDetail.TargetVersion)

		// this case is not possible at all as we would have caught it in the earlier uninstall/install
		// if this happens, something else is wrong so it is better to have this code for differentiation
		if exitCode == updateconstants.ExitCodeUnsupportedPlatform {
			return mgr.failed(updateDetail, log, updateconstants.ErrorUnsupportedServiceManager, message, true)
		}
		return mgr.failed(updateDetail, log, updateconstants.ErrorUninstallFailed, message, false)
	}

	if exitCode, err := mgr.install(mgr, log, updateDetail.SourceVersion, updateDetail); err != nil {
		// Fail the rollback process as a result of source version cannot be reinstalled
		message := updateutil.BuildMessage(
			err,
			"failed to install %v %v",
			updateDetail.PackageName,
			updateDetail.SourceVersion)

		// this case is not possible at all as we would have caught it in the earlier uninstall/install
		// if this happens, something else is wrong and it is better to have this code for differentiation
		if exitCode == updateconstants.ExitCodeUnsupportedPlatform {
			return mgr.failed(updateDetail, log, updateconstants.ErrorUnsupportedServiceManager, message, true)
		}
		return mgr.failed(updateDetail, log, updateconstants.ErrorInstallFailed, message, false)
	}

	if err = mgr.inProgress(updateDetail, log, RolledBack); err != nil {
		return err
	}
	return mgr.verify(mgr, log, updateDetail, true)
}

// uninstall executes the uninstall script for the specific version of agent
func uninstallAgent(mgr *updateManager, log log.T, version string, updateDetail *UpdateDetail) (exitCode updateconstants.UpdateScriptExitCode, err error) {
	log.Infof("Initiating %v %v uninstallation", updateDetail.PackageName, version)

	// find the path for the uninstall script
	uninstallPath := updateutil.UnInstallerFilePath(
		updateDetail.UpdateRoot,
		updateDetail.PackageName,
		version,
		mgr.Info.GetUnInstaller())

	// calculate work directory
	workDir := updateutil.UpdateArtifactFolder(
		updateDetail.UpdateRoot,
		updateDetail.PackageName,
		version)

	uninstallRetryCount := 3
	uninstallRetryDelay := 1000     // 1 second
	uninstallRetryDelayBase := 2000 // 2 seconds
	// Uninstall version - TODO - move the retry logic to ExeCommand while cleaning that function
	for retryCounter := 1; retryCounter <= uninstallRetryCount; retryCounter++ {
		_, exitCode, err = mgr.util.ExeCommand(
			log,
			uninstallPath,
			workDir,
			updateDetail.UpdateRoot,
			updateDetail.StdoutFileName,
			updateDetail.StderrFileName,
			false)
		if err == nil {
			break
		}
		if retryCounter < uninstallRetryCount {
			time.Sleep(time.Duration(uninstallRetryDelayBase+rand.Intn(uninstallRetryDelay)) * time.Millisecond)
		}
	}
	if err != nil {
		return exitCode, err
	}
	log.Infof("%v %v uninstalled successfully", updateDetail.PackageName, version)
	return exitCode, nil
}

// install executes the install script for the specific version of agent
func installAgent(mgr *updateManager, log log.T, version string, updateDetail *UpdateDetail) (exitCode updateconstants.UpdateScriptExitCode, err error) {
	log.Infof("Initiating %v %v installation", updateDetail.PackageName, version)

	// find the path for the install script
	installerPath := updateutil.InstallerFilePath(
		updateDetail.UpdateRoot,
		updateDetail.PackageName,
		version,
		mgr.Info.GetInstaller())
	// calculate work directory
	workDir := updateutil.UpdateArtifactFolder(
		updateDetail.UpdateRoot,
		updateDetail.PackageName,
		version)

	// Install version - TODO - move the retry logic to ExeCommand while cleaning that function
	installRetryCount := 3
	if updateDetail.State == Staged {
		installRetryCount = 4 // this value is taken because previous updater version had total 4 retries (2 target install + 2 rollback install)
	}
	for retryCounter := 1; retryCounter <= installRetryCount; retryCounter++ {
		_, exitCode, err = mgr.util.ExeCommand(
			log,
			installerPath,
			workDir,
			updateDetail.UpdateRoot,
			updateDetail.StdoutFileName,
			updateDetail.StderrFileName,
			false)
		if err == nil {
			break
		}
		if retryCounter < installRetryCount {
			backOff := getNextBackOff(retryCounter)

			// Increase backoff by 30 seconds if package manager fails
			if exitCode == updateconstants.ExitCodeUpdateUsingPkgMgr {
				backOff += time.Duration(30) * time.Second // 30 seconds
			}

			time.Sleep(backOff)
		}
	}
	if err != nil {
		return exitCode, err
	}
	log.Infof("%v %v installed successfully", updateDetail.PackageName, version)
	return exitCode, nil
}

// getNextBackOff gets back-off in milli-seconds based on retry counter
// (4*retryCounter) ms + 2000 ms random delay
func getNextBackOff(retryCounter int) (backOff time.Duration) {
	backOffMultiplier := 4
	maxBackOffSeconds := 20 // 20 seconds
	randomDelay := 2000     // 2 seconds

	backOffSeconds := backOffMultiplier * retryCounter
	if backOffSeconds > maxBackOffSeconds {
		backOffSeconds = maxBackOffSeconds
	}
	return time.Duration((backOffSeconds*1000)+rand.Intn(randomDelay)) * time.Millisecond
}

// cleanUninstalledVersions deletes leftover files from previously installed versions in update folder
func cleanUninstalledVersions(mgr *updateManager, log log.T, updateDetail *UpdateDetail) (err error) {
	log.Infof("Initiating cleanup of other versions.")
	var installedVersion string

	switch updateDetail.Result {
	case contracts.ResultStatusSuccess:
		installedVersion = updateDetail.TargetVersion
	case contracts.ResultStatusFailed:
		installedVersion = updateDetail.SourceVersion
	}

	path := appconfig.UpdaterArtifactsRoot + updateconstants.UpdateAmazonSSMAgentDir
	directoryNames, err := fileutil.GetDirectoryNames(path)

	if err != nil {
		return err
	}

	removedVersions := ""
	combinedErrors := fmt.Errorf("")
	for _, directoryName := range directoryNames {
		if directoryName == installedVersion {
			continue
		} else if err = fileutil.DeleteDirectory(path + directoryName); err != nil {
			combinedErrors = fmt.Errorf(combinedErrors.Error() + err.Error() + "\n")
		} else {
			removedVersions += directoryName + "\n"
		}
	}

	log.Infof("Installed version: %v", installedVersion)
	log.Infof("Removed versions: %v", removedVersions)

	if combinedErrors.Error() != "" {
		return combinedErrors
	}

	return nil
}

// downloadAndUnzipArtifact downloads installation package and unzips it
func downloadAndUnzipArtifact(
	mgr *updateManager,
	log log.T,
	downloadInput artifact.DownloadInput,
	updateDetail *UpdateDetail,
	version string) (err error) {

	log.Infof("Preparing source for version %v", version)
	// download installation zip files
	downloadOutput, err := downloadArtifact(mgr.Context, downloadInput)
	if err != nil ||
		downloadOutput.IsHashMatched == false ||
		downloadOutput.LocalFilePath == "" {
		if err != nil {
			return fmt.Errorf("failed to download file reliably, %v, %v", downloadInput.SourceURL, err.Error())
		}
		return fmt.Errorf("failed to download file reliably, %v", downloadInput.SourceURL)
	}

	// downloaded successfully, append message
	updateDetail.AppendInfo(log, "Successfully downloaded %v", downloadInput.SourceURL)

	// uncompress installation package
	if err = uncompress(
		log,
		downloadOutput.LocalFilePath,
		updateutil.UpdateArtifactFolder(updateDetail.UpdateRoot, updateDetail.PackageName, version)); err != nil {
		return fmt.Errorf("failed to uncompress installation package, %v", err.Error())
	}

	return nil
}
