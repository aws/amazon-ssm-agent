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

// Package noninteractivecommands implements session shell sessionPlugin with non-interactive command execution.
package noninteractivecommands

import (
	"github.com/aws/amazon-ssm-agent/agent/appconfig"
	"github.com/aws/amazon-ssm-agent/agent/context"
	agentContracts "github.com/aws/amazon-ssm-agent/agent/contracts"
	"github.com/aws/amazon-ssm-agent/agent/framework/processor/executer/iohandler"
	"github.com/aws/amazon-ssm-agent/agent/log"
	mgsContracts "github.com/aws/amazon-ssm-agent/agent/session/contracts"
	"github.com/aws/amazon-ssm-agent/agent/session/datachannel"
	"github.com/aws/amazon-ssm-agent/agent/session/plugins/sessionplugin"
	"github.com/aws/amazon-ssm-agent/agent/session/plugins/singlecommand"
	"github.com/aws/amazon-ssm-agent/agent/task"
)

// NonInteractiveCommandsPlugin is the type for the sessionPlugin.
type NonInteractiveCommandsPlugin struct {
	context       context.T
	sessionPlugin sessionplugin.ISessionPlugin
}

// Returns parameters required for CLI/console to start session
func (p *NonInteractiveCommandsPlugin) GetPluginParameters(parameters interface{}) interface{} {
	return p.sessionPlugin.GetPluginParameters(parameters)
}

// Override as NonInteractiveCommandsPlugin plugin requires handshake to establish session
func (p *NonInteractiveCommandsPlugin) RequireHandshake() bool {
	return true
}

// NewPlugin returns a new instance of the InteractiveCommands Plugin
func NewPlugin(context context.T) (sessionplugin.ISessionPlugin, error) {
	singleCommandPlugin, err := singlecommand.NewPlugin(context, appconfig.PluginNameNonInteractiveCommands)
	if err != nil {
		return nil, err
	}

	var plugin = NonInteractiveCommandsPlugin{
		context:       context,
		sessionPlugin: singleCommandPlugin,
	}
	return &plugin, nil
}

// name returns the name of non-interactive commands Plugin
func (p *NonInteractiveCommandsPlugin) name() string {
	return appconfig.PluginNameNonInteractiveCommands
}

// Execute executes command as passed in from document parameter via cmd.Exec.
// It reads message from cmd.stdout and writes to data channel.
func (p *NonInteractiveCommandsPlugin) Execute(config agentContracts.Configuration,
	cancelFlag task.CancelFlag,
	output iohandler.IOHandler,
	dataChannel datachannel.IDataChannel) {

	p.sessionPlugin.Execute(config, cancelFlag, output, dataChannel)
}

// InputStreamMessageHandler passes payload byte stream to command execution process
func (p *NonInteractiveCommandsPlugin) InputStreamMessageHandler(log log.T, streamDataMessage mgsContracts.AgentMessage) error {
	return p.sessionPlugin.InputStreamMessageHandler(log, streamDataMessage)
}
