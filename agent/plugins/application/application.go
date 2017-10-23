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

// Package application implements the application plugin.
//
// +build windows

package application

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
	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/amazon-ssm-agent/agent/plugins/pluginutil"
	"github.com/aws/amazon-ssm-agent/agent/task"
)

const (
	// defaultApplicationExecutionTimeoutInSeconds represents default timeout time for execution of applications in seconds
	defaultApplicationExecutionTimeoutInSeconds = 3600

	// defaultWorkingDirectory represents the default working directory
	defaultWorkingDirectory = ""
)

// msiExecCommand is the command for installing msi applications
var msiExecCommand = filepath.Join(os.Getenv("SystemRoot"), "System32", "msiexec.exe")

// Plugin is the type for the applications plugin.
type Plugin struct {
	// ExecuteCommand is an object that can execute commands.
	CommandExecuter executers.T
}

// ApplicationPluginInput represents one set of commands executed by the Applications plugin.
type ApplicationPluginInput struct {
	contracts.PluginInput
	ID             string
	Action         string
	Parameters     string
	Source         string
	SourceHash     string
	SourceHashType string
}

// NewPlugin returns a new instance of the plugin.
func NewPlugin() (*Plugin, error) {
	var plugin Plugin
	plugin.CommandExecuter = executers.ShellCommandExecuter{}
	return &plugin, nil
}

// Name returns the name of the plugin
func Name() string {
	return appconfig.PluginNameAwsApplications
}

func (p *Plugin) Execute(context context.T, config contracts.Configuration, cancelFlag task.CancelFlag, output iohandler.IOHandler) {
	log := context.Log()
	log.Infof("%v started with configuration %v", Name(), config)
	log.Debugf("DefaultWorkingDirectory %v", config.DefaultWorkingDirectory)

	if cancelFlag.ShutDown() {
		output.MarkAsShutdown()
	} else if cancelFlag.Canceled() {
		output.MarkAsCancelled()
	} else {
		p.runCommandsRawInput(log, config.PluginID, config.Properties, config.OrchestrationDirectory, config.DefaultWorkingDirectory, cancelFlag, output)
	}
	return
}

// runCommandsRawInput executes one set of commands and returns their output.
// The input is in the default json unmarshal format (e.g. map[string]interface{}).
func (p *Plugin) runCommandsRawInput(log log.T, pluginID string, rawPluginInput interface{}, orchestrationDirectory string, defaultWorkingDirectory string, cancelFlag task.CancelFlag, output iohandler.IOHandler) {
	var pluginInput ApplicationPluginInput
	err := jsonutil.Remarshal(rawPluginInput, &pluginInput)
	log.Debugf("Plugin input %v", pluginInput)
	if err != nil {
		errorString := fmt.Errorf("Invalid format in plugin properties %v;\nerror %v", rawPluginInput, err)
		output.MarkAsFailed(errorString)
		return
	}
	p.runCommands(log, pluginID, pluginInput, orchestrationDirectory, defaultWorkingDirectory, cancelFlag, output)
}

// runCommands executes one set of commands and returns their output.
func (p *Plugin) runCommands(log log.T, pluginID string, pluginInput ApplicationPluginInput, orchestrationDirectory string, defaultWorkingDirectory string, cancelFlag task.CancelFlag, output iohandler.IOHandler) {
	var err error

	// TODO:MF: This subdirectory is only needed because we could be running multiple sets of properties for the same plugin - otherwise the orchestration directory would already be unique
	orchestrationDir := fileutil.BuildPath(orchestrationDirectory, pluginInput.ID)
	log.Debugf("OrchestrationDir %v ", orchestrationDir)

	// create orchestration dir if needed
	if err = fileutil.MakeDirs(orchestrationDir); err != nil {
		log.Debug("failed to create orchestrationDir directory", orchestrationDir, err)
		output.MarkAsFailed(err)
		return
	}

	// Get application mode
	mode, err := getMsiApplicationMode(log, pluginInput)
	if err != nil {
		output.MarkAsFailed(err)
		return
	}
	log.Debugf("mode is %v", mode)

	var localFilePath string
	// Download file from source if available
	downloadOutput, err := pluginutil.DownloadFileFromSource(log, pluginInput.Source, pluginInput.SourceHash, pluginInput.SourceHashType)
	if err != nil || downloadOutput.IsHashMatched == false || downloadOutput.LocalFilePath == "" {
		errorString := fmt.Errorf("failed to download file reliably %v", pluginInput.Source)
		output.MarkAsFailed(errorString)
		return
	}
	localFilePath = downloadOutput.LocalFilePath
	log.Debugf("local path to file is %v", localFilePath)

	// Create msi related log file
	localSourceLogFilePath := localFilePath + ".msiexec.log.txt"
	log.Debugf("log path is %v", localSourceLogFilePath)

	// Construct Command Name and Arguments
	commandName := msiExecCommand
	commandArguments := []string{mode, localFilePath, "/quiet", "/norestart", "/log", localSourceLogFilePath}
	if pluginInput.Parameters != "" {
		log.Debugf("Got Parameters \"%v\"", pluginInput.Parameters)
		params := processParams(log, pluginInput.Parameters)
		commandArguments = append(commandArguments, params...)
	}

	// Execute Command
	exitCode, err := p.CommandExecuter.NewExecute(log, defaultWorkingDirectory, output.GetStdoutWriter(), output.GetStderrWriter(), cancelFlag, defaultApplicationExecutionTimeoutInSeconds, commandName, commandArguments)

	// Set output status
	output.SetExitCode(exitCode)
	setMsiExecStatus(log, pluginInput, cancelFlag, output)

	if err != nil {
		output.MarkAsFailed(fmt.Errorf("failed to run commands: %v", err))
		return
	}
}
