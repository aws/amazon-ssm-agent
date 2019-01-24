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

// Package restrictedshell implements session shell plugin with restricted commands.
package restrictedshell

import (
	"fmt"
	"strings"

	"github.com/aws/amazon-ssm-agent/agent/appconfig"
	"github.com/aws/amazon-ssm-agent/agent/context"
	agentContracts "github.com/aws/amazon-ssm-agent/agent/contracts"
	"github.com/aws/amazon-ssm-agent/agent/framework/processor/executer/iohandler"
	"github.com/aws/amazon-ssm-agent/agent/log"
	mgsContracts "github.com/aws/amazon-ssm-agent/agent/session/contracts"
	"github.com/aws/amazon-ssm-agent/agent/session/datachannel"
	"github.com/aws/amazon-ssm-agent/agent/session/plugins/sessionplugin"
	"github.com/aws/amazon-ssm-agent/agent/session/plugins/shell"
	"github.com/aws/amazon-ssm-agent/agent/task"
)

// RestrictedShellPlugin is the type for the plugin.
type RestrictedShellPlugin struct {
	shell sessionplugin.ISessionPlugin
}

// NewPlugin returns a new instance of the Restricted Shell Plugin
func NewPlugin() (sessionplugin.ISessionPlugin, error) {
	shellPlugin, err := shell.NewPlugin()
	if err != nil {
		return nil, err
	}

	var plugin = RestrictedShellPlugin{
		shell: shellPlugin,
	}
	return &plugin, nil
}

// name returns the name of Restricted Shell Plugin
func (p *RestrictedShellPlugin) name() string {
	return appconfig.PluginNameRestrictedStandardStream
}

// Execute starts pseudo terminal.
// It reads incoming message from data channel and writes to pty.stdin.
// It reads message from pty.stdout and writes to data channel
func (p *RestrictedShellPlugin) Execute(context context.T,
	config agentContracts.Configuration,
	cancelFlag task.CancelFlag,
	output iohandler.IOHandler,
	dataChannel datachannel.IDataChannel) {

	if strings.TrimSpace(config.Commands) != "" {
		p.shell.Execute(context, config, cancelFlag, output, dataChannel)
	} else {
		logger := context.Log()
		sessionPluginResultOutput := mgsContracts.SessionPluginResultOutput{}
		output.SetExitCode(appconfig.ErrorExitCode)
		output.SetStatus(agentContracts.ResultStatusFailed)
		sessionPluginResultOutput.Output = fmt.Sprintf("Commands cannot be empty for session type %s", p.name())
		output.SetOutput(sessionPluginResultOutput)
		logger.Error(sessionPluginResultOutput.Output)
		return
	}
}

// InputStreamMessageHandler passes payload byte stream to shell stdin
func (p *RestrictedShellPlugin) InputStreamMessageHandler(log log.T, streamDataMessage mgsContracts.AgentMessage) error {
	return p.shell.InputStreamMessageHandler(log, streamDataMessage)
}
