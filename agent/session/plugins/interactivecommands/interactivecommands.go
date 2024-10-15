// Copyright 2019 Amazon.com, Inc. or its affiliates. All Rights Reserved.
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

// Package interactivecommands implements session shell plugin with interactive commands.
package interactivecommands

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

// InteractiveCommandsPlugin is the type for the sessionPlugin.
type InteractiveCommandsPlugin struct {
	context       context.T
	sessionPlugin sessionplugin.ISessionPlugin
}

// Returns parameters required for CLI/console to start session
func (p *InteractiveCommandsPlugin) GetPluginParameters(parameters interface{}) interface{} {
	return p.sessionPlugin.GetPluginParameters(parameters)
}

// InteractiveCommands plugin doesn't require handshake to establish session
func (p *InteractiveCommandsPlugin) RequireHandshake() bool {
	return p.sessionPlugin.RequireHandshake()
}

// NewPlugin returns a new instance of the InteractiveCommands Plugin
func NewPlugin(context context.T) (sessionplugin.ISessionPlugin, error) {
	singleCommandPlugin, err := singlecommand.NewPlugin(context, appconfig.PluginNameInteractiveCommands)
	if err != nil {
		return nil, err
	}

	var plugin = InteractiveCommandsPlugin{
		context:       context,
		sessionPlugin: singleCommandPlugin,
	}
	return &plugin, nil
}

// name returns the name of interactive commands Plugin
func (p *InteractiveCommandsPlugin) name() string {
	return appconfig.PluginNameInteractiveCommands
}

// Execute executes command as passed in from document parameter via pty.stdin.
// It reads message from cmd.stdout and writes to data channel.
func (p *InteractiveCommandsPlugin) Execute(config agentContracts.Configuration,
	cancelFlag task.CancelFlag,
	output iohandler.IOHandler,
	dataChannel datachannel.IDataChannel) {

	p.sessionPlugin.Execute(config, cancelFlag, output, dataChannel)
}

// InputStreamMessageHandler passes payload byte stream to shell stdin
func (p *InteractiveCommandsPlugin) InputStreamMessageHandler(log log.T, streamDataMessage mgsContracts.AgentMessage) error {
	return p.sessionPlugin.InputStreamMessageHandler(log, streamDataMessage)
}
