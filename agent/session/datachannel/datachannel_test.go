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
	"bytes"
	"encoding/json"
	"errors"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/aws/amazon-ssm-agent/agent/appconfig"
	"github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/aws/amazon-ssm-agent/agent/crypto"
	cryptoMocks "github.com/aws/amazon-ssm-agent/agent/crypto/mocks"
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
	mockCipher                                 = &cryptoMocks.IBlockCipher{}
	mockCancelFlag                             = &task.MockCancelFlag{}
	clientId                                   = "dd01e56b-ff48-483e-a508-b5f073f31b16"
	createdDate                                = uint64(1503434274948)
	sessionId                                  = "2b196342-d7d4-436e-8f09-3883a1116ac3"
	instanceId                                 = "i-1234"
	messageId                                  = "dd01e56b-ff48-483e-a508-b5f073f31b16"
	kmskey                                     = "key"
	datakey                                    = []byte("datakey")
	token                                      = "token"
	region                                     = "us-east-1"
	signer                                     = &v4.Signer{Credentials: credentials.NewStaticCredentials("AKID", "SECRET", "SESSION")}
	onMessageHandler                           = func(input []byte) {}
	payload                                    = []byte("testPayload")
	versionString                              = "1.1.1.1.1"
	streamDataSequenceNumber                   = int64(0)
	expectedSequenceNumber                     = int64(0)
	serializedAgentMessages, streamingMessages = getAgentAndStreamingMessageList(7)
	inputStreamMessageHandler                  = func(log log.T, streamDataMessage mgsContracts.AgentMessage) error {
		return nil
	}
	sessionTypeRequest = mgsContracts.SessionTypeRequest{SessionType: appconfig.PluginNameStandardStream}
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
	err := dataChannel.dataChannelIncomingMessageHandler(mockLog, serializedAgentMessages[0])
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

	err = dataChannel.dataChannelIncomingMessageHandler(mockLog, serializedAgentMessages[1])
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

	err := dataChannel.dataChannelIncomingMessageHandler(mockLog, serializedAgentMessages[1])
	assert.Nil(t, err)

	err = dataChannel.dataChannelIncomingMessageHandler(mockLog, serializedAgentMessages[2])
	assert.Nil(t, err)

	err = dataChannel.dataChannelIncomingMessageHandler(mockLog, serializedAgentMessages[3])
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

	err := dataChannel.dataChannelIncomingMessageHandler(mockLog, serializedAgentMessage)

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

	err := dataChannel.dataChannelIncomingMessageHandler(mockLog, serializedAgentMessage)

	assert.Nil(t, err)
	assert.Equal(t, 0, dataChannel.OutgoingMessageBuffer.Messages.Len())
	mockCancelFlag.AssertExpectations(t)
}

func TestDataChannelIncomingMessageHandlerForPausePublicationMessage(t *testing.T) {
	dataChannel := getDataChannel()

	agentMessage := getAgentMessage(0, mgsContracts.PausePublicationMessage, uint32(0), payload)
	serializedAgentMessage, _ := agentMessage.Serialize(mockLog)

	err := dataChannel.dataChannelIncomingMessageHandler(mockLog, serializedAgentMessage)

	assert.Nil(t, err)
	assert.Equal(t, true, dataChannel.Pause)
}

func TestDataChannelIncomingMessageHandlerForStartPublicationMessage(t *testing.T) {
	dataChannel := getDataChannel()

	agentMessage := getAgentMessage(0, mgsContracts.StartPublicationMessage, uint32(0), payload)
	serializedAgentMessage, _ := agentMessage.Serialize(mockLog)

	err := dataChannel.dataChannelIncomingMessageHandler(mockLog, serializedAgentMessage)

	assert.Nil(t, err)
	assert.Equal(t, false, dataChannel.Pause)
}

func TestDataChannelHandshakeResponse(t *testing.T) {
	dataChannel := getDataChannel()

	mockChannel := &communicatorMocks.IWebSocketChannel{}
	dataChannel.wsChannel = mockChannel
	mockCipher := &cryptoMocks.IBlockCipher{}
	dataChannel.blockCipher = mockCipher
	// Default channel is not buffered, this causes a deadlock. Make the channel buffered.
	dataChannel.handshake.responseChan = make(chan bool, 1)
	dataChannel.encryptionEnabled = false

	handshakeResponsePayload, _ := json.Marshal(buildHandshakeResponse())
	agentMessageBytes, _ := getAgentMessage(int64(0), mgsContracts.InputStreamDataMessage,
		uint32(mgsContracts.HandshakeResponse), handshakeResponsePayload).Serialize(mockLog)

	mockChannel.On("SendMessage", mock.Anything, mock.Anything, mock.Anything).Return(nil)
	mockCipher.On("UpdateEncryptionKey", mockLog, datakey, sessionId, instanceId).Return(nil)

	err := dataChannel.dataChannelIncomingMessageHandler(mockLog, agentMessageBytes)
	assert.Nil(t, err)
	assert.True(t, dataChannel.encryptionEnabled)
	assert.True(t, <-dataChannel.handshake.responseChan)

	mockChannel.AssertExpectations(t)
	mockCipher.AssertExpectations(t)
	mockCancelFlag.AssertExpectations(t)
}

func TestDataChannelHandshakeResponseEncryptionClientFailure(t *testing.T) {
	dataChannel := getDataChannel()

	mockChannel := &communicatorMocks.IWebSocketChannel{}
	dataChannel.wsChannel = mockChannel
	mockCipher := &cryptoMocks.IBlockCipher{}
	dataChannel.blockCipher = mockCipher
	// Default channel is not buffered, this causes a deadlock. Make the channel buffered for test.
	dataChannel.handshake.responseChan = make(chan bool, 1)
	dataChannel.encryptionEnabled = false

	handshakeResponsePayload, _ := json.Marshal(buildHandshakeResponseEncryptionFailed())
	agentMessageBytes, _ := getAgentMessage(int64(0), mgsContracts.InputStreamDataMessage,
		uint32(mgsContracts.HandshakeResponse), handshakeResponsePayload).Serialize(mockLog)
	mockChannel.On("SendMessage", mock.Anything, mock.Anything, mock.Anything).Return(nil)
	mockCancelFlag.On("Set", task.Canceled).Return()

	err := dataChannel.dataChannelIncomingMessageHandler(mockLog, agentMessageBytes)

	assert.Nil(t, err)
	assert.False(t, dataChannel.encryptionEnabled)
	assert.NotNil(t, dataChannel.handshake.error)
	assert.True(t, <-dataChannel.handshake.responseChan)
	mockChannel.AssertExpectations(t)
	mockCancelFlag.AssertExpectations(t)
}

func TestDataChannelHandshakeResponseEncryptionAgentFailure(t *testing.T) {
	dataChannel := getDataChannel()

	mockChannel := &communicatorMocks.IWebSocketChannel{}
	dataChannel.wsChannel = mockChannel
	mockCipher := &cryptoMocks.IBlockCipher{}
	dataChannel.blockCipher = mockCipher
	// Default channel is not buffered, this causes a deadlock. Make the channel buffered for test.
	dataChannel.handshake.responseChan = make(chan bool, 1)
	dataChannel.encryptionEnabled = false

	handshakeResponsePayload, _ := json.Marshal(buildHandshakeResponse())
	agentMessageBytes, _ := getAgentMessage(int64(0), mgsContracts.InputStreamDataMessage,
		uint32(mgsContracts.HandshakeResponse), handshakeResponsePayload).Serialize(mockLog)

	// Account for acknowledgements being sent
	mockChannel.On("SendMessage", mock.Anything, mock.Anything, mock.Anything).Return(nil)
	// Throw error when processing handshake response
	errorString := "Failed to update encryption key. Something bad happened."
	mockCipher.On("UpdateEncryptionKey", mockLog, datakey, sessionId, instanceId).Return(errors.New(errorString))

	mockCancelFlag.On("Set", task.Canceled).Return()

	err := dataChannel.dataChannelIncomingMessageHandler(mockLog, agentMessageBytes)

	assert.Nil(t, err)
	assert.False(t, dataChannel.encryptionEnabled)
	assert.Contains(t, dataChannel.handshake.error.Error(), errorString)
	assert.True(t, <-dataChannel.handshake.responseChan)
	mockChannel.AssertExpectations(t)
	mockCancelFlag.AssertExpectations(t)
}

func TestDataCHannelHandshakeInitiate(t *testing.T) {
	dataChannel := getDataChannel()
	mockChannel := &communicatorMocks.IWebSocketChannel{}
	dataChannel.wsChannel = mockChannel

	// Set up block cipher
	mockCipher.On("GetKMSKeyId").Return(kmskey)
	dataChannel.blockCipher = mockCipher
	dataChannel.encryptionEnabled = true

	// Mocking sending of handshake request
	handshakeRequestPayload, _ := json.Marshal(dataChannel.buildHandshakeRequestPayload(mockLog, true, sessionTypeRequest))
	handshakeRequestMatcher := func(sentData []byte) bool {
		agentMessage := mgsContracts.AgentMessage{}
		agentMessage.Deserialize(mockLog, sentData)
		return bytes.Equal(agentMessage.Payload, handshakeRequestPayload)
	}
	mockChannel.On("SendMessage", mockLog, mock.MatchedBy(handshakeRequestMatcher), mock.Anything).Return(nil)

	// Mock sending of encryption challenge
	encChallengeRequestMatcher := func(sentData []byte) bool {
		agentMessage := mgsContracts.AgentMessage{}
		agentMessage.Deserialize(mockLog, sentData)
		var encChallengeReq = mgsContracts.EncryptionChallengeRequest{}
		json.Unmarshal(agentMessage.Payload, &encChallengeReq)
		return len(encChallengeReq.Challenge) == 64 && agentMessage.PayloadType == uint32(mgsContracts.EncChallengeRequest)
	}
	mockCipher.On("EncryptWithAESGCM", mock.AnythingOfType("[]uint8")).Return(func(s []byte) []byte { return s }, nil)
	mockChannel.On("SendMessage", mockLog, mock.MatchedBy(encChallengeRequestMatcher), mock.Anything).Return(nil)

	// Mock sending of handshake complete
	handshakeCompleteMatcher := func(sentData []uint8) bool {
		agentMessage := mgsContracts.AgentMessage{}
		agentMessage.Deserialize(mockLog, sentData)
		var sentHandshakeComplete = mgsContracts.HandshakeCompletePayload{}
		json.Unmarshal(agentMessage.Payload, &sentHandshakeComplete)
		handshakeComplete := dataChannel.buildHandshakeCompletePayload(mockLog)
		return sentHandshakeComplete.CustomerMessage == handshakeComplete.CustomerMessage &&
			agentMessage.PayloadType == uint32(mgsContracts.HandshakeComplete)
	}
	mockChannel.On("SendMessage", mockLog, mock.MatchedBy(handshakeCompleteMatcher), mock.Anything).Return(nil)

	// Default channel is not buffered, this causes a deadlock. Make the channel buffered for test.
	dataChannel.handshake.responseChan = make(chan bool, 1)
	dataChannel.handshake.responseChan <- true
	dataChannel.handshake.encryptionConfirmedChan = make(chan bool, 1)
	dataChannel.handshake.encryptionConfirmedChan <- true

	// This is necessary because PerformHandshake initializes the cipher
	newBlockCipher = func(log log.T, kmsKeyId string) (blockCipher crypto.IBlockCipher, err error) {
		return mockCipher, nil
	}

	err := dataChannel.PerformHandshake(mockLog, kmskey, true, sessionTypeRequest)

	assert.Nil(t, err)
	assert.True(t, dataChannel.handshake.complete)
	assert.Nil(t, dataChannel.handshake.error)
	mockCipher.AssertExpectations(t)
	mockChannel.AssertExpectations(t)
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

	var flag uint64 = 2 // Default: Bit 1 is set when this message is the final message in the sequence.

	if sequenceNumber == 0 {
		flag = 1
	}
	agentMessage := mgsContracts.AgentMessage{
		MessageType:    messageType,
		SchemaVersion:  schemaVersion,
		CreatedDate:    createdDate,
		SequenceNumber: sequenceNumber,
		Flags:          flag,
		MessageId:      messageUUID,
		PayloadType:    payloadType,
		Payload:        payload,
	}
	return &agentMessage
}

func buildHandshakeResponse() mgsContracts.HandshakeResponsePayload {
	handshakeResponse := mgsContracts.HandshakeResponsePayload{}
	handshakeResponse.ClientVersion = versionString
	handshakeResponse.ProcessedClientActions = []mgsContracts.ProcessedClientAction{}

	processedAction := mgsContracts.ProcessedClientAction{}
	processedAction.ActionType = mgsContracts.KMSEncryption
	processedAction.ActionStatus = mgsContracts.Success
	processedAction.ActionResult, _ = json.Marshal(mgsContracts.KMSEncryptionResponse{KMSCipherTextKey: datakey})
	handshakeResponse.ProcessedClientActions = append(handshakeResponse.ProcessedClientActions, processedAction)

	processedAction = mgsContracts.ProcessedClientAction{}
	processedAction.ActionType = mgsContracts.SessionType
	processedAction.ActionStatus = mgsContracts.Success
	handshakeResponse.ProcessedClientActions = append(handshakeResponse.ProcessedClientActions, processedAction)
	return handshakeResponse
}

func buildHandshakeResponseEncryptionFailed() mgsContracts.HandshakeResponsePayload {
	handshakeResponse := mgsContracts.HandshakeResponsePayload{}
	handshakeResponse.ClientVersion = versionString
	handshakeResponse.ProcessedClientActions = []mgsContracts.ProcessedClientAction{}

	processedAction := mgsContracts.ProcessedClientAction{}
	processedAction.ActionType = mgsContracts.KMSEncryption
	processedAction.ActionStatus = mgsContracts.Failed
	processedAction.Error = "KMSError"
	handshakeResponse.ProcessedClientActions = append(handshakeResponse.ProcessedClientActions, processedAction)
	return handshakeResponse
}
