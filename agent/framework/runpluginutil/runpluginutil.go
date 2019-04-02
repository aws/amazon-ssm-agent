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

// Package runpluginutil run plugin utility functions without referencing the actually plugin impl packages
package runpluginutil

import (
	"fmt"
	"strings"
	"time"

	"github.com/aws/amazon-ssm-agent/agent/appconfig"
	"github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/aws/amazon-ssm-agent/agent/contracts"
	"github.com/aws/amazon-ssm-agent/agent/fileutil"
	"github.com/aws/amazon-ssm-agent/agent/framework/processor/executer/iohandler"
	"github.com/aws/amazon-ssm-agent/agent/jsonutil"
	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/amazon-ssm-agent/agent/platform"
	"github.com/aws/amazon-ssm-agent/agent/plugins/pluginutil"
	"github.com/aws/amazon-ssm-agent/agent/task"
)

const (
	executeStep string = "execute"
	skipStep    string = "skip"
	failStep    string = "fail"
)

// TODO: rename to RCPlugin, this represents RCPlugin interface.
type T interface {
	Execute(context context.T, config contracts.Configuration, cancelFlag task.CancelFlag, output iohandler.IOHandler)
}

type PluginFactory interface {
	Create(context context.T) (T, error)
}

// PluginRegistry stores a set of plugins (both worker and long running plugins), indexed by ID.
type PluginRegistry map[string]PluginFactory

var SSMPluginRegistry PluginRegistry

// allPlugins is the list of all known plugins.
// This allows us to differentiate between the case where a document asks for a plugin that exists but isn't supported on this platform
// and the case where a plugin name isn't known at all to this version of the agent (and the user should probably upgrade their agent)
var allPlugins = map[string]struct{}{
	appconfig.PluginNameAwsAgentUpdate:         {},
	appconfig.PluginNameAwsApplications:        {},
	appconfig.PluginNameAwsConfigureDaemon:     {},
	appconfig.PluginNameAwsConfigurePackage:    {},
	appconfig.PluginNameAwsPowerShellModule:    {},
	appconfig.PluginNameAwsRunPowerShellScript: {},
	appconfig.PluginNameAwsRunShellScript:      {},
	appconfig.PluginNameAwsSoftwareInventory:   {},
	appconfig.PluginNameCloudWatch:             {},
	appconfig.PluginNameConfigureDocker:        {},
	appconfig.PluginNameDockerContainer:        {},
	appconfig.PluginNameDomainJoin:             {},
	appconfig.PluginEC2ConfigUpdate:            {},
	appconfig.PluginNameRefreshAssociation:     {},
	appconfig.PluginDownloadContent:            {},
	appconfig.PluginRunDocument:                {},
}

// allSessionPlugins is the list of all known session plugins.
var allSessionPlugins = map[string]struct{}{
	appconfig.PluginNameStandardStream:      {},
	appconfig.PluginNameInteractiveCommands: {},
	appconfig.PluginNamePort:                {},
}

// Assign method to global variables to allow unittest to override
var isSupportedPlugin = IsPluginSupportedForCurrentPlatform

// TODO remove executionID and creation date
// RunPlugins executes a set of plugins. The plugin configurations are given in a map with pluginId as key.
// Outputs the results of running the plugins, indexed by pluginId.
// Make this function private in case everybody tries to reference it everywhere, this is a private member of Executer
func RunPlugins(
	context context.T,
	plugins []contracts.PluginState,
	ioConfig contracts.IOConfiguration,
	registry PluginRegistry,
	resChan chan contracts.PluginResult,
	cancelFlag task.CancelFlag,
) (pluginOutputs map[string]*contracts.PluginResult) {

	pluginOutputs = make(map[string]*contracts.PluginResult)

	//Contains the logStreamPrefix without the pluginID
	logStreamPrefix := ioConfig.CloudWatchConfig.LogStreamPrefix

	for _, pluginState := range plugins {
		pluginID := pluginState.Id     // the identifier of the plugin
		pluginName := pluginState.Name // the name of the plugin
		pluginOutput := pluginState.Result
		pluginOutput.PluginID = pluginID
		pluginOutput.PluginName = pluginName
		pluginOutputs[pluginID] = &pluginOutput
		switch pluginOutput.Status {
		//TODO properly initialize the plugin status
		case "":
			context.Log().Debugf("plugin - %v has empty state, initialize as NotStarted",
				pluginName)
			pluginOutput.StartDateTime = time.Now()
			pluginOutput.Status = contracts.ResultStatusNotStarted

		case contracts.ResultStatusNotStarted, contracts.ResultStatusInProgress:
			context.Log().Debugf("plugin - %v status %v",
				pluginName,
				pluginOutput.Status)
			pluginOutput.StartDateTime = time.Now()

		case contracts.ResultStatusSuccessAndReboot:
			context.Log().Debugf("plugin - %v just experienced reboot, reset to InProgress...",
				pluginName)
			pluginOutput.Status = contracts.ResultStatusInProgress

		default:
			context.Log().Debugf("plugin - %v already executed, skipping...",
				pluginName)
			continue
		}

		context.Log().Debugf("Executing plugin - %v", pluginName)

		// populate plugin start time and status
		configuration := pluginState.Configuration

		if ioConfig.OutputS3BucketName != "" {
			pluginOutputs[pluginID].OutputS3BucketName = ioConfig.OutputS3BucketName
			if ioConfig.OutputS3KeyPrefix != "" {
				pluginOutputs[pluginID].OutputS3KeyPrefix = fileutil.BuildS3Path(ioConfig.OutputS3KeyPrefix, pluginName)

			}
		}
		//Append pluginID to logStreamPrefix. Replace ':' or '*' with '-' since LogStreamNames cannot have those characters
		if ioConfig.CloudWatchConfig.LogGroupName != "" {
			ioConfig.CloudWatchConfig.LogStreamPrefix = fmt.Sprintf("%s/%s", logStreamPrefix, pluginID)
			ioConfig.CloudWatchConfig.LogStreamPrefix = strings.Replace(ioConfig.CloudWatchConfig.LogStreamPrefix, ":", "-", -1)
			ioConfig.CloudWatchConfig.LogStreamPrefix = strings.Replace(ioConfig.CloudWatchConfig.LogStreamPrefix, "*", "-", -1)
		}

		var (
			r                  contracts.PluginResult
			pluginFactory      PluginFactory
			pluginHandlerFound bool
			isKnown            bool
			isSupported        bool
		)

		pluginFactory, pluginHandlerFound = registry[pluginName]
		isKnown, isSupported, _ = isSupportedPlugin(context.Log(), pluginName)
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
			context.Log().Infof("Running plugin %s", pluginName)
			r = runPlugin(context, pluginFactory, pluginName, configuration, cancelFlag, ioConfig)
			pluginOutputs[pluginID].Code = r.Code
			pluginOutputs[pluginID].Status = r.Status
			pluginOutputs[pluginID].Error = r.Error
			pluginOutputs[pluginID].Output = r.Output
			pluginOutputs[pluginID].StandardOutput = r.StandardOutput
			pluginOutputs[pluginID].StandardError = r.StandardError
			pluginOutputs[pluginID].StepName = r.StepName

		case skipStep:
			context.Log().Info(logMessage)
			pluginOutputs[pluginID].Status = contracts.ResultStatusSkipped
			pluginOutputs[pluginID].Code = 0
			pluginOutputs[pluginID].Output = logMessage
		case failStep:
			err := fmt.Errorf(logMessage)
			pluginOutputs[pluginID].Status = contracts.ResultStatusFailed
			pluginOutputs[pluginID].Error = err.Error()
			context.Log().Error(err)
		default:
			err := fmt.Errorf("Unknown error, Operation: %s, Plugin name: %s", operation, pluginName)
			pluginOutputs[pluginID].Status = contracts.ResultStatusFailed
			pluginOutputs[pluginID].Error = err.Error()
			context.Log().Error(err)
		}

		// set end time.
		pluginOutputs[pluginID].EndDateTime = time.Now()
		context.Log().Infof("Sending plugin %v completion message", pluginID)

		// truncate the result and send it back to buffer channel.
		result := *pluginOutputs[pluginID]
		pluginConfig := iohandler.DefaultOutputConfig()
		result.StandardOutput = pluginutil.StringPrefix(result.StandardOutput, pluginConfig.MaxStdoutLength, pluginConfig.OutputTruncatedSuffix)
		result.StandardError = pluginutil.StringPrefix(result.StandardError, pluginConfig.MaxStdoutLength, pluginConfig.OutputTruncatedSuffix)
		// send to buffer channel, guaranteed to not block since buffer size is plugin number
		resChan <- result

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
	factory PluginFactory,
	pluginName string,
	config contracts.Configuration,
	cancelFlag task.CancelFlag,
	ioConfig contracts.IOConfiguration) (res contracts.PluginResult) {
	// create a new context that includes plugin ID
	context = context.With("[pluginName=" + pluginName + "]")

	log := context.Log()
	var stepName string

	defer func() {
		// recover in case the plugin panics
		// this should handle some kind of seg fault errors.
		if err := recover(); err != nil {
			res.Status = contracts.ResultStatusFailed
			res.Code = 1
			res.Error = fmt.Errorf("Plugin crashed with message %v!", err).Error()
			log.Error(res.Error)
		}
	}()

	log.Debugf("Running %s", pluginName)
	var err error
	plugin, err := factory.Create(context)

	if err != nil {
		res.Status = contracts.ResultStatusFailed
		res.Code = 1
		res.Error = fmt.Errorf("failed to create plugin %v!", err).Error()
		log.Error(res.Error)
		return
	}

	res.StartDateTime = time.Now()
	defer func() { res.EndDateTime = time.Now() }()

	output := iohandler.NewDefaultIOHandler(log, ioConfig)
	//check if properties is a list. If true, then unroll
	switch config.Properties.(type) {
	case []interface{}:
		// Load each property as a list.
		var properties []interface{}
		if properties = pluginutil.LoadParametersAsList(log, config.Properties, &res); res.Code != 0 {
			return
		}
		for _, prop := range properties {
			config.Properties = prop
			propOutput := iohandler.NewDefaultIOHandler(log, ioConfig)
			stepName, err = getStepName(pluginName, config)
			if err != nil {
				errorString := fmt.Errorf("Invalid format in plugin properties %v;\nerror %v", config.Properties, err)
				output.MarkAsFailed(errorString)
			} else {
				executePlugin(context, plugin, pluginName, stepName, config, cancelFlag, propOutput)
			}

			output.Merge(log, propOutput)
		}

	default:
		stepName, err = getStepName(pluginName, config)
		if err != nil {
			errorString := fmt.Errorf("Invalid format in plugin properties %v;\nerror %v", config.Properties, err)
			output.MarkAsFailed(errorString)
		} else {
			executePlugin(context, plugin, pluginName, stepName, config, cancelFlag, output)
		}
	}

	if ioConfig.OutputS3BucketName != "" {
		if stepName != "" {
			// Colons are removed from s3 url's before uploading. Removing from here so worker can generate same url.
			stepName = strings.Replace(stepName, ":", "", -1)
		}

		res.StepName = stepName
	}
	res.Code = output.GetExitCode()
	res.Status = output.GetStatus()
	res.Output = output.GetOutput()
	res.StandardOutput = output.GetStdout()
	res.StandardError = output.GetStderr()

	return
}

// executePlugin executes the plugin that's passed in and initializes the necessary writers
func executePlugin(context context.T,
	plugin T,
	pluginName string,
	stepName string,
	config contracts.Configuration,
	cancelFlag task.CancelFlag,
	output iohandler.IOHandler) {
	log := context.Log()

	// Create the output object and execute the plugin
	defer output.Close(log)
	output.Init(log, pluginName, stepName)
	plugin.Execute(context, config, cancelFlag, output)
}

// GetPropertyName returns the ID field of property in a v1.2 SSM Document
func GetPropertyName(rawPluginInput interface{}) (propertyName string, err error) {
	pluginInput := struct{ ID string }{}
	err = jsonutil.Remarshal(rawPluginInput, &pluginInput)
	propertyName = pluginInput.ID
	return
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

// Returns the Property's ID field from v1.2 documents or the Name field of a Step in v2.x documents.
// This is required to generate the correct stdout/stderr s3 url
func getStepName(pluginName string, config contracts.Configuration) (stepName string, err error) {
	if config.PluginName == config.PluginID {
		if pluginName == appconfig.PluginNameCloudWatch {
			stepName = appconfig.PluginNameCloudWatch
		} else {
			stepName, err = GetPropertyName(config.Properties) //V10 Schema
		}
	} else {
		stepName = config.PluginID //V20 Schema
	}

	return
}
