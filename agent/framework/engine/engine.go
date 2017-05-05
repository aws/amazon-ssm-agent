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
	"strings"
	"time"

	"github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/aws/amazon-ssm-agent/agent/contracts"
	docModel "github.com/aws/amazon-ssm-agent/agent/docmanager/model"
	"github.com/aws/amazon-ssm-agent/agent/framework/plugin"
	"github.com/aws/amazon-ssm-agent/agent/framework/runpluginutil"
	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/amazon-ssm-agent/agent/platform"
	"github.com/aws/amazon-ssm-agent/agent/rebooter"
	"github.com/aws/amazon-ssm-agent/agent/task"
	"github.com/aws/amazon-ssm-agent/agent/times"
)

// SendDocumentLevelResponse is used to send status response before plugin begins
type SendDocumentLevelResponse func(messageID string, resultStatus contracts.ResultStatus, documentTraceOutput string)

// UpdateAssociation updates association status
type UpdateAssociation func(log log.T, executionID string, documentCreatedDate string, pluginOutputs map[string]*contracts.PluginResult, totalNumberOfPlugins int)

const (
	executeStep              string = "execute"
	skipStep                 string = "skip"
	failStep                 string = "fail"
	unsupportedStep          string = "unsupported"
	unrecognizedPrecondition string = "unrecognizedPrecondition"
)

// RunPlugins executes a set of plugins. The plugin configurations are given in a map with pluginId as key.
// Outputs the results of running the plugins, indexed by pluginId.
func RunPlugins(
	context context.T,
	executionID string,
	documentCreatedDate string,
	plugins []docModel.PluginState,
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
		operation, moreDetails := getStepExecutionOperation(context.Log(), isSupported, pluginHandlerFound, configuration.IsPreconditionEnabled, configuration.Preconditions)

		switch operation {
		case executeStep:
			context.Log().Infof("%s is a supported plugin", pluginName)
			r = runPlugin(context, p, pluginName, configuration, cancelFlag, runner)
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
		case skipStep:
			skipMessage := fmt.Sprintf("Step execution skipped due to incompatible platform. Plugin: %s", pluginName)
			context.Log().Info(skipMessage)
			pluginOutputs[pluginID].Status = contracts.ResultStatusSkipped
			pluginOutputs[pluginID].Code = 0
			pluginOutputs[pluginID].Output = skipMessage
		case failStep:
			err := fmt.Errorf("Plugin with name %s not found", pluginName)
			pluginOutputs[pluginID].Status = contracts.ResultStatusFailed
			pluginOutputs[pluginID].Error = err
			context.Log().Error(err)
		case unrecognizedPrecondition:
			err := fmt.Errorf("Unrecognized precondition(s): '%s' in plugin: %s, please update agent to latest version", moreDetails, pluginName)
			pluginOutputs[pluginID].Status = contracts.ResultStatusFailed
			pluginOutputs[pluginID].Error = err
			context.Log().Error(err)
		case unsupportedStep:
			err := fmt.Errorf("Plugin with name %s is not supported in current platform!\n%s", pluginName, platformDetail)
			pluginOutputs[pluginID].Status = contracts.ResultStatusFailed
			pluginOutputs[pluginID].Error = err
			context.Log().Error(err)
		default:
			err := fmt.Errorf("Unknown error, Operation: %s, Plugin name: %s", operation, pluginName)
			pluginOutputs[pluginID].Status = contracts.ResultStatusFailed
			pluginOutputs[pluginID].Error = err
			context.Log().Error(err)
		}

		// set end time.
		pluginOutputs[pluginID].EndDateTime = time.Now()
		if sendReply != nil {
			context.Log().Infof("Sending response on plugin completion: %v", pluginID)
			sendReply(executionID, pluginID, pluginOutputs)
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

// Checks plugin compatibility and step precondition and returns if it should be executed, skipped or failed
func getStepExecutionOperation(
	log log.T,
	isSupported bool,
	isPluginHandlerFound bool,
	isPreconditionEnabled bool,
	preconditions map[string]string,
) (string, string) {
	log.Debugf("isSupported flag = %t", isSupported)
	log.Debugf("isPluginHandlerFound flag = %t", isPluginHandlerFound)
	log.Debugf("isPreconditionEnabled flag = %t", isPreconditionEnabled)

	if !isPreconditionEnabled {
		// 1.x or 2.0 document
		if !isSupported {
			return unsupportedStep, ""
		} else if len(preconditions) > 0 || !isPluginHandlerFound {
			// if 1.x or 2.0 document contains precondition or plugin not found, failStep
			return failStep, ""
		} else {
			return executeStep, ""
		}
	} else {
		// 2.1 or higher (cross-platform) document
		if len(preconditions) == 0 {
			log.Debug("Cross-platform Precondition is not present")

			// precondition is not present - if pluginFound executeStep, else skipStep
			if isSupported && isPluginHandlerFound {
				return executeStep, ""
			} else {
				return skipStep, ""
			}
		} else {
			log.Debugf("Cross-platform Precondition is present, precondition = %v", preconditions)

			// Platform type of OS on the instance
			instancePlatformType, _ := platform.PlatformType(log)
			log.Debugf("OS platform type of this instance = %s", instancePlatformType)

			var isAllowed = true
			var unrecognizedPreconditionList []string

			for k, v := range preconditions {
				if strings.Compare(k, "platformType") != 0 {
					// if there's unrecognized precondition, mark it for unrecognizedPrecondition (which is a form of failure)
					unrecognizedPreconditionList = append(unrecognizedPreconditionList, k)
				} else if strings.Compare(instancePlatformType, strings.ToLower(v)) != 0 {
					// if precondition doesn't match for platformType, mark it for skip
					isAllowed = false
				}
			}

			if !isAllowed ||
				!isSupported ||
				!isPluginHandlerFound {
				return skipStep, ""
			} else if len(unrecognizedPreconditionList) > 0 {
				return unrecognizedPrecondition, strings.Join(unrecognizedPreconditionList, ", ")
			} else {
				return executeStep, ""
			}
		}
	}
}
