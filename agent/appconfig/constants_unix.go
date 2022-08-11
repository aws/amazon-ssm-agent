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

//go:build freebsd || linux || netbsd || openbsd
// +build freebsd linux netbsd openbsd

// Package appconfig manages the configuration of the agent.
package appconfig

import (
	"fmt"
	"os"
	"path/filepath"
	"syscall"

	"github.com/aws/amazon-ssm-agent/agent/network/certreader"
)

var (

	// AgentExtensions specified the root folder for various kinds of downloaded content
	AgentData = "/var/lib/amazon/ssm/"

	// PackageRoot specifies the directory under which packages will be downloaded and installed
	PackageRoot = AgentData + "packages"

	// PackageLockRoot specifies the directory under which package lock files will reside
	PackageLockRoot = AgentData + "locks/packages"

	// PackagePlatform is the platform name to use when looking for packages
	PackagePlatform = "linux"

	// DaemonRoot specifies the directory where daemon registration information is stored
	DaemonRoot = AgentData + "daemons"

	// LocalCommandRoot specifies the directory where users can submit command documents offline
	LocalCommandRoot = AgentData + "localcommands"

	// LocalCommandRootSubmitted is the directory where locally submitted command documents
	// are moved when they have been picked up
	LocalCommandRootSubmitted = AgentData + "localcommands/submitted"
	LocalCommandRootCompleted = AgentData + "localcommands/completed"

	// LocalCommandRootInvalid is the directory where locally submitted command documents
	// are moved if the service cannot validate the document (generally impossible via cli)
	LocalCommandRootInvalid = AgentData + "localcommands/invalid"

	// DownloadRoot specifies the directory under which files will be downloaded
	DownloadRoot = AgentData + "download/"

	// DefaultDataStorePath represents the directory for storing system data
	DefaultDataStorePath = AgentData

	// EC2ConfigDataStorePath represents the directory for storing ec2 config data
	EC2ConfigDataStorePath = "/var/lib/amazon/ec2config/"

	// EC2ConfigSettingPath represents the directory for storing ec2 config settings
	EC2ConfigSettingPath = "/var/lib/amazon/ec2configservice/"

	// UpdaterArtifactsRoot represents the directory for storing update related information
	UpdaterArtifactsRoot = AgentData + "update/"

	// UpdaterPidLockfile represents the location of the updater lockfile
	UpdaterPidLockfile = AgentData + "update.lock"

	// DefaultPluginPath represents the directory for storing plugins in SSM
	DefaultPluginPath = AgentData + "plugins"

	// ManifestCacheDirectory represents the directory for storing all downloaded manifest files
	ManifestCacheDirectory = AgentData + "manifests"

	// List all plugin names, unfortunately golang doesn't support const arrays of strings

	// RebootExitCode that would trigger a Soft Reboot
	RebootExitCode = 194

	// Default Custom Inventory Inventory Folder
	DefaultCustomInventoryFolder = AgentData + "inventory/custom"

	// Default Session files Folder
	SessionFilesPath = AgentData + "session"

	// PowerShellPluginCommandArgs is the arguments of powershell.exe to be used by the runPowerShellScript plugin
	PowerShellPluginCommandArgs = "-f"

	// Exit Code for a command that exits before completion (generally due to timeout or cancel)
	CommandStoppedPreemptivelyExitCode = 137 // Fatal error (128) + signal for SIGKILL (9) = 137

	// RunCommandScriptName is the script name where all downloaded or provided commands will be stored
	RunCommandScriptName = "_script.sh"

	NecessaryAgentBinaryPermissionMask  os.FileMode = 0511 // Require read/execute for root, execute for all
	DisallowedAgentBinaryPermissionMask os.FileMode = 0022 // Disallow write for group and user

	// customCertificateFileName is the name of the custom certificate
	customCertificateFileName = "amazon-ssm-agent.crt"

	// SSM Agent Update download legacy path
	LegacyUpdateDownloadFolder = "/var/log/amazon/ssm/download"

	// DefaultEC2SharedCredentialsFilePath represents the filepath for storing credentials for ec2 identity
	DefaultEC2SharedCredentialsFilePath = DefaultDataStorePath + "credentials"
)

// PowerShellPluginCommandName is the path of the powershell.exe to be used by the runPowerShellScript plugin
var PowerShellPluginCommandName string

// DefaultProgramFolder is the default folder for SSM
var DefaultProgramFolder = "/etc/amazon/ssm/"

var defaultWorkerPath = "/usr/bin/"
var DefaultSSMAgentBinaryPath = defaultWorkerPath + "amazon-ssm-agent"
var DefaultSSMAgentWorker = defaultWorkerPath + "ssm-agent-worker"
var DefaultDocumentWorker = defaultWorkerPath + "ssm-document-worker"
var DefaultSessionWorker = defaultWorkerPath + "ssm-session-worker"
var DefaultSessionLogger = defaultWorkerPath + "ssm-session-logger"

// AppConfigPath is the path of the AppConfig
var AppConfigPath = DefaultProgramFolder + AppConfigFileName

// CustomCertificatePath is the path of the custom certificate
var CustomCertificatePath = ""

// SeelogFilePath specifies the path to the seelog
var SeelogFilePath = DefaultProgramFolder + SeelogConfigFileName

var RuntimeConfigFolderPath = AgentData + "runtimeconfig"

func init() {
	/*
	   Powershell command used to be poweshell in alpha versions, now it's pwsh in prod versions
	*/
	PowerShellPluginCommandName = "/usr/bin/powershell"
	if _, err := os.Stat(PowerShellPluginCommandName); err != nil {
		PowerShellPluginCommandName = "/usr/bin/pwsh"
	}

	// curdir is amazon-ssm-agent current directory path
	curdir, err := filepath.Abs(filepath.Dir(os.Args[0]))
	if err != nil {
		return
	}

	// if curdir is not default worker dir, update paths of other binaries
	if curdir != defaultWorkerPath[:len(defaultWorkerPath)-1] {
		if validateAgentBinary("ssm-session-worker", curdir) &&
			validateAgentBinary("ssm-agent-worker", curdir) {

			DefaultSessionWorker = filepath.Join(curdir, "ssm-session-worker")
			DefaultSSMAgentWorker = filepath.Join(curdir, "ssm-agent-worker")
			DefaultProgramFolder = curdir

			if validateAgentBinary("ssm-document-worker", curdir) {
				DefaultDocumentWorker = filepath.Join(curdir, "ssm-document-worker")
			}

			if validateAgentBinary("ssm-session-logger", curdir) {
				DefaultSessionLogger = filepath.Join(curdir, "ssm-session-logger")
			}

			// Check if config is available in relative path
			const relativeConfigFolder = "configuration"
			if validateRelativeConfigFile(filepath.Join(curdir, relativeConfigFolder, AppConfigFileName)) {
				AppConfigPath = filepath.Join(curdir, relativeConfigFolder, AppConfigFileName)
			}

			// Check if seelog.xml is available in relative path
			if validateRelativeConfigFile(filepath.Join(curdir, relativeConfigFolder, SeelogConfigFileName)) {
				SeelogFilePath = filepath.Join(curdir, relativeConfigFolder, SeelogConfigFileName)
			}

			// Check if certificate is available in relative path
			const relativeCertsFolder = "certs"
			if _, err := certreader.ReadCertificate(filepath.Join(curdir, relativeCertsFolder, customCertificateFileName)); err == nil {
				CustomCertificatePath = filepath.Join(curdir, relativeCertsFolder, customCertificateFileName)
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

func validateRelativeConfigFile(filePath string) bool {
	// Get folder info
	info, err := os.Stat(filepath.Dir(filePath))
	if err != nil {
		return false
	}

	// Get folder resource information
	folderSys := info.Sys()
	if folderSys.(*syscall.Stat_t).Uid != 0 ||
		folderSys.(*syscall.Stat_t).Gid != 0 {
		return false
	}

	// Get file info
	info, err = os.Stat(filePath)
	if err != nil {
		return false
	}

	// Check if is file
	if !info.Mode().IsRegular() {
		return false
	}

	// check if file is owned by root
	fileSys := info.Sys()
	if fileSys.(*syscall.Stat_t).Uid != 0 ||
		fileSys.(*syscall.Stat_t).Gid != 0 {
		return false
	}

	return true
}
