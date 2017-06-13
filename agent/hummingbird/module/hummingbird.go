// Copyright 2017 Amazon.com, Inc. or its affiliates. All Rights Reserved.
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

// Package module implements the core module to start connection with HummingBird.
package module

import (
	"github.com/aws/amazon-ssm-agent/agent/appconfig"
	"github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/aws/amazon-ssm-agent/agent/contracts"
	"github.com/aws/amazon-ssm-agent/agent/hummingbird/service"
	"github.com/aws/amazon-ssm-agent/agent/platform"
	"github.com/gorilla/websocket"
)

// HummingBird encapsulates the logic on configuring, starting and stopping core modules
type HummingBird struct {
	context   context.T
	config    contracts.AgentConfiguration
	name      string
	mfsConfig appconfig.MfsCfg
	service   service.Service
	channel   Channel
}

type Channel struct {
	channelId  string
	connection *websocket.Conn
}

const (
	//TODO will change name
	name = "HummingBird"
)

// NewHummingBird gets HM core module that will manage the websocket connection between Agent and HM service.
func NewHummingBird(context context.T) *HummingBird {

	hummingBirdContext := context.With("[" + name + "]")
	log := hummingBirdContext.Log()
	config := context.AppConfig()

	instanceID, err := platform.InstanceID()
	if instanceID == "" {
		log.Errorf("no instanceID provided, %v", err)
		return nil
	}

	agentInfo := contracts.AgentInfo{
		Lang:      config.Os.Lang,
		Name:      config.Agent.Name,
		Version:   config.Agent.Version,
		Os:        config.Os.Name,
		OsVersion: config.Os.Version,
	}

	agentConfig := contracts.AgentConfiguration{
		AgentInfo:  agentInfo,
		InstanceID: instanceID,
	}

	mfsService := service.NewService(config.Agent.Region, config.Mfs.Endpoint, nil)

	return &HummingBird{
		context:   hummingBirdContext,
		config:    agentConfig,
		name:      name,
		mfsConfig: config.Mfs,
		service:   mfsService}
}

// ICoreModule implementation

// ModuleName returns the name of module
func (h *HummingBird) ModuleName() string {
	return name
}

// ModuleExecute starts the scheduling of the health check module
func (h *HummingBird) ModuleExecute(context context.T) (err error) {

	log := h.context.Log()

	instanceID, err := platform.InstanceID()
	if err != nil {
		log.Errorf("no instanceID provided, %v", err)
		return
	}

	channelId, err := h.service.CreateChannel(log, instanceID)
	if err != nil {
		log.Errorf("cannot create connection to MFS, %v", err)
		return
	}

	channel, err := h.service.GetChannel(log, channelId)
	if err != nil {
		log.Errorf("cannot upgrade to web socket connection to MFS, %v", err)
		return
	}

	h.channel = Channel{
		channelId:  channelId,
		connection: channel}

	return nil
}

// RequestStop handles the termination of the web socket plugin job
func (h *HummingBird) ModuleRequestStop(stopType contracts.StopType) (err error) {
	return nil
}
