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
	"archive/zip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"syscall"
	"unsafe"

	"github.com/aws/amazon-ssm-agent/agent/appconfig"
)

// Uncompress unzips the installation package
func Uncompress(src, dest string) error {
	r, err := zip.OpenReader(src)
	if err != nil {
		return err
	}
	defer func() {
		if err := r.Close(); err != nil {
			return
		}
	}()

	os.MkdirAll(dest, appconfig.ReadWriteExecuteAccess)
	// Closure to address file descriptors issue with all the deferred .Close() methods
	extractAndWriteFile := func(f *zip.File) error {
		rc, err := f.Open()
		if err != nil {
			return err
		}
		defer func() {
			if err := rc.Close(); err != nil {
				return
			}
		}()

		path := filepath.Join(dest, f.Name)

		if !isUnderDir(path, dest) {
			return fmt.Errorf("%v attepts to place files outside %v subtree", f.Name, dest)
		}
		if f.FileInfo().IsDir() {
			os.MkdirAll(path, f.Mode())
		} else {
			os.MkdirAll(filepath.Dir(path), f.Mode())
			f, err := os.OpenFile(path, appconfig.FileFlagsCreateOrTruncate, f.Mode())
			if err != nil {
				return err
			}
			defer func() {
				if err := f.Close(); err != nil {
					return
				}
			}()

			_, err = io.Copy(f, rc)
			if err != nil {
				return err
			}
		}
		return nil
	}
	for _, f := range r.File {
		err := extractAndWriteFile(f)
		if err != nil {
			return err
		}
	}

	return nil
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
