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
	"runtime/debug"
	"strconv"
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
	"github.com/aws/amazon-ssm-agent/agent/ssm/ssmparameterresolver"
	"github.com/aws/amazon-ssm-agent/agent/task"
)

const (
	executeStep string = "execute"
	skipStep    string = "skip"
	failStep    string = "fail"
)

// TODO: rename to RCPlugin, this represents RCPlugin interface.
type T interface {
	Execute(config contracts.Configuration, cancelFlag task.CancelFlag, output iohandler.IOHandler)
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
	appconfig.PluginNameStandardStream:         {},
	appconfig.PluginNameInteractiveCommands:    {},
	appconfig.PluginNamePort:                   {},
	appconfig.PluginNameNonInteractiveCommands: {},
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
	log := context.Log()

	defer func() {
		if r := recover(); r != nil {
			log.Errorf("Run plugins panic: \n%v", r)
			log.Errorf("Stacktrace:\n%s", debug.Stack())
		}
	}()

	for pluginIndex, pluginState := range plugins {
		pluginID := pluginState.Id     // the identifier of the plugin
		pluginName := pluginState.Name // the name of the plugin
		pluginOutput := pluginState.Result
		pluginOutput.PluginID = pluginID
		pluginOutput.PluginName = pluginName
		pluginOutputs[pluginID] = &pluginOutput
		log.Debugf("Checking Status for plugin %s - %s", pluginName, pluginOutput.Status)
		switch pluginOutput.Status {
		//TODO properly initialize the plugin status
		case "":
			log.Debugf("plugin - %v has empty state, initialize as NotStarted",
				pluginName)
			pluginOutput.StartDateTime = time.Now()
			pluginOutput.Status = contracts.ResultStatusNotStarted

		case contracts.ResultStatusNotStarted, contracts.ResultStatusInProgress:
			log.Debugf("plugin - %v status %v",
				pluginName,
				pluginOutput.Status)
			pluginOutput.StartDateTime = time.Now()

		case contracts.ResultStatusSuccessAndReboot:
			log.Debugf("plugin - %v just experienced reboot, reset to InProgress...",
				pluginName)
			pluginOutput.Status = contracts.ResultStatusInProgress
		case contracts.ResultStatusFailed:
			log.Debugf("plugin - %v already executed with failed status, skipping...",
				pluginName)
			resChan <- *pluginOutputs[pluginID]
			continue
		default:
			log.Debugf("plugin - %v already executed, skipping...",
				pluginName)
			continue
		}

		log.Debugf("Executing plugin - %v", pluginName)

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
		isKnown, isSupported, _ = isSupportedPlugin(log, pluginName)
		// checking if a prior step returned exit codes 168 or 169 to exit document.
		// If so we need to skip every other step
		shouldSkipStepDueToPriorFailedStep := getShouldPluginSkipBasedOnControlFlow(
			context,
			plugins,
			pluginIndex,
			pluginOutputs,
		)

		operation, logMessage := getStepExecutionOperation(
			log,
			pluginName,
			pluginID,
			isKnown,
			isSupported,
			pluginHandlerFound,
			configuration.IsPreconditionEnabled,
			configuration.Preconditions,
			shouldSkipStepDueToPriorFailedStep)

		switch operation {
		case executeStep:
			log.Infof("Running plugin %s %s", pluginName, pluginID)
			r = runPlugin(context, pluginFactory, pluginName, configuration, cancelFlag, ioConfig)
			pluginOutputs[pluginID].Code = r.Code
			pluginOutputs[pluginID].Status = r.Status
			pluginOutputs[pluginID].Error = r.Error
			pluginOutputs[pluginID].StandardError = r.StandardError
			pluginOutputs[pluginID].StandardOutput = r.StandardOutput
			pluginOutputs[pluginID].Output = r.Output
			pluginOutputs[pluginID].StepName = r.StepName

			onFailureProp := getStringPropByName(pluginState.Configuration.Properties, contracts.OnFailureModifier)
			hasOnFailureProp := onFailureProp == contracts.ModifierValueExit || onFailureProp == contracts.ModifierValueSuccessAndExit
			outputAddition := ""
			if pluginOutputs[pluginID].Code == contracts.ExitWithSuccess {
				outputAddition = "\nStep exited with code 168. Therefore, marking step as succeeded. Further document steps will be skipped."
				pluginOutputs[pluginID].Status = contracts.ResultStatusSuccess
				pluginOutputs[pluginID].Error = ""
				pluginOutputs[pluginID].StandardError = ""
				pluginOutputs[pluginID].StandardOutput = r.StandardOutput + outputAddition
			} else if pluginOutputs[pluginID].Code == contracts.ExitWithFailure {
				outputAddition = "\nStep exited with code 169. Therefore, marking step as Failed. Further document steps will be skipped."
				pluginOutputs[pluginID].StandardError = r.StandardError + outputAddition
				pluginOutputs[pluginID].StandardOutput = r.StandardOutput + outputAddition
			} else if pluginOutputs[pluginID].Status == contracts.ResultStatusFailed && hasOnFailureProp {
				outputAddition = "\nStep was found to have onFailure property. Further document steps will be skipped."
				pluginOutputs[pluginID].StandardError = r.StandardError + outputAddition
				pluginOutputs[pluginID].StandardOutput = r.StandardOutput + outputAddition
				if onFailureProp == contracts.ModifierValueSuccessAndExit {
					pluginOutputs[pluginID].Status = contracts.ResultStatusSuccess
					pluginOutputs[pluginID].Code = contracts.ExitWithSuccess
				}
			}

		case skipStep:
			log.Info(logMessage)
			pluginOutputs[pluginID].Status = contracts.ResultStatusSkipped
			pluginOutputs[pluginID].Code = 0
			pluginOutputs[pluginID].Output = logMessage
		case failStep:
			err := fmt.Errorf(logMessage)
			pluginOutputs[pluginID].Status = contracts.ResultStatusFailed
			pluginOutputs[pluginID].Error = err.Error()
			log.Error(err)
		default:
			err := fmt.Errorf("Unknown error, Operation: %s, Plugin name: %s", operation, pluginName)
			pluginOutputs[pluginID].Status = contracts.ResultStatusFailed
			pluginOutputs[pluginID].Error = err.Error()
			log.Error(err)
		}

		// set end time.
		pluginOutputs[pluginID].EndDateTime = time.Now()
		log.Infof("Sending plugin %v completion message", pluginID)

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

var runPlugin = func(
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
			log.Errorf("Stacktrace:\n%s", debug.Stack())
		}
	}()

	var err error
	plugin, err := factory.Create(context)

	if err != nil {
		res.Status = contracts.ResultStatusFailed
		res.Code = 1
		res.Error = fmt.Errorf("failed to create plugin %v", err).Error()
		log.Error(res.Error)
		return
	}

	res.StartDateTime = time.Now()
	defer func() { res.EndDateTime = time.Now() }()

	output := iohandler.NewDefaultIOHandler(context, ioConfig)
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
			propOutput := iohandler.NewDefaultIOHandler(context, ioConfig)
			stepName, err = getStepName(pluginName, config)
			if err != nil {
				errorString := fmt.Errorf("Invalid format in plugin properties %v;\nerror %v", config.Properties, err)
				output.MarkAsFailed(errorString)
			} else {
				executePlugin(plugin, pluginName, stepName, config, cancelFlag, propOutput)
			}

			output.Merge(propOutput)
		}

	default:
		stepName, err = getStepName(pluginName, config)
		if err != nil {
			errorString := fmt.Errorf("Invalid format in plugin properties %v;\nerror %v", config.Properties, err)
			output.MarkAsFailed(errorString)
		} else {
			executePlugin(plugin, pluginName, stepName, config, cancelFlag, output)
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
func executePlugin(
	plugin T,
	pluginName string,
	stepName string,
	config contracts.Configuration,
	cancelFlag task.CancelFlag,
	output iohandler.IOHandler) {

	// Create the output object and execute the plugin
	defer output.Close()
	output.Init(pluginName, stepName)
	plugin.Execute(config, cancelFlag, output)
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
	preconditions map[string][]contracts.PreconditionArgument,
	shouldSkipStepDueToPriorFailedStep bool,
) (string, string) {
	log.Debugf("isSupported flag = %t", isSupported)
	log.Debugf("isPluginHandlerFound flag = %t", isPluginHandlerFound)
	log.Debugf("isPreconditionEnabled flag = %t", isPreconditionEnabled)

	if shouldSkipStepDueToPriorFailedStep {
		return skipStep, fmt.Sprintf(
			"Plugin with name %s and id %s skipped due to prior step with an exit condition",
			pluginName,
			pluginId)
	}

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
					"Step execution skipped due to unsupported plugin: %s. Step name: %s",
					pluginName,
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
			} else if !isSupported || !isPluginHandlerFound {
				return skipStep, fmt.Sprintf(
					"Step execution skipped due to unsupported plugin: %s. Step name: %s",
					pluginName,
					pluginId)
			} else if !isAllowed {
				return skipStep, fmt.Sprintf(
					"Step execution skipped due to unsatisfied preconditions: '%s'. Step name: %s",
					strings.Join(unrecognizedPreconditionList, ", "),
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
	preconditions map[string][]contracts.PreconditionArgument,
) (bool, []string) {

	var isAllowed = true
	var unrecognizedPreconditionList []string

	// For current release, we only support "StringEquals" operator and "platformType"
	// operand, so explicitly checking for those and number of operands must be 2
	for key, value := range preconditions {
		switch key {
		case "StringEquals":
			if len(value) != 2 {
				unrecognizedPreconditionList = append(unrecognizedPreconditionList, fmt.Sprintf("\"%s\": operator accepts exactly 2 arguments", key))
			} else {
				if strings.Compare(value[0].InitialArgumentValue, value[1].InitialArgumentValue) == 0 {
					// StringEquals preconditions with identical arguments are not allowed
					if strings.Compare(value[0].InitialArgumentValue, "platformType") == 0 {
						unrecognizedPreconditionList = append(unrecognizedPreconditionList, fmt.Sprintf("\"%s\": [%v %v]", key, value[0].InitialArgumentValue, value[1].InitialArgumentValue))
					} else {
						// hide customer's parameters and constants
						unrecognizedPreconditionList = append(unrecognizedPreconditionList, fmt.Sprintf("\"%s\": operator's arguments can't be identical", key))
					}
				} else if ssmparameterresolver.TextContainsSsmParameters(value[0].InitialArgumentValue) || ssmparameterresolver.TextContainsSsmParameters(value[1].InitialArgumentValue) {
					unrecognizedPreconditionList = append(unrecognizedPreconditionList, fmt.Sprintf("\"%s\": operator's arguments can't contain SSM parameters", key))
				} else if ssmparameterresolver.TextContainsSecureSsmParameters(value[0].InitialArgumentValue) || ssmparameterresolver.TextContainsSecureSsmParameters(value[1].InitialArgumentValue) {
					unrecognizedPreconditionList = append(unrecognizedPreconditionList, fmt.Sprintf("\"%s\": operator's arguments can't contain secure SSM parameters", key))
				} else if strings.Compare(value[0].InitialArgumentValue, "platformType") == 0 || strings.Compare(value[1].InitialArgumentValue, "platformType") == 0 {
					// keep original logic for platformType variable
					// Platform type of OS on the instance
					instancePlatformType, _ := platform.PlatformType(log)
					log.Debugf("OS platform type of this instance = %s", instancePlatformType)

					// Variable and value can be in any order, i.e. both "StringEquals": ["platformType", "Windows"]
					// and "StringEquals": ["Windows", "platformType"] are valid
					var initialPlatformTypeValue string
					var resolvedPlatformTypeValue string
					if strings.Compare(value[0].InitialArgumentValue, "platformType") == 0 {
						initialPlatformTypeValue = value[1].InitialArgumentValue
						resolvedPlatformTypeValue = value[1].ResolvedArgumentValue
					} else {
						initialPlatformTypeValue = value[0].InitialArgumentValue
						resolvedPlatformTypeValue = value[0].ResolvedArgumentValue
					}

					if strings.Compare(strings.ToLower(initialPlatformTypeValue), strings.ToLower(resolvedPlatformTypeValue)) != 0 {
						unrecognizedPreconditionList = append(unrecognizedPreconditionList, fmt.Sprintf("\"%s\": the second argument for the platformType variable can't contain document parameters", key))
					} else if strings.Compare(instancePlatformType, strings.ToLower(initialPlatformTypeValue)) != 0 {
						// if precondition doesn't match for platformType, mark step for skip
						isAllowed = false
						unrecognizedPreconditionList = append(unrecognizedPreconditionList, fmt.Sprintf("\"%s\": [%v, %v]", key, value[0].InitialArgumentValue, value[1].InitialArgumentValue))
					}
				} else if strings.Compare(value[0].InitialArgumentValue, value[0].ResolvedArgumentValue) == 0 && strings.Compare(value[1].InitialArgumentValue, value[1].ResolvedArgumentValue) == 0 {
					unrecognizedPreconditionList = append(unrecognizedPreconditionList, fmt.Sprintf("\"%s\": at least one of operator's arguments must contain a valid document parameter", key))
				} else {
					if strings.Compare(value[0].ResolvedArgumentValue, value[1].ResolvedArgumentValue) != 0 {
						// if arbitrary StringEquals precondition is not satisfied, mark step for skip
						isAllowed = false
						unrecognizedPreconditionList = append(unrecognizedPreconditionList, fmt.Sprintf("\"%s\": [%v, %v]", key, value[0].InitialArgumentValue, value[1].InitialArgumentValue))
					}
				}
			}
		default:
			// mark for unrecognizedPrecondition (which is a form of failure)
			unrecognizedPreconditionList = append(unrecognizedPreconditionList, fmt.Sprintf("unrecognized operator: \"%s\"", key))
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

// Gets a property by name out of the plugin's inputs map, returns it as a string
// Supports bool and string only
func getStringPropByName(pluginProperties interface{}, propName string) string {
	// type cast
	pluginPropsMap, ok := pluginProperties.(map[string]interface{})
	if !ok {
		return ""
	}
	// get value from map
	propValueInterface, okm := pluginPropsMap[propName]
	if !okm {
		return ""
	}
	//type cast to string
	propValueStr, okString := propValueInterface.(string)
	propValueBool, okBool := propValueInterface.(bool)
	if !okString && !okBool {
		return ""
	}
	if !okString {
		return strconv.FormatBool(propValueBool)
	}
	return propValueStr
}

// This function handles deciding whether the current plugin should be skipped due to a prior plugin with onFailure
// or onSuccess modifiers. It also handles the finally modifier.
func getShouldPluginSkipBasedOnControlFlow(
	context context.T,
	plugins []contracts.PluginState,
	pluginIndex int,
	pluginOutputs map[string]*contracts.PluginResult,
) bool {
	log := context.Log()
	pluginState := plugins[pluginIndex]
	finallyProp := getStringPropByName(pluginState.Configuration.Properties, contracts.FinallyStepModifier)
	if finallyProp == contracts.ModifierValueTrue && pluginIndex == len(plugins)-1 {
		log.Infof(
			"Finally step detected for plugin %v",
			pluginState.Id,
		)
		// finally step, do not skip:
		return false
	}
	if finallyProp == contracts.ModifierValueTrue && pluginIndex < len(plugins)-1 {
		log.Infof(
			"FinallyStep detected for plugin %v, which is not the last plugin in list. Ignoring FinallyStep.",
			pluginState.Id,
		)
	}
	for prvPluginStateIdx := 0; prvPluginStateIdx < pluginIndex; prvPluginStateIdx++ {
		prevPluginId := plugins[prvPluginStateIdx].Id
		prvPluginResultCode := pluginOutputs[prevPluginId].Code
		onFailureProp := getStringPropByName(plugins[prvPluginStateIdx].Configuration.Properties, contracts.OnFailureModifier)
		isFailedStep := pluginOutputs[prevPluginId].Status == contracts.ResultStatusFailed
		isFailedAndExitStep := isFailedStep && onFailureProp == contracts.ModifierValueExit
		onSuccessProp := getStringPropByName(plugins[prvPluginStateIdx].Configuration.Properties, contracts.OnSuccessModifier)
		isSuccessStep := pluginOutputs[prevPluginId].Status == contracts.ResultStatusSuccess
		isSuccessAndExitStep := isSuccessStep && onSuccessProp == contracts.ModifierValueExit
		if prvPluginResultCode == contracts.ExitWithSuccess ||
			prvPluginResultCode == contracts.ExitWithFailure ||
			isFailedAndExitStep ||
			isSuccessAndExitStep {
			return true
		}
	}
	return false
}
