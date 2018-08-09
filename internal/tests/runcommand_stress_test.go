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

// AgentStressTestSuite defines agent test suite, and absorb the built-in basic suite
// functionality from testify - including a T() method which
// returns the current testing context
type AgentStressTestSuite struct {
	suite.Suite
	ssmAgent   agent.ISSMAgent
	mdsSdkMock *mdssdkmock.SSMMDSAPI
	log        log.T
}

// SetupTest makes sure that all the components referenced in the test case are initialized
// before each test
func (suite *AgentStressTestSuite) SetupTest() {
	log := logger.SSMLogger(true)
	suite.log = log

	config, err := appconfig.Config(true)
	if err != nil {
		log.Debugf("appconfig could not be loaded - %v", err)
		return
	}
	context := context.Default(log, config)

	// Mock MDS service to remove dependency on external service
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
	// For this test we don't need to inject all the modules
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

//TestCoreAgent tests the agent by mocking MDS to send N messages to the agent and start the execution of those messages
func (suite *AgentStressTestSuite) TestCoreAgent() {
	// This is the number of MDS messages that should be sent to the core agent
	numberOfMessages := 100

	// Mock MDs service so it returns only the desired number of messages, it'll return empty messages after that.
	// That's because the agent is a loop and it keeps polling messages
	suite.mdsSdkMock.On("GetMessagesRequest", mock.AnythingOfType("*ssmmds.GetMessagesInput")).Return(&request.Request{}, func(input *ssmmds.GetMessagesInput) *ssmmds.GetMessagesOutput {
		messageOutput, _ := testutils.GenerateMessages(testdata.EchoMDSMessage)
		return messageOutput
	}, nil).Times(numberOfMessages)

	suite.mdsSdkMock.On("GetMessagesRequest", mock.AnythingOfType("*ssmmds.GetMessagesInput")).Return(&request.Request{}, func(input *ssmmds.GetMessagesInput) *ssmmds.GetMessagesOutput {
		emptyMessage, _ := testutils.GenerateEmptyMessage()
		return emptyMessage
	}, nil)

	defer func() {
		// recover in case the agent panics
		// this should handle some kind of seg fault errors.
		if msg := recover(); msg != nil {
			suite.log.Errorf("Agent crashed with message %v!", msg)
			suite.log.Errorf("%s: %s", msg, debug.Stack())
			suite.T().Fail()
		}
		// close the log to get full logs after the test is done
		suite.log.Flush()
		suite.log.Close()
	}()

	// a channel to block test execution untill the agent is done processing the required number of messages
	c := make(chan int)
	suite.mdsSdkMock.On("SendReplyRequest", mock.AnythingOfType("*ssmmds.SendReplyInput")).Return(&request.Request{}, func(input *ssmmds.SendReplyInput) *ssmmds.SendReplyOutput {
		payload := input.Payload
		// unmarshal the reply sent back to MDS, verify that the document has succeed
		// If one document failed, it'll mark the test as failed. If we got reply for all the required messages
		// we end the test and it will be marked as passed.
		var sendReplyPayload messageContracts.SendReplyPayload
		json.Unmarshal([]byte(*payload), &sendReplyPayload)

		if sendReplyPayload.DocumentStatus == contracts.ResultStatusFailed || sendReplyPayload.DocumentStatus == contracts.ResultStatusTimedOut {
			suite.T().Errorf("Document execution %v", sendReplyPayload.DocumentStatus)
			c <- 1
		} else if sendReplyPayload.DocumentStatus == contracts.ResultStatusSuccess {
			numberOfMessages--
			if numberOfMessages == 0 {
				c <- 1
			}
		}
		return &ssmmds.SendReplyOutput{}
	})

	// start the agent and block test until it finishes executing documents
	suite.ssmAgent.Start()
	<-c

	// stop agent execution
	suite.ssmAgent.Stop()
}

func TestAgentStressTestSuite(t *testing.T) {
	suite.Run(t, new(AgentStressTestSuite))
}
