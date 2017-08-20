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

// Package process wraps up the os.Process interface and also provides os-specific process lookup functions
package proc

import (
	"os"
	"time"

	"github.com/aws/amazon-ssm-agent/agent/log"
)

//OSProcess is an abstracted interface of os.Process
type OSProcess interface {
	//generic ssm visible fields
	Pid() int
	StartTime() time.Time
	//kill the attached child process
	Kill() error
	//wait for the child to finish, if parent dies halfway, the child process is detached and becomes orphan
	//On Unix system, orphan is not be killed by systemd or upstart by default
	//On Windows service controller, stop service does not kill orphan by default
	//TODO confirm MSI Installer does not kill the child process
	Wait() (ProcessState, error)
}

//adapter over os.ProcessState
type ProcessState interface {
	Success() bool
}

//impl of OSProcess with os.Process embed
type WorkerProcess struct {
	*os.Process
	startTime time.Time
}

func (p *WorkerProcess) Pid() int {
	return p.Process.Pid
}

func (p *WorkerProcess) StartTime() time.Time {
	return p.startTime
}

func (p *WorkerProcess) Wait() (ProcessState, error) {
	state, err := p.Process.Wait()
	return ProcessState(state), err
}

//start a child process, with the resources attached to its parent
func StartProcess(name string, argv []string) (OSProcess, error) {
	var procAttr os.ProcAttr
	proc, err := os.StartProcess(name, argv, &procAttr)
	p := WorkerProcess{
		proc,
		time.Now().UTC(),
	}

	return &p, err
}

//os.FindProcess() doesn't work on Linux: https://groups.google.com/forum/#!topic/golang-nuts/hqrp0UHBK9k
//what we can only do is check whether it exists
func IsProcessExists(log log.T, pid int, createTime time.Time) bool {
	found, err := find_process(pid, createTime)
	if err != nil {
		log.Errorf("encountered error when finding process: %v", err)
	}
	return found
}
