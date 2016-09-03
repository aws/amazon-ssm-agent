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

// Package parser contains utilities for parsing and encoding MDS/SSM messages.
package parser

import (
	"encoding/json"

	"github.com/aws/amazon-ssm-agent/agent/contracts"
	"github.com/aws/amazon-ssm-agent/agent/log"
	messageContracts "github.com/aws/amazon-ssm-agent/agent/message/contracts"
	"github.com/aws/amazon-ssm-agent/agent/parameters"
)

// ParseMessageWithParams parses an MDS message and replaces the parameters where needed.
func ParseMessageWithParams(log log.T, payload string) (parsedMessage messageContracts.SendCommandPayload, err error) {
	// parse message to retrieve parameters
	err = json.Unmarshal([]byte(payload), &parsedMessage)
	if err != nil {
		log.Errorf("Encountered error while parsing SendCommandPayload")
		return
	}

	parameters := parameters.ValidParameters(log, parsedMessage.Parameters)

	// add default values for missing parameters
	for k, v := range parsedMessage.DocumentContent.Parameters {
		if _, ok := parameters[k]; !ok {
			parameters[k] = v.DefaultVal
		}
	}

	parsedMessage.DocumentContent.RuntimeConfig = ReplacePluginParameters(parsedMessage.DocumentContent.RuntimeConfig, parameters, log)
	return
}

// PrepareReplyPayloadToUpdateDocumentStatus creates the payload object for SendReply based on document status change.
func PrepareReplyPayloadToUpdateDocumentStatus(agentInfo contracts.AgentInfo, documentStatus contracts.ResultStatus, documentTraceOutput string) (payload messageContracts.SendReplyPayload) {
	payload = messageContracts.SendReplyPayload{
		AdditionalInfo: contracts.AdditionalInfo{
			Agent: agentInfo,
		},
		DocumentStatus:      documentStatus,
		DocumentTraceOutput: documentTraceOutput,
		RuntimeStatus:       nil,
	}
	return
}

// ReplacePluginParameters replaces parameters with their values, within the plugin Properties.
func ReplacePluginParameters(input map[string]*contracts.PluginConfig, params map[string]interface{}, logger log.T) (result map[string]*contracts.PluginConfig) {
	result = make(map[string]*contracts.PluginConfig)
	for pluginName, pluginConfig := range input {
		result[pluginName] = &contracts.PluginConfig{
			Settings:   parameters.ReplaceParameters(pluginConfig.Settings, params, logger),
			Properties: parameters.ReplaceParameters(pluginConfig.Properties, params, logger),
		}
	}
	return
}
