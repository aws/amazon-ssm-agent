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
	"golang.org/x/sys/windows/registry"
)

var PowerShellCommand = filepath.Join(os.Getenv("SystemRoot"), "System32", "WindowsPowerShell", "v1.0", "powershell.exe")

// GetStatus returns a ResultStatus variable based on the received exitCode
func GetStatus(exitCode int, cancelFlag task.CancelFlag) contracts.ResultStatus {
	switch exitCode {
	case appconfig.SuccessExitCode:
		return contracts.ResultStatusSuccess
	case appconfig.RebootExitCode:
		return contracts.ResultStatusSuccessAndReboot
	case appconfig.CommandStoppedPreemptivelyExitCode:
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
	return strings.Split(appconfig.PowerShellPluginCommandArgs, " ")
}

func LocalRegistryKeyGetStringsValue(path string, name string) (val []string, valtype uint32, err error) {
	key, err := openLocalRegistryKey(path)
	if err != nil {
		return nil, 0, err
	}
	defer key.Close()
	return key.GetStringsValue(name)
}

func openLocalRegistryKey(path string) (registry.Key, error) {
	return registry.OpenKey(registry.LOCAL_MACHINE, path, registry.ALL_ACCESS)
}
