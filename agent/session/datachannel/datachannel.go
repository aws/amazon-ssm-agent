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
	"container/list"
	"encoding/json"
	"fmt"
	"math"
	"math/rand"
	"sync"
	"time"

	"github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/amazon-ssm-agent/agent/platform"
	"github.com/aws/amazon-ssm-agent/agent/rip"
	"github.com/aws/amazon-ssm-agent/agent/session/communicator"
	mgsConfig "github.com/aws/amazon-ssm-agent/agent/session/config"
	mgsContracts "github.com/aws/amazon-ssm-agent/agent/session/contracts"
	"github.com/aws/amazon-ssm-agent/agent/session/retry"
	"github.com/aws/amazon-ssm-agent/agent/session/service"
	"github.com/aws/amazon-ssm-agent/agent/task"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/gorilla/websocket"
	"github.com/twinj/uuid"
)

const (
	acknowledgeMessageSchemaVersion  = 1
	acknowledgeMessageSequenceNumber = 0
	acknowledgeMessageFlags          = 3
)

type IDataChannel interface {
	Initialize(context context.T, mgsService service.Service, sessionId string, clientId string, instanceId string, role string)
	SetWebSocket(context context.T, mgsService service.Service, sessionId string, clientId string, onMessageHandler func(input []byte)) error
	Open(log log.T) error
	Close(log log.T) error
	Reconnect(log log.T) error
	SendMessage(log log.T, input []byte, inputType int) error
	SendStreamDataMessage(log log.T, dataType mgsContracts.PayloadType, inputData []byte) error
	ResendStreamDataMessageScheduler(log log.T) error
	ProcessAcknowledgedMessage(log log.T, acknowledgeMessageContent mgsContracts.AcknowledgeContent)
	SendAcknowledgeMessage(log log.T, agentMessage mgsContracts.AgentMessage) error
	AddDataToOutgoingMessageBuffer(streamMessage StreamingMessage)
	RemoveDataFromOutgoingMessageBuffer(streamMessageElement *list.Element)
	AddDataToIncomingMessageBuffer(streamMessage StreamingMessage)
	RemoveDataFromIncomingMessageBuffer(sequenceNumber int64)
	DataChannelIncomingMessageHandler(log log.T, streamMessageHandler StreamMessageHandler, rawMessage []byte, cancelFlag task.CancelFlag) error
}

// DataChannel used for session communication between the message gateway service and the agent.
type DataChannel struct {
	wsChannel  communicator.IWebSocketChannel
	Service    service.Service
	ChannelId  string
	ClientId   string
	InstanceId string
	Role       string
	Pause      bool
	//records sequence number of last acknowledged message received over data channel
	ExpectedSequenceNumber int64
	//records sequence number of last stream data message sent over data channel
	StreamDataSequenceNumber int64
	//buffer to store outgoing stream messages until acknowledged
	//using linked list for this buffer as access to oldest message is required and it support faster deletion from any position of list
	OutgoingMessageBuffer ListMessageBuffer
	//buffer to store incoming stream messages if received out of sequence
	//using map for this buffer as incoming messages can be out of order and retrieval would be faster by sequenceId
	IncomingMessageBuffer MapMessageBuffer
	//round trip time of latest acknowledged message
	RoundTripTime float64
	//round trip time variation of latest acknowledged message
	RoundTripTimeVariation float64
	//timeout used for resending unacknowledged message
	RetransmissionTimeout time.Duration
}

type ListMessageBuffer struct {
	Messages *list.List
	Capacity int
	Mutex    *sync.Mutex
}

type MapMessageBuffer struct {
	Messages map[int64]StreamingMessage
	Capacity int
	Mutex    *sync.Mutex
}

type StreamingMessage struct {
	Content        []byte
	SequenceNumber int64
	LastSentTime   time.Time
}

// NewDataChannel constructs datachannel objects.
func NewDataChannel(context context.T,
	channelId string,
	clientId string,
	onMessageHandler func(input []byte)) (*DataChannel, error) {

	log := context.Log()
	appConfig := context.AppConfig()

	messageGatewayServiceConfig := appConfig.Mgs
	if messageGatewayServiceConfig.Region == "" {
		fetchedRegion, err := platform.Region()
		if err != nil {
			return nil, fmt.Errorf("failed to get region with error: %s", err)
		}
		messageGatewayServiceConfig.Region = fetchedRegion
	}

	if messageGatewayServiceConfig.Endpoint == "" {
		fetchedEndpoint, err := getMgsEndpoint(messageGatewayServiceConfig.Region)
		if err != nil {
			return nil, fmt.Errorf("failed to get MessageGatewayService endpoint with error: %s", err)
		}
		messageGatewayServiceConfig.Endpoint = fetchedEndpoint
	}

	connectionTimeout := time.Duration(messageGatewayServiceConfig.StopTimeoutMillis) * time.Millisecond
	mgsService := service.NewService(log, messageGatewayServiceConfig, connectionTimeout)

	instanceID, err := platform.InstanceID()
	if instanceID == "" {
		return nil, fmt.Errorf("no instanceID provided, %s", err)
	}

	dataChannel := &DataChannel{}
	dataChannel.Initialize(context, mgsService, channelId, clientId, instanceID, mgsConfig.RolePublishSubscribe)
	if err := dataChannel.SetWebSocket(context, mgsService, channelId, clientId, onMessageHandler); err != nil {
		return nil, fmt.Errorf("failed to create websocket for datachannel with error: %s", err)
	}
	if err := dataChannel.Open(log); err != nil {
		return nil, fmt.Errorf("failed to open datachannel with error: %s", err)
	}
	dataChannel.ResendStreamDataMessageScheduler(log)
	return dataChannel, nil
}

// Initialize populates datachannel object.
func (dataChannel *DataChannel) Initialize(context context.T,
	mgsService service.Service,
	sessionId string,
	clientId string,
	instanceId string,
	role string) {

	dataChannel.Service = mgsService
	dataChannel.ChannelId = sessionId
	dataChannel.ClientId = clientId
	dataChannel.InstanceId = instanceId
	dataChannel.Role = role
	dataChannel.Pause = false
	dataChannel.ExpectedSequenceNumber = 0
	dataChannel.StreamDataSequenceNumber = 0
	dataChannel.OutgoingMessageBuffer = ListMessageBuffer{
		list.New(),
		mgsConfig.OutgoingMessageBufferCapacity,
		&sync.Mutex{},
	}
	dataChannel.IncomingMessageBuffer = MapMessageBuffer{
		make(map[int64]StreamingMessage),
		mgsConfig.IncomingMessageBufferCapacity,
		&sync.Mutex{},
	}
	dataChannel.RoundTripTime = float64(mgsConfig.DefaultRoundTripTime)
	dataChannel.RoundTripTimeVariation = mgsConfig.DefaultRoundTripTimeVariation
	dataChannel.RetransmissionTimeout = mgsConfig.DefaultTransmissionTimeout
	dataChannel.wsChannel = &communicator.WebSocketChannel{}
}

// SetWebSocket populates webchannel object.
func (dataChannel *DataChannel) SetWebSocket(context context.T,
	mgsService service.Service,
	sessionId string,
	clientId string,
	onMessageHandler func(input []byte)) error {

	log := context.Log()
	uuid.SwitchFormat(uuid.CleanHyphen)
	requestId := uuid.NewV4().String()

	log.Infof("Setting up datachannel for session: %s, requestId: %s, clientId: %s", sessionId, requestId, clientId)
	tokenValue, err := getDataChannelToken(log, mgsService, sessionId, requestId, clientId)
	if err != nil {
		log.Errorf("Failed to get datachannel token, error: %s", err)
		return err
	}

	onErrorHandler := func(err error) {
		uuid.SwitchFormat(uuid.CleanHyphen)
		requestId := uuid.NewV4().String()
		callable := func() (channel interface{}, err error) {
			tokenValue, err := getDataChannelToken(log, mgsService, sessionId, requestId, clientId)
			if err != nil {
				return dataChannel, err
			}
			dataChannel.wsChannel.SetChannelToken(tokenValue)
			if err = dataChannel.Reconnect(log); err != nil {
				return dataChannel, err
			}
			return dataChannel, nil
		}
		retryer := retry.RepeatableExponentialRetryer{
			CallableFunc:        callable,
			GeometricRatio:      mgsConfig.RetryGeometricRatio,
			InitialDelayInMilli: rand.Intn(mgsConfig.DataChannelRetryInitialDelayMillis) + mgsConfig.DataChannelRetryInitialDelayMillis,
			MaxDelayInMilli:     mgsConfig.DataChannelRetryMaxIntervalMillis,
			MaxAttempts:         mgsConfig.DataChannelNumMaxAttempts,
		}
		if _, err := retryer.Call(); err != nil {
			log.Error(err)
		}

		tokenValue, err := getDataChannelToken(log, mgsService, sessionId, requestId, clientId)
		if err != nil {
			log.Errorf("failed to get token, err: %s", err)
			return
		}
		dataChannel.wsChannel.SetChannelToken(tokenValue)
		dataChannel.Reconnect(log)
	}

	if err := dataChannel.wsChannel.Initialize(context,
		sessionId,
		mgsConfig.DataChannel,
		mgsConfig.RolePublishSubscribe,
		tokenValue,
		mgsService.GetRegion(),
		mgsService.GetV4Signer(),
		onMessageHandler,
		onErrorHandler); err != nil {
		log.Errorf("failed to initialize websocket channel for datachannel, error: %s", err)
		return err
	}
	return nil
}

// Open opens the websocket connection and sends the token for service to acknowledge the connection.
func (dataChannel *DataChannel) Open(log log.T) error {
	// Opens websocket connection
	if err := dataChannel.wsChannel.Open(log); err != nil {
		return fmt.Errorf("failed to connect data channel with error: %s", err)
	}

	// finalize handshake
	uuid.SwitchFormat(uuid.CleanHyphen)
	uid := uuid.NewV4().String()

	openDataChannelInput := service.OpenDataChannelInput{
		MessageSchemaVersion: aws.String(mgsConfig.MessageSchemaVersion),
		RequestId:            aws.String(uid),
		TokenValue:           aws.String(dataChannel.wsChannel.GetChannelToken()),
		ClientInstanceId:     aws.String(dataChannel.InstanceId),
		ClientId:             aws.String(dataChannel.ClientId),
	}
	jsonValue, err := json.Marshal(openDataChannelInput)
	if err != nil {
		return fmt.Errorf("error serializing openDataChannelInput: %s", err)
	}

	return dataChannel.SendMessage(log, jsonValue, websocket.TextMessage)
}

// SendMessage sends a message to the service through datachannel.
func (dataChannel *DataChannel) SendMessage(log log.T, input []byte, inputType int) error {
	return dataChannel.wsChannel.SendMessage(log, input, inputType)
}

// Reconnect reconnects datachannel to service endpoint.
func (dataChannel *DataChannel) Reconnect(log log.T) error {
	log.Debugf("Reconnecting with datachannel: %s, token: %t", dataChannel.ChannelId, dataChannel.wsChannel.GetChannelToken())

	if err := dataChannel.wsChannel.Close(log); err != nil {
		log.Debugf("Closing datachannel failed with error: %s", err)
	}

	if err := dataChannel.Open(log); err != nil {
		return fmt.Errorf("failed to reconnect datachannel with error: %s", err)
	}

	dataChannel.Pause = false
	log.Debugf("Successfully reconnected to datachannel %s", dataChannel.ChannelId)
	return nil
}

// Close closes datachannel - its web socket connection.
func (dataChannel *DataChannel) Close(log log.T) error {
	log.Infof("Closing datachannel with channel Id %s", dataChannel.ChannelId)
	return dataChannel.wsChannel.Close(log)
}

// SendStreamDataMessage sends a data message in a form of AgentMessage for streaming.
func (dataChannel *DataChannel) SendStreamDataMessage(log log.T, payloadType mgsContracts.PayloadType, inputData []byte) error {
	if len(inputData) == 0 {
		log.Debugf("Ignoring empty stream data payload. PayloadType: %d", payloadType)
		return nil
	}

	var flag uint64 = 0
	if dataChannel.StreamDataSequenceNumber == 0 {
		flag = 1
	}

	uuid.SwitchFormat(uuid.CleanHyphen)
	messageId := uuid.NewV4()
	agentMessage := &mgsContracts.AgentMessage{
		MessageType:    mgsContracts.OutputStreamDataMessage,
		SchemaVersion:  1,
		CreatedDate:    uint64(time.Now().UnixNano() / 1000000),
		SequenceNumber: dataChannel.StreamDataSequenceNumber,
		Flags:          flag,
		MessageId:      messageId,
		PayloadType:    uint32(payloadType),
		Payload:        inputData,
	}
	msg, err := agentMessage.Serialize(log)
	if err != nil {
		return fmt.Errorf("cannot serialize StreamData message %v", agentMessage)
	}

	if dataChannel.Pause {
		log.Tracef("Sending stream data message has been paused, saving stream data message sequence %d to local map: ", dataChannel.StreamDataSequenceNumber)
	} else {
		log.Tracef("Send stream data message %d with msg %s", dataChannel.StreamDataSequenceNumber, string(inputData))
		if err = dataChannel.SendMessage(log, msg, websocket.BinaryMessage); err != nil {
			log.Errorf("Error sending stream data message %v", err)
		}
	}

	streamingMessage := StreamingMessage{
		msg,
		dataChannel.StreamDataSequenceNumber,
		time.Now(),
	}
	log.Tracef("Add stream data to OutgoingMessageBuffer. Sequence Number: %d", streamingMessage.SequenceNumber)
	dataChannel.AddDataToOutgoingMessageBuffer(streamingMessage)
	dataChannel.StreamDataSequenceNumber = dataChannel.StreamDataSequenceNumber + 1
	return nil
}

// ResendStreamDataMessageScheduler spawns a separate go thread which keeps checking OutgoingMessageBuffer at fixed interval
// and resends first message if time elapsed since lastSentTime of the message is more than acknowledge wait time
func (dataChannel *DataChannel) ResendStreamDataMessageScheduler(log log.T) error {
	go func() {
		for {
			time.Sleep(mgsConfig.ResendSleepInterval)
			if dataChannel.Pause {
				log.Tracef("Resend stream data message has been paused")
				continue
			}
			streamMessageElement := dataChannel.OutgoingMessageBuffer.Messages.Front()
			if streamMessageElement == nil {
				continue
			}

			streamMessage := streamMessageElement.Value.(StreamingMessage)
			if time.Since(streamMessage.LastSentTime) > dataChannel.RetransmissionTimeout {
				log.Tracef("Resend stream data message: %d", streamMessage.SequenceNumber)
				if err := dataChannel.SendMessage(log, streamMessage.Content, websocket.BinaryMessage); err != nil {
					log.Errorf("Unable to send stream data message: %s", err)
				}
				streamMessage.LastSentTime = time.Now()
				streamMessageElement.Value = streamMessage
			}
		}
	}()
	return nil
}

// ProcessAcknowledgedMessage processes acknowledge messages by deleting them from OutgoingMessageBuffer.
func (dataChannel *DataChannel) ProcessAcknowledgedMessage(log log.T, acknowledgeMessageContent mgsContracts.AcknowledgeContent) {
	acknowledgeSequenceNumber := acknowledgeMessageContent.SequenceNumber
	for streamMessageElement := dataChannel.OutgoingMessageBuffer.Messages.Front(); streamMessageElement != nil; streamMessageElement = streamMessageElement.Next() {
		streamMessage := streamMessageElement.Value.(StreamingMessage)
		if streamMessage.SequenceNumber == acknowledgeSequenceNumber {

			//Calculate retransmission timeout based on latest round trip time of message
			dataChannel.calculateRetransmissionTimeout(log, streamMessage)

			log.Tracef("Delete stream data from OutgoingMessageBuffer. Sequence Number: %d", streamMessage.SequenceNumber)
			dataChannel.RemoveDataFromOutgoingMessageBuffer(streamMessageElement)
			break
		}
	}
}

// SendAcknowledgeMessage sends acknowledge message for stream data over data channel
func (dataChannel *DataChannel) SendAcknowledgeMessage(log log.T, streamDataMessage mgsContracts.AgentMessage) error {
	dataStreamAcknowledgeContent := &mgsContracts.AcknowledgeContent{
		MessageType:         streamDataMessage.MessageType,
		MessageId:           streamDataMessage.MessageId.String(),
		SequenceNumber:      streamDataMessage.SequenceNumber,
		IsSequentialMessage: true,
	}

	acknowledgeContentBytes, err := dataStreamAcknowledgeContent.Serialize(log)
	if err != nil {
		// should not happen
		log.Errorf("Cannot serialize Acknowledge message err: %v", err)
		return err
	}

	uuid.SwitchFormat(uuid.CleanHyphen)
	messageId := uuid.NewV4()
	agentMessage := &mgsContracts.AgentMessage{
		MessageType:    mgsContracts.AcknowledgeMessage,
		SchemaVersion:  acknowledgeMessageSchemaVersion,
		CreatedDate:    uint64(time.Now().UnixNano() / 1000000),
		SequenceNumber: acknowledgeMessageSequenceNumber,
		Flags:          acknowledgeMessageFlags,
		MessageId:      messageId,
		Payload:        acknowledgeContentBytes,
	}

	msg, err := agentMessage.Serialize(log)
	if err != nil {
		log.Errorf("Cannot serialize Acknowledge message err: %v", err)
		return err
	}

	log.Tracef("Send Acknowledge message for stream data: %d", streamDataMessage.SequenceNumber)
	err = dataChannel.SendMessage(log, msg, websocket.BinaryMessage)
	if err != nil {
		log.Errorf("Error sending acknowledge message %v", err)
		return err
	}
	return nil
}

// AddDataToOutgoingMessageBuffer adds given message at the end of OutputMessageBuffer if it has capacity.
func (dataChannel *DataChannel) AddDataToOutgoingMessageBuffer(streamMessage StreamingMessage) {
	if dataChannel.OutgoingMessageBuffer.Messages.Len() == dataChannel.OutgoingMessageBuffer.Capacity {
		return
	}
	dataChannel.OutgoingMessageBuffer.Mutex.Lock()
	dataChannel.OutgoingMessageBuffer.Messages.PushBack(streamMessage)
	dataChannel.OutgoingMessageBuffer.Mutex.Unlock()
}

// RemoveDataFromOutgoingMessageBuffer removes given element from OutgoingMessageBuffer.
func (dataChannel *DataChannel) RemoveDataFromOutgoingMessageBuffer(streamMessageElement *list.Element) {
	dataChannel.OutgoingMessageBuffer.Mutex.Lock()
	dataChannel.OutgoingMessageBuffer.Messages.Remove(streamMessageElement)
	dataChannel.OutgoingMessageBuffer.Mutex.Unlock()
}

// AddDataToIncomingMessageBuffer adds given message to IncomingMessageBuffer if it has capacity.
func (dataChannel *DataChannel) AddDataToIncomingMessageBuffer(streamMessage StreamingMessage) {
	if len(dataChannel.IncomingMessageBuffer.Messages) == dataChannel.IncomingMessageBuffer.Capacity {
		return
	}
	dataChannel.IncomingMessageBuffer.Mutex.Lock()
	dataChannel.IncomingMessageBuffer.Messages[streamMessage.SequenceNumber] = streamMessage
	dataChannel.IncomingMessageBuffer.Mutex.Unlock()
}

// RemoveDataFromIncomingMessageBuffer removes given sequence number message from IncomingMessageBuffer.
func (dataChannel *DataChannel) RemoveDataFromIncomingMessageBuffer(sequenceNumber int64) {
	dataChannel.IncomingMessageBuffer.Mutex.Lock()
	delete(dataChannel.IncomingMessageBuffer.Messages, sequenceNumber)
	dataChannel.IncomingMessageBuffer.Mutex.Unlock()
}

type StreamMessageHandler func(log log.T, streamDataMessage mgsContracts.AgentMessage) error

// DataChannelIncomingMessageHandler deserialize incoming message and
// processes that data with respective ProcessStreamMessagePayloadHandler that is passed in.
func (dataChannel *DataChannel) DataChannelIncomingMessageHandler(log log.T,
	streamMessageHandler StreamMessageHandler,
	rawMessage []byte,
	cancelFlag task.CancelFlag) error {

	streamDataMessage := &mgsContracts.AgentMessage{}
	if err := streamDataMessage.Deserialize(log, rawMessage); err != nil {
		log.Errorf("Cannot deserialize raw message: %s, err: %v.", string(rawMessage), err)
		return err
	}

	if err := streamDataMessage.Validate(); err != nil {
		log.Errorf("Invalid StreamDataMessage: %v, err: %v.", streamDataMessage, err)
		return err
	}

	switch streamDataMessage.MessageType {
	case mgsContracts.InputStreamDataMessage:
		return dataChannel.handleStreamDataMessage(log, streamMessageHandler, *streamDataMessage, rawMessage)
	case mgsContracts.AcknowledgeMessage:
		return dataChannel.handleAcknowledgeMessage(log, *streamDataMessage)
	case mgsContracts.ChannelClosedMessage:
		return dataChannel.handleChannelClosedMessage(log, *streamDataMessage, cancelFlag)
	case mgsContracts.PausePublicationMessage:
		dataChannel.handlePausePublicationMessage(log, *streamDataMessage, cancelFlag)
		return nil
	case mgsContracts.StartPublicationMessage:
		dataChannel.handleStartPublicationMessage(log, *streamDataMessage, cancelFlag)
		return nil
	default:
		log.Warn("Invalid message type received: %s", streamDataMessage.MessageType)
	}

	return nil
}

// calculateRetransmissionTimeout calculates message retransmission timeout value based on round trip time on given message.
func (dataChannel *DataChannel) calculateRetransmissionTimeout(log log.T, streamingMessage StreamingMessage) {
	newRoundTripTime := float64(time.Since(streamingMessage.LastSentTime))

	dataChannel.RoundTripTimeVariation = ((1 - mgsConfig.RTTVConstant) * dataChannel.RoundTripTimeVariation) +
		(mgsConfig.RTTVConstant * math.Abs(dataChannel.RoundTripTime-newRoundTripTime))

	dataChannel.RoundTripTime = ((1 - mgsConfig.RTTConstant) * dataChannel.RoundTripTime) +
		(mgsConfig.RTTConstant * newRoundTripTime)

	dataChannel.RetransmissionTimeout = time.Duration(dataChannel.RoundTripTime +
		math.Max(float64(mgsConfig.ClockGranularity), float64(4*dataChannel.RoundTripTimeVariation)))

	// Ensure RetransmissionTimeout do not exceed maximum timeout defined
	if dataChannel.RetransmissionTimeout > mgsConfig.MaxTransmissionTimeout {
		dataChannel.RetransmissionTimeout = mgsConfig.MaxTransmissionTimeout
	}

	log.Tracef("Retransmission timeout calculated in mills. "+
		"AcknowledgeMessageSequenceNumber: %d, RoundTripTime: %d, RoundTripTimeVariation: %d, RetransmissionTimeout: %d",
		streamingMessage.SequenceNumber,
		dataChannel.RoundTripTime,
		dataChannel.RoundTripTimeVariation,
		dataChannel.RetransmissionTimeout/time.Millisecond)
}

// handleStreamDataMessage handles incoming stream data messages by processing the payload and updating expectedSequenceNumber.
func (dataChannel *DataChannel) handleStreamDataMessage(log log.T,
	streamMessageHandler StreamMessageHandler,
	streamDataMessage mgsContracts.AgentMessage,
	rawMessage []byte) (err error) {

	dataChannel.Pause = false
	// On receiving expected stream data message, send acknowledgement, process it and increment expected sequence number by 1.
	// Further process messages from IncomingMessageBuffer
	if streamDataMessage.SequenceNumber == dataChannel.ExpectedSequenceNumber {
		if err = dataChannel.SendAcknowledgeMessage(log, streamDataMessage); err != nil {
			return err
		}

		log.Infof("Process new incoming stream data message. Sequence Number: %d", streamDataMessage.SequenceNumber)
		if err = streamMessageHandler(log, streamDataMessage); err != nil {
			log.Errorf("Unable to process stream data payload, err: %v.", err)
			return err
		}

		dataChannel.ExpectedSequenceNumber = dataChannel.ExpectedSequenceNumber + 1
		return dataChannel.processIncomingMessageBufferItems(log, streamMessageHandler)

		// If incoming message sequence number is greater than expected sequence number and IncomingMessageBuffer has capacity,
		// add message to IncomingMessageBuffer and send acknowledgement
	} else if streamDataMessage.SequenceNumber > dataChannel.ExpectedSequenceNumber {
		log.Infof("Unexpected sequence message received. Received Sequence Number: %s. Expected Sequence Number: %s",
			streamDataMessage.SequenceNumber, dataChannel.ExpectedSequenceNumber)

		if len(dataChannel.IncomingMessageBuffer.Messages) < dataChannel.IncomingMessageBuffer.Capacity {
			if err = dataChannel.SendAcknowledgeMessage(log, streamDataMessage); err != nil {
				return err
			}

			streamingMessage := StreamingMessage{
				rawMessage,
				streamDataMessage.SequenceNumber,
				time.Now(),
			}

			//Add message to buffer for future processing
			log.Debugf("Add stream data to IncomingMessageBuffer. Sequence Number: %d", streamDataMessage.SequenceNumber)
			dataChannel.AddDataToIncomingMessageBuffer(streamingMessage)
		}
	}
	return nil
}

// handleAcknowledgeMessage deserialize acknowledge content and process it.
func (dataChannel *DataChannel) handleAcknowledgeMessage(log log.T, streamDataMessage mgsContracts.AgentMessage) (err error) {
	dataChannel.Pause = false
	acknowledgeMessage := &mgsContracts.AcknowledgeContent{}
	if err = acknowledgeMessage.Deserialize(log, streamDataMessage); err != nil {
		log.Errorf("Cannot deserialize payload to AcknowledgeMessage: %s, err: %v.", string(streamDataMessage.Payload), err)
		return err
	}

	dataChannel.ProcessAcknowledgedMessage(log, *acknowledgeMessage)
	return nil
}

// handleChannelClosedMessage deserialize channel_closed message content and terminate the session.
func (dataChannel *DataChannel) handleChannelClosedMessage(log log.T, streamDataMessage mgsContracts.AgentMessage, cancelFlag task.CancelFlag) (err error) {
	channelClosedMessage := &mgsContracts.ChannelClosed{}
	if err = channelClosedMessage.Deserialize(log, streamDataMessage); err != nil {
		log.Errorf("Cannot deserialize payload to ChannelClosed message: %s, err: %v.", string(streamDataMessage.Payload), err)
		return err
	}

	log.Debugf("Processing terminate session request: messageId %s, sessionId %s", channelClosedMessage.MessageId, channelClosedMessage.SessionId)
	cancelFlag.Set(task.Canceled)

	return nil
}

// handlePausePublicationMessage sets pause status of datachannel to true.
func (dataChannel *DataChannel) handlePausePublicationMessage(log log.T, streamDataMessage mgsContracts.AgentMessage, cancelFlag task.CancelFlag) {
	dataChannel.Pause = true
	log.Debugf("Processed %s message. Datachannel pause status set to %s", streamDataMessage.MessageType, dataChannel.Pause)
}

// handleStartPublicationMessage sets pause status of datachannel to false.
func (dataChannel *DataChannel) handleStartPublicationMessage(log log.T, streamDataMessage mgsContracts.AgentMessage, cancelFlag task.CancelFlag) {
	dataChannel.Pause = false
	log.Debugf("Processed %s message. Datachannel pause status set to %s", streamDataMessage.MessageType, dataChannel.Pause)
}

// processIncomingMessageBufferItems checks if new expected sequence stream data is present in IncomingMessageBuffer.
// If so process it and increment expected sequence number.
// Repeat until expected sequence stream data is not found in IncomingMessageBuffer.
func (dataChannel *DataChannel) processIncomingMessageBufferItems(log log.T, streamMessageHandler StreamMessageHandler) (err error) {
	for {
		bufferedStreamMessage := dataChannel.IncomingMessageBuffer.Messages[dataChannel.ExpectedSequenceNumber]
		if bufferedStreamMessage.Content != nil {
			log.Debugf("Process stream data message from IncomingMessageBuffer. "+
				"Sequence Number: %d", bufferedStreamMessage.SequenceNumber)

			streamDataMessage := &mgsContracts.AgentMessage{}
			if err = streamDataMessage.Deserialize(log, bufferedStreamMessage.Content); err != nil {
				log.Errorf("Cannot deserialize raw message: %s, err: %v.", string(bufferedStreamMessage.Content), err)
				return err
			}
			if err = streamMessageHandler(log, *streamDataMessage); err != nil {
				log.Errorf("Unable to process stream data payload, err: %v.", err)
				return err
			}

			dataChannel.ExpectedSequenceNumber = dataChannel.ExpectedSequenceNumber + 1
			log.Debugf("Delete stream data from IncomingMessageBuffer. Sequence Number: %d", bufferedStreamMessage.SequenceNumber)
			dataChannel.RemoveDataFromIncomingMessageBuffer(bufferedStreamMessage.SequenceNumber)
		} else {
			break
		}
	}
	return nil
}

// getDataChannelToken calls CreateDataChannel to get the token for this session.
func getDataChannelToken(log log.T,
	mgsService service.Service,
	sessionId string,
	requestId string,
	clientId string) (tokenValue string, err error) {

	createDataChannelInput := &service.CreateDataChannelInput{
		MessageSchemaVersion: aws.String(mgsConfig.MessageSchemaVersion),
		RequestId:            aws.String(requestId),
		ClientId:             aws.String(clientId),
	}

	createDataChannelOutput, err := mgsService.CreateDataChannel(log, createDataChannelInput, sessionId)
	if err != nil || createDataChannelOutput == nil {
		return "", fmt.Errorf("CreateDataChannel failed with no output or error: %s", err)
	}

	log.Debugf("Successfully get datachannel token %s", *createDataChannelOutput.TokenValue)
	return *createDataChannelOutput.TokenValue, nil
}

// getMgsEndpoint builds mgs endpoint.
func getMgsEndpoint(region string) (string, error) {
	hostName := rip.GetMgsEndpoint(region)
	if hostName == "" {
		return "", fmt.Errorf("no MGS endpoint found in region %s", region)
	}
	var endpointBuilder bytes.Buffer
	endpointBuilder.WriteString(mgsConfig.HttpsPrefix)
	endpointBuilder.WriteString(hostName)
	return endpointBuilder.String(), nil
}
