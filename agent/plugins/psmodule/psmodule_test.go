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

// Package psmodule implements the power shell module plugin.
//
// +build windows

package psmodule

import (
	"fmt"
	"testing"

	"github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/aws/amazon-ssm-agent/agent/contracts"
	"github.com/aws/amazon-ssm-agent/agent/executers"
	"github.com/aws/amazon-ssm-agent/agent/framework/processor/executer/iohandler"
	"github.com/aws/amazon-ssm-agent/agent/framework/runpluginutil"
	"github.com/aws/amazon-ssm-agent/agent/jsonutil"
	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/amazon-ssm-agent/agent/task"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/twinj/uuid"
)

type TestCase struct {
	Input          PSModulePluginInput
	Output         iohandler.DefaultIOHandler
	ExecuterErrors error
	MessageID      string
}

type CommandTester func(p *Plugin, mockCancelFlag *task.MockCancelFlag, mockExecuter *executers.MockCommandExecuter, mockIOHandler *iohandlermocks.MockIOHandler)

const (
	orchestrationDirectory  = "OrchesDir"
	defaultWorkingDirectory = ""
	s3BucketName            = "bucket"
	s3KeyPrefix             = "key"
)

var TestCases = []TestCase{
	generateTestCaseOk("0"),
	generateTestCaseOk("1"),
	generateTestCaseFail("2"),
	generateTestCaseFail("3"),
}

var logger = log.NewMockLog()

func generateTestCaseOk(id string) TestCase {
	return TestCase{
		Input: PSModulePluginInput{
			RunCommand:       []string{"echo " + id},
			ID:               id + ".aws:runScript",
			WorkingDirectory: "Dir" + id,
			TimeoutSeconds:   "1",
		},
		Output: iohandler.DefaultIOHandler{},
	}

	testCase.Output.StdoutWriter = new(multiwritermock.MockDocumentIOMultiWriter)
	testCase.Output.StderrWriter = new(multiwritermock.MockDocumentIOMultiWriter)
	testCase.Output.SetStdout("standard output of test case " + id)
	testCase.Output.SetStderr("standard error of test case " + id)
	testCase.Output.ExitCode = 0
	testCase.Output.Status = "Success"
}

func generateTestCaseFail(id string) TestCase {
	t := generateTestCaseOk(id)
	t.ExecuterError = fmt.Errorf("Error happened for cmd %v", id)
	t.Output.SetStderr(combinedErrorOutput(t.ExecuterStdErr, t.ExecuterError))
	t.Output.ExitCode = 1
	t.Output.Status = "Failed"
	return t
}

// TestRunCommands tests the runCommands and runCommandsRawInput methods, which run one set of commands.
func TestRunCommands(t *testing.T) {
	for _, testCase := range TestCases {
		testRunCommands(t, testCase, true)
		testRunCommands(t, testCase, false)
	}
}

// testRunCommands tests the runCommands or the runCommandsRawInput method for one testcase.
func testRunCommands(t *testing.T, testCase TestCase, rawInput bool) {
	logger.On("Error", mock.Anything).Return(nil)
	logger.Infof("test run commands %v", testCase)
	runCommandTester := func(p *Plugin, mockCancelFlag *task.MockCancelFlag, mockExecuter *executers.MockCommandExecuter, mockIOHandler *iohandlermocks.MockIOHandler) {
		// set expectations
		setExecuterExpectations(mockExecuter, testCase, mockCancelFlag, p)
		setIOHandlerExpectations(mockIOHandler, testCase)

		// call method under test
		if rawInput {
			// prepare plugin input
			var rawPluginInput interface{}
			err := jsonutil.Remarshal(testCase.Input, &rawPluginInput)
			assert.Nil(t, err)

			p.runCommandsRawInput(logger, rawPluginInput, orchestrationDirectory, mockCancelFlag, s3BucketName, s3KeyPrefix)
		} else {
			p.runCommands(logger, testCase.Input, orchestrationDirectory, mockCancelFlag, s3BucketName, s3KeyPrefix)
		}
	}

	testExecution(t, runCommandTester)
}

// TestBucketsInDifferentRegions tests runCommands when S3Buckets are present in IAD and PDX region.
func TestBucketsInDifferentRegions(t *testing.T) {
	for _, testCase := range TestCases {
		testBucketsInDifferentRegions(t, testCase, true)
		testBucketsInDifferentRegions(t, testCase, false)
	}
}

// testBucketsInDifferentRegions tests the runCommands with S3 buckets present in IAD and PDX region.
func testBucketsInDifferentRegions(t *testing.T, testCase TestCase, testingBucketsInDifferentRegions bool) {
	logger.On("Error", mock.Anything).Return(nil)
	logger.Infof("test run commands %v", testCase)
	runCommandTester := func(p *Plugin, mockCancelFlag *task.MockCancelFlag, mockExecuter *executers.MockCommandExecuter, mockIOHandler *iohandlermocks.MockIOHandler) {
		// set expectations
		setExecuterExpectations(mockExecuter, testCase, mockCancelFlag, p)
		setIOHandlerExpectations(mockIOHandler, testCase)

		// call method under test
		p.runCommands(logger, testCase.Input, orchestrationDirectory, mockCancelFlag, s3BucketName, s3KeyPrefix)
	}

	testExecution(t, runCommandTester)
}

// TestExecute tests the Execute method, which runs multiple sets of commands.
func TestExecute(t *testing.T) {
	// test each plugin input as a separate execution
	for _, testCase := range TestCases {
		testExecute(t, testCase)
	}
}

// testExecute tests the run command plugin's Execute method.
func testExecute(t *testing.T, testCase TestCase) {
	executeTester := func(p *Plugin, mockCancelFlag *task.MockCancelFlag, mockExecuter *executers.MockCommandExecuter, mockIOHandler *iohandlermocks.MockIOHandler) {
		// setup expectations and correct outputs
		var pluginProperties []interface{}
		var correctOutputs string
		mockContext := context.NewMockDefault()

		// set expectations
		setCancelFlagExpectations(mockCancelFlag)
		setExecuterExpectations(mockExecuter, testCase, mockCancelFlag, p)
		setIOHandlerExpectations(mockIOHandler, testCase)

		// prepare plugin input
		var rawPluginInput interface{}
		err := jsonutil.Remarshal(testCase.Input, &rawPluginInput)
		assert.Nil(t, err)

		pluginProperties = append(pluginProperties, rawPluginInput)
		correctOutputs = testCase.Output.String()

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
				PluginID:               "aws:runCommand1",
			}, mockCancelFlag, runpluginutil.PluginRunner{})

		// assert that the flag is checked after every set of commands
		mockCancelFlag.AssertNumberOfCalls(t, "Canceled", 1)
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
		mockIOHandler.On("MarkAsFailed", mock.Anything, fmt.Errorf("failed to run commands: %v", t.ExecuterError)).Return()
		mockIOHandler.On("SetStatus", contracts.ResultStatusFailed).Return()
	}
}

func setCancelFlagExpectations(mockCancelFlag *task.MockCancelFlag, times int) {
	mockCancelFlag.On("Canceled").Return(false).Times(times)
	mockCancelFlag.On("ShutDown").Return(false).Times(times)
}
