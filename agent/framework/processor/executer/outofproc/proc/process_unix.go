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

// Package process wraps up the os.Process interface and also provides os-specific process lookup functions
package proc

import (
	"os/exec"
	"strconv"
	"strings"
	"syscall"
	"time"
)

//Unix man: http://www.skrenta.com/rt/man/ps.1.html , return the process table of the current user, in agent it'll be root
//verified on RHEL, Amazon Linux, Ubuntu, Centos, FreeBSD and Darwin
//TODO optimize this, do not print all processes; what we need is the process belongs to a specific user and no tty attached
var ps = func() ([]byte, error) {
	return exec.Command("ps", "-e", "-o", "pid,lstart").CombinedOutput()
}

func prepareProcess(command *exec.Cmd) {
	// set pgid to new pid, so that the process can survive when upstart/systemd kill the original process group
	command.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
}

//given the pid and the unix process startTime format string, return whether the process is still alive
func find_process(pid int, startTime time.Time) (bool, error) {
	output, err := ps()
	if err != nil {
		return false, err
	}
	proc_list := strings.Split(string(output), "\n")

	for i := 1; i < len(proc_list); i++ {
		parts := strings.Fields(proc_list[i])
		if len(parts) < 2 {
			continue
		}
		_pid, err := strconv.ParseInt(parts[0], 10, 64)
		if err != nil {
			return false, err
		}
		if pid == int(_pid) {
			return true, nil
		}
	}
	return false, nil
}

//TODO add time comparison
//compare the 2 UTC date time, whether the startTime is within one sec
func compareTimes(startTime time.Time, timeRaw string) bool {
	startTime = startTime.UTC()
	parsedTime, _ := time.Parse(time.ANSIC, timeRaw)
	return startTime.Before(parsedTime.Add(time.Second)) && startTime.After(parsedTime.Add(-time.Second))
}
