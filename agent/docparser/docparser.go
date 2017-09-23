// Copyright 2017 Amazon.com, Inc. or its affiliates. All Rights Reserved.
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

// Package docparser contains methods for parsing and encoding any type of document,
// i.e. association document, MDS/SSM messages, offline service documents, etc.
package docparser

import (
	"github.com/aws/amazon-ssm-agent/agent/appconfig"
	"github.com/aws/amazon-ssm-agent/agent/contracts"
	"github.com/aws/amazon-ssm-agent/agent/fileutil"
	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/amazon-ssm-agent/agent/parameters"
	"github.com/aws/amazon-ssm-agent/agent/parameterstore"
	"github.com/aws/amazon-ssm-agent/agent/updateutil"

	"fmt"
)

const (
	preconditionSchemaVersion string = "2.2"
)

type DocumentParserInfo struct {
	OrchestrationDir  string
	S3Bucket          string
	S3Prefix          string
	MessageId         string
	DocumentId        string
	DefaultWorkingDir string
}

// InitializeDocState is a method to obtain the state of the document.
// This method calls into ParseDocument to obtain the InstancePluginInformation
func InitializeDocState(log log.T,
	documentType contracts.DocumentType,
	docContent *contracts.DocumentContent,
	docInfo contracts.DocumentInfo,
	parserInfo DocumentParserInfo,
	params map[string]interface{}) (docState contracts.DocumentState, err error) {

	docState.SchemaVersion = docContent.SchemaVersion
	docState.DocumentType = documentType
	docState.DocumentInformation = docInfo

	pluginInfo, err := ParseDocument(log, docContent, parserInfo, params)
	if err != nil {
		return
	}
	docState.InstancePluginsInformation = pluginInfo
	return docState, nil
}

// ParseDocument is a method used to parse documents that are not received by any service (MDS or State manager)
func ParseDocument(log log.T,
	docContent *contracts.DocumentContent,
	parserInfo DocumentParserInfo,
	params map[string]interface{}) (pluginsInfo []contracts.PluginState, err error) {

	if err = validateSchema(docContent.SchemaVersion); err != nil {
		return
	}
	if err = getValidatedParameters(log, params, docContent); err != nil {
		return
	}

	return parseDocumentContent(*docContent, parserInfo)
}

// ParseParameters is a method to parse the ssm parameters into a string map interface
func ParseParameters(log log.T, params map[string][]*string, paramsDef map[string]*contracts.Parameter) map[string]interface{} {
	result := make(map[string]interface{})

	for name, param := range params {

		if definition, ok := paramsDef[name]; ok {
			switch definition.ParamType {
			case contracts.ParamTypeString:
				result[name] = *(param[0])
			case contracts.ParamTypeStringList:
				newParam := []string{}
				for _, value := range param {
					newParam = append(newParam, *value)
				}
				result[name] = newParam
			default:
				log.Debug("unknown parameter type ", definition.ParamType)
			}
		}
	}
	log.Debug("Parameters to be applied are ", result)
	return result
}

// parseDocumentContent parses an SSM Document and returns the plugin information
func parseDocumentContent(docContent contracts.DocumentContent, parserInfo DocumentParserInfo) (pluginsInfo []contracts.PluginState, err error) {

	switch docContent.SchemaVersion {
	case "1.0", "1.2":
		return parsePluginStateForV10Schema(docContent, parserInfo.OrchestrationDir, parserInfo.S3Bucket, parserInfo.S3Prefix, parserInfo.MessageId, parserInfo.DocumentId, parserInfo.DefaultWorkingDir)

	case "2.0", "2.0.1", "2.0.2", "2.0.3", "2.2":

		return parsePluginStateForV20Schema(docContent, parserInfo.OrchestrationDir, parserInfo.S3Bucket, parserInfo.S3Prefix, parserInfo.MessageId, parserInfo.DocumentId, parserInfo.DefaultWorkingDir)

	default:
		return pluginsInfo, fmt.Errorf("Unsupported document")
	}
}

// parsePluginStateForV10Schema initializes pluginsInfo for the docState. Used for document v1.0 and 1.2
func parsePluginStateForV10Schema(
	docContent contracts.DocumentContent,
	orchestrationDir, s3Bucket, s3Prefix, messageID, documentID, defaultWorkingDir string) (pluginsInfo []contracts.PluginState, err error) {

	if len(docContent.RuntimeConfig) == 0 {
		return pluginsInfo, fmt.Errorf("Unsupported schema format")
	}
	//initialize plugin states as map
	pluginsInfo = []contracts.PluginState{}
	// getPluginConfigurations converts from PluginConfig (structure from the MDS message) to plugin.Configuration (structure expected by the plugin)
	pluginConfigurations := []*contracts.Configuration{}
	for pluginName, pluginConfig := range docContent.RuntimeConfig {
		config := contracts.Configuration{
			Settings:                pluginConfig.Settings,
			Properties:              pluginConfig.Properties,
			OutputS3BucketName:      s3Bucket,
			OutputS3KeyPrefix:       fileutil.BuildS3Path(s3Prefix, pluginName),
			OrchestrationDirectory:  fileutil.BuildPath(orchestrationDir, pluginName),
			MessageId:               messageID,
			BookKeepingFileName:     documentID,
			PluginName:              pluginName,
			PluginID:                pluginName,
			DefaultWorkingDirectory: defaultWorkingDir,
		}
		pluginConfigurations = append(pluginConfigurations, &config)
	}

	for _, value := range pluginConfigurations {
		var plugin contracts.PluginState
		plugin.Configuration = *value
		plugin.Id = value.PluginID
		plugin.Name = value.PluginName
		pluginsInfo = append(pluginsInfo, plugin)
	}
	return
}

// parsePluginStateForV20Schema initializes instancePluginsInfo for the docState. Used by document v2.0.
func parsePluginStateForV20Schema(
	docContent contracts.DocumentContent,
	orchestrationDir, s3Bucket, s3Prefix, messageID, documentID, defaultWorkingDir string) (pluginsInfo []contracts.PluginState, err error) {

	if len(docContent.MainSteps) == 0 {
		return pluginsInfo, fmt.Errorf("Unsupported schema format")
	}
	//initialize plugin states as array
	pluginsInfo = []contracts.PluginState{}

	// set precondition flag based on document schema version
	isPreconditionEnabled := isPreconditionEnabled(docContent.SchemaVersion)

	// getPluginConfigurations converts from PluginConfig (structure from the MDS message) to plugin.Configuration (structure expected by the plugin)
	for _, instancePluginConfig := range docContent.MainSteps {
		pluginName := instancePluginConfig.Action
		config := contracts.Configuration{
			Settings:                instancePluginConfig.Settings,
			Properties:              instancePluginConfig.Inputs,
			OutputS3BucketName:      s3Bucket,
			OutputS3KeyPrefix:       fileutil.BuildS3Path(s3Prefix, pluginName),
			OrchestrationDirectory:  fileutil.BuildPath(orchestrationDir, instancePluginConfig.Name),
			MessageId:               messageID,
			BookKeepingFileName:     documentID,
			PluginName:              pluginName,
			PluginID:                instancePluginConfig.Name,
			Preconditions:           instancePluginConfig.Preconditions,
			IsPreconditionEnabled:   isPreconditionEnabled,
			DefaultWorkingDirectory: defaultWorkingDir,
		}

		var plugin contracts.PluginState
		plugin.Configuration = config
		plugin.Id = config.PluginID
		plugin.Name = config.PluginName
		pluginsInfo = append(pluginsInfo, plugin)
	}
	return
}

// validateSchema checks if the document schema version is supported by this agent version
func validateSchema(documentSchemaVersion string) error {
	// Check if the document version is supported by this agent version
	if _, isDocumentVersionSupport := appconfig.SupportedDocumentVersions[documentSchemaVersion]; !isDocumentVersionSupport {
		errorMsg := fmt.Sprintf(
			"Document with schema version %s is not supported by this version of ssm agent, please update to latest version",
			documentSchemaVersion)
		return fmt.Errorf("%v", errorMsg)
	}
	return nil
}

// getValidatedParameters validats the parameters and modifies the document content by replacing all ssm parameters with their actual values.
func getValidatedParameters(log log.T, params map[string]interface{}, docContent *contracts.DocumentContent) error {

	//ValidateParameterNames
	validParameters := parameters.ValidParameters(log, params)

	// add default values for missing parameters
	for k, v := range docContent.Parameters {
		if _, ok := validParameters[k]; !ok {
			validParameters[k] = v.DefaultVal
		}
	}

	log.Info("Validating SSM parameters")
	// Validates SSM parameters
	if err := parameterstore.ValidateSSMParameters(log, docContent.Parameters, validParameters); err != nil {
		return err
	}

	if err := replaceValidatedPluginParameters(docContent, validParameters, log); err != nil {
		return err
	}
	return nil
}

// replaceValidatedPluginParameters replaces parameters with their values, within the plugin Properties.
func replaceValidatedPluginParameters(
	docContent *contracts.DocumentContent,
	params map[string]interface{},
	logger log.T) error {
	var err error

	//TODO: Refactor this to not not reparse the docContent
	runtimeConfig := docContent.RuntimeConfig
	// we assume that one of the runtimeConfig and mainSteps should be nil
	if runtimeConfig != nil && len(runtimeConfig) != 0 {
		updatedRuntimeConfig := make(map[string]*contracts.PluginConfig)
		for pluginName, pluginConfig := range runtimeConfig {
			updatedRuntimeConfig[pluginName] = pluginConfig
			updatedRuntimeConfig[pluginName].Settings = parameters.ReplaceParameters(pluginConfig.Settings, params, logger)
			updatedRuntimeConfig[pluginName].Properties = parameters.ReplaceParameters(pluginConfig.Properties, params, logger)

			logger.Debug("Resolving SSM parameters")
			// Resolves SSM parameters
			if updatedRuntimeConfig[pluginName].Settings, err = parameterstore.Resolve(logger, updatedRuntimeConfig[pluginName].Settings); err != nil {
				return err
			}

			// Resolves SSM parameters
			if updatedRuntimeConfig[pluginName].Properties, err = parameterstore.Resolve(logger, updatedRuntimeConfig[pluginName].Properties); err != nil {
				return err
			}
		}
		docContent.RuntimeConfig = updatedRuntimeConfig
		return nil
	}

	mainSteps := docContent.MainSteps
	if mainSteps != nil || len(mainSteps) != 0 {
		updatedMainSteps := make([]*contracts.InstancePluginConfig, len(mainSteps))
		for index, instancePluginConfig := range mainSteps {
			updatedMainSteps[index] = instancePluginConfig
			updatedMainSteps[index].Settings = parameters.ReplaceParameters(instancePluginConfig.Settings, params, logger)
			updatedMainSteps[index].Inputs = parameters.ReplaceParameters(instancePluginConfig.Inputs, params, logger)

			logger.Debug("Resolving SSM parameters")
			// Resolves SSM parameters
			if updatedMainSteps[index].Settings, err = parameterstore.Resolve(logger, updatedMainSteps[index].Settings); err != nil {
				return err
			}

			// Resolves SSM parameters
			if updatedMainSteps[index].Inputs, err = parameterstore.Resolve(logger, updatedMainSteps[index].Inputs); err != nil {
				return err
			}
		}
		docContent.MainSteps = updatedMainSteps
		return nil
	}
	return nil
}

// isPreConditionEnabled checks if precondition support is enabled by checking document schema version
func isPreconditionEnabled(schemaVersion string) (response bool) {
	response = false

	// set precondition flag based on schema version
	versionCompare, err := updateutil.VersionCompare(schemaVersion, preconditionSchemaVersion)
	if err == nil && versionCompare >= 0 {
		response = true
	}

	return response
}
