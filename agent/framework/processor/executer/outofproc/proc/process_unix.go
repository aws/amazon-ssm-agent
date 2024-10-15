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

//go:build darwin || freebsd || linux || netbsd || openbsd
// +build darwin freebsd linux netbsd openbsd

// Package process wraps up the os.Process interface and also provides os-specific process lookup functions
package proc

import (
	"os/exec"
	"syscall"
)

func prepareProcess(command *exec.Cmd) {
	// set pgid to new pid, so that the process can survive when upstart/systemd kill the original process group
	command.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
}
