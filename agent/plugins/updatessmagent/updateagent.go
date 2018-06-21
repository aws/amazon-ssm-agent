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
	"github.com/aws/amazon-ssm-agent/agent/platform"
	"github.com/aws/amazon-ssm-agent/agent/s3util"
	"github.com/aws/amazon-ssm-agent/agent/task"
	"github.com/aws/amazon-ssm-agent/agent/updateutil"
	"github.com/aws/amazon-ssm-agent/agent/version"
)

// Plugin is the type for the RunCommand plugin.
type Plugin struct {
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
		context *updateutil.InstanceContext,
		updaterPath string,
		messageID string,
		stdout string,
		stderr string,
		keyPrefix string,
		bucketName string) (cmd string, err error)

	downloadManifest(log log.T,
		util updateutil.T,
		pluginInput *UpdatePluginInput,
		context *updateutil.InstanceContext,
		out iohandler.IOHandler) (manifest *Manifest, err error)

	downloadUpdater(log log.T,
		util updateutil.T,
		updaterPackageName string,
		manifest *Manifest,
		out iohandler.IOHandler,
		context *updateutil.InstanceContext) (version string, err error)

	validateUpdate(log log.T,
		pluginInput *UpdatePluginInput,
		context *updateutil.InstanceContext,
		manifest *Manifest,
		out iohandler.IOHandler) (noNeedToUpdate bool, err error)
}

// Assign method to global variables to allow unittest to override
var getAppConfig = appconfig.Config
var fileDownload = artifact.Download
var fileUncompress = fileutil.Uncompress
var updateAgent = runUpdateAgent

// NewPlugin returns a new instance of the plugin.
func NewPlugin(updatePluginConfig UpdatePluginConfig) (*Plugin, error) {
	var plugin Plugin
	plugin.ManifestLocation = updatePluginConfig.ManifestLocation
	return &plugin, nil
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
	startTime time.Time) {
	var pluginInput UpdatePluginInput
	var err error
	var context *updateutil.InstanceContext

	pluginConfig := iohandler.DefaultOutputConfig()

	if err = jsonutil.Remarshal(rawPluginInput, &pluginInput); err != nil {
		output.MarkAsFailed(fmt.Errorf("invalid format in plugin properties %v;\nerror %v", rawPluginInput, err))
		return
	}

	if context, err = util.CreateInstanceContext(log); err != nil {
		output.MarkAsFailed(err)
		return
	}

	//Use default manifest location is the override is not present
	if len(pluginInput.Source) == 0 {
		pluginInput.Source = p.ManifestLocation
	}
	//Calculate manifest location base on current instance's region
	pluginInput.Source = strings.Replace(pluginInput.Source, updateutil.RegionHolder, context.Region, -1)
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
	manifest, downloadErr := manager.downloadManifest(log, util, &pluginInput, context, output)
	if downloadErr != nil {
		output.MarkAsFailed(downloadErr)
		return
	}

	//Validate update details
	noNeedToUpdate := false
	if noNeedToUpdate, err = manager.validateUpdate(log, &pluginInput, context, manifest, output); noNeedToUpdate {
		if err != nil {
			output.MarkAsFailed(err)
		}
		return
	}

	//Download updater and retrieve the version number
	updaterVersion := ""
	if updaterVersion, err = manager.downloadUpdater(
		log, util, pluginInput.UpdaterName, manifest, output, context); err != nil {
		output.MarkAsFailed(err)
		return
	}

	//Generate update command base on the update detail
	cmd := ""
	if cmd, err = manager.generateUpdateCmd(log,
		manifest,
		&pluginInput,
		context,
		updateutil.UpdaterFilePath(appconfig.UpdaterArtifactsRoot, pluginInput.UpdaterName, updaterVersion),
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

	if err = util.ExeCommand(
		log,
		cmd,
		workDir,
		appconfig.UpdaterArtifactsRoot,
		pluginConfig.StdoutFileName,
		pluginConfig.StderrFileName,
		true); err != nil {
		output.MarkAsFailed(err)
		return
	}

	output.MarkAsInProgress()
	return
}

//generateUpdateCmd generates cmd for the updater
func (m *updateManager) generateUpdateCmd(log log.T,
	manifest *Manifest,
	pluginInput *UpdatePluginInput,
	context *updateutil.InstanceContext,
	updaterPath string,
	messageID string,
	stdout string,
	stderr string,
	keyPrefix string,
	bucketName string) (cmd string, err error) {
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

	return
}

//downloadManifest downloads manifest file from s3 bucket
func (m *updateManager) downloadManifest(log log.T,
	util updateutil.T,
	pluginInput *UpdatePluginInput,
	context *updateutil.InstanceContext,
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

	downloadOutput, downloadErr := fileDownload(log, downloadInput)
	if downloadErr != nil ||
		downloadOutput.IsHashMatched == false ||
		downloadOutput.LocalFilePath == "" {
		return nil, downloadErr
	}
	out.AppendInfof("Successfully downloaded %v\n", downloadInput.SourceURL)
	return ParseManifest(log, downloadOutput.LocalFilePath, context, pluginInput.AgentName)
}

//downloadUpdater downloads updater from the s3 bucket
func (m *updateManager) downloadUpdater(log log.T,
	util updateutil.T,
	updaterPackageName string,
	manifest *Manifest,
	out iohandler.IOHandler,
	context *updateutil.InstanceContext) (version string, err error) {
	var hash = ""
	var source = ""

	if version, err = manifest.LatestVersion(log, context, updaterPackageName); err != nil {
		return
	}
	if source, hash, err = manifest.DownloadURLAndHash(context, updaterPackageName, version); err != nil {
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
	downloadOutput, downloadErr := fileDownload(log, downloadInput)
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
		log,
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
	context *updateutil.InstanceContext,
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

func (p *Plugin) Execute(context context.T, config contracts.Configuration, cancelFlag task.CancelFlag, output iohandler.IOHandler) {
	log := context.Log()
	log.Info("RunCommand started with configuration ", config)
	util := new(updateutil.Utility)
	manager := new(updateManager)

	if cancelFlag.ShutDown() {
		output.MarkAsShutdown()
	} else if cancelFlag.Canceled() {
		output.MarkAsCancelled()
	} else {
		updateAgent(p,
			config,
			log,
			manager,
			util,
			config.Properties,
			cancelFlag,
			output,
			time.Now())
	}
	return
}

// Name returns the plugin name
func Name() string {
	return appconfig.PluginNameAwsAgentUpdate
}

// GetUpdatePluginConfig returns the default values for the update plugin
func GetUpdatePluginConfig(context context.T) UpdatePluginConfig {
	log := context.Log()
	region, err := platform.Region()
	if err != nil {
		log.Errorf("Error retrieving agent region in update plugin config. error: %v\n", err)
	}

	var manifestUrl string
	if strings.HasPrefix(region, s3util.ChinaRegionPrefix) {
		manifestUrl = ChinaManifestURL
	} else {
		manifestUrl = CommonManifestURL
	}

	return UpdatePluginConfig{
		ManifestLocation: manifestUrl,
	}
}
