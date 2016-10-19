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
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/aws/amazon-ssm-agent/agent/appconfig"
	"github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/aws/amazon-ssm-agent/agent/contracts"
	"github.com/aws/amazon-ssm-agent/agent/executers"
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
	Name    string `json:"name"`
	Version string `json:"version"`
	Action  string `json:"action"`
	Source  string `json:"source"`
}

// ConfigureComponentsPluginOutput represents the output of the plugin.
type ConfigureComponentPluginOutput struct {
	contracts.PluginOutput
}

// MarkAsSucceeded marks plugin as Successful.
func (result *ConfigureComponentPluginOutput) MarkAsSucceeded(reboot bool) {
	result.ExitCode = 0
	if reboot {
		result.Status = contracts.ResultStatusSuccessAndReboot
	}
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
		componentName string,
		version string,
		source string,
		output *ConfigureComponentPluginOutput,
		context *updateutil.InstanceContext) (manifest *ComponentManifest, err error)

	downloadPackage(log log.T,
		util Util,
		componentName string,
		version string,
		source string,
		output *ConfigureComponentPluginOutput,
		context *updateutil.InstanceContext) (filePath string, err error)

	validateInput(input *ConfigureComponentPluginInput) (valid bool, err error)

	getVersionToInstall(log log.T,
		input *ConfigureComponentPluginInput,
		util Util,
		context *updateutil.InstanceContext) (version string, installedVersion string, err error)

	getVersionToUninstall(log log.T,
		input *ConfigureComponentPluginInput,
		util Util,
		context *updateutil.InstanceContext) (version string, err error)
}

// TO DO: How to mock reboot?
// var reboot = rebooter.RequestPendingReboot

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

	if valid, err := manager.validateInput(&input); !valid {
		output.MarkAsFailed(log,
			fmt.Errorf("invalid input: %v", err))
		return
	}

	if context, err = updateUtil.CreateInstanceContext(log); err != nil {
		output.MarkAsFailed(log,
			fmt.Errorf("unable to create instance context: %v", err))
		return
	}
	// do not allow multiple actions to be performed at the same time for the same component
	// this is possible with multiple concurrent runcommand documents
	if err := lockComponent(input.Name, input.Action); err != nil {
		output.MarkAsFailed(log, err)
		return
	}
	defer unlockComponent(input.Name)

	switch input.Action {
	case InstallAction:
		// get version information
		version, installedVersion, versionErr := manager.getVersionToInstall(log, &input, configureUtil, context)
		if versionErr != nil {
			output.MarkAsFailed(log,
				fmt.Errorf("unable to determine version to install: %v", versionErr))
			return
		}

		// if already installed, exit
		if version == installedVersion {
			// TODO:MF: validate that installed version is basically valid - has manifest and at least one other file/folder?
			output.AppendInfo(log, "%v %v is already installed", input.Name, version)
			output.MarkAsSucceeded(false)
			return
		}

		// ensure manifest file and package
		manifest, ensureErr := ensurePackage(log, manager, configureUtil, input.Name, version, input.Source, &output, context)
		if ensureErr != nil {
			output.MarkAsFailed(log,
				fmt.Errorf("unable to obtain package: %v", ensureErr))
			return
		}

		// TODO:MF: set installing flag for version

		// NOTE: do not return before clearing installing flag after this point unless you want it to remain set
		// if different version is installed, uninstall
		if installedVersion != "" {
			// NOTE: if source is specified on an install and we need to redownload the package for the
			// currently installed version because it isn't valid on disk, we will pull from the source URI
			// even though that may or may not be the package that installed it - it is our only decent option
			uninstallManifest, ensureErr := ensurePackage(log, manager, configureUtil, input.Name, installedVersion, input.Source, &output, context)
			if ensureErr != nil {
				output.MarkAsFailed(log,
					fmt.Errorf("unable to obtain package: %v", ensureErr))
			} else {
				if err = runUninstallComponent(p,
					input.Name,
					installedVersion,
					input.Source,
					&output,
					log,
					uninstallManifest.Uninstall,
					configureUtil,
					updateUtil,
					context); err != nil {
					output.MarkAsFailed(log,
						fmt.Errorf("failed to uninstall currently installed version of component: %v", err))
				} else {
					if uninstallManifest.Reboot == "true" {
						// TODO:MF: set reboot flag and return without success or failure
					}
				}
			}
		}

		// TODO:MF: defer clearing installing

		// exit if we're in an error state
		if output.ExitCode != 0 {
			return
		}

		// install version
		if err = runInstallComponent(p,
			input.Name,
			version,
			input.Source,
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
		output.MarkAsSucceeded(manifest.Reboot == "true")
		// TO DO: How to mock reboot?
		// reboot according to manifest
		//if manifest.Reboot == RebootTrue {
		//	reboot()
		//}

	case UninstallAction:
		// get version information
		version, versionErr := manager.getVersionToUninstall(log, &input, configureUtil, context)
		if versionErr != nil {
			output.MarkAsFailed(log,
				fmt.Errorf("unable to determine version to uninstall: %v", versionErr))
			return
		}

		// ensure manifest file and package
		manifest, ensureErr := ensurePackage(log, manager, configureUtil, input.Name, version, input.Source, &output, context)
		if ensureErr != nil {
			output.MarkAsFailed(log,
				fmt.Errorf("unable to obtain package: %v", ensureErr))
			return
		}
		if err = runUninstallComponent(p,
			input.Name,
			version,
			input.Source,
			&output,
			log,
			manifest.Uninstall,
			configureUtil,
			updateUtil,
			context); err != nil {
			output.MarkAsFailed(log,
				fmt.Errorf("failed to uninstall component: %v", err))
			return
		}
		output.MarkAsSucceeded(manifest.Reboot == "true")
	default:
		output.MarkAsFailed(log,
			fmt.Errorf("unsupported action: %v", input.Action))
	}

	return
}

// ensurePackage validates local copy of the manifest and package and downloads if needed
func ensurePackage(log log.T,
	manager pluginHelper,
	util Util,
	componentName string,
	version string,
	source string,
	output *ConfigureComponentPluginOutput,
	context *updateutil.InstanceContext) (manifest *ComponentManifest, err error) {

	// manifest to download
	manifestName := getManifestName(componentName)

	// path to local manifest
	localManifestName := filepath.Join(appconfig.ComponentRoot, componentName, version, manifestName)

	// if we already have a valid manifest, return it
	if exist := filesysdep.Exists(localManifestName); exist {
		if manifest, err = parseComponentManifest(log, localManifestName); err == nil {
			// TODO:MF: consider verifying name and version in parsed manifest
			// TODO:MF: ensure the local package is valid before we return
			return
		} else {
			// TODO:MF: delete or rename invalid manifest
		}
	}
	// otherwise download it
	if manifest == nil {
		if manifest, err = manager.downloadManifest(log, util, componentName, version, source, output, context); err != nil {
			return
		}
	}
	// download package
	var filePath string
	if filePath, err = manager.downloadPackage(log, util, componentName, version, source, output, context); err != nil {
		return
	}

	packageDestination := filepath.Join(appconfig.ComponentRoot, componentName, version)
	if uncompressErr := filesysdep.Uncompress(filePath, packageDestination); uncompressErr != nil {
		err = fmt.Errorf("failed to extract component installer package %v from %v, %v", filePath, packageDestination, uncompressErr.Error())
		return
	}

	// TODO:MF: this could be considered a warning - it likely points to a real problem, but if uncompress succeeded, we could continue
	// delete compressed package after using
	if cleanupErr := filesysdep.RemoveAll(filePath); cleanupErr != nil {
		err = fmt.Errorf("failed to delete compressed package %v, %v", filePath, cleanupErr.Error())
		return
	}

	return manifest, nil
}

// validateInput ensures the plugin input matches the defined schema
func (m *configureManager) validateInput(input *ConfigureComponentPluginInput) (valid bool, err error) {
	// ensure non-empty name
	if input.Name == "" {
		return false, errors.New("empty name field")
	}

	// version not needed for uninstall
	if input.Action == UninstallAction && input.Version == "" {
		return true, nil
	}

	if version := input.Version; version != "" {
		// ensure version follows format <major>.<minor>.<build>
		if matched, err := regexp.MatchString(PatternVersion, version); matched == false || err != nil {
			return false,
				errors.New("invalid version - should be in format major.minor.build")
		}
	}

	return true, nil
}

// getVersionToInstall decides which version to install and whether there is an existing version (that is not in the process of installing)
func (m *configureManager) getVersionToInstall(log log.T,
	input *ConfigureComponentPluginInput,
	util Util,
	context *updateutil.InstanceContext) (version string, installedVersion string, err error) {
	installedVersion = util.GetCurrentVersion(input.Name)

	if input.Version != "" {
		version = input.Version
	} else {
		if version, err = util.GetLatestVersion(log, input.Name, input.Source, context); err != nil {
			return
		}
	}
	return version, installedVersion, nil
}

// getVersionToUninstall decides which version to uninstall
func (m *configureManager) getVersionToUninstall(log log.T,
	input *ConfigureComponentPluginInput,
	util Util,
	context *updateutil.InstanceContext) (version string, err error) {
	if input.Version != "" {
		version = input.Version
	} else if installedVersion := util.GetCurrentVersion(input.Name); installedVersion != "" {
		version = installedVersion
	} else {
		version, err = util.GetLatestVersion(log, input.Name, input.Source, context)
	}
	return
}

// downloadManifest downloads component configuration file from s3 bucket.
func (m *configureManager) downloadManifest(log log.T,
	util Util,
	componentName string,
	version string,
	source string,
	output *ConfigureComponentPluginOutput,
	context *updateutil.InstanceContext) (manifest *ComponentManifest, err error) {
	// manifest to download
	manifestName := getManifestName(componentName)

	// path to local manifest
	localManifestName := filepath.Join(appconfig.ComponentRoot, componentName, version, manifestName)

	// path to manifest
	manifestLocation := source
	if manifestLocation == "" {
		manifestLocation = getS3Location(componentName, version, context, manifestName)
	} else {
		//TODO:MF: I don't think source will contain a replaceable placeholder - I think it is a URI to a "folder" that gets a filename tacked onto the end
		manifestLocation = strings.Replace(manifestLocation, updateutil.FileNameHolder, manifestName, -1)
	}

	// path to download destination
	manifestDestination, err := util.CreateComponentFolder(componentName, version)
	if err != nil {
		errMessage := fmt.Sprintf("failed to create local component repository, %v", err.Error())
		return nil, errors.New(errMessage)
	}

	downloadInput := artifact.DownloadInput{
		SourceURL:            manifestLocation,
		DestinationDirectory: manifestDestination}

	// download manifest
	downloadOutput, downloadErr := networkdep.Download(log, downloadInput)
	if downloadErr != nil || downloadOutput.LocalFilePath == "" {
		errMessage := fmt.Sprintf("failed to download component manifest reliably, %v", downloadInput.SourceURL)
		if downloadErr != nil {
			errMessage = fmt.Sprintf("%v, %v", errMessage, downloadErr.Error())
		}
		return nil, errors.New(errMessage)
	}

	// rename manifest
	if err = filesysdep.Rename(downloadOutput.LocalFilePath, localManifestName); err != nil {
		errMessage := fmt.Sprintf("failed to rename %v to %v: %v", downloadOutput.LocalFilePath, localManifestName, err.Error())
		return nil, errors.New(errMessage)
	}

	output.AppendInfo(log, "Successfully downloaded %v", downloadInput.SourceURL)

	return parseComponentManifest(log, localManifestName)
}

// downloadPackage downloads the installation package from s3 bucket or source URI and uncompresses it
func (m *configureManager) downloadPackage(log log.T,
	util Util,
	componentName string,
	version string,
	source string,
	output *ConfigureComponentPluginOutput,
	context *updateutil.InstanceContext) (filePath string, err error) {
	// package to download
	packageName := getPackageName(componentName, context)

	// path to package
	packageLocation := source
	if packageLocation == "" {
		packageLocation = getS3Location(componentName, version, context, packageName)
	} else {
		//TODO:MF: I don't think source will contain a replaceable placeholder - I think it is a URI to a "folder" that gets a filename tacked onto the end
		packageLocation = strings.Replace(packageLocation, updateutil.FileNameHolder, packageName, -1)
	}

	// path to download destination
	packageDestination, err := util.CreateComponentFolder(componentName, version)
	if err != nil {
		errMessage := fmt.Sprintf("failed to create local component repository, %v", err.Error())
		return "", errors.New(errMessage)
	}

	downloadInput := artifact.DownloadInput{
		SourceURL:            packageLocation,
		DestinationDirectory: packageDestination}

	// download package
	downloadOutput, downloadErr := networkdep.Download(log, downloadInput)
	if downloadErr != nil || downloadOutput.LocalFilePath == "" {
		errMessage := fmt.Sprintf("failed to download component installation package reliably, %v", downloadInput.SourceURL)
		if downloadErr != nil {
			errMessage = fmt.Sprintf("%v, %v", errMessage, downloadErr.Error())
		}
		return "", errors.New(errMessage)
	}

	output.AppendInfo(log, "Successfully downloaded %v", downloadInput.SourceURL)

	return downloadOutput.LocalFilePath, nil
}

// runInstallComponent executes the install script for the specific version of component.
func runInstallComponent(p *Plugin,
	componentName string,
	version string,
	source string,
	output *ConfigureComponentPluginOutput,
	manager pluginHelper,
	log log.T,
	installCmd string,
	configureUtil Util,
	updateUtil updateutil.T,
	context *updateutil.InstanceContext) (err error) {
	log.Infof("Initiating %v %v installation", componentName, version)

	directory := filepath.Join(appconfig.ComponentRoot, componentName, version)

	// execute installation command
	if err = updateUtil.ExeCommand(
		log,
		installCmd,
		directory, directory,
		p.StdoutFileName, p.StderrFileName,
		false); err != nil {

		return err
	}

	output.AppendInfo(log, "Successfully installed %v %v", componentName, version)
	return nil
}

// runUninstallComponent executes the install script for the specific version of component.
func runUninstallComponent(p *Plugin,
	componentName string,
	version string,
	source string,
	output *ConfigureComponentPluginOutput,
	log log.T,
	uninstallCmd string,
	configureUtil Util,
	util updateutil.T,
	context *updateutil.InstanceContext) (err error) {
	log.Infof("Initiating %v %v uninstallation", componentName, version)

	directory := filepath.Join(appconfig.ComponentRoot, componentName, version)

	// execute installation command
	if err = util.ExeCommand(
		log,
		uninstallCmd,
		directory, directory,
		p.StdoutFileName, p.StderrFileName,
		false); err != nil {

		return err
	}

	// delete local component folder for this version
	if err = filesysdep.RemoveAll(directory); err != nil {
		return fmt.Errorf(
			"failed to delete directory %v due to %v", directory, err)
	}

	output.AppendInfo(log, "Successfully uninstalled %v %v", componentName, version)
	return nil
}

var runConfig = runConfigureComponent

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

		out[i] = runConfig(p,
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
