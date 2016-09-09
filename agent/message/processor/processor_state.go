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
	messageContracts "github.com/aws/amazon-ssm-agent/agent/message/contracts"
	"github.com/aws/amazon-ssm-agent/agent/times"
	"github.com/aws/aws-sdk-go/service/ssmmds"
)

// initializes CommandState - an interim state that is used around during an execution of a command
func initializeSendCommandState(pluginConfigurations map[string]*contracts.Configuration, msg ssmmds.Message, parsedMsg messageContracts.SendCommandPayload) messageContracts.DocumentState {

	//initialize document information with relevant values extracted from msg
	documentInfo := newDocumentInfo(msg, parsedMsg)

	//initialize plugin states
	pluginsInfo := make(map[string]messageContracts.PluginState)

	for key, value := range pluginConfigurations {
		var plugin messageContracts.PluginState
		plugin.Configuration = *value
		plugin.HasExecuted = false
		pluginsInfo[key] = plugin
	}

	//initialize command State
	return messageContracts.DocumentState{
		DocumentInformation: documentInfo,
		PluginsInformation:  pluginsInfo,
		DocumentType:        messageContracts.SendCommand,
	}
}

// initializes CancelCommandState
func initializeCancelCommandState(msg ssmmds.Message, parsedMsg messageContracts.CancelPayload) messageContracts.DocumentState {
	documentInfo := messageContracts.DocumentInfo{}
	documentInfo.Destination = *msg.Destination
	documentInfo.CreatedDate = *msg.CreatedDate
	documentInfo.MessageID = *msg.MessageId
	documentInfo.CommandID = getCommandID(*msg.MessageId)
	documentInfo.RunID = times.ToIsoDashUTC(times.DefaultClock.Now())
	documentInfo.DocumentStatus = contracts.ResultStatusInProgress

	cancelCommand := new(messageContracts.CancelCommandInfo)
	cancelCommand.Payload = *msg.Payload
	cancelCommand.CancelMessageID = parsedMsg.CancelMessageID
	commandID := getCommandID(parsedMsg.CancelMessageID)

	cancelCommand.CancelCommandID = commandID
	cancelCommand.DebugInfo = fmt.Sprintf("Command %v is yet to be cancelled", commandID)

	return messageContracts.DocumentState{
		DocumentInformation: documentInfo,
		CancelInformation:   *cancelCommand,
		DocumentType:        messageContracts.SendCommand,
	}
}
