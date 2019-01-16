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
	"testing"

	"github.com/aws/amazon-ssm-agent/agent/context"
	processorMock "github.com/aws/amazon-ssm-agent/agent/framework/processor/mock"
	"github.com/aws/amazon-ssm-agent/agent/log"
	communicatorMocks "github.com/aws/amazon-ssm-agent/agent/session/communicator/mocks"
	mgsConfig "github.com/aws/amazon-ssm-agent/agent/session/config"
	mgsContracts "github.com/aws/amazon-ssm-agent/agent/session/contracts"
	"github.com/aws/amazon-ssm-agent/agent/session/service"
	serviceMock "github.com/aws/amazon-ssm-agent/agent/session/service/mocks"
	"github.com/aws/aws-sdk-go/aws/credentials"
	v4 "github.com/aws/aws-sdk-go/aws/signer/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/twinj/uuid"
)

var (
	mockContext   = context.NewMockDefault()
	mockLog       = log.NewMockLog()
	mockProcessor = new(processorMock.MockedProcessor)
	mockService   = &serviceMock.Service{}
	mockWsChannel = &communicatorMocks.IWebSocketChannel{}
	messageId     = "dd01e56b-ff48-483e-a508-b5f073f31b16"
	schemaVersion = uint32(1)
	createdDate   = uint64(1503434274948)
	topic         = "test"
	taskId        = "2b196342-d7d4-436e-8f09-3883a1116ac3"
	instanceId    = "i-1234"
	token         = "token"
	region        = "us-east-1"
	signer        = &v4.Signer{Credentials: credentials.NewStaticCredentials("AKID", "SECRET", "SESSION")}
)

func TestInitialize(t *testing.T) {
	controlChannel := &ControlChannel{}
	controlChannel.Initialize(mockContext, mockService, mockProcessor, instanceId)

	assert.Equal(t, instanceId, controlChannel.ChannelId)
	assert.Equal(t, mockService, controlChannel.Service)
	assert.Equal(t, mgsConfig.RoleSubscribe, controlChannel.channelType)
	assert.Equal(t, mockProcessor, controlChannel.Processor)
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

	err := controlChannel.SetWebSocket(mockContext, mockService, mockProcessor, instanceId)

	assert.Nil(t, err)
	mockWsChannel.AssertExpectations(t)
	mockService.AssertExpectations(t)
}

func TestOpen(t *testing.T) {
	controlChannel := getControlChannel()

	mockWsChannel.On("Open", mock.Anything).Return(nil)
	mockWsChannel.On("GetChannelToken").Return(token)
	mockWsChannel.On("SendMessage", mock.Anything, mock.Anything, mock.Anything).Return(nil)

	// test open (includes SendMessage)
	err := controlChannel.Open(mockLog)

	assert.Nil(t, err)
	assert.Equal(t, token, controlChannel.wsChannel.GetChannelToken())
	mockWsChannel.AssertExpectations(t)
}

func TestReconnect(t *testing.T) {
	controlChannel := getControlChannel()

	mockWsChannel.On("Close", mock.Anything).Return(nil)
	mockWsChannel.On("Open", mock.Anything).Return(nil)
	mockWsChannel.On("GetChannelToken").Return(token)
	mockWsChannel.On("SendMessage", mock.Anything, mock.Anything, mock.Anything).Return(nil)

	// test reconnect
	err := controlChannel.Reconnect(mockLog)

	assert.Nil(t, err)
	assert.Equal(t, token, controlChannel.wsChannel.GetChannelToken())
	mockWsChannel.AssertExpectations(t)
}

func TestClose(t *testing.T) {
	controlChannel := getControlChannel()
	mockWsChannel.On("Close", mock.Anything).Return(nil)

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
	mockProcessor.On("Submit", mock.Anything).Return(nil)

	err := controlChannelIncomingMessageHandler(mockContext, mockProcessor, serializedBytes, "", "")

	assert.Nil(t, err)
	mockProcessor.AssertExpectations(t)
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
	mockProcessor.On("Cancel", mock.Anything).Return(nil)

	err := controlChannelIncomingMessageHandler(mockContext, mockProcessor, serializedBytes, "", "")

	assert.Nil(t, err)
	mockProcessor.AssertExpectations(t)
}

func getControlChannel() *ControlChannel {
	return &ControlChannel{
		wsChannel:   mockWsChannel,
		Service:     mockService,
		ChannelId:   instanceId,
		channelType: mgsConfig.RoleSubscribe,
		Processor:   mockProcessor}
}
