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

// Package mgsinteractor contains logic to open control channel and communicate with MGS
package mgsinteractor

import (
	"encoding/json"
	"github.com/aws/amazon-ssm-agent/agent/contracts"
	messageHandler "github.com/aws/amazon-ssm-agent/agent/messageservice/messagehandler"
	"github.com/aws/amazon-ssm-agent/agent/ssmconnectionchannel"
	"sync/atomic"
	"testing"
	"time"

	"github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/aws/amazon-ssm-agent/agent/messageservice/messagehandler/mocks"
	contextmocks "github.com/aws/amazon-ssm-agent/agent/mocks/context"
	mgsConfig "github.com/aws/amazon-ssm-agent/agent/session/config"
	mgsContracts "github.com/aws/amazon-ssm-agent/agent/session/contracts"
	"github.com/aws/amazon-ssm-agent/agent/session/controlchannel"
	controlChannelMock "github.com/aws/amazon-ssm-agent/agent/session/controlchannel/mocks"
	"github.com/aws/amazon-ssm-agent/agent/session/service"
	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
	"github.com/twinj/uuid"
)

var (
	instanceId    = "i-1234"
	messageId     = "dd01e56b-ff48-483e-a508-b5f073f31b16"
	taskId        = "2b196342-d7d4-436e-8f09-3883a1116ac3"
	topic         = "test"
	schemaVersion = uint32(1)
	createdDate   = uint64(1503434274948)
	errorMsg      = "plugin failed"
	s3Bucket      = "s3Bucket"
	s3UrlSuffix   = "s3UrlSuffix"
	cwlGroup      = "cwlGroup"
	cwlStream     = "cwlStream"
)

type MGSInteractorTestSuite struct {
	suite.Suite
}

// Testing the module execute
func (suite *MGSInteractorTestSuite) TestInitialize() {

	mockContext := contextmocks.NewMockDefault()
	messageHandlerMock := &mocks.IMessageHandler{}
	messageHandlerMock.On("RegisterReply", mock.Anything, mock.Anything)
	mgsInteractorRef, err := New(mockContext, messageHandlerMock)
	assert.Nil(suite.T(), err, "initialize passed")
	mgsInteractor := mgsInteractorRef.(*MGSInteractor)
	defer func() {
		close(mgsInteractor.incomingAgentMessageChan)
		close(mgsInteractor.replyChan)
		close(mgsInteractor.sendReplyProp.reply)
		close(mgsInteractor.updateWatcherDone)
	}()
	mockControlChannel := &controlChannelMock.IControlChannel{}
	mockControlChannel.On("SendMessage", mock.Anything, mock.Anything, websocket.BinaryMessage).Return(nil)

	setupControlChannel = func(context context.T, mgsService service.Service, instanceId string, agentMessageIncomingMessageChan chan mgsContracts.AgentMessage, ableToOpenMGSConnection *uint32) (controlchannel.IControlChannel, error) {
		return mockControlChannel, nil
	}

	var ableToOpenMGSConnection uint32
	mgsInteractor.Initialize(&ableToOpenMGSConnection)
	assert.True(suite.T(), true, "initialize passed")
}

func (suite *MGSInteractorTestSuite) TestInitializeHandlesNilAbleToOpenMGSConnection() {

	mockContext := contextmocks.NewMockDefault()
	messageHandlerMock := &mocks.IMessageHandler{}
	messageHandlerMock.On("RegisterReply", mock.Anything, mock.Anything)
	mgsInteractorRef, err := New(mockContext, messageHandlerMock)
	assert.Nil(suite.T(), err, "initialize passed")
	mgsInteractor := mgsInteractorRef.(*MGSInteractor)
	defer func() {
		close(mgsInteractor.incomingAgentMessageChan)
		close(mgsInteractor.replyChan)
		close(mgsInteractor.sendReplyProp.reply)
		close(mgsInteractor.updateWatcherDone)
	}()
	mockControlChannel := &controlChannelMock.IControlChannel{}
	mockControlChannel.On("SendMessage", mock.Anything, mock.Anything, websocket.BinaryMessage).Return(nil)

	setupControlChannel = func(context context.T, mgsService service.Service, instanceId string, agentMessageIncomingMessageChan chan mgsContracts.AgentMessage, ableToOpenMGSConnection *uint32) (controlchannel.IControlChannel, error) {
		return mockControlChannel, nil
	}

	var ableToOpenMGSConnection *uint32 = nil
	mgsInteractor.Initialize(ableToOpenMGSConnection)
	assert.True(suite.T(), true, "initialize passed")
}

func (suite *MGSInteractorTestSuite) TestInitializeReportsHealthyMGSConnectionIfControlChannelOpened() {
	mockContext := contextmocks.NewMockDefault()
	messageHandlerMock := &mocks.IMessageHandler{}
	messageHandlerMock.On("RegisterReply", mock.Anything, mock.Anything)
	mgsInteractorRef, err := New(mockContext, messageHandlerMock)
	assert.Nil(suite.T(), err, "initialize passed")
	mgsInteractor := mgsInteractorRef.(*MGSInteractor)

	mockControlChannel := &controlChannelMock.IControlChannel{}
	setupControlChannel = func(context context.T, mgsService service.Service, instanceId string, agentMessageIncomingMessageChan chan mgsContracts.AgentMessage, ableToOpenMGSConnection *uint32) (controlchannel.IControlChannel, error) {
		return mockControlChannel, nil
	}

	var ableToOpenMGSConnection uint32
	mgsInteractor.Initialize(&ableToOpenMGSConnection)
	assert.True(suite.T(), atomic.LoadUint32(&ableToOpenMGSConnection) != 0)
	assert.Equal(suite.T(), contracts.MGS, ssmconnectionchannel.GetConnectionChannel())
}

func (suite *MGSInteractorTestSuite) TestListenTaskAcknowledgeMsgDoesExist() {
	mockContext := contextmocks.NewMockDefault()
	messageHandlerMock := &mocks.IMessageHandler{}
	messageHandlerMock.On("RegisterReply", mock.Anything, mock.Anything)
	mgsInteractorRef, err := New(mockContext, messageHandlerMock)
	assert.Nil(suite.T(), err, "initialize passed")
	mgsInteractor := mgsInteractorRef.(*MGSInteractor)
	ackChan := make(chan bool, 1)
	mgsInteractor.sendReplyProp.replyAckChan.Store(messageId, ackChan)
	msg := mgsContracts.AcknowledgeTaskContent{
		MessageId: messageId,
		Topic:     mgsContracts.TaskCompleteMessage,
	}
	ackByte, err := json.Marshal(msg)
	assert.Nil(suite.T(), err)
	agentMessage := mgsContracts.AgentMessage{
		MessageId:   uuid.NewV4(),
		Payload:     ackByte,
		MessageType: mgsContracts.TaskAcknowledgeMessage,
	}
	mgsInteractor.processTaskAcknowledgeMessage(agentMessage)
	outputVal := <-ackChan
	assert.True(suite.T(), outputVal, "received wrong ack")
}

func (suite *MGSInteractorTestSuite) TestListenTaskAcknowledgeMsgDoesNotExist() {
	mockContext := contextmocks.NewMockDefault()
	messageHandlerMock := &mocks.IMessageHandler{}
	messageHandlerMock.On("RegisterReply", mock.Anything, mock.Anything)
	mgsInteractorRef, err := New(mockContext, messageHandlerMock)
	assert.Nil(suite.T(), err, "initialize passed")
	mgsInteractor := mgsInteractorRef.(*MGSInteractor)
	ackChan := make(chan bool, 1)
	mgsInteractor.sendReplyProp.replyAckChan.Store(messageId, ackChan)
	msg := mgsContracts.AcknowledgeTaskContent{
		MessageId: uuid.NewV4().String(), // generate random one
		Topic:     mgsContracts.TaskCompleteMessage,
	}
	ackByte, err := json.Marshal(msg)
	assert.Nil(suite.T(), err)
	agentMessage := mgsContracts.AgentMessage{
		MessageId:   uuid.NewV4(),
		Payload:     ackByte,
		MessageType: mgsContracts.TaskAcknowledgeMessage,
	}
	mgsInteractor.processTaskAcknowledgeMessage(agentMessage)
	var outputVal bool
	select {
	case outputVal = <-ackChan:
		break
	case <-time.After(100 * time.Millisecond):
	}
	assert.False(suite.T(), outputVal, "should not receive ack")
}

func (suite *MGSInteractorTestSuite) TestModuleStopClosingAlreadyClosedChannel() {
	mockContext := contextmocks.NewMockDefault()
	messageHandlerMock := &mocks.IMessageHandler{}
	messageHandlerMock.On("RegisterReply", mock.Anything, mock.Anything)
	mgsInteractorRef, err := New(mockContext, messageHandlerMock)
	assert.Nil(suite.T(), err, "initialize passed")
	mgsInteractor := mgsInteractorRef.(*MGSInteractor)
	mgsInteractor.listenReplyThreadEnded = make(chan struct{}, 1)

	mockControlChannel := &controlChannelMock.IControlChannel{}
	mockControlChannel.On("Close", mock.Anything).Return(nil)
	go func() {
		mgsInteractor.listenReplyThreadEnded <- struct{}{}
		mgsInteractor.sendReplyProp.allReplyClosed <- struct{}{}
	}()
	mgsInteractor.Close()
	assert.True(suite.T(), true, "close connection test passed")
}

func (suite *MGSInteractorTestSuite) TestAgentJobSendAcknowledgeWhenMessageHandlerError() {
	mockContext := contextmocks.NewMockDefault()
	messageHandlerMock := &mocks.IMessageHandler{}
	messageHandlerMock.On("RegisterReply", mock.Anything, mock.Anything)
	messageHandlerMock.On("Submit", mock.Anything).Return(messageHandler.ClosedProcessor)
	mgsInteractorRef, err := New(mockContext, messageHandlerMock)
	assert.Nil(suite.T(), err, "initialize passed")
	mgsInteractor := mgsInteractorRef.(*MGSInteractor)
	mgsInteractor.channelOpen = true
	mgsInteractor.ackSkipCodes = map[messageHandler.ErrorCode]string{
		messageHandler.ClosedProcessor: "51401",
	}
	mockControlChannel := &controlChannelMock.IControlChannel{}
	mockControlChannel.On("SendMessage", mock.Anything, mock.Anything, websocket.BinaryMessage).Return(nil)
	mgsInteractor.controlChannel = mockControlChannel
	agentJSON := "{\"Parameters\":{\"workingDirectory\":\"\",\"runCommand\":[\"echo hello; sleep 10\"]},\"DocumentContent\":{\"schemaVersion\":\"1.2\",\"description\":\"This document defines the PowerShell command to run or path to a script which is to be executed.\",\"runtimeConfig\":{\"aws:runScript\":{\"properties\":[{\"workingDirectory\":\"{{ workingDirectory }}\",\"timeoutSeconds\":\"{{ timeoutSeconds }}\",\"runCommand\":\"{{ runCommand }}\",\"id\":\"0.aws:runScript\"}]}},\"parameters\":{\"workingDirectory\":{\"default\":\"\",\"description\":\"Path to the working directory (Optional)\",\"type\":\"String\"},\"timeoutSeconds\":{\"default\":\"\",\"description\":\"Timeout in seconds (Optional)\",\"type\":\"String\"},\"runCommand\":{\"description\":\"List of commands to run (Required)\",\"type\":\"Array\"}}},\"CommandId\":\"55b78ece-7a7f-4198-aaf4-d8c8a3e960e6\",\"DocumentName\":\"AWS-RunPowerShellScript\",\"CloudWatchOutputEnabled\":\"true\"}"

	agentJobPayload := mgsContracts.AgentJobPayload{
		Payload:       agentJSON,
		JobId:         taskId,
		Topic:         "aws.ssm.sendCommand",
		SchemaVersion: 1,
	}
	payload, err := json.Marshal(agentJobPayload)
	assert.Nil(suite.T(), err)
	agentMessage := mgsContracts.AgentMessage{
		HeaderLength:   20,
		MessageType:    mgsContracts.AgentJobMessage,
		SchemaVersion:  schemaVersion,
		CreatedDate:    createdDate,
		SequenceNumber: 1,
		Flags:          2,
		MessageId:      uuid.NewV4(),
		Payload:        payload,
	}
	mgsInteractor.processAgentJobMessage(agentMessage)
	mockControlChannel.AssertNumberOfCalls(suite.T(), "SendMessage", 1)
}

func (suite *MGSInteractorTestSuite) TestAgentJobSendAcknowledgeWhenMessageParsingError() {
	mockContext := contextmocks.NewMockDefault()
	messageHandlerMock := &mocks.IMessageHandler{}
	messageHandlerMock.On("RegisterReply", mock.Anything, mock.Anything)
	messageHandlerMock.On("Submit", mock.Anything).Return(messageHandler.ClosedProcessor)
	mgsInteractorRef, err := New(mockContext, messageHandlerMock)
	assert.Nil(suite.T(), err, "initialize passed")
	mgsInteractor := mgsInteractorRef.(*MGSInteractor)
	mgsInteractor.channelOpen = true
	mockControlChannel := &controlChannelMock.IControlChannel{}
	mockControlChannel.On("SendMessage", mock.Anything, mock.Anything, websocket.BinaryMessage).Return(nil)
	mgsInteractor.controlChannel = mockControlChannel
	agentJSON := "{}"

	agentJobPayload := mgsContracts.AgentJobPayload{
		Payload:       agentJSON,
		JobId:         taskId,
		Topic:         "aws.ssm.sendCommand",
		SchemaVersion: 1,
	}
	payload, err := json.Marshal(agentJobPayload)
	assert.Nil(suite.T(), err)
	agentMessage := mgsContracts.AgentMessage{
		HeaderLength:   20,
		MessageType:    mgsContracts.AgentJobMessage,
		SchemaVersion:  schemaVersion,
		CreatedDate:    createdDate,
		SequenceNumber: 1,
		Flags:          2,
		MessageId:      uuid.NewV4(),
		Payload:        payload,
	}
	mgsInteractor.processAgentJobMessage(agentMessage)
	mockControlChannel.AssertNumberOfCalls(suite.T(), "SendMessage", 1)
}

func (suite *MGSInteractorTestSuite) TestGetMgsEndpoint() {
	// create mock context and log
	contextMock := contextmocks.NewMockDefault()

	mgsConfig.GetMgsEndpoint = func(context context.T, region string) string {
		if region == "us-east-1" {
			return "ssmmessages.us-east-1.amazonaws.com"
		} else if region == "cn-north-1" {
			return "ssmmessages.cn-north-1.amazonaws.com.cn"
		} else {
			return ""
		}
	}

	host, err := getMgsEndpoint(contextMock, "us-east-1")

	assert.Nil(suite.T(), err)
	assert.Equal(suite.T(), "https://ssmmessages.us-east-1.amazonaws.com", host)

	bjsHost, err := getMgsEndpoint(contextMock, "cn-north-1")
	assert.Nil(suite.T(), err)
	assert.Equal(suite.T(), "https://ssmmessages.cn-north-1.amazonaws.com.cn", bjsHost)
}

func (suite *MGSInteractorTestSuite) TestToISO8601() {
	isoTime := toISO8601(createdDate)
	assert.Equal(suite.T(), "2017-08-22T20:37:54.948Z", isoTime)
}

// Execute the test suite
func TestSessionTestSuite(t *testing.T) {
	suite.Run(t, new(MGSInteractorTestSuite))
}
