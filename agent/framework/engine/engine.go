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
	"github.com/aws/amazon-ssm-agent/agent/framework/runpluginutil"
	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/amazon-ssm-agent/agent/rebooter"
	stateModel "github.com/aws/amazon-ssm-agent/agent/statemanager/model"
	"github.com/aws/amazon-ssm-agent/agent/task"
	"github.com/aws/amazon-ssm-agent/agent/times"
)

// SendDocumentLevelResponse is used to send status response before plugin begins
type SendDocumentLevelResponse func(messageID string, resultStatus contracts.ResultStatus, documentTraceOutput string)

// UpdateAssociation updates association status
type UpdateAssociation func(log log.T, executionID string, documentCreatedDate string, pluginOutputs map[string]*contracts.PluginResult, totalNumberOfPlugins int)

// RunPlugins executes a set of plugins. The plugin configurations are given in a map with pluginId as key.
// Outputs the results of running the plugins, indexed by pluginId.
func RunPlugins(
	context context.T,
	executionID string,
	documentCreatedDate string,
	plugins []stateModel.PluginState,
	pluginRegistry runpluginutil.PluginRegistry,
	sendReply runpluginutil.SendResponse,
	updateAssoc runpluginutil.UpdateAssociation,
	cancelFlag task.CancelFlag,
) (pluginOutputs map[string]*contracts.PluginResult) {
	totalNumberOfActions := len(plugins)

	pluginOutputs = make(map[string]*contracts.PluginResult)

	for _, pluginState := range plugins {
		pluginID := pluginState.Id     // the identifier of the plugin
		pluginName := pluginState.Name // the name of the plugin
		pluginOutput := pluginState.Result
		pluginOutput.PluginName = pluginName
		pluginOutputs[pluginID] = &pluginOutput
		switch pluginOutput.Status {
		//TODO properly initialize the plugin status
		case "":
			context.Log().Debugf("plugin - %v of document - %v has empty state, initialize as NotStarted",
				pluginName,
				executionID)
			pluginOutput.StartDateTime = time.Now()
			pluginOutput.Status = contracts.ResultStatusNotStarted

		case contracts.ResultStatusNotStarted, contracts.ResultStatusInProgress:
			context.Log().Debugf("plugin - %v of document - %v status %v",
				pluginName,
				executionID,
				pluginOutput.Status)
			pluginOutput.StartDateTime = time.Now()

		case contracts.ResultStatusSuccessAndReboot:
			context.Log().Debugf("plugin - %v of document - %v just experienced reboot, reset to InProgress...",
				pluginName,
				executionID)
			pluginOutput.Status = contracts.ResultStatusInProgress

		default:
			context.Log().Debugf("plugin - %v of document - %v already executed, skipping...",
				pluginName,
				executionID)
			continue
		}

		context.Log().Debugf("Executing plugin - %v of document - %v", pluginName, executionID)

		// populate plugin start time and status
		configuration := pluginState.Configuration

		if configuration.OutputS3BucketName != "" {
			pluginOutputs[pluginID].OutputS3BucketName = configuration.OutputS3BucketName
			if configuration.OutputS3KeyPrefix != "" {
				pluginOutputs[pluginID].OutputS3KeyPrefix = configuration.OutputS3KeyPrefix

			}
		}
		var r contracts.PluginResult
		pluginHandlerFound := false

		//check if the said plugin is a worker plugin
		p, pluginHandlerFound := pluginRegistry[pluginName]
		if !pluginHandlerFound {
			//check if the said plugin is a long running plugin
			p, pluginHandlerFound = plugin.RegisteredLongRunningPlugins(context)[pluginName]
		}

		runner := runpluginutil.PluginRunner{
			RunPlugins:  RunPlugins,
			Plugins:     pluginRegistry,
			SendReply:   runpluginutil.NoReply,
			UpdateAssoc: runpluginutil.NoUpdate,
			CancelFlag:  cancelFlag,
		}

		isSupported, platformDetail := plugin.IsPluginSupportedForCurrentPlatform(context.Log(), pluginName)
		if isSupported {
			if pluginHandlerFound {
				context.Log().Infof("%s is a supported plugin", pluginName)
				r = runPlugin(context, p, pluginName, configuration, cancelFlag, runner)
			} else {
				err := fmt.Errorf("Plugin with name %s not found!", pluginName)
				pluginOutputs[pluginID].Status = contracts.ResultStatusFailed
				pluginOutputs[pluginID].Error = err
				context.Log().Error(err)
			}
		} else {
			err := fmt.Errorf("Plugin with name %s is not supported in current platform!\n%s", pluginName, platformDetail)
			pluginOutputs[pluginID].Status = contracts.ResultStatusFailed
			pluginOutputs[pluginID].Error = err
			context.Log().Error(err)
		}

		if pluginHandlerFound {
			pluginOutputs[pluginID].Code = r.Code
			pluginOutputs[pluginID].Status = r.Status
			pluginOutputs[pluginID].Error = r.Error
			pluginOutputs[pluginID].Output = r.Output
			pluginOutputs[pluginID].StandardOutput = r.StandardOutput
			pluginOutputs[pluginID].StandardError = r.StandardError

			if r.Status == contracts.ResultStatusSuccessAndReboot {
				context.Log().Debug("Requesting reboot...")
				//TODO move this into plugin.Execute()?
				rebooter.RequestPendingReboot(context.Log())
			}
		}
		// set end time.
		pluginOutputs[pluginID].EndDateTime = time.Now()
		if sendReply != nil {
			context.Log().Infof("Sending response on plugin completion: %v", pluginName)
			sendReply(executionID, pluginName, pluginOutputs)
		}
		if updateAssoc != nil {
			context.Log().Infof("Update association on plugin completion: %v", pluginID)
			updateAssoc(context.Log(), executionID, times.ToIso8601UTC(time.Now()), pluginOutputs, totalNumberOfActions)
		}
		//TODO handle cancelFlag here
		if pluginHandlerFound && r.Status == contracts.ResultStatusSuccessAndReboot {
			// do not execute the the next plugin
			break
		}

	}

	return
}

func runPlugin(
	context context.T,
	p runpluginutil.T,
	pluginID string,
	config contracts.Configuration,
	cancelFlag task.CancelFlag,
	runner runpluginutil.PluginRunner,
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
	return p.Execute(context, config, cancelFlag, runner)
}
