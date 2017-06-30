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

// Package psmodule implements the power shell module plugin.
//
// +build windows

package psmodule

import (
	"fmt"
	"io/ioutil"
	"os"
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

// PowerShellModulesDirectory is the directory where PowerShell Modules are installed
var PowerShellModulesDirectory = filepath.Join(os.Getenv("SystemRoot"), "System32", "WindowsPowerShell", "v1.0", "Modules")

// Plugin is the type for the psmodule plugin.
type Plugin struct {
	pluginutil.DefaultPlugin
}

// RunCommandPluginInput represents one set of commands executed by the RunCommand plugin.
type PSModulePluginInput struct {
	contracts.PluginInput
	RunCommand       interface{}
	ParsedCommands   []string
	ID               string
	WorkingDirectory string
	TimeoutSeconds   interface{}
	Source           string
	SourceHash       string
	SourceHashType   string
}

// NewPlugin returns a new instance of the plugin.
func NewPlugin(pluginConfig pluginutil.PluginConfig) (*Plugin, error) {
	var plugin Plugin
	plugin.MaxStdoutLength = pluginConfig.MaxStdoutLength
	plugin.MaxStderrLength = pluginConfig.MaxStderrLength
	plugin.StdoutFileName = pluginConfig.StdoutFileName
	plugin.StderrFileName = pluginConfig.StderrFileName
	plugin.OutputTruncatedSuffix = pluginConfig.OutputTruncatedSuffix
	plugin.ExecuteUploadOutputToS3Bucket = pluginutil.UploadOutputToS3BucketExecuter(plugin.UploadOutputToS3Bucket)
	plugin.CommandExecuter = executers.ShellCommandExecuter{}

	return &plugin, nil
}

// Name returns the name of the plugin
func Name() string {
	return appconfig.PluginNameAwsPowerShellModule
}

// Execute runs multiple sets of commands and returns their outputs.
// res.Output will contain a slice of RunCommandPluginOutput.
func (p *Plugin) Execute(context context.T, config contracts.Configuration, cancelFlag task.CancelFlag, subDocumentRunner runpluginutil.PluginRunner) (res contracts.PluginResult) {
	log := context.Log()
	log.Infof("%v started with configuration %v", Name(), config)
	res.StartDateTime = time.Now()
	defer func() { res.EndDateTime = time.Now() }()

	//loading Properties as list since aws:psModule uses properties as list
	var properties []interface{}
	if properties = pluginutil.LoadParametersAsList(log, config.Properties, &res); res.Code != 0 {

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
	var pluginInput PSModulePluginInput
	err := jsonutil.Remarshal(rawPluginInput, &pluginInput)
	log.Debugf("Plugin input %v", pluginInput)
	if err != nil {
		errorString := fmt.Errorf("Invalid format in plugin properties %v;\nerror %v", rawPluginInput, err)
		out.MarkAsFailed(log, errorString)
		return
	}

	pluginInput.ParsedCommands = pluginutil.ParseRunCommand(pluginInput.RunCommand, pluginInput.ParsedCommands)
	return p.runCommands(log, pluginID, pluginInput, orchestrationDirectory, cancelFlag, outputS3BucketName, outputS3KeyPrefix)
}

// runCommands executes one set of commands and returns their output.
func (p *Plugin) runCommands(log log.T, pluginID string, pluginInput PSModulePluginInput, orchestrationDirectory string, cancelFlag task.CancelFlag, outputS3BucketName string, outputS3KeyPrefix string) (out contracts.PluginOutput) {
	var err error

	// TODO:MF: This subdirectory is only needed because we could be running multiple sets of properties for the same plugin - otherwise the orchestration directory would already be unique
	orchestrationDir := fileutil.BuildPath(orchestrationDirectory, pluginInput.ID)
	log.Debugf("Running commands %v in workingDirectory %v; orchestrationDir %v ", pluginInput.ParsedCommands, pluginInput.WorkingDirectory, orchestrationDir)

	// create orchestration dir if needed
	if err = fileutil.MakeDirsWithExecuteAccess(orchestrationDir); err != nil {
		log.Debug("failed to create orchestrationDir directory", orchestrationDir)
		out.MarkAsFailed(log, err)
		return
	}

	// Create script file path
	scriptPath := filepath.Join(orchestrationDir, appconfig.RunCommandScriptName)
	log.Debugf("Writing commands %v to file %v", pluginInput, scriptPath)

	// Create script file
	if err = pluginutil.CreateScriptFile(log, scriptPath, pluginInput.ParsedCommands, fileutil.ByteOrderMarkSkip); err != nil {
		out.MarkAsFailed(log, fmt.Errorf("failed to create script file. %v", err))
		return
	}

	if pluginInput.Source != "" {
		// Download file from source if available
		downloadOutput, err := pluginutil.DownloadFileFromSource(log, pluginInput.Source, pluginInput.SourceHash, pluginInput.SourceHashType)
		if err != nil || downloadOutput.IsHashMatched == false || downloadOutput.LocalFilePath == "" {
			out.MarkAsFailed(log, fmt.Errorf("failed to download file reliably %v", pluginInput.Source))
			return
		} else {
			// Uncompress the zip file received
			if err = fileutil.Uncompress(downloadOutput.LocalFilePath, PowerShellModulesDirectory); err != nil {
				out.MarkAsFailed(log, fmt.Errorf("Failed to uncompress %v to %v: %v", downloadOutput.LocalFilePath, PowerShellModulesDirectory, err.Error()))
				return
			}
		}
	}

	// Set execution time
	executionTimeout := pluginutil.ValidateExecutionTimeout(log, pluginInput.TimeoutSeconds)

	// Create output file paths
	stdoutFilePath := filepath.Join(orchestrationDir, p.StdoutFileName)
	stderrFilePath := filepath.Join(orchestrationDir, p.StderrFileName)
	log.Debugf("stdout file %v, stderr file %v", stdoutFilePath, stderrFilePath)

	// Construct Command Name and Arguments
	commandName := pluginutil.GetShellCommand()
	commandArguments := append(pluginutil.GetShellArguments(), scriptPath, appconfig.ExitCodeTrap)

	// Execute Command
	stdout, stderr, exitCode, errs := p.CommandExecuter.Execute(log, pluginInput.WorkingDirectory, stdoutFilePath, stderrFilePath, cancelFlag, executionTimeout, commandName, commandArguments)

	// Set output status
	out.ExitCode = exitCode
	out.Status = pluginutil.GetStatus(out.ExitCode, cancelFlag)

	if len(errs) > 0 {
		for _, err := range errs {
			if out.Status != contracts.ResultStatusCancelled &&
				out.Status != contracts.ResultStatusTimedOut &&
				out.Status != contracts.ResultStatusSuccessAndReboot {
				out.MarkAsFailed(log, fmt.Errorf("failed to run commands: %v", err))
				out.Status = contracts.ResultStatusFailed
			}
		}
	}

	// read all standard output/error
	if bytesOut, err := ioutil.ReadAll(stdout); err != nil {
		log.Error(err)
	} else {
		out.AppendInfo(log, string(bytesOut))
	}
	if bytesErr, err := ioutil.ReadAll(stderr); err != nil {
		log.Error(err)
	} else {
		out.AppendError(log, string(bytesErr))
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

	// Return Json indented response
	responseContent, _ := jsonutil.Marshal(out)
	log.Debug("Returning response:\n", jsonutil.Indent(responseContent))
	return
}
