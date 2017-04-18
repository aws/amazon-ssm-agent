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

// Package runscript implements the runscript plugin.
package runscript

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

// Plugin is the type for the runscript plugin.
type Plugin struct {
	pluginutil.DefaultPlugin
	defaultWorkingDirectory string

	// Name is the plugin name (PowerShellScript or ShellScript)
	Name           string
	ScriptName     string
	ShellCommand   string
	ShellArguments []string
	ByteOrderMark  fileutil.ByteOrderMark
}

// RunScriptPluginInput represents one set of commands executed by the RunScript plugin.
type RunScriptPluginInput struct {
	contracts.PluginInput
	RunCommand       []string
	ID               string
	WorkingDirectory string
	TimeoutSeconds   interface{}
}

func (p *Plugin) AssignPluginConfigs(pluginConfig pluginutil.PluginConfig) {
	p.MaxStdoutLength = pluginConfig.MaxStdoutLength
	p.MaxStderrLength = pluginConfig.MaxStderrLength
	p.StdoutFileName = pluginConfig.StdoutFileName
	p.StderrFileName = pluginConfig.StderrFileName
	p.OutputTruncatedSuffix = pluginConfig.OutputTruncatedSuffix
	p.ExecuteUploadOutputToS3Bucket = pluginutil.UploadOutputToS3BucketExecuter(p.UploadOutputToS3Bucket)
	p.CommandExecuter = executers.ShellCommandExecuter{}
}

// Execute runs multiple sets of commands and returns their outputs.
// res.Output will contain a slice of RunScriptPluginOutput.
func (p *Plugin) Execute(context context.T, config contracts.Configuration, cancelFlag task.CancelFlag, subDocumentRunner runpluginutil.PluginRunner) (res contracts.PluginResult) {
	log := context.Log()
	log.Infof("%v started with configuration %v", p.Name, config)
	res.StartDateTime = time.Now()
	defer func() { res.EndDateTime = time.Now() }()
	log.Debugf("DefaultWorkingDirectory %v", config.DefaultWorkingDirectory)
	p.defaultWorkingDirectory = config.DefaultWorkingDirectory

	//loading Properties as list since aws:runPowershellScript & aws:runShellScript uses properties as list
	var properties []interface{}
	if properties = pluginutil.LoadParametersAsList(log, config.Properties, &res); res.Code != 0 {

		pluginutil.PersistPluginInformationToCurrent(log, config.PluginID, config, res)
		return res
	}

	out := make([]contracts.PluginOutput, len(properties))
	for i, prop := range properties {

		if cancelFlag.ShutDown() {
			out[i].ExitCode = 1
			out[i].Status = contracts.ResultStatusFailed
			break
		}

		if cancelFlag.Canceled() {
			out[i].ExitCode = 1
			out[i].Status = contracts.ResultStatusCancelled
			break
		}

		out[i] = p.runCommandsRawInput(log, config.PluginID, prop, config.OrchestrationDirectory, cancelFlag, config.OutputS3BucketName, config.OutputS3KeyPrefix)
	}

	// TODO: instance here we have to do more result processing, where individual sub properties results are merged smartly into plugin response.
	// Currently assuming we have only one work.
	if len(properties) > 0 {
		res.Code = out[0].ExitCode
		res.Status = out[0].Status
		res.Output = out[0].String()
	}

	pluginutil.PersistPluginInformationToCurrent(log, config.PluginID, config, res)

	return res
}

// runCommandsRawInput executes one set of commands and returns their output.
// The input is in the default json unmarshal format (e.g. map[string]interface{}).
func (p *Plugin) runCommandsRawInput(log log.T, pluginID string, rawPluginInput interface{}, orchestrationDirectory string, cancelFlag task.CancelFlag, outputS3BucketName string, outputS3KeyPrefix string) (out contracts.PluginOutput) {
	var pluginInput RunScriptPluginInput
	err := jsonutil.Remarshal(rawPluginInput, &pluginInput)
	if err != nil {
		errorString := fmt.Errorf("Invalid format in plugin properties %v;\nerror %v", rawPluginInput, err)
		out.MarkAsFailed(log, errorString)
		return
	}
	return p.runCommands(log, pluginID, pluginInput, orchestrationDirectory, cancelFlag, outputS3BucketName, outputS3KeyPrefix)
}

// runCommands executes one set of commands and returns their output.
func (p *Plugin) runCommands(log log.T, pluginID string, pluginInput RunScriptPluginInput, orchestrationDirectory string, cancelFlag task.CancelFlag, outputS3BucketName string, outputS3KeyPrefix string) (out contracts.PluginOutput) {
	var err error

	workingDir := pluginInput.WorkingDirectory
	if workingDir == "" {
		workingDir = p.defaultWorkingDirectory
	}

	// TODO:MF: This subdirectory is only needed because we could be running multiple sets of properties for the same plugin - otherwise the orchestration directory would already be unique
	orchestrationDir := fileutil.BuildPath(orchestrationDirectory, pluginInput.ID)
	log.Debugf("Running commands %v in workingDirectory %v; orchestrationDir %v ", pluginInput.RunCommand, workingDir, orchestrationDir)

	// create orchestration dir if needed
	if err = fileutil.MakeDirsWithExecuteAccess(orchestrationDir); err != nil {
		out.MarkAsFailed(log, fmt.Errorf("failed to create orchestrationDir directory, %v", orchestrationDir))
		return
	}

	// Create script file path
	scriptPath := filepath.Join(orchestrationDir, p.ScriptName)
	log.Debugf("Writing commands %v to file %v", pluginInput, scriptPath)

	// Create script file
	if err = pluginutil.CreateScriptFile(log, scriptPath, pluginInput.RunCommand, p.ByteOrderMark); err != nil {
		out.MarkAsFailed(log, fmt.Errorf("failed to create script file. %v", err))
		return
	}

	// Set execution time
	executionTimeout := pluginutil.ValidateExecutionTimeout(log, pluginInput.TimeoutSeconds)

	// Create output file paths
	stdoutFilePath := filepath.Join(orchestrationDir, p.StdoutFileName)
	stderrFilePath := filepath.Join(orchestrationDir, p.StderrFileName)
	log.Debugf("stdout file %v, stderr file %v", stdoutFilePath, stderrFilePath)

	// Construct Command Name and Arguments
	commandName := p.ShellCommand
	commandArguments := append(p.ShellArguments, scriptPath, appconfig.ExitCodeTrap)

	// Execute Command
	stdout, stderr, exitCode, errs := p.CommandExecuter.Execute(log, workingDir, stdoutFilePath, stderrFilePath, cancelFlag, executionTimeout, commandName, commandArguments)

	// Set output status
	out.ExitCode = exitCode
	out.Status = pluginutil.GetStatus(out.ExitCode, cancelFlag)

	if len(errs) > 0 {
		for _, err := range errs {
			if out.Status != contracts.ResultStatusCancelled &&
				out.Status != contracts.ResultStatusTimedOut &&
				out.Status != contracts.ResultStatusSuccessAndReboot {
				out.MarkAsFailed(log, fmt.Errorf("failed to run commands: %v", err))
			}
		}
	}
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
