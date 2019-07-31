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

// +build freebsd linux netbsd openbsd

// Package appconfig manages the configuration of the agent.
package appconfig

import (
	"fmt"
	"os"
	"path/filepath"
	"syscall"
)

const (

	// PackageRoot specifies the directory under which packages will be downloaded and installed
	PackageRoot = "/var/lib/amazon/ssm/packages"

	// PackageLockRoot specifies the directory under which package lock files will reside
	PackageLockRoot = "/var/lib/amazon/ssm/locks/packages"

	// PackagePlatform is the platform name to use when looking for packages
	PackagePlatform = "linux"

	// DaemonRoot specifies the directory where daemon registration information is stored
	DaemonRoot = "/var/lib/amazon/ssm/daemons"

	// LocalCommandRoot specifies the directory where users can submit command documents offline
	LocalCommandRoot = "/var/lib/amazon/ssm/localcommands"

	// LocalCommandRootSubmitted is the directory where locally submitted command documents
	// are moved when they have been picked up
	LocalCommandRootSubmitted = "/var/lib/amazon/ssm/localcommands/submitted"
	LocalCommandRootCompleted = "/var/lib/amazon/ssm/localcommands/completed"

	// LocalCommandRootInvalid is the directory where locally submitted command documents
	// are moved if the service cannot validate the document (generally impossible via cli)
	LocalCommandRootInvalid = "/var/lib/amazon/ssm/localcommands/invalid"

	// DownloadRoot specifies the directory under which files will be downloaded
	DownloadRoot = "/var/log/amazon/ssm/download/"

	// DefaultDataStorePath represents the directory for storing system data
	DefaultDataStorePath = "/var/lib/amazon/ssm/"

	// EC2ConfigDataStorePath represents the directory for storing ec2 config data
	EC2ConfigDataStorePath = "/var/lib/amazon/ec2config/"

	// EC2ConfigSettingPath represents the directory for storing ec2 config settings
	EC2ConfigSettingPath = "/var/lib/amazon/ec2configservice/"

	// UpdaterArtifactsRoot represents the directory for storing update related information
	UpdaterArtifactsRoot = "/var/lib/amazon/ssm/update/"

	// DefaultPluginPath represents the directory for storing plugins in SSM
	DefaultPluginPath = "/var/lib/amazon/ssm/plugins"

	// ManifestCacheDirectory represents the directory for storing all downloaded manifest files
	ManifestCacheDirectory = "/var/lib/amazon/ssm/manifests"

	// List all plugin names, unfortunately golang doesn't support const arrays of strings

	// RebootExitCode that would trigger a Soft Reboot
	RebootExitCode = 194

	// Default Custom Inventory Inventory Folder
	DefaultCustomInventoryFolder = DefaultDataStorePath + "inventory/custom"

	// PowerShellPluginCommandArgs is the arguments of powershell.exe to be used by the runPowerShellScript plugin
	PowerShellPluginCommandArgs = "-f"

	// Exit Code for a command that exits before completion (generally due to timeout or cancel)
	CommandStoppedPreemptivelyExitCode = 137 // Fatal error (128) + signal for SIGKILL (9) = 137

	// RunCommandScriptName is the script name where all downloaded or provided commands will be stored
	RunCommandScriptName = "_script.sh"

	NecessaryAgentBinaryPermissionMask  = 0511 // Require read/execute for root, execute for all
	DisallowedAgentBinaryPermissionMask = 0022 // Disallow write for group and user
)

// PowerShellPluginCommandName is the path of the powershell.exe to be used by the runPowerShellScript plugin
var PowerShellPluginCommandName string

// DefaultProgramFolder is the default folder for SSM
var DefaultProgramFolder = "/etc/amazon/ssm/"
var DefaultDocumentWorker = "/usr/bin/ssm-document-worker"
var DefaultSessionWorker = "/usr/bin/ssm-session-worker"
var DefaultSessionLogger = "/usr/bin/ssm-session-logger"

// AppConfigPath is the path of the AppConfig
var AppConfigPath = DefaultProgramFolder + AppConfigFileName

func init() {
	/*
	   Powershell command used to be poweshell in alpha versions, now it's pwsh in prod versions
	*/
	PowerShellPluginCommandName = "/usr/bin/powershell"
	if _, err := os.Stat(PowerShellPluginCommandName); err != nil {
		PowerShellPluginCommandName = "/usr/bin/pwsh"
	}

	// Find current directory path for amazon-ssm-agent, DefaultDocumentWorker should exist in same directory
	// if document-worker is not in the default location, try finding it in the same directory as amazon-ssm-agent
	if _, err := os.Stat(DefaultDocumentWorker); err != nil {
		// curdir is amazon-ssm-agent current directory path
		if curdir, err := filepath.Abs(filepath.Dir(os.Args[0])); err == nil {
			if validateAgentBinary("ssm-document-worker", curdir) &&
				validateAgentBinary("ssm-session-worker", curdir) &&
				validateAgentBinary("ssm-session-logger", curdir) {
				DefaultDocumentWorker = filepath.Join(curdir, "ssm-document-worker")
				DefaultSessionWorker = filepath.Join(curdir, "ssm-session-worker")
				DefaultSessionLogger = filepath.Join(curdir, "ssm-session-logger")
				DefaultProgramFolder = curdir
			}
		}
	}
}

func validateAgentBinary(filename, curdir string) bool {
	//  binaries exist in the directory
	if info, err := os.Stat(filepath.Join(curdir, filename)); err == nil {
		mode := info.Mode()
		fileSys := info.Sys()

		if (mode.Perm() & NecessaryAgentBinaryPermissionMask) != NecessaryAgentBinaryPermissionMask {
			// Some necessary permissions are not set
			fmt.Println("Warning: Some necessary permissions are not set for: ", filename)
			return false
		}

		if (mode.Perm() & DisallowedAgentBinaryPermissionMask) != 0 {
			// Some disallowed permissions are set
			fmt.Println("Warning: Some disallowed permissions are set for: ", filename)
			return false
		}

		//binary ownership is root
		if fileSys.(*syscall.Stat_t).Uid == 0 &&
			fileSys.(*syscall.Stat_t).Gid == 0 {
			return true
		}
	}
	return false
}
