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

// Package datachannel implements data channel which is used to interactively run commands.
package datachannel

import (
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/aws/amazon-ssm-agent/agent/log"
	communicatorMocks "github.com/aws/amazon-ssm-agent/agent/session/communicator/mocks"
	mgsConfig "github.com/aws/amazon-ssm-agent/agent/session/config"
	mgsContracts "github.com/aws/amazon-ssm-agent/agent/session/contracts"
	"github.com/aws/amazon-ssm-agent/agent/session/service"
	serviceMock "github.com/aws/amazon-ssm-agent/agent/session/service/mocks"
	"github.com/aws/amazon-ssm-agent/agent/task"
	"github.com/aws/aws-sdk-go/aws/credentials"
	v4 "github.com/aws/aws-sdk-go/aws/signer/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/twinj/uuid"
)

var (
	mockContext                                = context.NewMockDefault()
	mockLog                                    = log.NewMockLog()
	mockService                                = &serviceMock.Service{}
	mockWsChannel                              = &communicatorMocks.IWebSocketChannel{}
	mockCancelFlag                             = &task.MockCancelFlag{}
	clientId                                   = "dd01e56b-ff48-483e-a508-b5f073f31b16"
	createdDate                                = uint64(1503434274948)
	sessionId                                  = "2b196342-d7d4-436e-8f09-3883a1116ac3"
	instanceId                                 = "i-1234"
	messageId                                  = "dd01e56b-ff48-483e-a508-b5f073f31b16"
	token                                      = "token"
	region                                     = "us-east-1"
	signer                                     = &v4.Signer{Credentials: credentials.NewStaticCredentials("AKID", "SECRET", "SESSION")}
	onMessageHandler                           = func(input []byte) {}
	payload                                    = []byte("testPayload")
	streamDataSequenceNumber                   = int64(0)
	expectedSequenceNumber                     = int64(0)
	serializedAgentMessages, streamingMessages = getAgentAndStreamingMessageList(7)
	inputStreamMessageHandler                  = func(log log.T, streamDataMessage mgsContracts.AgentMessage) error {
		return nil
	}
)

func TestInitialize(t *testing.T) {
	dataChannel := getDataChannel()
	dataChannel.Initialize(
		mockContext,
		mockService,
		sessionId,
		clientId,
		instanceId,
		mgsConfig.RolePublishSubscribe,
		mockCancelFlag,
		inputStreamMessageHandler)

	assert.Equal(t, instanceId, dataChannel.InstanceId)
	assert.Equal(t, sessionId, dataChannel.ChannelId)
	assert.Equal(t, mockService, dataChannel.Service)
	assert.Equal(t, mgsConfig.RolePublishSubscribe, dataChannel.Role)
	assert.True(t, dataChannel.ExpectedSequenceNumber == 0)
	assert.True(t, dataChannel.StreamDataSequenceNumber == 0)
	assert.False(t, dataChannel.Pause)
	assert.NotNil(t, dataChannel.wsChannel)
	assert.NotNil(t, dataChannel.OutgoingMessageBuffer)
	assert.NotNil(t, dataChannel.IncomingMessageBuffer)
	assert.Equal(t, float64(mgsConfig.DefaultRoundTripTime), dataChannel.RoundTripTime)
	assert.Equal(t, float64(mgsConfig.DefaultRoundTripTimeVariation), dataChannel.RoundTripTimeVariation)
	assert.Equal(t, mgsConfig.DefaultTransmissionTimeout, dataChannel.RetransmissionTimeout)
}

func TestSetWebSocket(t *testing.T) {
	dataChannel := getDataChannel()

	createDataChannelOutput := service.CreateDataChannelOutput{TokenValue: &token}
	mockService.On("CreateDataChannel", mock.Anything, mock.Anything, mock.Anything).Return(&createDataChannelOutput, nil)
	mockService.On("GetRegion").Return(region)
	mockService.On("GetV4Signer").Return(signer)
	mockWsChannel.On("Initialize",
		mock.Anything,
		sessionId,
		mgsConfig.DataChannel,
		mgsConfig.RolePublishSubscribe,
		token,
		region,
		signer,
		mock.Anything,
		mock.Anything).Return(nil)

	err := dataChannel.SetWebSocket(mockContext, mockService, sessionId, clientId, onMessageHandler)

	assert.Nil(t, err)
	mockWsChannel.AssertExpectations(t)
	mockService.AssertExpectations(t)
}

func TestOpen(t *testing.T) {
	dataChannel := getDataChannel()

	mockWsChannel.On("Open", mock.Anything).Return(nil)
	mockWsChannel.On("GetChannelToken").Return(token)
	mockWsChannel.On("SendMessage", mock.Anything, mock.Anything, mock.Anything).Return(nil)

	// test open (includes SendMessage)
	err := dataChannel.Open(mockLog)

	assert.Nil(t, err)
	assert.Equal(t, token, dataChannel.wsChannel.GetChannelToken())
	mockWsChannel.AssertExpectations(t)
}

func TestReconnect(t *testing.T) {
	dataChannel := getDataChannel()

	mockWsChannel.On("Close", mock.Anything).Return(nil)
	mockWsChannel.On("Open", mock.Anything).Return(nil)
	mockWsChannel.On("GetChannelToken").Return(token)
	mockWsChannel.On("SendMessage", mock.Anything, mock.Anything, mock.Anything).Return(nil)

	// test reconnect
	err := dataChannel.Reconnect(mockLog)

	assert.Nil(t, err)
	assert.Equal(t, token, dataChannel.wsChannel.GetChannelToken())
	mockWsChannel.AssertExpectations(t)
}

func TestClose(t *testing.T) {
	dataChannel := getDataChannel()

	mockWsChannel.On("Close", mock.Anything).Return(nil)

	// test close
	err := dataChannel.Close(mockLog)

	assert.Nil(t, err)
	mockWsChannel.AssertExpectations(t)
}

func TestSendStreamDataMessage(t *testing.T) {
	dataChannel := getDataChannel()

	mockWsChannel.On("SendMessage", mock.Anything, mock.Anything, mock.Anything).Return(nil)

	dataChannel.SendStreamDataMessage(mockLog, mgsContracts.Output, payload)

	assert.Equal(t, streamDataSequenceNumber+1, dataChannel.StreamDataSequenceNumber)
	assert.Equal(t, 1, dataChannel.OutgoingMessageBuffer.Messages.Len())
	mockWsChannel.AssertExpectations(t)
}

func TestSendStreamDataMessageWhenPayloadIsEmpty(t *testing.T) {
	dataChannel := getDataChannel()
	mockChannel := &communicatorMocks.IWebSocketChannel{}
	dataChannel.wsChannel = mockChannel

	dataChannel.SendStreamDataMessage(mockLog, mgsContracts.Output, []byte(""))

	assert.Equal(t, streamDataSequenceNumber, dataChannel.StreamDataSequenceNumber)
	assert.Equal(t, 0, dataChannel.OutgoingMessageBuffer.Messages.Len())
	mockChannel.AssertNotCalled(t, "SendMessage", mock.Anything, mock.Anything, mock.Anything)
}

func TestResendStreamDataMessageScheduler(t *testing.T) {
	dataChannel := getDataChannel()

	mockWsChannel.On("SendMessage", mock.Anything, mock.Anything, mock.Anything).Return(nil)

	dataChannel.AddDataToOutgoingMessageBuffer(streamingMessages[0])

	var wg sync.WaitGroup
	wg.Add(1)
	// Spawning a separate go routine to close websocket connection.
	// This is required as ResendStreamDataMessageScheduler has a for loop which will continuosly resend data until channel is closed.
	go func() {
		time.Sleep(220 * time.Millisecond)
		wg.Done()
	}()

	dataChannel.ResendStreamDataMessageScheduler(mockLog)

	wg.Wait()
	mockWsChannel.AssertExpectations(t)
}

func TestProcessAcknowledgedMessage(t *testing.T) {
	dataChannel := getDataChannel()
	dataChannel.AddDataToOutgoingMessageBuffer(streamingMessages[0])
	dataStreamAcknowledgeContent := mgsContracts.AcknowledgeContent{
		MessageType:         mgsContracts.InputStreamDataMessage,
		MessageId:           messageId,
		SequenceNumber:      0,
		IsSequentialMessage: true,
	}

	dataChannel.ProcessAcknowledgedMessage(mockLog, dataStreamAcknowledgeContent)

	assert.Equal(t, 0, dataChannel.OutgoingMessageBuffer.Messages.Len())
}

func TestSendAcknowledgeMessage(t *testing.T) {
	dataChannel := getDataChannel()

	mockWsChannel.On("SendMessage", mock.Anything, mock.Anything, mock.Anything).Return(nil)
	agentMessage := getAgentMessage(int64(1), mgsContracts.InputStreamDataMessage, uint32(mgsContracts.Output), []byte(""))

	dataChannel.SendAcknowledgeMessage(mockLog, *agentMessage)

	mockWsChannel.AssertExpectations(t)
}

func TestSendAgentSessionStateMessage(t *testing.T) {
	dataChannel := getDataChannel()

	mockWsChannel.On("SendMessage", mock.Anything, mock.Anything, mock.Anything).Return(nil)
	dataChannel.SendAgentSessionStateMessage(mockLog, mgsContracts.Connected)

	mockWsChannel.AssertExpectations(t)
}

func TestAddDataToOutgoingMessageBuffer(t *testing.T) {
	dataChannel := getDataChannel()
	dataChannel.OutgoingMessageBuffer.Capacity = 2

	dataChannel.AddDataToOutgoingMessageBuffer(streamingMessages[0])
	assert.Equal(t, 1, dataChannel.OutgoingMessageBuffer.Messages.Len())
	bufferedStreamMessage := dataChannel.OutgoingMessageBuffer.Messages.Front().Value.(StreamingMessage)
	assert.Equal(t, int64(0), bufferedStreamMessage.SequenceNumber)

	dataChannel.AddDataToOutgoingMessageBuffer(streamingMessages[1])
	assert.Equal(t, 2, dataChannel.OutgoingMessageBuffer.Messages.Len())
	bufferedStreamMessage = dataChannel.OutgoingMessageBuffer.Messages.Front().Value.(StreamingMessage)
	assert.Equal(t, int64(0), bufferedStreamMessage.SequenceNumber)
	bufferedStreamMessage = dataChannel.OutgoingMessageBuffer.Messages.Back().Value.(StreamingMessage)
	assert.Equal(t, int64(1), bufferedStreamMessage.SequenceNumber)

	dataChannel.AddDataToOutgoingMessageBuffer(streamingMessages[2])
	assert.Equal(t, 2, dataChannel.OutgoingMessageBuffer.Messages.Len())
	bufferedStreamMessage = dataChannel.OutgoingMessageBuffer.Messages.Front().Value.(StreamingMessage)
	assert.Equal(t, int64(0), bufferedStreamMessage.SequenceNumber)
	bufferedStreamMessage = dataChannel.OutgoingMessageBuffer.Messages.Back().Value.(StreamingMessage)
	assert.Equal(t, int64(1), bufferedStreamMessage.SequenceNumber)
}

func TestRemoveDataFromOutgoingMessageBuffer(t *testing.T) {
	dataChannel := getDataChannel()
	for i := 0; i < 3; i++ {
		dataChannel.AddDataToOutgoingMessageBuffer(streamingMessages[i])
	}

	dataChannel.RemoveDataFromOutgoingMessageBuffer(dataChannel.OutgoingMessageBuffer.Messages.Front())
	assert.Equal(t, 2, dataChannel.OutgoingMessageBuffer.Messages.Len())
}

func TestAddDataToIncomingMessageBuffer(t *testing.T) {
	dataChannel := getDataChannel()
	dataChannel.IncomingMessageBuffer.Capacity = 2

	dataChannel.AddDataToIncomingMessageBuffer(streamingMessages[0])
	assert.Equal(t, 1, len(dataChannel.IncomingMessageBuffer.Messages))
	bufferedStreamMessage := dataChannel.IncomingMessageBuffer.Messages[0]
	assert.Equal(t, int64(0), bufferedStreamMessage.SequenceNumber)

	dataChannel.AddDataToIncomingMessageBuffer(streamingMessages[1])
	assert.Equal(t, 2, len(dataChannel.IncomingMessageBuffer.Messages))
	bufferedStreamMessage = dataChannel.IncomingMessageBuffer.Messages[0]
	assert.Equal(t, int64(0), bufferedStreamMessage.SequenceNumber)
	bufferedStreamMessage = dataChannel.IncomingMessageBuffer.Messages[1]
	assert.Equal(t, int64(1), bufferedStreamMessage.SequenceNumber)

	dataChannel.AddDataToIncomingMessageBuffer(streamingMessages[2])
	assert.Equal(t, 2, len(dataChannel.IncomingMessageBuffer.Messages))
	bufferedStreamMessage = dataChannel.IncomingMessageBuffer.Messages[0]
	assert.Equal(t, int64(0), bufferedStreamMessage.SequenceNumber)
	bufferedStreamMessage = dataChannel.IncomingMessageBuffer.Messages[1]
	assert.Equal(t, int64(1), bufferedStreamMessage.SequenceNumber)
	bufferedStreamMessage = dataChannel.IncomingMessageBuffer.Messages[2]
	assert.Nil(t, bufferedStreamMessage.Content)
}

func TestRemoveDataFromIncomingMessageBuffer(t *testing.T) {
	dataChannel := getDataChannel()
	for i := 0; i < 3; i++ {
		dataChannel.AddDataToIncomingMessageBuffer(streamingMessages[i])
	}

	dataChannel.RemoveDataFromIncomingMessageBuffer(0)
	assert.Equal(t, 2, len(dataChannel.IncomingMessageBuffer.Messages))
}

func TestDataChannelIncomingMessageHandlerForExpectedInputStreamDataMessage(t *testing.T) {
	dataChannel := getDataChannel()
	dataChannel.Pause = true
	mockChannel := &communicatorMocks.IWebSocketChannel{}
	dataChannel.wsChannel = mockChannel

	mockChannel.On("SendMessage", mock.Anything, mock.Anything, mock.Anything).Return(nil)

	// First scenario is to test when incoming message sequence number matches with expected sequence number
	// and no message found in IncomingMessageBuffer
	err := dataChannel.DataChannelIncomingMessageHandler(mockLog, serializedAgentMessages[0])
	assert.Nil(t, err)
	assert.Equal(t, int64(1), dataChannel.ExpectedSequenceNumber)
	assert.Equal(t, 0, len(dataChannel.IncomingMessageBuffer.Messages))
	mockChannel.AssertNumberOfCalls(t, "SendMessage", 1)
	assert.Equal(t, false, dataChannel.Pause)

	// Second scenario is to test when incoming message sequence number matches with expected sequence number
	// and there are more messages found in IncomingMessageBuffer to be processed
	dataChannel.AddDataToIncomingMessageBuffer(streamingMessages[2])
	dataChannel.AddDataToIncomingMessageBuffer(streamingMessages[6])
	dataChannel.AddDataToIncomingMessageBuffer(streamingMessages[4])
	dataChannel.AddDataToIncomingMessageBuffer(streamingMessages[3])

	err = dataChannel.DataChannelIncomingMessageHandler(mockLog, serializedAgentMessages[1])
	assert.Nil(t, err)
	assert.Equal(t, int64(5), dataChannel.ExpectedSequenceNumber)
	assert.Equal(t, 1, len(dataChannel.IncomingMessageBuffer.Messages))
	mockChannel.AssertNumberOfCalls(t, "SendMessage", 2)

	// All messages from buffer should get processed except sequence number 6 as expected number to be processed at this time is 5
	bufferedStreamMessage := dataChannel.IncomingMessageBuffer.Messages[6]
	assert.Equal(t, int64(6), bufferedStreamMessage.SequenceNumber)
}

func TestDataChannelIncomingMessageHandlerForUnexpectedInputStreamDataMessage(t *testing.T) {
	dataChannel := getDataChannel()
	dataChannel.Pause = true
	mockChannel := &communicatorMocks.IWebSocketChannel{}
	dataChannel.wsChannel = mockChannel
	dataChannel.IncomingMessageBuffer.Capacity = 2

	mockChannel.On("SendMessage", mock.Anything, mock.Anything, mock.Anything).Return(nil)

	err := dataChannel.DataChannelIncomingMessageHandler(mockLog, serializedAgentMessages[1])
	assert.Nil(t, err)

	err = dataChannel.DataChannelIncomingMessageHandler(mockLog, serializedAgentMessages[2])
	assert.Nil(t, err)

	err = dataChannel.DataChannelIncomingMessageHandler(mockLog, serializedAgentMessages[3])
	assert.Nil(t, err)

	assert.Equal(t, expectedSequenceNumber, dataChannel.ExpectedSequenceNumber)
	assert.Equal(t, 2, len(dataChannel.IncomingMessageBuffer.Messages))
	mockChannel.AssertNumberOfCalls(t, "SendMessage", 2)
	assert.Equal(t, false, dataChannel.Pause)

	bufferedStreamMessage := dataChannel.IncomingMessageBuffer.Messages[1]
	assert.Equal(t, int64(1), bufferedStreamMessage.SequenceNumber)
	bufferedStreamMessage = dataChannel.IncomingMessageBuffer.Messages[2]
	assert.Equal(t, int64(2), bufferedStreamMessage.SequenceNumber)
	bufferedStreamMessage = dataChannel.IncomingMessageBuffer.Messages[3]
	assert.Nil(t, bufferedStreamMessage.Content)
}

func TestDataChannelIncomingMessageHandlerForAcknowledgeMessage(t *testing.T) {
	dataChannel := getDataChannel()
	dataChannel.Pause = true
	mockChannel := &communicatorMocks.IWebSocketChannel{}
	dataChannel.wsChannel = mockChannel
	for i := 0; i < 3; i++ {
		dataChannel.AddDataToOutgoingMessageBuffer(streamingMessages[i])
	}

	acknowledgeContent := &mgsContracts.AcknowledgeContent{
		MessageType:         mgsContracts.AcknowledgeMessage,
		MessageId:           messageId,
		SequenceNumber:      1,
		IsSequentialMessage: true,
	}
	payload, _ = acknowledgeContent.Serialize(mockLog)
	agentMessage := getAgentMessage(0, mgsContracts.AcknowledgeMessage, uint32(0), payload)
	serializedAgentMessage, _ := agentMessage.Serialize(mockLog)

	err := dataChannel.DataChannelIncomingMessageHandler(mockLog, serializedAgentMessage)

	assert.Nil(t, err)
	assert.Equal(t, 2, dataChannel.OutgoingMessageBuffer.Messages.Len())
	assert.Equal(t, false, dataChannel.Pause)
}

func TestDataChannelIncomingMessageHandlerForChannelClosedMessage(t *testing.T) {
	dataChannel := getDataChannel()
	mockChannel := &communicatorMocks.IWebSocketChannel{}
	dataChannel.wsChannel = mockChannel

	mockCancelFlag.On("Set", mock.Anything)

	channelClosedMessage := &mgsContracts.ChannelClosed{
		MessageType:   mgsContracts.ChannelClosedMessage,
		MessageId:     messageId,
		DestinationId: messageId,
		SessionId:     messageId,
		SchemaVersion: 1,
		CreatedDate:   "2018-06-30",
	}
	payload, _ = channelClosedMessage.Serialize(mockLog)
	agentMessage := getAgentMessage(0, mgsContracts.ChannelClosedMessage, uint32(0), payload)
	serializedAgentMessage, _ := agentMessage.Serialize(mockLog)

	err := dataChannel.DataChannelIncomingMessageHandler(mockLog, serializedAgentMessage)

	assert.Nil(t, err)
	assert.Equal(t, 0, dataChannel.OutgoingMessageBuffer.Messages.Len())
	mockCancelFlag.AssertExpectations(t)
}

func TestDataChannelIncomingMessageHandlerForPausePublicationMessage(t *testing.T) {
	dataChannel := getDataChannel()

	agentMessage := getAgentMessage(0, mgsContracts.PausePublicationMessage, uint32(0), payload)
	serializedAgentMessage, _ := agentMessage.Serialize(mockLog)

	err := dataChannel.DataChannelIncomingMessageHandler(mockLog, serializedAgentMessage)

	assert.Nil(t, err)
	assert.Equal(t, true, dataChannel.Pause)
}

func TestDataChannelIncomingMessageHandlerForStartPublicationMessage(t *testing.T) {
	dataChannel := getDataChannel()

	agentMessage := getAgentMessage(0, mgsContracts.StartPublicationMessage, uint32(0), payload)
	serializedAgentMessage, _ := agentMessage.Serialize(mockLog)

	err := dataChannel.DataChannelIncomingMessageHandler(mockLog, serializedAgentMessage)

	assert.Nil(t, err)
	assert.Equal(t, false, dataChannel.Pause)
}

func getDataChannel() *DataChannel {
	dataChannel := &DataChannel{}
	dataChannel.Initialize(mockContext,
		mockService,
		sessionId,
		clientId,
		instanceId,
		mgsConfig.RolePublishSubscribe,
		mockCancelFlag,
		inputStreamMessageHandler)
	dataChannel.wsChannel = mockWsChannel
	return dataChannel
}

// getAgentAndStreamingMessageList returns a list of serialized agent messages and corresponding streaming messages
func getAgentAndStreamingMessageList(size int) (serializedAgentMessage [][]byte, streamingMessages []StreamingMessage) {
	var payload string
	streamingMessages = make([]StreamingMessage, size)
	serializedAgentMessage = make([][]byte, size)
	for i := 0; i < size; i++ {
		payload = "testPayload" + strconv.Itoa(i)
		agentMessage := getAgentMessage(int64(i), mgsContracts.InputStreamDataMessage, uint32(mgsContracts.Output), []byte(payload))
		serializedAgentMessage[i], _ = agentMessage.Serialize(mockLog)
		streamingMessages[i] = StreamingMessage{
			serializedAgentMessage[i],
			int64(i),
			time.Now(),
		}
	}
	return
}

// getAgentMessage constructs and returns AgentMessage with given sequenceNumber, messageType & payload
func getAgentMessage(sequenceNumber int64, messageType string, payloadType uint32, payload []byte) *mgsContracts.AgentMessage {
	messageUUID, _ := uuid.Parse(messageId)
	agentMessage := mgsContracts.AgentMessage{
		MessageType:    messageType,
		SchemaVersion:  schemaVersion,
		CreatedDate:    createdDate,
		SequenceNumber: sequenceNumber,
		Flags:          2,
		MessageId:      messageUUID,
		PayloadType:    payloadType,
		Payload:        payload,
	}
	return &agentMessage
}
