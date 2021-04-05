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
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/aws/amazon-ssm-agent/agent/appconfig"
	agentContracts "github.com/aws/amazon-ssm-agent/agent/contracts"
	"github.com/aws/amazon-ssm-agent/agent/log"
	mgsConfig "github.com/aws/amazon-ssm-agent/agent/session/config"
	mgsContracts "github.com/aws/amazon-ssm-agent/agent/session/contracts"
	"github.com/aws/amazon-ssm-agent/agent/session/shell/constants"
	"github.com/aws/amazon-ssm-agent/agent/session/shell/execcmd"
	"github.com/aws/amazon-ssm-agent/agent/session/utility"
	"github.com/creack/pty"
	"github.com/google/shlex"
)

var ptyFile *os.File

const (
	termEnvVariable       = "TERM=xterm-256color"
	langEnvVariable       = "LANG=C.UTF-8"
	langEnvVariableKey    = "LANG"
	startRecordSessionCmd = "script"
	newLineCharacter      = "\n"
	catCmd                = "cat"
	scriptFlag            = "-c"
	groupsIdentifier      = "groups="
)

// StartCommandExecutor starts command execution in different behaviors based on plugin type.
// For Standard_Stream and InteractiveCommands plugins, StartCommandExecutor starts pty and provides handles to stdin and stdout.
// For NonInteractiveCommands plugin, StartCommandExecutor defines a command executor with native os.Exec, without assigning stdin.
// isSessionLogger determines whether its a customer shell or shell used for logging.
func StartCommandExecutor(
	log log.T,
	shellProps mgsContracts.ShellProperties,
	isSessionLogger bool,
	config agentContracts.Configuration,
	plugin *ShellPlugin) (err error) {

	log.Info("Starting command executor")
	//Start the command with a pty
	var cmd *exec.Cmd

	appConfig, _ := appconfig.Config(false)

	if strings.TrimSpace(constants.GetShellCommand(shellProps)) == "" || isSessionLogger {

		cmd = exec.Command("sh")

	} else {
		if appConfig.Agent.ContainerMode || appconfig.PluginNameNonInteractiveCommands == plugin.name {

			commands, err := shlex.Split(constants.GetShellCommand(shellProps))
			if err != nil {
				log.Errorf("Failed to parse commands input: %s\n", err)
				return fmt.Errorf("Failed to parse commands input: %s\n", err)
			}
			if len(commands) > 1 {
				cmd = exec.Command(commands[0], commands[1:]...)
			} else {
				cmd = exec.Command(commands[0])
			}

		} else {
			commandArgs := append(utility.ShellPluginCommandArgs, constants.GetShellCommand(shellProps))
			cmd = exec.Command("sh", commandArgs...)
		}
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

	var sessionUser string
	if !constants.GetRunAsElevated(shellProps) && !isSessionLogger && !appConfig.Agent.ContainerMode {
		// We get here only when its a customer shell that needs to be started in a specific user mode.

		u := &utility.SessionUtil{}
		if config.RunAsEnabled {
			if strings.TrimSpace(config.RunAsUser) == "" {
				return errors.New("please set the RunAs default user")
			}

			// Check if user exists
			if userExists, _ := u.DoesUserExist(config.RunAsUser); !userExists {
				// if user does not exist, fail the session
				return fmt.Errorf("failed to start pty since RunAs user %s does not exist", config.RunAsUser)
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
			return err
		}
		cmd.SysProcAttr = &syscall.SysProcAttr{}
		cmd.SysProcAttr.Credential = &syscall.Credential{Uid: uid, Gid: gid, Groups: groups, NoSetGroups: false}

		// Setting home environment variable for RunAs user
		runAsUserHomeEnvVariable := constants.HomeEnvVariable + sessionUser
		cmd.Env = append(cmd.Env, runAsUserHomeEnvVariable)
	}

	if constants.GetRunAsElevated(shellProps) {
		cmd.Env = append(cmd.Env, constants.RootHomeEnvVariable)
	}

	if appconfig.PluginNameNonInteractiveCommands == plugin.name {
		outputPath := filepath.Join(config.OrchestrationDirectory, mgsConfig.ExecOutputFileName)
		outputWriter, err := os.OpenFile(outputPath, appconfig.FileFlagsCreateOrAppendReadWrite, appconfig.ReadWriteAccess)
		if err != nil {
			return fmt.Errorf("Failed to open file for writing command output. error: %s\n", err)
		}
		outputReader, err := os.Open(outputPath)
		if err != nil {
			return fmt.Errorf("Failed to read command output from file %s. error: %s\n", outputPath, err)
		}
		cmd.Stdout = outputWriter
		cmd.Stderr = outputWriter
		plugin.stdin = nil
		plugin.stdout = outputReader
	} else {
		ptyFile, err = pty.Start(cmd)
		if err != nil {
			log.Errorf("Failed to start pty: %s\n", err)
			return fmt.Errorf("Failed to start pty: %s\n", err)
		}
		plugin.stdin = ptyFile
		plugin.stdout = ptyFile
	}
	plugin.runAsUser = sessionUser
	plugin.execCmd = execcmd.NewExecCmd(cmd)

	return nil
}

//stop closes pty file.
func (p *ShellPlugin) stop(log log.T) (err error) {
	if ptyFile == nil {
		return nil
	}
	log.Info("Stopping pty")
	if err := ptyFile.Close(); err != nil {
		if err, ok := err.(*os.PathError); ok && err.Err != os.ErrClosed {
			return fmt.Errorf("unable to close ptyFile. %s", err)
		}
	}
	return nil
}

//SetSize sets size of console terminal window.
func SetSize(log log.T, ws_col, ws_row uint32) (err error) {
	if ptyFile == nil {
		return nil
	}
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

// runShellProfile executes the shell profile config
func (p *ShellPlugin) runShellProfile(log log.T, config agentContracts.Configuration) error {
	if strings.TrimSpace(config.ShellProfile.Linux) == "" {
		return nil
	}
	if p.stdin == nil {
		return nil
	}
	if _, err := p.stdin.Write([]byte(config.ShellProfile.Linux + newLineCharacter)); err != nil {
		log.Errorf("Unable to write to stdin, err: %v.", err)
		return err
	}
	return nil
}

// generateLogData generates a log file with the executed commands.
func (p *ShellPlugin) generateLogData(log log.T, config agentContracts.Configuration) error {
	var flagStderr bytes.Buffer
	loggerCmd := fmt.Sprintf("%s %s", catCmd, p.logger.ipcFilePath)

	// Sixty minutes is the maximum amount of time before the command is cancelled
	// If a command is running this long it is most likely a stuck process
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Minute)
	defer cancel()

	cmdWithFlag := exec.CommandContext(ctx, startRecordSessionCmd, p.logger.logFilePath, scriptFlag, loggerCmd)
	cmdWithFlag.Stderr = &flagStderr
	flagErr := cmdWithFlag.Run()
	if flagErr != nil {
		log.Debugf("Failed to generate transcript with -c flag: %v: %s", flagErr, flagStderr.String())

		var noFlagStderr bytes.Buffer

		// some versions of "script" does not take a -c flag when passing in commands.
		cmdWithoutFlag := exec.CommandContext(ctx, startRecordSessionCmd, p.logger.logFilePath, catCmd, p.logger.ipcFilePath)
		cmdWithoutFlag.Stderr = &noFlagStderr
		noFlagErr := cmdWithoutFlag.Run()
		if noFlagErr != nil {
			log.Debugf("Failed to generate transcript without -c flag: %v: %s", noFlagErr, noFlagStderr.String())
			return fmt.Errorf("Failed to generate transcript with the following errors:\n%v: %s\n%v:%s", flagErr, flagStderr.String(), noFlagErr, noFlagStderr.String())
		}
	}

	return nil
}

// isLogStreamingSupported checks if streaming of logs is supported since it depends on PowerShell's transcript logging
func (p *ShellPlugin) isLogStreamingSupported(log log.T) (logStreamingSupported bool, err error) {
	if appconfig.PluginNameNonInteractiveCommands == p.name {
		return false, nil
	}
	return true, nil
}

// getStreamingFilePath returns the file path of ipcFile for streaming
func (p *ShellPlugin) getStreamingFilePath(log log.T) (streamingFilePath string, err error) {
	return p.logger.ipcFilePath, nil
}

// isCleanupOfControlCharactersRequired returns true/false depending on whether log needs to be cleanup of control characters before streaming to destination
func (p *ShellPlugin) isCleanupOfControlCharactersRequired() bool {
	// Source of logs for linux platform is directly from shell stdout which contains control characters and need cleanup
	return true
}

// checkForLoggingInterruption is used to detect if log streaming to CW has been interrupted
var checkForLoggingInterruption = func(log log.T, ipcFile *os.File, plugin *ShellPlugin) {
	// Enable append only mode for the ipcTempFile to protect it from being modified
	u := &utility.SessionUtil{}
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	if err := u.SetAttr(ipcFile, utility.FS_APPEND_FL); err != nil {
		log.Debugf("Unable to set FS_APPEND_FL flag, %v", err)
		// Periodically check if ipcTempFile is missing
		for range ticker.C {
			if _, err := os.Stat(ipcFile.Name()); os.IsNotExist(err) {
				log.Warn("Local temp log file is missing, logging might be interrupted.")
				break
			}
		}
	} else {
		// Periodically check if ipcTempFile's append only attribute has been modified
		for range ticker.C {
			if attr, err := u.GetAttr(ipcFile); err != nil {
				log.Warnf("Unable to get attributes of local temp log file, logging might be interrupted. Err: %v", err)
				break
			} else if attr != utility.FS_APPEND_FL {
				log.Warn("Append only attribute of local temp log file has been modified, logging might be interrupted.")
				break
			}
		}
	}
}

//cleanupLogFile prepares temporary files for the cleanup
func (p *ShellPlugin) cleanupLogFile(log log.T, ipcFile *os.File) {
	// remove file property so deletion of the file can be done successfully
	u := &utility.SessionUtil{}
	if err := u.SetAttr(ipcFile, utility.FS_RESET_FL); err != nil {
		log.Debugf("Unable to reset file properties, %v", err)
	}
}

// InputStreamMessageHandler passes payload byte stream to shell stdin
func (p *ShellPlugin) InputStreamMessageHandler(log log.T, streamDataMessage mgsContracts.AgentMessage) error {
	var isPluginNonInteractive = appconfig.PluginNameNonInteractiveCommands == p.name
	if !isPluginNonInteractive && (p.stdin == nil || p.stdout == nil) {
		// This is to handle scenario when cli/console starts sending size data but pty has not been started yet
		// Since packets are rejected, cli/console will resend these packets until pty starts successfully in separate thread
		log.Tracef("Pty unavailable. Reject incoming message packet")
		return mgsContracts.ErrHandlerNotReady
	}

	switch mgsContracts.PayloadType(streamDataMessage.PayloadType) {
	case mgsContracts.Output:
		log.Tracef("Output message received: %d", streamDataMessage.SequenceNumber)
		if isPluginNonInteractive {
			var signal os.Signal = nil
			for _, message := range streamDataMessage.Payload {
				if sig, exists := appconfig.ByteControlSignalsLinux[message]; exists {
					log.Debugf("Received control signal. message: %v, signal: %v", string(message), sig)
					signal = sig
					break
				}
			}
			if signal != nil {
				defer func() {
					if err := p.execCmd.Wait(); err != nil {
						log.Errorf("Error received after processing control signal: %s", err)
					}
				}()
				if err := p.execCmd.Signal(signal); err != nil {
					log.Errorf("Sending signal %v to command process %v failed with error %v", signal, p.execCmd.Pid(), err)
					return err
				}
			}
			return nil
		}
		if _, err := p.stdin.Write(streamDataMessage.Payload); err != nil {
			log.Errorf("Unable to write to stdin, err: %v.", err)
			return err
		}
	case mgsContracts.Size:
		// Do not handle terminal resize for non-interactive plugin as there is no pty
		if isPluginNonInteractive {
			log.Debug("Terminal resize message is ignored in NonInteractiveCommands plugin")
			return nil
		}
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
