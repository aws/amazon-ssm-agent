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

// +build windows

// Package appconfig manages the configuration of the agent.
package appconfig

import (
	"fmt"
	"os"
	"path/filepath"
)

const (
	// SSMFolder is the path under local app data.
	SSMFolder = "Amazon\\SSM"

	// SSM plugins folder path under local app data.
	SSMPluginFolder = "Amazon\\SSM\\Plugins\\"

	// EC2ConfigAppDataFolder path under local app data required by updater
	EC2ConfigAppDataFolder = "Amazon\\Ec2Config"

	//Ec2configServiceFolder is the folder required by SSM agent
	EC2ConfigServiceFolder = "Amazon\\Ec2ConfigService"

	// ManifestCacheFolder path under local app data
	ManifestCacheFolder = "Amazon\\SSM\\Manifests"

	// Exit Code that would trigger a Soft Reboot
	RebootExitCode = 3010

	// PackagePlatform is the platform name to use when looking for packages
	PackagePlatform = "windows"

	// PowerShellPluginCommandArgs specifies the default arguments that we pass to powershell
	// Use Unrestricted as Execution Policy for running the script.
	// https://technet.microsoft.com/en-us/library/hh847748.aspx
	PowerShellPluginCommandArgs = "-InputFormat None -Noninteractive -NoProfile -ExecutionPolicy unrestricted -f"

	// Exit Code for a command that exits before completion (generally due to timeout or cancel)
	CommandStoppedPreemptivelyExitCode = -1

	// RunCommandScriptName is the script name where all downloaded or provided commands will be stored
	RunCommandScriptName = "_script.ps1"

	// ItemPropertyPath is the registry path for AmazonSSMAgent service
	ItemPropertyPath = "SYSTEM\\CurrentControlSet\\Services\\AmazonSSMAgent"

	// ItemPropertyName is the registry variable name that stores proxy settings
	ItemPropertyName = "Environment"
)

//PowerShellPluginCommandName is the path of the powershell.exe to be used by the runPowerShellScript plugin
var PowerShellPluginCommandName = filepath.Join(os.Getenv("SystemRoot"), "System32", "WindowsPowerShell", "v1.0", "powershell.exe")

// Program Folder
var DefaultProgramFolder string

//SSM Agent executable path
var DefaultSSMAgentWorker string

//Document executable path
var DefaultDocumentWorker string

//Session executable path
var DefaultSessionWorker string

//Session logger executable path
var DefaultSessionLogger string

// AppConfig Path
var AppConfigPath string

// DefaultDataStorePath represents the directory for storing system data
var DefaultDataStorePath string

// PackageRoot specifies the directory under which packages will be downloaded and installed
var PackageRoot string

// PackageLockRoot specifies the directory under which package lock files will reside
var PackageLockRoot string

// DaemonRoot specifies the directory where daemon registration information is stored
var DaemonRoot string

// LocalCommandRoot specifies the directory where users can submit command documents offline
var LocalCommandRoot string

// LocalCommandRootSubmitted is the directory where locally submitted command documents
// are moved when they have been picked up
var LocalCommandRootSubmitted string
var LocalCommandRootCompleted string

// LocalCommandRootInvalid is the directory where locally submitted command documents
// are moved if the service cannot validate the document (generally impossible via cli)
var LocalCommandRootInvalid string

// DefaultPluginPath represents the directory for storing plugins in SSM
var DefaultPluginPath string

// ManifestCacheDirectory represents the directory for storing all downloaded manifest files
var ManifestCacheDirectory string

// DownloadRoot specifies the directory under which files will be downloaded
var DownloadRoot string

// UpdaterArtifactsRoot represents the directory for storing update related information
var UpdaterArtifactsRoot string

// UpdaterPidLockfile represents the location of the updater lockfile
var UpdaterPidLockfile string

// EC2ConfigDataStorePath represents the directory for storing ec2 config data
var EC2ConfigDataStorePath string

// EC2ConfigSettingPath represents the directory for storing ec2 config settings
var EC2ConfigSettingPath string

// EC2UpdateArtifactsRoot represents the directory for storing ec2 config update related information
var EC2UpdateArtifactsRoot string

// EC2UpdaterDownloadRoot is the directory for downloading ec2 update related files
var EC2UpdaterDownloadRoot string

// UpdateContextFilePath is the path where the updatecontext.json file exists for Ec2 updater to find
var UpdateContextFilePath string

// SSMData specifies the directory we used to store SSM data.
var SSMDataPath string

// SessionFilesPath specifies the directory where session specific files are stored.
var SessionFilesPath string

// Windows environment variable %ProgramFiles%
var EnvProgramFiles string

// Windows environment variable %WINDIR%
var EnvWinDir string

// Default Custom Inventory Data Folder
var DefaultCustomInventoryFolder string

// Plugin folder path
var PluginFolder string

func init() {
	/*
		System environment variable "AllUsersProfile" maps to following locations in different locations:

		WindowsServer 2003  -> C:\Documents and Settings\All Users\Application Data
		WindowsServer 2008+ -> C:\ProgramData
	*/

	programData := os.Getenv("ProgramData")
	if programData == "" {
		programData = filepath.Join(os.Getenv("AllUsersProfile"), "Application Data")
	}
	SSMDataPath = filepath.Join(programData, SSMFolder)

	EnvProgramFiles = os.Getenv("ProgramFiles")
	EnvWinDir = os.Getenv("WINDIR")

	DefaultProgramFolder = filepath.Join(EnvProgramFiles, SSMFolder)
	DefaultPluginPath = filepath.Join(EnvProgramFiles, SSMPluginFolder)
	DefaultSSMAgentWorker = filepath.Join(DefaultProgramFolder, "ssm-agent-worker.exe")
	DefaultDocumentWorker = filepath.Join(DefaultProgramFolder, "ssm-document-worker.exe")
	DefaultSessionWorker = filepath.Join(DefaultProgramFolder, "ssm-session-worker.exe")
	DefaultSessionLogger = fmt.Sprintf("&'%s'", filepath.Join(DefaultProgramFolder, "ssm-session-logger.exe"))
	ManifestCacheDirectory = filepath.Join(EnvProgramFiles, ManifestCacheFolder)
	AppConfigPath = filepath.Join(DefaultProgramFolder, AppConfigFileName)
	DefaultDataStorePath = filepath.Join(SSMDataPath, "InstanceData")
	PackageRoot = filepath.Join(SSMDataPath, "Packages")
	PackageLockRoot = filepath.Join(SSMDataPath, "Locks\\Packages")
	DaemonRoot = filepath.Join(SSMDataPath, "Daemons")
	LocalCommandRoot = filepath.Join(SSMDataPath, "LocalCommands")
	LocalCommandRootSubmitted = filepath.Join(LocalCommandRoot, "Submitted")
	LocalCommandRootCompleted = filepath.Join(LocalCommandRoot, "Completed")
	LocalCommandRootInvalid = filepath.Join(LocalCommandRoot, "Invalid")
	DownloadRoot = filepath.Join(SSMDataPath, "Download")
	UpdaterArtifactsRoot = filepath.Join(SSMDataPath, "Update")
	UpdaterPidLockfile = filepath.Join(SSMDataPath, "update.lock")

	DefaultCustomInventoryFolder = filepath.Join(SSMDataPath, "Inventory", "Custom")
	EC2UpdateArtifactsRoot = filepath.Join(programData, EC2ConfigAppDataFolder, "Updater")
	EC2UpdaterDownloadRoot = filepath.Join(programData, EC2ConfigAppDataFolder, "Downloads")
	EC2ConfigDataStorePath = filepath.Join(programData, EC2ConfigAppDataFolder, "InstanceData")
	UpdateContextFilePath = filepath.Join(programData, EC2ConfigAppDataFolder, "Update\\UpdateContext.json")
	EC2ConfigSettingPath = filepath.Join(EnvProgramFiles, EC2ConfigServiceFolder, "Settings")
	SessionFilesPath = filepath.Join(SSMDataPath, "Session")

}
