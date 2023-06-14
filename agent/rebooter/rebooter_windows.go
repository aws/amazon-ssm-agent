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

//go:build windows
// +build windows

// Package rebooter provides utilities used to reboot a machine.
package rebooter

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/aws/amazon-ssm-agent/agent/log"
)

const (
	timeOutInSecondsBeforeReboot = "60"
)

var shutdownEXELocation = filepath.Join(os.Getenv("SystemRoot"), "System32", "shutdown.exe")

// reboot is performed by running the following command
// shutdown -r -t 60
// The above command will cause the machine to reboot after 60 seconds
func reboot(log log.T) error {
	log.Infof("rebooting the machine in %v seconds..", timeOutInSecondsBeforeReboot)
	command := exec.Command("shutdown", "-r", "-t", timeOutInSecondsBeforeReboot)
	var stdout, stderr bytes.Buffer
	command.Stderr = &stderr
	command.Stdout = &stdout
	err := command.Start()
	log.Infof("shutdown output: %v\n", stdout.String())

	if stderr.Len() != 0 {
		log.Warnf("shutdown error: %v\n", stderr.String())
	}

	if err != nil {
		log.Info("Retrying shutdown with at System32 location")
		stdout = bytes.Buffer{}
		stderr = bytes.Buffer{}
		command := exec.Command(shutdownEXELocation, "-r")
		command.Stdout = &stdout
		command.Stderr = &stderr
		err = command.Start()
		log.Infof("Shutdown output: %v\n", stdout.String())
		if stderr.Len() != 0 || err != nil {
			log.Errorf("Shutdown error: %v\n", stderr.String())
			return err
		}
	}

	return nil
}
