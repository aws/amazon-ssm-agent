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

// Package execcmd wraps up the os.Process interface.
package execcmd

import (
	"fmt"
	"os"
	"os/exec"
)

//IExecCmd is an abstracted interface of os.Process
type IExecCmd interface {
	Pid() int
	//kill the attached child process
	Kill() error
	//start the command execution process
	Start() error
	//wait for the child to finish
	Wait() error
	// sends a signal to the command process
	Signal(sig os.Signal) error
}

//implementation of IExecCmd with os.Process embed
type ExecCmd struct {
	Cmd *exec.Cmd
}

// NewExecCmd returns a new instance of the ExecCmd
func NewExecCmd(command *exec.Cmd) *ExecCmd {
	return &ExecCmd{Cmd: command}
}

// Pid returns the process id.
func (p *ExecCmd) Pid() int {
	return p.Cmd.Process.Pid
}

// Kill causes the Process to exit immediately. Kill does not wait until
// the Process has actually exited. This only kills the Process itself,
// not any other processes it may have started.
func (p *ExecCmd) Kill() error {
	if p.Cmd != nil && p.Cmd.Process != nil {
		return p.Cmd.Process.Kill()
	}
	return nil
}

// Start starts the command execution process with the Cmd.
func (p *ExecCmd) Start() error {
	if p.Cmd != nil {
		return p.Cmd.Start()
	}
	return nil
}

// Wait releases any resources associated with the Cmd.
func (p *ExecCmd) Wait() error {
	if p.Cmd != nil {
		return p.Cmd.Wait()
	}
	return nil
}

// Signal sends a signal to the command process
func (p *ExecCmd) Signal(sig os.Signal) error {
	if p.Cmd == nil || p.Cmd.Process == nil {
		return fmt.Errorf("the command execution has not started yet")
	}
	return p.Cmd.Process.Signal(sig)
}
