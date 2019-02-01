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
	"path/filepath"

	"strings"

	"github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/aws/amazon-ssm-agent/agent/contracts"
	"github.com/aws/amazon-ssm-agent/agent/executers"
	"github.com/aws/amazon-ssm-agent/agent/fileutil"
	"github.com/aws/amazon-ssm-agent/agent/framework/processor/executer/iohandler"
	"github.com/aws/amazon-ssm-agent/agent/jsonutil"
	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/amazon-ssm-agent/agent/plugins/pluginutil"
	"github.com/aws/amazon-ssm-agent/agent/task"
)

const (
	downloadsDir = "downloads" //Directory under the orchestration directory where the downloaded resource resides
)

// Plugin is the type for the runscript plugin.
type Plugin struct {
	// ExecuteCommand is an object that can execute commands.
	CommandExecuter executers.T
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

// Execute runs multiple sets of commands and returns their outputs.
// res.Output will contain a slice of RunScriptPluginOutput.
func (p *Plugin) Execute(context context.T, config contracts.Configuration, cancelFlag task.CancelFlag, output iohandler.IOHandler) {
	log := context.Log()
	log.Infof("%v started with configuration %v", p.Name, config)
	log.Debugf("DefaultWorkingDirectory %v", config.DefaultWorkingDirectory)

	if cancelFlag.ShutDown() {
		output.MarkAsShutdown()
	} else if cancelFlag.Canceled() {
		output.MarkAsCancelled()
	} else {
		p.runCommandsRawInput(log, config.PluginID, config.Properties, config.OrchestrationDirectory, config.DefaultWorkingDirectory, cancelFlag, output)
	}
}

// runCommandsRawInput executes one set of commands and returns their output.
// The input is in the default json unmarshal format (e.g. map[string]interface{}).
func (p *Plugin) runCommandsRawInput(log log.T, pluginID string, rawPluginInput interface{}, orchestrationDirectory string, defaultWorkingDirectory string, cancelFlag task.CancelFlag, output iohandler.IOHandler) {
	var pluginInput RunScriptPluginInput
	err := jsonutil.Remarshal(rawPluginInput, &pluginInput)
	if err != nil {
		errorString := fmt.Errorf("Invalid format in plugin properties %v;\nerror %v", rawPluginInput, err)
		output.MarkAsFailed(errorString)
		return
	}
	p.runCommands(log, pluginID, pluginInput, orchestrationDirectory, defaultWorkingDirectory, cancelFlag, output)
}

// runCommands executes one set of commands and returns their output.
func (p *Plugin) runCommands(log log.T, pluginID string, pluginInput RunScriptPluginInput, orchestrationDirectory string, defaultWorkingDirectory string, cancelFlag task.CancelFlag, output iohandler.IOHandler) {
	var err error
	var workingDir string

	if filepath.IsAbs(pluginInput.WorkingDirectory) {
		workingDir = pluginInput.WorkingDirectory
	} else {
		orchestrationDir := strings.TrimSuffix(orchestrationDirectory, pluginID)
		// The Document path is expected to have the name of the document
		workingDir = filepath.Join(orchestrationDir, downloadsDir, pluginInput.WorkingDirectory)
		if !fileutil.Exists(workingDir) {
			workingDir = defaultWorkingDirectory
		}
	}

	// TODO:MF: This subdirectory is only needed because we could be running multiple sets of properties for the same plugin - otherwise the orchestration directory would already be unique
	orchestrationDir := fileutil.BuildPath(orchestrationDirectory, pluginInput.ID)
	log.Debugf("Running commands %v in workingDirectory %v; orchestrationDir %v ", pluginInput.RunCommand, workingDir, orchestrationDir)

	// create orchestration dir if needed
	if err = fileutil.MakeDirsWithExecuteAccess(orchestrationDir); err != nil {
		output.MarkAsFailed(fmt.Errorf("failed to create orchestrationDir directory, %v", orchestrationDir))
		return
	}

	// Create script file path
	scriptPath := filepath.Join(orchestrationDir, p.ScriptName)
	log.Debugf("Writing commands %v to file %v", pluginInput, scriptPath)

	// Create script file
	if err = pluginutil.CreateScriptFile(log, scriptPath, pluginInput.RunCommand, p.ByteOrderMark); err != nil {
		output.MarkAsFailed(fmt.Errorf("failed to create script file. %v", err))
		return
	}

	// Set execution time
	executionTimeout := pluginutil.ValidateExecutionTimeout(log, pluginInput.TimeoutSeconds)

	// Construct Command Name and Arguments
	commandName := p.ShellCommand
	commandArguments := append(p.ShellArguments, scriptPath)

	// Execute Command
	exitCode, err := p.CommandExecuter.NewExecute(log, workingDir, output.GetStdoutWriter(), output.GetStderrWriter(), cancelFlag, executionTimeout, commandName, commandArguments)

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
