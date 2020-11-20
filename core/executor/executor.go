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

// Package executor contains general purpose command executing objects.
package executor

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/amazon-ssm-agent/core/workerprovider/longrunningprovider/model"
)

// OsProcess represent the process information for the worker, such as pid and binary name
type OsProcess struct {
	Pid        int
	PPid       int
	Executable string
	State      string
}

// IExecutor is the interface type for ProcessExecutor.
type IExecutor interface {
	Start(*model.WorkerConfig) (*model.Process, error)
	Processes() ([]OsProcess, error)
	IsPidRunning(pid int) (bool, error)
	Kill(pid int) error
}

// ProcessExecutor is specially added for testing purposes
type ProcessExecutor struct {
	log log.T
}

// NewWorkerDiscover returns worker discover
func NewProcessExecutor(log log.T) *ProcessExecutor {
	return &ProcessExecutor{
		log: log,
	}
}

// Start starts a list of shell commands in the given working directory.
// Returns process started, an exit code (0 if successfully launch, 1 if error launching process), and a set of errors.
// The errors need not be fatal - the output streams may still have data
// even though some errors are reported. For example, if the command got killed while executing,
// the streams will have whatever data was printed up to the kill point, and the errors will
// indicate that the process got terminated.
func (exc *ProcessExecutor) Start(workerConfig *model.WorkerConfig) (*model.Process, error) {
	exc.log.Debugf("Starting process base on config %+v", workerConfig)
	command := exec.Command(workerConfig.Path, workerConfig.Args...)
	prepareProcess(command)
	// configure environment variables
	prepareEnvironment(command)

	if err := command.Start(); err != nil {
		return &model.Process{
			Pid:    0,
			Status: model.Unknown,
		}, err
	}

	//TODO: support environment variable
	return &model.Process{
		Pid:    command.Process.Pid,
		Status: model.Active,
	}, nil
}

// Processes returns running processes on the instance
func (exc *ProcessExecutor) Processes() ([]OsProcess, error) {
	return getProcess()
}

// IsPidRunning returns true if process with pid is running
func (exc *ProcessExecutor) IsPidRunning(pid int) (bool, error) {
	processes, err := getProcess()

	if err != nil {
		return false, err
	}

	for _, process := range processes {
		if process.Pid == pid {
			if process.State == "Z" {
				return false, nil
			}
			return true, nil
		}
	}

	return false, nil
}

func (exc *ProcessExecutor) Kill(pid int) error {
	osProcess, err := os.FindProcess(pid)
	if err != nil {
		return fmt.Errorf("failed to find process %v, %s", pid, err)
	}

	exc.log.Debugf("Found process %v, terminating", pid)
	if err = osProcess.Kill(); err != nil {
		return fmt.Errorf("failed to kill process %v, %s", pid, err)
	}

	exc.log.Debugf("Attempting to clean up process %v resources", pid)
	if _, err = osProcess.Wait(); err != nil {
		// Debug because agent can only clean up child processes
		exc.log.Debugf("Failed to clean up non-child process %v, %s", pid, err)
	}

	return nil
}

// prepareEnvironment adds ssm agent standard environment variables to the command
func prepareEnvironment(command *exec.Cmd) {
	env := os.Environ()
	command.Env = env

	// Running powershell on linux erquired the HOME env variable to be set and to remove the TERM env variable
	validateEnvironmentVariables(command)
}

// fmtEnvVariable creates the string to append to the current set of environment variables.
func fmtEnvVariable(name string, val string) string {
	return fmt.Sprintf("%s=%s", name, val)
}
