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
}

// ConfigurePackagePluginInput represents one set of commands executed by the ConfigurePackage plugin.
type ConfigurePackagePluginInput struct {
	contracts.PluginInput
	Name       string `json:"name"`
	Version    string `json:"version"`
	Action     string `json:"action"`
	Source     string `json:"source"`
	Repository string `json:"repository"`
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

type configurePackage struct {
	contracts.Configuration
	runner runpluginutil.PluginRunner
}

type configurePackageManager interface {
	downloadPackage(context context.T,
		util configureUtil,
		packageName string,
		version string,
		output *contracts.PluginOutput) (filePath string, err error)

	validateInput(context context.T, input *ConfigurePackagePluginInput) (valid bool, err error)

	getVersionToInstall(context context.T, input *ConfigurePackagePluginInput, util configureUtil) (version string, installedVersion string, err error)

	getVersionToUninstall(context context.T, input *ConfigurePackagePluginInput, util configureUtil) (version string, err error)

	setMark(context context.T, packageName string, version string) error

	clearMark(context context.T, packageName string)

	ensurePackage(context context.T,
		util configureUtil,
		packageName string,
		version string,
		output *contracts.PluginOutput) (manifest *PackageManifest, err error)

	runUninstallPackagePre(context context.T,
		packageName string,
		version string,
		output *contracts.PluginOutput) (status contracts.ResultStatus, err error)

	runInstallPackage(context context.T,
		packageName string,
		version string,
		output *contracts.PluginOutput) (status contracts.ResultStatus, err error)

	runUninstallPackagePost(context context.T,
		packageName string,
		version string,
		output *contracts.PluginOutput) (status contracts.ResultStatus, err error)
}

// runConfigurePackage downloads the package and performs specified action
func runConfigurePackage(
	p *Plugin,
	context context.T,
	manager configurePackageManager,
	instanceContext *updateutil.InstanceContext,
	rawPluginInput interface{}) (output contracts.PluginOutput) {
	log := context.Log()

	var input ConfigurePackagePluginInput
	var err error
	if err = jsonutil.Remarshal(rawPluginInput, &input); err != nil {
		output.MarkAsFailed(log, fmt.Errorf("invalid format in plugin properties %v; \nerror %v", rawPluginInput, err))
		return
	}

	if valid, err := manager.validateInput(context, &input); !valid {
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

	configUtil := NewUtil(instanceContext, input.Repository)

	switch input.Action {
	case InstallAction:
		// get version information
		version, installedVersion, versionErr := manager.getVersionToInstall(context, &input, configUtil)
		if versionErr != nil {
			output.MarkAsFailed(log, fmt.Errorf("unable to determine version to install: %v", versionErr))
			return
		}

		// if already installed, exit
		if version == installedVersion {
			// TODO:MF: validate that installed version is basically valid - has manifest and at least one other non-etag file or folder?
			output.AppendInfof(log, "%v %v is already installed", input.Name, version)
			output.MarkAsSucceeded()
			return
		}

		// ensure manifest file and package
		_, ensureErr := manager.ensurePackage(context, configUtil, input.Name, version, &output)
		if ensureErr != nil {
			output.MarkAsFailed(log, fmt.Errorf("unable to obtain package: %v", ensureErr))
			return
		}

		// set installing flag for version
		if markErr := manager.setMark(context, input.Name, version); markErr != nil {
			output.MarkAsFailed(log, fmt.Errorf("unable to mark package installing: %v", markErr))
			return
		}

		// NOTE: do not return before clearing installing mark after this point unless you want it to remain set - once we defer the unmark it is OK to return again
		// if different version is installed, uninstall
		if installedVersion != "" {
			// NOTE: if source is specified on an install and we need to redownload the package for the
			// currently installed version because it isn't valid on disk, we will pull from the source URI
			// even though that may or may not be the package that installed it - it is our only decent option
			_, ensureErr := manager.ensurePackage(context, configUtil, input.Name, installedVersion, &output)
			if ensureErr != nil {
				output.AppendErrorf(log, "unable to obtain package: %v", ensureErr)
			} else {
				result, err := manager.runUninstallPackagePre(context,
					input.Name,
					installedVersion,
					&output)
				if err != nil {
					output.AppendErrorf(log, "failed to uninstall currently installed version of package: %v", err)
				} else {
					if result == contracts.ResultStatusSuccessAndReboot || result == contracts.ResultStatusPassedAndReboot {
						// Reboot before continuing
						output.MarkAsSuccessWithReboot()
						return
					}
				}
			}
		}

		// defer clearing installing
		defer manager.clearMark(context, input.Name)

		// install version
		result, err := manager.runInstallPackage(context,
			input.Name,
			version,
			&output)
		if err != nil {
			output.MarkAsFailed(log, fmt.Errorf("failed to install package: %v", err))
		} else if result == contracts.ResultStatusSuccessAndReboot || result == contracts.ResultStatusPassedAndReboot {
			output.AppendInfof(log, "Successfully installed %v %v", input.Name, version)
			output.MarkAsSuccessWithReboot()
		} else if result != contracts.ResultStatusSuccess {
			output.MarkAsFailed(log, fmt.Errorf("install action state was %v and not %v", result, contracts.ResultStatusSuccess))
		} else {
			output.AppendInfof(log, "Successfully installed %v %v", input.Name, version)
			output.MarkAsSucceeded()
		}

		// uninstall post action
		if installedVersion != "" {
			_, err := manager.runUninstallPackagePost(context,
				input.Name,
				installedVersion,
				&output)
			if err != nil {
				output.AppendErrorf(log, "failed to clean up currently installed version of package: %v", err)
			}
		}

	case UninstallAction:
		// get version information
		version, versionErr := manager.getVersionToUninstall(context, &input, configUtil)
		if versionErr != nil || version == "" {
			output.MarkAsFailed(log, fmt.Errorf("unable to determine version to uninstall: %v", versionErr))
			return
		}

		// ensure manifest file and package
		_, ensureErr := manager.ensurePackage(context, configUtil, input.Name, version, &output)
		if ensureErr != nil {
			output.MarkAsFailed(log, fmt.Errorf("unable to obtain package: %v", ensureErr))
			return
		}
		var resultPre, resultPost contracts.ResultStatus
		resultPre, err = manager.runUninstallPackagePre(context,
			input.Name,
			version,
			&output)
		if err != nil {
			output.MarkAsFailed(log, fmt.Errorf("failed to uninstall package: %v", err))
			return
		}
		resultPost, err = manager.runUninstallPackagePost(context,
			input.Name,
			version,
			&output)
		if err != nil {
			output.MarkAsFailed(log, fmt.Errorf("failed to uninstall package: %v", err))
			return
		}

		result := contracts.MergeResultStatus(resultPre, resultPost)
		if result == contracts.ResultStatusSuccessAndReboot || result == contracts.ResultStatusPassedAndReboot {
			output.AppendInfof(log, "Successfully uninstalled %v %v", input.Name, version)
			output.MarkAsSuccessWithReboot()
		} else if result != contracts.ResultStatusSuccess {
			output.MarkAsFailed(log, fmt.Errorf("uninstall action state was %v and not %v", result, contracts.ResultStatusSuccess))
		} else {
			output.AppendInfof(log, "Successfully uninstalled %v %v", input.Name, version)
			output.MarkAsSucceeded()
		}
	default:
		output.MarkAsFailed(log, fmt.Errorf("unsupported action: %v", input.Action))
	}

	return
}

// ensurePackage validates local copy of the manifest and package and downloads if needed
func (m *configurePackage) ensurePackage(context context.T,
	util configureUtil,
	packageName string,
	version string,
	output *contracts.PluginOutput) (manifest *PackageManifest, err error) {

	// manifest to download
	manifestName := getManifestName(packageName)

	// path to local manifest
	localManifestName := filepath.Join(appconfig.PackageRoot, packageName, version, manifestName)

	// if we already have a valid manifest, return it
	if exist := filesysdep.Exists(localManifestName); exist {
		if manifest, err = parsePackageManifest(context.Log(), localManifestName); err == nil {
			// TODO:MF: consider verifying name, version, platform, arch in parsed manifest
			// TODO:MF: ensure the local package is valid before we return
			return
		}
	}

	// TODO:OFFLINE: if source but no version, download to temp, determine version from manifest and copy to correct location

	// download package
	var filePath string
	if filePath, err = m.downloadPackage(context, util, packageName, version, output); err != nil {
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

	manifest, manifestErr := parsePackageManifest(context.Log(), localManifestName)
	if manifestErr != nil {
		err = fmt.Errorf("manifest is not valid for package %v, %v", filePath, manifestErr.Error())
		return
	}

	return manifest, nil
}

// validateInput ensures the plugin input matches the defined schema
func (m *configurePackage) validateInput(context context.T, input *ConfigurePackagePluginInput) (valid bool, err error) {
	// source not yet supported
	if input.Source != "" {
		return false, errors.New("source parameter is not supported in this version")
	}

	// ensure non-empty name
	if input.Name == "" {
		return false, errors.New("empty name field")
	}
	validNameValue := regexp.MustCompile(`^[a-zA-Z_]+(([-.])?[a-zA-Z0-9_]+)*$`)
	if !validNameValue.MatchString(input.Name) {
		return false, errors.New("invalid name, must start with letter or _; end with letter, number, or _; and contain only letters, numbers, -, _, or single . characters")
	}

	if version := input.Version; version != "" {
		// ensure version follows format <major>.<minor>.<build>
		if matched, err := regexp.MatchString(PatternVersion, version); matched == false || err != nil {
			return false, errors.New("invalid version - should be in format major.minor.build")
		}
	}

	// dump any unsupported value for Repository
	if input.Repository != "beta" && input.Repository != "gamma" {
		input.Repository = ""
	}

	return true, nil
}

// getVersionToInstall decides which version to install and whether there is an existing version (that is not in the process of installing)
func (m *configurePackage) getVersionToInstall(context context.T,
	input *ConfigurePackagePluginInput,
	util configureUtil) (version string, installedVersion string, err error) {
	installedVersion = util.GetCurrentVersion(input.Name)

	if input.Version != "" {
		version = input.Version
	} else {
		if version, err = util.GetLatestVersion(context.Log(), input.Name); err != nil {
			return
		}
	}
	return version, installedVersion, nil
}

// getVersionToUninstall decides which version to uninstall
func (m *configurePackage) getVersionToUninstall(context context.T,
	input *ConfigurePackagePluginInput,
	util configureUtil) (version string, err error) {
	if input.Version != "" {
		version = input.Version
	} else if installedVersion := util.GetCurrentVersion(input.Name); installedVersion != "" {
		version = installedVersion
	} else {
		version, err = util.GetLatestVersion(context.Log(), input.Name)
	}
	return
}

// setMark marks a particular version as installing so that if we uninstall and reboot we will know
// to continue with the install even though the package is already present
func (configurePackage) setMark(context context.T, packageName string, version string) error {
	return markInstallingPackage(packageName, version)
}

// clearMark removes the file marking a package as being in the process of installation
func (configurePackage) clearMark(context context.T, packageName string) {
	unmarkInstallingPackage(packageName)
}

// downloadPackage downloads the installation package from s3 bucket or source URI and uncompresses it
func (m *configurePackage) downloadPackage(context context.T,
	util configureUtil,
	packageName string,
	version string,
	output *contracts.PluginOutput) (filePath string, err error) {

	log := context.Log()
	//TODO:OFFLINE: build packageLocation from source URI
	//   We should probably support both a URI to a "folder" that gets a filename tacked onto the end
	//   and a full path to a compressed package file

	// path to package
	packageLocation := util.GetS3Location(packageName, version)

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

	output.AppendInfof(log, "Successfully downloaded %v", downloadInput.SourceURL)

	return downloadOutput.LocalFilePath, nil
}

// runInstallPackage executes the install script for the specific version of a package.
func (m *configurePackage) runInstallPackage(context context.T,
	packageName string,
	version string,
	output *contracts.PluginOutput) (status contracts.ResultStatus, err error) {
	status = contracts.ResultStatusSuccess

	directory := filepath.Join(appconfig.PackageRoot, packageName, version)
	if _, status, err = m.executeAction(context, "install", packageName, version, output, directory); err != nil {
		return status, err
	}
	return
}

// runUninstallPackagePre executes the uninstall script for the specific version of a package.
func (m *configurePackage) runUninstallPackagePre(context context.T,
	packageName string,
	version string,
	output *contracts.PluginOutput) (status contracts.ResultStatus, err error) {
	directory := filepath.Join(appconfig.PackageRoot, packageName, version)
	if _, status, err = m.executeAction(context, "uninstall", packageName, version, output, directory); err != nil {
		return status, err
	}
	return contracts.ResultStatusSuccess, nil
}

// runUninstallPackagePost performs post uninstall actions, like deleting the package folder
func (m *configurePackage) runUninstallPackagePost(context context.T,
	packageName string,
	version string,
	output *contracts.PluginOutput) (status contracts.ResultStatus, err error) {
	directory := filepath.Join(appconfig.PackageRoot, packageName, version)
	if err = filesysdep.RemoveAll(directory); err != nil {
		return contracts.ResultStatusFailed, fmt.Errorf("failed to delete directory %v due to %v", directory, err)
	}
	return contracts.ResultStatusSuccess, nil
}

// executeAction executes a command document as a sub-document of the current command and returns the result
func (m *configurePackage) executeAction(context context.T,
	actionName string,
	packageName string,
	version string,
	output *contracts.PluginOutput,
	executeDirectory string) (actionExists bool, status contracts.ResultStatus, err error) {
	status = contracts.ResultStatusSuccess
	err = nil
	fileName := fmt.Sprintf("%v.json", actionName)
	fileLocation := path.Join(executeDirectory, fileName)
	actionExists = filesysdep.Exists(fileLocation)

	log := context.Log()
	if actionExists {
		output.AppendInfof(log, "Initiating %v %v %v", packageName, version, actionName)
		file, err := filesysdep.ReadFile(fileLocation)
		if err != nil {
			return true, contracts.ResultStatusFailed, err
		}
		pluginsInfo, err := execdep.ParseDocument(context, file, m.OrchestrationDirectory, m.OutputS3BucketName, m.OutputS3KeyPrefix, m.MessageId, m.BookKeepingFileName, executeDirectory)
		if err != nil {
			return true, contracts.ResultStatusFailed, err
		}
		if len(pluginsInfo) == 0 {
			return true, contracts.ResultStatusFailed, fmt.Errorf("%v contained no work and may be malformed", fileName)
		}
		pluginOutputs := execdep.ExecuteDocument(m.runner, context, pluginsInfo, m.BookKeepingFileName, times.ToIso8601UTC(time.Now()))
		if pluginOutputs == nil {
			return true, contracts.ResultStatusFailed, errors.New("No output from executing install document (install.json)")
		}
		for _, pluginOut := range pluginOutputs {
			log.Debugf("Plugin %v ResultStatus %v", pluginOut.PluginName, pluginOut.Status)
			if pluginOut.StandardOutput != "" {
				output.AppendInfof(log, "%v output: %v", actionName, pluginOut.StandardOutput)
			}
			if pluginOut.StandardError != "" {
				output.AppendInfof(log, "%v errors: %v", actionName, pluginOut.StandardError)
			}
			if pluginOut.Error != nil {
				output.MarkAsFailed(log, pluginOut.Error)
			}
			status = contracts.MergeResultStatus(status, pluginOut.Status)

			//TODO:MF: make sure this subdocument's HasExecuted == true even if it returned SuccessAndReboot - the parent document status will control whether it runs again after reboot
		}
	}
	return
}

// getInstanceContext uses the updateUtil to return an instance context
func getInstanceContext(log log.T) (instanceContext *updateutil.InstanceContext, err error) {
	updateUtil := new(updateutil.Utility)
	return updateUtil.CreateInstanceContext(log)
}

var getContext = getInstanceContext
var runConfig = runConfigurePackage

// Execute runs multiple sets of commands and returns their outputs.
// res.Output will contain a slice of RunCommandPluginOutput.
func (p *Plugin) Execute(context context.T, config contracts.Configuration, cancelFlag task.CancelFlag, subDocumentRunner runpluginutil.PluginRunner) (res contracts.PluginResult) {
	log := context.Log()
	log.Info("RunCommand started with configuration ", config)

	res.StartDateTime = time.Now()
	defer func() { res.EndDateTime = time.Now() }()

	//loading Properties as list since V1.2 schema uses properties as list - if we do get a list we will execute all of them
	//TODO:MF: Consider handling this in conversion from 1.2 to the standard format by expanding multiple sets of properties into multiple plugins
	var properties []interface{}
	if properties, res = pluginutil.LoadParametersAsList(log, config.Properties); res.Code != 0 {
		pluginutil.PersistPluginInformationToCurrent(log, config.PluginID, config, res)
		return res
	}

	out := make([]contracts.PluginOutput, len(properties))

	instanceContext, err := getContext(log)
	if err != nil {
		for _, output := range out {
			output.MarkAsFailed(log,
				fmt.Errorf("unable to create instance context: %v", err))
		}
		return
	}
	manager := &configurePackage{Configuration: config, runner: subDocumentRunner}

	for i, prop := range properties {
		// check if a reboot has been requested
		if rebooter.RebootRequested() {
			log.Info("A plugin has requested a reboot.")
			break
		}

		if cancelFlag.ShutDown() {
			res.Code = 1
			res.Status = contracts.ResultStatusFailed
			break
		} else if cancelFlag.Canceled() {
			res.Code = 1
			res.Status = contracts.ResultStatusCancelled
			break
		}

		out[i] = runConfig(p,
			context,
			manager,
			instanceContext,
			prop)
	}

	if len(out) > 0 {
		// Input is a list for V1.2 schema but we only return results for the first one
		res.Code = out[0].ExitCode
		res.Status = out[0].Status
		res.Output = out[0].String()
		if config.OrchestrationDirectory != "" {
			useTemp := false
			outFile := filepath.Join(config.OrchestrationDirectory, p.StdoutFileName)
			// create orchestration dir if needed
			if err := filesysdep.MakeDirExecute(config.OrchestrationDirectory); err != nil {
				out[0].AppendError(log, "Failed to create orchestrationDir directory for log files")
			} else {
				if err := filesysdep.WriteFile(outFile, out[0].Stdout); err != nil {
					log.Debugf("Error writing to %v", outFile)
					out[0].AppendErrorf(log, "Error saving stdout: %v", err.Error())
				}
				errFile := filepath.Join(config.OrchestrationDirectory, p.StderrFileName)
				if err := filesysdep.WriteFile(errFile, out[0].Stderr); err != nil {
					log.Debugf("Error writing to %v", errFile)
					out[0].AppendErrorf(log, "Error saving stderr: %v", err.Error())
				}
			}
			uploadErrs := p.UploadOutputToS3Bucket(log,
				config.PluginID,
				config.OrchestrationDirectory,
				config.OutputS3BucketName,
				config.OutputS3KeyPrefix,
				useTemp,
				config.OrchestrationDirectory,
				out[0].Stdout,
				out[0].Stderr)
			for _, uploadErr := range uploadErrs {
				out[0].AppendError(log, uploadErr)
			}
		}
	}
	pluginutil.PersistPluginInformationToCurrent(log, config.PluginID, config, res)

	return res
}

// Name returns the name of the plugin.
func Name() string {
	return appconfig.PluginNameAwsConfigurePackage
}
