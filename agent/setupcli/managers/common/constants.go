// Copyright 2023 Amazon.com, Inc. or its affiliates. All Rights Reserved.
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

// Package common contains common constants and functions needed to be accessed across ssm-setup-cli
package common

import (
	"os"
	"path/filepath"
)

// AgentStatus holds agent's running status
type AgentStatus string

const (
	// UndefinedStatus states that agent's state is unknown or undefined
	UndefinedStatus AgentStatus = "UndefinedStatus"
	// Running states that agent's state is in Running mode
	Running AgentStatus = "Running"
	// Stopped states that agent's state is in Stopped mode
	Stopped AgentStatus = "Stopped"
	// NotInstalled states that agent is not installed on the instance
	NotInstalled AgentStatus = "NotInstalled"
)

// GetPowershellPath returns the path for powershell in Windows
func GetPowershellPath() string {
	return filepath.Join(os.Getenv("SystemRoot"), "System32", "WindowsPowerShell", "v1.0", "powershell.exe")
}

// SetupCLIEnvironment states the different environments the ssm-setup-cli can run
type SetupCLIEnvironment string

const (
	// GreengrassEnv denotes the greengrass environment
	GreengrassEnv SetupCLIEnvironment = "greengrass"
	// OnPremEnv denotes the onprem environment
	OnPremEnv SetupCLIEnvironment = "onprem"

	// AmazonWindowsSetupFile denotes the name of Agent Windows Setup File
	AmazonWindowsSetupFile = "AmazonSSMAgentSetup.exe"

	// AmazonSSMExecutable denotes windows executable used on Nano
	AmazonSSMExecutable = "amazon-ssm-agent.exe"

	// PackageNotInstalledExitCode denotes exitCode for package install error
	PackageNotInstalledExitCode = 1
)
