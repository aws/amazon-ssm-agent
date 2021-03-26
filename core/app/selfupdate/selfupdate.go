// Copyright 2020 Amazon.com, Inc. or its affiliates. All Rights Reserved.
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

// Package selfupdate provides an interface to force update with Message Gateway Service and S3

package selfupdate

import (
	"fmt"
	"hash/fnv"
	"math/rand"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"strings"
	"time"

	"github.com/aws/amazon-ssm-agent/agent/appconfig"
	logger "github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/amazon-ssm-agent/agent/platform"
	"github.com/aws/amazon-ssm-agent/agent/updateutil"
	"github.com/aws/amazon-ssm-agent/agent/updateutil/updateconstants"
	"github.com/aws/amazon-ssm-agent/agent/version"
	"github.com/aws/amazon-ssm-agent/core/app/context"
	"github.com/aws/amazon-ssm-agent/core/app/selfupdate/fileutil"
	"github.com/aws/amazon-ssm-agent/core/app/selfupdate/fileutil/artifact"
	"github.com/carlescere/scheduler"
	"github.com/nightlyone/lockfile"
)

const (
	updateDelayFactor = 43200 // 12 hours
	updateDelayBase   = 1800  // 1800 seconds
)

//UpdatePluginResult represents Agent update plugin result
type UpdateResult struct {
	StandOut      string    `json:"StandOut"`
	StartDateTime time.Time `json:"StartDateTime"`
}

type SelfUpdate struct {
	updateJob            *scheduler.Job
	updateFrequencyHrs   int
	context              context.ICoreAgentContext
	fileManager          artifact.IArtifact
	filsys               fileutil.IFileutil
	updateSchedulerTimer chan bool
}

var platformNameGetter = platform.PlatformName
var nanoChecker = platform.IsPlatformNanoServer
var execCommand = exec.Command
var cmdStart = (*exec.Cmd).Start
var lockFileName = appconfig.UpdaterPidLockfile

// The main purpose of these delegates is to easily test the self update
var (
	updateInitialize        func(string) error
	updateDownloadResource  func(string) error
	updateExecuteSelfUpdate func(log logger.T, region string) (pid int, err error)
)

//IUpdateProvider is the interface to start/stop selfupdate component
type ISelfUpdate interface {
	Start()
	Stop()
}

func NewSelfUpdater(context context.ICoreAgentContext) *SelfUpdate {
	selfupdateContext := context.With("[" + name + "]")
	selfupdateContext.Log().Debug("Initializing self update ...")
	fileManager := artifact.NewSelfUpdateArtifact(selfupdateContext.Log(), *context.AppConfig())
	fileutl := fileutil.NewFileUtil(context.Log())

	selfUpdateProvider := SelfUpdate{
		context:              selfupdateContext,
		fileManager:          fileManager,
		filsys:               fileutl,
		updateSchedulerTimer: make(chan bool, 1),
	}
	updateInitialize = selfUpdateProvider.init
	updateDownloadResource = selfUpdateProvider.downloadResource
	updateExecuteSelfUpdate = selfUpdateProvider.executeSelfUpdate
	return &selfUpdateProvider
}

// Start selfupdate component
func (u *SelfUpdate) Start() {
	log := u.context.Log()

	appConfig := u.context.AppConfig()

	// start the self update process when customer enable it
	if appConfig.Agent.SelfUpdate == true {
		log.Info("Self update is enabled. Starting scheduling ...")
		// initialize the schedule days
		u.loadScheduledFrequency(*appConfig)

		// SSM Agent should first waiting for update event from MGS, if it could not connect with MGS, it should
		// pull the manifest from s3 and check the version status
		u.selfupdateWithS3(log)
	} else {
		log.Debugf("self update is disabled, skipping")
	}

	return
}

func (u *SelfUpdate) loadScheduledFrequency(config appconfig.SsmagentConfig) {
	log := u.context.Log()
	u.updateFrequencyHrs = config.Agent.SelfUpdateScheduleDay * 24 // converting to hours
	log.Debugf("%v frequency is: %d day(s).", name, config.Agent.SelfUpdateScheduleDay)
}

// Generate the schedule job for self update via s3  periodically
func (u *SelfUpdate) scheduleSelfUpdateS3Job(log logger.T) {
	var err error
	if u.updateJob, err = scheduler.Every(u.updateFrequencyHrs).Hours().NotImmediately().Run(u.updateFromS3WithDelay); err != nil {
		log.Errorf("unable to schedule self update job. %v", err)
	}
	return
}

// selfupdate based on S3 manifest file pulling
func (u *SelfUpdate) selfupdateWithS3(log logger.T) {
	log.Debugf("Start scheduling job for s3 based self update")

	instanceId, _ := u.context.Identity().InstanceID()
	hash := fnv.New32a()
	hash.Write([]byte(instanceId))
	rand.Seed(time.Now().UTC().UnixNano() + int64(hash.Sum32()))

	go u.updateFromS3WithDelay()
	u.scheduleSelfUpdateS3Job(log)
}

// Stop selfupdate component
func (u *SelfUpdate) Stop() {
	log := u.context.Log()

	if u.updateJob != nil {
		u.updateJob.Quit <- true
		u.updateSchedulerTimer <- true
		log.Info("Self update stopped successfully...")
	}
}

// Periodically Pulling manifest file and updater from regional S3 bucket.
// Unzip updater and execute the updater
func (u *SelfUpdate) updateFromS3WithDelay() {
	nextTrigger := time.Duration(rand.Intn(updateDelayFactor)+updateDelayBase) * time.Second
	select {
	case <-time.After(nextTrigger):
		_ = u.updateFromS3()
	case <-u.updateSchedulerTimer:
		return
	}
}

// Periodically Pulling manifest file and updater from regional S3 bucket.
// Unzip updater and execute the updater
func (u *SelfUpdate) updateFromS3() (err error) {
	log := u.context.Log()
	log.Debugf("Start self updater")

	var pid int
	var instanceId, region string

	lockFileHandle, _ := lockfile.New(lockFileName)
	err = lockFileHandle.TryLockExpire(updateconstants.UpdateLockFileMinutes) // 60 minutes

	if err != nil {
		if err == lockfile.ErrBusy {
			log.Errorf("Failed to lock update lockfile, another update is in progress: %s", err)
			log.WriteEvent(logger.AgentUpdateResultMessage, "", u.generateEventCode(updateconstants.ErrorUpdaterLockBusy))
			return
		} else {
			log.Warnf("Proceeding update process with new lock. Failed to lock update lockfile: %s", err)
			log.WriteEvent(logger.AgentUpdateResultMessage, "", u.generateWarnEventCode(updateconstants.WarnUpdaterLockFail))
		}
	}

	defer func() {
		if err != nil {
			log.Debug("Unlocked file due to self update error")
			lockFileHandle.Unlock()
		}
		if msg := recover(); msg != nil {
			log.Errorf("update from S3 run panic: %v", msg)
			log.Errorf("%s: %s", msg, debug.Stack())
		}
	}()

	if region, err = u.context.Identity().Region(); err != nil {
		log.Errorf("Self update failed to get region from platform package, %s", err)
		log.WriteEvent(logger.AgentUpdateResultMessage, "", u.generateEventCode(updateconstants.ErrorInitializationFailed))
		return
	}

	if instanceId, err = u.context.Identity().InstanceID(); err != nil {
		log.Errorf("Self update failed to get the instance id, %v", err)
		log.WriteEvent(logger.AgentUpdateResultMessage, "", u.generateEventCode(updateconstants.ErrorInitializationFailed))
		return
	}

	if err = updateInitialize(instanceId); err != nil {
		log.Errorf("Self Update failed to init, %v", err)
		log.WriteEvent(logger.AgentUpdateResultMessage, "", u.generateEventCode(updateconstants.ErrorCreateUpdateFolder))
		return
	}

	if err = updateDownloadResource(region); err != nil {
		log.Errorf("Self Update failed to download resource, %v", err)
		log.WriteEvent(logger.AgentUpdateResultMessage, "", u.generateEventCode(updateconstants.ErrorDownloadUpdater))
		return
	}

	if pid, err = updateExecuteSelfUpdate(log, region); err != nil {
		log.Errorf("Self update failed to execute the self update, %v", err)
		log.WriteEvent(logger.AgentUpdateResultMessage, "", u.generateEventCode(updateconstants.ErrorExecuteUpdater))
		return
	}

	// change owner to updater process. This error should not unlock
	err = lockFileHandle.ChangeOwner(pid)
	if err != nil {
		log.Warnf("Failed to transfer ownership of update lockfile to updater, unlocking: %s", err)
		return
	}
	return
}

func (u *SelfUpdate) init(instanceId string) (err error) {
	log := u.context.Log()

	var orchestrationDir string

	orchestrationDir = filepath.Join(
		appconfig.DefaultDataStorePath,
		instanceId,
		appconfig.DefaultDocumentRootDirName,
		"orchestration", DefaultSelfUpdateFolder, DefaultOutputFolder)
	log.Debugf("orchestration dir is %v", orchestrationDir)

	if err = u.prepareUpdaterDir(log,
		appconfig.UpdaterArtifactsRoot,
		orchestrationDir, instanceId); err != nil {
		return err
	}

	log.Debugf("Self update init successfully...")
	return nil
}

func (u *SelfUpdate) generateEventCode(errorCode updateconstants.ErrorCode) string {
	return updateconstants.UpdateFailed + updateconstants.SelfUpdatePrefix + "_" + string(errorCode)
}

func (u *SelfUpdate) generateWarnEventCode(errorCode string) string {
	return updateconstants.UpdateSucceeded + updateconstants.SelfUpdatePrefix + "_" + errorCode
}

func (u *SelfUpdate) downloadResource(region string) (err error) {
	log := u.context.Log()
	var sourceURL, fileName string
	var updaterDownloadOutput artifact.DownloadOutput

	if fileName, err = u.getUpdaterFileName(log, runtime.GOARCH, updateconstants.CompressFormat); err != nil {
		return fmt.Errorf("selfupdate failed to get updater file name, %v", err)
	}

	sourceURL = u.generateDownloadUpdaterURL(log, region, fileName)
	log.Debugf("Download updater URL is , %s", sourceURL)

	downloadDirectory := filepath.Join(appconfig.DownloadRoot, "update")
	downloadInput := artifact.DownloadInput{
		SourceURL:            sourceURL,
		DestinationDirectory: downloadDirectory,
	}

	if updaterDownloadOutput, err = u.downloadResourceFromS3(downloadInput); err != nil {
		return fmt.Errorf("error during downloading updater, %v", err)
	}

	if err := u.unCompress(log, updaterDownloadOutput); err != nil {
		return fmt.Errorf("error during uncompress updater, %v", err)
	}

	return
}

func (u *SelfUpdate) downloadResourceFromS3(
	downloadInput artifact.DownloadInput) (downloadOutput artifact.DownloadOutput, err error) {
	log := u.context.Log()

	downloadOutput, err = u.fileManager.Download(downloadInput)
	if err != nil {
		return downloadOutput, fmt.Errorf("failed to download Download context %v, %v", downloadInput, err)
	}

	log.Debugf("Succeed to download the contents")
	log.Debugf("Local file path : %v", downloadOutput.LocalFilePath)
	log.Debugf("Is updated: %v", downloadOutput.IsUpdated)
	log.Debugf("Is hash matched %v", downloadOutput.IsHashMatched)
	return
}

func (u *SelfUpdate) executeSelfUpdate(log logger.T, region string) (pid int, err error) {
	var workDic, sourceURL, cmd string

	sourceURL = u.generateDownloadManifestURL(log, region)

	cmd = u.generateUpdateCmd(log, sourceURL)
	log.Infof("Self Update command %v", cmd)

	workDic = filepath.Join(appconfig.UpdaterArtifactsRoot, PackageName, PackageVersion)
	if pid, err = u.exeCommand(log, cmd, workDic); err != nil {
		return -1, fmt.Errorf("failed to execute command for self update, %v", err)
	}

	return pid, nil
}

// Generate the command to check current version is deprecated or not.
func (u *SelfUpdate) generateUpdateCmd(log logger.T, sourceURL string) (cmd string) {
	log.Debugf("Starting generate command for self update")

	cmd = filepath.Join(appconfig.UpdaterArtifactsRoot, PackageName, PackageVersion, Updater) + " -update" + " -selfupdate"

	cmd = u.buildUpdateCommand(cmd, updateconstants.ManifestFileUrlCmd, sourceURL)
	cmd = u.buildUpdateCommand(cmd, updateconstants.SourceVersionCmd, version.Version)

	return cmd
}

// BuildUpdateCommand builds command string with argument and value
func (u *SelfUpdate) buildUpdateCommand(cmd string, arg string, value string) string {
	if value == "" || arg == "" {
		return cmd
	}
	return fmt.Sprintf("%v -%v %v", cmd, arg, value)
}

func (u *SelfUpdate) exeCommand(
	log logger.T,
	cmd string,
	workingDir string) (pid int, err error) {

	parts := strings.Fields(cmd)

	command := execCommand(parts[0], parts[1:]...)
	command.Dir = workingDir
	prepareProcess(command)
	// Start command asynchronously
	err = cmdStart(command)
	pid = updateutil.GetCommandPid(command)
	return
}

func (u *SelfUpdate) unCompress(log logger.T,
	downloadOutput artifact.DownloadOutput) (err error) {

	log.Debugf("Starting to uncompress updater")

	dest := filepath.Join(appconfig.UpdaterArtifactsRoot, PackageName, PackageVersion)
	log.Debugf("Uncompress destination file path is %v", dest)
	log.Debugf("Source file path is %v", downloadOutput.LocalFilePath)
	if uncompressErr := u.fileManager.Uncompress(downloadOutput.LocalFilePath, dest); uncompressErr != nil {
		return fmt.Errorf("Failed to uncompress updater package for self update, %v, %v\n",
			downloadOutput.LocalFilePath,
			uncompressErr.Error())
	}
	log.Debugf("Succeed to uncompress the updater")

	return
}

func (u *SelfUpdate) generateDownloadUpdaterURL(log logger.T, region string, fileName string) (url string) {
	var urlFormat string

	if dynamicS3Endpoint := u.context.Identity().GetDefaultEndpoint("s3"); dynamicS3Endpoint != "" {
		urlFormat = "https://" + dynamicS3Endpoint + UrlPath
	} else {
		// could not retrieve the default s3 endpoint, generate endpoint from region information
		urlFormat = CommonUrlPath
	}

	urlFormat = strings.Replace(urlFormat, RegionHolder, region, -1)
	urlFormat = strings.Replace(urlFormat, FileNameHolder, fileName, -1)

	log.Debugf("updater download url is %s ", urlFormat)

	return urlFormat
}

func (u *SelfUpdate) generateDownloadManifestURL(log logger.T, region string) (manifestUrl string) {

	if dynamicS3Endpoint := u.context.Identity().GetDefaultEndpoint("s3"); dynamicS3Endpoint != "" {
		manifestUrl = "https://" + dynamicS3Endpoint + ManifestPath
	} else {
		// could not retrieve the default s3 endpoint, generate endpoint from region information
		manifestUrl = CommonManifestURL
	}

	manifestUrl = strings.Replace(manifestUrl, RegionHolder, region, -1)

	log.Debugf("manifest download url is %s", manifestUrl)

	return
}

// FileName generates downloadable file name base on agreed convension
func (u *SelfUpdate) getUpdaterFileName(log logger.T, Arch string, CompressFormat string) (fileName string, err error) {

	var PlatformName string

	if PlatformName, err = u.getPlatformName(log); err != nil {
		return "", fmt.Errorf("failed to get platform name, %s", err)
	}

	log.Debugf("Platform Name is %s", PlatformName)
	log.Debugf("Running Arch Name is %s", Arch)
	log.Debugf("Compress format is %s", CompressFormat)

	fileName = "amazon-ssm-agent-updater-{Platform}-{Arch}.{Compressed}"
	fileName = strings.Replace(fileName, PlatformHolder, PlatformName, -1)
	fileName = strings.Replace(fileName, ArchHolder, Arch, -1)
	fileName = strings.Replace(fileName, CompressedHolder, CompressFormat, -1)

	return
}

func (u *SelfUpdate) getPlatformName(log logger.T) (platformName string, err error) {
	var isNano bool

	if platformName, err = platformNameGetter(log); err != nil {
		log.Errorf("Failed to get platform name, %s", err)
		return
	}

	platformName = strings.ToLower(platformName)
	if strings.Contains(platformName, PlatformAmazonLinux) ||
		strings.Contains(platformName, PlatformRedHat) ||
		strings.Contains(platformName, PlatformCentOS) ||
		strings.Contains(platformName, PlatformSuseOS) ||
		strings.Contains(platformName, PlatformOracleLinux) {
		platformName = PlatformLinux
	} else if strings.Contains(platformName, PlatformRaspbian) {
		platformName = PlatformUbuntu
	} else if strings.Contains(platformName, PlatformDebian) {
		platformName = PlatformUbuntu
	} else if strings.Contains(platformName, PlatformMacOsX) {
		platformName = PlatformDarwin
	} else if strings.Contains(platformName, PlatformUbuntu) {
		if isSnap, err := u.isAgentInstalledUsingSnap(log); err == nil && isSnap {
			platformName = PlatformUbuntuSnap
		} else {
			platformName = PlatformUbuntu
		}
	} else if isNano, err = nanoChecker(log); isNano {
		return platformName, fmt.Errorf("self update doesn't support this platform")
	} else if strings.Contains(platformName, PlatformWindows) {
		platformName = PlatformWindows
	} else {
		return platformName, fmt.Errorf("self update doesn't support this platform")
	}

	return platformName, err
}

func (u *SelfUpdate) isAgentInstalledUsingSnap(log logger.T) (result bool, err error) {
	if _, commandErr := exec.Command("snap", "services", "amazon-ssm-agent").Output(); commandErr != nil {
		log.Debugf("Error checking 'snap services amazon-ssm-agent' - %v", commandErr)
		return false, commandErr
	}
	log.Debug("Agent is installed using snap")
	return true, nil
}

func (u *SelfUpdate) prepareUpdaterDir(log logger.T, rootDir, orchestDir string, instanceId string) (err error) {
	var fileName [2]string

	fileName[0] = DefaultStandOut
	fileName[1] = DefaultStandErr

	for _, filePath := range fileName {
		if err = u.createIfNotExist(log, rootDir, filePath); err != nil {
			return fmt.Errorf("failed to create file for self update, filepath: %v %v error : %v",
				rootDir,
				filePath,
				err)
		}
		log.Debugf("Clean up std file %v/%v", rootDir, filePath)
		if err = u.createIfNotExist(log, orchestDir, filePath); err != nil {
			return fmt.Errorf("failed to create file for self update orchestration, filepath: %v %v error : %v",
				orchestDir,
				filePath,
				err)
		}
		log.Debugf("Clean up std file %v/%v", orchestDir, filePath)
	}

	return
}

func (u *SelfUpdate) createIfNotExist(log logger.T, root, filePath string) (err error) {
	var FileWriter *os.File
	var fullFilePath string
	fullFilePath = filepath.Join(root, filePath)

	if u.filsys.Exists(fullFilePath) == false {
		err = u.filsys.MakeDirs(root)
	}

	FileWriter, _ = os.OpenFile(fullFilePath, appconfig.FileFlagsCreateOrAppend, appconfig.ReadWriteAccess)
	defer FileWriter.Close()
	return
}
