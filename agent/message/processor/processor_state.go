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

// Package processor implements MDS plugin processor
// processor_state contains utilities that interact with the state manager
package processor

import (
	"fmt"

	"github.com/aws/amazon-ssm-agent/agent/contracts"
	"github.com/aws/amazon-ssm-agent/agent/fileutil"
	messageContracts "github.com/aws/amazon-ssm-agent/agent/message/contracts"
	"github.com/aws/amazon-ssm-agent/agent/message/converter"
	stateModel "github.com/aws/amazon-ssm-agent/agent/statemanager/model"
	"github.com/aws/amazon-ssm-agent/agent/times"
	"github.com/aws/aws-sdk-go/service/ssmmds"
)

// initializes CommandState - an interim state that is used around during an execution of a command
func initializeSendCommandState(
	payload messageContracts.SendCommandPayload,
	orchestrationDir string,
	s3KeyPrefix string,
	msg ssmmds.Message) stateModel.DocumentState {

	//initialize document information with relevant values extracted from msg
	documentInfo := newDocumentInfo(msg, payload)
	//initialize command State
	docState := stateModel.DocumentState{
		DocumentInformation: documentInfo,
		DocumentType:        stateModel.SendCommand,
	}

	if payload.DocumentContent.RuntimeConfig != nil && len(payload.DocumentContent.RuntimeConfig) != 0 {
		pluginsInfo := initializeSendCommandStateWithRuntimeConfig(payload, orchestrationDir, s3KeyPrefix, documentInfo.MessageID)
		docState.InstancePluginsInformation = converter.ConvertPluginState(pluginsInfo)
		return docState
	}

	if payload.DocumentContent.MainSteps != nil && len(payload.DocumentContent.MainSteps) != 0 {
		instancePluginsInfo := initializeSendCommandStateWithMainStep(payload, orchestrationDir, s3KeyPrefix, documentInfo.MessageID)
		docState.InstancePluginsInformation = instancePluginsInfo
		return docState
	}

	return docState
}

// initializes CancelCommandState - an interim state that is used during a command cancelling
func initializeCancelCommandState(msg ssmmds.Message, parsedMsg messageContracts.CancelPayload) stateModel.DocumentState {
	documentInfo := stateModel.DocumentInfo{}
	documentInfo.InstanceID = *msg.Destination
	documentInfo.CreatedDate = *msg.CreatedDate
	documentInfo.MessageID = *msg.MessageId
	documentInfo.DocumentID = getCommandID(*msg.MessageId)
	documentInfo.RunID = times.ToIsoDashUTC(times.DefaultClock.Now())
	documentInfo.DocumentStatus = contracts.ResultStatusInProgress

	cancelCommand := new(stateModel.CancelCommandInfo)
	cancelCommand.Payload = *msg.Payload
	cancelCommand.CancelMessageID = parsedMsg.CancelMessageID
	commandID := getCommandID(parsedMsg.CancelMessageID)

	cancelCommand.CancelCommandID = commandID
	cancelCommand.DebugInfo = fmt.Sprintf("Command %v is yet to be cancelled", commandID)

	return stateModel.DocumentState{
		DocumentInformation: documentInfo,
		CancelInformation:   *cancelCommand,
		DocumentType:        stateModel.CancelCommand,
	}
}

// initializeSendCommandStateWithRuntimeConfig initializes pluginsInfo for the docState. Used for document v1.0 and 1.2
func initializeSendCommandStateWithRuntimeConfig(
	payload messageContracts.SendCommandPayload,
	orchestrationDir string,
	s3KeyPrefix string,
	messageID string) (pluginsInfo map[string]stateModel.PluginState) {

	//initialize plugin states as map
	pluginsInfo = make(map[string]stateModel.PluginState)
	// getPluginConfigurations converts from PluginConfig (structure from the MDS message) to plugin.Configuration (structure expected by the plugin)
	pluginConfigurations := make(map[string]*contracts.Configuration)
	for pluginName, pluginConfig := range payload.DocumentContent.RuntimeConfig {
		config := contracts.Configuration{
			Settings:               pluginConfig.Settings,
			Properties:             pluginConfig.Properties,
			OutputS3BucketName:     payload.OutputS3BucketName,
			OutputS3KeyPrefix:      fileutil.BuildS3Path(s3KeyPrefix, pluginName),
			OrchestrationDirectory: fileutil.BuildPath(orchestrationDir, pluginName),
			MessageId:              messageID,
			BookKeepingFileName:    payload.CommandID,
		}
		pluginConfigurations[pluginName] = &config
	}

	for key, value := range pluginConfigurations {
		var plugin stateModel.PluginState
		plugin.Configuration = *value
		plugin.HasExecuted = false
		plugin.Id = key
		plugin.Name = key
		pluginsInfo[key] = plugin
	}
	return
}

// initializeSendCommandStateWithMainStep initializes instancePluginsInfo for the docState. Used by document v2.0.
func initializeSendCommandStateWithMainStep(
	payload messageContracts.SendCommandPayload,
	orchestrationDir string,
	s3KeyPrefix string,
	messageID string) (instancePluginsInfo []stateModel.PluginState) {

	//initialize plugin states as array
	instancePluginsInfo = make([]stateModel.PluginState, len(payload.DocumentContent.MainSteps))

	// getPluginConfigurations converts from PluginConfig (structure from the MDS message) to plugin.Configuration (structure expected by the plugin)
	for index, instancePluginConfig := range payload.DocumentContent.MainSteps {
		pluginName := instancePluginConfig.Action
		config := contracts.Configuration{
			Settings:               instancePluginConfig.Settings,
			Properties:             instancePluginConfig.Inputs,
			OutputS3BucketName:     payload.OutputS3BucketName,
			OutputS3KeyPrefix:      fileutil.BuildS3Path(s3KeyPrefix, pluginName),
			OrchestrationDirectory: fileutil.BuildPath(orchestrationDir, pluginName),
			MessageId:              messageID,
			BookKeepingFileName:    payload.CommandID,
		}

		var plugin stateModel.PluginState
		plugin.Configuration = config
		plugin.HasExecuted = false
		plugin.Id = instancePluginConfig.Name
		plugin.Name = pluginName
		instancePluginsInfo[index] = plugin
	}
	return
}
