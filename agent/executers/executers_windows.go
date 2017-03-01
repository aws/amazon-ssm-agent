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

// +build windows

package executers

import (
	"os"
	"os/exec"
	"strings"

	logger "github.com/aws/amazon-ssm-agent/agent/log"
)

const (
	CWConfigIndex = 2
)

func prepareProcess(command *exec.Cmd) {
	// nothing to do on windows
}

func killProcess(process *os.Process, signal *timeoutSignal) error {
	// process kill doesn't send proper signal to the process status
	// Setting the signal to indicate execution was interrupted
	signal.execInterruptedOnWindows = true
	return process.Kill()
}

// Running powershell on linux required the HOME env variable to be set and to remove the TERM env variable
func validateEnvironmentVariables(command *exec.Cmd) {
}

// printCommand is to print the directory and command. For cloudwatch, first exposed credentials are removed
// from the configuration file before logging
func printCommand(log logger.T, workingDir string, commandName string, commandArguments []string) {
	commandArgs := commandArguments

	// TODO: Add logic to make this more generic
	if strings.Contains(commandName, "AWS.CloudWatch.exe") {
		if len(commandArguments) > CWConfigIndex {
			commandArgs[CWConfigIndex] = logger.PrintCWConfig(commandArguments[CWConfigIndex], log)
		}
	}

	log.Debug()
	log.Debugf("Running in directory %v, command: %v.", workingDir, commandName, commandArgs)
	log.Debug()
}
