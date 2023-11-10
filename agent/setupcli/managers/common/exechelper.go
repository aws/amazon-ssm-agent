// Copyright 2023 Amazon.com, Inc. or its affiliates. All Rights Reserved.
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

// Package common contains common constants and functions needed to be accessed across ssm-setup-cli
package common

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime/debug"
	"strings"
	"syscall"
	"time"

	"github.com/aws/amazon-ssm-agent/agent/appconfig"
	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/amazon-ssm-agent/agent/updateutil/updateconstants"
)

// defaultTimeout states the default timeout for command
const defaultTimeout = 30 * time.Second

var (
	execLookPath = exec.LookPath
)

// IManagerHelper is the interface containing functions related to command executions on the system
type IManagerHelper interface {
	// RunCommand executes command with timeout
	RunCommand(cmd string, args ...string) (string, error)

	// RunCommandWithCustomTimeout executes command with a custom timeout
	RunCommandWithCustomTimeout(timeout time.Duration, cmd string, args ...string) (string, error)

	// ExecCommandWithOutput executes command with output and error content returned
	ExecCommandWithOutput(
		log log.T,
		cmd string,
		workingDir string,
		outputRoot string,
		stdOut string,
		stdErr string) (pId int, updExitCode updateconstants.UpdateScriptExitCode, stdoutBytes *bytes.Buffer, errorBytes *bytes.Buffer, cmdErr error)

	// IsCommandAvailable checks if command can be executed on host
	IsCommandAvailable(cmd string) bool

	// IsTimeoutError returns true if error is context timeout error
	IsTimeoutError(err error) bool

	// IsExitCodeError returns true if error is command exit code error
	IsExitCodeError(err error) bool

	// GetExitCode returns the exit code for of exit code error, defaults to -1 if error is not exit code error
	GetExitCode(err error) int
}

// ManagerHelper implements IManagerHelper
type ManagerHelper struct {
	Timeout time.Duration
}

// RunCommand is used to execute commands with a default timeout set to 30 secs
func (m *ManagerHelper) RunCommand(cmd string, args ...string) (string, error) {
	if m.Timeout == time.Duration(0) {
		m.Timeout = defaultTimeout
	}
	return m.RunCommandWithCustomTimeout(m.Timeout, cmd, args...)
}

// RunCommandWithCustomTimeout is used to execute commands with timeout accepted as argument
func (m *ManagerHelper) RunCommandWithCustomTimeout(timout time.Duration, cmd string, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timout)
	defer cancel()

	byteArr, err := exec.CommandContext(ctx, cmd, args...).CombinedOutput()
	output := strings.TrimSpace(string(byteArr))

	return output, err
}

// IsTimeoutError returns true if error is context timeout error
func (m *ManagerHelper) IsTimeoutError(err error) bool {
	if err == nil {
		return false
	}
	return errors.Is(err, context.DeadlineExceeded)
}

// IsExitCodeError returns true if error is command exit code error
func (m *ManagerHelper) IsExitCodeError(err error) bool {
	if err == nil {
		return false
	}
	_, ok := err.(*exec.ExitError)
	return ok
}

// GetExitCode returns the exit code for of exit code error, defaults to -1 if error is not exit code error
func (m *ManagerHelper) GetExitCode(err error) int {
	if !m.IsExitCodeError(err) {
		return -1
	}

	return err.(*exec.ExitError).ExitCode()
}

// IsCommandAvailable checks if command can be executed on host
func (m *ManagerHelper) IsCommandAvailable(cmd string) bool {
	_, err := execLookPath(cmd)
	return err == nil
}

// ExecCommandWithOutput executes shell command and returns output and error of command execution
func (m *ManagerHelper) ExecCommandWithOutput(
	log log.T,
	cmd string,
	workingDir string,
	outputRoot string,
	stdOut string,
	stdErr string) (pId int, updExitCode updateconstants.UpdateScriptExitCode, stdoutBytes *bytes.Buffer, errorBytes *bytes.Buffer, cmdErr error) {

	parts := strings.Fields(cmd)
	pid := -1
	var updateExitCode updateconstants.UpdateScriptExitCode = -1
	tempCmd := setPlatformSpecificCommand(parts)
	command := exec.Command(tempCmd[0], tempCmd[1:]...)
	command.Dir = workingDir
	stdoutWriter, stderrWriter, err := setExeOutErr(outputRoot, stdOut, stdErr)
	if err != nil {
		return pid, updateExitCode, nil, nil, err
	}
	defer stdoutWriter.Close()
	defer stderrWriter.Close()
	var errBytes, stdOutBytes bytes.Buffer
	command.Stdout = &stdOutBytes
	command.Stderr = &errBytes

	var cmdStart = (*exec.Cmd).Start
	err = cmdStart(command)
	if err != nil {
		return pid, updateExitCode, &stdOutBytes, &errBytes, err
	}

	pid = getProcessPid(command)

	var timeout = updateconstants.DefaultUpdateExecutionTimeoutInSeconds
	timer := time.NewTimer(time.Duration(timeout) * time.Second)
	go killProcessOnTimeout(log, command, timer)
	err = command.Wait()
	if errBytes.Len() != 0 {
		stderrWriter.Write(errBytes.Bytes())
	}
	if stdOutBytes.Len() != 0 {
		stdoutWriter.Write(stdOutBytes.Bytes())
	}
	timedOut := !timer.Stop()
	if err != nil {
		log.Debugf("command returned error %v", err)
		if exitErr, ok := err.(*exec.ExitError); ok {
			// The program has exited with an exit code != 0
			if status, ok := exitErr.Sys().(syscall.WaitStatus); ok {
				exitCode := status.ExitStatus()
				if exitCode == -1 && timedOut {
					// set appropriate exit code based on cancel or timeout
					exitCode = appconfig.CommandStoppedPreemptivelyExitCode
					log.Infof("The execution of command was timedout.")
				}
				err = fmt.Errorf("The execution of command returned Exit Status: %d \n %v", exitCode, err.Error())
			}
		}
		return pid, updateExitCode, &stdOutBytes, &errBytes, err
	}
	return pid, updateExitCode, &stdOutBytes, nil, nil
}

func killProcessOnTimeout(log log.T, command *exec.Cmd, timer *time.Timer) {
	defer func() {
		if r := recover(); r != nil {
			log.Errorf("Kill process on timeout panic: \n%v", r)
			log.Errorf("Stacktrace:\n%s", debug.Stack())
		}
	}()
	<-timer.C
	log.Debug("Process exceeded timeout. Attempting to kill process!")

	// task has been exceeded the allowed execution timeout, kill process
	if err := command.Process.Kill(); err != nil {
		log.Error(err)
		return
	}
	log.Debug("Process killed!")
}

// setExeOutErr creates stderr and stdout file
func setExeOutErr(
	updaterRoot string,
	stdOutFileName string,
	stdErrFileName string) (stdoutWriter *os.File, stderrWriter *os.File, err error) {

	if err = os.MkdirAll(updaterRoot, appconfig.ReadWriteExecuteAccess); err != nil {
		return
	}

	stdOutPath := updateStdOutPath(updaterRoot, stdOutFileName)
	stdErrPath := updateStdErrPath(updaterRoot, stdErrFileName)

	// create stdout file
	// Allow append so that if arrays of run command write to the same file, we keep appending to the file.
	if stdoutWriter, err = os.OpenFile(stdOutPath, appconfig.FileFlagsCreateOrAppend, appconfig.ReadWriteAccess); err != nil {
		return
	}

	// create stderr file
	// Allow append so that if arrays of run command write to the same file, we keep appending to the file.
	if stderrWriter, err = os.OpenFile(stdErrPath, appconfig.FileFlagsCreateOrAppend, appconfig.ReadWriteAccess); err != nil {
		return
	}

	return stdoutWriter, stderrWriter, nil
}

// updateStdOutPath returns stand output file path
func updateStdOutPath(updateRoot string, fileName string) string {
	if fileName == "" {
		fileName = updateconstants.DefaultStandOut
	}
	return filepath.Join(updateRoot, fileName)
}

// updateStdErrPath returns stand error file path
func updateStdErrPath(updateRoot string, fileName string) string {
	if fileName == "" {
		fileName = updateconstants.DefaultStandErr
	}
	return filepath.Join(updateRoot, fileName)
}

// getProcessPid returns the pid of the process if present, defaults to pid -1
func getProcessPid(cmd *exec.Cmd) int {
	if cmd != nil && cmd.Process != nil {
		return cmd.Process.Pid
	}
	return -1
}
