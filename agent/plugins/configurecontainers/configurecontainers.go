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

package configurecontainers

import (
	"fmt"
	"io/ioutil"
	"path/filepath"
	"time"

	"github.com/aws/amazon-ssm-agent/agent/appconfig"
	"github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/aws/amazon-ssm-agent/agent/contracts"
	"github.com/aws/amazon-ssm-agent/agent/executers"
	"github.com/aws/amazon-ssm-agent/agent/fileutil"
	"github.com/aws/amazon-ssm-agent/agent/framework/runpluginutil"
	"github.com/aws/amazon-ssm-agent/agent/jsonutil"
	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/amazon-ssm-agent/agent/plugins/pluginutil"
	"github.com/aws/amazon-ssm-agent/agent/task"
)

const (
	//Action values
	INSTALL   = "Install"
	UNINSTALL = "Uninstall"
)

// Plugin is the type for the plugin.
type Plugin struct {
	pluginutil.DefaultPlugin
}

// ConfigureContainerPluginInput represents one set of commands executed by the configure container plugin.
type ConfigureContainerPluginInput struct {
	contracts.PluginInput
	ID     string
	Action string
}

// NewPlugin returns a new instance of the plugin.
func NewPlugin(pluginConfig pluginutil.PluginConfig) (*Plugin, error) {
	var plugin Plugin
	var err error
	plugin.MaxStdoutLength = pluginConfig.MaxStdoutLength
	plugin.MaxStderrLength = pluginConfig.MaxStderrLength
	plugin.StdoutFileName = pluginConfig.StdoutFileName
	plugin.StderrFileName = pluginConfig.StderrFileName
	plugin.OutputTruncatedSuffix = pluginConfig.OutputTruncatedSuffix
	plugin.ExecuteUploadOutputToS3Bucket = pluginutil.UploadOutputToS3BucketExecuter(plugin.UploadOutputToS3Bucket)
	plugin.CommandExecuter = executers.ShellCommandExecuter{}

	return &plugin, err
}

// Name returns the name of the plugin
func Name() string {
	return appconfig.PluginNameConfigureDocker
}

// Execute runs multiple sets of commands and returns their outputs.
// res.Output will contain a slice of ConfigureContainerPluginOutput.
func (p *Plugin) Execute(context context.T, config contracts.Configuration, cancelFlag task.CancelFlag, pluginRunner runpluginutil.PluginRunner) (res contracts.PluginResult) {
	log := context.Log()
	log.Infof("%v started with configuration %v", Name(), config)
	res.StartDateTime = time.Now()
	defer func() {
		res.EndDateTime = time.Now()
	}()

	var properties []interface{}
	if properties = pluginutil.LoadParametersAsList(log, config.Properties, &res); res.Code != 0 {
		pluginutil.PersistPluginInformationToCurrent(log, config.PluginID, config, res)
		pluginutil.PersistPluginInformationToCurrent(log, config.PluginID, config, res)
		return res
	}

	out := contracts.PluginOutput{}
	for _, prop := range properties {

		if cancelFlag.ShutDown() {
			out.MarkAsShutdown()
			break
		}

		if cancelFlag.Canceled() {
			out.MarkAsCancelled()
			break
		}

		out.Merge(log, p.runCommandsRawInput(log, config.PluginID, prop, config.OrchestrationDirectory, cancelFlag, config.OutputS3BucketName, config.OutputS3KeyPrefix))
	}

	res.Code = out.ExitCode
	res.Status = out.Status
	res.Output = out.String()
	res.StandardOutput = pluginutil.StringPrefix(out.Stdout, p.MaxStdoutLength, p.OutputTruncatedSuffix)
	res.StandardError = pluginutil.StringPrefix(out.Stderr, p.MaxStderrLength, p.OutputTruncatedSuffix)

	pluginutil.PersistPluginInformationToCurrent(log, config.PluginID, config, res)

	return res
}

// runCommandsRawInput executes one set of commands and returns their output.
// The input is in the default json unmarshal format (e.g. map[string]interface{}).
func (p *Plugin) runCommandsRawInput(log log.T, pluginID string, rawPluginInput interface{}, orchestrationDirectory string, cancelFlag task.CancelFlag, outputS3BucketName string, outputS3KeyPrefix string) (out contracts.PluginOutput) {
	var pluginInput ConfigureContainerPluginInput
	err := jsonutil.Remarshal(rawPluginInput, &pluginInput)
	log.Debugf("Plugin input %v", pluginInput)
	if err != nil {
		errorString := fmt.Errorf("Invalid format in plugin properties %v;\nerror %v", rawPluginInput, err)
		out.MarkAsFailed(log, errorString)
		return
	}

	return p.runCommands(log, pluginID, pluginInput, orchestrationDirectory, cancelFlag, outputS3BucketName, outputS3KeyPrefix)
}

// runCommands executes one set of commands and returns their output.
func (p *Plugin) runCommands(log log.T, pluginID string, pluginInput ConfigureContainerPluginInput, orchestrationDirectory string, cancelFlag task.CancelFlag, outputS3BucketName string, outputS3KeyPrefix string) (out contracts.PluginOutput) {
	var err error

	// TODO:MF: This subdirectory is only needed because we could be running multiple sets of properties for the same plugin - otherwise the orchestration directory would already be unique
	orchestrationDir := fileutil.BuildPath(orchestrationDirectory, pluginInput.ID)
	log.Debugf("OrchestrationDir %v ", orchestrationDir)

	// create orchestration dir if needed
	if err = fileutil.MakeDirs(orchestrationDir); err != nil {
		log.Debug("failed to create orchestrationDir directory", orchestrationDir, err)
		out.MarkAsFailed(log, err)
		return
	}
	log.Info("********************************starting configure Docker plugin**************************************")
	switch pluginInput.Action {
	case INSTALL:
		out = runInstallCommands(log, pluginInput, orchestrationDir)
	case UNINSTALL:
		out = runUninstallCommands(log, pluginInput, orchestrationDir)

	default:
		out.MarkAsFailed(log, fmt.Errorf("configure Action is set to unsupported value: %v", pluginInput.Action))
		return out
	}

	if outputS3BucketName != "" {
		// Create output file paths
		stdoutFilePath := filepath.Join(orchestrationDir, p.StdoutFileName)
		stderrFilePath := filepath.Join(orchestrationDir, p.StderrFileName)
		log.Debugf("stdout file %v, stderr file %v", stdoutFilePath, stderrFilePath)
		if err = ioutil.WriteFile(stdoutFilePath, []byte(out.Stdout), 0644); err != nil {
			log.Error(err)
		}
		if err = ioutil.WriteFile(stderrFilePath, []byte(out.Stderr), 0644); err != nil {
			log.Error(err)
		}
		// Upload output to S3
		s3PluginID := pluginInput.ID
		if s3PluginID == "" {
			s3PluginID = pluginID
		}
		uploadOutputToS3BucketErrors := p.ExecuteUploadOutputToS3Bucket(log, s3PluginID, orchestrationDir, outputS3BucketName, outputS3KeyPrefix, false, "", out.Stdout, out.Stderr)
		if len(uploadOutputToS3BucketErrors) > 0 {
			log.Errorf("Unable to upload the logs: %s", uploadOutputToS3BucketErrors)
		}

	}
	// Return Json indented response
	responseContent, _ := jsonutil.Marshal(out)
	log.Debug("Returning response:\n", jsonutil.Indent(responseContent))
	log.Info("********************************completing configure Docker plugin**************************************")
	return
}
