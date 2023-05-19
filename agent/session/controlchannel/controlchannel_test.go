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

// controlchannel package implement control communicator for web socket connection.
package controlchannel

import (
	"encoding/json"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/aws/amazon-ssm-agent/agent/contracts"
	"github.com/aws/amazon-ssm-agent/agent/mocks/context"
	"github.com/aws/amazon-ssm-agent/agent/mocks/log"
	communicatorMocks "github.com/aws/amazon-ssm-agent/agent/session/communicator/mocks"
	mgsConfig "github.com/aws/amazon-ssm-agent/agent/session/config"
	mgsContracts "github.com/aws/amazon-ssm-agent/agent/session/contracts"
	"github.com/aws/amazon-ssm-agent/agent/session/service"
	serviceMock "github.com/aws/amazon-ssm-agent/agent/session/service/mocks"
	eventlogMock "github.com/aws/amazon-ssm-agent/agent/session/telemetry/mocks"
	"github.com/aws/aws-sdk-go/aws/credentials"
	v4 "github.com/aws/aws-sdk-go/aws/signer/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/twinj/uuid"
)

var (
	mockContext                         = context.NewMockDefault()
	mockLog                             = log.NewMockLog()
	mockService                         = &serviceMock.Service{}
	mockWsChannel                       = &communicatorMocks.IWebSocketChannel{}
	mockEventLog                        = eventlogMock.IAuditLogTelemetry{}
	messageId                           = "dd01e56b-ff48-483e-a508-b5f073f31b16"
	mockTaskAckChan                     = make(chan mgsContracts.AcknowledgeTaskContent)
	mockAgentMessageIncomingMessageChan = make(chan mgsContracts.AgentMessage, 10)
	schemaVersion                       = uint32(1)
	createdDate                         = uint64(1503434274948)
	topic                               = "test"
	taskId                              = "2b196342-d7d4-436e-8f09-3883a1116ac3"
	instanceId                          = "i-1234"
	token                               = "token"
	region                              = "us-east-1"
	signer                              = &v4.Signer{Credentials: credentials.NewStaticCredentials("AKID", "SECRET", "SESSION")}
)

func TestInitialize(t *testing.T) {
	controlChannel := &ControlChannel{}
	controlChannel.AuditLogScheduler = &mockEventLog
	controlChannel.Initialize(mockContext, mockService, instanceId, mockAgentMessageIncomingMessageChan)

	assert.Equal(t, instanceId, controlChannel.ChannelId)
	assert.Equal(t, mockAgentMessageIncomingMessageChan, controlChannel.agentMessageIncomingMessageChan)
	assert.Equal(t, mockService, controlChannel.Service)
	assert.Equal(t, mgsConfig.RoleSubscribe, controlChannel.channelType)
	assert.NotNil(t, controlChannel.wsChannel)
}

func TestSetWebSocket(t *testing.T) {
	controlChannel := getControlChannel()
	createControlChannelOutput := service.CreateControlChannelOutput{TokenValue: &token}
	mockService.On("CreateControlChannel", mock.Anything, mock.Anything, mock.AnythingOfType("string")).Return(&createControlChannelOutput, nil)
	mockService.On("GetRegion").Return(region)
	mockService.On("GetV4Signer").Return(signer)
	mockWsChannel.On("Initialize",
		mock.Anything,
		mock.Anything,
		mock.Anything,
		mock.Anything,
		mock.Anything,
		mock.Anything,
		mock.Anything,
		mock.Anything,
		mock.Anything).Return(nil)

	var ableToOpenMGSConnection uint32
	err := controlChannel.SetWebSocket(mockContext, mockService, &ableToOpenMGSConnection)

	assert.Nil(t, err)
	mockWsChannel.AssertExpectations(t)
	mockService.AssertExpectations(t)
}

func TestOpen(t *testing.T) {
	controlChannel := getControlChannel()

	mockWsChannel.On("Open", mock.Anything, mock.Anything).Return(nil)
	mockWsChannel.On("GetChannelToken").Return(token)
	mockWsChannel.On("SendMessage", mock.Anything, mock.Anything, mock.Anything).Return(nil)
	mockEventLog.On("SendAuditMessage")

	// test open (includes SendMessage)
	var ableToOpenMGSConnection uint32
	err := controlChannel.Open(mockLog, &ableToOpenMGSConnection)

	assert.Nil(t, err)
	assert.Equal(t, token, controlChannel.wsChannel.GetChannelToken())
	mockWsChannel.AssertExpectations(t)
}

func TestOpenHandlesNilAbleToOpenMGSConnection(t *testing.T) {
	controlChannel := getControlChannel()

	mockWsChannel.On("Open", mock.Anything, mock.Anything).Return(nil)
	mockWsChannel.On("GetChannelToken").Return(token)
	mockWsChannel.On("SendMessage", mock.Anything, mock.Anything, mock.Anything).Return(nil)
	mockEventLog.On("SendAuditMessage")

	var ableToOpenMGSConnection *uint32 = nil
	err := controlChannel.Open(mockLog, ableToOpenMGSConnection)

	assert.Nil(t, err)
	assert.Equal(t, token, controlChannel.wsChannel.GetChannelToken())
	mockWsChannel.AssertExpectations(t)
}

func TestOpenReportsBadMGSConnectionStatusIfChannelCannotBeOpened(t *testing.T) {
	mockWsChannelForChannelCannotBeOpenedTest := &communicatorMocks.IWebSocketChannel{}
	controlChannel := getControlChannel()

	mockWsChannelForChannelCannotBeOpenedTest.On("Open", mock.Anything, mock.Anything).Return(errors.New(""))

	controlChannel.wsChannel = mockWsChannelForChannelCannotBeOpenedTest
	var ableToOpenMGSConnection uint32
	atomic.StoreUint32(&ableToOpenMGSConnection, 1)
	controlChannel.Open(mockLog, &ableToOpenMGSConnection)

	mockWsChannel.AssertExpectations(t)
	assert.False(t, atomic.LoadUint32(&ableToOpenMGSConnection) != 0)
}

func TestGetControlChannelTokenReportsBadMGSConnectionStatusIfCannotGetToken(t *testing.T) {
	mockServiceForGetControlChannelTest := &serviceMock.Service{}
	mockServiceForGetControlChannelTest.On("CreateControlChannel", mock.Anything, mock.Anything, mock.Anything).Return(nil, errors.New(""))

	var ableToOpenMGSConnection uint32
	atomic.StoreUint32(&ableToOpenMGSConnection, 1)
	getControlChannelToken(mockLog, mockServiceForGetControlChannelTest, mock.Anything, uuid.NewV4().String(), &ableToOpenMGSConnection)
	assert.False(t, atomic.LoadUint32(&ableToOpenMGSConnection) != 0)
}

func TestGetControlChannelTokenHandlesNilAbleToOpenMGSConnection(t *testing.T) {
	mockServiceForGetControlChannelTest := &serviceMock.Service{}
	mockServiceForGetControlChannelTest.On("CreateControlChannel", mock.Anything, mock.Anything, mock.Anything).Return(nil, errors.New(""))

	var ableToOpenMGSConnection *uint32 = nil
	getControlChannelToken(mockLog, mockServiceForGetControlChannelTest, mock.Anything, uuid.NewV4().String(), ableToOpenMGSConnection)
	mockServiceForGetControlChannelTest.AssertCalled(t, "CreateControlChannel", mock.Anything, mock.Anything, mock.Anything)
}

func TestReconnect(t *testing.T) {
	controlChannel := getControlChannel()

	mockWsChannel.On("Close", mock.Anything).Return(nil)
	mockWsChannel.On("Open", mock.Anything, mock.Anything).Return(nil)
	mockWsChannel.On("GetChannelToken").Return(token)
	mockWsChannel.On("SendMessage", mock.Anything, mock.Anything, mock.Anything).Return(nil)

	// test reconnect
	var ableToOpenMGSConnection uint32
	err := controlChannel.Reconnect(mockLog, &ableToOpenMGSConnection)

	assert.Nil(t, err)
	assert.Equal(t, token, controlChannel.wsChannel.GetChannelToken())
	mockWsChannel.AssertExpectations(t)
}

func TestReconnectReportsHealthyMGSConnectionIfSuccessful(t *testing.T) {
	controlChannel := getControlChannel()

	mockWsChannel.On("Close", mock.Anything).Return(nil)
	mockWsChannel.On("Open", mock.Anything, mock.Anything).Return(nil)
	mockWsChannel.On("GetChannelToken").Return(token)
	mockWsChannel.On("SendMessage", mock.Anything, mock.Anything, mock.Anything).Return(nil)

	// test reconnect
	var ableToOpenMGSConnection uint32
	err := controlChannel.Reconnect(mockLog, &ableToOpenMGSConnection)

	assert.Nil(t, err)
	assert.Equal(t, token, controlChannel.wsChannel.GetChannelToken())
	assert.True(t, atomic.LoadUint32(&ableToOpenMGSConnection) != 0)
	mockWsChannel.AssertExpectations(t)
}

func TestReconnectHandlesNilAbleToOpenMGSConnection(t *testing.T) {
	controlChannel := getControlChannel()

	mockWsChannel.On("Close", mock.Anything).Return(nil)
	mockWsChannel.On("Open", mock.Anything, mock.Anything).Return(nil)
	mockWsChannel.On("GetChannelToken").Return(token)
	mockWsChannel.On("SendMessage", mock.Anything, mock.Anything, mock.Anything).Return(nil)

	// test reconnect
	var ableToOpenMGSConnection *uint32 = nil
	err := controlChannel.Reconnect(mockLog, ableToOpenMGSConnection)

	assert.Nil(t, err)
	assert.Equal(t, token, controlChannel.wsChannel.GetChannelToken())
	mockWsChannel.AssertExpectations(t)
}

func TestClose(t *testing.T) {
	controlChannel := getControlChannel()
	mockWsChannel.On("Close", mock.Anything).Return(nil)
	mockEventLog.On("StopScheduler")

	// test close
	err := controlChannel.Close(mockLog)
	assert.Nil(t, err)
	mockWsChannel.AssertExpectations(t)
}

func TestCloseWhenControlChannelDoesNotExist(t *testing.T) {
	controlChannel := &ControlChannel{}
	mockWsChannel.On("Close", mock.Anything).Times(0)

	// test close
	err := controlChannel.Close(mockLog)

	assert.Nil(t, err)
	mockWsChannel.AssertExpectations(t)
}

func TestControlChannelIncomingMessageHandlerForStartSessionMessage(t *testing.T) {
	u, _ := uuid.Parse(messageId)
	agentJson := "{\"DataChannelId\":\"44da928d-1200-4501-a38a-f10d72e38cc4\",\"documentContent\":{\"schemaVersion\":\"1.0\"," +
		"\"inputs\":{\"cloudWatchLogGroup\":\"\",\"s3BucketName\":\"\",\"s3KeyPrefix\":\"\"},\"description\":\"Document to hold " +
		"regional settings for Session Manager\",\"sessionType\":\"Standard_Stream\",\"parameters\":{}},\"sessionId\":\"44da928d-1200-4501-a38a-f10d72e38cc4\"," +
		"\"DataChannelToken\":\"token\"}"
	mgsPayload := mgsContracts.MGSPayload{
		Payload:       string(agentJson),
		TaskId:        taskId,
		Topic:         topic,
		SchemaVersion: 1,
	}
	mgsPayloadJson, _ := json.Marshal(mgsPayload)
	agentMessage := &mgsContracts.AgentMessage{
		MessageType:    mgsContracts.InteractiveShellMessage,
		SchemaVersion:  schemaVersion,
		CreatedDate:    createdDate,
		SequenceNumber: 1,
		Flags:          2,
		MessageId:      u,
		Payload:        mgsPayloadJson,
	}
	serializedBytes, _ := agentMessage.Serialize(log.NewMockLog())

	err := controlChannelIncomingMessageHandler(mockContext, serializedBytes, mockAgentMessageIncomingMessageChan)

	assert.Nil(t, err)
}

func TestControlChannelIncomingMessageHandlerForAgentJobSendCommandMessage(t *testing.T) {
	u, _ := uuid.Parse(messageId)

	agentJSON := "{\"Parameters\":{\"workingDirectory\":\"\",\"runCommand\":[\"echo hello; sleep 10\"]},\"DocumentContent\":{\"schemaVersion\":\"1.2\",\"description\":\"This document defines the PowerShell command to run or path to a script which is to be executed.\",\"runtimeConfig\":{\"aws:runScript\":{\"properties\":[{\"workingDirectory\":\"{{ workingDirectory }}\",\"timeoutSeconds\":\"{{ timeoutSeconds }}\",\"runCommand\":\"{{ runCommand }}\",\"id\":\"0.aws:runScript\"}]}},\"parameters\":{\"workingDirectory\":{\"default\":\"\",\"description\":\"Path to the working directory (Optional)\",\"type\":\"String\"},\"timeoutSeconds\":{\"default\":\"\",\"description\":\"Timeout in seconds (Optional)\",\"type\":\"String\"},\"runCommand\":{\"description\":\"List of commands to run (Required)\",\"type\":\"Array\"}}},\"CommandId\":\"55b78ece-7a7f-4198-aaf4-d8c8a3e960e6\",\"DocumentName\":\"AWS-RunPowerShellScript\",\"CloudWatchOutputEnabled\":\"true\"}"

	agentJobPayload := mgsContracts.AgentJobPayload{
		Payload:       string(agentJSON),
		JobId:         taskId,
		Topic:         string(contracts.SendCommand),
		SchemaVersion: 1,
	}
	agentJobPayloadJson, _ := json.Marshal(agentJobPayload)
	agentMessage := &mgsContracts.AgentMessage{
		MessageType:    mgsContracts.AgentJobMessage,
		SchemaVersion:  schemaVersion,
		CreatedDate:    createdDate,
		SequenceNumber: 1,
		Flags:          2,
		MessageId:      u,
		Payload:        agentJobPayloadJson,
	}
	serializedBytes, _ := agentMessage.Serialize(log.NewMockLog())

	err := controlChannelIncomingMessageHandler(mockContext, serializedBytes, mockAgentMessageIncomingMessageChan)

	assert.Nil(t, err)
}

func TestControlChannelIncomingMessageHandlerForAgentJobCancelCommandMessage(t *testing.T) {
	u, _ := uuid.Parse(messageId)

	agentJSON := "{\"CancelMessageId\":\"55b78ece-7a7f-4198-aaf4-d8c8a3e960e6\"}"

	agentJobPayload := mgsContracts.AgentJobPayload{
		Payload:       string(agentJSON),
		JobId:         taskId,
		Topic:         string(contracts.CancelCommand),
		SchemaVersion: 1,
	}
	agentJobPayloadJson, _ := json.Marshal(agentJobPayload)
	agentMessage := &mgsContracts.AgentMessage{
		MessageType:    mgsContracts.AgentJobMessage,
		SchemaVersion:  schemaVersion,
		CreatedDate:    createdDate,
		SequenceNumber: 1,
		Flags:          2,
		MessageId:      u,
		Payload:        agentJobPayloadJson,
	}
	serializedBytes, _ := agentMessage.Serialize(log.NewMockLog())

	err := controlChannelIncomingMessageHandler(mockContext, serializedBytes, mockAgentMessageIncomingMessageChan)

	assert.Nil(t, err)
}

func TestControlChannelIncomingMessageHandlerForTerminateSessionMessage(t *testing.T) {
	u, _ := uuid.Parse(messageId)
	agentJson := "{\"MessageType\":\"channel_closed\"," +
		"\"MessageId\":\"44dd928d-1200-4501-a38a-f10d72e38cc4\"," +
		"\"DestinationId\":\"33dd928d-1200-4501-a38a-f10d72e38cc4\"," +
		"\"CreatedDate\":\"created-date\"," +
		"\"SessionId\":\"44da928d-1200-4501-a38a-f10d72e38cc4\"}"
	agentMessage := &mgsContracts.AgentMessage{
		MessageType:    mgsContracts.ChannelClosedMessage,
		SchemaVersion:  schemaVersion,
		CreatedDate:    createdDate,
		SequenceNumber: 1,
		Flags:          2,
		MessageId:      u,
		Payload:        []byte(agentJson),
	}
	serializedBytes, _ := agentMessage.Serialize(log.NewMockLog())
	err := controlChannelIncomingMessageHandler(mockContext, serializedBytes, mockAgentMessageIncomingMessageChan)

	assert.Nil(t, err)
}

func TestControlChannelIncomingMessageHandlerForTaskAcknowledgeMessage(t *testing.T) {
	u, _ := uuid.Parse(messageId)
	msg := &mgsContracts.AcknowledgeTaskContent{
		MessageId: messageId,
		TaskId:    messageId,
		Topic:     mgsContracts.TaskCompleteMessage,
	}

	payload, _ := msg.Serialize(log.NewMockLog())
	agentMessage := &mgsContracts.AgentMessage{
		MessageType:    mgsContracts.TaskAcknowledgeMessage,
		SchemaVersion:  schemaVersion,
		CreatedDate:    createdDate,
		SequenceNumber: 1,
		Flags:          2,
		MessageId:      u,
		Payload:        payload,
	}
	serializedBytes, _ := agentMessage.Serialize(log.NewMockLog())

	go func() {
		select {
		case <-mockTaskAckChan:
			break
		case <-time.After(10 * time.Millisecond):
			assert.Fail(t, "Channel should have the message")
		}
		close(mockTaskAckChan)
	}()

	err := controlChannelIncomingMessageHandler(mockContext, serializedBytes, mockAgentMessageIncomingMessageChan)
	assert.Nil(t, err)
}

func TestInitializeForContainer(t *testing.T) {
	controlChannel := getControlChannel()
	mockInstanceId := "9781c2480-edd4cdb9a93f6-3cb662a5d3"
	controlChannel.Initialize(mockContext, mockService, mockInstanceId, mockAgentMessageIncomingMessageChan)

	assert.Equal(t, mockInstanceId, controlChannel.ChannelId)
	assert.Equal(t, mockService, controlChannel.Service)
	assert.Equal(t, mgsConfig.RoleSubscribe, controlChannel.channelType)
	assert.NotNil(t, controlChannel.wsChannel)
}

func getControlChannel() *ControlChannel {
	return &ControlChannel{
		wsChannel:                       mockWsChannel,
		Service:                         mockService,
		ChannelId:                       instanceId,
		AuditLogScheduler:               &mockEventLog,
		channelType:                     mgsConfig.RoleSubscribe,
		agentMessageIncomingMessageChan: mockAgentMessageIncomingMessageChan,
		context:                         mockContext,
	}
}
