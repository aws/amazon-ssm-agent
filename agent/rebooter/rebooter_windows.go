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

// +build windows

// Package rebooter provides utilities used to reboot a machine.
package rebooter

import (
	"bytes"
	"os/exec"

	"github.com/aws/amazon-ssm-agent/agent/log"
)

const (
	timeOutInSecondsBeforeReboot = "60"
)

// reboot is performed by running the following command
// shutdown -r -t 60
// The above command will cause the machine to reboot after 60 seconds
func reboot(log log.T) (err error) {
	log.Infof("rebooting the machine in %v seconds..", timeOutInSecondsBeforeReboot)
	command := exec.Command("shutdown", "-r", "-t", timeOutInSecondsBeforeReboot)
	var stdout, stderr bytes.Buffer
	command.Stderr = &stderr
	command.Stdout = &stdout
	err = command.Start()
	log.Infof("shutdown output: %v\n", stdout.String())
	log.Errorf("shutdown error: %v\n", stderr.String())
	return
}
