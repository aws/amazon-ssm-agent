// Copyright 2016 Amazon.com, Inc. or its affiliates. All Rights Reserved.
//
// Licensed under the Amazon Software License (the "License"). You may not
// use this file except in compliance with the License. A copy of the
// License is located at
//
// http://aws.amazon.com/asl/
//
// or in the "license" file accompanying this file. This file is distributed
// on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND,
// express or implied. See the License for the specific language governing
// permissions and limitations under the License.

// +build windows

// Package appconfig manages the configuration of the agent.

package appconfig

const (
	// AppConfig Path
	AppConfigPath = "amazon-ssm-agent.json"

	// DownloadRoot specifies the directory under which files will be downloaded
	DownloadRoot = "download"

	// DefaultDataStorePath represents the directory for storing system data
	DefaultDataStorePath = "C:\\temp"

	// UpdaterArtifactsRoot represents the directory for storing update related information
	UpdaterArtifactsRoot = "C:\\temp\\ssm\\update"

	// List all plugin names, unfortunately golang doesn't support const arrays of strings

	// PluginNameAwsRunScript is the name of the run script plugin
	PluginNameAwsRunScript = "aws:runPowerShellScript"

	// PluginNameAwsRunScriptAlias1 specifies another alias that we want to provide.
	// Remove this if we dont want to support legacy documents.
	PluginNameAwsRunScriptAlias1 = "aws:runScript"

	// PluginNameAwsAgentUpdate is the name for agent update plugin
	PluginNameAwsAgentUpdate = "aws:updateSsmAgent"

	// Exit Code that would trigger a Soft Reboot
	RebootExitCode = 3010
)
