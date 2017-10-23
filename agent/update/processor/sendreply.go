// Copyright 2016 Amazon.com, Inc. or its affiliates. All Rights Reserved.
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

// Package processor contains the methods for update ssm agent.
// It also provides methods for sendReply and updateInstanceInfo
package processor

import (
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/aws/amazon-ssm-agent/agent/appconfig"
	"github.com/aws/amazon-ssm-agent/agent/contracts"
	"github.com/aws/amazon-ssm-agent/agent/framework/processor/executer/iohandler"
	"github.com/aws/amazon-ssm-agent/agent/log"
	messageContracts "github.com/aws/amazon-ssm-agent/agent/runcommand/contracts"
	messageService "github.com/aws/amazon-ssm-agent/agent/runcommand/mds"
	"github.com/aws/amazon-ssm-agent/agent/times"
)

var msgSvc messageService.Service
var msgSvcOnce sync.Once

var newMsgSvc = messageService.NewService
var getAppConfig = appconfig.Config

// Service is an interface represents for SendReply, UpdateInstanceInfo
type Service interface {
	SendReply(log log.T, update *UpdateDetail) error
	DeleteMessage(log log.T, update *UpdateDetail) error
	UpdateHealthCheck(log log.T, update *UpdateDetail, errorCode string) error
}

type svcManager struct{}

// SendReply sends message back to the service
func (s *svcManager) SendReply(log log.T, update *UpdateDetail) (err error) {
	var svc messageService.Service
	var config appconfig.SsmagentConfig
	payloadB := []byte{}

	if config, err = getAppConfig(false); err != nil {
		return fmt.Errorf("could not load config file %v", err.Error())
	}

	value := prepareReplyPayload(config, update)
	if payloadB, err = json.Marshal(value); err != nil {
		return fmt.Errorf("could not marshal reply payload %v", err.Error())
	}
	if svc, err = getMsgSvc(config); err != nil {
		return fmt.Errorf("could not load message service %v", err.Error())
	}

	payload := string(payloadB)
	return svc.SendReply(log, update.MessageID, payload)
}

// DeleteMessage calls the DeleteMessage MDS API.
func (s *svcManager) DeleteMessage(log log.T, update *UpdateDetail) (err error) {
	var svc messageService.Service
	var config appconfig.SsmagentConfig

	if config, err = getAppConfig(false); err != nil {
		return fmt.Errorf("could not load config file %v", err.Error())
	}
	if svc, err = getMsgSvc(config); err != nil {
		return fmt.Errorf("could not load message service %v", err)
	}

	return svc.DeleteMessage(log, update.MessageID)
}

// getMsgSvc gets cached message service
func getMsgSvc(config appconfig.SsmagentConfig) (svc messageService.Service, err error) {
	msgSvcOnce.Do(func() {
		connectionTimeout := time.Duration(config.Mds.StopTimeoutMillis) * time.Millisecond
		msgSvc = newMsgSvc(
			config.Agent.Region,
			config.Mds.Endpoint,
			nil,
			connectionTimeout)
	})

	if msgSvc == nil {
		return nil, fmt.Errorf("couldn't create message service")
	}
	return msgSvc, nil
}

// prepareReplyPayload setups the reply payload
func prepareReplyPayload(config appconfig.SsmagentConfig, update *UpdateDetail) (payload *messageContracts.SendReplyPayload) {
	runtimeStatuses := make(map[string]*contracts.PluginRuntimeStatus)
	rs := prepareRuntimeStatus(update)
	runtimeStatuses[appconfig.PluginNameAwsAgentUpdate] = &rs
	agentInfo := contracts.AgentInfo{
		Lang:      config.Os.Lang,
		Name:      config.Agent.Name,
		Version:   config.Agent.Version,
		Os:        config.Os.Name,
		OsVersion: config.Os.Version,
	}

	payload = &messageContracts.SendReplyPayload{
		AdditionalInfo: contracts.AdditionalInfo{
			Agent:    agentInfo,
			DateTime: times.ToIso8601UTC(time.Now()),
		},
		DocumentStatus:      rs.Status,
		DocumentTraceOutput: "",
		RuntimeStatus:       runtimeStatuses,
	}

	return payload
}

// prepareRuntimeStatus creates the structure for the runtimeStatus section of the payload of SendReply
// for a particular plugin.
func prepareRuntimeStatus(update *UpdateDetail) contracts.PluginRuntimeStatus {
	// Set default as failed, this will help us catch issues more proactively
	pluginStatus := update.Result
	code := 0
	if pluginStatus == contracts.ResultStatusFailed {
		code = 1
	}

	output := iohandler.TruncateOutput(update.StandardOut,
		update.StandardError,
		iohandler.MaximumPluginOutputSize)

	return contracts.PluginRuntimeStatus{
		Code:               code,
		Status:             pluginStatus,
		Output:             output,
		OutputS3BucketName: update.OutputS3BucketName,
		OutputS3KeyPrefix:  update.OutputS3KeyPrefix,
		StartDateTime:      times.ToIso8601UTC(update.StartDateTime),
		EndDateTime:        times.ToIso8601UTC(time.Now()),
	}
}
