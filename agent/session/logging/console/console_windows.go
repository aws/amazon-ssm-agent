// Copyright 2018 Amazon.com, Inc. or its affiliates. All Rights Reserved.
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
//
// +build windows

// Package console contains platform specific configurations to enable logging.
package console

import (
	"syscall"

	"golang.org/x/sys/windows"
)

func InitDisplayMode() (err error) {
	var (
		state          uint32
		fileDescriptor int
	)

	// gets handler for Stdout
	fileDescriptor = int(syscall.Stdout)
	handle := windows.Handle(fileDescriptor)

	// gets current console mode i.e. current console settings
	if err = windows.GetConsoleMode(handle, &state); err != nil {
		return
	}

	// this flag is set in order to support control character sequences
	// that control cursor movement, color/font mode
	// refer - https://docs.microsoft.com/en-us/windows/console/setconsolemode
	state |= windows.ENABLE_VIRTUAL_TERMINAL_PROCESSING

	// sets the console with new flag
	if err = windows.SetConsoleMode(handle, state); err != nil {
		return
	}

	return nil
}
