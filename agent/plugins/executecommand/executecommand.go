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
	"github.com/aws/amazon-ssm-agent/agent/framework/runpluginutil"
	"github.com/aws/amazon-ssm-agent/agent/jsonutil"
	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/amazon-ssm-agent/agent/plugins/executecommand/document"
	"github.com/aws/amazon-ssm-agent/agent/plugins/executecommand/filemanager"
	"github.com/aws/amazon-ssm-agent/agent/plugins/executecommand/gitresource"
	"github.com/aws/amazon-ssm-agent/agent/plugins/executecommand/remoteresource"
	"github.com/aws/amazon-ssm-agent/agent/plugins/executecommand/s3resource"
	"github.com/aws/amazon-ssm-agent/agent/plugins/executecommand/ssmdocresource"
	"github.com/aws/amazon-ssm-agent/agent/plugins/pluginutil"
	"github.com/aws/amazon-ssm-agent/agent/task"

	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"strconv"
	"time"
)

const (
	Github      = "Github"       //Github represents the location type "Github" from where the resource can be downloaded
	S3          = "S3"           //S3 represents the location type "S3" from where the resource is being downloaded
	SSMDocument = "SSM Document" //SSMDocument represents the location type as SSM Document

	executeCommandMaxDepth = 3 //Maximum depth of document execution
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
	LocationType      string `json:"locationType"`
	LocationInfo      string `json:"locationInfo"`
	EntireDirectory   string `json:"entireDirectory"`
	RuntimeParameters string `json:"runtimeParameters"`
	//TODO: Change the type of locationInfo and runtimeParameters to map[string]interface{} once Runcommand supports StringMaps
}

// Plugin is the type for the aws:executeCommand plugin.
type Plugin struct {
	pluginutil.DefaultPlugin
	executeCommandDepth   int
	remoteResourceCreator func(log log.T, locationType string, locationInfo string) (remoteresource.RemoteResource, error)
	pluginManager         executePluginManager
}

// executePlugin is the struct that implements executePluginManager
type executePlugin struct {
	filesys filemanager.FileSystem
	doc     document.ExecCommand
}

//NewExecutePluginManager returns an object of type executePlugin
func NewExecutePluginManager() executePlugin {
	return executePlugin{
		filesys: filemanager.FileSystemImpl{},
		doc:     document.ExecCommandImpl{},
	}

}

//TODO: Check if this can be replaced with private members
type executePluginManager interface {
	GetResource(log log.T, input *ExecutePluginInput, config contracts.Configuration, rem remoteresource.RemoteResource) (resourceInfo remoteresource.ResourceInfo, err error)
	PrepareDocumentForExecution(log log.T, remoteResourceInfo remoteresource.ResourceInfo, config contracts.Configuration, parameters string) (pluginsInfo []model.PluginState, err error)
	//ExecuteResource()
}

// Execute runs multiple sets of commands and returns their outputs.
// res.Output will contain a slice of RunCommandPluginOutput.
func (p *Plugin) Execute(context context.T, config contracts.Configuration, cancelFlag task.CancelFlag, subDocumentRunner runpluginutil.PluginRunner) (res contracts.PluginResult) {
	return p.execute(context, config, cancelFlag, subDocumentRunner, filemanager.FileSystemImpl{})
}

func (p *Plugin) execute(context context.T, config contracts.Configuration, cancelFlag task.CancelFlag, subDocumentRunner runpluginutil.PluginRunner, filesysdep filemanager.FileSystem) (res contracts.PluginResult) {
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
	} else if p.executeCommandDepth > executeCommandMaxDepth {
		// Error out if the max depth has been exceeded
		output.MarkAsFailed(log, fmt.Errorf("Maximum depth for document execution exceeded. "+
			"Maximum depth permitted - %v and current depth - %v", executeCommandMaxDepth, p.executeCommandDepth))

	} else {
		//Run aws:executeCommand plugin
		var resourceInfo remoteresource.ResourceInfo
		var pluginsInfo []model.PluginState

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

		if resourceInfo.TypeOfResource == remoteresource.Document {
			if pluginsInfo, err = p.pluginManager.PrepareDocumentForExecution(log, resourceInfo, config, input.RuntimeParameters); err != nil {
				output.MarkAsFailed(log, fmt.Errorf("There was an error while preparing documents - %v", err.Error()))
				return
			} else {
				log.Info("Plugin info - ", pluginsInfo)
			}
		}

		//ExecuteResource()

		output.MarkAsSucceeded()
	}
	if config.OrchestrationDirectory != "" {
		useTemp := false
		outFile := filepath.Join(config.OrchestrationDirectory, p.StdoutFileName)
		// create orchestration dir if needed
		if err := filesysdep.MakeDirs(config.OrchestrationDirectory); err != nil {
			output.AppendError(log, "Failed to create orchestrationDir directory for log files")
		} else {
			if err := filesysdep.WriteFile(outFile, output.Stdout); err != nil {
				log.Debugf("Error writing to %v", outFile)
				output.AppendErrorf(log, "Error saving stdout: %v", err.Error())
			}
			errFile := filepath.Join(config.OrchestrationDirectory, p.StderrFileName)
			if err := filesysdep.WriteFile(errFile, output.Stderr); err != nil {
				log.Debugf("Error writing to %v", errFile)
				output.AppendErrorf(log, "Error saving stderr: %v", err.Error())
			}
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
	}
	res.Code = output.ExitCode
	res.Status = output.Status
	res.Output = output.String()
	res.StandardOutput = pluginutil.StringPrefix(output.Stdout, p.MaxStdoutLength, p.OutputTruncatedSuffix)
	res.StandardError = pluginutil.StringPrefix(output.Stderr, p.MaxStderrLength, p.OutputTruncatedSuffix)
	pluginutil.PersistPluginInformationToCurrent(log, config.PluginID, config, res)

	return res
}

// GetRemoteResource figures out the type of location, downloads the resource, saves it on disk and returns information required for it
func (m executePlugin) GetResource(log log.T,
	input *ExecutePluginInput,
	config contracts.Configuration,
	remoteResource remoteresource.RemoteResource) (resourceInfo remoteresource.ResourceInfo, err error) {

	var entireDir bool
	destinationDir := config.OrchestrationDirectory
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
func (m executePlugin) PrepareDocumentForExecution(log log.T, remoteResourceInfo remoteresource.ResourceInfo, config contracts.Configuration, params string) (pluginsInfo []model.PluginState, err error) {
	parameters := make(map[string]interface{})
	if params != "" {

		log.Info("Params to be unmarshaled - ", params)
		//TODO: meloniam@ Remove the Unmarshalling once RC supports StringMaps
		//TODO: RunTimeParams will be of type map[string]interface{} from the beginning
		if json.Unmarshal([]byte(params), &parameters); err != nil {
			return
		}

		log.Info("Parameters passed in are ", parameters)
	}
	var rawDocument []byte
	if rawDocument, err = filemanager.ReadFileContents(log, m.filesys, remoteResourceInfo.LocalDestinationPath); err != nil {
		log.Error("Could not read document from remote resource - ", err)
		return nil, err
	}
	log.Infof("Sending the document received - %v for parsing", string(rawDocument))

	return m.doc.ParseDocument(log, rawDocument, config.OrchestrationDirectory, config.OutputS3BucketName, config.OutputS3KeyPrefix, config.MessageId, config.PluginID, config.DefaultWorkingDirectory, parameters)
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
	if input.LocationType != Github && input.LocationType != S3 && input.LocationType != SSMDocument {
		return false, errors.New("Unsupported location type")
	}

	// ensure non-empty location info
	if input.LocationInfo == "" {
		return false, errors.New("Location Information must be specified")
	}

	return true, nil
}

// newRemoteResource switches between the location type and returns a struct of the location type that implements remoteresource
func newRemoteResource(log log.T, locationType string, locationInfo string) (resource remoteresource.RemoteResource, err error) {

	switch locationType {
	case Github:
		// TODO: meloniam@ Replace nil with auth information once work is done
		// TODO: meloniam@ Replace string type to map[string]inteface{} type once Runcommand supports string maps
		return gitresource.NewGitResource(nil, locationInfo)

	case S3:
		return s3resource.NewS3Resource(log, locationInfo)

	case SSMDocument:
		return ssmdocresource.NewSSMDocResource(locationInfo)

	default:
		return nil, fmt.Errorf("Invalid Location type.")
	}
}
