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

// Package fileutil contains utilities for working with the file system.
package fileutil

import (
	"os"
	"syscall"
	"unsafe"

	"github.com/aws/amazon-ssm-agent/agent/appconfig"
	"github.com/aws/amazon-ssm-agent/agent/log"
)

// Uncompress unzips the installation package
func Uncompress(log log.T, src, dest string) error {
	return Unzip(src, dest)
}

// GetDiskSpaceInfo returns available, free, and total bytes respectively from system disk space
func GetDiskSpaceInfo() (diskSpaceInfo DiskSpaceInfo, err error) {
	var wd string
	var availBytes, totalBytes, freeBytes int64

	// Get a rooted path name
	if wd, err = os.Getwd(); err != nil {
		return
	}

	// Load kernel32.dll and find GetDiskFreeSpaceEX function
	getDiskFreeSpace := syscall.MustLoadDLL("kernel32.dll").MustFindProc("GetDiskFreeSpaceExW")

	// Get the available bytes (for arguments, GetDiskFreeSpace function takes dir name, avail, total, and free respectively)
	_, _, err = getDiskFreeSpace.Call(
		uintptr(unsafe.Pointer(syscall.StringToUTF16Ptr(wd))),
		uintptr(unsafe.Pointer(&availBytes)),
		uintptr(unsafe.Pointer(&totalBytes)),
		uintptr(unsafe.Pointer(&freeBytes)))

	return DiskSpaceInfo{
		AvailBytes: availBytes,
		FreeBytes:  freeBytes,
		TotalBytes: totalBytes,
	}, nil
}

// HardenDataFolder sets permission of %PROGRAM_DATA% folder for Windows. In
// Linux, each components handles the permission of its data.
func HardenDataFolder() error {
	return Harden(appconfig.SSMDataPath)
}
