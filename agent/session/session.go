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

// Package session implements the core module to start web-socket connection with message gateway service.
package session

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math/rand"
	"runtime/debug"
	"time"

	"github.com/aws/amazon-ssm-agent/agent/appconfig"
	"github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/aws/amazon-ssm-agent/agent/contracts"
	"github.com/aws/amazon-ssm-agent/agent/framework/docmanager"
	"github.com/aws/amazon-ssm-agent/agent/framework/processor"
	"github.com/aws/amazon-ssm-agent/agent/jsonutil"
	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/amazon-ssm-agent/agent/platform"
	mgsConfig "github.com/aws/amazon-ssm-agent/agent/session/config"
	mgsContracts "github.com/aws/amazon-ssm-agent/agent/session/contracts"
	"github.com/aws/amazon-ssm-agent/agent/session/controlchannel"
	"github.com/aws/amazon-ssm-agent/agent/session/retry"
	"github.com/aws/amazon-ssm-agent/agent/session/service"
	"github.com/gorilla/websocket"
	"github.com/twinj/uuid"
)

// Session encapsulates the logic on configuring, starting and stopping core modules
type Session struct {
	context        context.T
	agentConfig    contracts.AgentConfiguration
	name           string
	mgsConfig      appconfig.MgsConfig
	service        service.Service
	controlChannel controlchannel.IControlChannel
	processor      processor.Processor
}

// NewSession gets session core module that manages the web-socket connection between Agent and message gateway service.
func NewSession(context context.T) *Session {
	sessionContext := context.With("[" + mgsConfig.SessionServiceName + "]")
	log := sessionContext.Log()
	appConfig := context.AppConfig()

	instanceID, err := platform.InstanceID()
	if instanceID == "" {
		log.Errorf("no instanceID provided, %s", err)
		return nil
	}

	// If the current os is Nano server, SSM Agent doesn't support the Session Manager.
	isNanoServer, _ := platform.IsPlatformNanoServer(log)
	if isNanoServer {
		log.Info("Session core module is not supported on Windows Nano server.")
		return nil
	}

	agentInfo := contracts.AgentInfo{
		Lang:      appConfig.Os.Lang,
		Name:      appConfig.Agent.Name,
		Version:   appConfig.Agent.Version,
		Os:        appConfig.Os.Name,
		OsVersion: appConfig.Os.Version,
	}

	agentConfig := contracts.AgentConfiguration{
		AgentInfo:  agentInfo,
		InstanceID: instanceID,
	}

	messageGatewayServiceConfig := appConfig.Mgs
	if messageGatewayServiceConfig.Region == "" {
		fetchedRegion, err := platform.Region()
		if err != nil {
			log.Errorf("Failed to get region with error: %s", err)
			return nil
		}
		messageGatewayServiceConfig.Region = fetchedRegion
	}

	if messageGatewayServiceConfig.Endpoint == "" {
		fetchedEndpoint, err := getMgsEndpoint(messageGatewayServiceConfig.Region)
		if err != nil {
			log.Errorf("Failed to get MessageGatewayService endpoint with error: %s", err)
			return nil
		}
		messageGatewayServiceConfig.Endpoint = fetchedEndpoint
	}

	connectionTimeout := time.Duration(messageGatewayServiceConfig.StopTimeoutMillis) * time.Millisecond

	mgsService := service.NewService(log, messageGatewayServiceConfig, connectionTimeout)
	processor := processor.NewEngineProcessor(
		sessionContext,
		messageGatewayServiceConfig.SessionWorkersLimit,
		3, // TODO adjust this value
		[]contracts.DocumentType{contracts.StartSession, contracts.TerminateSession})

	controlChannel := &controlchannel.ControlChannel{}

	return &Session{
		context:        sessionContext,
		agentConfig:    agentConfig,
		name:           mgsConfig.SessionServiceName,
		mgsConfig:      messageGatewayServiceConfig,
		service:        mgsService,
		processor:      processor,
		controlChannel: controlChannel,
	}
}

// ICoreModule implementation

// ModuleName returns the name of module
func (s *Session) ModuleName() string {
	return s.name
}

var setupControlChannel = func(context context.T, service service.Service, processor processor.Processor, instanceId string) (controlchannel.IControlChannel, error) {
	retryer := retry.ExponentialRetryer{
		CallableFunc: func() (channel interface{}, err error) {
			controlChannel := &controlchannel.ControlChannel{}
			controlChannel.Initialize(context, service, processor, instanceId)
			if err := controlChannel.SetWebSocket(context, service, processor, instanceId); err != nil {
				return nil, err
			}

			if err := controlChannel.Open(context.Log()); err != nil {
				return nil, err
			}

			return controlChannel, nil
		},
		GeometricRatio:      mgsConfig.RetryGeometricRatio,
		InitialDelayInMilli: rand.Intn(mgsConfig.ControlChannelRetryInitialDelayMillis) + mgsConfig.ControlChannelRetryInitialDelayMillis,
		MaxDelayInMilli:     mgsConfig.ControlChannelRetryMaxIntervalMillis,
		MaxAttempts:         mgsConfig.ControlChannelNumMaxRetries,
	}

	channel, err := retryer.Call()
	if err != nil {
		// should never happen
		return nil, err
	}
	controlChannel := channel.(*controlchannel.ControlChannel)
	return controlChannel, nil
}

// ModuleExecute starts the scheduling of the session module
func (s *Session) ModuleExecute(context context.T) (err error) {
	log := s.context.Log()
	log.Info("Starting session document processing engine...")

	defer func() {
		if msg := recover(); msg != nil {
			log.Errorf("MessageGatewayService ModuleExecute run panic: %v", msg)
			log.Errorf("%s: %s", msg, debug.Stack())
		}
	}()

	instanceId := s.agentConfig.InstanceID

	resultChan, err := s.processor.Start()
	if err != nil {
		log.Errorf("unable to start session document processor: %s", err)
		return err
	}

	go s.listenReply(resultChan, instanceId)

	log.Info("SSM Agent is trying to setup control channel for Session Manager module.")
	s.controlChannel, err = setupControlChannel(s.context, s.service, s.processor, instanceId)
	if err != nil {
		log.Errorf("Failed to setup control channel, err: %v", err)
		return
	}

	log.Info("Starting receiving message from control channel")

	if err = s.processor.InitialProcessing(); err != nil {
		log.Errorf("initial processing in EngineProcessor encountered error: %v", err)
		return
	}

	return nil
}

// ModuleRequestStop handles the termination of the session module
func (s *Session) ModuleRequestStop(stopType contracts.StopType) (err error) {
	log := s.context.Log()
	log.Infof("Stopping %s.", s.name)
	defer func() {
		if msg := recover(); msg != nil {
			log.Errorf("MessageGatewayService ModuleRequestStop run panic: %v", msg)
			log.Errorf("%s: %s", msg, debug.Stack())
		}
	}()

	if s.controlChannel != nil {
		if err = s.controlChannel.Close(log); err != nil {
			log.Errorf("stopping controlchannel with error, %s", err)
		}
	}

	s.processor.Stop(stopType)

	return nil
}

// listenReply listens document result of session execution.
func (s *Session) listenReply(resultChan chan contracts.DocumentResult, instanceId string) {
	log := s.context.Log()
	log.Info("listening reply.")

	//processor guarantees to close this channel upon stop
	for res := range resultChan {
		if res.LastPlugin != "" {
			log.Infof("received plugin: %s result from Processor", res.LastPlugin)
		} else {
			log.Infof("session: %s complete", res.MessageID)

			//Deleting Old Log Files
			instanceID, _ := platform.InstanceID()
			go docmanager.DeleteSessionOrchestrationDirectories(log,
				instanceID,
				s.context.AppConfig().Agent.OrchestrationRootDir,
				s.context.AppConfig().Ssm.SessionLogsRetentionDurationHours)
		}
		msg, err := buildAgentTaskComplete(log, res, instanceId)
		if err != nil {
			log.Errorf("Cannot build AgentTaskComplete message %s", err)
			return
		}

		// For last document level result, no need to send reply because there will be only one plugin for shell plugin case.
		if msg != nil {
			err = s.controlChannel.SendMessage(log, msg, websocket.BinaryMessage)
			if err != nil {
				log.Errorf("Error sending reply message %v", err)
			}
		}
	}
}

// buildAgentTaskComplete builds AgentTaskComplete message.
func buildAgentTaskComplete(log log.T, res contracts.DocumentResult, instanceId string) (result []byte, err error) {
	uuid.SwitchFormat(uuid.CleanHyphen)
	messageId := uuid.NewV4()
	pluginId := res.LastPlugin
	var taskCompletePayload interface{}
	var messageType string

	// For SessionManager plugins, there is only one plugin in a document.
	// Send AgentTaskComplete when we get the plugin level result, and ignore this document level result.
	// For instance reboot scenarios, it only has document level result with "Failed" status, this result can't be ignored.
	if pluginId == "" && res.Status != contracts.ResultStatusFailed {
		return nil, nil
	}

	messageType = mgsContracts.TaskCompleteMessage
	taskCompletePayload = formatAgentTaskCompletePayload(log, pluginId, res.PluginResults, res.MessageID, instanceId, messageType)
	replyBytes, err := json.Marshal(taskCompletePayload)
	if err != nil {
		// should not happen
		return nil, fmt.Errorf("cannot marshal AgentReply payload to json string: %s, err: %s", taskCompletePayload, err)
	}
	payload := string(replyBytes)
	log.Info("Sending reply ", jsonutil.Indent(payload))

	agentMessage := &mgsContracts.AgentMessage{
		MessageType:    messageType,
		SchemaVersion:  1,
		CreatedDate:    uint64(time.Now().UnixNano() / 1000000),
		SequenceNumber: 0,
		Flags:          0,
		MessageId:      messageId,
		Payload:        replyBytes,
	}

	return agentMessage.Serialize(log)
}

// formatAgentTaskCompletePayload builds AgentTaskComplete message Payload from the total task result.
func formatAgentTaskCompletePayload(log log.T,
	pluginId string,
	pluginResults map[string]*contracts.PluginResult,
	sessionId string,
	instanceId string,
	topic string) mgsContracts.AgentTaskCompletePayload {

	if len(pluginResults) < 1 {
		log.Error("Error in FormatAgentTaskCompletePayload, the outputs map is empty!")
		return mgsContracts.AgentTaskCompletePayload{}
	}

	// get plugin result
	if pluginId == "" {
		// for instance reboot scenarios, it only contains document level result which does not contain pluginId.
		for key := range pluginResults {
			pluginId = key
			break
		}
	}
	pluginResult := pluginResults[pluginId]

	if pluginResult == nil {
		log.Error("Error in FormatAgentTaskCompletePayload, the pluginOutput is nil!")
		return mgsContracts.AgentTaskCompletePayload{}
	}

	sessionPluginResultOutput := mgsContracts.SessionPluginResultOutput{}
	if pluginResult.Error != "" {
		sessionPluginResultOutput.Output = pluginResult.Error
	} else if pluginResult.Output != nil {
		if err := jsonutil.Remarshal(pluginResult.Output, &sessionPluginResultOutput); err != nil {
			sessionPluginResultOutput.Output = fmt.Sprintf("%v", pluginResult.Output)
		}
	}

	payload := mgsContracts.AgentTaskCompletePayload{
		SchemaVersion:    1,
		TaskId:           sessionId,
		Topic:            topic,
		FinalTaskStatus:  string(pluginResult.Status),
		IsRoutingFailure: false,
		AwsAccountId:     "",
		InstanceId:       instanceId,
		Output:           sessionPluginResultOutput.Output,
		S3Bucket:         sessionPluginResultOutput.S3Bucket,
		S3UrlSuffix:      sessionPluginResultOutput.S3UrlSuffix,
		CwlGroup:         sessionPluginResultOutput.CwlGroup,
		CwlStream:        sessionPluginResultOutput.CwlStream,
	}
	return payload
}

// getMgsEndpoint builds mgs endpoint.
func getMgsEndpoint(region string) (string, error) {
	hostName := mgsConfig.GetMgsEndpointFromRip(region)
	if hostName == "" {
		return "", fmt.Errorf("no MGS endpoint found for region %s", region)
	}
	var endpointBuilder bytes.Buffer
	endpointBuilder.WriteString(mgsConfig.HttpsPrefix)
	endpointBuilder.WriteString(hostName)
	return endpointBuilder.String(), nil
}
