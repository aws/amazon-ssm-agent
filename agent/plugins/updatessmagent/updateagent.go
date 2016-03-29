// Copyright 2016 Amazon.com, Inc. or its affiliates. All Rights Reserved.
//
// Licensed under the Amazon Software License (the "License"). You may not
// use this file except in compliance with the License. A copy of the
// License is located at
//
// http://aws.amazon.com/asl/
//
// or in the "license" file accompanying this file. This file is distributed
// on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND,
// express or implied. See the License for the specific language governing
// permissions and limitations under the License.

// Package updatessmagent implements the UpdateSsmAgent plugin.
package updatessmagent

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/aws/amazon-ssm-agent/agent/appconfig"
	"github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/aws/amazon-ssm-agent/agent/contracts"
	"github.com/aws/amazon-ssm-agent/agent/fileutil"
	"github.com/aws/amazon-ssm-agent/agent/fileutil/artifact"
	"github.com/aws/amazon-ssm-agent/agent/jsonutil"
	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/amazon-ssm-agent/agent/rebooter"
	"github.com/aws/amazon-ssm-agent/agent/task"
	"github.com/aws/amazon-ssm-agent/agent/updateutil"
	"github.com/aws/amazon-ssm-agent/agent/version"
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

// UpdatePluginOutput represents the output of the plugin
type UpdatePluginOutput struct {
	contracts.PluginOutput
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
		out *UpdatePluginOutput) (noNeedToUpdate bool, err error)
}

// Assign method to global variables to allow unittest to override
var getAppConfig = appconfig.GetConfig
var fileDownload = artifact.Download
var fileUncompress = fileutil.Uncompress
var updateAgent = runUpdateAgent

//Succeed marks update as Successful
func (out *UpdatePluginOutput) Succeed() {
	out.ExitCode = 0
	out.Status = contracts.ResultStatusSuccess
}

//Failed marks update as Failed
func (out *UpdatePluginOutput) Failed(log log.T, err error) {
	out.ExitCode = 1
	out.Status = contracts.ResultStatusFailed
	if len(out.Stderr) != 0 {
		out.Stderr = fmt.Sprintf("\n%v\n%v", out.Stderr, err.Error())
	} else {
		out.Stderr = fmt.Sprintf("\n%v", err.Error())
	}
	log.Error(err.Error())
	out.Errors = append(out.Errors, err.Error())
}

//Pending mark update as Pending
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
func NewPlugin() (*Plugin, error) {
	config, err := getAppConfig(false)

	if err != nil {
		return nil, err
	}
	pluginConf, ok := config.Plugins[Name()]
	if !ok {
		return nil, fmt.Errorf("Missing configuration for plugin %v", Name())
	}

	var plugin Plugin
	err = jsonutil.Remarshal(pluginConf, &plugin)
	if err != nil {
		return nil, err
	}

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
	outputS3BucketName string,
	outputS3KeyPrefix string,
	startTime time.Time) (out UpdatePluginOutput) {
	var pluginInput UpdatePluginInput
	var err error
	var context *updateutil.InstanceContext

	if err = jsonutil.Remarshal(rawPluginInput, &pluginInput); err != nil {
		out.Failed(log,
			fmt.Errorf("invalid format in plugin properties %v;\nerror %v", rawPluginInput, err))
		return
	}

	if context, err = util.CreateInstanceContext(log); err != nil {
		out.Failed(log, err)
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
	out.AppendInfo(log, "Updating %v from %v to %v",
		pluginInput.AgentName,
		version.Version,
		targetVersion)

	//Download manifest file
	manifest, downloadErr := manager.downloadManifest(log, util, &pluginInput, context, &out)
	if downloadErr != nil {
		out.Failed(log, downloadErr)
		return
	}

	//Validate update details
	noNeedToUpdate := false
	if noNeedToUpdate, err = manager.validateUpdate(log, &pluginInput, context, manifest, &out); noNeedToUpdate {
		if err != nil {
			out.Failed(log, err)
		}
		return
	}

	//Download updater and retrieve the version number
	updaterVersion := ""
	if updaterVersion, err = manager.downloadUpdater(
		log, util, pluginInput.UpdaterName, manifest, &out, context); err != nil {
		out.Failed(log, err)
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
		p.StdoutFileName,
		p.StderrFileName,
		outputS3KeyPrefix,
		outputS3BucketName); err != nil {
		out.Failed(log, err)
		return
	}
	log.Debugf("Update command %v", cmd)

	//Save update plugin result to local file, updater will read it during agent update
	updatePluginResult := &updateutil.UpdatePluginResult{
		StandOut:      out.Stdout,
		StartDateTime: startTime,
	}
	if err = util.SaveUpdatePluginResult(log, appconfig.UpdaterArtifactsRoot, updatePluginResult); err != nil {
		out.Failed(log, err)
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
		p.StdoutFileName,
		p.StderrFileName,
		true); err != nil {
		out.Failed(log, err)
		return
	}

	out.Pending()
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
	out *UpdatePluginOutput) (manifest *Manifest, err error) {
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
	out.AppendInfo(log, "Successfully downloaded %v", downloadInput.SourceURL)
	return ParseManifest(log, downloadOutput.LocalFilePath, context, pluginInput.AgentName)
}

//downloadUpdater downloads updater from the s3 bucket
func (m *updateManager) downloadUpdater(log log.T,
	util updateutil.T,
	updaterPackageName string,
	manifest *Manifest,
	out *UpdatePluginOutput,
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
		return version, fmt.Errorf(errMessage)
	}
	out.AppendInfo(log, "Successfully downloaded %v", downloadInput.SourceURL)
	if uncompressErr := fileUncompress(
		downloadOutput.LocalFilePath,
		updateutil.UpdateArtifactFolder(appconfig.UpdaterArtifactsRoot, updaterPackageName, version)); uncompressErr != nil {
		return version, fmt.Errorf("failed to uncompress updater package, %v, %v",
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
	out *UpdatePluginOutput) (noNeedToUpdate bool, err error) {
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

	if pluginInput.TargetVersion == currentVersion {
		out.AppendInfo(log, "%v %v has already been installed, update skipped",
			pluginInput.AgentName,
			currentVersion)
		out.Succeed()
		return true, nil
	}
	if pluginInput.TargetVersion < currentVersion && !allowDowngrade {
		return true,
			fmt.Errorf(
				"updating %v to an older version, please enable allow downgrade to proceed",
				pluginInput.AgentName)

	}
	if !manifest.HasVersion(context, pluginInput.AgentName, pluginInput.TargetVersion) {
		return true,
			fmt.Errorf(
				"%v version %v is unsupported",
				pluginInput.AgentName,
				pluginInput.TargetVersion)
	}
	if !manifest.HasVersion(context, pluginInput.AgentName, currentVersion) {
		return true,
			fmt.Errorf(
				"%v current version %v is unsupported on current platform",
				pluginInput.AgentName,
				currentVersion)
	}

	return false, nil
}

// Execute runs multiple sets of commands and returns their outputs.
// res.Output will contain a slice of RunCommandPluginOutput.
func (p *Plugin) Execute(context context.T, config contracts.Configuration, cancelFlag task.CancelFlag) (res contracts.PluginResult) {
	log := context.Log()
	log.Info("RunCommand started with configuration ", config)
	util := new(updateutil.Utility)
	manager := new(updateManager)

	res.StartDateTime = time.Now()
	defer func() { res.EndDateTime = time.Now() }()

	out := make([]UpdatePluginOutput, len(config.Properties))
	for i, prop := range config.Properties {
		// check if a reboot has been requested
		if rebooter.RebootRequested() {
			log.Info("A plugin has requested a reboot.")
			break
		}

		if cancelFlag.ShutDown() {
			out[i] = UpdatePluginOutput{}
			out[i].Errors = []string{"Task canceled due to ShutDown"}
			break
		} else if cancelFlag.Canceled() {
			out[i] = UpdatePluginOutput{}
			out[i].Errors = []string{"Task canceled"}
			break
		}

		out[i] = updateAgent(p,
			config,
			log,
			manager,
			util,
			prop,
			cancelFlag,
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
	return appconfig.PluginNameAwsAgentUpdate
}
