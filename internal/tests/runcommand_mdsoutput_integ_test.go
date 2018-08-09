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

// Package tests represents stress and integration tests of the agent
package tests

import (
	"encoding/json"
	"github.com/stretchr/testify/assert"
	"runtime"
	"runtime/debug"
	"testing"

	"github.com/aws/amazon-ssm-agent/agent/agent"
	"github.com/aws/amazon-ssm-agent/agent/appconfig"
	"github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/aws/amazon-ssm-agent/agent/contracts"
	"github.com/aws/amazon-ssm-agent/agent/framework/coremanager"
	"github.com/aws/amazon-ssm-agent/agent/log"
	logger "github.com/aws/amazon-ssm-agent/agent/log/ssmlog"
	messageContracts "github.com/aws/amazon-ssm-agent/agent/runcommand/contracts"
	"github.com/aws/amazon-ssm-agent/internal/tests/testdata"
	"github.com/aws/amazon-ssm-agent/internal/tests/testutils"
	"github.com/aws/aws-sdk-go/aws/request"
	"github.com/aws/aws-sdk-go/service/ssmmds"
	mdssdkmock "github.com/aws/aws-sdk-go/service/ssmmds/ssmmdsiface/mocks"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
)

// RunCommandOutputTestSuite defines test suite for sending runcommand output, error and exit code to MDS service
type RunCommandOutputTestSuite struct {
	suite.Suite
	ssmAgent   agent.ISSMAgent
	mdsSdkMock *mdssdkmock.SSMMDSAPI
	log        log.T
}

func (suite *RunCommandOutputTestSuite) SetupTest() {
	log := logger.SSMLogger(true)
	suite.log = log

	config, err := appconfig.Config(true)
	if err != nil {
		log.Debugf("appconfig could not be loaded - %v", err)
		return
	}
	context := context.Default(log, config)

	sendMdsSdkRequest := func(req *request.Request) error {
		return nil
	}
	mdsSdkMock := testutils.NewMdsSdkMock()
	mdsService := testutils.NewMdsService(mdsSdkMock, sendMdsSdkRequest)
	suite.mdsSdkMock = mdsSdkMock

	// The actual runcommand core module with mocked MDS service injected
	runcommandService := testutils.NewRuncommandService(context, mdsService)
	var modules []contracts.ICoreModule
	modules = append(modules, runcommandService)

	// Create core manager that accepts runcommand core module
	var cpm *coremanager.CoreManager
	if cpm, err = testutils.NewCoreManager(context, &modules, log); err != nil {
		log.Errorf("error occurred when starting core manager: %v", err)
		return
	}
	// Create core ssm agent
	suite.ssmAgent = &agent.SSMAgent{}
	suite.ssmAgent.SetContext(context)
	suite.ssmAgent.SetCoreManager(cpm)
}

func (suite *RunCommandOutputTestSuite) TearDownSuite() {
	// Close the log only after the all tests are done.
	suite.log.Close()
}

func cleanUpRunCommandOutputTest(suite *RunCommandOutputTestSuite) {
	// recover in case the agent panics
	// this should handle some kind of seg fault errors.
	if msg := recover(); msg != nil {
		suite.T().Errorf("Agent crashed with message %v!", msg)
		suite.T().Errorf("%s: %s", msg, debug.Stack())
	}
	// flush the log to get full logs after the test is done, don't close the log unless all tests are done
	suite.log.Flush()
}

func verifyRunCommandOutput(suite *RunCommandOutputTestSuite,
	docContent string,
	stdout string,
	stderr string,
	code int,
	expectedResultStatus contracts.ResultStatus,
	wrongResultStatus contracts.ResultStatus) {
	suite.mdsSdkMock.On("GetMessagesRequest", mock.AnythingOfType("*ssmmds.GetMessagesInput")).Return(&request.Request{}, func(input *ssmmds.GetMessagesInput) *ssmmds.GetMessagesOutput {
		messageOutput, _ := testutils.GenerateMessages(docContent)
		return messageOutput
	}, nil).Once()

	suite.mdsSdkMock.On("GetMessagesRequest", mock.AnythingOfType("*ssmmds.GetMessagesInput")).Return(&request.Request{}, func(input *ssmmds.GetMessagesInput) *ssmmds.GetMessagesOutput {
		emptyMessage, _ := testutils.GenerateEmptyMessage()
		return emptyMessage
	}, nil)

	c := make(chan int)
	suite.mdsSdkMock.On("SendReplyRequest", mock.AnythingOfType("*ssmmds.SendReplyInput")).Return(&request.Request{}, func(input *ssmmds.SendReplyInput) *ssmmds.SendReplyOutput {
		// unmarshal the reply sent back to MDS, verify plugin output
		payload := input.Payload
		var sendReplyPayload messageContracts.SendReplyPayload
		json.Unmarshal([]byte(*payload), &sendReplyPayload)

		if sendReplyPayload.DocumentStatus == wrongResultStatus {
			suite.T().Errorf("Document execution %v, expected %v", sendReplyPayload.DocumentStatus, expectedResultStatus)
			c <- 1
		} else if sendReplyPayload.DocumentStatus == expectedResultStatus {
			foundPlugin := false
			for _, pluginStatus := range sendReplyPayload.RuntimeStatus {
				if pluginStatus.Status == expectedResultStatus {
					foundPlugin = true
					assert.Contains(suite.T(), pluginStatus.StandardOutput, stdout, "plugin stdout is not as expected")
					assert.Contains(suite.T(), pluginStatus.StandardError, stderr, "plugin stderr is not as expected")
					assert.Equal(suite.T(), pluginStatus.Code, code, "Exit code is %v expected %v", pluginStatus.Code, code)
				}
			}
			if !foundPlugin {
				suite.T().Errorf("Couldn't find plugin with result status %v", expectedResultStatus)
			}
			c <- 1
		}
		return &ssmmds.SendReplyOutput{}
	})

	defer func() {
		cleanUpRunCommandOutputTest(suite)
	}()

	suite.ssmAgent.Start()
	// block test execution
	<-c
	// stop agent execution
	suite.ssmAgent.Stop()
}

// TestV2DocumentOutputZeroExitCode verifies document schema 2.2 stdout, stderr and exit code have the expected value if runcommand returns 0 exit code
func (suite *RunCommandOutputTestSuite) TestV2DocumentOutputZeroExitCode() {
	verifyRunCommandOutput(suite, testdata.ZeroExitCodeMessageV2, testdata.CommandStdout, testdata.CommandStderr, testdata.ZeroExitCode, contracts.ResultStatusSuccess, contracts.ResultStatusFailed)
}

// TestV2DocumentOutputNonZeroExitCode verifies document schema 2.2 stdout, stderr and exit code have the expected value if runcommand returns non zero exit code
func (suite *RunCommandOutputTestSuite) TestV2DocumentOutputNonZeroExitCode() {
	verifyRunCommandOutput(suite, testdata.NonZeroExitCodeMessageV2, testdata.CommandStdout, testdata.CommandStderr, testdata.NonZeroExitCode, contracts.ResultStatusFailed, contracts.ResultStatusSuccess)
}

// TestV1DocumentOutputZeroExitCode verifies document schema 1.2 stdout, stderr and exit code have the expected value if runcommand returns 0 exit code
func (suite *RunCommandOutputTestSuite) TestV1DocumentOutputZeroExitCode() {
	content := testdata.ZeroExitCodeMessage
	if runtime.GOOS == "windows" {
		content = testdata.ZeroExitCodeMessage_Windows
	}
	verifyRunCommandOutput(suite, content, testdata.CommandStdout, testdata.CommandStderr, testdata.ZeroExitCode, contracts.ResultStatusSuccess, contracts.ResultStatusFailed)
}

// TestV1DocumentOutputNonZeroExitCode verifies document schema 1.2 stdout, stderr and exit code have the expected value if runcommand returns non zero exit code
func (suite *RunCommandOutputTestSuite) TestV1DocumentOutputNonZeroExitCode() {
	content := testdata.NonZeroExitCodeMessage
	if runtime.GOOS == "windows" {
		content = testdata.NonZeroExitCodeMessage_Windows
	}
	verifyRunCommandOutput(suite, content, testdata.CommandStdout, testdata.CommandStderr, testdata.NonZeroExitCode, contracts.ResultStatusFailed, contracts.ResultStatusSuccess)
}
func TestRunCommandOutputIntegTestSuite(t *testing.T) {
	suite.Run(t, new(RunCommandOutputTestSuite))
}
