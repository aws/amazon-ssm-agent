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
	"fmt"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"runtime/debug"
	"testing"

	"github.com/aws/amazon-ssm-agent/agent/framework/coremodules"
	"github.com/aws/amazon-ssm-agent/common/identity/identity"

	"github.com/aws/amazon-ssm-agent/agent/agent"
	"github.com/aws/amazon-ssm-agent/agent/appconfig"
	"github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/aws/amazon-ssm-agent/agent/contracts"
	"github.com/aws/amazon-ssm-agent/agent/fileutil"
	"github.com/aws/amazon-ssm-agent/agent/framework/coremanager"
	"github.com/aws/amazon-ssm-agent/agent/log"
	logger "github.com/aws/amazon-ssm-agent/agent/log/ssmlog"
	messageContracts "github.com/aws/amazon-ssm-agent/agent/runcommand/contracts"
	"github.com/aws/amazon-ssm-agent/core/app/runtimeconfiginit"
	"github.com/aws/amazon-ssm-agent/internal/tests/testdata"
	"github.com/aws/amazon-ssm-agent/internal/tests/testutils"
	"github.com/aws/aws-sdk-go/aws/request"
	"github.com/aws/aws-sdk-go/service/ssmmds"
	mdssdkmock "github.com/aws/aws-sdk-go/service/ssmmds/ssmmdsiface/mocks"
	assert "github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
)

// CrashWorkerTestSuite defines test suite for sending a command to the agent and handling the worker process crash
type CrashWorkerTestSuite struct {
	context context.T
	suite.Suite
	ssmAgent   agent.ISSMAgent
	mdsSdkMock *mdssdkmock.SsmmdsAPI
	log        log.T
}

func (suite *CrashWorkerTestSuite) SetupTest() {
	log := logger.SSMLogger(true)
	suite.log = log

	config, err := appconfig.Config(true)
	if err != nil {
		log.Debugf("appconfig could not be loaded - %v", err)
		return
	}
	identitySelector := identity.NewDefaultAgentIdentitySelector(log)
	agentIdentity, err := identity.NewAgentIdentity(log, &config, identitySelector)
	if err != nil {
		log.Debugf("unable to assume identity - %v", err)
		return
	}

	suite.context = context.Default(log, config, agentIdentity)

	rtci := runtimeconfiginit.New(log, agentIdentity)
	if err := rtci.Init(); err != nil {
		panic(fmt.Sprintf("Failed to initialize runtimeconfig: %v", err))
	}

	// Mock MDS service to remove dependency on external service
	sendMdsSdkRequest := func(req *request.Request) error {
		return nil
	}
	mdsSdkMock := testutils.NewMdsSdkMock()
	mdsService := testutils.NewMdsService(suite.context, mdsSdkMock, sendMdsSdkRequest)

	suite.mdsSdkMock = mdsSdkMock

	messageServiceModule := testutils.NewMessageService(suite.context, mdsService)
	var modules []contracts.ICoreModuleWrapper
	modules = append(modules, coremodules.NewCoreModuleWrapper(log, messageServiceModule))

	// Create core manager that accepts runcommand core module
	// For this test we don't need to inject all the modules
	var cpm *coremanager.CoreManager
	if cpm, err = testutils.NewCoreManager(suite.context, &modules); err != nil {
		log.Errorf("error occurred when starting core manager: %v", err)
		return
	}
	// Create core ssm agent
	suite.ssmAgent = &agent.SSMAgent{}
	suite.ssmAgent.SetContext(suite.context)
	suite.ssmAgent.SetCoreManager(cpm)
}

func (suite *CrashWorkerTestSuite) TearDownSuite() {
	// Cleanup runtime config
	os.RemoveAll(appconfig.RuntimeConfigFolderPath)

	// Close the log only after the all tests are done.
	suite.log.Close()

	instanceId, _ := suite.context.Identity().InstanceID()
	//Empty the current folder
	currentDirectory := filepath.Join(appconfig.DefaultDataStorePath,
		instanceId,
		appconfig.DefaultDocumentRootDirName,
		appconfig.DefaultLocationOfState,
		appconfig.DefaultLocationOfCurrent)
	files, _ := fileutil.GetFileNames(currentDirectory)
	for _, file := range files {
		fileutil.DeleteFile(path.Join(currentDirectory, file))
	}
}

func cleanUpCrashWorkerTest(suite *CrashWorkerTestSuite) {
	// recover in case the agent panics
	// this should handle some kind of seg fault errors.
	if msg := recover(); msg != nil {
		suite.T().Errorf("Agent crashed with message %v!", msg)
		suite.T().Errorf("%s: %s", msg, debug.Stack())
	}
	// flush the log to get full logs after the test is done, don't close the log unless all tests are done
	suite.log.Flush()
}

// TestDocumentWorkerCrash tests the agent processes documents in isolation
// the test sends a document that's expected to crash and another that's expected to succeed
// then verify the first document fails when document worker crashes and sends valid results
// and second document succeeds and sends the valid output
func (suite *CrashWorkerTestSuite) TestDocumentWorkerCrash() {
	//send MDS message that's expected to crash document worker
	var idOfCrashMessage string
	suite.mdsSdkMock.On("GetMessagesRequest", mock.AnythingOfType("*ssmmds.GetMessagesInput")).Return(&request.Request{HTTPRequest: &http.Request{}}, func(input *ssmmds.GetMessagesInput) *ssmmds.GetMessagesOutput {
		messageOutput, _ := testutils.GenerateMessages(suite.context, testdata.CrashWorkerMDSMessage)
		idOfCrashMessage = *messageOutput.Messages[0].MessageId
		return messageOutput
	}, nil).Once()

	//send MDS message that's expected to succeed
	var idOfGoodMessage string
	suite.mdsSdkMock.On("GetMessagesRequest", mock.AnythingOfType("*ssmmds.GetMessagesInput")).Return(&request.Request{HTTPRequest: &http.Request{}}, func(input *ssmmds.GetMessagesInput) *ssmmds.GetMessagesOutput {
		messageOutput, _ := testutils.GenerateMessages(suite.context, testdata.EchoMDSMessage)
		idOfGoodMessage = *messageOutput.Messages[0].MessageId
		return messageOutput
	}, nil).Once()

	suite.mdsSdkMock.On("GetMessagesRequest", mock.AnythingOfType("*ssmmds.GetMessagesInput")).Return(&request.Request{HTTPRequest: &http.Request{}}, func(input *ssmmds.GetMessagesInput) *ssmmds.GetMessagesOutput {
		emptyMessage, _ := testutils.GenerateEmptyMessage(suite.context)
		return emptyMessage
	}, nil)

	defer func() {
		cleanUpCrashWorkerTest(suite)
	}()

	// a channel to block test execution untill the agent is done processing the required number of messages
	c := make(chan int)
	suite.mdsSdkMock.On("SendReplyRequest", mock.AnythingOfType("*ssmmds.SendReplyInput")).Return(&request.Request{HTTPRequest: &http.Request{}}, func(input *ssmmds.SendReplyInput) *ssmmds.SendReplyOutput {
		messageId := *input.MessageId
		payload := input.Payload
		var sendReplyPayload messageContracts.SendReplyPayload
		json.Unmarshal([]byte(*payload), &sendReplyPayload)

		//verify that document worker crashed and agent was able to send back failed result
		if messageId == idOfCrashMessage {
			if sendReplyPayload.DocumentStatus == contracts.ResultStatusFailed {
				suite.T().Logf("Document execution %v", sendReplyPayload.DocumentStatus)
				foundPlugin := false
				for _, pluginStatus := range sendReplyPayload.RuntimeStatus {
					if pluginStatus.Status == contracts.ResultStatusFailed {
						foundPlugin = true
						assert.Contains(suite.T(), pluginStatus.Output, testdata.CrashWorkerErrorMessage, "plugin output doesn't contain the expected error message")
					}
				}
				if !foundPlugin {
					suite.T().Error("Couldn't find plugin with result status failed")
				}
				c <- 1
			} else if sendReplyPayload.DocumentStatus == contracts.ResultStatusSuccess {
				suite.T().Errorf("Document execution %v but it was supposed to fail", sendReplyPayload.DocumentStatus)
				c <- 1
			}
		}
		// verify that document execution succeeds in parallel and is not affected by the crashing document
		if messageId == idOfGoodMessage {
			if sendReplyPayload.DocumentStatus == contracts.ResultStatusFailed || sendReplyPayload.DocumentStatus == contracts.ResultStatusTimedOut {
				suite.T().Errorf("Document execution %v", sendReplyPayload.DocumentStatus)
				c <- 1
			} else if sendReplyPayload.DocumentStatus == contracts.ResultStatusSuccess {
				suite.T().Logf("Document execution %v", sendReplyPayload.DocumentStatus)
				foundPlugin := false
				for _, pluginStatus := range sendReplyPayload.RuntimeStatus {
					if pluginStatus.Status == contracts.ResultStatusSuccess {
						foundPlugin = true
						assert.Contains(suite.T(), pluginStatus.Output, testdata.EchoMessageOutput, "plugin output doesn't contain the expected error message")
					}
				}
				if !foundPlugin {
					suite.T().Error("Couldn't find plugin with result status failed")
				}
				c <- 1
			}
		}
		return &ssmmds.SendReplyOutput{}
	})

	// start the agent and block test until it finishes executing both documents
	suite.ssmAgent.Start()
	//wait for the first document to complete
	<-c
	//wait for the second document to complete
	<-c

	//Verify that the agent cleans up document state directories after worker crashes
	//Current folder will still have the document state
	//The next time agent runs it'll try to process documents in current folder
	//but will find that the document worker finished execution and it will remove it from current folder
	folders := []string{
		appconfig.DefaultLocationOfPending,
		appconfig.DefaultLocationOfCompleted,
		appconfig.DefaultLocationOfCorrupt}
	instanceId, _ := suite.context.Identity().InstanceID()
	for _, folder := range folders {
		directoryName := filepath.Join(appconfig.DefaultDataStorePath,
			instanceId,
			appconfig.DefaultDocumentRootDirName,
			appconfig.DefaultLocationOfState,
			folder)
		isDirEmpty, _ := fileutil.IsDirEmpty(directoryName)
		suite.T().Logf("Checking directory %s", directoryName)
		assert.True(suite.T(), isDirEmpty, "Directory is not empty")
	}

	// stop agent execution
	suite.ssmAgent.Stop()
}

func TestCrashWorkerTestSuite(t *testing.T) {
	suite.Run(t, new(CrashWorkerTestSuite))
}
