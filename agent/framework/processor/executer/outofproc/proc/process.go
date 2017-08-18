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

	"github.com/aws/amazon-ssm-agent/agent/context"
)

//ProcessController encapsulate a os.Process object and provide limited access to the subprocesss
//fork does not apply in multi-thread scenario, we have to start a new executable
type ProcessController interface {
	//start a new process, with its I/O attached to the current parent process
	StartProcess(name string, argv []string) (pid int, err error)
	//release the attached sub-process; if the sub-process is already detached, this call should be no-op
	Release() error
	//kill the enclosed process, no-op if the process non exists
	Kill() error
	//given pid and process create time, return true is the process is still active
	Find(pid int, createTime string) bool
}

type OSProcess struct {
	process  *os.Process
	attached bool
	context  context.T
}

func NewOSProcess(ctx context.T) *OSProcess {
	return &OSProcess{
		attached: false,
		context:  ctx.With("[OSProcess]"),
	}
}

func (p *OSProcess) StartProcess(name string, argv []string) (pid int, err error) {
	log := p.context.Log()
	var procAttr os.ProcAttr
	if p.process, err = os.StartProcess(name, argv, &procAttr); err != nil {
		log.Errorf("start process: &v encountered error : %v", name, err)
		return
	}
	pid = p.process.Pid
	p.attached = true
	return
}

func (p *OSProcess) Release() error {
	if p.attached {
		p.context.Log().Debug("Releasing os process...")
		p.attached = false
		return p.process.Release()
	}
	return nil
}

//TODO revisit this when os.Process is actually lost during restart
func (p *OSProcess) Kill() error {
	if p.attached {
		p.context.Log().Debug("Killing os process...")
		p.attached = false
		return p.Kill()
	}
	return nil
}

func (p *OSProcess) Find(pid int, createTime string) bool {
	found, err := find_process(pid, createTime)
	if err != nil {
		p.context.Log().Errorf("error encountered when looking up process info: %v", err)
		return false
	}
	return found
}
