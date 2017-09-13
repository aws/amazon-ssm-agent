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
	"github.com/aws/amazon-ssm-agent/agent/jsonutil"
	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/amazon-ssm-agent/agent/platform"
	"github.com/aws/amazon-ssm-agent/agent/plugins/configurepackage/birdwatcher"
	"github.com/aws/amazon-ssm-agent/agent/plugins/configurepackage/installer"
	"github.com/aws/amazon-ssm-agent/agent/plugins/configurepackage/localpackages"
	"github.com/aws/amazon-ssm-agent/agent/plugins/configurepackage/packageservice"
	"github.com/aws/amazon-ssm-agent/agent/plugins/configurepackage/ssms3"
	"github.com/aws/amazon-ssm-agent/agent/plugins/configurepackage/trace"
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
	packageServiceSelector func(log log.T, serviceEndpoint string, localrepo localpackages.Repository) packageservice.PackageService
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
	tracer trace.Tracer,
	context context.T,
	config contracts.Configuration,
	repository localpackages.Repository,
	packageService packageservice.PackageService,
	input *ConfigurePackagePluginInput,
	output contracts.PluginOutputer) (inst installer.Installer, uninst installer.Installer, installState localpackages.InstallState, installedVersion string) {

	prepareTrace := tracer.BeginSection(fmt.Sprintf("prepare %s", input.Action))
	defer prepareTrace.End()

	switch input.Action {
	case InstallAction:
		// get version information
		var version string
		var err error
		trace := tracer.BeginSection("determine version to install")
		version, installedVersion, installState, err = getVersionToInstall(tracer, context, repository, packageService, input)
		if err != nil {
			trace.WithError(err).End()
			output.MarkAsFailed(nil, nil)
			return
		}
		trace.AppendInfof("installed: %v in state %v, to install: %v", installedVersion, installState, version).End()

		// ensure manifest file and package
		trace = tracer.BeginSection("ensure package is local available")
		inst, err = ensurePackage(context, repository, packageService, input.Name, version, config)
		if err != nil {
			trace.WithError(err).End()
			output.MarkAsFailed(nil, nil)
			return
		}
		trace.End()

		// if different version is installed, uninstall
		trace = tracer.BeginSection("ensure old package is local available")
		if installedVersion != "" && installedVersion != version {
			uninst, err = ensurePackage(context, repository, packageService, input.Name, installedVersion, config)
			if err != nil {
				trace.WithError(err)
			}
		}
		trace.End()

	case UninstallAction:
		// get version information
		var version string
		var err error
		trace := tracer.BeginSection("determine version to uninstall")
		version, installState, err = getVersionToUninstall(context, repository, input)
		installedVersion = version
		if err != nil || version == "" {
			trace.WithError(err).End()
			output.MarkAsFailed(nil, nil)
			return
		}
		trace.AppendInfof("installed: %v in state: %v", version, installState).End()

		// ensure manifest file and package
		trace = tracer.BeginSection("ensure package is local available")
		uninst, err = ensurePackage(context, repository, packageService, input.Name, version, config)
		if err != nil {
			trace.WithError(err).End()
			output.MarkAsFailed(nil, nil)
			return
		}
		trace.End()

	default:
		prepareTrace.AppendErrorf("unsupported action: %v", input.Action)
		output.MarkAsFailed(nil, nil)
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
	config contracts.Configuration) (installer.Installer, error) {

	currentState, currentVersion := repository.GetInstallState(context, packageName)
	if err := repository.ValidatePackage(context, packageName, version); err != nil || (currentVersion == version && currentState == localpackages.Failed) {
		context.Log().Debugf("Current %v Target %v State %v", currentVersion, version, currentState)
		context.Log().Debugf("Refreshing package content for %v %v %v", packageName, version, err)
		if err = repository.RefreshPackage(context, packageName, version, packageService.PackageServiceName(), buildDownloadDelegate(context, packageService, packageName, version)); err != nil {
			return nil, err
		}
		if err = repository.ValidatePackage(context, packageName, version); err != nil {
			// TODO: Remove from repository?
			return nil, err
		}
	}
	return repository.GetInstaller(context, config, packageName, version), nil
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
func getVersionToInstall(tracer trace.Tracer,
	context context.T,
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
func checkAlreadyInstalled(
	tracer trace.Tracer,
	context context.T,
	repository localpackages.Repository,
	installedVersion string,
	installState localpackages.InstallState,
	inst installer.Installer,
	uninst installer.Installer,
	output contracts.PluginOutputer) bool {

	checkTrace := tracer.BeginSection("check if already installed")
	defer checkTrace.End()

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
			validateTrace := tracer.BeginSection(fmt.Sprintf("run validate for %s/%s", instToCheck.PackageName(), instToCheck.Version()))

			validateOutput := instToCheck.Validate(context)
			validateTrace.WithExitcode(int64(validateOutput.GetExitCode()))

			if validateOutput.GetStatus() == contracts.ResultStatusSuccess {
				if installState == localpackages.Installing {
					validateTrace.AppendInfof("Successfully installed %v %v", packageName, targetVersion)
					if uninst != nil {
						cleanupAfterUninstall(context, repository, uninst, output)
					}
					// TODO: report result
					output.MarkAsSucceeded()
				} else if installState == localpackages.RollbackInstall {
					validateTrace.AppendInfof("Failed to install %v %v, successfully rolled back to %v %v", uninst.PackageName(), uninst.Version(), inst.PackageName(), inst.Version())
					cleanupAfterUninstall(context, repository, inst, output)
					// TODO: report result
					output.MarkAsFailed(nil, nil)
				} else {
					validateTrace.AppendInfof("%v %v is already installed", packageName, targetVersion)
					output.MarkAsSucceeded()
				}
				if installState != localpackages.Installed && installState != localpackages.Unknown {
					repository.SetInstallState(context, packageName, instToCheck.Version(), localpackages.Installed)
				}

				validateTrace.End()
				return true
			}

			validateTrace.AppendInfo(validateOutput.GetStdout())
			validateTrace.AppendError(validateOutput.GetStderr())
			validateTrace.End()
		}
	}

	checkTrace.WithExitcode(1)
	return false
}

// selectService chooses the implementation of PackageService to use for a given execution of the plugin
func selectService(log log.T, serviceEndpoint string, localrepo localpackages.Repository) packageservice.PackageService {
	region, _ := platform.Region()
	appCfg, err := appconfig.Config(false)

	if (err == nil && appCfg.Birdwatcher.ForceEnable) || !ssms3.UseSSMS3Service(log, serviceEndpoint, region) {
		log.Debugf("S3 repository is not marked active in %v %v", region, serviceEndpoint)
		return birdwatcher.New(serviceEndpoint, localrepo)
	}
	return ssms3.New(serviceEndpoint, region)
}

// Execute runs the plugin operation and returns output
// res.Output will contain a slice of RunCommandPluginOutput
func (p *Plugin) Execute(context context.T, config contracts.Configuration, cancelFlag task.CancelFlag) (res contracts.PluginResult) {
	return p.execute(context, config, cancelFlag)
}

func (p *Plugin) execute(context context.T, config contracts.Configuration, cancelFlag task.CancelFlag) (res contracts.PluginResult) {
	log := context.Log()
	log.Info("RunCommand started with configuration ", config)

	tracer := trace.NewTracer(log)
	defer tracer.BeginSection("configurePackage").End()

	res.StartDateTime = time.Now()
	defer func() {
		res.EndDateTime = time.Now()
	}()

	out := trace.PluginOutputTrace{Tracer: tracer}
	if cancelFlag.ShutDown() {
		out.MarkAsShutdown()
	} else if cancelFlag.Canceled() {
		out.MarkAsCancelled()
	} else if input, err := parseAndValidateInput(config.Properties); err != nil {
		tracer.CurrentTrace().WithError(err).End()
		out.MarkAsFailed(nil, nil)
	} else if err := lockPackage(input.Name, input.Action); err != nil {
		// do not allow multiple actions to be performed at the same time for the same package
		// this is possible with multiple concurrent runcommand documents
		tracer.CurrentTrace().WithError(err).End()
		out.MarkAsFailed(nil, nil)
	} else {
		defer unlockPackage(input.Name)

		packageService := p.packageServiceSelector(log, input.Repository, p.localRepository)

		log.Debugf("Prepare for %v %v %v", input.Action, input.Name, input.Version)
		inst, uninst, installState, installedVersion := prepareConfigurePackage(
			tracer,
			context,
			config,
			p.localRepository,
			packageService,
			input,
			&out)
		log.Debugf("HasInst %v, HasUninst %v, InstallState %v, InstalledVersion %v", inst != nil, uninst != nil, installState, installedVersion)
		// if already failed or already installed and valid, do not execute install
		if out.GetStatus() != contracts.ResultStatusFailed && !checkAlreadyInstalled(tracer, context, p.localRepository, installedVersion, installState, inst, uninst, &out) {
			log.Debugf("Calling execute, current status %v", out.GetStatus())
			executeConfigurePackage(context,
				p.localRepository,
				inst,
				uninst,
				installState,
				&out)
			if !out.GetStatus().IsReboot() {
				version := input.Version
				if input.Action == InstallAction {
					version = inst.Version()
				}
				err := packageService.ReportResult(context.Log(), packageservice.PackageResult{
					Exitcode:               int64(out.GetExitCode()),
					Operation:              input.Action,
					PackageName:            input.Name,
					PreviousPackageVersion: installedVersion,
					Timing:                 1,
					Version:                version,
				})
				if err != nil {
					out.AppendErrorf(log, "Error reporting results: %v", err.Error())
				}
			}
		}

		if config.OrchestrationDirectory != "" {
			useTemp := false
			outFile := filepath.Join(config.OrchestrationDirectory, p.StdoutFileName)
			// create orchestration dir if needed
			if err := filesysdep.MakeDirExecute(config.OrchestrationDirectory); err != nil {
				out.AppendError(log, "Failed to create orchestrationDir directory for log files")
			} else {
				if err := filesysdep.WriteFile(outFile, out.GetStdout()); err != nil {
					log.Debugf("Error writing to %v", outFile)
					out.AppendErrorf(log, "Error saving stdout: %v", err.Error())
				}
				errFile := filepath.Join(config.OrchestrationDirectory, p.StderrFileName)
				if err := filesysdep.WriteFile(errFile, out.GetStderr()); err != nil {
					log.Debugf("Error writing to %v", errFile)
					out.AppendErrorf(log, "Error saving stderr: %v", err.Error())
				}
			}
			uploadErrs := p.ExecuteUploadOutputToS3Bucket(log,
				config.PluginID,
				config.OrchestrationDirectory,
				config.OutputS3BucketName,
				config.OutputS3KeyPrefix,
				useTemp,
				config.OrchestrationDirectory,
				out.GetStdout(),
				out.GetStderr())
			for _, uploadErr := range uploadErrs {
				out.AppendError(log, uploadErr)
			}
		}
	}
	res.Code = out.GetExitCode()
	res.Status = out.GetStatus()

	// convert trace
	traceout := tracer.ToPluginOutput()
	res.Output = traceout.String()
	res.StandardOutput = pluginutil.StringPrefix(traceout.GetStdout(), p.MaxStdoutLength, p.OutputTruncatedSuffix)
	res.StandardError = pluginutil.StringPrefix(traceout.GetStderr(), p.MaxStderrLength, p.OutputTruncatedSuffix)

	return res
}

// Name returns the name of the plugin.
func Name() string {
	return appconfig.PluginNameAwsConfigurePackage
}
