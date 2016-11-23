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
	"github.com/aws/amazon-ssm-agent/agent/framework/runpluginutil"
	"github.com/aws/amazon-ssm-agent/agent/jsonutil"
	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/amazon-ssm-agent/agent/platform"
	"github.com/aws/amazon-ssm-agent/agent/plugins/pluginutil"
	"github.com/aws/amazon-ssm-agent/agent/rebooter"
	"github.com/aws/amazon-ssm-agent/agent/task"
	"github.com/aws/amazon-ssm-agent/agent/updateutil"
)

// Plugin is the type for the RunCommand plugin.
type Plugin struct {
	// UploadToS3Sync is true if uploading to S3 should be done synchronously, false for async.
	UploadToS3Sync bool

	// StdoutFileName is the name of the file that stores standard output.
	StdoutFileName string

	// StderrFileName is the name of the file that stores standard error.
	StderrFileName string

	// MaxStdoutLength is the maximum length of the standard output returned in the plugin result.
	// If the output is longer, it will be truncated. The full output will be uploaded to s3.
	MaxStdoutLength int

	// MaxStderrLength is the maximum length of the standard error returned in the plugin result.
	MaxStderrLength int

	// OutputTruncatedSuffix is an optional suffix that is inserted at the end of the truncated stdout/stderr.
	OutputTruncatedSuffix string

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
	ManifestLocation      string
	StdoutFileName        string
	StderrFileName        string
	MaxStdoutLength       int
	MaxStderrLength       int
	OutputTruncatedSuffix string
}

// UpdatePluginOutput represents the output of the plugin
type UpdatePluginOutput struct {
	contracts.PluginOutput
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
		messageID string) (cmd string, err error)

	generateUpdateCmd(log log.T,
		updaterPath string) (cmd string, err error)

	downloadManifest(log log.T,
		util updateutil.T,
		pluginInput *UpdatePluginInput,
		context *updateutil.InstanceContext,
		out *UpdatePluginOutput) (manifest *Manifest, err error)

	downloadUpdater(log log.T,
		util updateutil.T,
		updaterPackageName string,
		manifest *Manifest,
		out *UpdatePluginOutput,
		context *updateutil.InstanceContext) (version string, err error)

	validateUpdate(log log.T,
		pluginInput *UpdatePluginInput,
		context *updateutil.InstanceContext,
		manifest *Manifest,
		out *UpdatePluginOutput, version string) (noNeedToUpdate bool, err error)

	loadUpdateContext(log log.T,
		path string) (updateContext *UpdateContextFile, err error)
}

// Assign method to global variables to allow unittest to override
var fileDownload = artifact.Download
var fileUncompress = fileutil.Uncompress
var updateAgent = runUpdateAgent
var mkDirAll = os.MkdirAll
var agentVersion string

// setToSuccess marks update as Successful
func (out *UpdatePluginOutput) setToSuccess() {
	out.ExitCode = 0
	out.Status = contracts.ResultStatusSuccess
}

// Pending mark update as Pending
func (out *UpdatePluginOutput) Pending() {
	out.ExitCode = 0
	out.Status = contracts.ResultStatusInProgress
}

// AppendInfo adds info to UpdateContext StandardOut
func (out *UpdatePluginOutput) AppendInfo(log log.T, format string, params ...interface{}) {
	message := fmt.Sprintf(format, params...)
	log.Info(message)
	out.Stdout = fmt.Sprintf("%v\n%v", out.Stdout, message)
}

// NewPlugin returns a new instance of the plugin.
func NewPlugin(updatePluginConfig UpdatePluginConfig) (*Plugin, error) {
	var plugin Plugin
	plugin.ManifestLocation = updatePluginConfig.ManifestLocation
	plugin.MaxStdoutLength = updatePluginConfig.MaxStdoutLength
	plugin.MaxStderrLength = updatePluginConfig.MaxStderrLength
	plugin.StdoutFileName = updatePluginConfig.StdoutFileName
	plugin.StderrFileName = updatePluginConfig.StderrFileName
	plugin.OutputTruncatedSuffix = updatePluginConfig.OutputTruncatedSuffix
	return &plugin, nil
}

// getEC2ConfigCurrentVersion gets the current version of EC2 config installed on the platform
func getEC2ConfigCurrentVersion(log log.T) string {
	cmdName := "wmic"
	var err error
	cmdArgs := []string{"DATAFILE", "where", "name='C:\\\\Program Files\\\\Amazon\\\\Ec2ConfigService\\\\Ec2Config.exe'", "get", "Version", "/format:list"}
	var cmdOut []byte
	if cmdOut, err = exec.Command(cmdName, cmdArgs...).Output(); err != nil {
		log.Errorf("There was an error running %v %v. Error = %s", cmdName, cmdArgs, err)
		return minimumVersion
	}

	contents := string(cmdOut)
	data := strings.Split(contents, "=")
	if len(data) > 1 {
		version := strings.TrimSpace(data[1])
		versionsplit := strings.Split(version, ".")
		version = strings.TrimSuffix(version, versionsplit[3])
		res := strings.TrimSuffix(version, ".")
		log.Debug("GetEC2ConfigCurrentVersion: current version is ", res)
		return res
	}

	return minimumVersion
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
	outputS3BucketName string,
	outputS3KeyPrefix string,
	startTime time.Time) (out UpdatePluginOutput) {
	var pluginInput UpdatePluginInput
	var updatecontext *UpdateContextFile = new(UpdateContextFile)
	var err error
	var context *updateutil.InstanceContext

	if err = jsonutil.Remarshal(rawPluginInput, &pluginInput); err != nil {
		out.MarkAsFailed(log,
			fmt.Errorf("invalid format in plugin properties %v;\nerror %v", rawPluginInput, err))
		return
	}

	if context, err = util.CreateInstanceContext(log); err != nil {
		out.MarkAsFailed(log, err)
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

	agentVersion = minimumVersion
	agentVersion = getEC2ConfigCurrentVersion(log)

	// If disk space is not sufficient, fail the update to prevent installation and notify user in output
	// If loading disk space fails, continue to update (agent update is backed by rollback handler)
	log.Infof("Checking available disk space ...")
	if isDiskSpaceSufficient, err := util.IsDiskSpaceSufficientForUpdate(log); err == nil && !isDiskSpaceSufficient {
		out.MarkAsFailed(log, errors.New("Insufficient available disk space"))
		return
	}

	out.AppendInfo(log, "Updating %v from %v to %v",
		pluginInput.AgentName,
		agentVersion,
		targetVersion)

	if updatecontext, err = manager.loadUpdateContext(log, appconfig.UpdateContextFilePath); err != nil {
		log.Error("Update context load error: ", err)
	}

	//Update only when no other update process is running
	if updatecontext.UpdateState != notStarted && updatecontext.UpdateState != completed { //update process is running
		out.MarkAsFailed(log, fmt.Errorf("Another update in progress, try again later"))
	} else { //if update process is not running

		//Download manifest file
		manifest, downloadErr := manager.downloadManifest(log, util, &pluginInput, context, &out)
		if downloadErr != nil {
			out.MarkAsFailed(log, downloadErr)
			return
		}

		//Validate update details
		noNeedToUpdate := false
		if noNeedToUpdate, err = manager.validateUpdate(log, &pluginInput, context, manifest, &out, agentVersion); noNeedToUpdate {
			if err != nil {
				out.MarkAsFailed(log, err)
			}
			return
		}

		//Download updater and retrieve the version number
		updaterVersion := ""
		if updaterVersion, err = manager.downloadUpdater(
			log, util, pluginInput.UpdaterName, manifest, &out, context); err != nil {
			out.MarkAsFailed(log, err)
			return
		}

		//Generate update command base on the update detail
		cmd := ""
		if cmd, err = manager.generateSetupUpdateCmd(log,
			manifest,
			&pluginInput,
			context,
			UpdaterFilePath(appconfig.EC2UpdateArtifactsRoot, pluginInput.UpdaterName, updaterVersion),
			config.MessageId); err != nil {
			out.MarkAsFailed(log, err)
			return
		}
		log.Debugf("Setup update command %v", cmd)

		//Save update plugin result to local file, updater will read it during agent update
		updatePluginResult := &updateutil.UpdatePluginResult{
			StandOut:      out.Stdout,
			StartDateTime: startTime,
		}
		if err = util.SaveUpdatePluginResult(log, appconfig.EC2UpdateArtifactsRoot, updatePluginResult); err != nil {
			out.MarkAsFailed(log, err)
			return
		}

		workDir := updateutil.UpdateArtifactFolder(
			appconfig.EC2UpdateArtifactsRoot, pluginInput.UpdaterName, updaterVersion)

		//Command to setup the installation
		if err = util.ExeCommand(log, cmd, workDir, appconfig.EC2UpdateArtifactsRoot, p.StdoutFileName, p.StderrFileName, false); err != nil {
			out.MarkAsFailed(log, err)
			return
		}
		cmd = ""

		log.Infof("Start Installation")
		log.Infof("Hand over update process to %v", pluginInput.UpdaterName)
		//Execute updater, hand over the update process
		if cmd, err = manager.generateUpdateCmd(log,
			UpdaterFilePath(appconfig.EC2UpdateArtifactsRoot, pluginInput.UpdaterName, updaterVersion)); err != nil {
			out.MarkAsFailed(log, err)
			return
		}
		log.Debugf("Setup update command %v", cmd)
		if err = util.ExeCommand(log, cmd, workDir, appconfig.EC2UpdateArtifactsRoot, p.StdoutFileName, p.StderrFileName, true); err != nil {
			out.MarkAsFailed(log, err)
			return
		}
		out.Pending()
	}
	return out
}

// TODO Create a command package for command execution
// generateSetupUpdateCmd generates cmd to setup the installation process
func (m *updateManager) generateSetupUpdateCmd(log log.T,
	manifest *Manifest,
	pluginInput *UpdatePluginInput,
	context *updateutil.InstanceContext,
	updaterPath string,
	messageID string) (cmd string, err error) {
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
	out *UpdatePluginOutput) (manifest *Manifest, err error) {
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
	out.AppendInfo(log, "Successfully downloaded %v", downloadInput.SourceURL)
	return ParseManifest(log, downloadOutput.LocalFilePath)
}

// downloadUpdater downloads updater from the s3 bucket
func (m *updateManager) downloadUpdater(log log.T,
	util updateutil.T,
	updaterPackageName string,
	manifest *Manifest,
	out *UpdatePluginOutput,
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
		SourceURL:            source,
		SourceHashValue:      hash,
		SourceHashType:       updateutil.HashType,
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

	out.AppendInfo(log, "Successfully downloaded %v", downloadInput.SourceURL)
	if uncompressErr := fileUncompress(
		downloadOutput.LocalFilePath,
		updateutil.UpdateArtifactFolder(appconfig.EC2UpdateArtifactsRoot, updaterPackageName, version)); uncompressErr != nil {
		return version, fmt.Errorf("failed to uncompress updater package, %v, %v",
			downloadOutput.LocalFilePath,
			uncompressErr.Error())
	}
	out.AppendInfo(log, "Successfully downloaded %v", downloadInput.SourceURL)

	return version, nil
}

// validateUpdate validates manifest against update request
func (m *updateManager) validateUpdate(log log.T,
	pluginInput *UpdatePluginInput,
	context *updateutil.InstanceContext,
	manifest *Manifest,
	out *UpdatePluginOutput, currentVersion string) (noNeedToUpdate bool, err error) {
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
		out.AppendInfo(log, "%v %v has already been installed, update skipped",
			pluginInput.AgentName,
			currentVersion)
		out.setToSuccess()
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
// res.Output will contain a slice of RunCommandPluginOutput.
func (p *Plugin) Execute(context context.T, config contracts.Configuration, cancelFlag task.CancelFlag, subDocumentRunner runpluginutil.PluginRunner) (res contracts.PluginResult) {
	log := context.Log()
	log.Info("RunCommand started with update configuration for EC2 config update ", config)
	util := new(updateutil.Utility)
	manager := new(updateManager)

	res.StartDateTime = time.Now()
	defer func() { res.EndDateTime = time.Now() }()

	test := make([]interface{}, 1)
	test[0] = config.Properties

	//loading Properties as list since aws:updateSsmAgent uses properties as list
	var properties []interface{}
	if properties, res = pluginutil.LoadParametersAsList(log, test); res.Code != 0 {
		return res
	}

	out := make([]UpdatePluginOutput, len(properties))
	for i, prop := range properties {
		// check if a reboot has been requested
		if rebooter.RebootRequested() {
			log.Info("A plugin has requested a reboot.")
			return
		}

		if cancelFlag.ShutDown() {
			out[i] = UpdatePluginOutput{}
			out[i].ExitCode = 1
			out[i].Status = contracts.ResultStatusFailed
			break
		} else if cancelFlag.Canceled() {
			out[i] = UpdatePluginOutput{}
			out[i].ExitCode = 1
			out[i].Status = contracts.ResultStatusCancelled
			break
		}

		out[i] = updateAgent(p,
			config,
			log,
			manager,
			util,
			prop,
			config.OutputS3BucketName,
			config.OutputS3KeyPrefix,
			res.StartDateTime)

		res.Code = out[i].ExitCode
		res.Status = out[i].Status
		res.Output = fmt.Sprintf("%v", out[i].String())
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
	if region == "cn-north-1" {
		//TODO test on BJS
		manifestURL = "https://s3.cn-north-1.amazonaws.com.cn/aws-ssm-cn-north-1/manifest.json"
	} else {
		manifestURL = "https://aws-ssm-{Region}.s3.amazonaws.com/manifest.json"
	}

	return UpdatePluginConfig{
		ManifestLocation:      manifestURL,
		StdoutFileName:        "stdout",
		StderrFileName:        "stderr",
		MaxStdoutLength:       2500,
		MaxStderrLength:       2500,
		OutputTruncatedSuffix: "--output truncated--",
	}
}
