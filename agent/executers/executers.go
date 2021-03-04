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

// Package executers contains general purpose (shell) command executing objects.
package executers

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"runtime"
	"runtime/debug"
	"strings"
	"syscall"
	"time"

	"github.com/aws/amazon-ssm-agent/agent/appconfig"
	"github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/aws/amazon-ssm-agent/agent/fileutil"
	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/amazon-ssm-agent/agent/platform"
	"github.com/aws/amazon-ssm-agent/agent/task"
)

const (
	// envVar* constants are names of environment variables set for processes executed by ssm agent and should start with AWS_SSM_
	envVarInstanceID      = "AWS_SSM_INSTANCE_ID"
	envVarRegionName      = "AWS_SSM_REGION_NAME"
	envVarPlatformName    = "AWS_SSM_PLATFORM_NAME"
	envVarPlatformVersion = "AWS_SSM_PLATFORM_VERSION"
)

// T is the interface type for ShellCommandExecuter.
type T interface {
	//TODO: Remove Execute and rename NewExecute to Execute.
	Execute(context.T, string, string, string, task.CancelFlag, int, string, []string, map[string]string) (io.Reader, io.Reader, int, []error)
	NewExecute(context.T, string, io.Writer, io.Writer, task.CancelFlag, int, string, []string, map[string]string) (int, error)
	StartExe(context.T, string, io.Writer, io.Writer, task.CancelFlag, string, []string) (*os.Process, int, error)
}

// ShellCommandExecuter is specially added for testing purposes
type ShellCommandExecuter struct {
}

type timeoutSignal struct {
	// process kill doesn't send proper signal to the process status
	// Setting the execInterruptedOnWindows to indicate execution was interrupted
	execInterruptedOnWindows bool
}

// Execute executes a list of shell commands in the given working directory.
// If no file path is provided for either stdout or stderr, output will be written to a byte buffer.
// Returns readers for the standard output and standard error streams, process exit code, and a set of errors.
// The errors need not be fatal - the output streams may still have data
// even though some errors are reported. For example, if the command got killed while executing,
// the streams will have whatever data was printed up to the kill point, and the errors will
// indicate that the process got terminated.
//
// For files, the reader returned will not contain more than appconfig.MaxStdoutLength and appconfig.MaxStderrLength respectively
// so if the caller needs to process more output than that, it should open its own reader on the output files.
//
// For byte buffer output, the reader will be a reader over the buffer, which will accumulate the entire output.  Be careful
// not to use the byte buffer approach for extremely large output (or unknown output) because it could take up a large amount
// of memory.
func (ShellCommandExecuter) Execute(
	context context.T,
	workingDir string,
	stdoutFilePath string,
	stderrFilePath string,
	cancelFlag task.CancelFlag,
	executionTimeout int,
	commandName string,
	commandArguments []string,
	envVars map[string]string,
) (stdout io.Reader, stderr io.Reader, exitCode int, errs []error) {

	var stdoutWriter io.Writer
	var stdoutBuf *bytes.Buffer
	if stdoutFilePath != "" {
		// create stdout file
		// fix the permissions appropriately
		// Allow append so that if arrays of run command write to the same file, we keep appending to the file.
		stdoutFileWriter, err := os.OpenFile(stdoutFilePath, appconfig.FileFlagsCreateOrAppend, appconfig.ReadWriteAccess)
		if err != nil {
			return
		}
		stdoutWriter = stdoutFileWriter
		defer stdoutFileWriter.Close()
	} else {
		stdoutBuf = bytes.NewBuffer(nil)
		stdoutWriter = stdoutBuf
	}

	var stderrWriter io.Writer
	var stderrBuf *bytes.Buffer
	if stderrFilePath != "" {
		// create stderr file
		// fix the permissions appropriately
		// Allow append so that if arrays of run command write to the same file, we keep appending to the file.
		stderrFileWriter, err := os.OpenFile(stderrFilePath, appconfig.FileFlagsCreateOrAppend, appconfig.ReadWriteAccess)
		if err != nil {
			return
		}
		stderrWriter = stderrFileWriter
		defer stderrFileWriter.Close() // ExecuteCommand creates a copy of the handle
	} else {
		stderrBuf = bytes.NewBuffer(nil)
		stderrWriter = stderrBuf
	}

	// NOTE: Regarding the defer close of the file writers.
	// Technically, closing the files should happen after ExecuteCommand and before opening the files for reading.
	// In this case, there is no need for that because the child process inherits copies of the file handles and does
	// the actual writing to the files. So, when using files, it does not matter when we close our copies of the file
	// writers as long as it is after the process starts.

	var err error
	exitCode, err = ExecuteCommand(context, cancelFlag, workingDir, stdoutWriter, stderrWriter, executionTimeout, commandName, commandArguments, envVars)
	if err != nil {
		errs = append(errs, err)
	}

	// create reader from stdout, if it exist, otherwise use the buffer
	if fileutil.Exists(stdoutFilePath) {
		stdoutReader, err := os.Open(stdoutFilePath)
		if err != nil {
			// some unexpected error (file should exist)
			errs = append(errs, err)
		}
		defer stdoutReader.Close()
		stdoutString, _ := ioutil.ReadAll(io.LimitReader(stdoutReader, appconfig.MaxStdoutLength))
		stdout = bytes.NewReader([]byte(stdoutString))
	} else {
		stdout = bytes.NewReader(stdoutBuf.Bytes())
	}

	// create reader from stderr, if it exist, otherwise use the buffer
	if fileutil.Exists(stderrFilePath) {
		stderrReader, err := os.Open(stderrFilePath)
		if err != nil {
			// some unexpected error (file should exist)
			errs = append(errs, err)
		}
		defer stderrReader.Close()
		stderrString, _ := ioutil.ReadAll(io.LimitReader(stderrReader, appconfig.MaxStderrLength))
		stderr = bytes.NewReader([]byte(stderrString))
	} else {
		stderr = bytes.NewReader(stderrBuf.Bytes())
	}

	return
}

// NewExecute executes a list of shell commands in the given working directory and provides the stdout and stderr writers.
func (ShellCommandExecuter) NewExecute(
	context context.T,
	workingDir string,
	stdoutWriter io.Writer,
	stderrWriter io.Writer,
	cancelFlag task.CancelFlag,
	executionTimeout int,
	commandName string,
	commandArguments []string,
	envVars map[string]string,
) (exitCode int, err error) {
	exitCode, err = ExecuteCommand(context, cancelFlag, workingDir, stdoutWriter, stderrWriter, executionTimeout, commandName, commandArguments, envVars)
	return
}

// StartExe starts a list of shell commands in the given working directory.
// Returns process started, an exit code (0 if successfully launch, 1 if error launching process), and a set of errors.
// The errors need not be fatal - the output streams may still have data
// even though some errors are reported. For example, if the command got killed while executing,
// the streams will have whatever data was printed up to the kill point, and the errors will
// indicate that the process got terminated.
func (ShellCommandExecuter) StartExe(
	context context.T,
	workingDir string,
	stdoutWriter io.Writer,
	stderrWriter io.Writer,
	cancelFlag task.CancelFlag,
	commandName string,
	commandArguments []string,
) (process *os.Process, exitCode int, err error) {
	process, exitCode, err = StartCommand(context, cancelFlag, workingDir, stdoutWriter, stderrWriter, commandName, commandArguments)
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

// Wrapper around a writer (such as a file) that provides a way to stop writing to the file
// This allows us to disconnect the writer on cancel or timeout even if the process is running and writing a large volume of output
type cancellableWriter struct {
	baseWriter    io.Writer
	cancelChannel chan bool
	cancelled     bool
}

func newWriter(writer io.Writer) (*cancellableWriter, chan bool) {
	newCancelChannel := make(chan bool, 1)
	return &cancellableWriter{
		baseWriter:    writer,
		cancelChannel: newCancelChannel,
	}, newCancelChannel
}

func (r *cancellableWriter) Write(p []byte) (n int, err error) {
	if r.cancelled {
		runtime.Gosched()
		return 0, io.ErrClosedPipe
	}
	select {
	case <-r.cancelChannel:
		r.cancelled = true
		return 0, io.ErrClosedPipe
	default:
		n, err = r.baseWriter.Write(p)
		// Necessary to prevent a busy loop from a process writing massive amounts of output from starving a timeout timer
		// Yield after the write to prevent another process from interrupting the write to the underlying writer
		runtime.Gosched()
		return
	}
}

// ExecuteCommand executes the given commands using the given working directory.
// Standard output and standard error are sent to the given writers.
func ExecuteCommand(
	context context.T,
	cancelFlag task.CancelFlag,
	workingDir string,
	stdoutWriter io.Writer,
	stderrWriter io.Writer,
	executionTimeout int,
	commandName string,
	commandArguments []string,
	envVars map[string]string,
) (exitCode int, err error) {
	log := context.Log()

	stdoutInterruptable, stopStdout := newWriter(stdoutWriter)
	stderrInterruptable, stopStderr := newWriter(stderrWriter)

	command := exec.Command(commandName, commandArguments...)
	command.Dir = workingDir
	exitCode = 0

	// If we assign the writers directly, the command may never exit even though a command.Process.Wait() does due to https://github.com/golang/go/issues/13155
	// However, if we run goroutines to copy from the StdoutPipe and StderrPipe we may lose the last write.
	command.Stdout = stdoutInterruptable
	command.Stderr = stderrInterruptable
	/*
		stdoutPipe, err := command.StdoutPipe()
		if err != nil {
			return 1, err
		}
		stderrPipe, err := command.StderrPipe()
		if err != nil {
			return 1, err
		}
		go io.Copy(stdoutInterruptable, stdoutPipe)
		go io.Copy(stderrInterruptable, stderrPipe)
	*/

	// configure OS-specific process settings
	prepareProcess(command)

	// configure environment variables
	prepareEnvironment(context, command, envVars)

	log.Debug()
	log.Debugf("Running in directory %v, command: %v %v", workingDir, commandName, commandArguments)
	log.Debug()
	if err = command.Start(); err != nil {
		log.Error("error occurred starting the command", err)
		exitCode = 1
		return
	}

	signal := timeoutSignal{}

	cancelled := make(chan bool, 1)
	go func() {
		cancelState := cancelFlag.Wait()
		if cancelFlag.Canceled() {
			cancelled <- true
			log.Debug("Cancel flag set to cancelled")
		}
		log.Debugf("Cancel flag set to %v", cancelState)
	}()

	done := make(chan error, 1)
	go func() {
		done <- command.Wait()
	}()

	select {
	case <-time.After(time.Duration(executionTimeout) * time.Second):
		stopStdout <- true
		stopStderr <- true
		if err = killProcess(command.Process, &signal); err != nil {
			exitCode = 1
			log.Error(err)
		} else {
			// set appropriate exit code based on timeout
			exitCode = appconfig.CommandStoppedPreemptivelyExitCode
			err = &exec.ExitError{Stderr: []byte("Process timed out")}
			log.Infof("The execution of command was timedout.")
		}
	case <-cancelled:
		// task has been asked to cancel, kill process
		log.Debug("Process cancelled. Attempting to stop process.")
		stopStdout <- true
		stopStderr <- true
		if err = killProcess(command.Process, &signal); err != nil {
			exitCode = 1
			log.Error(err)
		} else {
			// set appropriate exit code based on cancel
			exitCode = appconfig.CommandStoppedPreemptivelyExitCode
			err = &exec.ExitError{Stderr: []byte("Cancelled process")}
			log.Infof("The execution of command was cancelled.")
		}
	case err = <-done:
		log.Debug("Process completed.")
		if err != nil {
			exitCode = 1
			log.Debugf("command returned error %v", err)
			if exiterr, ok := err.(*exec.ExitError); ok {
				// The program has exited with an exit code != 0
				if status, ok := exiterr.Sys().(syscall.WaitStatus); ok {
					exitCode = status.ExitStatus()

					if signal.execInterruptedOnWindows {
						log.Debug("command interrupted by cancel or timeout")
						exitCode = -1
					}

					// First try to handle Cancel and Timeout scenarios
					// SIGKILL will result in an exitcode of -1
					if exitCode == -1 {
						if cancelFlag.Canceled() {
							// set appropriate exit code based on cancel or timeout
							exitCode = appconfig.CommandStoppedPreemptivelyExitCode
							log.Infof("The execution of command was cancelled.")
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
		}
	}
	return
}

// StartCommand starts the given commands using the given working directory.
// Standard output and standard error are sent to the given writers.
func StartCommand(context context.T,
	cancelFlag task.CancelFlag,
	workingDir string,
	stdoutWriter io.Writer,
	stderrWriter io.Writer,
	commandName string,
	commandArguments []string,
) (process *os.Process, exitCode int, err error) {
	log := context.Log()
	command := exec.Command(commandName, commandArguments...)
	command.Dir = workingDir
	command.Stdout = stdoutWriter
	command.Stderr = stderrWriter
	exitCode = 0

	// configure OS-specific process settings
	prepareProcess(command)

	// configure environment variables
	prepareEnvironment(context, command, make(map[string]string))

	log.Debug()
	log.Debugf("Running in directory %v, command: %v %v", workingDir, commandName, commandArguments)
	log.Debug()
	if err = command.Start(); err != nil {
		log.Error("error occurred starting the command: ", err)
		exitCode = 1
		return
	}

	process = command.Process
	signal := timeoutSignal{}
	// Async commands don't use cancellable writers because we rely on the process having an independent copy of
	// the writer when it is a file handle and when the cancellable writer is assigned, it doesn't (by design) give
	// a reference to the file handle to the process
	cancelChannel := make(chan bool, 2)
	go killProcessOnCancel(log, command, cancelChannel, cancelChannel, cancelFlag, &signal)

	return
}

// killProcessOnCancel waits for a cancel request.
// If a cancel request is received, this method kills the underlying
// process of the command. This will unblock the command.Wait() call.
// If the task completed successfully this method returns with no action.
func killProcessOnCancel(log log.T, command *exec.Cmd, cancelStdout chan bool, cancelStderr chan bool, cancelFlag task.CancelFlag, signal *timeoutSignal) {
	defer func() {
		if r := recover(); r != nil {
			log.Errorf("Kill process on cancel panic: %v", r)
			log.Errorf("Stacktrace:\n%s", debug.Stack())
		}
	}()
	cancelFlag.Wait()
	if cancelFlag.Canceled() {
		log.Debug("Process cancelled. Attempting to stop process.")

		cancelStdout <- true
		cancelStderr <- true
		runtime.Gosched()

		// task has been asked to cancel, kill process
		if err := killProcess(command.Process, signal); err != nil {
			log.Error(err)
		} else {
			log.Debug("Process stopped successfully.")
		}
		return
	}
}

// prepareEnvironment adds ssm agent standard environment variables or environment variables defined by customer/other plugins to the command
func prepareEnvironment(context context.T, command *exec.Cmd, envVars map[string]string) {
	log := context.Log()
	env := os.Environ()

	for key, val := range envVars {
		env = append(env, fmtEnvVariable(key, val))
	}
	if instance, err := context.Identity().InstanceID(); err == nil {
		env = append(env, fmtEnvVariable(envVarInstanceID, instance))
	}
	if region, err := context.Identity().Region(); err == nil {
		env = append(env, fmtEnvVariable(envVarRegionName, region))
	}
	if platformName, err := platform.PlatformName(log); err == nil {
		env = append(env, fmtEnvVariable(envVarPlatformName, platformName))
	} else {
		log.Warnf("There was an error retrieving the platformName while setting the environment variables: %v", err)
	}
	if platformVersion, err := platform.PlatformVersion(log); err == nil {
		env = append(env, fmtEnvVariable(envVarPlatformVersion, platformVersion))
	} else {
		log.Warnf("There was an error retrieving the platformVersion while setting the environment variables: %v", err)
	}
	command.Env = env

	// Running powershell on linux erquired the HOME env variable to be set and to remove the TERM env variable
	validateEnvironmentVariables(command)
}

// fmtEnvVariable creates the string to append to the current set of environment variables.
func fmtEnvVariable(name string, val string) string {
	return fmt.Sprintf("%s=%s", name, val)
}

// QuoteShString replaces the quote
func QuoteShString(str string) (result string) {
	// Simple quote replacement for now
	return fmt.Sprintf("'%v'", strings.Replace(str, "'", "'\\''", -1))
}

// QuotePsString replaces the quote
func QuotePsString(str string) (result string) {
	// Simple quote replacement for now
	str = strings.Replace(str, "`", "``", -1)
	return fmt.Sprintf("\"%v\"", strings.Replace(str, "\"", "`\"", -1))
}
