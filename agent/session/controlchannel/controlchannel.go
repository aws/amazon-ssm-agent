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

// Package controlchannel implement control communicator for web socket connection.
package controlchannel

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"net/http"
	"strings"
	"sync/atomic"
	"time"

	"github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/amazon-ssm-agent/agent/network"
	"github.com/aws/amazon-ssm-agent/agent/platform"
	"github.com/aws/amazon-ssm-agent/agent/session/communicator"
	mgsConfig "github.com/aws/amazon-ssm-agent/agent/session/config"
	mgsContracts "github.com/aws/amazon-ssm-agent/agent/session/contracts"
	"github.com/aws/amazon-ssm-agent/agent/session/retry"
	"github.com/aws/amazon-ssm-agent/agent/session/service"
	"github.com/aws/amazon-ssm-agent/agent/session/telemetry"
	"github.com/aws/amazon-ssm-agent/agent/ssmconnectionchannel"
	"github.com/aws/amazon-ssm-agent/agent/version"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/gorilla/websocket"
	"github.com/twinj/uuid"
)

type IControlChannel interface {
	Initialize(context context.T, mgsService service.Service, instanceId string, agentMessageIncomingMessageChan chan mgsContracts.AgentMessage)
	SetWebSocket(context context.T, mgsService service.Service, ableToOpenMGSConnection *uint32) error
	SendMessage(log log.T, input []byte, inputType int) error
	Reconnect(context context.T, ableToOpenMGSConnection *uint32) error
	Close(log log.T) error
	Open(context context.T, ableToOpenMGSConnection *uint32) error
}

// ControlChannel used for communication between the message gateway service and the agent.
type ControlChannel struct {
	wsChannel                       communicator.IWebSocketChannel
	context                         context.T
	ChannelId                       string
	Service                         service.Service
	AuditLogScheduler               telemetry.IAuditLogTelemetry
	channelType                     string
	agentMessageIncomingMessageChan chan mgsContracts.AgentMessage
	readyMessageChan                chan bool
}

// Initialize populates controlchannel object and opens controlchannel to communicate with mgs.
func (controlChannel *ControlChannel) Initialize(context context.T,
	mgsService service.Service,
	instanceId string,
	agentMessageIncomingMessageChan chan mgsContracts.AgentMessage) {

	log := context.Log()
	controlChannel.Service = mgsService
	controlChannel.ChannelId = instanceId
	controlChannel.channelType = mgsConfig.RoleSubscribe
	controlChannel.wsChannel = &communicator.WebSocketChannel{}
	controlChannel.AuditLogScheduler = telemetry.GetAuditLogTelemetryInstance(context, controlChannel.wsChannel)
	controlChannel.agentMessageIncomingMessageChan = agentMessageIncomingMessageChan
	controlChannel.readyMessageChan = make(chan bool)
	controlChannel.context = context
	log.Debugf("Initialized controlchannel for instance: %s", instanceId)
}

// SetWebSocket populates webchannel object.
func (controlChannel *ControlChannel) SetWebSocket(context context.T,
	mgsService service.Service, ableToOpenMGSConnection *uint32) error {

	log := context.Log()
	uuid.SwitchFormat(uuid.CleanHyphen)
	uid := uuid.NewV4().String()

	log.Infof("Setting up websocket for controlchannel for instance: %s, requestId: %s", controlChannel.ChannelId, uid)
	tokenValue, err := getControlChannelToken(context, mgsService, controlChannel.ChannelId, uid, ableToOpenMGSConnection)
	if err != nil {
		log.Errorf("Failed to get controlchannel token, error: %s", err)
		return err
	}

	onMessageHandler := func(input []byte) {
		controlChannelIncomingMessageHandler(context, input, controlChannel.agentMessageIncomingMessageChan, controlChannel.readyMessageChan)
	}
	onErrorHandler := func(err error) {
		callable := func() (channel interface{}, err error) {
			uuid.SwitchFormat(uuid.CleanHyphen)
			requestId := uuid.NewV4().String()
			tokenValue, err := getControlChannelToken(context, mgsService, controlChannel.ChannelId, requestId, ableToOpenMGSConnection)
			if err != nil {
				return controlChannel, err
			}
			controlChannel.wsChannel.SetChannelToken(tokenValue)
			if err := controlChannel.Reconnect(context, ableToOpenMGSConnection); err != nil {
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
			NonRetryableErrors:  getNonRetryableControlChannelErrors(),
		}

		// add a jitter to the first control-channel call
		maxDelayMillis := int64(float64(mgsConfig.ControlChannelRetryInitialDelayMillis) * mgsConfig.RetryJitterRatio)
		delayWithJitter(maxDelayMillis)

		retryer.Init()
		if _, err := retryer.Call(); err != nil {
			if ableToOpenMGSConnection != nil {
				atomic.StoreUint32(ableToOpenMGSConnection, 0)
			}
			ssmconnectionchannel.SetConnectionChannel(context, ssmconnectionchannel.MGSFailed)
			log.Errorf("failed to reconnect to the control channel with error: %v", err)
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
		if ableToOpenMGSConnection != nil {
			atomic.StoreUint32(ableToOpenMGSConnection, 0)
		}
		ssmconnectionchannel.SetConnectionChannel(context, ssmconnectionchannel.MGSFailed)
		log.Errorf("failed to initialize websocket channel for controlchannel, error: %s", err)
		return err
	}
	return nil
}

// SendMessage sends a message to the service through control channel.
func (controlChannel *ControlChannel) SendMessage(log log.T, input []byte, inputType int) error {
	// This function may be called even before the control channel is initialized. Hence, this nil check is needed to avoid panic
	// While loading pending and in-progress documents during agent restart, we may receive replies even before the control channel is opened.
	// These replies will be saved in the local disk and sent immediately when the connection established
	if controlChannel.wsChannel == nil {
		return fmt.Errorf("ws not initialized still")
	}
	return controlChannel.wsChannel.SendMessage(log, input, inputType)
}

// Reconnect reconnects a controlchannel.
func (controlChannel *ControlChannel) Reconnect(context context.T, ableToOpenMGSConnection *uint32) error {
	log := context.Log()
	log.Debugf("Reconnecting controlchannel %s", controlChannel.ChannelId)

	if err := controlChannel.wsChannel.Close(log); err != nil {
		log.Warnf("closing controlchannel failed with error: %s", err)
	}

	if err := controlChannel.Open(context, ableToOpenMGSConnection); err != nil {
		return fmt.Errorf("failed to reconnect controlchannel with error: %s", err)
	}

	if ableToOpenMGSConnection != nil {
		atomic.StoreUint32(ableToOpenMGSConnection, 1)
	}
	ssmconnectionchannel.SetConnectionChannel(context, ssmconnectionchannel.MGSSuccess)
	log.Debugf("Successfully reconnected with controlchannel with type %s", controlChannel.channelType)
	return nil
}

// Close closes controlchannel - its web socket connection.
func (controlChannel *ControlChannel) Close(log log.T) error {
	log.Infof("Closing controlchannel with channel Id %s", controlChannel.ChannelId)
	if controlChannel.AuditLogScheduler != nil {
		controlChannel.AuditLogScheduler.StopScheduler()
	}
	if controlChannel.wsChannel != nil {
		return controlChannel.wsChannel.Close(log)
	}
	return nil
}

// Open opens a websocket connection and sends the token for service to acknowledge the connection.
func (controlChannel *ControlChannel) Open(context context.T, ableToOpenMGSConnection *uint32) error {
	log := context.Log()
	controlChannelDialerInput := &websocket.Dialer{
		TLSClientConfig: network.GetDefaultTLSConfig(log, controlChannel.context.AppConfig()),
		Proxy:           http.ProxyFromEnvironment,
		WriteBufferSize: mgsConfig.ControlChannelWriteBufferSizeLimit,
	}
	if err := controlChannel.wsChannel.Open(log, controlChannelDialerInput); err != nil {
		if ableToOpenMGSConnection != nil {
			atomic.StoreUint32(ableToOpenMGSConnection, 0)
		}
		ssmconnectionchannel.SetConnectionChannel(context, ssmconnectionchannel.MGSFailed)
		return fmt.Errorf("failed to connect controlchannel with error: %s", err)
	}

	uuid.SwitchFormat(uuid.CleanHyphen)
	uid := uuid.NewV4().String()

	instancePlatformType, _ := platform.PlatformType(log)

	openControlChannelInput := service.OpenControlChannelInput{
		MessageSchemaVersion:   aws.String(mgsConfig.MessageSchemaVersion),
		RequestId:              aws.String(uid),
		TokenValue:             aws.String(controlChannel.wsChannel.GetChannelToken()),
		AgentVersion:           aws.String(version.Version),
		PlatformType:           aws.String(instancePlatformType),
		RequireAcknowledgement: aws.Bool(true),
	}

	jsonValue, err := json.Marshal(openControlChannelInput)
	if err != nil {
		return fmt.Errorf("error serializing openControlChannelInput: %s", err)
	}

	if err = controlChannel.SendMessage(log, jsonValue, websocket.TextMessage); err != nil {
		return err
	}

	select {
	case ready := <-controlChannel.readyMessageChan:
		log.Infof("Control channel ready message received: %v", ready)
		controlChannel.AuditLogScheduler.SendAuditMessage()
		return nil
	case <-time.After(mgsConfig.ControlChannelReadyTimeout):
		log.Errorf("Did not receive control channel ready notification before the timeout")
		return fmt.Errorf("control channel readiness check timed out")
	}
}

// controlChannelIncomingMessageHandler handles the incoming messages coming to the agent.
func controlChannelIncomingMessageHandler(context context.T,
	rawMessage []byte,
	incomingAgentMessageChan chan mgsContracts.AgentMessage,
	readyMessageChan chan bool) error {

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
	log.Infof("received message through control channel %v, message type: %s", agentMessage.MessageId, agentMessage.MessageType)

	if agentMessage.MessageType == mgsContracts.ControlChannelReady {
		select {
		case readyMessageChan <- true:
			log.Tracef("Send true to readyMessageChan")
		case <-time.After(mgsConfig.ControlChannelReadyTimeout):
			log.Warnf("The control_channel_ready message is not processed before the timeout. Break from select statement.")
		}
	} else {
		incomingAgentMessageChan <- *agentMessage
	}
	return nil
}

// getControlChannelToken calls CreateControlChannel to get the token for this instance
func getControlChannelToken(context context.T,
	mgsService service.Service,
	instanceId string,
	requestId string,
	ableToOpenMGSConnection *uint32) (tokenValue string, err error) {

	const accessDeniedErr string = "<AccessDeniedException>"

	createControlChannelInput := &service.CreateControlChannelInput{
		MessageSchemaVersion: aws.String(mgsConfig.MessageSchemaVersion),
		RequestId:            aws.String(requestId),
	}
	log := context.Log()
	createControlChannelOutput, err := mgsService.CreateControlChannel(log, createControlChannelInput, instanceId)
	if err != nil || createControlChannelOutput == nil {
		if ableToOpenMGSConnection != nil {
			atomic.StoreUint32(ableToOpenMGSConnection, 0)
		}
		// checks whether CreateControlChannel throws AccessDenied
		if err != nil && strings.Contains(err.Error(), accessDeniedErr) {
			ssmconnectionchannel.SetConnectionChannel(context, ssmconnectionchannel.MGSFailedDueToAccessDenied)
		} else {
			ssmconnectionchannel.SetConnectionChannel(context, ssmconnectionchannel.MGSFailed)
		}
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

// getNonRetryableControlChannelErrors returns list of non retryable errors for control channel retry strategy
func getNonRetryableControlChannelErrors() []string {
	return []string{}
}
