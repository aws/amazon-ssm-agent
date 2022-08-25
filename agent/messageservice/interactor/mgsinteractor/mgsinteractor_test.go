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
	"testing"
	"time"

	"github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/aws/amazon-ssm-agent/agent/messageservice/messagehandler/mocks"
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

	mockContext := context.NewMockDefault()
	messageHandlerMock := &mocks.IMessageHandler{}
	messageHandlerMock.On("RegisterReply", mock.Anything, mock.Anything)
	mgsInteractorRef, err := New(mockContext, messageHandlerMock)
	assert.Nil(suite.T(), err, "initialize passed")
	mgsInteractor := mgsInteractorRef.(*MGSInteractor)
	defer func() {
		close(mgsInteractor.incomingAgentMessageChan)
		close(mgsInteractor.replyChan)
		close(mgsInteractor.sendReplyProp.reply)
	}()
	mockControlChannel := &controlChannelMock.IControlChannel{}
	mockControlChannel.On("SendMessage", mock.Anything, mock.Anything, websocket.BinaryMessage).Return(nil)

	setupControlChannel = func(context context.T, mgsService service.Service, instanceId string, agentMessageIncomingMessageChan chan mgsContracts.AgentMessage) (controlchannel.IControlChannel, error) {
		return mockControlChannel, nil
	}
	mgsInteractor.Initialize()
	assert.True(suite.T(), true, "initialize passed")
}

func (suite *MGSInteractorTestSuite) TestListenTaskAcknowledgeMsgDoesExist() {
	mockContext := context.NewMockDefault()
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
	mockContext := context.NewMockDefault()
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
	mockContext := context.NewMockDefault()
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

func (suite *MGSInteractorTestSuite) TestGetMgsEndpoint() {
	// create mock context and log
	contextMock := context.NewMockDefault()

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
