// Copyright 2016 Amazon.com, Inc. or its affiliates. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License"). You may not
// use this file except in compliance with the License. A copy of the
// License is located at
//
// http://aws.amazon.com/apache2.0/
//
// or in the "license" file accompanying this file. This file is distributed
// on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND,
// either express or implied. See the License for the specific language governing
// permissions and limitations under the License.

// Package application implements the application plugin.
//
// +build windows

package application

import (
	"fmt"

	"github.com/aws/amazon-ssm-agent/agent/appconfig"
	"github.com/aws/amazon-ssm-agent/agent/contracts"
	"github.com/aws/amazon-ssm-agent/agent/framework/processor/executer/iohandler"
	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/amazon-ssm-agent/agent/task"
)

const (
	INSTALL = "Install"

	UNINSTALL = "Uninstall"

	REPAIR = "Repair"
)

const (
	ErrorUnknownProduct = 1605

	ErrorSuccessRebootInitiated = 1641
)

// getMsiApplicationMode returns the msi exec mode based on plugin input
func getMsiApplicationMode(log log.T, pluginInput ApplicationPluginInput) (string, error) {
	switch pluginInput.Action {
	case INSTALL:
		return "/i", nil
	case UNINSTALL:
		return "/x", nil
	case REPAIR:
		return "/fa", nil
	default:
		return "", fmt.Errorf("MsiAction is set to unsupported value: %v", pluginInput.Action)
	}
}

// setMsiExecStatus sets the exit status and output to be returned to the user based on exit code
func setMsiExecStatus(log log.T, pluginInput ApplicationPluginInput, cancelFlag task.CancelFlag, out iohandler.IOHandler) {
	out.AppendInfo(pluginInput.Source)
	out.SetStatus(contracts.ResultStatusFailed)
	isUnKnownError := false

	switch out.GetExitCode() {
	case appconfig.SuccessExitCode:
		out.SetStatus(contracts.ResultStatusSuccess)
	case ErrorUnknownProduct:
		if pluginInput.Action == UNINSTALL {
			// Uninstall will skip, if product is not currently installed.
			// This is needed to support idempotent behavior.
			out.SetStatus(contracts.ResultStatusSuccess)
		}
	case ErrorSuccessRebootInitiated:
		fallthrough
	case appconfig.RebootExitCode:
		out.SetStatus(contracts.ResultStatusSuccessAndReboot)
	case appconfig.CommandStoppedPreemptivelyExitCode:
		if cancelFlag.ShutDown() {
			out.SetStatus(contracts.ResultStatusFailed)
		}
		if cancelFlag.Canceled() {
			out.SetStatus(contracts.ResultStatusCancelled)
		}
		out.SetStatus(contracts.ResultStatusTimedOut)
	default:
		isUnKnownError = true
	}

	if isUnKnownError {
		// Note: Sample Stderr:
		// Action:{Installed}; Status:{Failed};
		// ErrorCode:{1620}; ErrorMsg:{ERROR_INSTALL_PACKAGE_INVALID};
		// Description:{This installation package could not be opened. Contact the application vendor to verify that this is a valid Windows Installer package.};
		// Source:{https:///}

		// Construct stderr in above format using StandardMsiErrorCodes
		out.AppendErrorf("Action:{%v}; Status:{%v}; ErrorCode:{%v}; %v Source:{%v};", pluginInput.Action, out.GetStatus(), out.GetExitCode(), getExitCodeDescription(out.GetExitCode()), pluginInput.Source)
	}

	// Logging msiexec.Result
	log.Debug("logging stdouts & errors after setting final status for msiexec")
	log.Debugf("resultCode: %v", out.GetExitCode())
	log.Debugf("resultCode: %v", out.GetExitCode())
	log.Debugf("stdout: %v", out.GetStdout())
	log.Debugf("stderr: %v", out.GetStderr())
}

// processParams smartly divides the input parameter string into valid string blocks
func processParams(log log.T, str string) []string {

	// Sample transformation:
	// str = "/v value "some path" myproperty=value"
	// result: []string{"/v", "value", "some path", "myproperty=value"}

	// contains the last split location of the string
	lastbit := 0

	params := []string{}

	// true if first quote was encountered else false
	quoteInit := false

	// Iterate through each character in str
	for i, c := range str {

		// Look for quotes or spaces
		// By default we split a string using space as a delimiter
		// If a quote(") is encountered then wait for the next quote irrespective of any spaces in between
		if c == '"' {
			if quoteInit {
				quoteInit = false
				params = append(params, str[lastbit:i+1])
				lastbit = i + 1
			} else {
				quoteInit = true
				lastbit = i
			}
		} else if c == ' ' && !quoteInit {
			if lastbit != i {
				params = append(params, str[lastbit:i])
			}
			lastbit = i + 1
		}
	}

	// This handles the last word in str
	if lastbit < len(str) {
		params = append(params, str[lastbit:])
	}

	log.Debug("Parameters after processing...")
	for _, param := range params {
		log.Debug(param)
	}

	return params
}

// Using https://msdn.microsoft.com/en-us/library/aa376931(v=vs.85).aspx as a reference, this returns
// description of error codes regarding msi-exec.
// getExitCodeDescription returns the description for errorCode
func getExitCodeDescription(errorCode int) string {

	// NOTE: The error description is in the format of ErrorMsg:{message}; Description:{description};
	switch errorCode {
	case 0:
		return "ErrorMsg:{ERROR_SUCCESS}; Description:{The action completed successfully.};"
	case 13:
		return "ErrorMsg:{ERROR_INVALID_DATA}; Description:{The data is invalid.};"
	case 87:
		return "ErrorMsg:{ERROR_INVALID_PARAMETER}; Description:{One of the parameters was invalid.};"
	case 120:
		return "ErrorMsg:{ERROR_CALL_NOT_IMPLEMENTED}; Description:{This value is returned when a custom action attempts to call a function that cannot be called from custom actions. The function returns the value ERROR_CALL_NOT_IMPLEMENTED. Available beginning with Windows Installer version 3.0.};"
	case 1259:
		// this might not be used afterall since we do /quiet install
		return "ErrorMsg:{ERROR_APPHELP_BLOCK}; Description:{If Windows Installer determines a product may be incompatible with the current operating system, it displays a dialog box informing the user and asking whether to try to install anyway. This error code is returned if the user chooses not to try the installation.};"
	case 1601:
		return "ErrorMsg:{ERROR_INSTALL_SERVICE_FAILURE}; Description:{The Windows Installer service could not be accessed. Contact your support personnel to verify that the Windows Installer service is properly registered.};"
	case 1602:
		return "ErrorMsg:{ERROR_INSTALL_USEREXIT}; Description:{The user cancels installation.};"
	case 1603:
		return "ErrorMsg:{ERROR_INSTALL_FAILURE}; Description:{A fatal error occurred during installation.};"
	case 1604:
		return "ErrorMsg:{ERROR_INSTALL_SUSPEND}; Description:{Installation suspended, incomplete.};"
	case 1605:
		return "ErrorMsg:{ERROR_UNKNOWN_PRODUCT}; Description:{This action is only valid for products that are currently installed.};"
	case 1606:
		return "ErrorMsg:{ERROR_UNKNOWN_FEATURE}; Description:{The feature identifier is not registered.};"
	case 1607:
		return "ErrorMsg:{ERROR_UNKNOWN_COMPONENT}; Description:{The component identifier is not registered.};"
	case 1608:
		return "ErrorMsg:{ERROR_INSTALL_SUSPEND}; Description:{Installation suspended, incomplete.};"
	case 1609:
		return "ErrorMsg:{ERROR_INVALID_HANDLE_STATE}; Description: The handle is in an invalid state.};"
	case 1610:
		return "ErrorMsg:{ERROR_BAD_CONFIGURATION}; Description:{The configuration data for this product is corrupt. Contact your support personnel.};"
	case 1611:
		return "ErrorMsg:{ERROR_INDEX_ABSENT}; Descriptio{: The component qualifier not present.};"
	case 1612:
		return "ErrorMsg:{ERROR_INSTALL_SOURCE_ABSENT}; Description:{The installation source for this product is not available. Verify that the source exists and that you can access it.};"
	case 1613:
		return "ErrorMsg:{ERROR_INSTALL_PACKAGE_VERSION}; Description:{This installation package cannot be installed by the Windows Installer service. You must install a Windows service pack that contains a newer version of the Windows Installer service.};"
	case 1614:
		return "ErrorMsg:{ERROR_PRODUCT_UNINSTALLED}; Description:{The product is uninstalled.};"
	case 1615:
		return "ErrorMsg:{ERROR_BAD_QUERY_SYNTAX}; Description:{The SQL query syntax is invalid or unsupported.};"
	case 1616:
		return "ErrorMsg:{ERROR_INVALID_FIELD}; Description:{The record field does not exist.};"
	case 1618:
		return "ErrorMsg:{ERROR_INSTALL_ALREADY_RUNNING}; Description:{Another installation is already in progress. Complete that installation before proceeding with this install.};"
	case 1619:
		return "ErrorMsg:{ERROR_INSTALL_PACKAGE_OPEN_FAILED}; Description:{This installation package could not be opened. Verify that the package exists and is accessible, or contact the application vendor to verify that this is a valid Windows Installer package.};"
	case 1620:
		return "ErrorMsg:{ERROR_INSTALL_PACKAGE_INVALID}; Description:{This installation package could not be opened. Contact the application vendor to verify that this is a valid Windows Installer package.};"
	case 1621:
		// this should not be ever used - because we do /quiet installation of msi
		return "ErrorMsg:{ERROR_INSTALL_UI_FAILURE}; Description:{There was an error starting the Windows Installer service user interface. Contact your support personnel.};"
	case 1622:
		return "ErrorMsg:{ERROR_INSTALL_LOG_FAILURE}; Description:{There was an error opening installation log file. Verify that the specified log file location exists and is writable.};"
	case 1623:
		return "ErrorMsg:{ERROR_INSTALL_LANGUAGE_UNSUPPORTED}; Description:{This language of this installation package is not supported by your system.};"
	case 1624:
		return "ErrorMsg:{ERROR_INSTALL_TRANSFORM_FAILURE}; Description:{There was an error applying transforms. Verify that the specified transform paths are valid.};"
	case 1625:
		return "ErrorMsg:{ERROR_INSTALL_PACKAGE_REJECTED}; Description:{This installation is forbidden by system policy. Contact your system administrator.};"
	case 1626:
		return "ErrorMsg:{ERROR_FUNCTION_NOT_CALLED}; Description:{The function could not be executed.};"
	case 1627:
		return "ErrorMsg:{ERROR_FUNCTION_FAILED}; Description:{The function failed during execution.};"
	case 1628:
		return "ErrorMsg:{ERROR_INVALID_TABLE}; Description:{The function failed during execution.};"
	case 1629:
		return "ErrorMsg:{ERROR_DATATYPE_MISMATCH}; Description: The data supplied is the wrong type.};"
	case 1630:
		return "ErrorMsg:{ERROR_UNSUPPORTED_TYPE}; Description:{Data of this type is not supported.};"
	case 1631:
		return "ErrorMsg:{ERROR_CREATE_FAILED}; Description:{The Windows Installer service failed to start. Contact your support personnel.};"
	case 1632:
		return "ErrorMsg:{ERROR_INSTALL_TEMP_UNWRITABLE}; Description:{The Temp folder is either full or inaccessible. Verify that the Temp folder exists and that you can write to it.};"
	case 1633:
		return "ErrorMsg:{ERROR_INSTALL_PLATFORM_UNSUPPORTED}; Description:{This installation package is not supported on this platform. Contact your application vendor.};"
	case 1634:
		return "ErrorMsg:{ERROR_INSTALL_NOTUSED}; Description:{Component is not used on this machine.};"
	case 1635:
		return "ErrorMsg:{ERROR_PATCH_PACKAGE_OPEN_FAILED}; Description:{This patch package could not be opened. Verify that the patch package exists and is accessible, or contact the application vendor to verify that this is a valid Windows Installer patch package.};"
	case 1636:
		return "ErrorMsg:{ERROR_PATCH_PACKAGE_INVALID}; Description:{This patch package could not be opened. Contact the application vendor to verify that this is a valid Windows Installer patch package.};"
	case 1637:
		return "ErrorMsg:{ERROR_PATCH_PACKAGE_UNSUPPORTED}; Description:{This patch package cannot be processed by the Windows Installer service. You must install a Windows service pack that contains a newer version of the Windows Installer service.};"
	case 1638:
		return "ErrorMsg:{ERROR_PRODUCT_VERSION}; Description:{Another version of this product is already installed. Installation of this version cannot continue. To configure or remove the existing version of this product, use Add/Remove Programs in Control Panel.};"
	case 1639:
		return "ErrorMsg:{ERROR_INVALID_COMMAND_LINE}; Description:{Invalid command line argument. Consult the Windows Installer SDK for detailed command-line help.};"
	case 1640:
		return "ErrorMsg:{ERROR_INSTALL_REMOTE_DISALLOWED}; Description:{The current user is not permitted to perform installations from a client session of a server running the Terminal Server role service.};"
	case 1641:
		return "ErrorMsg:{ERROR_SUCCESS_REBOOT_INITIATED}; Description:{The installer has initiated a restart. This message is indicative of a success.};"
	case 1642:
		return "ErrorMsg:{ERROR_PATCH_TARGET_NOT_FOUND}; Description:{The installer cannot install the upgrade patch because the program being upgraded may be missing or the upgrade patch updates a different version of the program. Verify that the program to be upgraded exists on your computer and that you have the correct upgrade patch.};"
	case 1643:
		return "ErrorMsg:{ERROR_PATCH_PACKAGE_REJECTED}; Description:{The patch package is not permitted by system policy.};"
	case 1644:
		return "ErrorMsg:{ERROR_INSTALL_TRANSFORM_REJECTED}; Description:{One or more customizations are not permitted by system policy.};"
	case 1645:
		return "ErrorMsg:{ERROR_INSTALL_REMOTE_PROHIBITED}; Description:{Windows Installer does not permit installation from a Remote Desktop Connection.};"
	case 1646:
		return "ErrorMsg:{ERROR_PATCH_REMOVAL_UNSUPPORTED}; Description:{The patch package is not a removable patch package. Available beginning with Windows Installer version 3.0.};"
	case 1647:
		return "ErrorMsg:{ERROR_UNKNOWN_PATCH}; Description:{The patch is not applied to this product. Available beginning with Windows Installer version 3.0.};"
	case 1648:
		return "ErrorMsg:{ERROR_PATCH_NO_SEQUENCE}; Description:{No valid sequence could be found for the set of patches. Available beginning with Windows Installer version 3.0.};"
	case 1649:
		return "ErrorMsg:{ERROR_PATCH_REMOVAL_DISALLOWED}; Description:{Patch removal was disallowed by policy. Available beginning with Windows Installer version 3.0.};"
	case 1650:
		return "ErrorMsg:{ERROR_INVALID_PATCH_XML}; Description:{The XML patch data is invalid. Available beginning with Windows Installer version 3.0.};"
	case 1651:
		return "ErrorMsg:{ERROR_PATCH_MANAGED_ADVERTISED_PRODUCT}; Description:{Administrative user failed to apply patch for a per-user managed or a per-machine application that is in advertise state. Available beginning with Windows Installer version 3.0.};"
	case 1652:
		return "ErrorMss:{ERROR_INSTALL_SERVICE_SAFEBOOT}; Description:{Windows Installer is not accessible when the computer is in Safe Mode. Exit Safe Mode and try again or try using System Restore to return your computer to a previous state. Available beginning with Windows Installer version 4.0.};"
	case 1653:
		return "ErrorMsg:{ERROR_ROLLBACK_DISABLED}; Description:{Could not perform a multiple-package transaction because rollback has been disabled. Multiple-Package Installations cannot run if rollback is disabled. Available beginning with Windows Installer version 4.5.};"
	case 1654:
		return "ErrorMsg:{ERROR_INSTALL_REJECTED}; Description:{The app that you are trying to run is not supported on this version of Windows. A Windows Installer package, patch, or transform that has not been signed by Microsoft cannot be installed on an ARM computer.};"
	case 3010:
		return "ErrorMsg:{ERROR_SUCCESS_REBOOT_REQUIRED}; Description:{A restart is required to complete the install. This message is indicative of a success. This does not include installs where the ForceReboot action is run.};"
	default:
		return fmt.Sprintf("ErrorCode: %v; Description: Unknown", errorCode)
	}
}
