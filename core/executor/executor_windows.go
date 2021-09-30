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

//go:build windows
// +build windows

package executor

import (
	"os"
	"os/exec"

	ps "github.com/mitchellh/go-ps"
)

const (
	CWConfigIndex = 2
)

func prepareProcess(command *exec.Cmd) {
	// nothing to do on windows
}

func terminateProcess(process *os.Process) error {
	// process kill doesn't send proper signal to the process status
	// Setting the signal to indicate execution was interrupted
	return process.Kill()
}

func killProcess(process *os.Process) error {
	// process kill doesn't send proper signal to the process status
	// Setting the signal to indicate execution was interrupted
	return process.Kill()
}

// Running powershell on linux required the HOME env variable to be set and to remove the TERM env variable
func validateEnvironmentVariables(command *exec.Cmd) {
}

var getProcess = func() ([]OsProcess, error) {
	processes, err := ps.Processes()
	if err != nil {
		return nil, err
	}

	results := make([]OsProcess, len(processes))
	for _, process := range processes {
		results = append(results, OsProcess{Pid: process.Pid(), PPid: process.PPid(), Executable: process.Executable()})
	}

	return results, nil
}
