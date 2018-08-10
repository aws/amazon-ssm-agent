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

// Package shell implements session shell plugin.
package shell

import (
	"fmt"
	"os"

	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/amazon-ssm-agent/agent/session/winpty"
)

var pty *winpty.WinPTY

const (
	defaultConsoleCol = 200
	defaultConsoleRow = 60
	winptyDllName     = "winpty"
	winptyDllPath     = ""
	winptyCmd         = "powershell"

	startRecordSessionCmd = "Start-Transcript"
	newLineCharacter      = "\r"
	screenBufferSizeCmd   = "$host.UI.RawUI.BufferSize = New-Object System.Management.Automation.Host.Size($host.UI.RawUI.BufferSize.Width,%d)%s"
)

//StartPty starts winpty agent and provides handles to stdin and stdout.
func StartPty(log log.T) (stdin *os.File, stdout *os.File, err error) {
	log.Info("Starting winpty")
	if pty, err = winpty.Start(winptyDllPath, winptyDllName, winptyCmd, defaultConsoleCol, defaultConsoleRow); err != nil {
		return nil, nil, fmt.Errorf("Start winpty failed: %s", err)
	}

	return pty.StdIn, pty.StdOut, nil
}

//Stop closes winpty process handle and stdin/stdout.
func Stop(log log.T) (err error) {
	log.Info("Stopping winpty")
	if err = pty.Close(); err != nil {
		return fmt.Errorf("Stop winpty failed: %s", err)
	}

	return nil
}

//SetSize sets size of console terminal window.
func SetSize(log log.T, ws_col, ws_row uint32) (err error) {
	if err = pty.SetSize(ws_col, ws_row); err != nil {
		return fmt.Errorf("Set winpty size failed: %s", err)
	}

	return nil
}
