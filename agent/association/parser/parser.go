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
	"path"
	"path/filepath"

	"github.com/aws/amazon-ssm-agent/agent/appconfig"
	"github.com/aws/amazon-ssm-agent/agent/association/model"
	"github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/aws/amazon-ssm-agent/agent/contracts"
	"github.com/aws/amazon-ssm-agent/agent/fileutil"
	"github.com/aws/amazon-ssm-agent/agent/jsonutil"
	"github.com/aws/amazon-ssm-agent/agent/log"
	messageContracts "github.com/aws/amazon-ssm-agent/agent/message/contracts"
	"github.com/aws/amazon-ssm-agent/agent/message/parameters"
	messageParser "github.com/aws/amazon-ssm-agent/agent/message/parser"
	"github.com/aws/amazon-ssm-agent/agent/times"
)

// ParseDocumentWithParams parses an document and replaces the parameters where needed.
func ParseDocumentWithParams(log log.T,
	rawData *model.AssociationRawData) (*messageContracts.SendCommandPayload, error) {

	rawDataContent, _ := jsonutil.Marshal(rawData)
	log.Info("Processing assocation ", rawData.Association.Name)
	log.Info("Processing assocation ", jsonutil.Indent(rawDataContent))

	payload := &messageContracts.SendCommandPayload{}
	payload.Parameters = parseParameters(rawData.Parameter.Parameters)
	if err := json.Unmarshal([]byte(*rawData.Document), &payload.DocumentContent); err != nil {
		return nil, err
	}
	payload.DocumentName = *rawData.Association.Name
	payload.CommandID = rawData.ID

	validParams := parameters.ValidParameters(log, payload.Parameters)
	// add default values for missing parameters
	for k, v := range payload.DocumentContent.Parameters {
		if _, ok := validParams[k]; !ok {
			validParams[k] = v.DefaultVal
		}
	}

	payload.DocumentContent.RuntimeConfig =
		messageParser.ReplacePluginParameters(payload.DocumentContent.RuntimeConfig, validParams, log)

	return payload, nil
}

// InitializeCommandState - an interim state that is used around during an execution of a command
func InitializeCommandState(context context.T,
	payload *messageContracts.SendCommandPayload,
	rawData *model.AssociationRawData) (map[string]*contracts.Configuration, messageContracts.CommandState) {

	//initialize document information with relevant values extracted from msg
	documentInfo := newDocumentInfo(rawData, payload)

	// adapt plugin configuration format from MDS to plugin expected format
	s3KeyPrefix := path.Join(payload.OutputS3KeyPrefix, payload.CommandID, documentInfo.Destination)

	orchestrationRootDir := filepath.Join(appconfig.DefaultDataStorePath,
		documentInfo.Destination,
		appconfig.DefaultCommandRootDirName,
		context.AppConfig().Agent.OrchestrationRootDir)

	// getPluginConfigurations converts from PluginConfig (structure from the MDS message) to plugin.Configuration (structure expected by the plugin)
	pluginConfigurations := make(map[string]*contracts.Configuration)
	for pluginName, pluginConfig := range payload.DocumentContent.RuntimeConfig {
		pluginConfigurations[pluginName] = &contracts.Configuration{
			Properties:             pluginConfig.Properties,
			OutputS3BucketName:     payload.OutputS3BucketName,
			OutputS3KeyPrefix:      filepath.Join(s3KeyPrefix, fileutil.RemoveInvalidChars(pluginName)),
			OrchestrationDirectory: filepath.Join(orchestrationRootDir, fileutil.RemoveInvalidChars(pluginName)),
			MessageId:              documentInfo.MessageID,
			BookKeepingFileName:    payload.CommandID,
		}
	}

	//initialize plugin states
	pluginsInfo := make(map[string]messageContracts.PluginState)

	for key, value := range pluginConfigurations {
		var plugin messageContracts.PluginState
		plugin.Configuration = *value
		plugin.HasExecuted = false
		pluginsInfo[key] = plugin
	}

	//initialize command State
	return pluginConfigurations, messageContracts.CommandState{
		DocumentInformation: documentInfo,
		PluginsInformation:  pluginsInfo,
	}
}

// newDocumentInfo initializes new DocumentInfo object
func newDocumentInfo(rawData *model.AssociationRawData, payload *messageContracts.SendCommandPayload) messageContracts.DocumentInfo {

	documentInfo := new(messageContracts.DocumentInfo)

	documentInfo.CommandID = rawData.ID
	documentInfo.Destination = *rawData.Association.InstanceId
	documentInfo.MessageID = fmt.Sprintf("aws.ssm.%v.%v", documentInfo.CommandID, documentInfo.Destination)
	documentInfo.RunID = times.ToIsoDashUTC(times.DefaultClock.Now())
	documentInfo.CreatedDate = rawData.CreateDate
	documentInfo.DocumentName = payload.DocumentName
	documentInfo.IsCommand = false
	documentInfo.DocumentStatus = contracts.ResultStatusInProgress
	documentInfo.DocumentTraceOutput = ""

	return *documentInfo
}

func parseParameters(params map[string][]*string) map[string]interface{} {
	result := make(map[string]interface{})

	for name, param := range params {
		if len(param) > 1 {
			result[name] = param
		} else if len(param) == 1 {
			result[name] = param[0]
		}
	}
	return result
}
