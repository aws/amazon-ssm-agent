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
	"os"
	"path/filepath"

	"github.com/aws/amazon-ssm-agent/agent/appconfig"
	"github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/aws/amazon-ssm-agent/agent/contracts"
	"github.com/aws/amazon-ssm-agent/agent/executers"
	"github.com/aws/amazon-ssm-agent/agent/fileutil"
	"github.com/aws/amazon-ssm-agent/agent/framework/processor/executer/iohandler"
	"github.com/aws/amazon-ssm-agent/agent/jsonutil"
	"github.com/aws/amazon-ssm-agent/agent/plugins/pluginutil"
	"github.com/aws/amazon-ssm-agent/agent/task"
)

// SourceHashType is set as default sha256.
const Sha256SourceHashType = "sha256"

// PowerShellModulesDirectory is the directory where PowerShell Modules are installed
var PowerShellModulesDirectory = filepath.Join(os.Getenv("SystemRoot"), "System32", "WindowsPowerShell", "v1.0", "Modules")

// Plugin is the type for the psmodule plugin.
type Plugin struct {
	context context.T
	// ExecuteCommand is an object that can execute commands.
	CommandExecuter executers.T
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
func NewPlugin(context context.T) (*Plugin, error) {
	return &Plugin{
		context:         context,
		CommandExecuter: executers.ShellCommandExecuter{},
	}, nil
}

// Name returns the name of the plugin
func Name() string {
	return appconfig.PluginNameAwsPowerShellModule
}

func (p *Plugin) Execute(config contracts.Configuration, cancelFlag task.CancelFlag, output iohandler.IOHandler) {
	log := p.context.Log()
	log.Infof("%v started with configuration %v", Name(), config)

	if cancelFlag.ShutDown() {
		output.MarkAsShutdown()
	} else if cancelFlag.Canceled() {
		output.MarkAsCancelled()
	} else {
		p.runCommandsRawInput(config.PluginID, config.Properties, config.OrchestrationDirectory, cancelFlag, output)
	}
	return
}

// runCommandsRawInput executes one set of commands and returns their output.
// The input is in the default json unmarshal format (e.g. map[string]interface{}).
func (p *Plugin) runCommandsRawInput(pluginID string, rawPluginInput interface{}, orchestrationDirectory string, cancelFlag task.CancelFlag, output iohandler.IOHandler) {
	var pluginInput PSModulePluginInput
	err := jsonutil.Remarshal(rawPluginInput, &pluginInput)
	p.context.Log().Debugf("Plugin input %v", pluginInput)
	if err != nil {
		errorString := fmt.Errorf("Invalid format in plugin properties %v;\nerror %v", rawPluginInput, err)
		output.MarkAsFailed(errorString)
		return
	}

	pluginInput.ParsedCommands = pluginutil.ParseRunCommand(pluginInput.RunCommand, pluginInput.ParsedCommands)
	p.runCommands(pluginID, pluginInput, orchestrationDirectory, cancelFlag, output)
}

// runCommands executes one set of commands and returns their output.
func (p *Plugin) runCommands(pluginID string, pluginInput PSModulePluginInput, orchestrationDirectory string, cancelFlag task.CancelFlag, output iohandler.IOHandler) {
	var err error
	log := p.context.Log()

	// TODO:MF: This subdirectory is only needed because we could be running multiple sets of properties for the same plugin - otherwise the orchestration directory would already be unique
	orchestrationDir := fileutil.BuildPath(orchestrationDirectory, pluginInput.ID)
	log.Debugf("Running commands %v in workingDirectory %v; orchestrationDir %v ", pluginInput.ParsedCommands, pluginInput.WorkingDirectory, orchestrationDir)

	// create orchestration dir if needed
	if err = fileutil.MakeDirsWithExecuteAccess(orchestrationDir); err != nil {
		log.Debug("failed to create orchestrationDir directory", orchestrationDir)
		output.MarkAsFailed(err)
		return
	}

	// Create script file path
	scriptPath := filepath.Join(orchestrationDir, appconfig.RunCommandScriptName)
	log.Debugf("Writing commands %v to file %v", pluginInput, scriptPath)

	// Create script file
	if err = pluginutil.CreateScriptFile(log, scriptPath, pluginInput.ParsedCommands, fileutil.ByteOrderMarkSkip); err != nil {
		output.MarkAsFailed(fmt.Errorf("failed to create script file. %v", err))
		return
	}

	if pluginInput.Source != "" {
		//change hash type to be default sha256
		pluginInput.SourceHashType = Sha256SourceHashType
		// Download file from source if available
		downloadOutput, err := pluginutil.DownloadFileFromSource(p.context, pluginInput.Source, pluginInput.SourceHash, pluginInput.SourceHashType)
		if err != nil || downloadOutput.IsHashMatched == false || downloadOutput.LocalFilePath == "" {
			output.MarkAsFailed(fmt.Errorf("failed to download file reliably %v", pluginInput.Source))
			// Only delete the file if source is not a local path
			if exists, err := fileutil.LocalFileExist(pluginInput.Source); err == nil && !exists {
				// delete downloaded file, if it exists
				pluginutil.CleanupFile(log, downloadOutput.LocalFilePath)
			}
			return
		} else {
			// Uncompress the zip file received
			if err = fileutil.Uncompress(log, downloadOutput.LocalFilePath, PowerShellModulesDirectory); err != nil {
				output.MarkAsFailed(fmt.Errorf("Failed to uncompress %v to %v: %v", downloadOutput.LocalFilePath, PowerShellModulesDirectory, err.Error()))
				return
			}
			// Only delete the file if source is not a local path
			if exists, err := fileutil.LocalFileExist(pluginInput.Source); err == nil && !exists {
				// delete downloaded file, if it exists
				pluginutil.CleanupFile(log, downloadOutput.LocalFilePath)
			}
		}
	}

	// Set execution time
	executionTimeout := pluginutil.ValidateExecutionTimeout(log, pluginInput.TimeoutSeconds)

	// Construct Command Name and Arguments
	commandName := pluginutil.GetShellCommand()
	commandArguments := append(pluginutil.GetShellArguments(), scriptPath)

	// Execute Command
	exitCode, err := p.CommandExecuter.NewExecute(p.context, pluginInput.WorkingDirectory, output.GetStdoutWriter(), output.GetStderrWriter(), cancelFlag, executionTimeout, commandName, commandArguments, make(map[string]string))

	// Set output status
	output.SetExitCode(exitCode)
	output.SetStatus(pluginutil.GetStatus(exitCode, cancelFlag))

	if err != nil {
		status := output.GetStatus()
		if status != contracts.ResultStatusCancelled &&
			status != contracts.ResultStatusTimedOut &&
			status != contracts.ResultStatusSuccessAndReboot {
			output.MarkAsFailed(fmt.Errorf("failed to run commands: %v", err))
		}
	}
}
