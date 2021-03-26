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
	"time"

	"github.com/aws/amazon-ssm-agent/agent/appconfig"
	"github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/aws/amazon-ssm-agent/agent/contracts"
	"github.com/aws/amazon-ssm-agent/agent/fileutil"
	"github.com/aws/amazon-ssm-agent/agent/framework/processor/executer/iohandler"
	"github.com/aws/amazon-ssm-agent/agent/jsonutil"
	"github.com/aws/amazon-ssm-agent/agent/task"
	"github.com/aws/amazon-ssm-agent/agent/updateutil"
	"github.com/aws/amazon-ssm-agent/agent/updateutil/updateconstants"
	"github.com/aws/amazon-ssm-agent/agent/updateutil/updateinfo"
	"github.com/aws/amazon-ssm-agent/agent/updateutil/updatemanifest"
	"github.com/aws/amazon-ssm-agent/agent/updateutil/updates3util"
	"github.com/aws/amazon-ssm-agent/agent/version"
	"github.com/aws/amazon-ssm-agent/core/executor"
	"github.com/nightlyone/lockfile"
)

const (
	noOfRetries          = 2
	updateRetryDelayBase = 1000 // 1000 millisecond
	updateRetryDelay     = 500  // 500 millisecond
)

// Plugin is the type for the RunCommand plugin.
type Plugin struct {
	Context context.T
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

// Assign method to global variables to allow unittest to override
var currentAgentVersion = version.Version
var updateAgent = runUpdateAgent
var getLockObj = lockfile.New
var updateUtilRef updateutil.T // added mainly for testing

// NewPlugin returns a new instance of the plugin.
func NewPlugin(context context.T) (*Plugin, error) {
	return &Plugin{
		context,
	}, nil
}

// updateAgent downloads the installation packages and update the agent
func runUpdateAgent(
	config contracts.Configuration,
	context context.T,
	util updateutil.T,
	s3util updates3util.T,
	manifest updatemanifest.T,
	rawPluginInput interface{},
	output iohandler.IOHandler,
	startTime time.Time,
	exec executor.IExecutor,
	downloadFolder string) (pid int) {
	log := context.Log()
	var pluginInput UpdatePluginInput
	var err error

	pluginConfig := iohandler.DefaultOutputConfig()

	if err = jsonutil.Remarshal(rawPluginInput, &pluginInput); err != nil {
		output.MarkAsFailed(fmt.Errorf("invalid format in plugin properties %v;\nerror %v", rawPluginInput, err))
		return
	}

	//Calculate updater package name base on agent name
	pluginInput.UpdaterName = pluginInput.AgentName + updateconstants.UpdaterPackageNamePrefix

	// if TargetVersion is Empty, set to None and it will be resolved in the updater
	if len(pluginInput.TargetVersion) == 0 {
		pluginInput.TargetVersion = "None"
	}

	//Download manifest file and populate manifest object
	if downloadErr := s3util.DownloadManifest(manifest, pluginInput.Source); downloadErr != nil {
		output.MarkAsFailed(downloadErr)
		return
	}
	output.AppendInfo("Successfully downloaded manifest\n")

	//Download updater and retrieve the version number
	updaterVersion := ""
	if updaterVersion, err = s3util.DownloadUpdater(manifest, pluginInput.UpdaterName, downloadFolder); err != nil {
		output.MarkAsFailed(err)
		return
	}
	output.AppendInfof("Successfully downloaded updater version %s\n", updaterVersion)

	//Generate update command base on the update detail
	cmd := ""
	if cmd, err = generateUpdateCmd(
		&pluginInput,
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
	if isDiskSpaceSufficient, err := util.IsDiskSpaceSufficientForUpdate(log); !isDiskSpaceSufficient || err != nil {
		if err != nil {
			output.MarkAsFailed(err)
			return
		}
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

func (p *Plugin) Execute(config contracts.Configuration, cancelFlag task.CancelFlag, output iohandler.IOHandler) {
	log := p.Context.Log()
	log.Info("RunCommand started with configuration ", config)
	if updateUtilRef == nil {
		updateUtilRef = &updateutil.Utility{
			Context: p.Context,
		}
	}
	executor := executor.NewProcessExecutor(log)

	updateInfo, err := updateinfo.New(p.Context)

	if err != nil {
		log.Warnf("Failed to create update info object: %s", err)
		output.MarkAsFailed(fmt.Errorf("Failed to create update info object: %s", err))
		return
	}

	manifest := updatemanifest.New(p.Context, updateInfo)
	updateS3Util := updates3util.New(p.Context)

	if cancelFlag.ShutDown() {
		output.MarkAsShutdown()
	} else if cancelFlag.Canceled() {
		output.MarkAsCancelled()
	} else {
		// create update directory before creating locks
		var downloadFolder string
		var directoryErr error
		if downloadFolder, directoryErr = updateUtilRef.CreateUpdateDownloadFolder(); directoryErr != nil {
			log.Warnf("error while creating update directory: %v", directoryErr)
		}

		// First check if lock is locked by anyone
		lock, _ := getLockObj(appconfig.UpdaterPidLockfile)
		err = lock.TryLockExpireWithRetry(updateconstants.UpdateLockFileMinutes)

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

		pid := updateAgent(
			config,
			p.Context,
			updateUtilRef,
			updateS3Util,
			manifest,
			config.Properties,
			output,
			time.Now(),
			executor,
			downloadFolder)

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

func generateUpdateCmd(
	pluginInput *UpdatePluginInput,
	updaterVersion string,
	messageID string,
	stdout string,
	stderr string,
	keyPrefix string,
	bucketName string) (cmd string, err error) {
	updaterPath := updateutil.UpdaterFilePath(appconfig.UpdaterArtifactsRoot, pluginInput.UpdaterName, updaterVersion)
	cmd = updaterPath + " -update"

	cmd = updateutil.BuildUpdateCommand(cmd, updateconstants.SourceVersionCmd, currentAgentVersion)
	cmd = updateutil.BuildUpdateCommand(cmd, updateconstants.TargetVersionCmd, pluginInput.TargetVersion)

	cmd = updateutil.BuildUpdateCommand(cmd, updateconstants.PackageNameCmd, pluginInput.AgentName)
	cmd = updateutil.BuildUpdateCommand(cmd, updateconstants.MessageIDCmd, messageID)

	cmd = updateutil.BuildUpdateCommand(cmd, updateconstants.StdoutFileName, stdout)
	cmd = updateutil.BuildUpdateCommand(cmd, updateconstants.StderrFileName, stderr)

	cmd = updateutil.BuildUpdateCommand(cmd, updateconstants.OutputKeyPrefixCmd, keyPrefix)
	cmd = updateutil.BuildUpdateCommand(cmd, updateconstants.OutputBucketNameCmd, bucketName)

	cmd = updateutil.BuildUpdateCommand(cmd, updateconstants.ManifestFileUrlCmd, pluginInput.Source)

	allowDowngrade, err := strconv.ParseBool(pluginInput.AllowDowngrade)
	if err != nil {
		return "", err
	}

	// Tell the updater if downgrade is not allowed
	if !allowDowngrade {
		cmd += " -" + updateconstants.DisableDowngradeCmd
	}

	return
}
