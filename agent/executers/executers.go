// Copyright 2016 Amazon.com, Inc. or its affiliates. All Rights Reserved.
//
// Licensed under the Amazon Software License (the "License"). You may not
// use this file except in compliance with the License. A copy of the
// License is located at
//
// http://aws.amazon.com/asl/
//
// or in the "license" file accompanying this file. This file is distributed
// on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND,
// express or implied. See the License for the specific language governing
// permissions and limitations under the License.

// Package executers contains general purpose (shell) command executing objects.
package executers

import (
	"bytes"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/aws/amazon-ssm-agent/agent/fileutil"
	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/amazon-ssm-agent/agent/task"
)

// NewDefaultShellExecuter creates a shell executer where the shell command is 'sh'.
func NewDefaultShellExecuter() *ShellCommandExecuter {
	return &ShellCommandExecuter{
		ShellCommand:          "sh",
		ShellDefaultArguments: []string{},
		ShellExitCodeTrap:     "", // currently this is not being used, however to capture $LASTEXITCODE from powershell we will probably need to make use of this.
		ScriptName:            "_script.sh",
		StdoutFileName:        "stdout",
		StderrFileName:        "stderr",
	}
}

// ShellCommandExecuter executes commands by invoking a shell (e.g. sh, bash, etc.)
type ShellCommandExecuter struct {
	ShellCommand          string
	ShellDefaultArguments []string
	ShellExitCodeTrap     string
	ScriptName            string
	StdoutFileName        string
	StderrFileName        string
}

// Execute executes a list of shell commands in the given working directory.
// The orchestration directory specifies where to create the script file and where
// to save stdout and stderr. The orchestration directory will be created if it doesn't exist.
// Returns readers for the standard output and standard error streams and a set of errors.
// The errors need not be fatal - the output streams may still have data
// even though some errors are reported. For example, if the command got killed while executing,
// the streams will have whatever data was printed up to the kill point, and the errors will
// indicate that the process got terminated.
func (sh ShellCommandExecuter) Execute(
	log log.T,
	workingDir string,
	scriptPath string,
	orchestrationDir string,
	cancelFlag task.CancelFlag,
	executionTimeout int,
) (stdout io.Reader, stderr io.Reader, exitCode int, errs []error) {

	stdoutFilePath := filepath.Join(orchestrationDir, sh.StdoutFileName)
	stderrFilePath := filepath.Join(orchestrationDir, sh.StderrFileName)
	log.Debugf("Command orchestration directory %v stdout file %v, stderr file %v", orchestrationDir, stdoutFilePath, stderrFilePath)

	var err error
	if sh.ShellCommand == "" {
		exitCode, err = runCommandOutputToFiles(log, cancelFlag, workingDir, stdoutFilePath, stderrFilePath, scriptPath, sh.ShellDefaultArguments, executionTimeout)
		if err != nil {
			errs = append(errs, err)
		}
	} else {
		exitCode, err = runCommandOutputToFiles(log, cancelFlag, workingDir, stdoutFilePath, stderrFilePath, sh.ShellCommand, sh.ShellDefaultArguments, executionTimeout, scriptPath)
		if err != nil {
			errs = append(errs, err)
		}
	}

	emptyReader := bytes.NewReader([]byte{})

	// create reader from stdout, if it exist, otherwise use empty reader
	if fileutil.Exists(stdoutFilePath) {
		stdout, err = os.Open(stdoutFilePath)
		if err != nil {
			// some unexpected error (file should exist)
			errs = append(errs, err)
		}
	} else {
		stdout = emptyReader
	}

	// create reader from stderr, if it exist, otherwise use empty reader
	if fileutil.Exists(stderrFilePath) {
		stderr, err = os.Open(stderrFilePath)
		if err != nil {
			// some unexpected error (file should exist)
			errs = append(errs, err)
		}
	} else {
		stderr = emptyReader
	}

	return
}

// CreateScriptFile creates a script containing the given commands.
func CreateScriptFile(scriptPath string, commands []string) (err error) {
	// create script
	file, err := os.Create(scriptPath)
	if err != nil {
		return
	}
	defer file.Close()

	// write commands
	_, err = file.WriteString(strings.Join(commands, "\n") + "\n")
	if err != nil {
		return
	}
	return
}

// runCommandOutputToFiles runs the given commands using the given working directory.
// The directory must exist. Standard output and standard error are sent to the given files.
func runCommandOutputToFiles(
	log log.T,
	cancelFlag task.CancelFlag,
	workingDir string,
	stdoutFilePath string,
	stderrFilePath string,
	commandName string,
	defaultArgs []string,
	executionTimeout int,
	args ...string,
) (exitCode int, err error) {

	// create stdout file
	// fix the permissions appropriately
	// Allow append so that if arrays of run command write to the same file, we keep appending to the file.
	stdoutWriter, err := os.OpenFile(stdoutFilePath, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0600)
	if err != nil {
		return
	}
	defer stdoutWriter.Close()

	// create stderr file
	// fix the permissions appropriately
	// Allow append so that if arrays of run command write to the same file, we keep appending to the file.
	stderrWriter, err := os.OpenFile(stderrFilePath, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0600)
	if err != nil {
		return
	}
	defer stderrWriter.Close()

	return RunCommand(log, cancelFlag, workingDir, stdoutWriter, stderrWriter, commandName, defaultArgs, executionTimeout, args...)
}

// RunCommand runs the given commands using the given working directory.
// Standard output and standard error are sent to the given writers.
func RunCommand(log log.T,
	cancelFlag task.CancelFlag,
	workingDir string,
	stdoutWriter io.Writer,
	stderrWriter io.Writer,
	commandName string,
	defaultArgs []string,
	executionTimeout int,
	args ...string,
) (exitCode int, err error) {

	// create command configuration
	args2 := append(defaultArgs, args...)
	command := exec.Command(commandName, args2...)
	command.Dir = workingDir
	command.Stdout = stdoutWriter
	command.Stderr = stderrWriter
	exitCode = 0

	// configure OS-specific process settings
	prepareProcess(command)

	log.Debug()
	log.Debugf("Running in directory %v, command: %v %v.", workingDir, commandName, args2)
	log.Debug()
	if err = command.Start(); err != nil {
		log.Error("error occurred starting the command", err)
		exitCode = 1
		return
	}

	go killProcessOnCancel(log, command, cancelFlag)

	timer := time.NewTimer(time.Duration(executionTimeout) * time.Second)
	go killProcessOnTimeout(log, command, timer)

	err = command.Wait()
	timedOut := !timer.Stop() // returns false if called previously - indicates timedOut.
	if err != nil {
		exitCode = 1
		log.Debugf("command failed to run %v", err)
		if exiterr, ok := err.(*exec.ExitError); ok {
			// The program has exited with an exit code != 0
			if status, ok := exiterr.Sys().(syscall.WaitStatus); ok {
				exitCode = status.ExitStatus()
				// First try to handle Cancel and Timeout scenarios
				// SIGKILL will result in an exitcode of -1
				if exitCode == -1 {
					if cancelFlag.Canceled() {
						// set appropriate exit code based on cancel or timeout
						exitCode = CommandStoppedPreemptivelyExitCode
						log.Infof("The execution of command was cancelled.")
					} else if timedOut {
						// set appropriate exit code based on cancel or timeout
						exitCode = CommandStoppedPreemptivelyExitCode
						log.Infof("The execution of command was timedout.")
					}
				} else {
					log.Infof("The execution of command returned Exit Status: %d", exitCode)
				}
			}
		}
	} else {
		// check if cancellation or timeout failed to kill the process
		// This will not occur as we do a SIGKILL, which is not recoverable.
		if cancelFlag.Canceled() {
			// This is when the cancellation failed and the command completed successfully
			log.Errorf("the cancellation failed to stop the process.")
			// do not return as the command could have been cancelled and also timedout
		}
		if timedOut {
			// This is when the timeout failed and the command completed successfully
			log.Errorf("the timeout failed to stop the process.")
		}
	}

	log.Debug("Done waiting!")
	return
}

// killProcessOnCancel waits for a cancel request.
// If a cancel request is received, this method kills the underlying
// process of the command. This will unblock the command.Wait() call.
// If the task completed successfully this method returns with no action.
func killProcessOnCancel(log log.T, command *exec.Cmd, cancelFlag task.CancelFlag) {
	cancelFlag.Wait()
	if cancelFlag.Canceled() {
		log.Debug("Process cancelled. Attempting to stop process.")

		// task has been asked to cancel, kill process
		if err := killProcess(command.Process); err != nil {
			log.Error(err)
			return
		}

		log.Debug("Process stopped successfully.")
	}
}

// killProcessOnTimeout waits for a timeout.
// When the timeout is reached, this method kills the underlying
// process of the command. This will unblock the command.Wait() call.
// If the task completed successfully this method returns with no action.
func killProcessOnTimeout(log log.T, command *exec.Cmd, timer *time.Timer) {
	<-timer.C
	log.Debug("Process exceeded timeout. Attempting to stop process.")

	// task has been exceeded the allowed execution timeout, kill process
	if err := killProcess(command.Process); err != nil {
		log.Error(err)
		return
	}

	log.Debug("Process stopped successfully")
}

// DeleteDirectory deletes a directory and all its content.
func DeleteDirectory(log log.T, dirName string) {
	if err := os.RemoveAll(dirName); err != nil {
		log.Error("error deleting directory", err)
	}
}
