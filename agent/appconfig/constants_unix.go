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

// +build darwin freebsd linux netbsd openbsd

// Package appconfig manages the configuration of the agent.
package appconfig

const (
	// Program Folder
	DefaultProgramFolder = "/etc/amazon/ssm/"

	// AppConfig Path
	AppConfigPath = DefaultProgramFolder + AppConfigFileName

	// DownloadRoot specifies the directory under which files will be downloaded
	DownloadRoot = "/var/log/amazon/ssm/download/"

	// DefaultDataStorePath represents the directory for storing system data
	DefaultDataStorePath = "/var/lib/amazon/ssm/"

	// UpdaterArtifactsRoot represents the directory for storing update related information
	UpdaterArtifactsRoot = "/var/lib/amazon/ssm/update/"

	// List all plugin names, unfortunately golang doesn't support const arrays of strings

	// PluginNameAwsRunScript is the name for run script plugin
	PluginNameAwsRunScript = "aws:runShellScript"

	// RebootExitCode that would trigger a Soft Reboot
	RebootExitCode = 194
)
