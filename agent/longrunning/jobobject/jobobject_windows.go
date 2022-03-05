// +build windows

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

// Package JobObject allows creation of job object for SSM agent process.
// This is to to control the lifetime of daemon processes launched via the RunDaemon plugin.
package jobobject

import (
	"syscall"
	"unsafe"

	"github.com/aws/amazon-ssm-agent/agent/log/ssmlog"
)

const (
	JobObjectExtendedLimitInformation = 9
	childprocessNotInheritHandle      = false
	processSetQuotaAccess             = 0x100
	processTerminateAccess            = 0x1
	jobObjectLimitkillonClose         = 0x2000
)

type (
	HANDLE uintptr
)

// Windows APIs
var (
	kernel32                 = syscall.NewLazyDLL("kernel32.dll")
	CreateJobObjectW         = kernel32.NewProc("CreateJobObjectW")
	AssignProcessToJobObject = kernel32.NewProc("AssignProcessToJobObject")
	SetInformationJobObject  = kernel32.NewProc("SetInformationJobObject")
)

var SSMjobObject syscall.Handle

type JobObjectBasicLimit struct {
	PerProcessUserTimeLimit uint64
	PerJobUserTimeLimit     uint64
	LimitFlags              uint32
	MinimumWorkingSetSize   uintptr
	MaximumWorkingSetSize   uintptr
	ActiveProcessLimit      uint32
	Affinity                uintptr
	PriorityClass           uint32
	SchedulingClass         uint32
}

type IoCounters struct {
	ReadOperationCount  uint64
	WriteOperationCount uint64
	OtherOperationCount uint64
	ReadTransferCount   uint64
	WriteTransferCount  uint64
	OtherTransferCount  uint64
}

type JobObjectExtendedLimit struct {
	BasicLimitInformation JobObjectBasicLimit
	IoInfo                IoCounters
	ProcessMemoryLimit    uintptr
	JobMemoryLimit        uintptr
	PeakProcessMemoryUsed uintptr
	PeakJobMemoryUsed     uintptr
}

// Function setInformationJobObject allows setting of specific properties on Job Objects.
func setInformationJobObject(job syscall.Handle, infoclass uint32, info uintptr, infolen uint32) (err error) {
	r1, _, e1 := SetInformationJobObject.Call(
		uintptr(job),
		uintptr(infoclass),
		uintptr(info),
		uintptr(infolen))
	if r1 == 0 {
		if e1 != nil {
			err = error(e1)
		} else {
			err = syscall.EINVAL
		}
	}
	return
}

// Function createJobObject creates a new job object.
func createJobObject(jobAttrs *syscall.SecurityAttributes, name *uint16) (handle syscall.Handle, err error) {
	r1, _, e1 := CreateJobObjectW.Call(
		uintptr(unsafe.Pointer(jobAttrs)),
		uintptr(unsafe.Pointer(name)))

	handle = syscall.Handle(r1)
	if handle == 0 {
		if e1 != nil {
			err = error(e1)
		} else {
			err = syscall.EINVAL
		}
	}
	return
}

// Function AttachProcessToJobObject attached child processes to the SSM agent job object.
func AttachProcessToJobObject(Pid uint32) (err error) {
	handle, err := syscall.OpenProcess(processSetQuotaAccess|processTerminateAccess, childprocessNotInheritHandle, Pid)
	if err != nil {
		return err
	}
	defer syscall.CloseHandle(handle)

	r1, _, e1 := AssignProcessToJobObject.Call(
		uintptr(SSMjobObject),
		uintptr(handle))

	if r1 == 0 {
		if e1 != nil {
			err = error(e1)
		} else {
			err = syscall.EINVAL
		}
	}
	return err
}

// Set up a job object for the SSM agent process on Windows. This is to control the lifetime of daemon processes
// launched via the ConfigureDaemon/RunDaemon plugin.
// The init function is automatically invoked prior to main function being invoked.
func init() {
	log := ssmlog.SSMLogger(true)

	var err error
	SSMjobObject, err = createJobObject(nil, nil)
	if err != nil {
		log.Infof("SSM Agent CreateJobObject failed: %v", err)
		return
	}

	var jobinfo JobObjectExtendedLimit
	jobinfo.BasicLimitInformation.LimitFlags = jobObjectLimitkillonClose

	err = setInformationJobObject(SSMjobObject, JobObjectExtendedLimitInformation, uintptr(unsafe.Pointer(&jobinfo)), uint32(unsafe.Sizeof(jobinfo)))
	if err != nil {
		log.Infof("SetInformationJobObject failed: %v", err)
		syscall.Close(SSMjobObject)
		return
	}
	log.Infof("Windows Only: Job object creation on SSM agent successful")
	return
}
