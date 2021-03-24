// Copyright 2021 Amazon.com, Inc. or its affiliates. All Rights Reserved.
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

// Package constants manages the configuration of the session shell.
package constants

import (
	mgsContracts "github.com/aws/amazon-ssm-agent/agent/session/contracts"
)

const (
	HomeEnvVariable     = "HOME=/home/"
	RootHomeEnvVariable = "HOME=/root"
)

func GetShellCommand(shellProps mgsContracts.ShellProperties) string {
	return shellProps.Linux.Commands
}

func GetRunAsElevated(shellProps mgsContracts.ShellProperties) bool {
	return shellProps.Linux.RunAsElevated
}
