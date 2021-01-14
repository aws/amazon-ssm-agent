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
	"errors"
	"fmt"
	"math/rand"
	"strconv"
	"strings"
	"time"

	"github.com/aws/amazon-ssm-agent/agent/appconfig"
	"github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/aws/amazon-ssm-agent/agent/contracts"
	"github.com/aws/amazon-ssm-agent/agent/fileutil"
	"github.com/aws/amazon-ssm-agent/agent/fileutil/artifact"
	"github.com/aws/amazon-ssm-agent/agent/framework/processor/executer/iohandler"
	"github.com/aws/amazon-ssm-agent/agent/jsonutil"
	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/amazon-ssm-agent/agent/s3util"
	"github.com/aws/amazon-ssm-agent/agent/task"
	"github.com/aws/amazon-ssm-agent/agent/updateutil"
	"github.com/aws/amazon-ssm-agent/agent/version"
	"github.com/aws/amazon-ssm-agent/common/identity"
	"github.com/aws/amazon-ssm-agent/core/executor"
	"github.com/nightlyone/lockfile"
)

// Plugin is the type for the RunCommand plugin.
type Plugin struct {
	Context context.T
	// Manifest location
	ManifestLocation string
}

// UpdatePluginInput represents one set of commands executed by the UpdateAgent plugin.
type UpdatePluginInput struct {
	contracts.PluginInput
	AgentName      string `json:"agentName"`
	AllowDowngrade string `json:"allowDowngrade"`
	TargetVersion  string `json:"targetVersion"`
	Source         string `json:"source"`
	UpdaterName    string `json:"-"`
}

// UpdatePluginConfig is used for initializing update agent plugin with default values
type UpdatePluginConfig struct {
	ManifestLocation string
}

type updateManager struct{}

type pluginHelper interface {
	generateUpdateCmd(log log.T,
		manifest *Manifest,
		pluginInput *UpdatePluginInput,
		context *updateutil.InstanceInfo,
		updaterVersion string,
		messageID string,
		stdout string,
		stderr string,
		keyPrefix string,
		bucketName string) (cmd string, err error)

	downloadManifest(context context.T,
		util updateutil.T,
		pluginInput *UpdatePluginInput,
		info *updateutil.InstanceInfo,
		out iohandler.IOHandler) (manifest *Manifest, err error)

	downloadUpdater(context context.T,
		util updateutil.T,
		updaterPackageName string,
		manifest *Manifest,
		out iohandler.IOHandler,
		info *updateutil.InstanceInfo) (version string, err error)

	validateUpdate(log log.T,
		pluginInput *UpdatePluginInput,
		context *updateutil.InstanceInfo,
		manifest *Manifest,
		out iohandler.IOHandler) (noNeedToUpdate bool, err error)
}

// Assign method to global variables to allow unittest to override
var fileDownload = artifact.Download
var fileUncompress = fileutil.Uncompress
var updateAgent = runUpdateAgent
var getLockObj = lockfile.New
var updateUtilRef updateutil.T // added mainly for testing

// NewPlugin returns a new instance of the plugin.
func NewPlugin(context context.T, updatePluginConfig UpdatePluginConfig) (*Plugin, error) {
	return &Plugin{
		context,
		updatePluginConfig.ManifestLocation,
	}, nil
}

// updateAgent downloads the installation packages and update the agent
func runUpdateAgent(
	p *Plugin,
	config contracts.Configuration,
	log log.T,
	manager pluginHelper,
	util updateutil.T,
	rawPluginInput interface{},
	cancelFlag task.CancelFlag,
	output iohandler.IOHandler,
	startTime time.Time,
	exec executor.IExecutor) (pid int) {
	var pluginInput UpdatePluginInput
	var err error
	var instanceInfo *updateutil.InstanceInfo

	pluginConfig := iohandler.DefaultOutputConfig()

	if err = jsonutil.Remarshal(rawPluginInput, &pluginInput); err != nil {
		output.MarkAsFailed(fmt.Errorf("invalid format in plugin properties %v;\nerror %v", rawPluginInput, err))
		return
	}

	if instanceInfo, err = util.CreateInstanceInfo(log); err != nil {
		output.MarkAsFailed(err)
		return
	}

	//Use default manifest location is the override is not present
	if len(pluginInput.Source) == 0 {
		pluginInput.Source = p.ManifestLocation
	}
	//Calculate manifest location base on current instance's region
	pluginInput.Source = strings.Replace(pluginInput.Source, updateutil.RegionHolder, instanceInfo.Region, -1)
	//Calculate updater package name base on agent name
	pluginInput.UpdaterName = pluginInput.AgentName + updateutil.UpdaterPackageNamePrefix
	//Generate update output
	targetVersion := pluginInput.TargetVersion
	if len(targetVersion) == 0 {
		targetVersion = "latest"
	}
	output.AppendInfof("Updating %v from %v to %v\n",
		pluginInput.AgentName,
		version.Version,
		targetVersion)

	//Download manifest file
	var manifest *Manifest
	var downloadErr error

	noOfRetries := 2
	updateRetryDelayBase := 1000 // 1000 millisecond
	updateRetryDelay := 500      // 500 millisecond

	manifest, downloadErr = manager.downloadManifest(p.Context, util, &pluginInput, instanceInfo, output)
	if downloadErr != nil {
		output.MarkAsFailed(downloadErr)
		return
	}

	//Validate update details
	noNeedToUpdate := false
	if noNeedToUpdate, err = manager.validateUpdate(log, &pluginInput, instanceInfo, manifest, output); noNeedToUpdate {
		if err != nil {
			output.MarkAsFailed(err)
		}
		return
	}

	//Download updater and retrieve the version number
	updaterVersion := ""
	if updaterVersion, err = manager.downloadUpdater(
		p.Context, util, pluginInput.UpdaterName, manifest, output, instanceInfo); err != nil {
		output.MarkAsFailed(err)
		return
	}

	//Generate update command base on the update detail
	cmd := ""
	if cmd, err = manager.generateUpdateCmd(log,
		manifest,
		&pluginInput,
		instanceInfo,
		updaterVersion,
		config.MessageId,
		pluginConfig.StdoutFileName,
		pluginConfig.StderrFileName,
		fileutil.BuildS3Path(output.GetIOConfig().OutputS3KeyPrefix, config.PluginID),
		output.GetIOConfig().OutputS3BucketName); err != nil {
		output.MarkAsFailed(err)
		return
	}
	log.Debugf("Update command %v", cmd)

	//Save update plugin result to local file, updater will read it during agent update
	updatePluginResult := &updateutil.UpdatePluginResult{
		StandOut:      output.GetStdout(),
		StartDateTime: startTime,
	}
	if err = util.SaveUpdatePluginResult(log, appconfig.UpdaterArtifactsRoot, updatePluginResult); err != nil {
		output.MarkAsFailed(err)
		return
	}

	// If disk space is not sufficient, fail the update to prevent installation and notify user in output
	// If loading disk space fails, continue to update (agent update is backed by rollback handler)
	log.Infof("Checking available disk space ...")
	if isDiskSpaceSufficient, err := util.IsDiskSpaceSufficientForUpdate(log); err == nil && !isDiskSpaceSufficient {
		output.MarkAsFailed(errors.New("Insufficient available disk space"))
		return
	}

	log.Infof("Start Installation")
	log.Infof("Hand over update process to %v", pluginInput.UpdaterName)
	//Execute updater, hand over the update process
	workDir := updateutil.UpdateArtifactFolder(
		appconfig.UpdaterArtifactsRoot, pluginInput.UpdaterName, updaterVersion)

	for retryCounter := 1; retryCounter <= noOfRetries; retryCounter++ {
		pid, _, err = util.ExeCommand(
			log,
			cmd,
			workDir,
			appconfig.UpdaterArtifactsRoot,
			pluginConfig.StdoutFileName,
			pluginConfig.StderrFileName,
			true)
		if err == nil {
			break
		}
		if retryCounter < noOfRetries {
			time.Sleep(time.Duration(updateRetryDelayBase+rand.Intn(updateRetryDelay)) * time.Millisecond)
		}
	}

	if err != nil {
		output.MarkAsFailed(err)
		return
	}

	// Sleep for 1 second and verify updater is running
	time.Sleep(time.Second)
	isRunning, procErr := exec.IsPidRunning(pid)
	if procErr != nil {
		log.Warnf("Failed to check if updater process is running: %s", err)
	} else {
		if !isRunning {
			errMsg := "Updater died before updating, make sure your system is supported"
			log.Error(errMsg)
			output.MarkAsFailed(fmt.Errorf(errMsg))

			exec.Kill(pid)
			return
		} else {
			log.Info("Updater is running")
		}
	}

	output.MarkAsInProgress()
	return
}

//generateUpdateCmd generates cmd for the updater
func (m *updateManager) generateUpdateCmd(log log.T,
	manifest *Manifest,
	pluginInput *UpdatePluginInput,
	context *updateutil.InstanceInfo,
	updaterVersion string,
	messageID string,
	stdout string,
	stderr string,
	keyPrefix string,
	bucketName string) (cmd string, err error) {
	updaterPath := updateutil.UpdaterFilePath(appconfig.UpdaterArtifactsRoot, pluginInput.UpdaterName, updaterVersion)
	cmd = updaterPath + " -update"
	source := ""
	hash := ""

	//Get download url and hash value from for the current version of ssm agent
	if source, hash, err = manifest.DownloadURLAndHash(
		context, pluginInput.AgentName, version.Version); err != nil {
		return
	}
	cmd = updateutil.BuildUpdateCommand(cmd, updateutil.SourceVersionCmd, version.Version)
	cmd = updateutil.BuildUpdateCommand(cmd, updateutil.SourceLocationCmd, source)
	cmd = updateutil.BuildUpdateCommand(cmd, updateutil.SourceHashCmd, hash)

	//Get download url and hash value from for the target version of ssm agent
	if source, hash, err = manifest.DownloadURLAndHash(
		context, pluginInput.AgentName, pluginInput.TargetVersion); err != nil {
		return
	}
	cmd = updateutil.BuildUpdateCommand(cmd, updateutil.TargetVersionCmd, pluginInput.TargetVersion)
	cmd = updateutil.BuildUpdateCommand(cmd, updateutil.TargetLocationCmd, source)
	cmd = updateutil.BuildUpdateCommand(cmd, updateutil.TargetHashCmd, hash)

	cmd = updateutil.BuildUpdateCommand(cmd, updateutil.PackageNameCmd, pluginInput.AgentName)
	cmd = updateutil.BuildUpdateCommand(cmd, updateutil.MessageIDCmd, messageID)

	cmd = updateutil.BuildUpdateCommand(cmd, updateutil.StdoutFileName, stdout)
	cmd = updateutil.BuildUpdateCommand(cmd, updateutil.StderrFileName, stderr)

	cmd = updateutil.BuildUpdateCommand(cmd, updateutil.OutputKeyPrefixCmd, keyPrefix)
	cmd = updateutil.BuildUpdateCommand(cmd, updateutil.OutputBucketNameCmd, bucketName)

	versionSplit := strings.Split(updaterVersion, ".")
	majorVersion, _ := strconv.Atoi(versionSplit[0])
	if majorVersion > 2 {
		cmd = updateutil.BuildUpdateCommand(cmd, updateutil.ManifestFileUrlCmd, pluginInput.Source)
	}
	return
}

//downloadManifest downloads manifest file from s3 bucket
func (m *updateManager) downloadManifest(context context.T,
	util updateutil.T,
	pluginInput *UpdatePluginInput,
	info *updateutil.InstanceInfo,
	out iohandler.IOHandler) (manifest *Manifest, err error) {
	//Download source
	var updateDownload = ""
	updateDownload, err = util.CreateUpdateDownloadFolder()
	if err != nil {
		return nil, err
	}

	downloadInput := artifact.DownloadInput{
		SourceURL:            pluginInput.Source,
		DestinationDirectory: updateDownload,
	}

	downloadOutput, downloadErr := fileDownload(context, downloadInput)
	if downloadErr != nil ||
		downloadOutput.IsHashMatched == false ||
		downloadOutput.LocalFilePath == "" {
		return nil, downloadErr
	}
	out.AppendInfof("Successfully downloaded %v\n", downloadInput.SourceURL)
	return ParseManifest(context.Log(), downloadOutput.LocalFilePath, info, pluginInput.AgentName)
}

//downloadUpdater downloads updater from the s3 bucket
func (m *updateManager) downloadUpdater(context context.T,
	util updateutil.T,
	updaterPackageName string,
	manifest *Manifest,
	out iohandler.IOHandler,
	info *updateutil.InstanceInfo) (version string, err error) {
	var hash = ""
	var source = ""

	if version, err = manifest.LatestVersion(context.Log(), info, updaterPackageName); err != nil {
		return
	}
	if source, hash, err = manifest.DownloadURLAndHash(info, updaterPackageName, version); err != nil {
		return
	}
	var updateDownloadFolder = ""
	if updateDownloadFolder, err = util.CreateUpdateDownloadFolder(); err != nil {
		return
	}

	downloadInput := artifact.DownloadInput{
		SourceURL: source,
		SourceChecksums: map[string]string{
			updateutil.HashType: hash,
		},
		DestinationDirectory: updateDownloadFolder,
	}
	downloadOutput, downloadErr := fileDownload(context, downloadInput)
	if downloadErr != nil ||
		downloadOutput.IsHashMatched == false ||
		downloadOutput.LocalFilePath == "" {

		errMessage := fmt.Sprintf("failed to download file reliably, %v\n", downloadInput.SourceURL)
		if downloadErr != nil {
			errMessage = fmt.Sprintf("%v, %v", errMessage, downloadErr.Error())
		}
		return version, errors.New(errMessage)
	}
	out.AppendInfof("Successfully downloaded %v\n", downloadInput.SourceURL)
	if uncompressErr := fileUncompress(
		context.Log(),
		downloadOutput.LocalFilePath,
		updateutil.UpdateArtifactFolder(appconfig.UpdaterArtifactsRoot, updaterPackageName, version)); uncompressErr != nil {
		return version, fmt.Errorf("failed to uncompress updater package, %v, %v\n",
			downloadOutput.LocalFilePath,
			uncompressErr.Error())
	}

	return version, nil
}

//validateUpdate validates manifest against update request
func (m *updateManager) validateUpdate(log log.T,
	pluginInput *UpdatePluginInput,
	context *updateutil.InstanceInfo,
	manifest *Manifest,
	out iohandler.IOHandler) (noNeedToUpdate bool, err error) {
	currentVersion := version.Version
	var allowDowngrade = false
	if len(pluginInput.TargetVersion) == 0 {
		if pluginInput.TargetVersion, err = manifest.LatestVersion(log, context, pluginInput.AgentName); err != nil {
			return true, err
		}
	}

	if allowDowngrade, err = strconv.ParseBool(pluginInput.AllowDowngrade); err != nil {
		return true, err
	}

	res, err := updateutil.CompareVersion(pluginInput.TargetVersion, currentVersion)
	if err != nil {
		return true, err
	}

	if res == 0 {
		out.AppendInfof("%v %v has already been installed, update skipped\n",
			pluginInput.AgentName,
			currentVersion)
		out.MarkAsSucceeded()
		return true, nil
	}

	if res == -1 && !allowDowngrade {
		return true,
			fmt.Errorf(
				"updating %v to an older version, please enable allow downgrade to proceed\n",
				pluginInput.AgentName)

	}
	if !manifest.HasVersion(context, pluginInput.AgentName, pluginInput.TargetVersion) {
		return true,
			fmt.Errorf(
				"%v version %v is unsupported\n",
				pluginInput.AgentName,
				pluginInput.TargetVersion)
	}
	if !manifest.HasVersion(context, pluginInput.AgentName, currentVersion) {
		return true,
			fmt.Errorf(
				"%v current version %v is unsupported on current platform\n",
				pluginInput.AgentName,
				currentVersion)
	}

	return false, nil
}

func (p *Plugin) Execute(config contracts.Configuration, cancelFlag task.CancelFlag, output iohandler.IOHandler) {
	log := p.Context.Log()
	log.Info("RunCommand started with configuration ", config)
	if updateUtilRef == nil {
		updateUtilRef = &updateutil.Utility{
			Context: p.Context,
		}
	}
	manager := new(updateManager)
	executor := executor.NewProcessExecutor(log)

	if cancelFlag.ShutDown() {
		output.MarkAsShutdown()
	} else if cancelFlag.Canceled() {
		output.MarkAsCancelled()
	} else {
		// create update directory before creating locks
		if _, directoryErr := updateUtilRef.CreateUpdateDownloadFolder(); directoryErr != nil {
			log.Warnf("error while creating update directory: %v", directoryErr)
		}

		// First check if lock is locked by anyone
		lock, _ := getLockObj(appconfig.UpdaterPidLockfile)
		err := lock.TryLockExpireWithRetry(updateutil.UpdateLockFileMinutes)

		if err != nil {
			if err == lockfile.ErrBusy {
				log.Warnf("Failed to lock update lockfile, another update is in progress: %s", err)
				output.MarkAsFailed(fmt.Errorf("Another update in progress, try again later"))
				return
			} else {
				log.Warnf("Proceeding update process with new lock. Failed to lock update lockfile: %s", err)
			}
		}

		defer func() {
			if err := recover(); err != nil {
				// If we panic, we want to release the lock.
				log.Errorf("UpdateAgent panicked with error '%s'. Unlocking lockfile", err)
				_ = lock.Unlock()

				if output.GetStatus() != contracts.ResultStatusFailed {
					output.MarkAsFailed(fmt.Errorf("Panic with error: '%s'", err))
				}
			}
		}()

		pid := updateAgent(p,
			config,
			log,
			manager,
			updateUtilRef,
			config.Properties,
			cancelFlag,
			output,
			time.Now(),
			executor)

		// If starting update fails, we unlock
		if output.GetStatus() != contracts.ResultStatusInProgress {
			err = lock.Unlock()
			if err != nil {
				log.Warnf("Failed to unlock update lockfile: %s", err)
			}
			return
		}

		// We need to change ownership to the updater processes because
		// the document worker dies right after this function
		// If we don't change ownership, other updates can start before before the updater has finished
		err = lock.ChangeOwner(pid)
		if err != nil {
			log.Warnf("Failed to transfer ownership of update lockfile to updater, unlocking: %s", err)
			_ = lock.Unlock()
		}
	}
}

// Name returns the plugin name
func Name() string {
	return appconfig.PluginNameAwsAgentUpdate
}

// GetUpdatePluginConfig returns the default values for the update plugin
func GetUpdatePluginConfig(context context.T) UpdatePluginConfig {
	log := context.Log()
	identity := context.Identity()
	region, err := identity.Region()
	if err != nil {
		log.Errorf("Error retrieving agent region in update plugin config. error: %v\n", err)
	}

	var manifestUrl string
	manifestUrl = retrieveDynamicS3ManifestUrl(identity, "s3")
	if manifestUrl == "" {
		if strings.HasPrefix(region, s3util.ChinaRegionPrefix) {
			manifestUrl = ChinaManifestURL
		} else {
			manifestUrl = CommonManifestURL
		}
	}

	return UpdatePluginConfig{
		ManifestLocation: manifestUrl,
	}
}

func retrieveDynamicS3ManifestUrl(agentIdentity identity.IAgentIdentity, service string) string {
	if dynamicS3Endpoint := agentIdentity.GetDefaultEndpoint(service); dynamicS3Endpoint != "" {
		return "https://" + dynamicS3Endpoint + ManifestPath
	}
	return ""
}
