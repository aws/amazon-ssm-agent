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
	"github.com/aws/amazon-ssm-agent/agent/plugins/pluginutil"
	messageContracts "github.com/aws/amazon-ssm-agent/agent/runcommand/contracts"
	"github.com/aws/amazon-ssm-agent/agent/task"
	"github.com/aws/amazon-ssm-agent/common/identity"
	"github.com/aws/amazon-ssm-agent/common/runtimeconfig"
)

const (
	downloadsDir = "downloads" //Directory under the orchestration directory where the downloaded resource resides
)

var getRemoteProvider = identity.GetRemoteProvider

// Plugin is the type for the runscript plugin.
type Plugin struct {
	Context context.T
	// ExecuteCommand is an object that can execute commands.
	CommandExecuter executers.T
	// Name is the plugin name (PowerShellScript or ShellScript)
	Name                  string
	ScriptName            string
	ShellCommand          string
	ShellArguments        []string
	ByteOrderMark         fileutil.ByteOrderMark
	IdentityRuntimeClient runtimeconfig.IIdentityRuntimeConfigClient
}

// RunScriptPluginInput represents one set of commands executed by the RunScript plugin.
type RunScriptPluginInput struct {
	contracts.PluginInput
	RunCommand       []string
	Environment      map[string]string
	ID               string
	WorkingDirectory string
	TimeoutSeconds   interface{}
}

// Execute runs multiple sets of commands and returns their outputs.
// res.Output will contain a slice of RunScriptPluginOutput.
func (p *Plugin) Execute(config contracts.Configuration, cancelFlag task.CancelFlag, output iohandler.IOHandler) {
	log := p.Context.Log()
	log.Infof("%v started with configuration %v", p.Name, config)
	log.Debugf("DefaultWorkingDirectory %v", config.DefaultWorkingDirectory)

	runCommandID, err := messageContracts.GetCommandID(config.MessageId)

	if err != nil {
		log.Warnf("Error extracting RunCommandID from config %v: Error: %v", config, err)
		runCommandID = ""
	}

	if cancelFlag.ShutDown() {
		output.MarkAsShutdown()
	} else if cancelFlag.Canceled() {
		output.MarkAsCancelled()
	} else {
		p.runCommandsRawInput(config.PluginID, config.Properties, config.OrchestrationDirectory, config.DefaultWorkingDirectory, cancelFlag, output, runCommandID)
	}
}

func (p *Plugin) setShareCredsEnvironment(pluginInput RunScriptPluginInput) {
	credentialProvider, ok := getRemoteProvider(p.Context.Identity())
	if !ok {
		return
	}

	// Don't set environment variables if credentials are not being shared
	if !credentialProvider.SharesCredentials() {
		return
	}

	// Get identity runtime config
	identityConfig, err := p.IdentityRuntimeClient.GetConfig()
	if err != nil {
		p.Context.Log().Infof("Failed to get identity runtime config, unable to set profile and creds file: %v", err)
		return
	}

	if identityConfig.ShareProfile != "" {
		pluginInput.Environment["AWS_PROFILE"] = identityConfig.ShareProfile
	}

	if identityConfig.ShareFile != "" {
		pluginInput.Environment["AWS_SHARED_CREDENTIALS_FILE"] = identityConfig.ShareFile
	}
}

func (p *Plugin) setCommandIdEnvironment(pluginInput RunScriptPluginInput, runCommandID string) {
	if runCommandID != "" {
		// Check if "SSM_COMMAND_ID" exists already in the env. If so, log that it will be overwritten
		if _, ok := pluginInput.Environment["SSM_COMMAND_ID"]; ok {
			p.Context.Log().Warnf("The environment variable 'SSM_COMMAND_ID' has been detected as pre-existing and will be overwritten with the CommandId of this execution.")
		}
		pluginInput.Environment["SSM_COMMAND_ID"] = runCommandID
	}
}

// runCommandsRawInput executes one set of commands and returns their output.
// The input is in the default json unmarshal format (e.g. map[string]interface{}).
func (p *Plugin) runCommandsRawInput(pluginID string, rawPluginInput interface{}, orchestrationDirectory string, defaultWorkingDirectory string, cancelFlag task.CancelFlag, output iohandler.IOHandler, runCommandID string) {
	var pluginInput RunScriptPluginInput
	err := jsonutil.Remarshal(rawPluginInput, &pluginInput)
	if err != nil {
		errorString := fmt.Errorf("Invalid format in plugin properties %v;\nerror %v", rawPluginInput, err)
		output.MarkAsFailed(errorString)
		return
	}

	if pluginInput.Environment == nil {
		pluginInput.Environment = make(map[string]string)
	}

	p.setCommandIdEnvironment(pluginInput, runCommandID)
	p.setShareCredsEnvironment(pluginInput)

	p.runCommands(pluginID, pluginInput, orchestrationDirectory, defaultWorkingDirectory, cancelFlag, output)
}

// runCommands executes one set of commands and returns their output.
func (p *Plugin) runCommands(pluginID string, pluginInput RunScriptPluginInput, orchestrationDirectory string, defaultWorkingDirectory string, cancelFlag task.CancelFlag, output iohandler.IOHandler) {
	log := p.Context.Log()
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
	log.Debugf("Running commands %v with environment variables %v in workingDirectory %v; orchestrationDir %v ", pluginInput.RunCommand, pluginInput.Environment, workingDir, orchestrationDir)

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
	exitCode, err := p.CommandExecuter.NewExecute(p.Context, workingDir, output.GetStdoutWriter(), output.GetStderrWriter(), cancelFlag, executionTimeout, commandName, commandArguments, pluginInput.Environment)

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
