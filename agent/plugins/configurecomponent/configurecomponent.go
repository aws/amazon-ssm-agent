// Copyright 2016 Amazon.com, Inc. or its affiliates. All Rights Reserved.
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
// permissions and limitations under the License.

// Package configurecomponent implements the ConfigureComponent plugin.
package configurecomponent

import (
	"errors"
	"fmt"

	"strings"

	"os"
	"path/filepath"

	"time"

	"github.com/aws/amazon-ssm-agent/agent/appconfig"
	"github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/aws/amazon-ssm-agent/agent/contracts"
	"github.com/aws/amazon-ssm-agent/agent/executers"
	"github.com/aws/amazon-ssm-agent/agent/fileutil"
	"github.com/aws/amazon-ssm-agent/agent/fileutil/artifact"
	"github.com/aws/amazon-ssm-agent/agent/jsonutil"
	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/amazon-ssm-agent/agent/plugins/pluginutil"
	"github.com/aws/amazon-ssm-agent/agent/rebooter"
	"github.com/aws/amazon-ssm-agent/agent/task"
	"github.com/aws/amazon-ssm-agent/agent/updateutil"
)

// Plugin is the type for the configurecomponent plugin.
type Plugin struct {
	pluginutil.DefaultPlugin
}

// ConfigureComponentPluginInput represents one set of commands executed by the ConfigureComponent plugin.
type ConfigureComponentPluginInput struct {
	contracts.PluginInput
	Name    string
	Version string
	Action  string
	Source  string
}

// ConfigureComponentsPluginOutput represents the output of the plugin.
type ConfigureComponentPluginOutput struct {
	contracts.PluginOutput
}

// MarkAsSucceeded marks plugin as Successful.
func (result *ConfigureComponentPluginOutput) MarkAsSucceeded() {
	result.ExitCode = 0
	result.Status = contracts.ResultStatusSuccess
}

// MarkAsFailed marks plugin as Failed.
func (result *ConfigureComponentPluginOutput) MarkAsFailed(log log.T, err error) {
	result.ExitCode = 1
	result.Status = contracts.ResultStatusFailed
	if result.Stderr != "" {
		result.Stderr = fmt.Sprintf("\n%v\n%v", result.Stderr, err.Error())
	} else {
		result.Stderr = fmt.Sprintf("\n%v", err.Error())
	}
	log.Error("failed to configure component", err.Error())
	result.Errors = append(result.Errors, err.Error())
}

// AppendInfo adds info to ConfigureComponentPluginOutput StandardOut.
func (result *ConfigureComponentPluginOutput) AppendInfo(log log.T, format string, params ...interface{}) {
	message := fmt.Sprintf(format, params...)
	log.Info(message)
	result.Stdout = fmt.Sprintf("%v\n%v", result.Stdout, message)
}

// NewPlugin returns a new instance of the plugin.
func NewPlugin(pluginConfig pluginutil.PluginConfig) (*Plugin, error) {
	var plugin Plugin
	plugin.MaxStdoutLength = pluginConfig.MaxStdoutLength
	plugin.MaxStderrLength = pluginConfig.MaxStderrLength
	plugin.StdoutFileName = pluginConfig.StdoutFileName
	plugin.StderrFileName = pluginConfig.StderrFileName
	plugin.OutputTruncatedSuffix = pluginConfig.OutputTruncatedSuffix
	plugin.Uploader = pluginutil.GetS3Config()
	plugin.ExecuteUploadOutputToS3Bucket = pluginutil.UploadOutputToS3BucketExecuter(plugin.UploadOutputToS3Bucket)

	exec := executers.ShellCommandExecuter{}
	plugin.ExecuteCommand = pluginutil.CommandExecuter(exec.Execute)

	return &plugin, nil
}

type configureManager struct{}

type pluginHelper interface {
	downloadManifest(log log.T,
		util Util,
		input *ConfigureComponentPluginInput,
		output *ConfigureComponentPluginOutput,
		context *updateutil.InstanceContext) (manifest *ComponentManifest, err error)

	downloadPackage(log log.T,
		util Util,
		input *ConfigureComponentPluginInput,
		output *ConfigureComponentPluginOutput,
		context *updateutil.InstanceContext) (err error)

	extractPackage(directory string) (err error)
}

// Assign method to global variables to allow unit tests to override
var fileDownload = artifact.Download
var fileUncompress = fileutil.Uncompress
var fileRemove = os.RemoveAll
var configureComponent = runConfigureComponent
var installComponent = runInstallComponent
var uninstallComponent = runUninstallComponent

// runConfigureComponent downloads the component manifest and performs specified action
func runConfigureComponent(
	p *Plugin,
	log log.T,
	manager pluginHelper,
	configureUtil Util,
	updateUtil updateutil.T,
	rawPluginInput interface{}) (output ConfigureComponentPluginOutput) {
	var input ConfigureComponentPluginInput
	var err error
	var context *updateutil.InstanceContext

	if err = jsonutil.Remarshal(rawPluginInput, &input); err != nil {
		output.MarkAsFailed(log,
			fmt.Errorf("invalid format in plugin properties %v; \nerror %v", rawPluginInput, err))
		return
	}

	if context, err = updateUtil.CreateInstanceContext(log); err != nil {
		output.MarkAsFailed(log, err)
		return
	}

	// download manifest file
	manifest, downloadErr := manager.downloadManifest(log, configureUtil, &input, &output, context)
	if downloadErr != nil {
		output.MarkAsFailed(log, downloadErr)
		return
	}

	switch input.Action {
	case InstallAction:
		if err = installComponent(p,
			&input,
			&output,
			manager,
			log,
			manifest.Install,
			configureUtil,
			updateUtil,
			context); err != nil {
			output.MarkAsFailed(log,
				fmt.Errorf("failed to install component: %v", err))
			return
		}

		// TO DO: Reboot if requested
	case UninstallAction:
		if err = uninstallComponent(p,
			&input,
			&output,
			log,
			manifest.Uninstall,
			updateUtil,
			context); err != nil {
			output.MarkAsFailed(log,
				fmt.Errorf("failed to uninstall component: %v", err))
			return
		}
	}

	return
}

// downloadManifest downloads component configuration file from s3 bucket.
func (m *configureManager) downloadManifest(log log.T,
	util Util,
	input *ConfigureComponentPluginInput,
	output *ConfigureComponentPluginOutput,
	context *updateutil.InstanceContext) (manifest *ComponentManifest, err error) {
	// manifest to download
	manifestName := createManifestName(input.Name)

	// path to manifest
	manifestLocation := input.Source
	if manifestLocation == "" {
		manifestLocation = createS3Location(input.Name, input.Version, context, manifestName)
	} else {
		manifestLocation = strings.Replace(input.Source, updateutil.FileNameHolder, input.Name, -1)
	}

	// path to download destination
	manifestDestination, err := util.CreateComponentFolder(input.Name, input.Version)
	if err != nil {
		errMessage := fmt.Sprintf("failed to create local component repository, %v", err.Error())
		return nil, errors.New(errMessage)
	}

	downloadInput := artifact.DownloadInput{
		SourceURL:            manifestLocation,
		DestinationDirectory: manifestDestination}

	// download package
	downloadOutput, downloadErr := fileDownload(log, downloadInput)
	if downloadErr != nil || downloadOutput.LocalFilePath == "" {
		errMessage := fmt.Sprintf("failed to download component manifest reliably, %v", downloadInput.SourceURL)
		if downloadErr != nil {
			errMessage = fmt.Sprintf("%v, %v", errMessage, downloadErr.Error())
		}
		return nil, errors.New(errMessage)
	}

	output.AppendInfo(log, "Successfully downloaded %v", downloadInput.SourceURL)

	return ParseComponentManifest(log, downloadOutput.LocalFilePath)
}

// downloadPackage downloads the installation package from s3 bucket.
func (m *configureManager) downloadPackage(log log.T,
	util Util,
	input *ConfigureComponentPluginInput,
	output *ConfigureComponentPluginOutput,
	context *updateutil.InstanceContext) (err error) {
	// package to download
	packageName := createPackageName(input.Name, context)

	// path to package
	packageLocation := input.Source
	if packageLocation == "" {
		packageLocation = createS3Location(input.Name, input.Version, context, packageName)
	}

	// path to download destination
	packageDestination, err := util.CreateComponentFolder(input.Name, input.Version)
	if err != nil {
		errMessage := fmt.Sprintf("failed to create local component repository, %v", err.Error())
		return errors.New(errMessage)
	}

	downloadInput := artifact.DownloadInput{
		SourceURL:            packageLocation,
		DestinationDirectory: packageDestination}

	// download package
	downloadOutput, downloadErr := fileDownload(log, downloadInput)
	if downloadErr != nil || downloadOutput.LocalFilePath == "" {
		errMessage := fmt.Sprintf("failed to download component installation package reliably, %v", downloadInput.SourceURL)
		if downloadErr != nil {
			errMessage = fmt.Sprintf("%v, %v", errMessage, downloadErr.Error())
		}
		return errors.New(errMessage)
	}

	output.AppendInfo(log, "Successfully downloaded %v", downloadInput.SourceURL)

	return nil
}

// extractPackage extracts the contents of the compressed installation package.
func (m *configureManager) extractPackage(directory string) (err error) {
	if err = fileUncompress(directory, directory); err != nil {
		return fmt.Errorf("failed to uncompress component installer package, %v, %v", directory, err.Error())
	}
	return nil
}

// runInstallComponent executes the install script for the specific version of component.
// TO DO: Update (check if existing version of component exists; if so, uninstall before installing)
func runInstallComponent(p *Plugin,
	input *ConfigureComponentPluginInput,
	output *ConfigureComponentPluginOutput,
	manager pluginHelper,
	log log.T,
	installCmd string,
	configureUtil Util,
	updateUtil updateutil.T,
	context *updateutil.InstanceContext) (err error) {
	log.Infof("Initiating %v %v installation", input.Name, input.Version)

	directory := filepath.Join(appconfig.ComponentRoot, input.Name, input.Version)

	// download package
	if err = manager.downloadPackage(log, configureUtil, input, output, context); err != nil {
		return err
	}

	// extract package
	if err = manager.extractPackage(directory); err != nil {
		return err
	}

	// execute installation command
	if err = updateUtil.ExeCommand(
		log,
		installCmd,
		directory, directory,
		p.StdoutFileName, p.StderrFileName,
		false); err != nil {

		return err
	}

	log.Infof("%v %v installed successfully", input.Name, input.Version)
	return nil
}

// runUninstallComponent executes the install script for the specific version of component.
func runUninstallComponent(p *Plugin,
	input *ConfigureComponentPluginInput,
	output *ConfigureComponentPluginOutput,
	log log.T,
	uninstallCmd string,
	util updateutil.T,
	context *updateutil.InstanceContext) (err error) {
	log.Infof("Initiating %v %v uninstallation", input.Name, input.Version)

	directory := filepath.Join(appconfig.ComponentRoot, input.Name, input.Version)

	// execute installation command
	if err = util.ExeCommand(
		log,
		uninstallCmd,
		directory, directory,
		p.StdoutFileName, p.StderrFileName,
		false); err != nil {

		return err
	}

	// delete local component folder
	if err = fileRemove(directory); err != nil {
		return fmt.Errorf(
			"failed to delete directory %v due to %v", directory, err)
	}

	log.Infof("%v %v uninstalled successfully", input.Name, input.Version)
	return nil
}

// Execute runs multiple sets of commands and returns their outputs.
// res.Output will contain a slice of RunCommandPluginOutput.
func (p *Plugin) Execute(context context.T, config contracts.Configuration, cancelFlag task.CancelFlag) (res contracts.PluginResult) {
	log := context.Log()
	log.Info("RunCommand started with configuration ", config)
	configureUtil := new(Utility)
	updateUtil := new(updateutil.Utility)
	manager := new(configureManager)

	res.StartDateTime = time.Now()
	defer func() { res.EndDateTime = time.Now() }()

	//loading Properties as list since aws:configureComponent uses properties as list
	var properties []interface{}
	if properties, res = pluginutil.LoadParametersAsList(log, config.Properties); res.Code != 0 {
		return res
	}

	out := make([]ConfigureComponentPluginOutput, len(properties))
	for i, prop := range properties {
		// check if a reboot has been requested
		if rebooter.RebootRequested() {
			log.Info("A plugin has requested a reboot.")
			break
		}

		if cancelFlag.ShutDown() {
			out[i] = ConfigureComponentPluginOutput{}
			out[i].Errors = []string{"Execution canceled due to ShutDown"}
			break
		} else if cancelFlag.Canceled() {
			out[i] = ConfigureComponentPluginOutput{}
			out[i].Errors = []string{"Execution canceled"}
			break
		}

		out[i] = configureComponent(p,
			log,
			manager,
			configureUtil,
			updateUtil,
			prop)

		res.Code = out[i].ExitCode
		res.Status = out[i].Status
		res.Output = fmt.Sprintf("%v", out[i].String())
	}

	return
}

// Name returns the name of the plugin.
func Name() string {
	return appconfig.PluginNameAwsConfigureComponent
}
