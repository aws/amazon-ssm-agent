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
// +build darwin freebsd linux netbsd openbsd

// Package shell implements session shell plugin.
package shell

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/kr/pty"
)

var ptyFile *os.File

const (
	termEnvVariable       = "TERM=xterm-256color"
	startRecordSessionCmd = "script"
	newLineCharacter      = "\n"
	screenBufferSizeCmd   = "screen -h %d%s"
)

//StartPty starts pty and provides handles to stdin and stdout
func StartPty(log log.T) (stdin *os.File, stdout *os.File, err error) {
	log.Info("Starting pty")
	//Start the command with a pty
	cmd := exec.Command("sh")

	//TERM is set as linux by pty which has an issue where vi editor screen does not get cleared.
	//Setting TERM as xterm-256color as used by standard terminals to fix this issue
	cmd.Env = append(os.Environ(),
		termEnvVariable,
	)

	ptyFile, err = pty.Start(cmd)
	if err != nil {
		return nil, nil, fmt.Errorf("Failed to start pty: %s\n", err)
	}

	return ptyFile, ptyFile, nil
}

//Stop closes pty file.
func Stop(log log.T) (err error) {
	log.Info("Stopping pty")
	if err := ptyFile.Close(); err != nil {
		return fmt.Errorf("unable to close ptyFile. %s", err)
	}
	return nil
}

//SetSize sets size of console terminal window.
func SetSize(log log.T, ws_col, ws_row uint32) (err error) {
	winSize := pty.Winsize{
		Cols: uint16(ws_col),
		Rows: uint16(ws_row),
	}

	if err := pty.Setsize(ptyFile, &winSize); err != nil {
		return fmt.Errorf("set pty size failed: %s", err)
	}
	return nil
}
