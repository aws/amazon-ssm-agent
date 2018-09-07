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
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/aws/amazon-ssm-agent/agent/appconfig"
	agentContracts "github.com/aws/amazon-ssm-agent/agent/contracts"
	"github.com/aws/amazon-ssm-agent/agent/log"
	mgsConfig "github.com/aws/amazon-ssm-agent/agent/session/config"
	"github.com/kr/pty"
)

var ptyFile *os.File

const (
	termEnvVariable       = "TERM=xterm-256color"
	startRecordSessionCmd = "script"
	newLineCharacter      = "\n"
	screenBufferSizeCmd   = "screen -h %d%s"
	homeEnvVariable       = "HOME=/home/" + appconfig.DefaultRunAsUserName
)

var getUserAndGroupIdCall = func(log log.T) (uid int, gid int, err error) {
	return getUserAndGroupId(log)
}

//StartPty starts pty and provides handles to stdin and stdout
func StartPty(log log.T, isSessionShell bool) (stdin *os.File, stdout *os.File, err error) {
	log.Info("Starting pty")
	//Start the command with a pty
	cmd := exec.Command("sh")

	//TERM is set as linux by pty which has an issue where vi editor screen does not get cleared.
	//Setting TERM as xterm-256color as used by standard terminals to fix this issue
	cmd.Env = append(os.Environ(),
		termEnvVariable,
		homeEnvVariable,
	)

	// Get the uid and gid of the runas user.
	if isSessionShell {
		log.Info("Starting pty")
		uid, gid, err := getUserAndGroupIdCall(log)
		if err != nil {
			return nil, nil, err
		}
		cmd.SysProcAttr = &syscall.SysProcAttr{}
		cmd.SysProcAttr.Credential = &syscall.Credential{Uid: uint32(uid), Gid: uint32(gid)}
	}

	ptyFile, err = pty.Start(cmd)
	if err != nil {
		log.Errorf("Failed to start pty: %s\n", err)
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

//getUserAndGroupId returns the uid and gid of the runas user.
func getUserAndGroupId(log log.T) (uid int, gid int, err error) {
	shellCmdArgs := append(ShellPluginCommandArgs, fmt.Sprintf("id -u %s", appconfig.DefaultRunAsUserName))
	cmd := exec.Command(ShellPluginCommandName, shellCmdArgs...)
	out, err := cmd.Output()
	if err != nil {
		log.Errorf("Failed retrieve uid for %s: %v", appconfig.DefaultRunAsUserName, err)
		return
	}

	u, err := strconv.Atoi(strings.TrimSpace(string(out)))
	if err != nil {
		log.Errorf("%s not found: %v", appconfig.DefaultRunAsUserName, err)
	}

	shellCmdArgs = append(ShellPluginCommandArgs, fmt.Sprintf("id -g %s", appconfig.DefaultRunAsUserName))
	cmd = exec.Command(ShellPluginCommandName, shellCmdArgs...)
	out, err = cmd.Output()
	if err != nil {
		log.Errorf("Failed retrieve gid for %s: %v", appconfig.DefaultRunAsUserName, err)
		return
	}

	g, err := strconv.Atoi(strings.TrimSpace(string(out)))
	if err != nil {
		log.Errorf("%s not found: %v", appconfig.DefaultRunAsUserName, err)
	}

	// Make sure they are non-zero valid positive ids
	if u > 0 && g > 0 {
		return u, g, nil
	}
	return
}

// generateLogData generates a log file with the executed commands.
func (p *ShellPlugin) generateLogData(log log.T, config agentContracts.Configuration) error {
	shadowShellInput, _, err := StartPty(log, false)
	if err != nil {
		return err
	}

	defer func() {
		if err := recover(); err != nil {
			if err = Stop(log); err != nil {
				log.Errorf("Error occured while closing pty: %v", err)
			}
		}
	}()

	time.Sleep(5 * time.Second)

	// Increase buffer size
	screenBufferSizeCmdInput := fmt.Sprintf(screenBufferSizeCmd, mgsConfig.ScreenBufferSize, newLineCharacter)
	shadowShellInput.Write([]byte(screenBufferSizeCmdInput))

	time.Sleep(5 * time.Second)

	// Start shell recording
	recordCmdInput := fmt.Sprintf("%s %s%s", startRecordSessionCmd, p.logFilePath, newLineCharacter)
	shadowShellInput.Write([]byte(recordCmdInput))

	time.Sleep(5 * time.Second)

	// Start shell logger
	loggerCmdInput := fmt.Sprintf("%s %s%s", appconfig.DefaultSessionLogger, p.ipcFilePath, newLineCharacter)
	shadowShellInput.Write([]byte(loggerCmdInput))

	// Sleep till the logger completes execution
	time.Sleep(time.Minute)

	exitCmdInput := fmt.Sprintf("%s%s", mgsConfig.Exit, newLineCharacter)

	// Exit start record command
	shadowShellInput.Write([]byte(exitCmdInput))

	// Sleep until start record command is exited successfully
	time.Sleep(30 * time.Second)

	// Exit screen buffer command
	shadowShellInput.Write([]byte(exitCmdInput))

	// Sleep till screen buffer command is exited successfully
	time.Sleep(5 * time.Second)

	// Exit shell
	shadowShellInput.Write([]byte(exitCmdInput))

	// Sleep till shell is exited successfully
	time.Sleep(5 * time.Second)

	// Close pty
	shadowShellInput.Close()

	// Sleep till the shell successfully exits before uploading
	time.Sleep(15 * time.Second)

	return nil
}
