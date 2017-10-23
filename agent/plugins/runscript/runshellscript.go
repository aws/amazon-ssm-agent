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

// Package runscript implements the RunScript plugin.
// RunShellScript contains implementation of the plugin that runs shell scripts on linux
package runscript

import (
	"github.com/aws/amazon-ssm-agent/agent/appconfig"
	"github.com/aws/amazon-ssm-agent/agent/executers"
	"github.com/aws/amazon-ssm-agent/agent/fileutil"
	"github.com/aws/amazon-ssm-agent/agent/log"
)

// runShellPlugin is the type for the RunShellScript plugin and embeds Plugin struct.
type runShellPlugin struct {
	Plugin
}

var shellScriptName = "_script.sh"
var shellCommand = "sh"
var shellArgs = []string{"-c"}

// NewRunShellPlugin returns a new instance of the SHPlugin.
func NewRunShellPlugin(log log.T) (*runShellPlugin, error) {
	shplugin := runShellPlugin{
		Plugin{
			Name:            appconfig.PluginNameAwsRunShellScript,
			ScriptName:      shellScriptName,
			ShellCommand:    shellCommand,
			ShellArguments:  shellArgs,
			ByteOrderMark:   fileutil.ByteOrderMarkSkip,
			CommandExecuter: executers.ShellCommandExecuter{},
		},
	}

	return &shplugin, nil
}
