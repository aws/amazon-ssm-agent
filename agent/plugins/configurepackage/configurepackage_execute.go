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
	"github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/aws/amazon-ssm-agent/agent/contracts"
	"github.com/aws/amazon-ssm-agent/agent/plugins/configurepackage/installer"
	"github.com/aws/amazon-ssm-agent/agent/plugins/configurepackage/localpackages"
)

// TODO: consider passing in the timeout and cancel channels - does cancel trigger rollback?
// executeConfigurePackage performs install and uninstall actions, with rollback support and recovery after reboots
func executeConfigurePackage(context context.T,
	repository localpackages.Repository,
	inst installer.Installer,
	uninst installer.Installer,
	initialInstallState localpackages.InstallState,
	output *contracts.PluginOutput) {

	switch initialInstallState {
	case localpackages.Installing:
		// This could be picking up an install after reboot or an upgrade that rebooted during install (after a successful uninstall)
		executeInstall(context, repository, inst, uninst, false, output)
	case localpackages.RollbackInstall:
		executeInstall(context, repository, uninst, inst, true, output)
	case localpackages.RollbackUninstall:
		executeUninstall(context, repository, uninst, inst, true, output)
	default:
		if uninst != nil {
			executeUninstall(context, repository, inst, uninst, false, output)
		} else {
			executeInstall(context, repository, inst, uninst, false, output)
		}
	}
}

// set package install state and log any error
func setNewInstallState(context context.T, repository localpackages.Repository, inst installer.Installer, newInstallState localpackages.InstallState) {
	if err := repository.SetInstallState(context, inst.PackageName(), inst.Version(), newInstallState); err != nil {
		context.Log().Errorf("Failed to set new install state: %v", err)
	}
}

// executeInstall performs install and validation of a package
func executeInstall(context context.T,
	repository localpackages.Repository,
	inst installer.Installer,
	uninst installer.Installer,
	isRollback bool,
	output *contracts.PluginOutput) {

	if isRollback {
		setNewInstallState(context, repository, inst, localpackages.RollbackInstall)
	} else {
		setNewInstallState(context, repository, inst, localpackages.Installing)
	}

	log := context.Log()
	result := inst.Install(context)
	output.AppendInfo(log, result.Stdout)
	output.AppendError(log, result.Stderr)
	if result.Status == contracts.ResultStatusSuccess {
		result = inst.Validate(context)
		output.AppendInfo(log, result.Stdout)
		output.AppendError(log, result.Stderr)
	}
	if result.Status.IsReboot() {
		output.AppendInfof(log, "Rebooting to finish installation of %v %v", inst.PackageName(), inst.Version())
		output.MarkAsSuccessWithReboot()
		return
	}
	if !result.Status.IsSuccess() {
		output.AppendErrorf(log, "Failed to install package; install status %v", result.Status)
		if isRollback || uninst == nil {
			output.MarkAsFailed(context.Log(), nil)
			// TODO: Remove from repository if this isn't the last successfully installed version?  Run uninstall to clean up?
			setNewInstallState(context, repository, inst, localpackages.Failed)
			return
		}
		// Execute rollback
		executeUninstall(context, repository, uninst, inst, true, output)
		return
	}
	if uninst != nil {
		cleanupAfterUninstall(context, repository, uninst, output)
	}
	if isRollback {
		output.AppendInfof(log, "Failed to install %v %v, successfully rolled back to %v %v", uninst.PackageName(), uninst.Version(), inst.PackageName(), inst.Version())
		setNewInstallState(context, repository, inst, localpackages.Installed)
		output.MarkAsFailed(context.Log(), nil)
		return
	}
	output.AppendInfof(log, "Successfully installed %v %v", inst.PackageName(), inst.Version())
	setNewInstallState(context, repository, inst, localpackages.Installed)
	output.MarkAsSucceeded()
	return
}

// executeUninstall performs uninstall of a package
func executeUninstall(context context.T,
	repository localpackages.Repository,
	inst installer.Installer,
	uninst installer.Installer,
	isRollback bool,
	output *contracts.PluginOutput) {

	if isRollback {
		setNewInstallState(context, repository, uninst, localpackages.RollbackUninstall)
	} else {
		if inst != nil {
			setNewInstallState(context, repository, uninst, localpackages.Upgrading)
		} else {
			setNewInstallState(context, repository, uninst, localpackages.Uninstalling)
		}
	}

	log := context.Log()
	result := uninst.Uninstall(context)
	output.AppendInfo(log, result.Stdout)
	output.AppendError(log, result.Stderr)
	if !result.Status.IsSuccess() {
		output.AppendErrorf(context.Log(), "Failed to uninstall version %v of package; uninstall status %v", uninst.Version(), result.Status)
		if inst != nil {
			executeInstall(context, repository, inst, uninst, isRollback, output)
			return
		}
		setNewInstallState(context, repository, uninst, localpackages.Failed)
		output.MarkAsFailed(context.Log(), nil)
		return
	}
	if result.Status.IsReboot() {
		output.AppendInfof(context.Log(), "Rebooting to finish uninstall of %v %v", uninst.PackageName(), uninst.Version())
		output.MarkAsSuccessWithReboot()
		return
	}
	output.AppendInfof(context.Log(), "Successfully uninstalled %v %v", uninst.PackageName(), uninst.Version())
	if inst != nil {
		executeInstall(context, repository, inst, uninst, isRollback, output)
		return
	}
	cleanupAfterUninstall(context, repository, uninst, output)
	setNewInstallState(context, repository, uninst, localpackages.None)
	output.MarkAsSucceeded()
}

// cleanupAfterUninstall removes packages that are no longer needed in the repository
func cleanupAfterUninstall(context context.T, repository localpackages.Repository, uninst installer.Installer, output *contracts.PluginOutput) {
	if err := repository.RemovePackage(context, uninst.PackageName(), uninst.Version()); err != nil {
		output.AppendErrorf(context.Log(), "Error cleaning up uninstalled version %v", err)
	}
}
