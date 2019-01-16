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

package runscript

import (
	"fmt"
	"testing"

	"github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/aws/amazon-ssm-agent/agent/contracts"
	"github.com/aws/amazon-ssm-agent/agent/executers"
	"github.com/aws/amazon-ssm-agent/agent/framework/processor/executer/iohandler"
	iohandlermocks "github.com/aws/amazon-ssm-agent/agent/framework/processor/executer/iohandler/mock"
	multiwritermock "github.com/aws/amazon-ssm-agent/agent/framework/processor/executer/iohandler/multiwriter/mock"
	"github.com/aws/amazon-ssm-agent/agent/jsonutil"
	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/amazon-ssm-agent/agent/task"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/twinj/uuid"
)

type TestCase struct {
	Input          RunScriptPluginInput
	Output         iohandler.DefaultIOHandler
	ExecuterStdOut string
	ExecuterStdErr string
	ExecuterError  error
	MessageID      string
}

type CommandTester func(p *Plugin, mockCancelFlag *task.MockCancelFlag, mockExecuter *executers.MockCommandExecuter, mockIOHandler *iohandlermocks.MockIOHandler)

const (
	orchestrationDirectory  = "OrchesDir"
	defaultWorkingDirectory = ""
	s3BucketName            = "bucket"
	s3KeyPrefix             = "key"
	pluginID                = "aws:runScript1"
)

var TestCases = []TestCase{
	generateTestCaseOk("0"),
	generateTestCaseOk("1"),
	generateTestCaseFail("2"),
	generateTestCaseFail("3"),
}

var MultiInputTestCases = generateTestCaseMultipleInputsOk([]string{"0", "1"})

var logger = log.NewMockLog()

func generateTestCaseOk(id string) TestCase {
	input := RunScriptPluginInput{
		RunCommand:       []string{"echo " + id},
		ID:               id + ".aws:runScript",
		WorkingDirectory: "/Dir" + id,
		TimeoutSeconds:   "1",
	}
	testCase := TestCase{
		Input:  input,
		Output: iohandler.DefaultIOHandler{},
	}

	testCase.Output.StdoutWriter = new(multiwritermock.MockDocumentIOMultiWriter)
	testCase.Output.StderrWriter = new(multiwritermock.MockDocumentIOMultiWriter)
	testCase.Output.SetStdout("standard output of test case " + id)
	testCase.ExecuterStdOut = testCase.Output.GetStdout()
	testCase.Output.SetStderr("standard error of test case " + id)
	testCase.ExecuterStdErr = testCase.Output.GetStderr()
	testCase.Output.ExitCode = 0
	testCase.Output.Status = "Success"

	return testCase
}

func generateTestCaseMultipleInputsOk(ids []string) []TestCase {
	testCases := make([]TestCase, 0)
	for _, id := range ids {
		testCases = append(testCases, generateTestCaseOk(id))
	}
	return testCases
}

func combinedErrorOutput(stderr string, err error) string {
	var expectedStdErr string
	if err != nil {
		expectedStdErr = fmt.Sprintf("failed to run commands: %v", err)
	}

	if len(expectedStdErr) > 0 {
		expectedStdErr = fmt.Sprintf("%v\n%v", expectedStdErr, stderr)
	} else {
		expectedStdErr = stderr
	}
	return expectedStdErr
}

func generateTestCaseFail(id string) TestCase {
	t := generateTestCaseOk(id)
	t.ExecuterError = fmt.Errorf("Error happened for cmd %v", id)
	t.Output.SetStderr(combinedErrorOutput(t.ExecuterStdErr, t.ExecuterError))
	t.Output.ExitCode = 1
	t.Output.Status = "Failed"
	return t
}

// TestRunScripts tests the runScripts and runScriptsRawInput methods, which run one set of commands.
func TestRunScripts(t *testing.T) {
	for _, testCase := range TestCases {
		testRunScripts(t, testCase, true)
		testRunScripts(t, testCase, false)
	}
}

// testRunScripts tests the runScripts or the runScriptsRawInput method for one testcase.
func testRunScripts(t *testing.T, testCase TestCase, rawInput bool) {
	logger.On("Error", mock.Anything).Return(nil)
	logger.Infof("test run commands %v", testCase)
	runScriptTester := func(p *Plugin, mockCancelFlag *task.MockCancelFlag, mockExecuter *executers.MockCommandExecuter, mockIOHandler *iohandlermocks.MockIOHandler) {
		// set expectations
		setExecuterExpectations(mockExecuter, testCase, mockCancelFlag, p)
		setIOHandlerExpectations(mockIOHandler, testCase)

		// call method under test
		if rawInput {
			// prepare plugin input
			var rawPluginInput interface{}
			err := jsonutil.Remarshal(testCase.Input, &rawPluginInput)
			assert.Nil(t, err)

			p.runCommandsRawInput(logger, pluginID, rawPluginInput, orchestrationDirectory, defaultWorkingDirectory, mockCancelFlag, mockIOHandler)
		} else {
			p.runCommands(logger, pluginID, testCase.Input, orchestrationDirectory, defaultWorkingDirectory, mockCancelFlag, mockIOHandler)
		}
	}

	testExecution(t, runScriptTester)
}

// TestBucketsInDifferentRegions tests runScripts when S3Buckets are present in IAD and PDX region.
func TestBucketsInDifferentRegions(t *testing.T) {
	for _, testCase := range TestCases {
		testBucketsInDifferentRegions(t, testCase, true)
		testBucketsInDifferentRegions(t, testCase, false)
	}
}

// testBucketsInDifferentRegions tests the runScripts with S3 buckets present in IAD and PDX region.
func testBucketsInDifferentRegions(t *testing.T, testCase TestCase, testingBucketsInDifferentRegions bool) {
	logger.On("Error", mock.Anything).Return(nil)
	logger.Infof("test run commands %v", testCase)
	runScriptTester := func(p *Plugin, mockCancelFlag *task.MockCancelFlag, mockExecuter *executers.MockCommandExecuter, mockIOHandler *iohandlermocks.MockIOHandler) {
		// set expectations
		setExecuterExpectations(mockExecuter, testCase, mockCancelFlag, p)
		setIOHandlerExpectations(mockIOHandler, testCase)

		// call method under test
		p.runCommands(logger, pluginID, testCase.Input, orchestrationDirectory, defaultWorkingDirectory, mockCancelFlag, mockIOHandler)
	}

	testExecution(t, runScriptTester)
}

// TestExecute tests the Execute method, which runs multiple sets of commands.
func TestExecute(t *testing.T) {
	// test each plugin input as a separate execution
	for _, testCase := range TestCases {
		testExecute(t, testCase)
		testExecute(t, testCase)
	}
	for _, testCase := range MultiInputTestCases {
		testExecute(t, testCase)
	}
}

func arrayPropertyBuilder(t *testing.T, testCases []TestCase) interface{} {
	var pluginProperties []interface{}

	// prepare plugin input
	for _, testCase := range testCases {
		var rawPluginInput interface{}
		err := jsonutil.Remarshal(testCase.Input, &rawPluginInput)
		assert.Nil(t, err)
		pluginProperties = append(pluginProperties, rawPluginInput)
	}
	return pluginProperties
}

func singleValuePropertyBuilder(t *testing.T, testCase TestCase) interface{} {
	// prepare plugin input
	var rawPluginInput interface{}
	err := jsonutil.Remarshal(testCase.Input, &rawPluginInput)
	assert.Nil(t, err)

	return rawPluginInput
}

// Build expected outputs for non-truncated case
func buildOutputs(testCases []TestCase) (out string, err string, combined string) {
	for _, testCase := range testCases {
		if len(out) > 0 {
			out = fmt.Sprintf("%v\n%v", out, testCase.Output.GetStdout())
		} else {
			out = testCase.Output.GetStdout()
		}
		if len(err) > 0 {
			err = fmt.Sprintf("%v\n%v", err, testCase.Output.GetStderr())
		} else {
			err = testCase.Output.GetStderr()
		}
	}
	combined = out
	if len(err) > 0 {
		combined = fmt.Sprintf("%v%v%v", combined, "\n----------ERROR-------\n", err)
	}
	return out, err, combined
}

func testExecuteMultiInput(t *testing.T, testCases []TestCase) {
	executeTester := func(p *Plugin, mockCancelFlag *task.MockCancelFlag, mockExecuter *executers.MockCommandExecuter, mockIOHandler *iohandlermocks.MockIOHandler) {
		// setup expectations and correct outputs
		mockContext := context.NewMockDefault()

		// set expectations
		setCancelFlagExpectations(mockCancelFlag, len(testCases))
		for _, testCase := range testCases {
			setExecuterExpectations(mockExecuter, testCase, mockCancelFlag, p)
			setIOHandlerExpectations(mockIOHandler, testCase)
		}

		// prepare plugin input
		pluginProperties := arrayPropertyBuilder(t, testCases)

		//Create messageId which is in the format of aws.ssm.<commandID>.<InstanceID>
		commandID := uuid.NewV4().String()

		// call plugin
		p.Execute(
			mockContext,
			contracts.Configuration{
				Properties:             pluginProperties,
				OutputS3BucketName:     s3BucketName,
				OutputS3KeyPrefix:      s3KeyPrefix,
				OrchestrationDirectory: orchestrationDirectory,
				BookKeepingFileName:    commandID,
				PluginID:               pluginID,
			}, mockCancelFlag, mockIOHandler)
	}

	testExecution(t, executeTester)
}

// testExecute tests the run command plugin's Execute method.
func testExecute(t *testing.T, testCase TestCase) {
	executeTester := func(p *Plugin, mockCancelFlag *task.MockCancelFlag, mockExecuter *executers.MockCommandExecuter, mockIOHandler *iohandlermocks.MockIOHandler) {
		// setup expectations and correct outputs
		mockContext := context.NewMockDefault()

		// set expectations
		setCancelFlagExpectations(mockCancelFlag, 1)
		setExecuterExpectations(mockExecuter, testCase, mockCancelFlag, p)
		setIOHandlerExpectations(mockIOHandler, testCase)

		// prepare plugin input
		pluginProperties := singleValuePropertyBuilder(t, testCase)

		//Create messageId which is in the format of aws.ssm.<commandID>.<InstanceID>
		commandID := uuid.NewV4().String()

		// call plugin
		p.Execute(
			mockContext,
			contracts.Configuration{
				Properties:             pluginProperties,
				OutputS3BucketName:     s3BucketName,
				OutputS3KeyPrefix:      s3KeyPrefix,
				OrchestrationDirectory: orchestrationDirectory,
				BookKeepingFileName:    commandID,
				PluginID:               pluginID,
			}, mockCancelFlag, mockIOHandler)
	}

	testExecution(t, executeTester)
}

// testExecution sets up boiler plate mocked objects then delegates to a more
// specific tester, then asserts general expectations on the mocked objects.
// It is the responsibility of the inner tester to set up expectations
// and assert specific result conditions.
func testExecution(t *testing.T, commandtester CommandTester) {
	// create mocked objects
	mockCancelFlag := new(task.MockCancelFlag)
	mockExecuter := new(executers.MockCommandExecuter)
	mockIOHandler := new(iohandlermocks.MockIOHandler)

	// create plugin
	p := new(Plugin)
	p.CommandExecuter = mockExecuter
	p.Name = "aws:runShellScript"
	p.ScriptName = "_script.sh"
	p.ShellCommand = "sh"
	p.ShellArguments = []string{"-c"}

	// run inner command tester
	commandtester(p, mockCancelFlag, mockExecuter, mockIOHandler)

	// assert that the expectations were met
	mockExecuter.AssertExpectations(t)
	mockCancelFlag.AssertExpectations(t)
	mockIOHandler.AssertExpectations(t)
}

func setExecuterExpectations(mockExecuter *executers.MockCommandExecuter, t TestCase, cancelFlag task.CancelFlag, p *Plugin) {
	mockExecuter.On("NewExecute", mock.Anything, t.Input.WorkingDirectory, t.Output.StdoutWriter, t.Output.StderrWriter, cancelFlag, mock.Anything, mock.Anything, mock.Anything).Return(
		t.Output.ExitCode, t.ExecuterError)
}

func setIOHandlerExpectations(mockIOHandler *iohandlermocks.MockIOHandler, t TestCase) {
	mockIOHandler.On("GetStdoutWriter").Return(t.Output.StdoutWriter)
	mockIOHandler.On("GetStderrWriter").Return(t.Output.StderrWriter)
	mockIOHandler.On("SetExitCode", t.Output.ExitCode).Return()
	mockIOHandler.On("SetStatus", t.Output.Status).Return()
	if t.ExecuterError != nil {
		mockIOHandler.On("GetStatus").Return(t.Output.Status)
		mockIOHandler.On("MarkAsFailed", fmt.Errorf("failed to run commands: %v", t.ExecuterError)).Return()
		mockIOHandler.On("SetStatus", contracts.ResultStatusFailed).Return()
	}
}

func setCancelFlagExpectations(mockCancelFlag *task.MockCancelFlag, times int) {
	mockCancelFlag.On("Canceled").Return(false).Times(times)
	mockCancelFlag.On("ShutDown").Return(false).Times(times)
}
