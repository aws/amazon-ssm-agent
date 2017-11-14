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

// +build integration

// Package executers contains general purpose (shell) command executing objects.
package executers

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/aws/amazon-ssm-agent/agent/appconfig"
	"github.com/aws/amazon-ssm-agent/agent/fileutil"
	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/amazon-ssm-agent/agent/task"
	"github.com/stretchr/testify/assert"
	"github.com/twinj/uuid"
)

const (
	stdoutMsg                       = "hello stdout"
	stderrMsg                       = "hello stderr"
	stdoutMsg2                      = "bye stdout"
	stderrMsg2                      = "bye stderr"
	cancelWaitTimeoutSeconds        = 3.0
	successExitCode                 = 0
	processTerminatedByUserExitCode = 137
	defaultExecutionTimeout         = 5000
	stdOutFileName                  = "stdout"
	stdErrFileName                  = "stderr"
)

type CommandInvoker func(commands []string) (stdout io.Reader, stderr io.Reader, exitCode int, errs []error)

type TestCase struct {
	Commands         []string
	ExpectedStdout   string
	ExpectedStderr   string
	ExpectedExitCode int
}

var RunCommandTestCases = []TestCase{
	// test stdout is captured
	{
		Commands:         []string{"echo", stdoutMsg},
		ExpectedStdout:   stdoutMsg + "\n",
		ExpectedStderr:   "",
		ExpectedExitCode: successExitCode,
	},
	// test stderr is captured
	{
		Commands:         []string{"awk", awkPrintToStderr(stderrMsg)},
		ExpectedStdout:   "",
		ExpectedStderr:   stderrMsg + "\n",
		ExpectedExitCode: successExitCode,
	},
	// test both stdout and stderr are captured
	{
		Commands:         []string{"sh", "-c", echoToStdout(stdoutMsg) + ";" + echoToStderr(stderrMsg)},
		ExpectedStdout:   stdoutMsg + "\n",
		ExpectedStderr:   stderrMsg + "\n",
		ExpectedExitCode: successExitCode,
	},
	// instance id environment variable is set
	{
		Commands:         []string{"sh", "-c", fmt.Sprintf("echo $%v", envVarInstanceID)},
		ExpectedStdout:   testInstanceID + "\n",
		ExpectedStderr:   "",
		ExpectedExitCode: successExitCode,
	},
	// region name environment variable is set
	{
		Commands:         []string{"sh", "-c", fmt.Sprintf("echo $%v", envVarRegionName)},
		ExpectedStdout:   testRegionName + "\n",
		ExpectedStderr:   "",
		ExpectedExitCode: successExitCode,
	},
}

var RunCommandCancelTestCases = []TestCase{
	// test stdout and stderr are captured
	{
		Commands:         []string{"sleep", "10"},
		ExpectedStdout:   "",
		ExpectedStderr:   "",
		ExpectedExitCode: processTerminatedByUserExitCode,
	},
}

var RunCommandAsyncTestCases = []TestCase{
	// test both stdout and stderr are captured
	{
		Commands:         []string{"sh", "-c", echoToStdout(stdoutMsg) + ";" + echoToStderr(stderrMsg)},
		ExpectedStdout:   stdoutMsg + "\n",
		ExpectedStderr:   stderrMsg + "\n",
		ExpectedExitCode: successExitCode,
	},
	// test both stdout and stderr are captured if there is a delay between multiple outputs
	{
		Commands: []string{
			"sh",
			"-c",
			echoToStdout(stdoutMsg) + ";" + echoToStderr(stderrMsg) + ";" + "sleep 1" + ";" + echoToStdout(stdoutMsg2) + ";" + echoToStderr(stderrMsg2),
		},
		ExpectedStdout:   stdoutMsg + "\n" + stdoutMsg2 + "\n",
		ExpectedStderr:   stderrMsg + "\n" + stderrMsg2 + "\n",
		ExpectedExitCode: successExitCode,
	},
}

var ShellCommandExecuterTestCases = []TestCase{
	// test stdout and stderr are captured
	{
		Commands: []string{
			"sh",
			"-c",
			echoToStdout(stdoutMsg) + ";" + echoToStderr(stderrMsg),
		},
		ExpectedStdout:   stdoutMsg + "\n",
		ExpectedStderr:   stderrMsg + "\n",
		ExpectedExitCode: successExitCode,
	},
	// test both stdout and stderr are captured if there is a delay between multiple outputs
	{
		Commands: []string{
			"sh",
			"-c",
			echoToStdout(stdoutMsg) + ";" + echoToStderr(stderrMsg) + ";" + "sleep 1" + ";" + echoToStdout(stdoutMsg2) + ";" + echoToStderr(stderrMsg2),
		},
		ExpectedStdout:   stdoutMsg + "\n" + stdoutMsg2 + "\n",
		ExpectedStderr:   stderrMsg + "\n" + stderrMsg2 + "\n",
		ExpectedExitCode: successExitCode,
	},
}

var ShellCommandExecuterCancelTestCases = []TestCase{
	{
		Commands: []string{
			"sh",
			"-c",
			echoToStdout(stdoutMsg) + ";" + echoToStderr(stderrMsg) + ";" + "sleep 10" + ";" + echoToStdout("bye stdout") + ";" + echoToStderr("bye stderr"),
		},
		ExpectedStdout:   stdoutMsg + "\n",
		ExpectedStderr:   stderrMsg + "\n",
		ExpectedExitCode: processTerminatedByUserExitCode,
	},
}

var logger = log.NewMockLog()

// TestRunCommand tests that RunCommand (in memory call, no local script or output files) works correctly.
func TestRunCommand(t *testing.T) {
	instanceTemp := instance
	instance = &instanceInfoStub{instanceID: testInstanceID, regionName: testRegionName}
	defer func() { instance = instanceTemp }()

	for _, testCase := range RunCommandTestCases {
		runCommandInvoker, _ := prepareTestRunCommand(t)
		testCommandInvoker(t, runCommandInvoker, testCase)
	}
}

// TestRunCommand_cancel tests that RunCommand (in memory call, no local script or output files) is canceled correctly.
func TestRunCommand_cancel(t *testing.T) {
	instanceTemp := instance
	instance = &instanceInfoStub{instanceID: testInstanceID, regionName: testRegionName}
	defer func() { instance = instanceTemp }()

	for _, testCase := range RunCommandCancelTestCases {
		runCommandInvoker, cancelFlag := prepareTestRunCommand(t)
		testCommandInvokerCancel(t, runCommandInvoker, cancelFlag, testCase)
	}
}

// TestRunCommand_cancel tests that RunCommand (in memory call, no local script or output files) is canceled correctly.
func TestRunCommand_async(t *testing.T) {
	instanceTemp := instance
	instance = &instanceInfoStub{instanceID: testInstanceID, regionName: testRegionName}
	defer func() { instance = instanceTemp }()

	for _, testCase := range RunCommandAsyncTestCases {
		startCommandInvoker, cancelFlag := prepareTestStartCommand(t)
		testCommandInvoker(t, startCommandInvoker, testCase)
		testCommandInvokerShutdown(t, startCommandInvoker, cancelFlag, testCase)
	}
}

// TestShellCommandExecuter tests that ShellCommandExecuter (creates local script, redirects outputs to files) works
func TestShellCommandExecuter(t *testing.T) {
	instanceTemp := instance
	instance = &instanceInfoStub{instanceID: testInstanceID, regionName: testRegionName}
	defer func() { instance = instanceTemp }()
	runTest := func(testCase TestCase) {
		orchestrationDir, shCommandExecuterInvoker, _ := prepareTestShellCommandExecuter(t)
		defer fileutil.DeleteDirectory(orchestrationDir)
		testCommandInvoker(t, shCommandExecuterInvoker, testCase)
	}
	runTestShutdown := func(testCase TestCase) {
		orchestrationDir, shCommandExecuterInvoker, cancelFlag := prepareTestShellCommandExecuter(t)
		defer fileutil.DeleteDirectory(orchestrationDir)
		testCommandInvokerShutdown(t, shCommandExecuterInvoker, cancelFlag, testCase)
	}

	for _, testCase := range ShellCommandExecuterTestCases {
		runTest(testCase)
		runTestShutdown(testCase)
	}
}

// TestShellCommandExecuter_cancel tests that ShellCommandExecuter (creates local script, redirects outputs to files) is canceled correctly
func TestShellCommandExecuter_cancel(t *testing.T) {
	instanceTemp := instance
	instance = &instanceInfoStub{instanceID: testInstanceID, regionName: testRegionName}
	defer func() { instance = instanceTemp }()

	runTest := func(testCase TestCase) {
		orchestrationDir, shCommandExecuterInvoker, cancelFlag := prepareTestShellCommandExecuter(t)
		defer fileutil.DeleteDirectory(orchestrationDir)
		testCommandInvokerCancel(t, shCommandExecuterInvoker, cancelFlag, testCase)
	}

	for _, testCase := range ShellCommandExecuterCancelTestCases {
		runTest(testCase)
	}
}

func testCommandInvoker(t *testing.T, invoke CommandInvoker, testCase TestCase) {
	logger.Infof("testCommandInvoker")
	stdout, stderr, exitCode, errs := invoke(testCase.Commands)
	logger.Infof("errors %v", errs)

	assert.Equal(t, 0, len(errs))
	assertReaderEquals(t, testCase.ExpectedStdout, stdout)
	assertReaderEquals(t, testCase.ExpectedStderr, stderr)
	assert.Equal(t, exitCode, testCase.ExpectedExitCode)
}

//using long-running testcases for this test
func testCommandInvokerShutdown(t *testing.T, invoke CommandInvoker, cancelFlag task.CancelFlag, testCase TestCase) {
	go func() {
		time.Sleep(100 * time.Millisecond)
		cancelFlag.Set(task.ShutDown)
	}()
	logger.Infof("testCommandInvoker with shutdown")
	stdout, stderr, exitCode, errs := invoke(testCase.Commands)
	logger.Infof("errors %v", errs)
	// command should be uninterferred
	assert.Equal(t, 0, len(errs))
	assertReaderEquals(t, testCase.ExpectedStdout, stdout)
	assertReaderEquals(t, testCase.ExpectedStderr, stderr)
	assert.Equal(t, exitCode, testCase.ExpectedExitCode)
}

func testCommandInvokerCancel(t *testing.T, invoke CommandInvoker, cancelFlag task.CancelFlag, testCase TestCase) {
	go func() {
		time.Sleep(100 * time.Millisecond)
		cancelFlag.Set(task.Canceled)
	}()

	start := time.Now().UTC()
	stdout, stderr, exitCode, errs := invoke(testCase.Commands)
	duration := time.Since(start)

	// test that the job returned before the normal time
	assert.True(t, duration.Seconds() <= cancelWaitTimeoutSeconds, "The command took too long to kill (%v)!", duration)

	// test that we receive kill exception
	assert.Equal(t, len(errs), 1)
	assert.IsType(t, &exec.ExitError{}, errs[0])

	assertReaderEquals(t, testCase.ExpectedStdout, stdout)
	assertReaderEquals(t, testCase.ExpectedStderr, stderr)

	assert.Equal(t, exitCode, testCase.ExpectedExitCode)
}

// echoToStdout returns a shell command that outputs a message to the standard output stream.
func echoToStdout(msg string) string {
	return fmt.Sprintf(`echo "%v"`, msg)
}

// echoToStderr returns a shell command that outputs a message to the standard error stream.
func echoToStderr(msg string) string {
	return fmt.Sprintf("awk '%v'", awkPrintToStderr(msg))
}

// awkPrintToStderr returns an awk script that outputs a message to the standard error stream.
func awkPrintToStderr(stderrMsg string) string {
	return fmt.Sprintf(`BEGIN{print "%v" > "/dev/stderr"}`, stderrMsg)
}

// prepareTestShellCommandExecuter contains boiler plate code for testing shell executer, to avoid duplication.
func prepareTestShellCommandExecuter(t *testing.T) (orchestrationDir string, commandInvoker CommandInvoker, cancelFlag task.CancelFlag) {
	// create shell executer, cancel flag, working dir
	sh := ShellCommandExecuter{}
	cancelFlag = task.NewChanneledCancelFlag()
	orchestrationDir, err := ioutil.TempDir("", "TestShellExecute")
	if err != nil {
		t.Fatal(err)
	}
	workDir := "."

	// commandInvoker calls the shell then sets the state of the flag to completed
	commandInvoker = func(commands []string) (stdout io.Reader, stderr io.Reader, exitCode int, errs []error) {
		defer cancelFlag.Set(task.Completed)
		scriptPath := filepath.Join(orchestrationDir, appconfig.RunCommandScriptName)
		stdoutFilePath := filepath.Join(orchestrationDir, stdOutFileName)
		stderrFilePath := filepath.Join(orchestrationDir, stdErrFileName)

		// Used to mimic the process
		CreateScriptFile(scriptPath, commands)
		return sh.Execute(logger, workDir, stdoutFilePath, stderrFilePath, cancelFlag, defaultExecutionTimeout, commands[0], commands[1:])
	}

	return
}

// prepareTestRunCommand contains boiler plate code for testing run command, to avoid duplication.
func prepareTestRunCommand(t *testing.T) (commandInvoker CommandInvoker, cancelFlag task.CancelFlag) {
	cancelFlag = task.NewChanneledCancelFlag()
	commandInvoker = func(commands []string) (stdout io.Reader, stderr io.Reader, exitCode int, errs []error) {
		defer cancelFlag.Set(task.Completed)

		// run command and output to in-memory buffers
		var stdoutBuf bytes.Buffer
		var stderrBuf bytes.Buffer
		workDir := "."
		tempExitCode, err := ExecuteCommand(logger, cancelFlag, workDir, &stdoutBuf, &stderrBuf, defaultExecutionTimeout, commands[0], commands[1:])
		exitCode = tempExitCode

		// record error if any
		if err != nil {
			errs = append(errs, err)
		} else {
			errs = []error{}
		}

		// return readers that read from in-memory buffers
		stdout = bytes.NewReader(stdoutBuf.Bytes())
		stderr = bytes.NewReader(stderrBuf.Bytes())
		return
	}
	return
}

// prepareTestStartCommand contains boiler plate code for testing start command, to avoid duplication.
func prepareTestStartCommand(t *testing.T) (commandInvoker CommandInvoker, cancelFlag task.CancelFlag) {
	cancelFlag = task.NewChanneledCancelFlag()
	commandInvoker = func(commands []string) (stdout io.Reader, stderr io.Reader, exitCode int, errs []error) {
		// run command and output to temp files
		uuid.SwitchFormat(uuid.CleanHyphen)
		orchestrationDir, err := ioutil.TempDir("", "TestAsyncExecute")
		if err != nil {
			t.Fatal(err)
		}
		defer fileutil.DeleteDirectory(orchestrationDir)

		stdoutFilePath := filepath.Join(orchestrationDir, uuid.NewV4().String())
		stdoutWriter, err := os.OpenFile(stdoutFilePath, appconfig.FileFlagsCreateOrAppend, appconfig.ReadWriteAccess)
		if err != nil {
			t.Fatal(err)
		}
		defer os.Remove(stdoutFilePath)

		stderrFilePath := filepath.Join(orchestrationDir, uuid.NewV4().String())
		stderrWriter, err := os.OpenFile(stderrFilePath, appconfig.FileFlagsCreateOrAppend, appconfig.ReadWriteAccess)
		if err != nil {
			t.Fatal(err)
		}
		defer os.Remove(stdoutFilePath)

		workDir := "."
		process, tempExitCode, err := StartCommand(logger, cancelFlag, workDir, stdoutWriter, stderrWriter, commands[0], commands[1:])
		stdoutWriter.Close()
		stderrWriter.Close()
		exitCode = tempExitCode

		// record error if any
		if err != nil {
			errs = append(errs, err)
		} else {
			errs = []error{}
		}

		// wait for async process to finish
		process.Wait()

		// read temp files before they are deleted and return buffers
		stdoutBuf := bytes.NewBuffer(nil)
		stdoutReader, _ := os.Open(stdoutFilePath)
		defer stdoutReader.Close()
		_, err = io.Copy(stdoutBuf, stdoutReader)
		if err != nil {
			t.Fatal(err)
		}

		stderrBuf := bytes.NewBuffer(nil)
		stderrReader, _ := os.Open(stderrFilePath)
		defer stderrReader.Close()
		_, err = io.Copy(stderrBuf, stderrReader)
		if err != nil {
			t.Fatal(err)
		}

		stdout = bytes.NewReader(stdoutBuf.Bytes())
		stderr = bytes.NewReader(stderrBuf.Bytes())
		return
	}
	return
}

// assertReaderEquals is a convenience method that reads everything from a Reader then compares to a string.
func assertReaderEquals(t *testing.T, expected string, reader io.Reader) {
	actual, err := ioutil.ReadAll(reader)
	assert.Nil(t, err)
	assert.Equal(t, expected, string(actual))
}

// TestCreateScriptFile tests that CreateScriptFile correctly returns error (to avoid shadowing bug).
func TestCreateScriptFile(t *testing.T) {
	err := CreateScriptFile("/someDir,ThatDoes:Not#Exist/scriptName.sh", []string{"echo hello"})
	assert.NotNil(t, err)
}
