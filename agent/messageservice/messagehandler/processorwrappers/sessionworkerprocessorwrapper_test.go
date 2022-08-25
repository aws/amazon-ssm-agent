// Copyright 2021 Amazon.com, Inc. or its affiliates. All Rights Reserved.
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

// Package processorwrappers implements different processor wrappers to handle the processors which launches
// document worker and session worker for now
package processorwrappers

import (
	"testing"
	"time"

	"github.com/aws/amazon-ssm-agent/agent/framework/processor"

	"github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/aws/amazon-ssm-agent/agent/contracts"
	"github.com/aws/amazon-ssm-agent/agent/messageservice/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
	"github.com/twinj/uuid"
)

var (
	sessionnDocInfo = contracts.DocumentInfo{
		CreatedDate:  "2017-06-10T01-23-07.853Z",
		CommandID:    "13e8e6ad-e195-4ccb-86ee-328153b0dafe",
		MessageID:    "MessageID",
		DocumentName: "AWS-RunPowerShellScript",
		InstanceID:   "i-400e1090",
		RunCount:     0,
	}

	sessionDocState = contracts.DocumentState{
		DocumentInformation: sessionnDocInfo,
		DocumentType:        contracts.StartSession,
	}
)

type SessionWorkerProcessorWrapperTestSuite struct {
	suite.Suite
	contextMock                   *context.Mock
	sessionWorkerProcessorWrapper *SessionWorkerProcessorWrapper
	documentResultChan            chan contracts.DocumentResult
	outputMap                     map[contracts.UpstreamServiceName]chan contracts.DocumentResult
}

func (suite *SessionWorkerProcessorWrapperTestSuite) SetupTest() {
	contextMock := context.NewMockDefault()
	suite.contextMock = contextMock
	suite.documentResultChan = make(chan contracts.DocumentResult)
	suite.outputMap = make(map[contracts.UpstreamServiceName]chan contracts.DocumentResult)
	suite.outputMap[contracts.MessageGatewayService] = suite.documentResultChan

	workerConfigs := utils.LoadProcessorWorkerConfig(contextMock)
	var sessionProcessorWrapper *SessionWorkerProcessorWrapper
	for _, config := range workerConfigs {
		if config.WorkerName == utils.SessionWorkerName {
			sessionProcessorWrapper = NewSessionWorkerProcessorWrapper(contextMock, config).(*SessionWorkerProcessorWrapper)
			break
		}
	}
	suite.sessionWorkerProcessorWrapper = sessionProcessorWrapper
}

func (suite *SessionWorkerProcessorWrapperTestSuite) TestInitialize() {
	err := suite.sessionWorkerProcessorWrapper.Initialize(suite.outputMap)

	assert.Nil(suite.T(), err)
}

func (suite *SessionWorkerProcessorWrapperTestSuite) TestGetName() {
	name := suite.sessionWorkerProcessorWrapper.GetName()
	assert.Equal(suite.T(), utils.SessionProcessor, name)
}

func (suite *SessionWorkerProcessorWrapperTestSuite) TestGetStartWorker() {
	worker := suite.sessionWorkerProcessorWrapper.GetStartWorker()
	assert.Equal(suite.T(), contracts.StartSession, worker)
}
func (suite *SessionWorkerProcessorWrapperTestSuite) TestGetTerminateWorker() {
	worker := suite.sessionWorkerProcessorWrapper.GetTerminateWorker()
	assert.Equal(suite.T(), contracts.TerminateSession, worker)
}

func (suite *SessionWorkerProcessorWrapperTestSuite) TestPushToProcessor() {
	errorCode := suite.sessionWorkerProcessorWrapper.PushToProcessor(sessionDocState)
	assert.Equal(suite.T(), processor.ErrorCode(""), errorCode)
}

func (suite *SessionWorkerProcessorWrapperTestSuite) TestPushToProcessorWithUnsupportedDoc() {
	sessionDocState.DocumentType = contracts.SendCommand
	errorCode := suite.sessionWorkerProcessorWrapper.PushToProcessor(sessionDocState)
	assert.Equal(suite.T(), processor.UnsupportedDocType, errorCode)
}

func (suite *SessionWorkerProcessorWrapperTestSuite) TestListenSessionReply_ShouldNotReceiveMessage_WithEmptyLastPlugin() {
	pluginResults := make(map[string]*contracts.PluginResult)
	pluginResult := contracts.PluginResult{
		PluginName: "Standard_Stream",
		Status:     contracts.ResultStatusInProgress,
	}
	pluginResults["Standard_Stream"] = &pluginResult
	messageId := uuid.NewV4()
	result := contracts.DocumentResult{
		Status:          contracts.ResultStatusInProgress,
		PluginResults:   pluginResults,
		LastPlugin:      "",
		MessageID:       messageId.String(),
		AssociationID:   "",
		NPlugins:        1,
		DocumentName:    "documentName",
		DocumentVersion: "1",
	}

	go suite.sessionWorkerProcessorWrapper.listenSessionReply(suite.documentResultChan, suite.outputMap)
	suite.documentResultChan <- result
	select {
	case <-suite.documentResultChan:
		assert.Fail(suite.T(), "should not receive message")
	case <-time.After(100 * time.Millisecond):
		assert.True(suite.T(), true, "channel should not receive message")
	}
}

func (suite *SessionWorkerProcessorWrapperTestSuite) TestListenSessionReply_ShouldNotReceiveMessage_WithNonEmptyLastPlugin() {
	pluginResults := make(map[string]*contracts.PluginResult)
	pluginResult := contracts.PluginResult{
		PluginName: "Standard_Stream",
		Status:     contracts.ResultStatusInProgress,
	}
	pluginResults["Standard_Stream"] = &pluginResult
	messageId := uuid.NewV4()
	result := contracts.DocumentResult{
		Status:          contracts.ResultStatusInProgress,
		PluginResults:   pluginResults,
		LastPlugin:      "not empty",
		MessageID:       messageId.String(),
		AssociationID:   "",
		NPlugins:        1,
		DocumentName:    "documentName",
		DocumentVersion: "1",
	}

	go suite.sessionWorkerProcessorWrapper.listenSessionReply(suite.documentResultChan, suite.outputMap)
	suite.documentResultChan <- result
	select {
	case <-suite.documentResultChan:
		assert.True(suite.T(), true, "message should be passed")
	case <-time.After(100 * time.Millisecond):
		assert.Fail(suite.T(), "message should have been passed")
	}
}

// Execute the test suite
func TestSessionWorkerProcessorWrapperTestSuite(t *testing.T) {
	suite.Run(t, new(SessionWorkerProcessorWrapperTestSuite))
}
