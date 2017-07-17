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
package basicexecuter

import (
	"fmt"
	"strings"
	"time"

	"github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/aws/amazon-ssm-agent/agent/contracts"
	docModel "github.com/aws/amazon-ssm-agent/agent/docmanager/model"
	"github.com/aws/amazon-ssm-agent/agent/framework/runpluginutil"
	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/amazon-ssm-agent/agent/message/processor/executer/plugin"
	"github.com/aws/amazon-ssm-agent/agent/platform"
	"github.com/aws/amazon-ssm-agent/agent/rebooter"
	"github.com/aws/amazon-ssm-agent/agent/task"
	"github.com/aws/amazon-ssm-agent/agent/times"
)

const (
	executeStep string = "execute"
	skipStep    string = "skip"
	failStep    string = "fail"
)

// Assign method to global variables to allow unittest to override
var isSupportedPlugin = plugin.IsPluginSupportedForCurrentPlatform

// RunPlugins executes a set of plugins. The plugin configurations are given in a map with pluginId as key.
// Outputs the results of running the plugins, indexed by pluginId.
// Make this function private in case everybody tries to reference it everywhere, this is a private member of Executer
func runPlugins(
	context context.T,
	executionID string,
	documentCreatedDate string,
	plugins []docModel.PluginState,
	pluginRegistry runpluginutil.PluginRegistry,
	resChan chan contracts.PluginResult,
	cancelFlag task.CancelFlag,
) (pluginOutputs map[string]*contracts.PluginResult) {

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

		runner := runpluginutil.PluginRunner{
			RunPlugins:  RunPluginsLegacy,
			Plugins:     pluginRegistry,
			SendReply:   runpluginutil.NoReply,
			UpdateAssoc: runpluginutil.NoUpdate,
			CancelFlag:  cancelFlag,
		}

		isKnown, isSupported, _ := isSupportedPlugin(context.Log(), pluginName)
		operation, logMessage := getStepExecutionOperation(
			context.Log(),
			pluginName,
			pluginID,
			isKnown,
			isSupported,
			pluginHandlerFound,
			configuration.IsPreconditionEnabled,
			configuration.Preconditions)

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
			context.Log().Info(logMessage)
			pluginOutputs[pluginID].Status = contracts.ResultStatusSkipped
			pluginOutputs[pluginID].Code = 0
			pluginOutputs[pluginID].Output = logMessage
		case failStep:
			err := fmt.Errorf(logMessage)
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
		context.Log().Infof("Sending plugin %v completion message", pluginID)
		// send to buffer channel, guranteed not block since buffer size is plugin number
		resChan <- *pluginOutputs[pluginID]

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
	pluginName string,
	pluginId string,
	isKnown bool,
	isSupported bool,
	isPluginHandlerFound bool,
	isPreconditionEnabled bool,
	preconditions map[string][]string,
) (string, string) {
	log.Debugf("isSupported flag = %t", isSupported)
	log.Debugf("isPluginHandlerFound flag = %t", isPluginHandlerFound)
	log.Debugf("isPreconditionEnabled flag = %t", isPreconditionEnabled)

	if !isPreconditionEnabled {
		// 1.x or 2.0 document
		if !isKnown {
			return failStep, fmt.Sprintf(
				"Plugin with name %s is not supported by this version of ssm agent, please update to latest version. Step name: %s",
				pluginName,
				pluginId)
		} else if !isSupported {
			return failStep, fmt.Sprintf(
				"Plugin with name %s is not supported in current platform. Step name: %s",
				pluginName,
				pluginId)
		} else if len(preconditions) > 0 {
			// if 1.x or 2.0 document contains precondition or plugin not found, failStep
			return failStep, fmt.Sprintf(
				"Precondition is not supported for document schema version prior to 2.2. Step name: %s",
				pluginId)
		} else if !isPluginHandlerFound {
			return failStep, fmt.Sprintf(
				"Plugin with name %s not found. Step name: %s",
				pluginName,
				pluginId)
		} else {
			return executeStep, ""
		}
	} else {
		// 2.2 or higher (cross-platform) document
		if len(preconditions) == 0 {
			log.Debug("Cross-platform Precondition is not present")

			// precondition is not present - if pluginFound executeStep, else skipStep
			if !isKnown {
				return failStep, fmt.Sprintf(
					"Plugin with name %s is not supported by this version of ssm agent, please update to latest version. Step name: %s",
					pluginName,
					pluginId)
			} else if isSupported && isPluginHandlerFound {
				return executeStep, ""
			} else {
				return skipStep, fmt.Sprintf(
					"Step execution skipped due to incompatible platform. Step name: %s",
					pluginId)
			}
		} else {
			log.Debugf("Cross-platform Precondition is present, precondition = %v", preconditions)

			isAllowed, unrecognizedPreconditionList := evaluatePreconditions(log, preconditions)

			if isAllowed && !isKnown {
				return failStep, fmt.Sprintf(
					"Plugin with name %s is not supported by this version of ssm agent, please update to latest version. Step name: %s",
					pluginName,
					pluginId)
			} else if !isAllowed || !isSupported || !isPluginHandlerFound {
				return skipStep, fmt.Sprintf(
					"Step execution skipped due to incompatible platform. Step name: %s",
					pluginId)
			} else if len(unrecognizedPreconditionList) > 0 {
				return failStep, fmt.Sprintf(
					"Unrecognized precondition(s): '%s', please update agent to latest version. Step name: %s",
					strings.Join(unrecognizedPreconditionList, ", "),
					pluginId)
			} else {
				return executeStep, ""
			}
		}
	}
}

// Evaluate precondition and return precondition result and unrecognized preconditions (if any)
func evaluatePreconditions(
	log log.T,
	preconditions map[string][]string,
) (bool, []string) {

	var isAllowed = true
	var unrecognizedPreconditionList []string

	// For current release, we only support "StringEquals" operator and "platformType"
	// operand, so explicitly checking for those and number of operands must be 2
	for key, value := range preconditions {
		switch key {
		case "StringEquals":
			// Platform type of OS on the instance
			instancePlatformType, _ := platform.PlatformType(log)
			log.Debugf("OS platform type of this instance = %s", instancePlatformType)

			if len(value) != 2 ||
				(strings.Compare(value[0], "platformType") == 0 && strings.Compare(value[1], "platformType") == 0) ||
				(strings.Compare(value[0], "platformType") != 0 && strings.Compare(value[1], "platformType") != 0) {

				unrecognizedPreconditionList = append(unrecognizedPreconditionList, fmt.Sprintf("\"%s\": %v", key, value))
			} else {
				// Variable and value can be in any order, i.e. both "StringEquals": ["platformType", "Windows"]
				// and "StringEquals": ["Windows", "platformType"] are valid
				var platformTypeValue string
				if strings.Compare(value[0], "platformType") == 0 {
					platformTypeValue = value[1]
				} else {
					platformTypeValue = value[0]
				}

				if strings.Compare(instancePlatformType, strings.ToLower(platformTypeValue)) != 0 {
					// if precondition doesn't match for platformType, mark step for skip
					isAllowed = false
				}
			}
		default:
			// mark for unrecognizedPrecondition (which is a form of failure)
			unrecognizedPreconditionList = append(unrecognizedPreconditionList, fmt.Sprintf("\"%s\": %v", key, value))
		}
	}

	return isAllowed, unrecognizedPreconditionList
}

//TODO this is kept for plugin to use internally, will be removed in future
func RunPluginsLegacy(
	context context.T,
	executionID string,
	documentCreatedDate string,
	plugins []docModel.PluginState,
	pluginRegistry runpluginutil.PluginRegistry,
	sendReply runpluginutil.SendResponseLegacy,
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

		runner := runpluginutil.PluginRunner{
			RunPlugins:  RunPluginsLegacy,
			Plugins:     pluginRegistry,
			SendReply:   runpluginutil.NoReply,
			UpdateAssoc: runpluginutil.NoUpdate,
			CancelFlag:  cancelFlag,
		}

		isKnown, isSupported, _ := isSupportedPlugin(context.Log(), pluginName)
		operation, logMessage := getStepExecutionOperation(
			context.Log(),
			pluginName,
			pluginID,
			isKnown,
			isSupported,
			pluginHandlerFound,
			configuration.IsPreconditionEnabled,
			configuration.Preconditions)

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
			context.Log().Info(logMessage)
			pluginOutputs[pluginID].Status = contracts.ResultStatusSkipped
			pluginOutputs[pluginID].Code = 0
			pluginOutputs[pluginID].Output = logMessage
		case failStep:
			err := fmt.Errorf(logMessage)
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
