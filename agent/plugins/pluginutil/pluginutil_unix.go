// Copyright 2016 Amazon.com, Inc. or its affiliates. All Rights Reserved.
//
// Licensed under the AWS Customer Agreement (the "License"). You may
// not use this file except in compliance with the License. A copy of the
// License is located at
//
// http://aws.amazon.com/agreement/
//
// Package pluginutil implements some common functions shared by multiple plugins.
// pluginutil_unix contains a function for returning the ResultStatus based on the exitCode
//
// +build darwin freebsd linux netbsd openbsd

package pluginutil

import (
	"fmt"

	"github.com/aws/amazon-ssm-agent/agent/appconfig"
	"github.com/aws/amazon-ssm-agent/agent/contracts"
	"github.com/aws/amazon-ssm-agent/agent/task"
)

var ShellCommand = "sh"
var ShellArgs = []string{"-c"}

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
	return ShellCommand
}

func GetShellArguments() []string {
	return ShellArgs
}

func LocalRegistryKeyGetStringsValue(path string, name string) (val []string, valtype uint32, err error) {
	return nil, 0, fmt.Errorf("Not supported.")
}
