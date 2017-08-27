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

// Package process wraps up the os.Process interface and also provides os-specific process lookup functions
package proc

import (
	"errors"
	"fmt"
	"os/exec"
	"syscall"
	"time"
)

var (
	//https://msdn.microsoft.com/en-us/library/windows/desktop/ms724290(v=vs.85).aspx
	windowsBaseTime = time.Date(1601, 1, 1, 0, 0, 0, 0, time.UTC)
)

func prepareProcess(command *exec.Cmd) {
	// nothing to do on windows
}

//given the pid and the high order filetime, look up the process
func find_process(pid int, startTime time.Time) (bool, error) {
	const da = syscall.STANDARD_RIGHTS_READ |
		syscall.PROCESS_QUERY_INFORMATION | syscall.SYNCHRONIZE
	handle, err := syscall.OpenProcess(da, false, uint32(pid))
	defer syscall.CloseHandle(handle)
	if err != nil {
		return false, fmt.Errorf("open process error: ", err)
	}
	//process exists, check whether the creation time matches
	var u syscall.Rusage
	err = syscall.GetProcessTimes(syscall.Handle(handle), &u.CreationTime, &u.ExitTime, &u.KernelTime, &u.UserTime)

	if err != nil {
		return false, errors.New("unable to get process time")
	}
	//TODO add start time comparison
	return true, nil
}

//TODO add date comparison
//compare the filetime and Date time, whether they are within 1sec range
func compare(ftime syscall.Filetime, startTime time.Time) bool {
	parsedTime := windowsBaseTime.Add(time.Duration(ftime.Nanoseconds()))
	return startTime.Before(parsedTime.Add(time.Second)) && startTime.After(parsedTime.Add(-time.Second))
}
