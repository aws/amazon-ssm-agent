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
	gocontext "context"
	"encoding/json"
	"fmt"
	"math/rand"
	"net"
	"net/http"
	"strings"
	"sync/atomic"
	"time"

	"github.com/google/uuid"

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
	telemetryV2 "github.com/aws/amazon-ssm-agent/agent/telemetry/exporter/control_channel_exporter"
	"github.com/aws/amazon-ssm-agent/agent/version"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/gorilla/websocket"
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
	TelemetryExporter               telemetryV2.ITelemetryExporter
	channelType                     string
	agentMessageIncomingMessageChan chan mgsContracts.AgentMessage
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

	if !context.AppConfig().Agent.GlobalEnhancedTelemetryEnabled {
		log.Info("Agent GlobalEnhancedTelemetry is disabled, hence not adding telemetryExporter to control channel")
	} else {
		controlChannel.TelemetryExporter = telemetryV2.GetControlChannelTelemetryExporter(context, controlChannel.wsChannel)
	}

	controlChannel.agentMessageIncomingMessageChan = agentMessageIncomingMessageChan
	controlChannel.context = context
	log.Debugf("Initialized controlchannel for instance: %s", instanceId)
}

// SetWebSocket populates webchannel object.
func (controlChannel *ControlChannel) SetWebSocket(context context.T,
	mgsService service.Service, ableToOpenMGSConnection *uint32) error {

	log := context.Log()

	log.Infof("Setting up websocket for controlchannel for instance: %s", controlChannel.ChannelId)

	onMessageHandler := func(input []byte) {
		controlChannelIncomingMessageHandler(context, input, controlChannel.agentMessageIncomingMessageChan)
	}
	onErrorHandler := func(err error) {
		callable := func() (channel interface{}, err error) {
			// Token is minted inside Open() after successful dial, so we only call Reconnect here
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

	// Token is empty here; it will be minted inside Open() after successful websocket dial.
	// The token param is retained because the shared IWebSocketChannel interface is also
	// used by DataChannel which consumes the Initialize-time token directly.
	if err := controlChannel.wsChannel.Initialize(context,
		controlChannel.ChannelId,
		mgsConfig.ControlChannel,
		mgsConfig.RoleSubscribe,
		"", // token minted in Open() after dial for control channel
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
	if controlChannel.TelemetryExporter != nil {
		controlChannel.TelemetryExporter.StopExporter()
	}
	if controlChannel.wsChannel != nil {
		return controlChannel.wsChannel.Close(log)
	}
	return nil
}

// Open opens a websocket connection and sends the token for service to acknowledge the connection.
// The control-channel token is minted AFTER the websocket dial succeeds, immediately before
// the handshake, to avoid token expiry during slow/failing dials (e.g., AZ faults).
func (controlChannel *ControlChannel) Open(context context.T, ableToOpenMGSConnection *uint32) error {
	log := context.Log()
	appCfg := controlChannel.context.AppConfig()

	// Build the dialer with configurable dial timeout via NetDialContext.
	// A timeout of 0 means no timeout (pre-fix behavior / intentional opt-out).
	// HandshakeTimeout = dialTimeout + offset (always > dial timeout when enabled),
	// so the per-AZ connect division under Dialer.Timeout is preserved and the
	// upgrade gets a guaranteed >= offset floor. When disabled (0), HandshakeTimeout is also 0.
	dialTimeoutMillis := appCfg.Mgs.ControlChannelDialTimeoutMillis
	var netDialContext func(ctx gocontext.Context, network, addr string) (net.Conn, error)
	var handshakeTimeout time.Duration
	if dialTimeoutMillis > 0 {
		dialTimeout := time.Duration(dialTimeoutMillis) * time.Millisecond
		handshakeTimeout = dialTimeout + mgsConfig.ControlChannelHandshakeTimeoutOffset
		netDialContext = func(ctx gocontext.Context, network, addr string) (net.Conn, error) {
			d := &net.Dialer{Timeout: dialTimeout}
			return d.DialContext(ctx, network, addr)
		}
	}
	// When dialTimeoutMillis <= 0: netDialContext = nil, handshakeTimeout = 0 (both unbounded)

	controlChannelDialerInput := &websocket.Dialer{
		TLSClientConfig:  network.GetDefaultTLSConfig(log, appCfg),
		Proxy:            http.ProxyFromEnvironment,
		WriteBufferSize:  mgsConfig.ControlChannelWriteBufferSizeLimit,
		HandshakeTimeout: handshakeTimeout,
		NetDialContext:   netDialContext,
	}
	if err := controlChannel.wsChannel.Open(log, controlChannelDialerInput); err != nil {
		if ableToOpenMGSConnection != nil {
			atomic.StoreUint32(ableToOpenMGSConnection, 0)
		}
		ssmconnectionchannel.SetConnectionChannel(context, ssmconnectionchannel.MGSFailed)
		return fmt.Errorf("failed to connect controlchannel with error: %s", err)
	}

	// Mint the token AFTER successful dial, immediately before the handshake.
	// This closes the window where a token minted pre-dial can expire during a slow/failing dial.
	uid := uuid.New().String()
	tokenValue, err := getControlChannelToken(context, controlChannel.Service, controlChannel.ChannelId, uid, ableToOpenMGSConnection)
	if err != nil {
		log.Errorf("Failed to get controlchannel token after dial, error: %s", err)
		// Close the just-dialed socket to reap listener/pinger goroutines
		if closeErr := controlChannel.wsChannel.Close(log); closeErr != nil {
			log.Warnf("failed to close websocket after token mint failure: %s", closeErr)
		}
		return fmt.Errorf("failed to get controlchannel token: %s", err)
	}
	controlChannel.wsChannel.SetChannelToken(tokenValue)

	openControlChannelInput := service.OpenControlChannelInput{
		MessageSchemaVersion: aws.String(mgsConfig.MessageSchemaVersion),
		RequestId:            aws.String(uid),
		TokenValue:           aws.String(controlChannel.wsChannel.GetChannelToken()),
		AgentVersion:         aws.String(version.Version),
		PlatformType:         aws.String(platform.PlatformType(log)),
	}

	jsonValue, err := json.Marshal(openControlChannelInput)
	if err != nil {
		// Close socket on marshal failure to prevent resource leak
		if closeErr := controlChannel.wsChannel.Close(log); closeErr != nil {
			log.Warnf("failed to close websocket after marshal failure: %s", closeErr)
		}
		// Reset health state for MDS-fallback parity with dial-failure path
		if ableToOpenMGSConnection != nil {
			atomic.StoreUint32(ableToOpenMGSConnection, 0)
		}
		ssmconnectionchannel.SetConnectionChannel(context, ssmconnectionchannel.MGSFailed)
		return fmt.Errorf("error serializing openControlChannelInput: %s", err)
	}

	if err = controlChannel.SendMessage(log, jsonValue, websocket.TextMessage); err == nil {
		controlChannel.AuditLogScheduler.SendAuditMessage()
	} else {
		// Close socket on send failure to reap listener/pinger goroutines
		if closeErr := controlChannel.wsChannel.Close(log); closeErr != nil {
			log.Warnf("failed to close websocket after send failure: %s", closeErr)
		}
		// Reset health state for MDS-fallback parity with dial-failure path
		if ableToOpenMGSConnection != nil {
			atomic.StoreUint32(ableToOpenMGSConnection, 0)
		}
		ssmconnectionchannel.SetConnectionChannel(context, ssmconnectionchannel.MGSFailed)
	}
	return err
}

// controlChannelIncomingMessageHandler handles the incoming messages coming to the agent.
func controlChannelIncomingMessageHandler(context context.T,
	rawMessage []byte,
	incomingAgentMessageChan chan mgsContracts.AgentMessage) error {

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
	log.Debugf("received message through control channel %v", agentMessage.MessageId)
	incomingAgentMessageChan <- *agentMessage
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
