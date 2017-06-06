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
	"time"

	"github.com/aws/amazon-ssm-agent/agent/appconfig"
	"github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/aws/amazon-ssm-agent/agent/contracts"
	"github.com/aws/amazon-ssm-agent/agent/fileutil"
	"github.com/aws/amazon-ssm-agent/agent/framework/runpluginutil"
	"github.com/aws/amazon-ssm-agent/agent/jsonutil"
	"github.com/aws/amazon-ssm-agent/agent/platform"
	"github.com/aws/amazon-ssm-agent/agent/plugins/configurepackage/installer"
	"github.com/aws/amazon-ssm-agent/agent/plugins/configurepackage/localpackages"
	"github.com/aws/amazon-ssm-agent/agent/plugins/configurepackage/packageservice"
	"github.com/aws/amazon-ssm-agent/agent/plugins/configurepackage/ssms3"
	"github.com/aws/amazon-ssm-agent/agent/plugins/pluginutil"
	"github.com/aws/amazon-ssm-agent/agent/task"
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
	runner         runpluginutil.PluginRunner
	repository     localpackages.Repository
	packageservice packageservice.PackageService
}

type configurePackageManager interface {
	getVersionToInstall(context context.T, input *ConfigurePackagePluginInput) (version string, installedVersion string, installState localpackages.InstallState, err error)

	getVersionToUninstall(context context.T, input *ConfigurePackagePluginInput) (version string, err error)

	ensurePackage(context context.T,
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
	input *ConfigurePackagePluginInput) (output contracts.PluginOutput) {

	var err error
	log := context.Log()

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
		version, installedVersion, installState, versionErr := manager.getVersionToInstall(context, input)
		if versionErr != nil {
			output.MarkAsFailed(log, fmt.Errorf("unable to determine version to install: %v", versionErr))
			return
		}

		// ensure manifest file and package
		ensureErr := manager.ensurePackage(context, input.Name, version, &output)
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
			ensureErr := manager.ensurePackage(context, input.Name, installedVersion, &output)
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
			log.Debugf("expected %v but got %v", contracts.ResultStatusSuccess, result)
			output.MarkAsFailed(log, fmt.Errorf("install action was not successful"))
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
		version, versionErr := manager.getVersionToUninstall(context, input)
		if versionErr != nil || version == "" {
			output.MarkAsFailed(log, fmt.Errorf("unable to determine version to uninstall: %v", versionErr))
			return
		}

		// ensure manifest file and package
		ensureErr := manager.ensurePackage(context, input.Name, version, &output)
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
	packageName string,
	version string,
	output *contracts.PluginOutput) error {

	currentState, currentVersion := m.repository.GetInstallState(context, packageName)
	if err := m.repository.ValidatePackage(context, packageName, version); err != nil || (currentVersion == version && currentState == localpackages.Failed) {
		context.Log().Debugf("Current %v Target %v State %v", currentVersion, version, currentState)
		context.Log().Debugf("Refreshing package content for %v %v %v", packageName, version, err)
		return m.repository.RefreshPackage(context, packageName, version, func(targetDirectory string) error {
			filePath, err := m.packageservice.DownloadArtifact(context.Log(), packageName, version)
			if err != nil {
				return err
			}

			if uncompressErr := filesysdep.Uncompress(filePath, targetDirectory); uncompressErr != nil {
				return fmt.Errorf("failed to extract package installer package %v from %v, %v", filePath, targetDirectory, uncompressErr.Error())
			}

			// NOTE: this could be considered a warning - it likely points to a real problem, but if uncompress succeeded, we could continue
			// delete compressed package after using
			if cleanupErr := filesysdep.RemoveAll(filePath); cleanupErr != nil {
				return fmt.Errorf("failed to delete compressed package %v, %v", filePath, cleanupErr.Error())
			}

			return nil
		})
	}
	return nil
}

func parseAndValidateInput(rawPluginInput interface{}) (*ConfigurePackagePluginInput, error) {
	var input ConfigurePackagePluginInput
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
func validateInput(input *ConfigurePackagePluginInput) (valid bool, err error) {
	// source not yet supported
	if input.Source != "" {
		return false, errors.New("source parameter is not supported in this version")
	}

	// ensure non-empty name
	if input.Name == "" {
		return false, errors.New("empty name field")
	}

	// dump any unsupported value for Repository
	if input.Repository != "beta" && input.Repository != "gamma" {
		input.Repository = ""
	}

	return true, nil
}

// getVersionToInstall decides which version to install and whether there is an existing version (that is not in the process of installing)
func (m *configurePackage) getVersionToInstall(context context.T,
	input *ConfigurePackagePluginInput) (version string, installedVersion string, installState localpackages.InstallState, err error) {

	installedVersion = m.repository.GetInstalledVersion(context, input.Name)
	currentState, currentVersion := m.repository.GetInstallState(context, input.Name)
	if currentState == localpackages.Failed {
		// TODO: once rollback is implemented, this will only happen if install failed with no previous successful install or if rollback failed
		installedVersion = currentVersion
	}

	if !packageservice.IsLatest(input.Version) {
		version = input.Version
	} else {
		version, err = m.packageservice.DownloadManifest(context.Log(), input.Name, packageservice.Latest)
		if err != nil {
			return "", installedVersion, currentState, err
		}
	}
	return version, installedVersion, currentState, nil
}

// getVersionToUninstall decides which version to uninstall
func (m *configurePackage) getVersionToUninstall(context context.T,
	input *ConfigurePackagePluginInput) (string, error) {

	installedVersion := m.repository.GetInstalledVersion(context, input.Name)

	if !packageservice.IsLatest(input.Version) {
		if input.Version != installedVersion {
			return "", fmt.Errorf("selected version (%s) is not installed (%s)", input.Version, installedVersion)
		}
	}

	if installedVersion != "" {
		return installedVersion, nil
	}

	return "", nil
}

// setInstallState sets the current installation state for the package in the persistent store.
func (m *configurePackage) setInstallState(context context.T, packageName string, version string, state localpackages.InstallState) error {
	err := m.repository.SetInstallState(context, packageName, version, state)
	if err != nil {
		context.Log().Errorf("failed to set install state to Installing: %v", err)
	}
	return err
}

// TODO:MF: Get the Installer in the main function and call these methods directly, the helper method isn't really necessary
// runValidatePackage executes the install script for the specific version of a package.
func (m *configurePackage) runValidatePackage(context context.T,
	packageName string,
	version string,
	output *contracts.PluginOutput) (status contracts.ResultStatus, err error) {
	log := context.Log()

	var inst installer.Installer
	config := m.Configuration
	config.OutputS3KeyPrefix = fileutil.BuildS3Path(config.OutputS3KeyPrefix, config.PluginID)
	if inst = m.repository.GetInstaller(context, config, m.runner, packageName, version); inst == nil {
		return contracts.ResultStatusFailed, fmt.Errorf("failed to validate %v", packageName)
	}
	result := inst.Validate(context)

	output.AppendInfo(log, result.Stdout)
	output.AppendError(log, result.Stderr)

	return result.Status, nil
}

// runInstallPackage executes the install script for the specific version of a package.
func (m *configurePackage) runInstallPackage(context context.T,
	packageName string,
	version string,
	output *contracts.PluginOutput) (status contracts.ResultStatus, err error) {
	log := context.Log()

	var inst installer.Installer
	config := m.Configuration
	config.OutputS3KeyPrefix = fileutil.BuildS3Path(config.OutputS3KeyPrefix, config.PluginID)
	if inst = m.repository.GetInstaller(context, config, m.runner, packageName, version); inst == nil {
		return contracts.ResultStatusFailed, fmt.Errorf("failed to install %v", packageName)
	}
	result := inst.Install(context)

	output.AppendInfo(log, result.Stdout)
	output.AppendError(log, result.Stderr)

	return result.Status, nil
}

// runUninstallPackagePre executes the uninstall script for the specific version of a package.
func (m *configurePackage) runUninstallPackagePre(context context.T,
	packageName string,
	version string,
	output *contracts.PluginOutput) (status contracts.ResultStatus, err error) {
	log := context.Log()

	var inst installer.Installer
	config := m.Configuration
	config.OutputS3KeyPrefix = fileutil.BuildS3Path(config.OutputS3KeyPrefix, config.PluginID)
	if inst = m.repository.GetInstaller(context, config, m.runner, packageName, version); inst == nil {
		return contracts.ResultStatusFailed, fmt.Errorf("failed to uninstall %v", packageName)
	}
	result := inst.Uninstall(context)

	output.AppendInfo(log, result.Stdout)
	output.AppendError(log, result.Stderr)

	return result.Status, nil
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

	repository := localpackages.NewRepository()

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

		input, err := parseAndValidateInput(prop)
		if err != nil {
			var output contracts.PluginOutput
			output.MarkAsFailed(log, err)
			out[i] = output
			continue
		}

		var pkgservice packageservice.PackageService
		region, _ := platform.Region()
		pkgservice = ssms3.New(log, input.Repository, region)
		//pkgservice = birdwatcher.New(log)

		manager := &configurePackage{
			Configuration:  config,
			runner:         subDocumentRunner,
			repository:     repository,
			packageservice: pkgservice,
		}

		out[i] = runConfig(p,
			context,
			manager,
			input)
	}

	if len(out) > 0 {
		// Input is a list for V1.2 schema but we only return results for the first one
		res.Code = out[0].ExitCode
		res.Status = out[0].Status
		res.Output = out[0].String()
		res.StandardOutput = pluginutil.StringPrefix(out[0].Stdout, p.MaxStdoutLength, p.OutputTruncatedSuffix)
		res.StandardError = pluginutil.StringPrefix(out[0].Stderr, p.MaxStderrLength, p.OutputTruncatedSuffix)
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
