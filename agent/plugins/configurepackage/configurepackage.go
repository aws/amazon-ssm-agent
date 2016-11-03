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

// Package configurepackage implements the ConfigurePackage plugin.
package configurepackage

import (
	"errors"
	"fmt"
	"path"
	"path/filepath"
	"regexp"
	"time"

	"github.com/aws/amazon-ssm-agent/agent/appconfig"
	"github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/aws/amazon-ssm-agent/agent/contracts"
	"github.com/aws/amazon-ssm-agent/agent/executers"
	"github.com/aws/amazon-ssm-agent/agent/fileutil/artifact"
	"github.com/aws/amazon-ssm-agent/agent/framework/runpluginutil"
	"github.com/aws/amazon-ssm-agent/agent/jsonutil"
	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/amazon-ssm-agent/agent/plugins/pluginutil"
	"github.com/aws/amazon-ssm-agent/agent/rebooter"
	"github.com/aws/amazon-ssm-agent/agent/task"
	"github.com/aws/amazon-ssm-agent/agent/times"
	"github.com/aws/amazon-ssm-agent/agent/updateutil"
)

// Plugin is the type for the configurepackage plugin.
type Plugin struct {
	pluginutil.DefaultPlugin
	context          context.T
	runner           runpluginutil.PluginRunner
	orchestrationDir string
	s3Bucket         string
	s3Prefix         string
	messageID        string
	documentID       string
}

// ConfigurePackagePluginInput represents one set of commands executed by the ConfigurePackage plugin.
type ConfigurePackagePluginInput struct {
	contracts.PluginInput
	Name    string `json:"name"`
	Version string `json:"version"`
	Action  string `json:"action"`
	Source  string `json:"source"`
}

// ConfigurePackagePluginOutput represents the output of the plugin.
type ConfigurePackagePluginOutput struct {
	contracts.PluginOutput
}

// MarkAsSucceeded marks plugin as Successful.
func (result *ConfigurePackagePluginOutput) MarkAsSucceeded(reboot bool) {
	result.ExitCode = 0
	if reboot {
		result.Status = contracts.ResultStatusSuccessAndReboot
	} else {
		result.Status = contracts.ResultStatusSuccess
	}
}

// MarkAsFailed marks plugin as Failed.
func (result *ConfigurePackagePluginOutput) MarkAsFailed(log log.T, err error) {
	result.ExitCode = 1
	result.Status = contracts.ResultStatusFailed
	if result.Stderr != "" {
		result.Stderr = fmt.Sprintf("\n%v\n%v", result.Stderr, err.Error())
	} else {
		result.Stderr = fmt.Sprintf("\n%v", err.Error())
	}
	log.Error("failed to configure package", err.Error())
	result.Errors = append(result.Errors, err.Error())
}

// AppendInfo adds info to ConfigurePackagePluginOutput StandardOut.
func (result *ConfigurePackagePluginOutput) AppendInfo(log log.T, format string, params ...interface{}) {
	message := fmt.Sprintf(format, params...)
	log.Info(message)
	result.Stdout = fmt.Sprintf("%v\n%v", result.Stdout, message)
}

// AppendError adds errors to ConfigurePackagePluginOutput StandardErr.
func (result *ConfigurePackagePluginOutput) AppendError(log log.T, format string, params ...interface{}) {
	message := fmt.Sprintf(format, params...)
	log.Error(message)
	result.Stderr = fmt.Sprintf("%v\n%v", result.Stderr, message)
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
	downloadPackage(log log.T,
		util configureUtil,
		packageName string,
		version string,
		output *ConfigurePackagePluginOutput,
		context *updateutil.InstanceContext) (filePath string, err error)

	validateInput(input *ConfigurePackagePluginInput) (valid bool, err error)

	getVersionToInstall(log log.T,
		input *ConfigurePackagePluginInput,
		util configureUtil,
		context *updateutil.InstanceContext) (version string, installedVersion string, err error)

	getVersionToUninstall(log log.T,
		input *ConfigurePackagePluginInput,
		util configureUtil,
		context *updateutil.InstanceContext) (version string, err error)
}

// runConfigurePackage downloads the package and performs specified action
func runConfigurePackage(
	p *Plugin,
	log log.T,
	manager pluginHelper,
	util configureUtil,
	instanceContext *updateutil.InstanceContext,
	rawPluginInput interface{}) (output ConfigurePackagePluginOutput) {
	var input ConfigurePackagePluginInput
	var err error
	if err = jsonutil.Remarshal(rawPluginInput, &input); err != nil {
		output.MarkAsFailed(log, fmt.Errorf("invalid format in plugin properties %v; \nerror %v", rawPluginInput, err))
		return
	}

	if valid, err := manager.validateInput(&input); !valid {
		output.MarkAsFailed(log, fmt.Errorf("invalid input: %v", err))
		return
	}

	// do not allow multiple actions to be performed at the same time for the same package
	// this is possible with multiple concurrent runcommand documents
	if err := lockPackage(input.Name, input.Action); err != nil {
		output.MarkAsFailed(log, err)
		return
	}
	defer unlockPackage(input.Name)

	switch input.Action {
	case InstallAction:
		// get version information
		version, installedVersion, versionErr := manager.getVersionToInstall(log, &input, util, instanceContext)
		if versionErr != nil {
			output.MarkAsFailed(log, fmt.Errorf("unable to determine version to install: %v", versionErr))
			return
		}

		// if already installed, exit
		if version == installedVersion {
			// TODO:MF: validate that installed version is basically valid - has manifest and at least one other non-etag file or folder?
			output.AppendInfo(log, "%v %v is already installed", input.Name, version)
			output.MarkAsSucceeded(false)
			return
		}

		// ensure manifest file and package
		_, ensureErr := ensurePackage(log, manager, util, input.Name, version, &output, instanceContext)
		if ensureErr != nil {
			output.MarkAsFailed(log, fmt.Errorf("unable to obtain package: %v", ensureErr))
			return
		}

		// set installing flag for version
		if markErr := markInstallingPackage(input.Name, version); markErr != nil {
			output.MarkAsFailed(log, fmt.Errorf("unable to mark package installing: %v", markErr))
			return
		}

		// NOTE: do not return before clearing installing mark after this point unless you want it to remain set - once we defer the unmark it is OK to return again
		// if different version is installed, uninstall
		if installedVersion != "" {
			// NOTE: if source is specified on an install and we need to redownload the package for the
			// currently installed version because it isn't valid on disk, we will pull from the source URI
			// even though that may or may not be the package that installed it - it is our only decent option
			_, ensureErr := ensurePackage(log, manager, util, input.Name, installedVersion, &output, instanceContext)
			if ensureErr != nil {
				output.AppendError(log, "unable to obtain package: %v", ensureErr)
			} else {
				result, err := runUninstallPackagePre(p,
					input.Name,
					installedVersion,
					&output,
					log,
					instanceContext)
				if err != nil {
					output.AppendError(log, "failed to uninstall currently installed version of package: %v", err)
				} else {
					if result == contracts.ResultStatusSuccessAndReboot || result == contracts.ResultStatusPassedAndReboot {
						// Reboot before continuing
						output.MarkAsSucceeded(true)
						return
					}
				}
			}
		}

		// defer clearing installing
		defer unmarkInstallingPackage(input.Name)

		// install version
		result, err := runInstallPackage(p,
			input.Name,
			version,
			&output,
			manager,
			log,
			instanceContext)
		if err != nil {
			output.MarkAsFailed(log, fmt.Errorf("failed to install package: %v", err))
		} else if result == contracts.ResultStatusSuccessAndReboot || result == contracts.ResultStatusPassedAndReboot {
			output.AppendInfo(log, "Successfully installed %v %v", input.Name, version)
			output.MarkAsSucceeded(true)
		} else if result != contracts.ResultStatusSuccess {
			output.MarkAsFailed(log, fmt.Errorf("install action state was %v and not %v", result, contracts.ResultStatusSuccess))
		} else {
			output.AppendInfo(log, "Successfully installed %v %v", input.Name, version)
			output.MarkAsSucceeded(false)
		}

		// uninstall post action
		if installedVersion != "" {
			_, err := runUninstallPackagePost(p,
				input.Name,
				installedVersion,
				&output,
				log,
				instanceContext)
			if err != nil {
				output.AppendError(log, "failed to clean up currently installed version of package: %v", err)
			}
		}

	case UninstallAction:
		// get version information
		version, versionErr := manager.getVersionToUninstall(log, &input, util, instanceContext)
		if versionErr != nil || version == "" {
			output.MarkAsFailed(log, fmt.Errorf("unable to determine version to uninstall: %v", versionErr))
			return
		}

		// ensure manifest file and package
		_, ensureErr := ensurePackage(log, manager, util, input.Name, version, &output, instanceContext)
		if ensureErr != nil {
			output.MarkAsFailed(log, fmt.Errorf("unable to obtain package: %v", ensureErr))
			return
		}
		var resultPre, resultPost contracts.ResultStatus
		resultPre, err = runUninstallPackagePre(p,
			input.Name,
			version,
			&output,
			log,
			instanceContext)
		if err == nil {
			resultPost, err = runUninstallPackagePost(p,
				input.Name,
				version,
				&output,
				log,
				instanceContext)
		}
		result := contracts.MergeResultStatus(resultPre, resultPost)
		if err != nil {
			output.MarkAsFailed(log, fmt.Errorf("failed to uninstall package: %v", err))
		} else if result == contracts.ResultStatusSuccessAndReboot || result == contracts.ResultStatusPassedAndReboot {
			output.AppendInfo(log, "Successfully uninstalled %v %v", input.Name, version)
			output.MarkAsSucceeded(true)
		} else if result != contracts.ResultStatusSuccess {
			output.MarkAsFailed(log, fmt.Errorf("uninstall action state was %v and not %v", result, contracts.ResultStatusSuccess))
		} else {
			output.AppendInfo(log, "Successfully uninstalled %v %v", input.Name, version)
			output.MarkAsSucceeded(false)
		}
	default:
		output.MarkAsFailed(log, fmt.Errorf("unsupported action: %v", input.Action))
	}

	return
}

// ensurePackage validates local copy of the manifest and package and downloads if needed
func ensurePackage(log log.T,
	manager pluginHelper,
	util configureUtil,
	packageName string,
	version string,
	output *ConfigurePackagePluginOutput,
	context *updateutil.InstanceContext) (manifest *PackageManifest, err error) {

	// manifest to download
	manifestName := getManifestName(packageName)

	// path to local manifest
	localManifestName := filepath.Join(appconfig.PackageRoot, packageName, version, manifestName)

	// if we already have a valid manifest, return it
	if exist := filesysdep.Exists(localManifestName); exist {
		if manifest, err = parsePackageManifest(log, localManifestName); err == nil {
			// TODO:MF: consider verifying name, version, platform, arch in parsed manifest
			// TODO:MF: ensure the local package is valid before we return
			return
		}
	}

	// TODO:OFFLINE: if source but no version, download to temp, determine version from manifest and copy to correct location

	// download package
	var filePath string
	if filePath, err = manager.downloadPackage(log, util, packageName, version, output, context); err != nil {
		return
	}

	packageDestination := filepath.Join(appconfig.PackageRoot, packageName, version)
	if uncompressErr := filesysdep.Uncompress(filePath, packageDestination); uncompressErr != nil {
		err = fmt.Errorf("failed to extract package installer package %v from %v, %v", filePath, packageDestination, uncompressErr.Error())
		return
	}

	// NOTE: this could be considered a warning - it likely points to a real problem, but if uncompress succeeded, we could continue
	// delete compressed package after using
	if cleanupErr := filesysdep.RemoveAll(filePath); cleanupErr != nil {
		err = fmt.Errorf("failed to delete compressed package %v, %v", filePath, cleanupErr.Error())
		return
	}

	manifest, manifestErr := parsePackageManifest(log, localManifestName)
	if manifestErr != nil {
		err = fmt.Errorf("manifest is not valid for package %v, %v", filePath, manifestErr.Error())
		return
	}

	return manifest, nil
}

// validateInput ensures the plugin input matches the defined schema
func (m *configureManager) validateInput(input *ConfigurePackagePluginInput) (valid bool, err error) {
	// source not yet supported
	if input.Source != "" {
		return false, errors.New("source parameter is not supported in this version")
	}

	// ensure non-empty name
	if input.Name == "" {
		return false, errors.New("empty name field")
	}
	validNameValue := regexp.MustCompile(`^[a-zA-Z_]+([-.][a-zA-Z0-9_]+)*$`)
	if !validNameValue.MatchString(input.Name) {
		return false, errors.New("Invalid name, must start with letter or _; end with letter, number, or _; and contain only letters, numbers, -, _, or single . characters")
	}

	if version := input.Version; version != "" {
		// ensure version follows format <major>.<minor>.<build>
		if matched, err := regexp.MatchString(PatternVersion, version); matched == false || err != nil {
			return false, errors.New("invalid version - should be in format major.minor.build")
		}
	}

	return true, nil
}

// getVersionToInstall decides which version to install and whether there is an existing version (that is not in the process of installing)
func (m *configureManager) getVersionToInstall(log log.T,
	input *ConfigurePackagePluginInput,
	util configureUtil,
	context *updateutil.InstanceContext) (version string, installedVersion string, err error) {
	installedVersion = util.GetCurrentVersion(input.Name)

	if input.Version != "" {
		version = input.Version
	} else {
		if version, err = util.GetLatestVersion(log, input.Name, context); err != nil {
			return
		}
	}
	return version, installedVersion, nil
}

// getVersionToUninstall decides which version to uninstall
func (m *configureManager) getVersionToUninstall(log log.T,
	input *ConfigurePackagePluginInput,
	util configureUtil,
	context *updateutil.InstanceContext) (version string, err error) {
	if input.Version != "" {
		version = input.Version
	} else if installedVersion := util.GetCurrentVersion(input.Name); installedVersion != "" {
		version = installedVersion
	} else {
		version, err = util.GetLatestVersion(log, input.Name, context)
	}
	return
}

// downloadPackage downloads the installation package from s3 bucket or source URI and uncompresses it
func (m *configureManager) downloadPackage(log log.T,
	util configureUtil,
	packageName string,
	version string,
	output *ConfigurePackagePluginOutput,
	context *updateutil.InstanceContext) (filePath string, err error) {
	// package to download
	packageFilename := getPackageFilename(packageName, context)

	//TODO:OFFLINE: build packageLocation from source URI
	//   We should probably support both a URI to a "folder" that gets a filename tacked onto the end
	//   and a full path to a compressed package file

	// path to package
	packageLocation := getS3Location(packageName, version, context, packageFilename)

	// path to download destination
	packageDestination, createErr := util.CreatePackageFolder(packageName, version)
	if createErr != nil {
		return "", fmt.Errorf("failed to create local package repository, %v", createErr.Error())
	}

	downloadInput := artifact.DownloadInput{
		SourceURL:            packageLocation,
		DestinationDirectory: packageDestination}

	// download package
	downloadOutput, downloadErr := networkdep.Download(log, downloadInput)
	if downloadErr != nil || downloadOutput.LocalFilePath == "" {
		errMessage := fmt.Sprintf("failed to download installation package reliably, %v", downloadInput.SourceURL)
		if downloadErr != nil {
			errMessage = fmt.Sprintf("%v, %v", errMessage, downloadErr.Error())
		}
		// attempt to clean up failed download folder
		if errCleanup := filesysdep.RemoveAll(packageDestination); errCleanup != nil {
			log.Errorf("Failed to clean up destination folder %v after failed download: %v", packageDestination, errCleanup)
		}
		// return download error
		return "", errors.New(errMessage)
	}

	output.AppendInfo(log, "Successfully downloaded %v", downloadInput.SourceURL)

	return downloadOutput.LocalFilePath, nil
}

// runInstallPackage executes the install script for the specific version of a package.
func runInstallPackage(p *Plugin,
	packageName string,
	version string,
	output *ConfigurePackagePluginOutput,
	manager pluginHelper,
	log log.T,
	instanceContext *updateutil.InstanceContext,
) (status contracts.ResultStatus, err error) {
	status = contracts.ResultStatusSuccess

	directory := filepath.Join(appconfig.PackageRoot, packageName, version)
	if _, status, err = executeAction(p, "install", packageName, version, log, output, directory); err != nil {
		return status, err
	}
	return
}

// runUninstallPackagePre executes the uninstall script for the specific version of a package.
func runUninstallPackagePre(p *Plugin,
	packageName string,
	version string,
	output *ConfigurePackagePluginOutput,
	log log.T,
	context *updateutil.InstanceContext,
) (status contracts.ResultStatus, err error) {
	directory := filepath.Join(appconfig.PackageRoot, packageName, version)
	if _, status, err = executeAction(p, "uninstall", packageName, version, log, output, directory); err != nil {
		return status, err
	}
	return contracts.ResultStatusSuccess, nil
}

// runUninstallPackagePost performs post uninstall actions, like deleting the package folder
func runUninstallPackagePost(p *Plugin,
	packageName string,
	version string,
	output *ConfigurePackagePluginOutput,
	log log.T,
	context *updateutil.InstanceContext,
) (status contracts.ResultStatus, err error) {
	directory := filepath.Join(appconfig.PackageRoot, packageName, version)
	if err = filesysdep.RemoveAll(directory); err != nil {
		return contracts.ResultStatusFailed, fmt.Errorf("failed to delete directory %v due to %v", directory, err)
	}
	return contracts.ResultStatusSuccess, nil
}

// executeAction executes a command document as a sub-document of the current command and returns the result
func executeAction(p *Plugin,
	actionName string,
	packageName string,
	version string,
	log log.T,
	output *ConfigurePackagePluginOutput,
	executeDirectory string,
) (actionExists bool, status contracts.ResultStatus, err error) {
	status = contracts.ResultStatusSuccess
	err = nil
	fileName := fmt.Sprintf("%v.json", actionName)
	fileLocation := path.Join(executeDirectory, fileName)
	actionExists = filesysdep.Exists(fileLocation)

	if actionExists {
		log.Infof("Initiating %v %v %v", packageName, version, actionName)
		file, err := filesysdep.ReadFile(fileLocation)
		if err != nil {
			return true, contracts.ResultStatusFailed, err
		}
		pluginsInfo, err := execdep.ParseDocument(p, file, p.orchestrationDir, p.s3Bucket, p.s3Prefix, p.messageID, p.documentID, executeDirectory)
		if err != nil {
			return true, contracts.ResultStatusFailed, err
		}
		if len(pluginsInfo) == 0 {
			return true, contracts.ResultStatusFailed, fmt.Errorf("%v contained no work and may be malformed", fileName)
		}
		pluginOutputs := execdep.ExecuteDocument(p, pluginsInfo, p.documentID, times.ToIso8601UTC(time.Now()))
		if pluginOutputs == nil {
			return true, contracts.ResultStatusFailed, errors.New("No output from executing install document (install.json)")
		}
		for _, pluginOut := range pluginOutputs {
			log.Debugf("Plugin %v ResultStatus %v", pluginOut.PluginName, pluginOut.Status)
			if pluginOut.StandardOutput != "" {
				output.AppendInfo(log, "%v output: %v", actionName, pluginOut.StandardOutput)
			}
			if pluginOut.StandardError != "" {
				output.AppendInfo(log, "%v errors: %v", actionName, pluginOut.StandardError)
			}
			if pluginOut.Error != nil {
				output.Errors = append(output.Errors, pluginOut.Error.Error())
			}
			status = contracts.MergeResultStatus(status, pluginOut.Status)
		}
	}
	return
}

// getInstanceContext uses the updateUtil to return an instance context
func getInstanceContext(log log.T) (context *updateutil.InstanceContext, err error) {
	updateUtil := new(updateutil.Utility)
	return updateUtil.CreateInstanceContext(log)
}

var getContext = getInstanceContext
var runConfig = runConfigurePackage

// Execute runs multiple sets of commands and returns their outputs.
// res.Output will contain a slice of RunCommandPluginOutput.
func (p *Plugin) Execute(context context.T, config contracts.Configuration, cancelFlag task.CancelFlag, subDocumentRunner runpluginutil.PluginRunner) (res contracts.PluginResult) {
	p.context = context
	p.orchestrationDir = config.OrchestrationDirectory
	p.s3Bucket = config.OutputS3BucketName
	p.s3Prefix = config.OutputS3KeyPrefix
	p.messageID = config.MessageId
	p.documentID = config.BookKeepingFileName
	p.runner = subDocumentRunner
	log := context.Log()
	log.Info("RunCommand started with configuration ", config)
	util := new(configureUtilImp)
	manager := new(configureManager)

	res.StartDateTime = time.Now()
	defer func() { res.EndDateTime = time.Now() }()

	//loading Properties as list since aws:configurePackage uses properties as list
	var properties []interface{}
	if properties, res = pluginutil.LoadParametersAsList(log, config.Properties); res.Code != 0 {
		pluginutil.PersistPluginInformationToCurrent(log, Name(), config, res)
		return res
	}

	out := make([]ConfigurePackagePluginOutput, len(properties))
	for i, prop := range properties {
		// check if a reboot has been requested
		if rebooter.RebootRequested() {
			log.Info("A plugin has requested a reboot.")
			break
		}

		if cancelFlag.ShutDown() {
			out[i] = ConfigurePackagePluginOutput{}
			out[i].Errors = []string{"Execution canceled due to ShutDown"}
			break
		} else if cancelFlag.Canceled() {
			out[i] = ConfigurePackagePluginOutput{}
			out[i].Errors = []string{"Execution canceled"}
			break
		}

		instanceContext, err := getContext(log)
		if err != nil {
			out[i].MarkAsFailed(log,
				fmt.Errorf("unable to create instance context: %v", err))
			return
		}

		out[i] = runConfig(p,
			log,
			manager,
			util,
			instanceContext,
			prop)
		//TODO:MF: make sure this subdocument's HasExecuted == true even if it returned SuccessAndReboot - the parent document status will control whether it runs again after reboot
	}

	// TODO:MF: handle merging multiple results for cases where we had more than one operation
	// Currently assuming we have only one work.
	if len(properties) > 0 {
		res.Code = out[0].ExitCode
		res.Status = out[0].Status
		res.Output = out[0].String()
		res.StandardOutput = contracts.TruncateOutput(out[0].Stdout, "", 24000)
		res.StandardError = contracts.TruncateOutput(out[0].Stderr, "", 8000)
	}

	pluginutil.PersistPluginInformationToCurrent(log, Name(), config, res)

	return res
}

// Name returns the name of the plugin.
func Name() string {
	return appconfig.PluginNameAwsConfigurePackage
}
