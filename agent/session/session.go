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
	"runtime/debug"
	"time"

	"github.com/aws/amazon-ssm-agent/agent/appconfig"
	"github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/aws/amazon-ssm-agent/agent/contracts"
	"github.com/aws/amazon-ssm-agent/agent/framework/processor"
	"github.com/aws/amazon-ssm-agent/agent/platform"
	mgsConfig "github.com/aws/amazon-ssm-agent/agent/session/config"
	"github.com/aws/amazon-ssm-agent/agent/session/controlchannel"
	"github.com/aws/amazon-ssm-agent/agent/session/service"
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
		fetchedEndpoint, err := getMgsEndpoint()
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

	if err = s.processor.InitialProcessing(); err != nil {
		log.Errorf("initial processing in EngineProcessor encountered error: %v", err)
		return
	}

	// TODO: add retry for create/open controlchannel
	s.controlChannel.Initialize(s.context, s.service, s.processor, instanceId)
	if s.controlChannel.SetWebSocket(s.context, s.service, s.processor, instanceId); err != nil {
		log.Errorf("failed to populate websocket for controlchannel, error %s", err)
	}
	if err := s.controlChannel.Open(s.context.Log()); err != nil {
		log.Errorf("failed to open controlchannel, error %s", err)
	}

	log.Info("Starting receiving message from control channel")
	return nil
}

// ModuleRequestStop handles the termination of the session module
func (s *Session) ModuleRequestStop(stopType contracts.StopType) (err error) {
	log := s.context.Log()
	log.Infof("Stopping %s.", s.name)

	if s.controlChannel != nil {
		err = s.controlChannel.Close(log)
		log.Errorf("stopping controlchannel with error, %s", err)
	}

	s.processor.Stop(stopType)

	return nil
}

// listenReply listens document result of session execution.
func (s *Session) listenReply(resultChan chan contracts.DocumentResult, instanceId string) {
	log := s.context.Log()
	log.Info("listening reply.")
	// TODO:add implementation.
}

// getMgsEndpoint builds mgs endpoint.
func getMgsEndpoint() (string, error) {
	hostName, err := mgsConfig.GetHostName()
	if err != nil {
		return "", err
	}
	var endpointBuilder bytes.Buffer
	endpointBuilder.WriteString(mgsConfig.HttpsPrefix)
	endpointBuilder.WriteString(hostName)
	return endpointBuilder.String(), err
}
