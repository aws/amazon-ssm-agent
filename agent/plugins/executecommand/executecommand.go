// Copyright 2017 Amazon.com, Inc. or its affiliates. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License"). You may not
// use this file except in compliance with the License. A copy of the
// License is located at
//
// http://aws.amazon.com/apache2.0/
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or
// implied. See the License for the specific language governing
// permissions and limitations under the License..

// Package executecommand implements the aws:executeCommand plugin
package executecommand

import (
	"github.com/aws/amazon-ssm-agent/agent/appconfig"
	"github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/aws/amazon-ssm-agent/agent/contracts"
	"github.com/aws/amazon-ssm-agent/agent/docmanager/model"
	"github.com/aws/amazon-ssm-agent/agent/executers"
	"github.com/aws/amazon-ssm-agent/agent/framework/processor/executer/basicexecuter"
	"github.com/aws/amazon-ssm-agent/agent/jsonutil"
	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/amazon-ssm-agent/agent/plugins/executecommand/executor"
	"github.com/aws/amazon-ssm-agent/agent/plugins/executecommand/filemanager"
	"github.com/aws/amazon-ssm-agent/agent/plugins/executecommand/gitresource"
	"github.com/aws/amazon-ssm-agent/agent/plugins/executecommand/gitresource/privategithub"
	"github.com/aws/amazon-ssm-agent/agent/plugins/executecommand/remoteresource"
	"github.com/aws/amazon-ssm-agent/agent/plugins/executecommand/s3resource"
	"github.com/aws/amazon-ssm-agent/agent/plugins/executecommand/ssmdocresource"
	"github.com/aws/amazon-ssm-agent/agent/plugins/pluginutil"
	"github.com/aws/amazon-ssm-agent/agent/task"
	"github.com/aws/amazon-ssm-agent/agent/times"
	"github.com/go-yaml/yaml"

	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"time"
)

const (
	GitHub      = "GitHub"      //Github represents the location type "GitHub" from where the resource can be downloaded
	S3          = "S3"          //S3 represents the location type "S3" from where the resource is being downloaded
	SSMDocument = "SSMDocument" //SSMDocument represents the location type as SSM Document

	executeCommandMaxDepth = 3            //Maximum depth of document execution
	artifactsDir           = "_artifacts" //Directory under the orchestration directory where the downloaded resource resides
)

// NewPlugin returns a new instance of the plugin.
func NewPlugin(pluginConfig pluginutil.PluginConfig) (*Plugin, error) {
	var plugin Plugin
	plugin.MaxStdoutLength = pluginConfig.MaxStdoutLength
	plugin.MaxStderrLength = pluginConfig.MaxStderrLength
	plugin.StdoutFileName = pluginConfig.StdoutFileName
	plugin.StderrFileName = pluginConfig.StderrFileName
	plugin.OutputTruncatedSuffix = pluginConfig.OutputTruncatedSuffix
	plugin.ExecuteUploadOutputToS3Bucket = pluginutil.UploadOutputToS3BucketExecuter(plugin.UploadOutputToS3Bucket)

	plugin.remoteResourceCreator = newRemoteResource
	plugin.pluginManager = NewExecutePluginManager()

	return &plugin, nil
}

// ExecutePluginInput is a struct that holds the parameters sent through send command
type ExecutePluginInput struct {
	contracts.PluginInput
	LocationType       string      `json:"locationType"`
	LocationInfo       string      `json:"locationInfo"`
	EntireDirectory    string      `json:"entireDirectory"`
	DocumentParameters string      `json:"documentParameters"`
	ScriptArguments    []string    `json:"scriptArguments"`
	ExecutionTimeout   interface{} `json:"executionTimeout"`
	// TODO: 08/25/2017 meloniam@ Change the type of locationInfo and documentParameters to map[string]interface{}
	// TODO: https://amazon.awsapps.com/workdocs/index.html#/document/7d56a42ea5b040a7c33548d77dc98040f0fb380bbbfb2fd580c861225e2ee1c7
}

// Plugin is the type for the aws:executeCommand plugin.
type Plugin struct {
	pluginutil.DefaultPlugin
	remoteResourceCreator func(log log.T, locationType string, locationInfo string) (remoteresource.RemoteResource, error)
	pluginManager         executePluginManager
}

// ExecutePluginDepth is the struct that is sent through to the sub-documents to maintain the depth of execution
type ExecutePluginDepth struct {
	executeCommandDepth int
}

// executeImpl is the struct that implements executePluginManager
type executeImpl struct {
	filesys filemanager.FileSystem
	exec    executor.ExecCommand
}

//NewExecutePluginManager returns an object of type executePlugin
func NewExecutePluginManager() executeImpl {
	return executeImpl{
		filesys: filemanager.FileSystemImpl{},
		exec: executor.ExecCommandImpl{
			ScriptExecutor: executers.ShellCommandExecuter{},
			DocExecutor:    basicexecuter.NewBasicExecuter,
		},
	}

}

type executePluginManager interface {
	GetResource(log log.T, input *ExecutePluginInput, config contracts.Configuration, rem remoteresource.RemoteResource) (resourceInfo remoteresource.ResourceInfo, err error)
	PrepareDocumentForExecution(log log.T, remoteResourceInfo remoteresource.ResourceInfo, config contracts.Configuration, parameters string) (pluginsInfo []model.PluginState, err error)
	ExecuteDocument(context context.T, pluginsInfo []model.PluginState, bookkeepingFile string, output *contracts.PluginOutput)
	ExecuteScript(log log.T, fileName string, arguments []string, executionTimeout int, output *contracts.PluginOutput)
	CleanUpDownloadedContent(log log.T, orchestrationDir string, filesysdep filemanager.FileSystem) error
}

// Execute runs multiple sets of commands and returns their outputs.
// res.Output will contain a slice of RunCommandPluginOutput.
func (p *Plugin) Execute(context context.T, config contracts.Configuration, cancelFlag task.CancelFlag) (res contracts.PluginResult) {
	return p.execute(context, config, cancelFlag, filemanager.FileSystemImpl{})
}

func (p *Plugin) execute(context context.T, config contracts.Configuration, cancelFlag task.CancelFlag, filesysdep filemanager.FileSystem) (res contracts.PluginResult) {
	log := context.Log()
	log.Info("Plugin aws:execute started with configuration", config)

	res.StartDateTime = time.Now()
	defer func() { res.EndDateTime = time.Now() }()

	var output contracts.PluginOutput

	if cancelFlag.ShutDown() {
		res.Code = 1
		res.Status = contracts.ResultStatusFailed
		output.MarkAsShutdown()
	} else if cancelFlag.Canceled() {
		res.Code = 1
		res.Status = contracts.ResultStatusCancelled
		output.MarkAsCancelled()
	} else if input, err := parseAndValidateInput(config.Properties); err != nil {
		output.MarkAsFailed(log, err)
	} else {

		p.runExecuteCommand(context, input, config, &output, filesysdep)
	}
	if err := p.pluginManager.CleanUpDownloadedContent(log, config.OrchestrationDirectory, filesysdep); err != nil {
		output.AppendErrorf(log, "Error while cleaning up the downloaded resources: %v", err.Error())
	}

	useTemp := false
	outFile := filepath.Join(config.OrchestrationDirectory, p.StdoutFileName)

	if err := filesysdep.WriteFile(outFile, output.Stdout); err != nil {
		log.Debugf("Error writing to %v", outFile)
		output.AppendErrorf(log, "Error saving stdout: %v", err.Error())
	}
	errFile := filepath.Join(config.OrchestrationDirectory, p.StderrFileName)
	if err := filesysdep.WriteFile(errFile, output.Stderr); err != nil {
		log.Debugf("Error writing to %v", errFile)
		output.AppendErrorf(log, "Error saving stderr: %v", err.Error())
	}

	uploadErrs := p.ExecuteUploadOutputToS3Bucket(log,
		config.PluginID,
		config.OrchestrationDirectory,
		config.OutputS3BucketName,
		config.OutputS3KeyPrefix,
		useTemp,
		config.OrchestrationDirectory,
		output.Stdout,
		output.Stderr)
	for _, uploadErr := range uploadErrs {
		output.AppendError(log, uploadErr)
	}

	res.Code = output.ExitCode
	res.Status = output.Status
	res.Output = output.String()
	res.StandardOutput = pluginutil.StringPrefix(output.Stdout, p.MaxStdoutLength, p.OutputTruncatedSuffix)
	res.StandardError = pluginutil.StringPrefix(output.Stderr, p.MaxStderrLength, p.OutputTruncatedSuffix)
	pluginutil.PersistPluginInformationToCurrent(log, config.PluginID, config, res)

	return res
}

// runExecuteCommand is a method that runs the actual logic for
func (p *Plugin) runExecuteCommand(context context.T, input *ExecutePluginInput, config contracts.Configuration, output *contracts.PluginOutput, filesysdep filemanager.FileSystem) {

	log := context.Log()

	if err := filesysdep.MakeDirs(config.OrchestrationDirectory); err != nil {
		output.MarkAsFailed(log, fmt.Errorf("Failed to create orchestrationDir directory for log files - %v", err.Error()))
		return
	}

	//Run aws:executeCommand plugin
	log.Debug("Inside runExecuteCommand function")
	var resourceInfo remoteresource.ResourceInfo
	var pluginsInfo []model.PluginState

	// Set execution time
	executionTimeout := pluginutil.ValidateExecutionTimeout(log, input.ExecutionTimeout)

	//Set the depth of execution to be 1 for the first level execution
	execDepth := 1
	// Getting the current depth of execution and checking against maximum depth
	if config.Settings != nil {
		if settings, ok := config.Settings.(*ExecutePluginDepth); !ok {
			log.Error("Plugin setting is not of the right type")
			output.MarkAsFailed(log, errors.New("There was an error obtaining the depth of execution"))
			return
		} else {
			execDepth = settings.executeCommandDepth + 1
			if execDepth > executeCommandMaxDepth {
				output.MarkAsFailed(log, fmt.Errorf("Maximum depth for document execution exceeded. "+
					"Maximum depth permitted - %v and current depth - %v", executeCommandMaxDepth, execDepth))
				return
			}
		}
	}
	log.Info("Depth of execution - ", execDepth)
	// remoteResourceCreator makes a call to a function that creates a new remote resource based on the location type
	log.Debug("Creating resource of type - ", input.LocationType)
	remoteResource, err := p.remoteResourceCreator(log, input.LocationType, input.LocationInfo)
	if err != nil {
		output.MarkAsFailed(log, err)
		return
	}
	if resourceInfo, err = p.pluginManager.GetResource(log, input, config, remoteResource); err != nil {
		output.MarkAsFailed(log, fmt.Errorf("Unable to obtain the remote resource: %v", err.Error()))
		return
	}

	// This parameter validation is possible only after the resource type is figured out
	if valid, err := validateParameters(input, resourceInfo); !valid {
		output.MarkAsFailed(log, err)
		return
	}

	switch resourceInfo.TypeOfResource {
	case remoteresource.Document:
		if pluginsInfo, err = p.pluginManager.PrepareDocumentForExecution(log, resourceInfo, config, input.DocumentParameters); err != nil {
			output.MarkAsFailed(log, fmt.Errorf("There was an error while preparing documents - %v", err.Error()))
			return
		}
		// Sending execution depth in Configuration.Settings to the sub-documents
		for i, plugins := range pluginsInfo {
			plugins.Configuration.Settings = &ExecutePluginDepth{executeCommandDepth: execDepth}
			pluginsInfo[i] = plugins
		}
		p.pluginManager.ExecuteDocument(context, pluginsInfo, config.BookKeepingFileName, output)

	case remoteresource.Script:
		p.pluginManager.ExecuteScript(log, resourceInfo.LocalDestinationPath, input.ScriptArguments, executionTimeout, output)
	default:
		output.MarkAsFailed(log, fmt.Errorf("Type of resource not supported - %v", resourceInfo.TypeOfResource))
	}
	return
}

// GetResource figures out the type of location, downloads the resource, saves it on disk and returns information required for it
func (m executeImpl) GetResource(log log.T,
	input *ExecutePluginInput,
	config contracts.Configuration,
	remoteResource remoteresource.RemoteResource) (resourceInfo remoteresource.ResourceInfo, err error) {

	var entireDir bool
	destinationDir := filepath.Join(config.OrchestrationDirectory, artifactsDir)
	if entireDir, err = strconv.ParseBool(input.EntireDirectory); err != nil {
		return
	}
	log.Debug("About to validate location info")
	if valid, err := remoteResource.ValidateLocationInfo(); !valid {
		return resourceInfo, err
	}
	log.Debug("Downloading resource")
	if err = remoteResource.Download(log, m.filesys, entireDir, destinationDir); err != nil {
		return

	}
	resourceInfo = remoteResource.PopulateResourceInfo(log, destinationDir, entireDir)

	log.Info("Path to the resource on disk - ", resourceInfo.LocalDestinationPath)

	return resourceInfo, err
}

// PrepareDocumentForExecution parses the raw content of the document, validates it and returns a PluginState that can be executed.
func (m executeImpl) PrepareDocumentForExecution(log log.T, remoteResourceInfo remoteresource.ResourceInfo, config contracts.Configuration, params string) (pluginsInfo []model.PluginState, err error) {
	parameters := make(map[string]interface{})
	if params != "" {

		log.Info("Params to be unmarshaled - ", params)
		// TODO: meloniam@ 08/24/2017 - https://amazon.awsapps.com/workdocs/index.html#/document/7d56a42ea5b040a7c33548d77dc98040f0fb380bbbfb2fd580c861225e2ee1c7
		// TODO: Remove the Unmarshalling once RC supports StringMap
		// TODO: documentParameters will be of type map[string]interface{} from the beginning
		if remoteResourceInfo.ResourceExtension == remoteresource.JSONExtension {
			if json.Unmarshal([]byte(params), &parameters); err != nil {
				log.Error("Unmarshalling JSON remote resource parameters failed. Please make sure the document is in the right format")
				return pluginsInfo, err
			}
		} else if remoteResourceInfo.ResourceExtension == remoteresource.YAMLExtension {
			if yaml.Unmarshal([]byte(params), &parameters); err != nil {
				log.Error("Unmarshalling YAML remote resource parameters failed. Please make sure the document is in the right format")
				return pluginsInfo, err
			}
		} else {
			return pluginsInfo, errors.New("Extension type for documents is not supported")
		}

		log.Info("Parameters passed in are ", parameters)
	}
	var rawDocument []byte
	if rawDocument, err = filemanager.ReadFileContents(log, m.filesys, remoteResourceInfo.LocalDestinationPath); err != nil {
		log.Error("Could not read document from remote resource - ", err)
		return nil, err
	}
	log.Infof("Sending the document received for parsing - %v", string(rawDocument))

	return m.exec.ParseDocument(log, remoteResourceInfo.ResourceExtension, rawDocument, config.OrchestrationDirectory, config.OutputS3BucketName, config.OutputS3KeyPrefix, config.MessageId, config.PluginID, config.DefaultWorkingDirectory, parameters)
}

// ExecuteDocument sends the document for execution
func (m executeImpl) ExecuteDocument(context context.T,
	pluginsInfo []model.PluginState,
	bookKeepingFile string,
	output *contracts.PluginOutput) {

	m.exec.ExecuteDocument(context, pluginsInfo, bookKeepingFile, times.ToIso8601UTC(time.Now()), output)
	return

}

// ExecuteScript sends the script for execution
func (m executeImpl) ExecuteScript(log log.T, fileName string,
	arguments []string,
	executionTimeout int,
	output *contracts.PluginOutput) {
	m.exec.ExecuteScript(log, fileName, arguments, executionTimeout, output)
	return
}

// CleanUpDownloadedContent deletes the directories that contain the contents downloaded for executing the command
func (m executeImpl) CleanUpDownloadedContent(log log.T, orchestrationDir string, filesysdep filemanager.FileSystem) error {

	var artifactsDirWalk = func(path string, info os.FileInfo, e error) (err error) {
		if filesysdep.Exists(path) && (filepath.Base(path) == artifactsDir) {
			filesysdep.DeleteDirectory(path)
		}
		return err
	}

	err := filesysdep.Walk(orchestrationDir, artifactsDirWalk)
	if err != nil {
		log.Errorf("Error deleting the downloaded artifacts - %v", err.Error())
	}
	return err
}

// Name returns the plugin name
func Name() string {
	return appconfig.PluginExecuteCommand
}

// parseAndValidateInput parses the input json file and also validates its inputs
func parseAndValidateInput(rawPluginInput interface{}) (*ExecutePluginInput, error) {
	var input ExecutePluginInput
	var err error
	if err = jsonutil.Remarshal(rawPluginInput, &input); err != nil {
		return nil, fmt.Errorf("invalid format in plugin properties %v; \nerror %v", rawPluginInput, err)
	}

	if valid, err := validateInput(&input); !valid {
		return nil, fmt.Errorf("invalid input: %v", err)
	}

	return &input, nil
}

// validateInput ensures the plugin input matches the defined schema
func validateInput(input *ExecutePluginInput) (valid bool, err error) {
	// ensure non-empty location type
	if input.LocationType == "" {
		return false, errors.New("Location Type must be specified")
	}
	//ensure all entries are valid
	if input.LocationType != GitHub && input.LocationType != S3 && input.LocationType != SSMDocument {
		return false, errors.New("Unsupported location type")
	}
	// ensure non-empty location info
	if input.LocationInfo == "" {
		return false, errors.New("Location Information must be specified")
	}
	return true, nil
}

// validateParameters ensures that documents do not have scriptArguments
// and scripts do not have documentParameters
func validateParameters(input *ExecutePluginInput, resourceInfo remoteresource.ResourceInfo) (valid bool, err error) {
	if (len(input.ScriptArguments) == 1 && input.ScriptArguments[0] != "") || len(input.ScriptArguments) > 1 {
		if remoteresource.Document == resourceInfo.TypeOfResource {

			return false, errors.New("Document type of resource cannot specify script type parameters")
		}
	}
	// TODO: meloniam@ 08/24/2017 Validation of documentParameters will be done as StringMap type
	// TODO: https://amazon.awsapps.com/workdocs/index.html#/document/7d56a42ea5b040a7c33548d77dc98040f0fb380bbbfb2fd580c861225e2ee1c7
	if resourceInfo.TypeOfResource == remoteresource.Script && input.DocumentParameters != "" {
		return false, errors.New("Script type of resource cannot have document parameters specified")
	}
	return true, nil
}

// newRemoteResource switches between the location type and returns a struct of the location type that implements remoteresource
func newRemoteResource(log log.T, locationType string, locationInfo string) (resource remoteresource.RemoteResource, err error) {
	switch locationType {
	case GitHub:
		// TODO: meloniam@ 08/24/2017 Replace string type to map[string]inteface{} type once Runcommand supports string maps
		// TODO: https://amazon.awsapps.com/workdocs/index.html#/document/7d56a42ea5b040a7c33548d77dc98040f0fb380bbbfb2fd580c861225e2ee1c7
		token := privategithub.NewTokenInfoImpl()
		return gitresource.NewGitResource(log, locationInfo, token)
	case S3:
		return s3resource.NewS3Resource(log, locationInfo)
	case SSMDocument:
		return ssmdocresource.NewSSMDocResource(locationInfo)
	default:
		return nil, fmt.Errorf("Invalid Location type - %v", locationType)
	}
}
