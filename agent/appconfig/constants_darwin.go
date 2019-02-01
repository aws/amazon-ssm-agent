// Copyright 2018 Amazon.com, Inc. or its affiliates. All Rights Reserved.
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

// +build darwin

// Package appconfig manages the configuration of the agent.
package appconfig

const (
	// DefaultProgramFolder is the default folder for SSM
	DefaultProgramFolder = "/opt/aws/ssm/"

	// AppConfigPath is the path of the AppConfig
	AppConfigPath = DefaultProgramFolder + AppConfigFileName

	// PackageRoot specifies the directory under which packages will be downloaded and installed
	PackageRoot = DefaultProgramFolder + "packages"

	// PackageLockRoot specifies the directory under which package lock files will reside
	PackageLockRoot = DefaultProgramFolder + "locks/packages"

	// PackagePlatform is the platform name to use when looking for packages
	PackagePlatform = "darwin"

	// DaemonRoot specifies the directory where daemon registration information is stored
	DaemonRoot = DefaultProgramFolder + "daemons"

	// LocalCommandRoot specifies the directory where users can submit command documents offline
	LocalCommandRoot = DefaultProgramFolder + "localcommands"

	// LocalCommandRootSubmitted is the directory where locally submitted command documents
	// are moved when they have been picked up
	LocalCommandRootSubmitted = DefaultProgramFolder + "localcommands/submitted"
	LocalCommandRootCompleted = DefaultProgramFolder + "localcommands/completed"

	// LocalCommandRootInvalid is the directory where locally submitted command documents
	// are moved if the service cannot validate the document (generally impossible via cli)
	LocalCommandRootInvalid = DefaultProgramFolder + "localcommands/invalid"

	// DownloadRoot specifies the directory under which files will be downloaded
	DownloadRoot = DefaultProgramFolder + "download/"

	// DefaultDataStorePath represents the directory for storing system data
	DefaultDataStorePath = DefaultProgramFolder + "data/"

	// EC2ConfigDataStorePath represents the directory for storing ec2 config data
	EC2ConfigDataStorePath = "/var/lib/amazon/ec2config/"

	// EC2ConfigSettingPath represents the directory for storing ec2 config settings
	EC2ConfigSettingPath = "/var/lib/amazon/ec2configservice/"

	// UpdaterArtifactsRoot represents the directory for storing update related information
	UpdaterArtifactsRoot = DefaultProgramFolder + "update/"

	// DefaultPluginPath represents the directory for storing plugins in SSM
	DefaultPluginPath = DefaultProgramFolder + "plugins"

	// ManifestCacheDirectory represents the directory for storing all downloaded manifest files
	ManifestCacheDirectory = DefaultProgramFolder + "manifests"

	// List all plugin names, unfortunately golang doesn't support const arrays of strings

	// RebootExitCode that would trigger a Soft Reboot
	RebootExitCode = 194

	// Default Custom Inventory Inventory Folder
	DefaultCustomInventoryFolder = DefaultDataStorePath + "inventory/custom"

	DefaultDocumentWorker = DefaultProgramFolder + "bin/ssm-document-worker"
	DefaultSessionWorker  = DefaultProgramFolder + "bin/ssm-session-worker"
	DefaultSessionLogger  = DefaultProgramFolder + "bin/ssm-session-logger"

	// PowerShellPluginCommandName is the path of the powershell.exe to be used by the runPowerShellScript plugin
	PowerShellPluginCommandName = "/usr/bin/powershell"

	// PowerShellPluginCommandArgs is the arguments of powershell.exe to be used by the runPowerShellScript plugin
	PowerShellPluginCommandArgs = ""

	// Exit Code for a command that exits before completion (generally due to timeout or cancel)
	CommandStoppedPreemptivelyExitCode = 137 // Fatal error (128) + signal for SIGKILL (9) = 137

	// RunCommandScriptName is the script name where all downloaded or provided commands will be stored
	RunCommandScriptName = "_script.sh"
)
