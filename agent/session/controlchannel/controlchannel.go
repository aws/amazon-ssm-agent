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
	"fmt"
	"math/rand"
	"path/filepath"
	"time"

	"github.com/aws/amazon-ssm-agent/agent/appconfig"
	"github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/aws/amazon-ssm-agent/agent/contracts"
	"github.com/aws/amazon-ssm-agent/agent/framework/processor"
	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/amazon-ssm-agent/agent/platform"
	"github.com/aws/amazon-ssm-agent/agent/session/communicator"
	mgsConfig "github.com/aws/amazon-ssm-agent/agent/session/config"
	mgsContracts "github.com/aws/amazon-ssm-agent/agent/session/contracts"
	"github.com/aws/amazon-ssm-agent/agent/session/retry"
	"github.com/aws/amazon-ssm-agent/agent/session/service"
	telemetry "github.com/aws/amazon-ssm-agent/agent/session/telemetry"
	"github.com/aws/amazon-ssm-agent/agent/times"
	"github.com/aws/amazon-ssm-agent/agent/version"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/gorilla/websocket"
	"github.com/twinj/uuid"
)

type IControlChannel interface {
	Initialize(context context.T, mgsService service.Service, processor processor.Processor, instanceId string, taskAckChan chan mgsContracts.AcknowledgeTaskContent)
	SetWebSocket(context context.T, mgsService service.Service, processor processor.Processor, instanceId string) error
	SendMessage(log log.T, input []byte, inputType int) error
	Reconnect(log log.T) error
	Close(log log.T) error
	Open(log log.T) error
}

// ControlChannel used for communication between the message gateway service and the agent.
type ControlChannel struct {
	wsChannel         communicator.IWebSocketChannel
	Processor         processor.Processor
	ChannelId         string
	Service           service.Service
	AuditLogScheduler telemetry.IAuditLogTelemetry
	channelType       string
	taskAckChan       chan mgsContracts.AcknowledgeTaskContent
}

// Initialize populates controlchannel object and opens controlchannel to communicate with mgs.
func (controlChannel *ControlChannel) Initialize(context context.T,
	mgsService service.Service,
	processor processor.Processor,
	instanceId string,
	taskAckChan chan mgsContracts.AcknowledgeTaskContent) {

	log := context.Log()
	controlChannel.Service = mgsService
	controlChannel.ChannelId = instanceId
	controlChannel.channelType = mgsConfig.RoleSubscribe
	controlChannel.Processor = processor
	controlChannel.wsChannel = &communicator.WebSocketChannel{}
	controlChannel.AuditLogScheduler = telemetry.GetAuditLogTelemetryInstance(context, controlChannel.wsChannel)
	controlChannel.taskAckChan = taskAckChan
	log.Debugf("Initialized controlchannel for instance: %s", instanceId)
}

// SetWebSocket populates webchannel object.
func (controlChannel *ControlChannel) SetWebSocket(context context.T,
	mgsService service.Service,
	processor processor.Processor,
	instanceId string) error {

	log := context.Log()
	uuid.SwitchFormat(uuid.CleanHyphen)
	uid := uuid.NewV4().String()

	log.Infof("Setting up websocket for controlchannel for instance: %s, requestId: %s", controlChannel.ChannelId, uid)
	tokenValue, err := getControlChannelToken(log, mgsService, controlChannel.ChannelId, uid)
	if err != nil {
		log.Errorf("Failed to get controlchannel token, error: %s", err)
		return err
	}

	config := context.AppConfig()
	shortInstanceId, _ := context.Identity().ShortInstanceID()
	orchestrationRootDir := filepath.Join(appconfig.DefaultDataStorePath, shortInstanceId, appconfig.DefaultSessionRootDirName, config.Agent.OrchestrationRootDir)

	onMessageHandler := func(input []byte) {
		controlChannelIncomingMessageHandler(context, processor, input, orchestrationRootDir, instanceId, controlChannel.taskAckChan)
	}
	onErrorHandler := func(err error) {
		callable := func() (channel interface{}, err error) {
			uuid.SwitchFormat(uuid.CleanHyphen)
			requestId := uuid.NewV4().String()
			tokenValue, err := getControlChannelToken(log, mgsService, controlChannel.ChannelId, requestId)
			if err != nil {
				return controlChannel, err
			}
			controlChannel.wsChannel.SetChannelToken(tokenValue)
			if err := controlChannel.Reconnect(log); err != nil {
				return controlChannel, err
			}
			return controlChannel, nil
		}
		retryer := retry.ExponentialRetryer{
			CallableFunc:        callable,
			GeometricRatio:      mgsConfig.RetryGeometricRatio,
			JitterRatio:         mgsConfig.RetryJitterRatio,
			InitialDelayInMilli: rand.Intn(mgsConfig.ControlChannelRetryInitialDelayMillis) + mgsConfig.ControlChannelRetryInitialDelayMillis,
			MaxDelayInMilli:     mgsConfig.ControlChannelRetryMaxIntervalMillis,
			MaxAttempts:         mgsConfig.ControlChannelNumMaxRetries,
		}

		// add a jitter to the first control-channel call
		maxDelayMillis := int64(float64(mgsConfig.ControlChannelRetryInitialDelayMillis) * mgsConfig.RetryJitterRatio)
		delayWithJitter(maxDelayMillis)

		retryer.Init()
		if _, err := retryer.Call(); err != nil {
			// should never happen
			log.Errorf("failed to reconnect to the controlchannel with error: %v", err)
		}
	}

	if err := controlChannel.wsChannel.Initialize(context,
		controlChannel.ChannelId,
		mgsConfig.ControlChannel,
		mgsConfig.RoleSubscribe,
		tokenValue,
		mgsService.GetRegion(),
		mgsService.GetV4Signer(),
		onMessageHandler,
		onErrorHandler); err != nil {
		log.Errorf("failed to initialize websocket channel for controlchannel, error: %s", err)
		return err
	}
	return nil
}

// SendMessage sends a message to the service through controlchannel.
func (controlChannel *ControlChannel) SendMessage(log log.T, input []byte, inputType int) error {
	return controlChannel.wsChannel.SendMessage(log, input, inputType)
}

// Reconnect reconnects a controlchannel.
func (controlChannel *ControlChannel) Reconnect(log log.T) error {
	log.Debugf("Reconnecting controlchannel %s", controlChannel.ChannelId)

	if err := controlChannel.wsChannel.Close(log); err != nil {
		log.Warnf("closing controlchannel failed with error: %s", err)
	}

	if err := controlChannel.Open(log); err != nil {
		return fmt.Errorf("failed to reconnect controlchannel with error: %s", err)
	}

	log.Debugf("Successfully reconnected with controlchannel with type %s", controlChannel.channelType)
	return nil
}

// Close closes controlchannel - its web socket connection.
func (controlChannel *ControlChannel) Close(log log.T) error {
	log.Infof("Closing controlchannel with channel Id %s", controlChannel.ChannelId)
	if controlChannel.wsChannel != nil {
		return controlChannel.wsChannel.Close(log)
	}
	if controlChannel.AuditLogScheduler != nil {
		controlChannel.AuditLogScheduler.StopScheduler()
	}
	return nil
}

// Open opens a websocket connection and sends the token for service to acknowledge the connection.
func (controlChannel *ControlChannel) Open(log log.T) error {
	if err := controlChannel.wsChannel.Open(log); err != nil {
		return fmt.Errorf("failed to connect controlchannel with error: %s", err)
	}

	uuid.SwitchFormat(uuid.CleanHyphen)
	uid := uuid.NewV4().String()

	instancePlatformType, _ := platform.PlatformType(log)

	openControlChannelInput := service.OpenControlChannelInput{
		MessageSchemaVersion: aws.String(mgsConfig.MessageSchemaVersion),
		RequestId:            aws.String(uid),
		TokenValue:           aws.String(controlChannel.wsChannel.GetChannelToken()),
		AgentVersion:         aws.String(version.Version),
		PlatformType:         aws.String(instancePlatformType),
	}

	jsonValue, err := json.Marshal(openControlChannelInput)
	if err != nil {
		return fmt.Errorf("error serializing openControlChannelInput: %s", err)
	}

	if err = controlChannel.SendMessage(log, jsonValue, websocket.TextMessage); err == nil {
		controlChannel.AuditLogScheduler.SendAuditMessage()
	}
	return err
}

// controlChannelIncomingMessageHandler handles the incoming messages coming to the agent.
func controlChannelIncomingMessageHandler(context context.T,
	processor processor.Processor,
	rawMessage []byte,
	orchestrationRootDir string,
	instanceId string,
	taskAckChan chan mgsContracts.AcknowledgeTaskContent) error {

	log := context.Log()

	agentMessage := &mgsContracts.AgentMessage{}
	if err := agentMessage.Deserialize(log, rawMessage); err != nil {
		log.Debugf("Cannot deserialize raw message, err: %v.", err)
		return err
	}

	if err := agentMessage.Validate(); err != nil {
		log.Debugf("Invalid AgentMessage: %s, err: %v.", agentMessage.MessageId, err)
		return err
	}

	if agentMessage.MessageType == mgsContracts.InteractiveShellMessage {
		uuid.SwitchFormat(uuid.CleanHyphen)
		clientId := uuid.NewV4().String()
		return sendStartSessionMessageToProcessor(processor, context, agentMessage, orchestrationRootDir, instanceId, clientId)
	} else if agentMessage.MessageType == mgsContracts.ChannelClosedMessage {
		return sendTerminateSessionMessageToProcessor(processor, context, instanceId, *agentMessage)
	} else if agentMessage.MessageType == mgsContracts.TaskAcknowledgeMessage {
		return processAcknowledgeMessage(context, *agentMessage, taskAckChan)
	}

	return fmt.Errorf("invalid message type: %s", agentMessage.MessageType)
}

// sendStartSessionMessageToProcessor sends a StartSession message to the processor.
func sendStartSessionMessageToProcessor(
	processor processor.Processor,
	context context.T,
	agentMessage *mgsContracts.AgentMessage,
	orchestrationRootDir string,
	instanceId string,
	clientId string) error {

	log := context.Log()
	log.Debugf("Processing StartSession message %s", agentMessage.MessageId.String())

	docState, err := agentMessage.ParseAgentMessage(context, orchestrationRootDir, instanceId, clientId)
	if err != nil {
		log.Errorf("Cannot parse AgentTask message to documentState: %s, err: %v.", agentMessage.MessageId, err)
		return err
	}

	// Submit message to processor
	processor.Submit(*docState)
	return nil
}

// sendTerminateSessionMessageToProcessor sends a TerminateSession message to the processor.
func sendTerminateSessionMessageToProcessor(
	processor processor.Processor,
	context context.T,
	instanceId string,
	agentMessage mgsContracts.AgentMessage) error {

	log := context.Log()
	log.Debugf("Processing TerminateSession message %s", agentMessage.MessageId.String())

	channelClosed := &mgsContracts.ChannelClosed{}
	if err := channelClosed.Deserialize(log, agentMessage); err != nil {
		log.Errorf("Cannot parse AgentTask message to ChannelClosed message: %s, err: %v.", agentMessage.MessageId, err)
		return err
	}
	log.Debugf("ChannelClosed message %s, sessionId %s", channelClosed.MessageId, channelClosed.SessionId)

	documentInfo := contracts.DocumentInfo{
		InstanceID:     instanceId,
		CreatedDate:    channelClosed.CreatedDate,
		MessageID:      channelClosed.SessionId,
		CommandID:      channelClosed.SessionId,
		DocumentID:     channelClosed.SessionId,
		RunID:          times.ToIsoDashUTC(times.DefaultClock.Now()),
		DocumentStatus: contracts.ResultStatusInProgress,
	}

	cancelSessionInfo := new(contracts.CancelCommandInfo)
	cancelSessionInfo.Payload = ""
	sessionId := channelClosed.SessionId
	cancelSessionInfo.CancelMessageID = sessionId
	cancelSessionInfo.CancelCommandID = sessionId
	cancelSessionInfo.DebugInfo = fmt.Sprintf("Session %v is yet to be terminated", sessionId)

	docState := contracts.DocumentState{
		DocumentInformation: documentInfo,
		CancelInformation:   *cancelSessionInfo,
		DocumentType:        contracts.TerminateSession,
	}

	// Submit message to processor
	processor.Cancel(docState)
	return nil
}

// processAcknowledgeMessage sends the acknowledgment message on chan so session can process it.
func processAcknowledgeMessage(
	context context.T,
	agentMessage mgsContracts.AgentMessage,
	taskAckChan chan mgsContracts.AcknowledgeTaskContent) error {

	log := context.Log()
	log.Debugf("Processing Task Acknowledge message %s", agentMessage.MessageId.String())
	taskAcknowledge := &mgsContracts.AcknowledgeTaskContent{}
	if err := taskAcknowledge.Deserialize(log, agentMessage); err != nil {
		log.Errorf("Cannot parse AgentTask message to TaskAcknowledgeMessage message: %s, err: %v.", agentMessage.MessageId.String(), err)
		return err
	}
	log.Debugf("TaskAcknowledgeMessage for topic [%s] and sessionId [%s]", taskAcknowledge.Topic, taskAcknowledge.TaskId)

	// handover to service
	taskAckChan <- *taskAcknowledge
	return nil
}

// getControlChannelToken calls CreateControlChannel to get the token for this instance
func getControlChannelToken(log log.T,
	mgsService service.Service,
	instanceId string,
	requestId string) (tokenValue string, err error) {

	createControlChannelInput := &service.CreateControlChannelInput{
		MessageSchemaVersion: aws.String(mgsConfig.MessageSchemaVersion),
		RequestId:            aws.String(requestId),
	}

	createControlChannelOutput, err := mgsService.CreateControlChannel(log, createControlChannelInput, instanceId)
	if err != nil || createControlChannelOutput == nil {
		return "", fmt.Errorf("CreateControlChannel failed with error: %s", err)
	}

	log.Debug("Successfully get controlchannel token")
	return *createControlChannelOutput.TokenValue, nil
}

// delayWithJitter adds a delay before next operation
func delayWithJitter(maxDelayMillis int64) {
	if maxDelayMillis <= 0 {
		return
	}
	jitter := rand.Int63n(maxDelayMillis)
	time.Sleep(time.Duration(jitter) * time.Millisecond)
}
