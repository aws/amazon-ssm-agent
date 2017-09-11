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

// Package copycontent implements the aws:copyContent plugin
package copycontent

import (
	"github.com/aws/amazon-ssm-agent/agent/appconfig"
	"github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/aws/amazon-ssm-agent/agent/contracts"
	"github.com/aws/amazon-ssm-agent/agent/filemanager"
	"github.com/aws/amazon-ssm-agent/agent/jsonutil"
	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/amazon-ssm-agent/agent/platform"
	"github.com/aws/amazon-ssm-agent/agent/plugins/copycontent/gitresource"
	"github.com/aws/amazon-ssm-agent/agent/plugins/copycontent/gitresource/privategithub"
	"github.com/aws/amazon-ssm-agent/agent/plugins/copycontent/remoteresource"
	"github.com/aws/amazon-ssm-agent/agent/plugins/copycontent/s3resource"
	"github.com/aws/amazon-ssm-agent/agent/plugins/copycontent/ssmdocresource"
	"github.com/aws/amazon-ssm-agent/agent/plugins/pluginutil"
	"github.com/aws/amazon-ssm-agent/agent/task"

	"errors"
	"fmt"
	"path/filepath"
	"time"
)

const (
	GitHub      = "GitHub"      //Github represents the location type "GitHub" from where the resource can be downloaded
	S3          = "S3"          //S3 represents the location type "S3" from where the resource is being downloaded
	SSMDocument = "SSMDocument" //SSMDocument represents the location type as SSM Document

	downloadsDir = "downloads" //Directory under the orchestration directory where the downloaded resource resides

	FailExitCode = 1
	PassExitCode = 0
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

	return &plugin, nil
}

// Plugin is the type for the aws:copyContent plugin.
type Plugin struct {
	pluginutil.DefaultPlugin
	remoteResourceCreator func(log log.T, locationType string, locationInfo string) (remoteresource.RemoteResource, error)
	filesys               filemanager.FileSystem
}

// ExecutePluginInput is a struct that holds the parameters sent through send command
type CopyContentPlugin struct {
	contracts.PluginInput
	LocationType   string `json:"locationType"`
	LocationInfo   string `json:"locationInfo"`
	DestinationDir string `json:"destinationDirectory"`
	// TODO: 08/25/2017 meloniam@ Change the type of locationInfo and documentParameters to map[string]interface{}
	// TODO: https://amazon.awsapps.com/workdocs/index.html#/document/7d56a42ea5b040a7c33548d77dc98040f0fb380bbbfb2fd580c861225e2ee1c7
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

// Execute runs multiple sets of commands and returns their outputs.
// res.Output will contain a slice of RunCommandPluginOutput.
func (p *Plugin) Execute(context context.T, config contracts.Configuration, cancelFlag task.CancelFlag) (res contracts.PluginResult) {
	p.filesys = filemanager.FileSystemImpl{}
	return p.execute(context, config, cancelFlag)
}

func (p *Plugin) execute(context context.T, config contracts.Configuration, cancelFlag task.CancelFlag) (res contracts.PluginResult) {
	log := context.Log()
	log.Info("Plugin aws:copyContent started with configuration", config)

	res.StartDateTime = time.Now()
	defer func() { res.EndDateTime = time.Now() }()

	var output contracts.PluginOutput

	if cancelFlag.ShutDown() {
		res.Code = FailExitCode
		res.Status = contracts.ResultStatusFailed
		output.MarkAsShutdown()
	} else if cancelFlag.Canceled() {
		res.Code = FailExitCode
		res.Status = contracts.ResultStatusCancelled
		output.MarkAsCancelled()
	} else if input, err := parseAndValidateInput(config.Properties); err != nil {
		output.MarkAsFailed(log, err)
	} else {

		p.runCopyContent(log, input, config, &output)
	}

	if config.OrchestrationDirectory != "" {
		useTemp := false
		outFile := filepath.Join(config.OrchestrationDirectory, p.StdoutFileName)

		if err := p.filesys.MakeDirs(config.OrchestrationDirectory); err != nil {
			output.AppendError(log, "Failed to create orchestrationDir directory for log files")
		} else {
			if err := p.filesys.WriteFile(outFile, output.Stdout); err != nil {
				log.Debugf("Error writing to %v", outFile)
				output.AppendErrorf(log, "Error saving stdout: %v", err.Error())
			}
			errFile := filepath.Join(config.OrchestrationDirectory, p.StderrFileName)
			if err := p.filesys.WriteFile(errFile, output.Stderr); err != nil {
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

// runCopyContent figures out the type of location, downloads the resource, saves it on disk and returns information required for it
func (p *Plugin) runCopyContent(log log.T, input *CopyContentPlugin, config contracts.Configuration, output *contracts.PluginOutput) {

	//Run aws:copyContent plugin
	log.Debug("Inside run copyContent function")

	// remoteResourceCreator makes a call to a function that creates a new remote resource based on the location type
	log.Debug("Creating resource of type - ", input.LocationType)
	remoteResource, err := p.remoteResourceCreator(log, input.LocationType, input.LocationInfo)
	if err != nil {
		output.MarkAsFailed(log, err)
		return
	}
	var destinationDir string

	// If path is absolute, then download to the path,
	// else download to <downloads dir>/relative path
	instanceID, err := platform.InstanceID()
	if filepath.IsAbs(input.DestinationDir) {
		destinationDir = input.DestinationDir
	} else {

		destinationDir = filepath.Join(appconfig.DefaultDataStorePath, instanceID, appconfig.DefaultDocumentRootDirName, downloadsDir, input.DestinationDir)
	}

	log.Debug("About to validate location info")
	if valid, err := remoteResource.ValidateLocationInfo(); !valid {
		output.MarkAsFailed(log, err)
		return
	}
	log.Debug("Downloading resource")
	if err = remoteResource.Download(log, p.filesys, destinationDir); err != nil {
		output.MarkAsFailed(log, err)
		return
	}
	output.AppendInfof(log, "Content downloaded to %v", destinationDir)
	output.MarkAsSucceeded()
	return
}

// Name returns the plugin name
func Name() string {
	return appconfig.PluginCopyContent
}

// parseAndValidateInput parses the input json file and also validates its inputs
func parseAndValidateInput(rawPluginInput interface{}) (*CopyContentPlugin, error) {
	var input CopyContentPlugin
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
func validateInput(input *CopyContentPlugin) (valid bool, err error) {
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
