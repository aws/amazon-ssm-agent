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
	"fmt"

	"github.com/aws/amazon-ssm-agent/agent/contracts"
	"github.com/aws/amazon-ssm-agent/agent/log"
	messageContracts "github.com/aws/amazon-ssm-agent/agent/message/contracts"
	"github.com/aws/amazon-ssm-agent/agent/parameters"
	"github.com/aws/amazon-ssm-agent/agent/parameterstore"
)

// ParseMessageWithParams parses an MDS message and replaces the parameters where needed.
func ParseMessageWithParams(log log.T, payload string) (parsedMessage messageContracts.SendCommandPayload, err error) {
	// parse message to retrieve parameters
	err = json.Unmarshal([]byte(payload), &parsedMessage)
	if err != nil {
		errorMsg := "Encountered error while parsing input - internal error"
		log.Errorf(errorMsg)
		return parsedMessage, fmt.Errorf("%v", errorMsg)
	}

	parameters := parameters.ValidParameters(log, parsedMessage.Parameters)

	// add default values for missing parameters
	for k, v := range parsedMessage.DocumentContent.Parameters {
		if _, ok := parameters[k]; !ok {
			parameters[k] = v.DefaultVal
		}
	}

	err = ReplacePluginParameters(&parsedMessage, parameters, log)
	if err != nil {
		return
	}
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
func ReplacePluginParameters(
	payload *messageContracts.SendCommandPayload,
	params map[string]interface{},
	logger log.T) error {
	var err error

	// Validates SSM parameters
	if err = parameterstore.ValidateSSMParameters(logger, payload.DocumentContent.Parameters, params); err != nil {
		return err
	}

	runtimeConfig := payload.DocumentContent.RuntimeConfig
	// we assume that one of the runtimeConfig and mainSteps should be nil
	if runtimeConfig != nil && len(runtimeConfig) != 0 {
		updatedRuntimeConfig := make(map[string]*contracts.PluginConfig)
		for pluginName, pluginConfig := range runtimeConfig {
			updatedRuntimeConfig[pluginName] = &contracts.PluginConfig{
				Settings:   parameters.ReplaceParameters(pluginConfig.Settings, params, logger),
				Properties: parameters.ReplaceParameters(pluginConfig.Properties, params, logger),
			}

			// Resolves SSM parameters
			if updatedRuntimeConfig[pluginName].Settings, err = parameterstore.Resolve(logger, updatedRuntimeConfig[pluginName].Settings, false); err != nil {
				return err
			}

			// Resolves SSM parameters
			if updatedRuntimeConfig[pluginName].Properties, err = parameterstore.Resolve(logger, updatedRuntimeConfig[pluginName].Properties, false); err != nil {
				return err
			}
		}
		payload.DocumentContent.RuntimeConfig = updatedRuntimeConfig
		return nil
	}

	mainSteps := payload.DocumentContent.MainSteps
	if mainSteps != nil || len(mainSteps) != 0 {
		updatedMainSteps := make([]*contracts.InstancePluginConfig, len(mainSteps))
		for index, instancePluginConfig := range mainSteps {
			updatedMainSteps[index] = &contracts.InstancePluginConfig{
				Action:      instancePluginConfig.Action,
				Name:        instancePluginConfig.Name,
				MaxAttempts: instancePluginConfig.MaxAttempts,
				OnFailure:   instancePluginConfig.OnFailure,
				Timeout:     instancePluginConfig.Timeout,
				Settings:    parameters.ReplaceParameters(instancePluginConfig.Settings, params, logger),
				Inputs:      parameters.ReplaceParameters(instancePluginConfig.Inputs, params, logger),
			}

			// Resolves SSM parameters
			if updatedMainSteps[index].Settings, err = parameterstore.Resolve(logger, updatedMainSteps[index].Settings, false); err != nil {
				return err
			}

			// Resolves SSM parameters
			if updatedMainSteps[index].Inputs, err = parameterstore.Resolve(logger, updatedMainSteps[index].Inputs, false); err != nil {
				return err
			}
		}
		payload.DocumentContent.MainSteps = updatedMainSteps
		return nil
	}
	return nil
}
