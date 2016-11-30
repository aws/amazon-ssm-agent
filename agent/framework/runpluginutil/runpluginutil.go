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

// Package runpluginutil provides interfaces for running plugins that can be referenced by other plugins and a utility method for parsing documents
package runpluginutil

import (
	"encoding/json"
	"fmt"

	"github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/aws/amazon-ssm-agent/agent/contracts"
	"github.com/aws/amazon-ssm-agent/agent/fileutil"
	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/amazon-ssm-agent/agent/statemanager/model"
	"github.com/aws/amazon-ssm-agent/agent/task"
)

// SendResponse is used to send response on plugin completion.
// If pluginID is empty it will send responses of all plugins.
// If pluginID is specified, response will be sent of that particular plugin.
type SendResponse func(messageID string, pluginID string, results map[string]*contracts.PluginResult)

func NoReply(messageID string, pluginID string, results map[string]*contracts.PluginResult) {}

// SendDocumentLevelResponse is used to send status response before plugin begins
type SendDocumentLevelResponse func(messageID string, resultStatus contracts.ResultStatus, documentTraceOutput string)

// UpdateAssociation updates association status
type UpdateAssociation func(log log.T, documentID string, documentCreatedDate string, pluginOutputs map[string]*contracts.PluginResult, totalNumberOfPlugins int)

func NoUpdate(log log.T, documentID string, documentCreatedDate string, pluginOutputs map[string]*contracts.PluginResult, totalNumberOfPlugins int) {
}

// T is the interface type for plugins.
type T interface {
	Execute(context context.T, config contracts.Configuration, cancelFlag task.CancelFlag, subDocumentRunner PluginRunner) contracts.PluginResult
}

// PluginRegistry stores a set of plugins (both worker and long running plugins), indexed by ID.
type PluginRegistry map[string]T

type PluginRunner struct {
	RunPlugins func(
		context context.T,
		documentID string,
		documentCreatedDate string,
		plugins []model.PluginState,
		pluginRegistry PluginRegistry,
		sendReply SendResponse,
		updateAssoc UpdateAssociation,
		cancelFlag task.CancelFlag,
	) (pluginOutputs map[string]*contracts.PluginResult)
	Plugins     PluginRegistry
	SendReply   SendResponse
	UpdateAssoc UpdateAssociation
	CancelFlag  task.CancelFlag
}

// TODO:MF: Factor out the Configuration processing in processor_state and re-use here
func ParseDocument(context context.T, documentRaw []byte, orchestrationDir string, s3Bucket string, s3KeyPrefix string, messageID string, documentID string, defaultWorkingDirectory string) (pluginsInfo []model.PluginState, err error) {
	var docContent contracts.DocumentContent
	err = json.Unmarshal(documentRaw, &docContent)
	//TODO:MF: return this error
	pluginConfigurations := make([]*contracts.Configuration, 0)

	if docContent.SchemaVersion == "2.0" {
		for _, pluginConfig := range docContent.MainSteps {
			pluginName := pluginConfig.Action
			config := contracts.Configuration{
				Settings:                pluginConfig.Settings,
				Properties:              pluginConfig.Inputs,
				OutputS3BucketName:      s3Bucket,
				OutputS3KeyPrefix:       fileutil.BuildS3Path(s3KeyPrefix, pluginConfig.Name),
				OrchestrationDirectory:  fileutil.BuildPath(orchestrationDir, pluginConfig.Name),
				MessageId:               messageID,
				BookKeepingFileName:     documentID,
				PluginName:              pluginName,
				PluginID:                pluginConfig.Name,
				DefaultWorkingDirectory: defaultWorkingDirectory,
			}
			pluginConfigurations = append(pluginConfigurations, &config)
		}
	} else if docContent.SchemaVersion == "1.2" {
		for pluginName, pluginConfig := range docContent.RuntimeConfig {
			config := contracts.Configuration{
				Settings:                pluginConfig.Settings,
				Properties:              pluginConfig.Properties,
				OutputS3BucketName:      s3Bucket,
				OutputS3KeyPrefix:       fileutil.BuildS3Path(s3KeyPrefix, pluginName),
				OrchestrationDirectory:  fileutil.BuildPath(orchestrationDir, pluginName),
				MessageId:               messageID,
				BookKeepingFileName:     documentID,
				PluginName:              pluginName,
				PluginID:                pluginName,
				DefaultWorkingDirectory: defaultWorkingDirectory,
			}
			pluginConfigurations = append(pluginConfigurations, &config)
		}
	} else {
		err = fmt.Errorf("unsupported schema version %v", docContent.SchemaVersion)
	}
	//initialize plugin states
	pluginsInfo = make([]model.PluginState, 0, len(pluginConfigurations))

	// TODO:MF: Use converter here if this was the 1.2 format (had runtimeconfig instead of mainsteps)
	for _, value := range pluginConfigurations {
		var plugin model.PluginState
		plugin.Id = value.PluginID
		plugin.Name = value.PluginName
		plugin.Configuration = *value
		plugin.HasExecuted = false
		pluginsInfo = append(pluginsInfo, plugin)
	}

	return
}

func (r *PluginRunner) ExecuteDocument(context context.T, pluginInput []model.PluginState, documentID string, documentCreatedDate string) (pluginOutputs map[string]*contracts.PluginResult) {
	log := context.Log()
	for _, state := range pluginInput {
		log.Debugf("Executing document contains input for plugin %v", state.Name)
	}

	return r.RunPlugins(context, documentID, documentCreatedDate, pluginInput, r.Plugins, r.SendReply, r.UpdateAssoc, r.CancelFlag)
}
