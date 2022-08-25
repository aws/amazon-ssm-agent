// Copyright 2022 Amazon.com, Inc. or its affiliates. All Rights Reserved.
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

//go:build windows
// +build windows

// Package constants manages the configuration of the session shell.
package constants

import (
	"fmt"

	"github.com/aws/amazon-ssm-agent/agent/framework/docparser/parameters"
	mgsContracts "github.com/aws/amazon-ssm-agent/agent/session/contracts"
)

// GetShellCommand return customer commands.
func GetShellCommand(shellProps mgsContracts.ShellProperties) string {
	return shellProps.Windows.Commands
}

// GetRunAsElevated return RunAsElevated value.
func GetRunAsElevated(shellProps mgsContracts.ShellProperties) bool {
	return shellProps.Windows.RunAsElevated
}

// GetSeparateOutputStream return whether need separate output stderr and stderr for non-interactive session.
func GetSeparateOutputStream(shellProps mgsContracts.ShellProperties) (bool, error) {
	separateOutPutStream, err := parameters.ConvertToBool(shellProps.Windows.SeparateOutputStream)
	if err != nil {
		err = fmt.Errorf("unable to convert separateOutPutStream: %v", err)
	}
	return separateOutPutStream, err
}

// GetStdOutSeparatorPrefix return the prefix used for StdOut partition
func GetStdOutSeparatorPrefix(shellProps mgsContracts.ShellProperties) string {
	return shellProps.Windows.StdOutSeparatorPrefix
}

// GetStdErrSeparatorPrefix return the prefix used for StdErr partition
func GetStdErrSeparatorPrefix(shellProps mgsContracts.ShellProperties) string {
	return shellProps.Windows.StdErrSeparatorPrefix
}
