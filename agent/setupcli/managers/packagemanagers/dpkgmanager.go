// Copyright 2023 Amazon.com, Inc. or its affiliates. All Rights Reserved.
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

//go:build freebsd || linux || netbsd || openbsd
// +build freebsd linux netbsd openbsd

// Package packagemanagers holds functions querying using local package manager
package packagemanagers

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/amazon-ssm-agent/agent/setupcli/managers/common"
	"github.com/aws/amazon-ssm-agent/agent/setupcli/managers/servicemanagers"
	"github.com/aws/amazon-ssm-agent/agent/setupcli/managers/verificationmanagers"
	"github.com/aws/amazon-ssm-agent/agent/setupcli/utility"
)

type dpkgManager struct {
	managerHelper common.IManagerHelper
}

const debFile = "amazon-ssm-agent.deb"

// GetFilesReqForInstall returns all the files the package manager needs to install the agent
func (m *dpkgManager) GetFilesReqForInstall(log log.T) []string {
	return []string{
		debFile,
	}
}

// InstallAgent installs the agent using package manager, folderPath should contain all files required for installation
func (m *dpkgManager) InstallAgent(log log.T, folderPath string) error {
	debPath := filepath.Join(folderPath, debFile)
	log.Infof("Debian file path: %v", debPath)
	// 32-bit arm with 256 mb memory took 36 seconds to install, using 1 minute timeout
	output, err := m.managerHelper.RunCommandWithCustomTimeout(time.Minute, "dpkg", "-i", debPath)
	if err != nil {
		if m.managerHelper.IsTimeoutError(err) {
			return fmt.Errorf("dpkg install: Command timed out")
		}
		return fmt.Errorf("dpkg install: Failed to install deb package with output '%v' and error: %v", output, err)
	}
	return nil
}

// UninstallAgent uninstalls the agent using the package manager
func (m *dpkgManager) UninstallAgent(log log.T, installedAgentVersionPath string) error {
	output, err := m.managerHelper.RunCommand("dpkg", "-P", "amazon-ssm-agent")

	if err != nil {
		if m.managerHelper.IsTimeoutError(err) {
			return fmt.Errorf("dpkg uninstall: Command timed out")
		}

		return fmt.Errorf("dpkg uninstall: failed to uninstall agent with output '%s' and error: %v", output, err)
	}

	return nil
}

// IsAgentInstalled returns true if agent is installed using package manager, returns error for any unexpected errors
func (m *dpkgManager) IsAgentInstalled() (bool, error) {
	output, err := m.managerHelper.RunCommand("dpkg", "-s", "amazon-ssm-agent")
	if err == nil {
		for _, line := range strings.Split(strings.TrimSpace(output), "\n") {
			line = strings.TrimSpace(line)
			if strings.HasPrefix(line, "Status: ") {
				status := strings.TrimSpace(strings.Split(line, "Status: ")[1])
				if strings.Contains(status, "install ok installed") {
					return true, nil
				} else if strings.Contains(status, "deinstall ok config-files") {
					return false, nil
				}
				return false, fmt.Errorf("unexpected status output from dpkg -s: %s", status)
			}
		}
	}

	if m.managerHelper.IsExitCodeError(err) {
		exitCode := m.managerHelper.GetExitCode(err)
		if exitCode == common.PackageNotInstalledExitCode {
			return false, nil
		}
		return false, fmt.Errorf("dpkg isInstalled: Unexpected exit code, output '%s' and exit code: %v", output, exitCode)
	}
	if m.managerHelper.IsTimeoutError(err) {
		return false, fmt.Errorf("dpkg isInstalled: Command timed out")
	}
	return false, fmt.Errorf("dpkg isInstalled: Unexpected error with output '%s' and error: %v", output, err)
}

// IsManagerEnvironment returns true if all commands required by the package manager are available
func (m *dpkgManager) IsManagerEnvironment() bool {
	return m.managerHelper.IsCommandAvailable("dpkg")
}

// GetName returns the package manager name
func (m *dpkgManager) GetName() string {
	return "dpkg"
}

// GetSupportedServiceManagers returns all the service manager types that the package manager supports
func (m *dpkgManager) GetSupportedServiceManagers() []servicemanagers.ServiceManager {
	return []servicemanagers.ServiceManager{servicemanagers.SystemCtl, servicemanagers.Upstart}
}

// GetType returns the package manager type
func (m *dpkgManager) GetType() PackageManager {
	return Dpkg
}

// GetSupportedVerificationManager returns verification manager types that the package manager supports
func (m *dpkgManager) GetSupportedVerificationManager() verificationmanagers.VerificationManager {
	return verificationmanagers.Linux
}

// GetFileExtension returns the file extension of the agent using the package manager
func (m *dpkgManager) GetFileExtension() string {
	return ".deb"
}

// GetInstalledAgentVersion returns the version of the installed agent
func (m *dpkgManager) GetInstalledAgentVersion() (string, error) {
	// Because packages are available in dpkg unless it is removed with --purge we need to check if agent is installed
	isInstalled, err := m.IsAgentInstalled()
	if !isInstalled || err != nil {
		return "", fmt.Errorf("agent is not installed with dpkg: %v", err)
	}
	output, err := m.managerHelper.RunCommand("amazon-ssm-agent", "--version")
	versionCleanup := utility.CleanupVersion(output)
	if err == nil {
		return versionCleanup, nil
	}
	if versionCleanup != "" {
		return versionCleanup, nil
	}
	output, err = m.managerHelper.RunCommand("dpkg-query", "-f=${version}", "-W", "amazon-ssm-agent")
	if err == nil {
		return utility.CleanupVersion(output), nil
	}

	if m.managerHelper.IsExitCodeError(err) {
		exitCode := m.managerHelper.GetExitCode(err)
		return "", fmt.Errorf("dpkg getVersion: Unexpected exit code, output '%v' and exit code: %v", output, exitCode)
	}

	if m.managerHelper.IsTimeoutError(err) {
		return "", fmt.Errorf("dpkg getVersion: Command timed out")
	}

	return "", fmt.Errorf("dpkg getVersion: Unexpected error with output '%s' and error: %v", output, err)
}
