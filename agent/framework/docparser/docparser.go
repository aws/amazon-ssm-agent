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
	"fmt"
	"path/filepath"
	"strings"

	"github.com/aws/amazon-ssm-agent/agent/appconfig"
	"github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/aws/amazon-ssm-agent/agent/contracts"
	"github.com/aws/amazon-ssm-agent/agent/fileutil"
	"github.com/aws/amazon-ssm-agent/agent/framework/docparser/parameters"
	"github.com/aws/amazon-ssm-agent/agent/framework/docparser/parameterstore"
	"github.com/aws/amazon-ssm-agent/agent/jsonutil"
	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/amazon-ssm-agent/agent/versionutil"
)

const (
	preconditionSchemaVersion string = "2.2"
)

// DocumentParserInfo represents the parsed information from the request
type DocumentParserInfo struct {
	OrchestrationDir  string
	S3Bucket          string
	S3Prefix          string
	MessageId         string
	DocumentId        string
	DefaultWorkingDir string
	CloudWatchConfig  contracts.CloudWatchConfiguration
}

// InitializeDocState is a method to obtain the state of the document.
// This method calls into ParseDocument to obtain the InstancePluginInformation
func InitializeDocState(
	context context.T,
	documentType contracts.DocumentType,
	docContent IDocumentContent,
	docInfo contracts.DocumentInfo,
	parserInfo DocumentParserInfo,
	params map[string]interface{}) (docState contracts.DocumentState, err error) {

	docState.SchemaVersion = docContent.GetSchemaVersion()
	docState.DocumentType = documentType
	docState.DocumentInformation = docInfo
	docState.IOConfig = docContent.GetIOConfiguration(parserInfo)

	pluginInfo, err := docContent.ParseDocument(context, docInfo, parserInfo, params)
	if err != nil {
		return
	}
	docState.InstancePluginsInformation = pluginInfo
	return docState, nil
}

type IDocumentContent interface {
	GetSchemaVersion() string
	GetIOConfiguration(parserInfo DocumentParserInfo) contracts.IOConfiguration
	ParseDocument(context context.T, docInfo contracts.DocumentInfo, parserInfo DocumentParserInfo, params map[string]interface{}) (pluginsInfo []contracts.PluginState, err error)
}

// TODO: move DocumentContent/SessionDocumentContent from contracts to docparser.
type DocContent contracts.DocumentContent
type SessionDocContent contracts.SessionDocumentContent

// GetSchemaVersion is a method used to get document schema version
func (docContent *DocContent) GetSchemaVersion() string {
	return docContent.SchemaVersion
}

// GetIOConfiguration is a method used to get IO config from the document
func (docContent *DocContent) GetIOConfiguration(parserInfo DocumentParserInfo) contracts.IOConfiguration {
	return contracts.IOConfiguration{
		OrchestrationDirectory: parserInfo.OrchestrationDir,
		OutputS3BucketName:     parserInfo.S3Bucket,
		OutputS3KeyPrefix:      parserInfo.S3Prefix,
		CloudWatchConfig:       parserInfo.CloudWatchConfig,
	}
}

// ParseDocument is a method used to parse documents that are not received by any service (MDS or State manager)
func (docContent *DocContent) ParseDocument(context context.T,
	docInfo contracts.DocumentInfo,
	parserInfo DocumentParserInfo,
	params map[string]interface{}) (pluginsInfo []contracts.PluginState, err error) {

	if err = validateSchema(docContent.SchemaVersion); err != nil {
		return
	}
	if err = getValidatedParameters(context, params, docContent); err != nil {
		return
	}

	return parseDocumentContent(*docContent, parserInfo, context.Log(), params)
}

// GetSchemaVersion is a method used to get document schema version
func (sessionDocContent *SessionDocContent) GetSchemaVersion() string {
	return sessionDocContent.SchemaVersion
}

// GetIOConfiguration is a method used to get IO config from the document
func (sessionDocContent *SessionDocContent) GetIOConfiguration(parserInfo DocumentParserInfo) contracts.IOConfiguration {
	return contracts.IOConfiguration{
		OrchestrationDirectory: parserInfo.OrchestrationDir,
		OutputS3BucketName:     sessionDocContent.Inputs.S3BucketName,
		OutputS3KeyPrefix:      sessionDocContent.Inputs.S3KeyPrefix,
	}
}

// ParseDocument is a method used to parse documents that are not received by any service (MDS or State manager)
func (sessionDocContent *SessionDocContent) ParseDocument(context context.T,
	docInfo contracts.DocumentInfo,
	parserInfo DocumentParserInfo,
	params map[string]interface{}) (pluginsInfo []contracts.PluginState, err error) {
	log := context.Log()

	if err = validateSessionDocumentSchema(sessionDocContent.SchemaVersion); err != nil {
		return
	}
	if err = validateAndReplaceSessionDocumentParameters(context, params, sessionDocContent); err != nil {
		return
	}

	resolvedDocContent, _ := jsonutil.MarshalIndent(*sessionDocContent)
	log.Debugf("Resolved session document content %s", resolvedDocContent)

	return sessionDocContent.parsePluginStateForStartSession(parserInfo, docInfo)
}

// validateAndReplaceSessionDocumentParameters validates the parameters and modifies the document content by replacing all parameters with their actual values.
func validateAndReplaceSessionDocumentParameters(context context.T, params map[string]interface{}, docContent *SessionDocContent) error {
	log := context.Log()
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
	if err := parameterstore.ValidateSSMParameters(context, docContent.Parameters, validParameters); err != nil {
		return err
	}

	err := replaceValidatedSessionParameters(context, docContent, validParameters)
	return err
}

// replaceValidatedSessionParameters replaces parameters with their values.
func replaceValidatedSessionParameters(
	context context.T,
	docContent *SessionDocContent,
	params map[string]interface{}) error {
	logger := context.Log()
	var err error

	if docContent.Properties != nil {

		// Replace document parameters
		docContent.Properties = parameters.ReplaceParameters(docContent.Properties, params, logger)

		// Resolve SSM parameters
		if docContent.Properties, err = parameterstore.Resolve(context, docContent.Properties); err != nil {
			return err
		}
	}

	inputs := docContent.Inputs
	var rawData map[string]interface{}
	if err = jsonutil.Remarshal(inputs, &rawData); err != nil {
		logger.Errorf("Encountered an error while parsing document: %v", err)
		return err
	}
	resolvedRawData := parameters.ReplaceParameters(rawData, params, logger)

	// Resolve SSM Parameters
	if resolvedRawData, err = parameterstore.Resolve(context, resolvedRawData); err != nil {
		return err
	}

	var resolvedInputs contracts.SessionInputs
	if err = jsonutil.Remarshal(resolvedRawData, &resolvedInputs); err != nil {
		logger.Errorf("Encountered an error while resolving document content: %v", err)
		return err
	}
	docContent.Inputs = resolvedInputs

	return nil
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
			case contracts.ParamTypeStringMap:
				result[name] = *(param[0])
			default:
				log.Debug("unknown parameter type ", definition.ParamType)
			}
		}
	}
	log.Debug("Parameters to be applied are ", result)
	return result
}

// parseDocumentContent parses an SSM Document and returns the plugin information
func parseDocumentContent(docContent DocContent, parserInfo DocumentParserInfo, log log.T, params map[string]interface{}) (pluginsInfo []contracts.PluginState, err error) {

	switch docContent.SchemaVersion {
	case "1.0", "1.2":
		return parsePluginStateForV10Schema(docContent, parserInfo.OrchestrationDir, parserInfo.S3Bucket, parserInfo.S3Prefix, parserInfo.MessageId, parserInfo.DocumentId, parserInfo.DefaultWorkingDir)

	case "2.0", "2.0.1", "2.0.2", "2.0.3", "2.2":

		return parsePluginStateForV20Schema(docContent, parserInfo.OrchestrationDir, parserInfo.S3Bucket, parserInfo.S3Prefix, parserInfo.MessageId, parserInfo.DocumentId, parserInfo.DefaultWorkingDir, log, params)

	default:
		return pluginsInfo, fmt.Errorf("Unsupported document")
	}
}

// parsePluginStateForV10Schema initializes pluginsInfo for the docState. Used for document v1.0 and 1.2
func parsePluginStateForV10Schema(
	docContent DocContent,
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
	docContent DocContent,
	orchestrationDir, s3Bucket, s3Prefix, messageID, documentID, defaultWorkingDir string, log log.T, params map[string]interface{}) (pluginsInfo []contracts.PluginState, err error) {

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
			Preconditions:           parsePluginParametersInPreconditions(&docContent, instancePluginConfig.Preconditions, params, log),
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

// parsePluginParametersInPreconditions modifies plugin preconditions as defined in PluginConfig to match the structure
// expected by the plugin executor (plugin.Configuration -> PreconditionArgument)
func parsePluginParametersInPreconditions(docContent *DocContent, precondition map[string][]string, params map[string]interface{}, log log.T) map[string][]contracts.PreconditionArgument {
	parsedPreconditions := make(map[string][]contracts.PreconditionArgument)
	for operator, args := range precondition {
		parsedPreconditions[operator] = validateAndReplaceParametersInPreconditionArguments(docContent, args, params, log)
	}
	return parsedPreconditions
}

// validateAndReplaceParametersInPreconditionArguments validates document parameters and modifies plugin preconditions
// by replacing document parameters with their values
// TODO: return a list of invalid parameters and expose it to the user in the document execution output
// Idea: add a section to the DocumentState to store all warnings and errors that occur during document processing
// and access them later in the sendReply or UpdateAssociation
func validateAndReplaceParametersInPreconditionArguments(docContent *DocContent, args []string, params map[string]interface{}, log log.T) []contracts.PreconditionArgument {

	//ValidateParameterNames
	validParameters := parameters.ValidParameters(log, params)

	// add default values for missing parameters
	for k, v := range docContent.Parameters {
		if _, ok := validParameters[k]; !ok {
			validParameters[k] = v.DefaultVal
		}
	}

	// replace document parameters in each of the arguments
	parsedArguments := make([]contracts.PreconditionArgument, len(args))
	for i, arg := range args {
		parsedArguments[i] = contracts.PreconditionArgument{
			InitialArgumentValue:  arg,
			ResolvedArgumentValue: parameters.ReplaceParameters(arg, validParameters, log).(string),
		}
	}
	return parsedArguments
}

// parsePluginStateForStartSession initializes instancePluginsInfo for the docState. Used by startSession.
func (sessionDocContent *SessionDocContent) parsePluginStateForStartSession(
	parserInfo DocumentParserInfo,
	docInfo contracts.DocumentInfo) (pluginsInfo []contracts.PluginState, err error) {

	// getPluginConfigurations converts from PluginConfig (structure from the MGS message) to plugin.Configuration (structure expected by the plugin)
	pluginName := sessionDocContent.SessionType

	// decides which user to use. User from IAM principal always has higher priority than the default one.
	runAsUser := sessionDocContent.Inputs.RunAsDefaultUser
	if strings.TrimSpace(docInfo.RunAsUser) != "" {
		runAsUser = docInfo.RunAsUser
	}

	s3EncryptionEnabled, err := parameters.ConvertToBool(sessionDocContent.Inputs.S3EncryptionEnabled)
	if err != nil {
		return
	}
	cloudWatchEncryptionEnabled, err := parameters.ConvertToBool(sessionDocContent.Inputs.CloudWatchEncryptionEnabled)
	if err != nil {
		return
	}
	runAsEnabled, err := parameters.ConvertToBool(sessionDocContent.Inputs.RunAsEnabled)
	if err != nil {
		return
	}

	config := contracts.Configuration{
		MessageId:                   parserInfo.MessageId,
		BookKeepingFileName:         parserInfo.DocumentId,
		PluginName:                  pluginName,
		PluginID:                    pluginName,
		DefaultWorkingDirectory:     parserInfo.DefaultWorkingDir,
		SessionId:                   docInfo.DocumentID,
		OutputS3KeyPrefix:           sessionDocContent.Inputs.S3KeyPrefix,
		OutputS3BucketName:          sessionDocContent.Inputs.S3BucketName,
		S3EncryptionEnabled:         s3EncryptionEnabled,
		OrchestrationDirectory:      fileutil.BuildPath(parserInfo.OrchestrationDir, pluginName),
		ClientId:                    docInfo.ClientId,
		CloudWatchLogGroup:          sessionDocContent.Inputs.CloudWatchLogGroupName,
		CloudWatchStreamingEnabled:  sessionDocContent.Inputs.CloudWatchStreamingEnabled,
		CloudWatchEncryptionEnabled: cloudWatchEncryptionEnabled,
		KmsKeyId:                    sessionDocContent.Inputs.KmsKeyId,
		Properties:                  sessionDocContent.Properties,
		RunAsEnabled:                runAsEnabled,
		RunAsUser:                   runAsUser,
		ShellProfile:                sessionDocContent.Inputs.ShellProfile,
		SessionOwner:                docInfo.SessionOwner,
	}

	var plugin contracts.PluginState
	plugin.Configuration = config
	plugin.Id = config.PluginID
	plugin.Name = config.PluginName
	pluginsInfo = append(pluginsInfo, plugin)

	return
}

// validateSessionDocumentSchema checks if the session manager document schema version is supported by this agent version
func validateSessionDocumentSchema(documentSchemaVersion string) error {
	// Check if the document version is supported by this agent version
	if _, isDocumentVersionSupport := appconfig.SupportedSessionDocumentVersions[documentSchemaVersion]; !isDocumentVersionSupport {
		errorMsg := fmt.Sprintf(
			"Document with schema version %s is not supported by this version of ssm agent, please update to latest version",
			documentSchemaVersion)
		return fmt.Errorf("%v", errorMsg)
	}
	return nil
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

// getValidatedParameters validates the parameters and modifies the document content by replacing all ssm parameters with their actual values.
func getValidatedParameters(context context.T, params map[string]interface{}, docContent *DocContent) error {
	log := context.Log()

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
	if err := parameterstore.ValidateSSMParameters(context, docContent.Parameters, validParameters); err != nil {
		return err
	}

	err := replaceValidatedPluginParameters(context, docContent, validParameters)
	return err
}

// replaceValidatedPluginParameters replaces parameters with their values, within the plugin Properties.
func replaceValidatedPluginParameters(
	context context.T,
	docContent *DocContent,
	params map[string]interface{}) error {
	logger := context.Log()
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
			if updatedRuntimeConfig[pluginName].Settings, err = parameterstore.Resolve(context, updatedRuntimeConfig[pluginName].Settings); err != nil {
				return err
			}

			// Resolves SSM parameters
			if updatedRuntimeConfig[pluginName].Properties, err = parameterstore.Resolve(context, updatedRuntimeConfig[pluginName].Properties); err != nil {
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
			if updatedMainSteps[index].Settings, err = parameterstore.Resolve(context, updatedMainSteps[index].Settings); err != nil {
				return err
			}

			// Resolves SSM parameters
			if updatedMainSteps[index].Inputs, err = parameterstore.Resolve(context, updatedMainSteps[index].Inputs); err != nil {
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
	versionCompare, err := versionutil.VersionCompare(schemaVersion, preconditionSchemaVersion)
	if err == nil && versionCompare >= 0 {
		response = true
	}

	return response
}

// ParseDocumentNameAndVersion parses the name and version from the document name
func ParseDocumentNameAndVersion(name string) (docName, docVersion string) {
	if len(name) == 0 {
		return "", ""
	}

	//This gets the document name and version if the fullARN is provided
	//if arn:aws:ssm:us-east-1:1234567890:document/NameOfDoc:2 is provided
	//docNameWithVersion will be NameOfDoc:2
	docNameWithVersion := filepath.Base(name)
	docNameArray := strings.Split(docNameWithVersion, ":")
	if len(docNameArray) > 1 {
		// docVersion will be 2
		docVersion = docNameArray[1]
	}

	//This gets the document name if the fullARN is provided
	//docName will be arn:aws:ssm:us-east-1:1234567890:document/NameOfDoc
	docName = strings.TrimSuffix(name, ":"+docVersion)

	return docName, docVersion
}
