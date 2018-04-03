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
	"github.com/aws/amazon-ssm-agent/agent/fileutil"
	"github.com/aws/amazon-ssm-agent/agent/fileutil/filemanager"
	"github.com/aws/amazon-ssm-agent/agent/jsonutil"
	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/amazon-ssm-agent/agent/plugins/downloadcontent/gitresource"
	"github.com/aws/amazon-ssm-agent/agent/plugins/downloadcontent/gitresource/privategithub"
	"github.com/aws/amazon-ssm-agent/agent/plugins/downloadcontent/remoteresource"
	"github.com/aws/amazon-ssm-agent/agent/plugins/downloadcontent/s3resource"
	"github.com/aws/amazon-ssm-agent/agent/plugins/downloadcontent/ssmdocresource"
	"github.com/aws/amazon-ssm-agent/agent/task"

	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/aws/amazon-ssm-agent/agent/framework/processor/executer/iohandler"
)

const (
	GitHub      = "GitHub"      //Github represents the source type "GitHub" from where the resource can be downloaded
	S3          = "S3"          //S3 represents the source type "S3" from where the resource is being downloaded
	SSMDocument = "SSMDocument" //SSMDocument represents the source type as SSM Document

	downloadsDir = "downloads" //Directory under the orchestration directory where the downloaded resource resides

	FailExitCode = 1
	PassExitCode = 0
)

var SetPermission = SetFilePermissions

// NewPlugin returns a new instance of the plugin.
func NewPlugin() (*Plugin, error) {
	var plugin Plugin
	plugin.remoteResourceCreator = newRemoteResource
	return &plugin, nil
}

// Plugin is the type for the aws:downloadContent plugin.
type Plugin struct {
	remoteResourceCreator func(log log.T, sourceType string, SourceInfo string) (remoteresource.RemoteResource, error)
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

// newRemoteResource switches between the source type and returns a struct of the source type that implements remoteresource
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
		return nil, fmt.Errorf("Invalid SourceType - %v", SourceType)
	}
}

// Execute runs multiple sets of commands and returns their outputs.
// res.Output will contain a slice of RunCommandPluginOutput.
func (p *Plugin) Execute(context context.T, config contracts.Configuration, cancelFlag task.CancelFlag, output iohandler.IOHandler) {
	p.filesys = filemanager.FileSystemImpl{}
	p.execute(context, config, cancelFlag, output)
}

func (p *Plugin) execute(context context.T, config contracts.Configuration, cancelFlag task.CancelFlag, output iohandler.IOHandler) {
	log := context.Log()
	log.Info("Plugin aws:downloadContent started with configuration", config)

	if cancelFlag.ShutDown() {
		output.MarkAsShutdown()
	} else if cancelFlag.Canceled() {
		output.MarkAsCancelled()
	} else if input, err := parseAndValidateInput(config.Properties); err != nil {
		output.MarkAsFailed(err)
	} else {
		p.runCopyContent(log, input, config, output)
	}
}

// runCopyContent figures out the type of source, downloads the resource, saves it on disk and returns information required for it
func (p *Plugin) runCopyContent(log log.T, input *DownloadContentPlugin, config contracts.Configuration, output iohandler.IOHandler) {

	//Run aws:downloadContent plugin
	log.Debug("Inside run downloadcontent function")

	// remoteResourceCreator makes a call to a function that creates a new remote resource based on the source type
	log.Debug("Creating resource of type - ", input.SourceType)
	remoteResource, err := p.remoteResourceCreator(log, input.SourceType, input.SourceInfo)
	if err != nil {
		output.MarkAsFailed(err)
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

	log.Debug("About to validate source info")
	if valid, err := remoteResource.ValidateLocationInfo(); !valid {
		output.MarkAsFailed(err)
		return
	}

	var result *remoteresource.DownloadResult
	log.Debug("Downloading resource")
	if err, result = remoteResource.DownloadRemoteResource(log, p.filesys, destinationPath); err != nil {
		output.MarkAsFailed(err)
		return
	}

	if err := setPermissions(log, result); err != nil {
		output.MarkAsFailed(fmt.Errorf("Failed to set right permissions to the content. Error - %v", err))
		return
	}

	output.AppendInfof("Content downloaded to %v", destinationPath)
	output.MarkAsSucceeded()
	return
}

func setPermissions(log log.T, result *remoteresource.DownloadResult) error {
	for _, path := range result.Files {
		log.Infof("Setting permission for file %v", path)
		if fileutil.IsDirectory(path) {
			return fmt.Errorf("Internal error - file is expected, but found directory - %v", path)
		}
		if err := SetPermission(log, path); err != nil {
			return fmt.Errorf("Failed to set right permissions to the content. Error - %v", err)
		}
	}

	return nil
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
	// ensure non-empty source type
	if input.SourceType == "" {
		return false, errors.New("SourceType must be specified")
	}
	//ensure all entries are valid
	if input.SourceType != GitHub && input.SourceType != S3 && input.SourceType != SSMDocument {
		return false, errors.New("Unsupported source type")
	}
	// ensure non-empty source info
	if input.SourceInfo == "" {
		return false, errors.New("SourceInfo must be specified")
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
