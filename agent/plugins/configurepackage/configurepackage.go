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
	"path/filepath"
	"regexp"
	"time"

	"github.com/aws/amazon-ssm-agent/agent/appconfig"
	"github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/aws/amazon-ssm-agent/agent/contracts"
	"github.com/aws/amazon-ssm-agent/agent/fileutil"
	"github.com/aws/amazon-ssm-agent/agent/fileutil/artifact"
	"github.com/aws/amazon-ssm-agent/agent/framework/runpluginutil"
	"github.com/aws/amazon-ssm-agent/agent/jsonutil"
	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/amazon-ssm-agent/agent/plugins/configurepackage/localpackages"
	"github.com/aws/amazon-ssm-agent/agent/plugins/pluginutil"
	"github.com/aws/amazon-ssm-agent/agent/task"
	"github.com/aws/amazon-ssm-agent/agent/times"
	"github.com/aws/amazon-ssm-agent/agent/updateutil"
)

const (
	// InstallAction represents the json command to install package
	InstallAction = "Install"
	// UninstallAction represents the json command to uninstall package
	UninstallAction = "Uninstall"
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
	plugin.ExecuteUploadOutputToS3Bucket = pluginutil.UploadOutputToS3BucketExecuter(plugin.UploadOutputToS3Bucket)

	return &plugin, nil
}

type configurePackage struct {
	contracts.Configuration
	runner     runpluginutil.PluginRunner
	repository localpackages.Repository
}

type configurePackageManager interface {
	validateInput(context context.T, input *ConfigurePackagePluginInput) (valid bool, err error)

	getVersionToInstall(context context.T, input *ConfigurePackagePluginInput, util configureUtil) (version string, installedVersion string, installState localpackages.InstallState, err error)

	getVersionToUninstall(context context.T, input *ConfigurePackagePluginInput, util configureUtil) (version string, err error)

	ensurePackage(context context.T,
		util configureUtil,
		packageName string,
		version string,
		output *contracts.PluginOutput) error

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

	runValidatePackage(context context.T,
		packageName string,
		version string,
		output *contracts.PluginOutput) (status contracts.ResultStatus, err error)

	setInstallState(context context.T, packageName string, version string, state localpackages.InstallState) error
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
		version, installedVersion, installState, versionErr := manager.getVersionToInstall(context, &input, configUtil)
		if versionErr != nil {
			output.MarkAsFailed(log, fmt.Errorf("unable to determine version to install: %v", versionErr))
			return
		}

		// ensure manifest file and package
		ensureErr := manager.ensurePackage(context, configUtil, input.Name, version, &output)
		if ensureErr != nil {
			output.MarkAsFailed(log, fmt.Errorf("unable to obtain package: %v", ensureErr))
			return
		}

		// if no previous version, set state to new
		if installState == localpackages.None {
			manager.setInstallState(context, input.Name, version, localpackages.New)
		}

		// if already installed and valid, exit
		if (version == installedVersion && (installState == localpackages.Installed || installState == localpackages.Unknown)) || installState == localpackages.Installing {
			// TODO: When existing packages have idempotent installers and no reboot loops, remove this check for installing packages and allow the install to continue until it reports success without reboot
			if result, err := manager.runValidatePackage(context, input.Name, version, &output); err == nil && result == contracts.ResultStatusSuccess {
				if installState == localpackages.Installing {
					output.AppendInfof(log, "Successfully installed %v %v", input.Name, version)
				} else {
					output.AppendInfof(log, "%v %v is already installed", input.Name, version)
				}
				output.MarkAsSucceeded()
				if installState != localpackages.Installed && installState != localpackages.Unknown {
					manager.setInstallState(context, input.Name, version, localpackages.Installed)
					return
				}
				return
			}
		}

		// if different version is installed, uninstall
		// if status is "installing" then we are returning after a reboot in a case where the uninstall of the previous version has already happened
		if installedVersion != "" && installedVersion != version && installState != localpackages.Installing {
			ensureErr := manager.ensurePackage(context, configUtil, input.Name, installedVersion, &output)
			if ensureErr != nil {
				output.AppendErrorf(log, "unable to obtain package: %v", ensureErr)
			} else {
				manager.setInstallState(context, input.Name, version, localpackages.Upgrading)
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

		// install version
		manager.setInstallState(context, input.Name, version, localpackages.Installing)
		result, err := manager.runInstallPackage(context,
			input.Name,
			version,
			&output)
		if err != nil {
			output.MarkAsFailed(log, fmt.Errorf("failed to install package: %v", err))
			manager.setInstallState(context, input.Name, version, localpackages.Failed)
		} else if result == contracts.ResultStatusSuccessAndReboot || result == contracts.ResultStatusPassedAndReboot {
			output.AppendInfof(log, "Rebooting to finish installation of %v %v", input.Name, version)
			output.MarkAsSuccessWithReboot()
			// NOTE: When we support rollback, we should exit here before we delete the previous package
			// NOTE: status remains "installing" here, plugin will re-run and either pass validation after reboot and get marked as "installed" or run the idempotent install again until it succeeds or arrives at a valid state
		} else if result != contracts.ResultStatusSuccess {
			output.MarkAsFailed(log, fmt.Errorf("install action state was %v and not %v", result, contracts.ResultStatusSuccess))
			manager.setInstallState(context, input.Name, version, localpackages.Failed)
		} else {
			if result, err := manager.runValidatePackage(context, input.Name, version, &output); err != nil || result != contracts.ResultStatusSuccess {
				output.MarkAsFailed(log, fmt.Errorf("failed to install package.  Validation status %v, Validation error %v", result, err))
				manager.setInstallState(context, input.Name, version, localpackages.Failed)
			} else {
				output.AppendInfof(log, "Successfully installed %v %v", input.Name, version)
				output.MarkAsSucceeded()
				manager.setInstallState(context, input.Name, version, localpackages.Installed)
			}
		}

		// NOTE: When we support rollback, we should rollback here if status is failed
		// uninstall post action
		if installedVersion != "" && installedVersion != version {
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
		ensureErr := manager.ensurePackage(context, configUtil, input.Name, version, &output)
		if ensureErr != nil {
			output.MarkAsFailed(log, fmt.Errorf("unable to obtain package: %v", ensureErr))
			return
		}

		manager.setInstallState(context, input.Name, version, localpackages.Uninstalling)
		var resultPre, resultPost contracts.ResultStatus
		resultPre, err = manager.runUninstallPackagePre(context,
			input.Name,
			version,
			&output)
		if err != nil {
			output.MarkAsFailed(log, fmt.Errorf("failed to uninstall package: %v", err))
			manager.setInstallState(context, input.Name, version, localpackages.Failed)
			return
		} else {
			if resultPre == contracts.ResultStatusSuccessAndReboot || resultPre == contracts.ResultStatusPassedAndReboot {
				// Reboot before continuing
				output.MarkAsSuccessWithReboot()
				return
			}
		}

		resultPost, err = manager.runUninstallPackagePost(context,
			input.Name,
			version,
			&output)
		if err != nil {
			output.MarkAsFailed(log, fmt.Errorf("failed to uninstall package: %v", err))
			manager.setInstallState(context, input.Name, version, localpackages.Failed)
			return
		}
		manager.setInstallState(context, input.Name, version, localpackages.Uninstalled)

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
	output *contracts.PluginOutput) error {

	currentState, currentVersion := m.repository.GetInstallState(context, packageName)
	if err := m.repository.ValidatePackage(context, packageName, version); err != nil || (currentVersion == version && currentState == localpackages.Failed) {
		context.Log().Debugf("Current %v Target %v State %v", currentVersion, version, currentState)
		context.Log().Debugf("Refreshing package content for %v %v %v", packageName, version, err)
		return m.repository.RefreshPackage(context, packageName, version, func(targetDirectory string) error {
			return downloadPackageFromS3(context, util.GetS3Location(packageName, version), targetDirectory)
		})
	}
	return nil
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
	util configureUtil) (version string, installedVersion string, installState localpackages.InstallState, err error) {
	installedVersion = m.repository.GetInstalledVersion(context, input.Name)
	currentState, currentVersion := m.repository.GetInstallState(context, input.Name)
	if currentState == localpackages.Failed {
		// TODO: once rollback is implemented, this will only happen if install failed with no previous successful install or if rollback failed
		installedVersion = currentVersion
	}

	if input.Version != "" {
		version = input.Version
	} else {
		if version, err = util.GetLatestVersion(context.Log(), input.Name); err != nil {
			return
		}
	}
	return version, installedVersion, currentState, nil
}

// getVersionToUninstall decides which version to uninstall
func (m *configurePackage) getVersionToUninstall(context context.T,
	input *ConfigurePackagePluginInput,
	util configureUtil) (version string, err error) {
	if input.Version != "" {
		version = input.Version
	} else if installedVersion := m.repository.GetInstalledVersion(context, input.Name); installedVersion != "" {
		version = installedVersion
	} else {
		version, err = util.GetLatestVersion(context.Log(), input.Name)
	}
	return
}

// downloadPackageFromS3 downloads and uncompresses the installation package from s3 bucket
func downloadPackageFromS3(context context.T, packageS3Source string, packageDestination string) error {
	log := context.Log()
	downloadInput := artifact.DownloadInput{
		SourceURL:            packageS3Source,
		DestinationDirectory: packageDestination}

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
		return errors.New(errMessage)
	}

	filePath := downloadOutput.LocalFilePath
	if uncompressErr := filesysdep.Uncompress(filePath, packageDestination); uncompressErr != nil {
		return fmt.Errorf("failed to extract package installer package %v from %v, %v", filePath, packageDestination, uncompressErr.Error())
	}

	// NOTE: this could be considered a warning - it likely points to a real problem, but if uncompress succeeded, we could continue
	// delete compressed package after using
	if cleanupErr := filesysdep.RemoveAll(filePath); cleanupErr != nil {
		return fmt.Errorf("failed to delete compressed package %v, %v", filePath, cleanupErr.Error())
	}

	return nil
}

// setInstallState sets the current installation state for the package in the persistent store.
func (m *configurePackage) setInstallState(context context.T, packageName string, version string, state localpackages.InstallState) error {
	err := m.repository.SetInstallState(context, packageName, version, state)
	if err != nil {
		context.Log().Errorf("failed to set install state to Installing: %v", err)
	}
	return err
}

// runValidatePackage executes the install script for the specific version of a package.
func (m *configurePackage) runValidatePackage(context context.T,
	packageName string,
	version string,
	output *contracts.PluginOutput) (status contracts.ResultStatus, err error) {
	if exists, status, err := m.executeAction(context, "validate", packageName, version, output); exists {
		return status, err
	}
	return contracts.ResultStatusSuccess, nil
}

// runInstallPackage executes the install script for the specific version of a package.
func (m *configurePackage) runInstallPackage(context context.T,
	packageName string,
	version string,
	output *contracts.PluginOutput) (status contracts.ResultStatus, err error) {
	if exists, status, err := m.executeAction(context, "install", packageName, version, output); exists {
		return status, err
	}
	return contracts.ResultStatusSuccess, nil
}

// runUninstallPackagePre executes the uninstall script for the specific version of a package.
func (m *configurePackage) runUninstallPackagePre(context context.T,
	packageName string,
	version string,
	output *contracts.PluginOutput) (status contracts.ResultStatus, err error) {
	if exists, status, err := m.executeAction(context, "uninstall", packageName, version, output); exists {
		return status, err
	}
	return contracts.ResultStatusSuccess, nil
}

// runUninstallPackagePost performs post uninstall actions, like deleting the package folder
func (m *configurePackage) runUninstallPackagePost(context context.T,
	packageName string,
	version string,
	output *contracts.PluginOutput) (status contracts.ResultStatus, err error) {

	if err = m.repository.RemovePackage(context, packageName, version); err != nil {
		return contracts.ResultStatusFailed, err
	}
	return contracts.ResultStatusSuccess, nil
}

// executeAction executes a command document as a sub-document of the current command and returns the result
func (m *configurePackage) executeAction(context context.T,
	actionName string,
	packageName string,
	version string,
	output *contracts.PluginOutput) (actionExists bool, status contracts.ResultStatus, err error) {
	status = contracts.ResultStatusSuccess
	err = nil

	log := context.Log()
	actionExists, actionContent, executeDirectory, err := m.repository.GetAction(context, packageName, version, actionName)
	if err != nil {
		return true, contracts.ResultStatusFailed, err
	}
	if actionExists {
		output.AppendInfof(log, "Initiating %v %v %v", packageName, version, actionName)
		var s3Prefix string
		if m.OutputS3BucketName != "" {
			s3Prefix = fileutil.BuildS3Path(m.OutputS3KeyPrefix, m.PluginID, actionName)
		}
		pluginsInfo, err := execdep.ParseDocument(context, actionContent, m.OrchestrationDirectory, m.OutputS3BucketName, s3Prefix, m.MessageId, m.BookKeepingFileName, executeDirectory)
		if err != nil {
			return true, contracts.ResultStatusFailed, err
		}
		if len(pluginsInfo) == 0 {
			return true, contracts.ResultStatusFailed, fmt.Errorf("%v document contained no work and may be malformed", actionName)
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
	if properties = pluginutil.LoadParametersAsList(log, config.Properties, &res); res.Code != 0 {
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
	manager := &configurePackage{Configuration: config, runner: subDocumentRunner, repository: localpackages.NewRepository()}

	for i, prop := range properties {

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
			uploadErrs := p.ExecuteUploadOutputToS3Bucket(log,
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
