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

package executers

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"regexp"

	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/amazon-ssm-agent/agent/plugins/pluginutil"
	"github.com/aws/amazon-ssm-agent/agent/task"
	"github.com/stretchr/testify/assert"
)

const (
	stdoutMsg                       = "hello stdout"
	stderrMsg                       = "hello stderr"
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

type TestCaseRegexp struct {
	Commands         []string
	PatternStdout    string
	PatternStderr    string
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

var EnvVariableTestCases = []TestCaseRegexp{
	// instance id environment variable is set
	{
		Commands: []string{
			"sh",
			"-c",
			echoToStdout(fmt.Sprintf("$%v", envVarInstanceId)),
		},
		PatternStdout:    ".+", // not an empty string
		PatternStderr:    "",
		ExpectedExitCode: successExitCode,
	},
	// region name environment variable is set
	{
		Commands: []string{
			"sh",
			"-c",
			echoToStdout(fmt.Sprintf("$%v", envVarRegionName)),
		},
		PatternStdout:    ".+", // not an empty string
		PatternStderr:    "",
		ExpectedExitCode: successExitCode,
	},
}

var logger = log.NewMockLog()

// TestRunCommand tests that RunCommand (in memory call, no local script or output files) works correctly.
func TestRunCommand(t *testing.T) {
	for _, testCase := range RunCommandTestCases {
		runCommandInvoker, _ := prepareTestRunCommand(t)
		testCommandInvoker(t, runCommandInvoker, testCase)
	}
}

// TestRunCommand_cancel tests that RunCommand (in memory call, no local script or output files) is canceled correctly.
func TestRunCommand_cancel(t *testing.T) {
	for _, testCase := range RunCommandCancelTestCases {
		runCommandInvoker, cancelFlag := prepareTestRunCommand(t)
		testCommandInvokerCancel(t, runCommandInvoker, cancelFlag, testCase)
	}
}

// TestShellCommandExecuter tests that ShellCommandExecuter (creates local script, redirects outputs to files) works
func TestShellCommandExecuter(t *testing.T) {
	runTest := func(testCase TestCase) {
		orchestrationDir, shCommandExecuterInvoker, _ := prepareTestShellCommandExecuter(t)
		defer pluginutil.DeleteDirectory(logger, orchestrationDir)
		testCommandInvoker(t, shCommandExecuterInvoker, testCase)
	}

	for _, testCase := range ShellCommandExecuterTestCases {
		runTest(testCase)
	}
}

// TestShellCommandExecuter_cancel tests that ShellCommandExecuter (creates local script, redirects outputs to files) is canceled correctly
func TestShellCommandExecuter_cancel(t *testing.T) {
	runTest := func(testCase TestCase) {
		orchestrationDir, shCommandExecuterInvoker, cancelFlag := prepareTestShellCommandExecuter(t)
		defer pluginutil.DeleteDirectory(logger, orchestrationDir)
		testCommandInvokerCancel(t, shCommandExecuterInvoker, cancelFlag, testCase)
	}

	for _, testCase := range ShellCommandExecuterCancelTestCases {
		runTest(testCase)
	}
}

// TestRunCommand tests that RunCommand (in memory call, no local script or output files) works correctly.
func TestEnvVariable(t *testing.T) {
	for _, testCase := range EnvVariableTestCases {
		runCommandInvoker, _ := prepareTestRunCommand(t)
		testCommandInvokerRegexp(t, runCommandInvoker, testCase)
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

func testCommandInvokerRegexp(t *testing.T, invoke CommandInvoker, testCase TestCaseRegexp) {
	logger.Infof("testCommandInvokerRegexp")
	stdout, stderr, exitCode, errs := invoke(testCase.Commands)
	logger.Infof("errors %v", errs)

	assert.Equal(t, 0, len(errs))
	assertReaderRegexpMatch(t, testCase.PatternStdout, stdout)
	assertReaderRegexpMatch(t, testCase.PatternStderr, stderr)
	assert.Equal(t, exitCode, testCase.ExpectedExitCode)
}

func testCommandInvokerCancel(t *testing.T, invoke CommandInvoker, cancelFlag task.CancelFlag, testCase TestCase) {
	go func() {
		time.Sleep(100 * time.Millisecond)
		cancelFlag.Set(task.Canceled)
	}()

	start := time.Now()
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
		scriptPath := filepath.Join(orchestrationDir, pluginutil.RunCommandScriptName)
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
		tempExitCode, err := RunCommand(logger, cancelFlag, workDir, &stdoutBuf, &stderrBuf, defaultExecutionTimeout, commands[0], commands[1:])
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

// assertReaderEquals is a convenience method that reads everything from a Reader then compares to a string.
func assertReaderEquals(t *testing.T, expected string, reader io.Reader) {
	actual, err := ioutil.ReadAll(reader)
	assert.Nil(t, err)
	assert.Equal(t, expected, string(actual))
}

// assertReaderRegexpMatch is a convenience method that reads everything from a Reader and matches against a regexp pattern.
func assertReaderRegexpMatch(t *testing.T, pattern string, reader io.Reader) {
	actual, err := ioutil.ReadAll(reader)
	assert.Nil(t, err)
	isMatch, err := regexp.MatchString(pattern, string(actual))
	assert.Nil(t, err)
	assert.True(t, isMatch, fmt.Sprintf("actual {%v}", string(actual)))
}

// TestCreateScriptFile tests that CreateScriptFile correctly returns error (to avoid shadowing bug).
func TestCreateScriptFile(t *testing.T) {
	err := CreateScriptFile("/someDir,ThatDoes:Not#Exist/scriptName.sh", []string{"echo hello"})
	assert.NotNil(t, err)
}
