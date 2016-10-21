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

	// Exit Code that would trigger a Soft Reboot
	RebootExitCode = 3010

	// List all plugin names, unfortunately golang doesn't support const arrays of strings

	// PluginNameAwsRunScript is the name of the run script plugin
	PluginNameAwsRunScript = "aws:runPowerShellScript"

	// PluginNameAwsPowerShellModule is the name of the PowerShell Module
	PluginNameAwsPowerShellModule = "aws:psModule"

	// PluginNameAwsApplications is the name of the Applications plugin
	PluginNameAwsApplications = "aws:applications"
)

// Program Folder
var DefaultProgramFolder string

// AppConfig Path
var AppConfigPath string

// DefaultDataStorePath represents the directory for storing system data
var DefaultDataStorePath string

// DefaultPluginPath represents the directory for storing plugins in SSM
var DefaultPluginPath string

// DownloadRoot specifies the directory under which files will be downloaded
var DownloadRoot string

// UpdaterArtifactsRoot represents the directory for storing update related information
var UpdaterArtifactsRoot string

// EC2UpdaterArtifactsRoot represents the directory for storing ec2 config update related information
var EC2UpdateArtifactsRoot string

// EC2UpdaterDownloadRoot is the directory for downloading ec2 update related files
var EC2UpdaterDownloadRoot string

// UpdateContextFilePath is the path where the updatecontext.json file exists for Ec2 updater to find
var UpdateContextFilePath string

// SSMData specifies the directory we used to store SSM data.
var SSMDataPath string

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
	temp := os.Getenv("TEMP")

	DefaultProgramFolder = filepath.Join(EnvProgramFiles, SSMFolder)
	DefaultPluginPath = filepath.Join(EnvProgramFiles, SSMPluginFolder)
	AppConfigPath = filepath.Join(DefaultProgramFolder, AppConfigFileName)
	DefaultDataStorePath = filepath.Join(SSMDataPath, "InstanceData")
	DownloadRoot = filepath.Join(temp, SSMFolder, "Download")
	UpdaterArtifactsRoot = filepath.Join(temp, SSMFolder, "Update")
	DefaultCustomInventoryFolder = filepath.Join(SSMDataPath, "Inventory", "Custom")
	EC2UpdateArtifactsRoot = filepath.Join(EnvWinDir, EC2ConfigServiceFolder, "Update")
	EC2UpdaterDownloadRoot = filepath.Join(temp, EC2ConfigAppDataFolder, "Download")
	UpdateContextFilePath = filepath.Join(programData, EC2ConfigAppDataFolder, "Update\\UpdateContext.json")
}
