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
	"strconv"
	"syscall"
)

//given the pid and the high order filetime, look up the process
func find_process(pid int, stime string) (bool, error) {
	const da = syscall.STANDARD_RIGHTS_READ |
		syscall.PROCESS_QUERY_INFORMATION | syscall.SYNCHRONIZE
	handle, err := syscall.OpenProcess(da, false, uint32(pid))
	if err != nil {
		return false, fmt.Errorf("open process error: ", err)
	}
	//process exists, check whether the creation time matches
	var u syscall.Rusage
	err = syscall.GetProcessTimes(syscall.Handle(handle), &u.CreationTime, &u.ExitTime, &u.KernelTime, &u.UserTime)

	highDateTime, err := strconv.ParseUint(stime, 10, 32)
	if err != nil {
		return false, errors.New("unable to parse filetime")
	}
	if u.CreationTime.HighDateTime == highDateTime {
		return true, nil
	} else {
		return false, nil
	}

}

//return the high-order filetime of the current UTC time
func get_current_time() string {
	var curtime = syscall.Filetime{}
	syscall.GetSystemTimeAsFileTime(&curtime)
	return strconv.FormatUint(curtime.HighDateTime, 10)
}
