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

// +build darwin freebsd linux netbsd openbsd

// Package rebooter provides utilities used to reboot a machine.
package rebooter

import (
	"bytes"
	"os/exec"
	"syscall"

	"github.com/aws/amazon-ssm-agent/agent/log"
)

const (
	timeOutInMinutesBeforeReboot = "+1" // Indicates 1 minute
)

// reboot is performed by running the following command
// /sbin/shutdown -r +1
// The above command will cause the machine to reboot after 1 minute
func reboot(log log.T) (err error) {
	log.Infof("Rebooting the machine in %v Minutes..", timeOutInMinutesBeforeReboot)
	command := exec.Command("/sbin/shutdown", "-r", timeOutInMinutesBeforeReboot)
	command.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	var stdout, stderr bytes.Buffer
	command.Stderr = &stderr
	command.Stdout = &stdout
	err = command.Start()
	log.Infof("shutdown output: %v\n", stdout.String())

	if stderr.Len() != 0 {
		log.Errorf("shutdown error: %v\n", stderr.String())
	}
	return
}
