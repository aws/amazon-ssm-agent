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
	"bytes"
	"context"
	"encoding/json"
	"errors"
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
	mgsContracts "github.com/aws/amazon-ssm-agent/agent/session/contracts"
	"github.com/aws/amazon-ssm-agent/agent/session/utility"
	"github.com/kr/pty"
)

var ptyFile *os.File

const (
	termEnvVariable       = "TERM=xterm-256color"
	langEnvVariable       = "LANG=C.UTF-8"
	langEnvVariableKey    = "LANG"
	startRecordSessionCmd = "script"
	catCmd                = "cat"
	scriptFlag            = "-c"
	homeEnvVariable       = "HOME=/home/"
	groupsIdentifier      = "groups="
)

//StartPty starts pty and provides handles to stdin and stdout
// isSessionLogger determines whether its a customer shell or shell used for logging.
func StartPty(
	log log.T,
	shellProps mgsContracts.ShellProperties,
	isSessionLogger bool,
	config agentContracts.Configuration) (stdin *os.File, stdout *os.File, err error) {
	log.Info("Starting pty")
	//Start the command with a pty
	var cmd *exec.Cmd
	if strings.TrimSpace(shellProps.Linux.Commands) == "" || isSessionLogger {
		cmd = exec.Command("sh")
	} else {
		commandArgs := append(utility.ShellPluginCommandArgs, shellProps.Linux.Commands)
		cmd = exec.Command("sh", commandArgs...)
	}

	//TERM is set as linux by pty which has an issue where vi editor screen does not get cleared.
	//Setting TERM as xterm-256color as used by standard terminals to fix this issue
	cmd.Env = append(os.Environ(), termEnvVariable)

	//If LANG environment variable is not set, shell defaults to POSIX which can contain 256 single-byte characters.
	//Setting C.UTF-8 as default LANG environment variable as Session Manager supports UTF-8 encoding only.
	langEnvVariableValue := os.Getenv(langEnvVariableKey)
	if langEnvVariableValue == "" {
		cmd.Env = append(cmd.Env, langEnvVariable)
	}

	appConfig, _ := appconfig.Config(false)

	if !shellProps.Linux.RunAsElevated && !isSessionLogger && !appConfig.Agent.ContainerMode {
		// We get here only when its a customer shell that needs to be started in a specific user mode.

		var sessionUser string
		u := &utility.SessionUtil{}
		if config.RunAsEnabled {
			if strings.TrimSpace(config.RunAsUser) == "" {
				return nil, nil, errors.New("please set the RunAs default user")
			}

			// Check if user exists
			if userExists, _ := u.DoesUserExist(config.RunAsUser); !userExists {
				// if user does not exist, fail the session
				return nil, nil, fmt.Errorf("failed to start pty since RunAs user %s does not exist", config.RunAsUser)
			}

			sessionUser = config.RunAsUser
		} else {
			// Start as ssm-user
			// Create ssm-user before starting a session.
			u.CreateLocalAdminUser(log)

			sessionUser = appconfig.DefaultRunAsUserName
		}

		// Get the uid and gid of the runas user.
		uid, gid, groups, err := getUserCredentials(log, sessionUser)
		if err != nil {
			return nil, nil, err
		}
		cmd.SysProcAttr = &syscall.SysProcAttr{}
		cmd.SysProcAttr.Credential = &syscall.Credential{Uid: uid, Gid: gid, Groups: groups, NoSetGroups: false}

		// Setting home environment variable for RunAs user
		runAsUserHomeEnvVariable := homeEnvVariable + sessionUser
		cmd.Env = append(cmd.Env, runAsUserHomeEnvVariable)
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

// getUserCredentials returns the uid, gid and groups associated to the runas user.
func getUserCredentials(log log.T, sessionUser string) (uint32, uint32, []uint32, error) {
	uidCmdArgs := append(utility.ShellPluginCommandArgs, fmt.Sprintf("id -u %s", sessionUser))
	cmd := exec.Command(utility.ShellPluginCommandName, uidCmdArgs...)
	out, err := cmd.Output()
	if err != nil {
		log.Errorf("Failed to retrieve uid for %s: %v", sessionUser, err)
		return 0, 0, nil, err
	}

	uid, err := strconv.Atoi(strings.TrimSpace(string(out)))
	if err != nil {
		log.Errorf("%s not found: %v", sessionUser, err)
		return 0, 0, nil, err
	}

	gidCmdArgs := append(utility.ShellPluginCommandArgs, fmt.Sprintf("id -g %s", sessionUser))
	cmd = exec.Command(utility.ShellPluginCommandName, gidCmdArgs...)
	out, err = cmd.Output()
	if err != nil {
		log.Errorf("Failed to retrieve gid for %s: %v", sessionUser, err)
		return 0, 0, nil, err
	}

	gid, err := strconv.Atoi(strings.TrimSpace(string(out)))
	if err != nil {
		log.Errorf("%s not found: %v", sessionUser, err)
		return 0, 0, nil, err
	}

	// Get the list of associated groups
	groupNamesCmdArgs := append(utility.ShellPluginCommandArgs, fmt.Sprintf("id %s", sessionUser))
	cmd = exec.Command(utility.ShellPluginCommandName, groupNamesCmdArgs...)
	out, err = cmd.Output()
	if err != nil {
		log.Errorf("Failed to retrieve groups for %s: %v", sessionUser, err)
		return 0, 0, nil, err
	}

	// Example format of output: uid=1873601143(ssm-user) gid=1873600513(domain users) groups=1873600513(domain users),1873601620(joiners),1873601125(aws delegated add workstations to domain users)
	// Extract groups from the output
	groupsIndex := strings.Index(string(out), groupsIdentifier)
	var groupIds []uint32

	if groupsIndex > 0 {
		// Extract groups names and ids from the output
		groupNamesAndIds := strings.Split(string(out)[groupsIndex+len(groupsIdentifier):], ",")

		// Extract group ids from the output
		for _, value := range groupNamesAndIds {
			groupId, err := strconv.Atoi(strings.TrimSpace(value[:strings.Index(value, "(")]))
			if err != nil {
				log.Errorf("Failed to retrieve group id from %s: %v", value, err)
				return 0, 0, nil, err
			}

			groupIds = append(groupIds, uint32(groupId))
		}
	}

	// Make sure they are non-zero valid positive ids
	if uid > 0 && gid > 0 {
		return uint32(uid), uint32(gid), groupIds, nil
	}

	return 0, 0, nil, errors.New("invalid uid and gid")
}

// generateLogData generates a log file with the executed commands.
func (p *ShellPlugin) generateLogData(log log.T, config agentContracts.Configuration) error {
	var flagStderr bytes.Buffer
	loggerCmd := fmt.Sprintf("%s %s", catCmd, p.ipcFilePath)

	// Sixty minutes is the maximum amount of time before the command is cancelled
	// If a command is running this long it is most likely a stuck process
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Minute)
	defer cancel()

	cmdWithFlag := exec.CommandContext(ctx, startRecordSessionCmd, p.logFilePath, scriptFlag, loggerCmd)
	cmdWithFlag.Stderr = &flagStderr
	flagErr := cmdWithFlag.Run()
	if flagErr != nil {
		log.Debugf("Failed to generate transcript with -c flag: %v: %s", flagErr, flagStderr.String())

		var noFlagStderr bytes.Buffer

		// some versions of "script" does not take a -c flag when passing in commands.
		cmdWithoutFlag := exec.CommandContext(ctx, startRecordSessionCmd, p.logFilePath, catCmd, p.ipcFilePath)
		cmdWithoutFlag.Stderr = &noFlagStderr
		noFlagErr := cmdWithoutFlag.Run()
		if noFlagErr != nil {
			log.Debugf("Failed to generate transcript without -c flag: %v: %s", noFlagErr, noFlagStderr.String())
			return errors.New(fmt.Sprintf("Failed to generate transcript with the following errors:\n%v: %s\n%v:%s", flagErr, flagStderr.String(), noFlagErr, noFlagStderr.String()))
		}
	}

	return nil
}

// InputStreamMessageHandler passes payload byte stream to shell stdin
func (p *ShellPlugin) InputStreamMessageHandler(log log.T, streamDataMessage mgsContracts.AgentMessage) error {
	if p.stdin == nil || p.stdout == nil {
		// This is to handle scenario when cli/console starts sending size data but pty has not been started yet
		// Since packets are rejected, cli/console will resend these packets until pty starts successfully in separate thread
		log.Tracef("Pty unavailable. Reject incoming message packet")
		return nil
	}

	switch mgsContracts.PayloadType(streamDataMessage.PayloadType) {
	case mgsContracts.Output:
		log.Tracef("Output message received: %d", streamDataMessage.SequenceNumber)
		if _, err := p.stdin.Write(streamDataMessage.Payload); err != nil {
			log.Errorf("Unable to write to stdin, err: %v.", err)
			return err
		}
	case mgsContracts.Size:
		var size mgsContracts.SizeData
		if err := json.Unmarshal(streamDataMessage.Payload, &size); err != nil {
			log.Errorf("Invalid size message: %s", err)
			return err
		}
		log.Tracef("Resize data received: cols: %d, rows: %d", size.Cols, size.Rows)
		if err := SetSize(log, size.Cols, size.Rows); err != nil {
			log.Errorf("Unable to set pty size: %s", err)
			return err
		}
	}
	return nil
}
