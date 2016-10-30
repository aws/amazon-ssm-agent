// Copyright 2016 Amazon.com, Inc. or its affiliates. All Rights Reserved.
//
// Licensed under the AWS Customer Agreement (the "License"). You may
// not use this file except in compliance with the License. A copy of the
// License is located at
//
// http://aws.amazon.com/agreement/
//
// Package pluginutil implements some common functions shared by multiple plugins.
// pluginutil_windows contains a function for returning the ResultStatus based on the exitCode
//
// +build windows

package pluginutil

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/aws/amazon-ssm-agent/agent/appconfig"
	"github.com/aws/amazon-ssm-agent/agent/contracts"
	"github.com/aws/amazon-ssm-agent/agent/task"
)

const (
	// RunCommandScriptName is the script name where all downloaded or provided commands will be stored
	RunCommandScriptName = "_script.ps1"
	// PowershellArgs specifies the default arguments that we pass to powershell
	// Use Unrestricted as Execution Policy for running the script.
	// https://technet.microsoft.com/en-us/library/hh847748.aspx
	PowerShellArgs = "-InputFormat None -Noninteractive -NoProfile -ExecutionPolicy unrestricted -f"
	// Currently we run powershell as powershell.exe [arguments], with this approach we are not able to get the $LASTEXITCODE value
	// if we want to run multiple commands then we need to run them via shell and not directly the command.
	// https://groups.google.com/forum/#!topic/golang-nuts/ggd3ww3ZKcI
	ExitCodeTrap                       = " ; exit $LASTEXITCODE"
	CommandStoppedPreemptivelyExitCode = -1
)

var PowerShellCommand = filepath.Join(os.Getenv("SystemRoot"), "System32", "WindowsPowerShell", "v1.0", "powershell.exe")

// GetStatus returns a ResultStatus variable based on the received exitCode
func GetStatus(exitCode int, cancelFlag task.CancelFlag) contracts.ResultStatus {
	switch exitCode {
	case appconfig.SuccessExitCode:
		return contracts.ResultStatusSuccess
	case appconfig.RebootExitCode:
		return contracts.ResultStatusSuccessAndReboot
	case CommandStoppedPreemptivelyExitCode:
		if cancelFlag.ShutDown() {
			return contracts.ResultStatusFailed
		}
		if cancelFlag.Canceled() {
			return contracts.ResultStatusCancelled
		}
		return contracts.ResultStatusTimedOut
	default:
		return contracts.ResultStatusFailed
	}
}

func GetShellCommand() string {
	return PowerShellCommand
}

func GetShellArguments() []string {
	return strings.Split(PowerShellArgs, " ")
}

func GetScriptSelfDeleteCommand(scriptPath string) string {
	return "Remove-Item " + scriptPath
}
