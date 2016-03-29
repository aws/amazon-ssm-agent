// Copyright 2016 Amazon.com, Inc. or its affiliates. All Rights Reserved.
//
// Licensed under the Amazon Software License (the "License"). You may not
// use this file except in compliance with the License. A copy of the
// License is located at
//
// http://aws.amazon.com/asl/
//
// or in the "license" file accompanying this file. This file is distributed
// on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND,
// express or implied. See the License for the specific language governing
// permissions and limitations under the License.

// +build darwin freebsd linux netbsd openbsd

// Package updateutil contains updater specific utilities.
package updateutil

import (
	"os/exec"
	"syscall"
)

func prepareProcess(command *exec.Cmd) {
	// make the process the leader of its process group
	// (otherwise we cannot kill it properly)
	command.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
}

const (
	// CompressFormat represents the compress format for linux platform
	CompressFormat = "tar.gz"
)
