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

// Package messageservice implements the core module to start MDS and MGS connections
package messagehandler

import (
	"testing"

	"github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/aws/amazon-ssm-agent/agent/contracts"
	"github.com/aws/amazon-ssm-agent/agent/messageservice/messagehandler/processorwrappers"
	"github.com/aws/amazon-ssm-agent/agent/messageservice/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
)

var (
	docInfo = contracts.DocumentInfo{
		CreatedDate:  "2017-06-10T01-23-07.853Z",
		CommandID:    "13e8e6ad-e195-4ccb-86ee-328153b0dafe",
		MessageID:    "MessageID",
		DocumentName: "AWS-RunPowerShellScript",
		InstanceID:   "i-400e1090",
		RunCount:     0,
	}

	docState = &contracts.DocumentState{
		DocumentInformation: docInfo,
		DocumentType:        contracts.StartSession,
	}
)

type MessageHandlerTestSuite struct {
	suite.Suite
	mockContext                    *context.Mock
	messagehandler                 IMessageHandler
	mockIncomingMessageChan        chan contracts.DocumentState
	commmandWorkerProcessorWrapper processorwrappers.IProcessorWrapper
	sessionWorkerProcessorWrapper  processorwrappers.IProcessorWrapper
	mockReplyChan                  chan contracts.DocumentResult
	mockReplyMap                   map[contracts.UpstreamServiceName]chan contracts.DocumentResult
	mockDocTypeProcessorFuncMap    map[contracts.DocumentType]processorwrappers.IProcessorWrapper
	mockProcessorsLoaded           map[utils.ProcessorName]processorwrappers.IProcessorWrapper
}

func (suite *MessageHandlerTestSuite) SetupTest() {
	mockContext := context.NewMockDefault()

	mockIncomingMessageChan := make(chan contracts.DocumentState)
	mockReplyChan := make(chan contracts.DocumentResult)
	workerConfigs := utils.LoadProcessorWorkerConfig(mockContext)
	for _, config := range workerConfigs {
		if config.WorkerName == utils.DocumentWorkerName {
			suite.commmandWorkerProcessorWrapper = processorwrappers.NewCommandWorkerProcessorWrapper(mockContext, config)
		}
		if config.WorkerName == utils.SessionWorkerName {
			suite.sessionWorkerProcessorWrapper = processorwrappers.NewSessionWorkerProcessorWrapper(mockContext, config)
		}
	}
	suite.mockReplyMap = make(map[contracts.UpstreamServiceName]chan contracts.DocumentResult)
	suite.mockDocTypeProcessorFuncMap = make(map[contracts.DocumentType]processorwrappers.IProcessorWrapper)
	suite.mockProcessorsLoaded = make(map[utils.ProcessorName]processorwrappers.IProcessorWrapper)
	suite.mockContext = mockContext
	suite.mockIncomingMessageChan = mockIncomingMessageChan
	suite.messagehandler = &MessageHandler{
		name:                    Name,
		context:                 mockContext,
		replyMap:                suite.mockReplyMap,
		docTypeProcessorFuncMap: suite.mockDocTypeProcessorFuncMap,
		processorsLoaded:        suite.mockProcessorsLoaded,
	}

	suite.mockReplyChan = mockReplyChan
}

func (suite *MessageHandlerTestSuite) TestGetName() {
	rst := suite.messagehandler.GetName()
	assert.Equal(suite.T(), rst, Name)
}

func (suite *MessageHandlerTestSuite) TestInitializeAndRegisterProcessor() {
	cmdErr := suite.messagehandler.InitializeAndRegisterProcessor(suite.commmandWorkerProcessorWrapper)
	sessErr := suite.messagehandler.InitializeAndRegisterProcessor(suite.sessionWorkerProcessorWrapper)

	assert.Nil(suite.T(), cmdErr)
	assert.Nil(suite.T(), sessErr)
}

func (suite *MessageHandlerTestSuite) TestInitialize() {
	err := suite.messagehandler.Initialize()

	assert.Nil(suite.T(), err)
}

func (suite *MessageHandlerTestSuite) TestSubmit() {
	suite.messagehandler.InitializeAndRegisterProcessor(suite.sessionWorkerProcessorWrapper)
	suite.messagehandler.Initialize()
	errCode := suite.messagehandler.Submit(docState)

	assert.Equal(suite.T(), ErrorCode(""), errCode)
}

func (suite *MessageHandlerTestSuite) TestSubmitWithWrongDocumentType() {
	suite.messagehandler.InitializeAndRegisterProcessor(suite.sessionWorkerProcessorWrapper)
	suite.messagehandler.Initialize()
	docState.DocumentType = "wrong"
	errCode := suite.messagehandler.Submit(docState)

	assert.Equal(suite.T(), UnexpectedDocumentType, errCode)
}

func (suite *MessageHandlerTestSuite) TestRegisterReply() {
	suite.messagehandler.RegisterReply(contracts.MessageGatewayService, suite.mockReplyChan)

	assert.Equal(suite.T(), suite.mockReplyChan, suite.mockReplyMap[contracts.MessageGatewayService])

}

func (suite *MessageHandlerTestSuite) TestStops() {
	suite.messagehandler.Initialize()
	err := suite.messagehandler.Stop()

	assert.Nil(suite.T(), err)

	suite.messagehandler.Initialize()
	err = suite.messagehandler.Stop()

	assert.Nil(suite.T(), err)
}

func TestMessageHandlerTestSuite(t *testing.T) {
	suite.Run(t, new(MessageHandlerTestSuite))
}
