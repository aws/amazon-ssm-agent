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

// Package engine contains the general purpose plugin runner of the plugin framework.
package engine

import (
	"fmt"
	"time"

	"github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/aws/amazon-ssm-agent/agent/contracts"
	"github.com/aws/amazon-ssm-agent/agent/framework/plugin"
	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/amazon-ssm-agent/agent/rebooter"
	stateModel "github.com/aws/amazon-ssm-agent/agent/statemanager/model"
	"github.com/aws/amazon-ssm-agent/agent/task"
)

// SendResponse is used to send response on plugin completion.
// If pluginID is empty it will send responses of all plugins.
// If pluginID is specified, response will be sent of that particular plugin.
type SendResponse func(messageID string, pluginID string, results map[string]*contracts.PluginResult)

// SendDocumentLevelResponse is used to send status response before plugin begins
type SendDocumentLevelResponse func(messageID string, resultStatus contracts.ResultStatus, documentTraceOutput string)

// UpdateAssociation updates association status
type UpdateAssociation func(log log.T, documentID string, pluginOutputs map[string]*contracts.PluginResult, totalNumberOfPlugins int)

// RunPlugins executes a set of plugins. The plugin configurations are given in a map with pluginId as key.
// Outputs the results of running the plugins, indexed by pluginId.
func RunPlugins(
	context context.T,
	documentID string,
	plugins map[string]stateModel.PluginState,
	pluginRegistry plugin.PluginRegistry,
	sendReply SendResponse,
	updateAssoc UpdateAssociation,
	cancelFlag task.CancelFlag,
) (pluginOutputs map[string]*contracts.PluginResult) {

	totalNumberOfPlugins := len(plugins)

	pluginOutputs = make(map[string]*contracts.PluginResult)
	for pluginID, pluginState := range plugins {
		if pluginState.HasExecuted {
			context.Log().Debugf(
				"Skipping execution of Plugin - %v of command - %v since it has already executed.",
				pluginID,
				documentID)
			pluginOutput := pluginState.Result
			pluginOutputs[pluginID] = &pluginOutput
			continue
		}
		context.Log().Debugf("Executing plugin - %v of command - %v", pluginID, documentID)

		// populate plugin start time and status
		configuration := pluginState.Configuration

		pluginOutputs[pluginID] = &contracts.PluginResult{
			Status:        contracts.ResultStatusInProgress,
			StartDateTime: time.Now(),
		}
		if configuration.OutputS3BucketName != "" {
			pluginOutputs[pluginID].OutputS3BucketName = configuration.OutputS3BucketName
			if configuration.OutputS3KeyPrefix != "" {
				pluginOutputs[pluginID].OutputS3KeyPrefix = configuration.OutputS3KeyPrefix

			}
		}
		var r contracts.PluginResult
		pluginHandlerFound := false

		//check if the said plugin is a long running plugin
		handler, isLongRunningPlugin := plugin.RegisteredLongRunningPlugins(context)[pluginID]
		//check if the said plugin is a worker plugin
		p, isWorkerPlugin := pluginRegistry[pluginID]

		isSupported, platformDetail := plugin.IsPluginSupportedForCurrentPlatform(context.Log(), pluginID)
		if isSupported {
			switch {
			case isLongRunningPlugin:
				pluginHandlerFound = true
				context.Log().Infof("%s is a long running plugin", pluginID)
				r = runPlugin(context, handler, pluginID, configuration, cancelFlag)
			case isWorkerPlugin:
				pluginHandlerFound = true
				context.Log().Infof("%s is a worker plugin", pluginID)
				r = runPlugin(context, p, pluginID, configuration, cancelFlag)
			default:
				err := fmt.Errorf("Plugin with id %s not found!", pluginID)
				pluginOutputs[pluginID].Status = contracts.ResultStatusFailed
				pluginOutputs[pluginID].Error = err
				context.Log().Error(err)
			}
		} else {
			err := fmt.Errorf("Plugin with id %s is not supported in current platform!\n%s", pluginID, platformDetail)
			pluginOutputs[pluginID].Status = contracts.ResultStatusFailed
			pluginOutputs[pluginID].Error = err
			context.Log().Error(err)
		}

		if pluginHandlerFound {
			pluginOutputs[pluginID].Code = r.Code
			pluginOutputs[pluginID].Status = r.Status
			pluginOutputs[pluginID].Error = r.Error
			pluginOutputs[pluginID].Output = r.Output

			if r.Status == contracts.ResultStatusSuccessAndReboot {
				context.Log().Debug("Requesting reboot...")
				rebooter.RequestPendingReboot()
			}
		}
		// set end time.
		pluginOutputs[pluginID].EndDateTime = time.Now()
		log := context.Log()
		if sendReply != nil {
			log.Infof("Sending response on plugin completion: %v", pluginID)
			sendReply(documentID, pluginID, pluginOutputs)
		}
		if updateAssoc != nil {
			log.Infof("Update assocition on plugin completion: %v", pluginID)
			updateAssoc(log, documentID, pluginOutputs, totalNumberOfPlugins)
		}

	}

	return
}

func runPlugin(
	context context.T,
	p plugin.T,
	pluginID string,
	config contracts.Configuration,
	cancelFlag task.CancelFlag,
) (res contracts.PluginResult) {
	// create a new context that includes plugin ID
	context = context.With("[pluginID=" + pluginID + "]")

	log := context.Log()
	defer func() {
		// recover in case the plugin panics
		// this should handle some kind of seg fault errors.
		if err := recover(); err != nil {
			res.Status = contracts.ResultStatusFailed
			res.Code = 1
			res.Error = fmt.Errorf("Plugin crashed with message %v!", err)
			log.Error(res.Error)
		}
	}()
	log.Debugf("Running %s", pluginID)
	return p.Execute(context, config, cancelFlag)
}
