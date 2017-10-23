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
// RunPowerShellScript contains implementation of the plugin that runs powershell scripts on linux or windows
package runscript

import (
	"strings"

	"github.com/aws/amazon-ssm-agent/agent/appconfig"
	"github.com/aws/amazon-ssm-agent/agent/executers"
	"github.com/aws/amazon-ssm-agent/agent/fileutil"
)

// powerShellScriptName is the script name where all downloaded or provided commands will be stored
var powerShellScriptName = "_script.ps1"

// PSPlugin is the type for the RunPowerShellScript plugin and embeds Plugin struct.
type runPowerShellPlugin struct {
	Plugin
}

// NewRunPowerShellPlugin returns a new instance of the PSPlugin.
func NewRunPowerShellPlugin() (*runPowerShellPlugin, error) {
	psplugin := runPowerShellPlugin{
		Plugin{
			Name:            appconfig.PluginNameAwsRunPowerShellScript,
			ScriptName:      powerShellScriptName,
			ShellCommand:    appconfig.PowerShellPluginCommandName,
			ShellArguments:  strings.Split(appconfig.PowerShellPluginCommandArgs, " "),
			ByteOrderMark:   fileutil.ByteOrderMarkEmit,
			CommandExecuter: executers.ShellCommandExecuter{},
		},
	}

	return &psplugin, nil
}
