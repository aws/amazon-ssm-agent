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

// Package standardstream implements session standard stream plugin.
package standardstream

import (
	"github.com/aws/amazon-ssm-agent/agent/appconfig"
	"github.com/aws/amazon-ssm-agent/agent/context"
	agentContracts "github.com/aws/amazon-ssm-agent/agent/contracts"
	"github.com/aws/amazon-ssm-agent/agent/framework/processor/executer/iohandler"
	"github.com/aws/amazon-ssm-agent/agent/log"
	mgsContracts "github.com/aws/amazon-ssm-agent/agent/session/contracts"
	"github.com/aws/amazon-ssm-agent/agent/session/datachannel"
	"github.com/aws/amazon-ssm-agent/agent/session/plugins/sessionplugin"
	"github.com/aws/amazon-ssm-agent/agent/session/shell"
	"github.com/aws/amazon-ssm-agent/agent/task"
)

// StandardStreamPlugin is the type for the plugin.
type StandardStreamPlugin struct {
	shell shell.IShellPlugin
}

// Returns parameters required for CLI/console to start session
func (p *StandardStreamPlugin) GetPluginParameters(parameters interface{}) interface{} {
	return nil
}

// StandardStream plugin doesn't require handshake to establish session
func (p *StandardStreamPlugin) RequireHandshake() bool {
	return false
}

// NewPlugin returns a new instance of the Standard Stream Plugin
func NewPlugin() (sessionplugin.ISessionPlugin, error) {
	shellPlugin, err := shell.NewPlugin(appconfig.PluginNameStandardStream)
	if err != nil {
		return nil, err
	}

	var plugin = StandardStreamPlugin{
		shell: shellPlugin,
	}

	return &plugin, nil
}

// name returns the name of Standard Stream Plugin
func (p *StandardStreamPlugin) name() string {
	return appconfig.PluginNameStandardStream
}

// Execute starts pseudo terminal.
// It reads incoming message from data channel and writes to pty.stdin.
// It reads message from pty.stdout and writes to data channel
func (p *StandardStreamPlugin) Execute(context context.T,
	config agentContracts.Configuration,
	cancelFlag task.CancelFlag,
	output iohandler.IOHandler,
	dataChannel datachannel.IDataChannel) {

	p.shell.Execute(context, config, cancelFlag, output, dataChannel, mgsContracts.ShellProperties{})
}

// InputStreamMessageHandler passes payload byte stream to shell stdin
func (p *StandardStreamPlugin) InputStreamMessageHandler(log log.T, streamDataMessage mgsContracts.AgentMessage) error {
	return p.shell.InputStreamMessageHandler(log, streamDataMessage)
}
