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

// +build windows

// Package updateec2config implements the UpdateEC2Config plugin.
package updateec2config

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
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

//TODO move the interface and structs into a separate file to reduce the size of this main file
// pluginHelper is a interface that has helper functions for update manager
type pluginHelper interface {
	generateSetupUpdateCmd(log log.T,
		manifest *Manifest,
		pluginInput *UpdatePluginInput,
		context *updateutil.InstanceContext,
		updaterPath string,
		messageID string,
		agentVersion string) (cmd string, err error)

	generateUpdateCmd(log log.T,
		updaterPath string) (cmd string, err error)

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
		out iohandler.IOHandler, version string) (noNeedToUpdate bool, err error)

	loadUpdateContext(log log.T,
		path string) (updateContext *UpdateContextFile, err error)
}

// Assign method to global variables to allow unittest to override
var fileDownload = artifact.Download
var fileUncompress = fileutil.Uncompress
var updateAgent = runUpdateAgent
var mkDirAll = os.MkdirAll

// NewPlugin returns a new instance of the plugin.
func NewPlugin(updatePluginConfig UpdatePluginConfig) (*Plugin, error) {
	var plugin Plugin
	plugin.ManifestLocation = updatePluginConfig.ManifestLocation
	return &plugin, nil
}

// getEC2ConfigCurrentVersion gets the current version of EC2 config installed on the platform
func getEC2ConfigCurrentVersion(log log.T) (res string, err error) {

	ec2ConfigCLICmd := filepath.Join(os.Getenv("ProgramFiles"), "Amazon", "Ec2ConfigService", "ec2config-cli.exe")
	var cmdOut []byte
	cmdArgs := []string{"--ec2config-version"}

	if cmdOut, err = exec.Command(ec2ConfigCLICmd, cmdArgs...).CombinedOutput(); err != nil {
		log.Errorf("There was an error running %v %v. Error = %s", ec2ConfigCLICmd, cmdArgs, err)
		return res, err
	}

	data := string(cmdOut)
	if len(data) > 1 {
		version := strings.TrimSpace(data)
		log.Debug("GetEC2ConfigCurrentVersion: version after trimming space is ", version)
		return version, err
	}

	return res, fmt.Errorf("Ec2Config version cannot be determined")
}

// TODO add to update manager to merge codes between update agent and update ssm agent to avoid duplication
// updateAgent downloads the installation packages and update the agent
func runUpdateAgent(
	p *Plugin,
	config contracts.Configuration,
	log log.T,
	manager pluginHelper,
	util updateutil.T,
	rawPluginInput interface{},
	output iohandler.IOHandler,
	startTime time.Time) {
	var pluginInput UpdatePluginInput
	var updatecontext *UpdateContextFile = new(UpdateContextFile)
	var err error
	var context *updateutil.InstanceContext

	pluginConfig := iohandler.DefaultOutputConfig()

	if err = jsonutil.Remarshal(rawPluginInput, &pluginInput); err != nil {
		output.MarkAsFailed(fmt.Errorf("invalid format in plugin properties %v;\nerror %v", rawPluginInput, err))
		return
	}

	log.Debugf("[runUpdateAgent]: Will now create the instance context")
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

	var agentVersion string
	log.Debugf("[runUpdateAgent]: getting the current version of ec2config ")
	if agentVersion, err = getEC2ConfigCurrentVersion(log); err != nil {
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

	output.AppendInfof("Updating %v from %v to %v",
		pluginInput.AgentName,
		agentVersion,
		targetVersion)

	if updatecontext, err = manager.loadUpdateContext(log, appconfig.UpdateContextFilePath); err != nil {
		log.Error("Update context load error: ", err)
	}

	//Update only when no other update process is running
	if updatecontext.UpdateState != notStarted && updatecontext.UpdateState != completed { //update process is running
		output.MarkAsFailed(fmt.Errorf("Another update in progress, try again later"))
	} else { //if update process is not running

		//Download manifest file
		manifest, downloadErr := manager.downloadManifest(log, util, &pluginInput, context, output)
		if downloadErr != nil {
			output.MarkAsFailed(downloadErr)
			return
		}

		//Validate update details
		noNeedToUpdate := false
		if noNeedToUpdate, err = manager.validateUpdate(log, &pluginInput, context, manifest, output, agentVersion); noNeedToUpdate {
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
		if cmd, err = manager.generateSetupUpdateCmd(log,
			manifest,
			&pluginInput,
			context,
			UpdaterFilePath(appconfig.EC2UpdateArtifactsRoot, pluginInput.UpdaterName, updaterVersion),
			config.MessageId,
			agentVersion); err != nil {
			output.MarkAsFailed(err)
			return
		}
		log.Debugf("Setup update command %v", cmd)

		//Save update plugin result to local file, updater will read it during agent update
		updatePluginResult := &updateutil.UpdatePluginResult{
			StandOut:      output.GetStdout(),
			StartDateTime: startTime,
		}
		if err = util.SaveUpdatePluginResult(log, appconfig.EC2UpdateArtifactsRoot, updatePluginResult); err != nil {
			output.MarkAsFailed(err)
			return
		}

		workDir := updateutil.UpdateArtifactFolder(
			appconfig.EC2UpdateArtifactsRoot, pluginInput.UpdaterName, updaterVersion)

		//Command to setup the installation
		if _, _, err = util.ExeCommand(log, cmd, workDir, appconfig.EC2UpdateArtifactsRoot, pluginConfig.StdoutFileName, pluginConfig.StderrFileName, false); err != nil {
			output.MarkAsFailed(err)
			return
		}
		cmd = ""

		log.Infof("Start Installation")
		log.Infof("Hand over update process to %v", pluginInput.UpdaterName)
		//Execute updater, hand over the update process
		if cmd, err = manager.generateUpdateCmd(log,
			UpdaterFilePath(appconfig.EC2UpdateArtifactsRoot, pluginInput.UpdaterName, updaterVersion)); err != nil {
			output.MarkAsFailed(err)
			return
		}
		log.Debugf("Setup update command %v", cmd)
		if _, _, err = util.ExeCommand(log, cmd, workDir, appconfig.EC2UpdateArtifactsRoot, pluginConfig.StdoutFileName, pluginConfig.StderrFileName, true); err != nil {
			output.MarkAsFailed(err)
			return
		}
		output.MarkAsInProgress()
	}
	return
}

// TODO Create a command package for command execution
// generateSetupUpdateCmd generates cmd to setup the installation process
func (m *updateManager) generateSetupUpdateCmd(log log.T,
	manifest *Manifest,
	pluginInput *UpdatePluginInput,
	context *updateutil.InstanceContext,
	updaterPath string,
	messageID string,
	agentVersion string) (cmd string, err error) {
	cmd = updaterPath + SetupInstallCmd //Command sent to updater to setup the installation
	source := ""
	hash := ""

	//Get download url and hash value from for the current version of ssm agent
	if source, hash, err = manifest.DownloadURLAndHash(
		context, EC2ConfigAgentName, agentVersion, EC2SetupFileName, S3Format, HTTPFormat); err != nil {
		return
	}
	cmd = updateutil.BuildUpdateCommand(cmd, SourceVersionCmd, agentVersion)
	cmd = updateutil.BuildUpdateCommand(cmd, SourceLocationCmd, source)
	cmd = updateutil.BuildUpdateCommand(cmd, SourceHashCmd, hash)

	//Get download url and hash value from for the target version of ssm agent
	if source, hash, err = manifest.DownloadURLAndHash(
		context, EC2ConfigAgentName, pluginInput.TargetVersion, EC2SetupFileName, S3Format, HTTPFormat); err != nil {
		return
	}
	cmd = updateutil.BuildUpdateCommand(cmd, TargetVersionCmd, pluginInput.TargetVersion)
	cmd = updateutil.BuildUpdateCommand(cmd, TargetLocationCmd, source)
	cmd = updateutil.BuildUpdateCommand(cmd, TargetHashCmd, hash)

	cmd = updateutil.BuildUpdateCommand(cmd, "-"+updateutil.PackageNameCmd, EC2ConfigAgentName)

	//messageID obtained from ssm is in the format = aws.ssm.{message-id}.{instance-id}. Parsing for use here
	messageinfo := strings.Split(messageID, ".")
	cmd = updateutil.BuildUpdateCommand(cmd, MessageIDCmd, messageID)

	cmd = updateutil.BuildUpdateCommand(cmd, DocumentIDCmd, messageID)

	cmd = updateutil.BuildUpdateCommand(cmd, HistoryCmd, numHistories)

	var appConfig appconfig.SsmagentConfig
	appConfig, err = appconfig.Config(false)
	if err != nil {
		log.Error("something went wrong while generating the setup installation command")
		return "", err
	}

	cmd = updateutil.BuildUpdateCommand(cmd, MdsEndpointCmd, appConfig.Mds.Endpoint)

	cmd = updateutil.BuildUpdateCommand(cmd, InstanceID, messageinfo[3])

	cmd = updateutil.BuildUpdateCommand(cmd, RegionIDCmd, context.Region)

	user_agent := "EC2Config" + "/" + agentVersion
	cmd = updateutil.BuildUpdateCommand(cmd, UserAgentCmd, user_agent)

	cmd = cmd + UpdateHealthCmd //sends command to update health information after setting up installation

	log.Debug("Setup installation command is ", cmd)
	return
}

// generateUpdateCmd generates the command to perform update
func (m *updateManager) generateUpdateCmd(log log.T,
	updaterPath string) (cmd string, err error) {
	cmd = updaterPath + UpdateCmd //argument provided to the updater to perform update

	cmd = updateutil.BuildUpdateCommand(cmd, HistoryCmd, numHistories)

	var appConfig appconfig.SsmagentConfig
	appConfig, err = appconfig.Config(false)
	if err != nil {
		log.Error("something went wrong while generating update command")
		return "", err
	}

	cmd = updateutil.BuildUpdateCommand(cmd, MdsEndpointCmd, appConfig.Mds.Endpoint)

	log.Debug("Update command is ", cmd)
	return
}

// createUpdateDownloadFolder creates folder for storing update downloads
func createUpdateDownloadFolder() (folder string, err error) {
	root := filepath.Join(appconfig.EC2UpdaterDownloadRoot, "update")
	if err = mkDirAll(root, os.ModePerm|os.ModeDir); err != nil {
		return "", err
	}

	return root, nil
}

// downloadManifest downloads manifest file from s3 bucket
func (m *updateManager) downloadManifest(log log.T,
	util updateutil.T,
	pluginInput *UpdatePluginInput,
	context *updateutil.InstanceContext,
	out iohandler.IOHandler) (manifest *Manifest, err error) {
	//Download source
	var updateDownload = ""

	if updateDownload, err = createUpdateDownloadFolder(); err != nil {
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
	out.AppendInfof("Successfully downloaded %v", downloadInput.SourceURL)
	return ParseManifest(log, downloadOutput.LocalFilePath)
}

// downloadUpdater downloads updater from the s3 bucket
func (m *updateManager) downloadUpdater(log log.T,
	util updateutil.T,
	updaterPackageName string,
	manifest *Manifest,
	out iohandler.IOHandler,
	context *updateutil.InstanceContext) (version string, err error) {
	var hash = ""
	var source = ""

	if version, err = manifest.LatestVersion(log, context); err != nil {
		return
	}
	if source, hash, err = manifest.DownloadURLAndHash(context, EC2UpdaterPackageName, version, EC2UpdaterFileName, HTTPFormat, S3Format); err != nil {
		return
	}
	var updateDownloadFolder = ""
	if updateDownloadFolder, err = createUpdateDownloadFolder(); err != nil {
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

		errMessage := fmt.Sprintf("failed to download file reliably, %v", downloadInput.SourceURL)
		if downloadErr != nil {
			errMessage = fmt.Sprintf("%v, %v", errMessage, downloadErr.Error())
		}
		return version, errors.New(errMessage)
	}

	out.AppendInfof("Successfully downloaded %v", downloadInput.SourceURL)
	if uncompressErr := fileUncompress(
		log,
		downloadOutput.LocalFilePath,
		updateutil.UpdateArtifactFolder(appconfig.EC2UpdateArtifactsRoot, updaterPackageName, version)); uncompressErr != nil {
		return version, fmt.Errorf("failed to uncompress updater package, %v, %v",
			downloadOutput.LocalFilePath,
			uncompressErr.Error())
	}

	return version, nil
}

// validateUpdate validates manifest against update request
func (m *updateManager) validateUpdate(log log.T,
	pluginInput *UpdatePluginInput,
	context *updateutil.InstanceContext,
	manifest *Manifest,
	out iohandler.IOHandler, currentVersion string) (noNeedToUpdate bool, err error) {
	var allowDowngrade = false

	if len(pluginInput.TargetVersion) == 0 {
		if pluginInput.TargetVersion, err = manifest.LatestVersion(log, context); err != nil {
			return true, err
		}
	}

	if allowDowngrade, err = strconv.ParseBool(pluginInput.AllowDowngrade); err != nil {
		return true, err
	}

	if pluginInput.TargetVersion == currentVersion {
		out.AppendInfof("%v %v has already been installed, update skipped",
			pluginInput.AgentName,
			currentVersion)
		out.MarkAsSucceeded()
		return true, nil
	}
	if pluginInput.TargetVersion < currentVersion && !allowDowngrade {
		return true,
			fmt.Errorf(
				"updating %v to an older version, please enable allow downgrade to proceed",
				pluginInput.AgentName)

	}
	if !manifest.HasVersion(context, pluginInput.TargetVersion) {
		return true,
			fmt.Errorf(
				"%v version %v is unsupported",
				pluginInput.AgentName,
				pluginInput.TargetVersion)
	}
	if !manifest.HasVersion(context, currentVersion) {
		return true,
			fmt.Errorf(
				"%v current version %v is unsupported on current platform",
				pluginInput.AgentName,
				currentVersion)
	}

	return false, nil
}

// TODO Make common methods go into utility/helper/common package. Check if Execute can be added to that package
// Execute runs multiple sets of commands and returns their outputs.
func (p *Plugin) Execute(context context.T, config contracts.Configuration, cancelFlag task.CancelFlag, output iohandler.IOHandler) {
	log := context.Log()
	log.Info("RunCommand started with update configuration for EC2 config update ", config)
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
			output,
			time.Now())
	}
	return
}

// Name returns the plugin name
func Name() string {
	return appconfig.PluginEC2ConfigUpdate
}

// GetUpdatePluginConfig returns the default values for the update plugin
func GetUpdatePluginConfig(context context.T) UpdatePluginConfig {
	log := context.Log()
	region, err := platform.Region()
	if err != nil {
		log.Errorf("Error retrieving agent region in update plugin config. error: %v", err)
	}

	var manifestURL string
	manifestURL = retrieveDynamicS3ManifestUrl(region, "s3")
	if manifestURL == "" {
		if strings.HasPrefix(region, s3util.ChinaRegionPrefix) {
			manifestURL = ChinaManifestURL
		} else {
			manifestURL = CommonManifestURL
		}
	}

	return UpdatePluginConfig{
		ManifestLocation: manifestURL,
	}
}

func retrieveDynamicS3ManifestUrl(region string, service string) string {
	if dynamicS3Endpoint := platform.GetDefaultEndPoint(region, service); dynamicS3Endpoint != "" {
		return "https://" + dynamicS3Endpoint + ManifestPath
	}
	return ""
}
