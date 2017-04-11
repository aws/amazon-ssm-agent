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
	"github.com/aws/amazon-ssm-agent/agent/framework/runpluginutil"
	"github.com/aws/amazon-ssm-agent/agent/jsonutil"
	"github.com/aws/amazon-ssm-agent/agent/log"
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
	packageServiceSelector func(serviceEndpoint string) packageservice.PackageService
	localRepository        localpackages.Repository
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

	plugin.localRepository = localpackages.NewRepository()
	plugin.packageServiceSelector = selectService

	return &plugin, nil
}

// prepareConfigurePackage ensures the packages are present with the right version for the scenario requested and returns their installers
func prepareConfigurePackage(
	context context.T,
	config contracts.Configuration,
	runner runpluginutil.PluginRunner,
	repository localpackages.Repository,
	packageService packageservice.PackageService,
	input *ConfigurePackagePluginInput,
	output *contracts.PluginOutput) (inst installer.Installer, uninst installer.Installer, installState localpackages.InstallState, installedVersion string) {

	log := context.Log()

	switch input.Action {
	case InstallAction:
		// get version information
		var version string
		var err error
		version, installedVersion, installState, err = getVersionToInstall(context, repository, packageService, input)
		if err != nil {
			output.MarkAsFailed(log, fmt.Errorf("unable to determine version to install: %v", err))
			return
		}

		// ensure manifest file and package
		inst, err = ensurePackage(context, repository, packageService, input.Name, version, config, runner)
		if err != nil {
			output.MarkAsFailed(log, fmt.Errorf("unable to obtain package: %v", err))
			return
		}

		// if different version is installed, uninstall
		if installedVersion != "" && installedVersion != version {
			uninst, err = ensurePackage(context, repository, packageService, input.Name, installedVersion, config, runner)
			if err != nil {
				output.AppendErrorf(log, "unable to obtain package: %v", err)
			}
		}

	case UninstallAction:
		// get version information
		var version string
		var err error
		version, installState, err = getVersionToUninstall(context, repository, input)
		installedVersion = version
		if err != nil || version == "" {
			output.MarkAsFailed(log, fmt.Errorf("unable to determine version to uninstall: %v", err))
			return
		}

		// ensure manifest file and package
		uninst, err = ensurePackage(context, repository, packageService, input.Name, version, config, runner)
		if err != nil {
			output.MarkAsFailed(log, fmt.Errorf("unable to obtain package: %v", err))
			return
		}

	default:
		output.MarkAsFailed(log, fmt.Errorf("unsupported action: %v", input.Action))
		return
	}

	return inst, uninst, installState, installedVersion
}

// ensurePackage validates local copy of the manifest and package and downloads if needed, returning the installer
func ensurePackage(context context.T,
	repository localpackages.Repository,
	packageService packageservice.PackageService,
	packageName string,
	version string,
	config contracts.Configuration,
	runner runpluginutil.PluginRunner) (installer.Installer, error) {

	currentState, currentVersion := repository.GetInstallState(context, packageName)
	if err := repository.ValidatePackage(context, packageName, version); err != nil || (currentVersion == version && currentState == localpackages.Failed) {
		context.Log().Debugf("Current %v Target %v State %v", currentVersion, version, currentState)
		context.Log().Debugf("Refreshing package content for %v %v %v", packageName, version, err)
		if err = repository.RefreshPackage(context, packageName, version, buildDownloadDelegate(context, packageService, packageName, version)); err != nil {
			return nil, err
		}
		if err = repository.ValidatePackage(context, packageName, version); err != nil {
			// TODO: Remove from repository?
			return nil, err
		}
	}
	return repository.GetInstaller(context, config, runner, packageName, version), nil
}

// buildDownloadDelegate constructs the delegate used by the repository to download a package from the service
func buildDownloadDelegate(context context.T, packageService packageservice.PackageService, packageName string, version string) func(string) error {
	return func(targetDirectory string) error {
		filePath, err := packageService.DownloadArtifact(context.Log(), packageName, version)
		if err != nil {
			return err
		}

		// TODO: Consider putting uncompress into the ssminstaller new and not deleting it (since the zip is the repository-validatable artifact)
		if uncompressErr := filesysdep.Uncompress(filePath, targetDirectory); uncompressErr != nil {
			return fmt.Errorf("failed to extract package installer package %v from %v, %v", filePath, targetDirectory, uncompressErr.Error())
		}

		// NOTE: this could be considered a warning - it likely points to a real problem, but if uncompress succeeded, we could continue
		// delete compressed package after using
		if cleanupErr := filesysdep.RemoveAll(filePath); cleanupErr != nil {
			return fmt.Errorf("failed to delete compressed package %v, %v", filePath, cleanupErr.Error())
		}

		return nil
	}
}

// getVersionToInstall decides which version to install and whether there is an existing version (that is not in the process of installing)
func getVersionToInstall(context context.T,
	repository localpackages.Repository,
	packageService packageservice.PackageService,
	input *ConfigurePackagePluginInput) (version string, installedVersion string, installState localpackages.InstallState, err error) {

	installedVersion = repository.GetInstalledVersion(context, input.Name)
	currentState, currentVersion := repository.GetInstallState(context, input.Name)
	if currentState == localpackages.Failed {
		// This will only happen if install failed with no previous successful install or if rollback failed
		installedVersion = currentVersion
	}

	if !packageservice.IsLatest(input.Version) {
		version = input.Version
	} else {
		version, err = packageService.DownloadManifest(context.Log(), input.Name, packageservice.Latest)
		if err != nil {
			return "", installedVersion, currentState, err
		}
	}
	return version, installedVersion, currentState, nil
}

// getVersionToUninstall decides which version to uninstall
func getVersionToUninstall(context context.T,
	repository localpackages.Repository,
	input *ConfigurePackagePluginInput) (string, localpackages.InstallState, error) {

	installedVersion := repository.GetInstalledVersion(context, input.Name)
	currentState, _ := repository.GetInstallState(context, input.Name)

	if !packageservice.IsLatest(input.Version) {
		if input.Version != installedVersion {
			return installedVersion, currentState, fmt.Errorf("selected version (%s) is not installed (%s)", input.Version, installedVersion)
		}
	}

	if installedVersion == "" {
		return "", localpackages.None, nil
	}

	return installedVersion, currentState, nil
}

// parseAndValidateInput marshals raw JSON and returns the result of input validation or an error
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

// checkAlreadyInstalled returns true if the version being installed is already in a valid installed state
func checkAlreadyInstalled(context context.T,
	repository localpackages.Repository,
	installedVersion string,
	installState localpackages.InstallState,
	inst installer.Installer,
	uninst installer.Installer,
	output *contracts.PluginOutput) bool {
	if inst != nil {
		targetVersion := inst.Version()
		packageName := inst.PackageName()
		var instToCheck installer.Installer
		// TODO: When existing packages have idempotent installers and no reboot loops, remove this check for installing packages and allow the install to continue until it reports success without reboot
		if uninst != nil && installState == localpackages.RollbackInstall {
			// This supports rollback to a version whose installer contains an unconditional reboot
			instToCheck = uninst
		}
		if (targetVersion == installedVersion &&
			(installState == localpackages.Installed || installState == localpackages.Unknown)) ||
			installState == localpackages.Installing {
			instToCheck = inst
		}
		if instToCheck != nil {
			log := context.Log()
			validateOutput := instToCheck.Validate(context)
			if validateOutput.Status == contracts.ResultStatusSuccess {
				if installState == localpackages.Installing {
					output.AppendInfof(log, "Successfully installed %v %v", packageName, targetVersion)
					if uninst != nil {
						cleanupAfterUninstall(context, repository, uninst, output)
					}
					// TODO: report result
					output.MarkAsSucceeded()
				} else if installState == localpackages.RollbackInstall {
					output.AppendInfof(context.Log(), "Failed to install %v %v, successfully rolled back to %v %v", uninst.PackageName(), uninst.Version(), inst.PackageName(), inst.Version())
					cleanupAfterUninstall(context, repository, inst, output)
					// TODO: report result
					output.MarkAsFailed(context.Log(), nil)
				} else {
					output.AppendInfof(log, "%v %v is already installed", packageName, targetVersion)
					output.MarkAsSucceeded()
				}
				if installState != localpackages.Installed && installState != localpackages.Unknown {
					repository.SetInstallState(context, packageName, instToCheck.Version(), localpackages.Installed)
				}
				return true
			} else {
				output.AppendInfo(log, validateOutput.Stdout)
				output.AppendError(log, validateOutput.Stderr)
			}
		}
	}
	return false
}

// selectService chooses the implementation of PackageService to use for a given execution of the plugin
func selectService(serviceEndpoint string) packageservice.PackageService {
	region, _ := platform.Region()
	packageService := ssms3.New(serviceEndpoint, region)
	//packageService = birdwatcher.New(log, repository)

	return packageService
}

// Execute runs the plugin operation and returns output
// res.Output will contain a slice of RunCommandPluginOutput
func (p *Plugin) Execute(context context.T, config contracts.Configuration, cancelFlag task.CancelFlag, subDocumentRunner runpluginutil.PluginRunner) (res contracts.PluginResult) {
	return p.execute(context, config, cancelFlag, subDocumentRunner, pluginutil.PersistPluginInformationToCurrent)
}

func (p *Plugin) execute(context context.T, config contracts.Configuration, cancelFlag task.CancelFlag, subDocumentRunner runpluginutil.PluginRunner, persistPluginInfo func(log log.T, pluginID string, config contracts.Configuration, res contracts.PluginResult)) (res contracts.PluginResult) {
	log := context.Log()
	log.Info("RunCommand started with configuration ", config)

	res.StartDateTime = time.Now()
	defer func() { res.EndDateTime = time.Now() }()

	if cancelFlag.ShutDown() {
		res.Code = 1
		res.Status = contracts.ResultStatusFailed
	} else if cancelFlag.Canceled() {
		res.Code = 1
		res.Status = contracts.ResultStatusCancelled
	}

	output := contracts.PluginOutput{}
	input, err := parseAndValidateInput(config.Properties)
	if err != nil {
		output.MarkAsFailed(log, err)
	} else {
		// do not allow multiple actions to be performed at the same time for the same package
		// this is possible with multiple concurrent runcommand documents
		if err := lockPackage(input.Name, input.Action); err != nil {
			output.MarkAsFailed(log, err)
			return
		}
		defer unlockPackage(input.Name)

		packageService := p.packageServiceSelector(input.Repository)

		log.Debugf("Prepare for %v %v %v", input.Action, input.Name, input.Version)
		inst, uninst, installState, installedVersion := prepareConfigurePackage(
			context,
			config,
			subDocumentRunner,
			p.localRepository,
			packageService,
			input,
			&output)
		log.Debugf("HasInst %v, HasUninst %v, InstallState %v, InstalledVersion %v", inst != nil, uninst != nil, installState, installedVersion)
		// if already failed or already installed and valid, do not execute install
		if output.Status != contracts.ResultStatusFailed && !checkAlreadyInstalled(context, p.localRepository, installedVersion, installState, inst, uninst, &output) {
			log.Debugf("Calling execute, current status %v", output.Status)
			result := executeConfigurePackage(context,
				p.localRepository,
				inst,
				uninst,
				installState,
				&output)
			if !output.Status.IsReboot() {
				packageService.ReportResult(context.Log(), result)
			}
		}
	}

	if config.OrchestrationDirectory != "" {
		useTemp := false
		outFile := filepath.Join(config.OrchestrationDirectory, p.StdoutFileName)
		// create orchestration dir if needed
		if err := filesysdep.MakeDirExecute(config.OrchestrationDirectory); err != nil {
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
	persistPluginInfo(log, config.PluginID, config, res)

	res.Code = output.ExitCode
	res.Status = output.Status
	res.Output = output.String()
	res.StandardOutput = pluginutil.StringPrefix(output.Stdout, p.MaxStdoutLength, p.OutputTruncatedSuffix)
	res.StandardError = pluginutil.StringPrefix(output.Stderr, p.MaxStderrLength, p.OutputTruncatedSuffix)

	return res
}

// Name returns the name of the plugin.
func Name() string {
	return appconfig.PluginNameAwsConfigurePackage
}
