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
	"github.com/aws/amazon-ssm-agent/agent/rebooter"
	"github.com/aws/amazon-ssm-agent/agent/task"
)

// SendResponse is used to send response on plugin completion.
// If pluginID is empty it will send responses of all plugins.
// If pluginID is specified, response will be sent of that particular plugin.
type SendResponse func(messageID string, pluginID string, results map[string]*contracts.PluginResult)

// SendDocumentLevelResponse is used to send status response before plugin begins
type SendDocumentLevelResponse func(messageID string, resultStatus contracts.ResultStatus, documentTraceOutput string)

// RunPlugins executes a set of plugins. The plugin configurations are given in a map with pluginId as key.
// Outputs the results of running the plugins, indexed by pluginId.
func RunPlugins(
	context context.T,
	documentID string,
	plugins map[string]*contracts.Configuration,
	pluginRegistry plugin.PluginRegistry,
	sendReply SendResponse,
	cancelFlag task.CancelFlag,
) (pluginOutputs map[string]*contracts.PluginResult) {

	requestReboot := false

	pluginOutputs = make(map[string]*contracts.PluginResult)
	for pluginID, pluginConfig := range plugins {
		// populate plugin start time and status
		pluginOutputs[pluginID] = &contracts.PluginResult{
			Status:        contracts.ResultStatusInProgress,
			StartDateTime: time.Now(),
		}
		if pluginConfig.OutputS3BucketName != "" {
			pluginOutputs[pluginID].OutputS3BucketName = pluginConfig.OutputS3BucketName
			if pluginConfig.OutputS3KeyPrefix != "" {
				pluginOutputs[pluginID].OutputS3KeyPrefix = pluginConfig.OutputS3KeyPrefix

			}
		}
		var r contracts.PluginResult
		pluginHandlerFound := false

		//check if the said plugin is a long running plugin
		handler, isLongRunningPlugin := plugin.RegisteredLongRunningPlugins(context)[pluginID]
		//check if the said plugin is a worker plugin
		p, isWorkerPlugin := pluginRegistry[pluginID]

		switch {
		case isLongRunningPlugin:
			pluginHandlerFound = true
			context.Log().Infof("%s is a long running plugin", pluginID)
			r = runPlugin(context, handler, pluginID, *pluginConfig, cancelFlag)
		case isWorkerPlugin:
			pluginHandlerFound = true
			context.Log().Infof("%s is a worker plugin", pluginID)
			r = runPlugin(context, p, pluginID, *pluginConfig, cancelFlag)
		default:
			err := fmt.Errorf("Plugin with id %s not found!", pluginID)
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
				requestReboot = true
			}
		}
		// set end time.
		pluginOutputs[pluginID].EndDateTime = time.Now()

		if sendReply != nil {
			context.Log().Infof("Sending response on plugin completion: %v", pluginID)
			sendReply(documentID, pluginID, pluginOutputs)
		}

	}

	// request reboot if any of the plugins have requested a reboot
	if requestReboot {
		context.Log().Debug("Requesting reboot...")
		go rebooter.RequestPendingReboot()
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
