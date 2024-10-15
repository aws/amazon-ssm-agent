// Copyright 2020 Amazon.com, Inc. or its affiliates. All Rights Reserved.
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

// Package singlecommand implements session shell plugin with interactive or non-interactive single command.
package singlecommand

import (
	"fmt"

	"github.com/aws/amazon-ssm-agent/agent/appconfig"
	"github.com/aws/amazon-ssm-agent/agent/context"
	agentContracts "github.com/aws/amazon-ssm-agent/agent/contracts"
	"github.com/aws/amazon-ssm-agent/agent/framework/processor/executer/iohandler"
	"github.com/aws/amazon-ssm-agent/agent/jsonutil"
	"github.com/aws/amazon-ssm-agent/agent/log"
	mgsContracts "github.com/aws/amazon-ssm-agent/agent/session/contracts"
	"github.com/aws/amazon-ssm-agent/agent/session/datachannel"
	"github.com/aws/amazon-ssm-agent/agent/session/plugins/sessionplugin"
	"github.com/aws/amazon-ssm-agent/agent/session/shell"
	"github.com/aws/amazon-ssm-agent/agent/task"
)

// SingleCommand is the generic plugin structure for InteractiveCommands and NonInteractiveCommands plugins.
type SingleCommand struct {
	context    context.T
	shell      shell.IShellPlugin
	pluginName string
}

// Returns parameters required for CLI/console to start session
func (p *SingleCommand) GetPluginParameters(parameters interface{}) interface{} {
	return nil
}

// SingleCommand by default does not require handshake to establish session
// TODO: change to default to require handshake once InteractiveCommands plugin enforces handshake.
func (p *SingleCommand) RequireHandshake() bool {
	return false
}

// NewPlugin returns a new instance of the InteractiveCommands or NonInteractiveCommands plugin.
func NewPlugin(context context.T, name string) (sessionplugin.ISessionPlugin, error) {
	shellPlugin, err := shell.NewPlugin(context, name)
	if err != nil {
		return nil, err
	}

	var plugin = SingleCommand{
		context:    context,
		shell:      shellPlugin,
		pluginName: name,
	}
	return &plugin, nil
}

// name returns the name of plugin, which can be either InteractiveCommands or NonInteractiveCommands
func (p *SingleCommand) name() string {
	return p.pluginName
}

// Execute executes a command as passed in from document parameter, and writes command output to data channel.
// This function is shared between InteractiveCommands and NonInteractiveCommands plugins.
func (p *SingleCommand) Execute(config agentContracts.Configuration,
	cancelFlag task.CancelFlag,
	output iohandler.IOHandler,
	dataChannel datachannel.IDataChannel) {

	logger := p.context.Log()
	var shellProps mgsContracts.ShellProperties
	err := jsonutil.Remarshal(config.Properties, &shellProps)
	logger.Debugf("Plugin properties %v", shellProps)
	if err != nil {
		sessionPluginResultOutput := mgsContracts.SessionPluginResultOutput{}
		output.SetExitCode(appconfig.ErrorExitCode)
		output.SetStatus(agentContracts.ResultStatusFailed)
		sessionPluginResultOutput.Output = fmt.Sprintf("Invalid format in session properties %v;\nerror %v", config.Properties, err)
		output.SetOutput(sessionPluginResultOutput)
		logger.Error(sessionPluginResultOutput.Output)
		return
	}

	if err := p.validateProperties(shellProps); err != nil {
		sessionPluginResultOutput := mgsContracts.SessionPluginResultOutput{}
		output.SetExitCode(appconfig.ErrorExitCode)
		output.SetStatus(agentContracts.ResultStatusFailed)
		sessionPluginResultOutput.Output = err.Error()
		output.SetOutput(sessionPluginResultOutput)
		logger.Error(sessionPluginResultOutput.Output)
		return
	}

	// streaming of logs is not supported for single commands scenario, set it to false
	config.CloudWatchStreamingEnabled = false

	p.shell.Execute(config, cancelFlag, output, dataChannel, shellProps)
}

// InputStreamMessageHandler passes payload byte stream to command execution process.
func (p *SingleCommand) InputStreamMessageHandler(log log.T, streamDataMessage mgsContracts.AgentMessage) error {
	return p.shell.InputStreamMessageHandler(log, streamDataMessage)
}
