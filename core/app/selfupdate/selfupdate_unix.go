// Copyright 2020 Amazon.com, Inc. or its affiliates. All Rights Reserved.
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

// +build darwin freebsd linux netbsd openbsd

// Package selfupdate provides an interface to force update with Message Gateway Service and S3
// This file contains the constant for generating command for linux os

package selfupdate

import (
	"os/exec"
	"syscall"
)

func prepareProcess(command *exec.Cmd) {
	// make the process the leader of its process group
	// (otherwise we cannot kill it properly)
	command.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
}
