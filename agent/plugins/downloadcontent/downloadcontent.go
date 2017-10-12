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

// Package downloadcontent implements the aws:downloadContent plugin
package downloadcontent

import (
	"github.com/aws/amazon-ssm-agent/agent/appconfig"
	"github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/aws/amazon-ssm-agent/agent/contracts"
	"github.com/aws/amazon-ssm-agent/agent/fileutil/filemanager"
	"github.com/aws/amazon-ssm-agent/agent/jsonutil"
	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/amazon-ssm-agent/agent/plugins/downloadcontent/gitresource"
	"github.com/aws/amazon-ssm-agent/agent/plugins/downloadcontent/gitresource/privategithub"
	"github.com/aws/amazon-ssm-agent/agent/plugins/downloadcontent/remoteresource"
	"github.com/aws/amazon-ssm-agent/agent/plugins/downloadcontent/s3resource"
	"github.com/aws/amazon-ssm-agent/agent/plugins/downloadcontent/ssmdocresource"
	"github.com/aws/amazon-ssm-agent/agent/plugins/pluginutil"
	"github.com/aws/amazon-ssm-agent/agent/task"

	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
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

var SetPermission = SetFilePermissions

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

// Plugin is the type for the aws:downloadContent plugin.
type Plugin struct {
	pluginutil.DefaultPlugin
	remoteResourceCreator func(log log.T, locationType string, SourceInfo string) (remoteresource.RemoteResource, error)
	filesys               filemanager.FileSystem
}

// ExecutePluginInput is a struct that holds the parameters sent through send command
type DownloadContentPlugin struct {
	contracts.PluginInput
	SourceType      string `json:"sourceType"`
	SourceInfo      string `json:"sourceInfo"`
	DestinationPath string `json:"destinationPath"`
	// TODO: 08/25/2017 meloniam@ Change the type of SourceInfo and documentParameters to map[string]interface{}
	// TODO: https://amazon.awsapps.com/workdocs/index.html#/document/7d56a42ea5b040a7c33548d77dc98040f0fb380bbbfb2fd580c861225e2ee1c7
}

// newRemoteResource switches between the location type and returns a struct of the location type that implements remoteresource
func newRemoteResource(log log.T, SourceType string, SourceInfo string) (resource remoteresource.RemoteResource, err error) {
	switch SourceType {
	case GitHub:
		// TODO: meloniam@ 08/24/2017 Replace string type to map[string]inteface{} type once Runcommand supports string maps
		// TODO: https://amazon.awsapps.com/workdocs/index.html#/document/7d56a42ea5b040a7c33548d77dc98040f0fb380bbbfb2fd580c861225e2ee1c7
		token := privategithub.NewTokenInfoImpl()
		return gitresource.NewGitResource(log, SourceInfo, token)
	case S3:
		return s3resource.NewS3Resource(log, SourceInfo)
	case SSMDocument:
		return ssmdocresource.NewSSMDocResource(SourceInfo)
	default:
		return nil, fmt.Errorf("Invalid Location type - %v", SourceType)
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
	log.Info("Plugin aws:downloadContent started with configuration", config)

	res.StartDateTime = time.Now()
	defer func() { res.EndDateTime = time.Now() }()

	var output contracts.PluginOutput

	if cancelFlag.ShutDown() {
		output.MarkAsShutdown()
	} else if cancelFlag.Canceled() {
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
		// TODO: meloniam@ 09/28/2017: Reduce the number of arguments here for all plugins.
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
	return res
}

// runCopyContent figures out the type of location, downloads the resource, saves it on disk and returns information required for it
func (p *Plugin) runCopyContent(log log.T, input *DownloadContentPlugin, config contracts.Configuration, output *contracts.PluginOutput) {

	//Run aws:downloadContent plugin
	log.Debug("Inside run downloadcontent function")

	// remoteResourceCreator makes a call to a function that creates a new remote resource based on the location type
	log.Debug("Creating resource of type - ", input.SourceType)
	remoteResource, err := p.remoteResourceCreator(log, input.SourceType, input.SourceInfo)
	if err != nil {
		output.MarkAsFailed(log, err)
		return
	}
	var destinationPath string

	// If path is absolute, then download to the path,
	// else download to orchestrationDir/<downloads dir>/relative path
	if filepath.IsAbs(input.DestinationPath) {
		destinationPath = input.DestinationPath
	} else {
		log.Debugf("PluginId, plugin name, orch dir  - %v, %v, %v ", config.PluginID, config.PluginName, config.OrchestrationDirectory)
		orchestrationDir := strings.TrimSuffix(config.OrchestrationDirectory, config.PluginID)

		// The reason for not using Join or Buildpath here is so that the trailing "\" in case of windows is not dropped.
		destinationPath = filepath.Join(orchestrationDir, downloadsDir) + string(os.PathSeparator) + input.DestinationPath
	}

	log.Debug("About to validate location info")
	if valid, err := remoteResource.ValidateLocationInfo(); !valid {
		output.MarkAsFailed(log, err)
		return
	}
	log.Debug("Downloading resource")
	if err = remoteResource.Download(log, p.filesys, destinationPath); err != nil {
		output.MarkAsFailed(log, err)
		return
	}

	if err := SetPermission(log, destinationPath); err != nil {
		output.MarkAsFailed(log, fmt.Errorf("Failed to set right permissions to the content. Error - %v", err))
	}

	output.AppendInfof(log, "Content downloaded to %v", destinationPath)
	output.MarkAsSucceeded()
	return
}

// Name returns the plugin name
func Name() string {
	return appconfig.PluginDownloadContent
}

// parseAndValidateInput parses the input json file and also validates its inputs
func parseAndValidateInput(rawPluginInput interface{}) (*DownloadContentPlugin, error) {
	var input DownloadContentPlugin
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
func validateInput(input *DownloadContentPlugin) (valid bool, err error) {
	// ensure non-empty location type
	if input.SourceType == "" {
		return false, errors.New("Location Type must be specified")
	}
	//ensure all entries are valid
	if input.SourceType != GitHub && input.SourceType != S3 && input.SourceType != SSMDocument {
		return false, errors.New("Unsupported location type")
	}
	// ensure non-empty location info
	if input.SourceInfo == "" {
		return false, errors.New("Location Information must be specified")
	}
	return true, nil
}

// SetFilePermissions applies execute permissions to the folder
func SetFilePermissions(log log.T, workingDir string) error {

	var permissionsWalk = func(path string, info os.FileInfo, e error) (err error) {
		log.Info("Changing permissions for ", path)
		return os.Chmod(path, appconfig.ReadWriteExecuteAccess)
	}

	err := filepath.Walk(workingDir, permissionsWalk)
	if err != nil {
		log.Errorf("Error while changing the permissions of files - %v", err.Error())
	}
	return err
}
