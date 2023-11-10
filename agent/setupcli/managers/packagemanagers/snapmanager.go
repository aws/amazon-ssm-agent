// Copyright 2021 Amazon.com, Inc. or its affiliates. All Rights Reserved.
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

type snapManager struct {
	managerHelper common.IManagerHelper
}

const (
	assertFile                        = "amazon-ssm-agent.assert"
	snapFile                          = "amazon-ssm-agent.snap"
	snapAutoRefreshInProgressExitCode = 10
	snapAgentdir                      = "/snap/amazon-ssm-agent/current/amazon-ssm-agent"
)

var waitTimeInterval = 10 * time.Second

// GetFilesReqForInstall returns all the files the package manager needs to install the agent
func (m *snapManager) GetFilesReqForInstall(log log.T) []string {
	return []string{
		assertFile,
		snapFile,
	}
}

// InstallAgent installs the agent using package manager, folderPath should contain all files required for installation
func (m *snapManager) InstallAgent(log log.T, folderPath string) error {
	assertPath := filepath.Join(folderPath, assertFile)
	snapPath := filepath.Join(folderPath, snapFile)

	output, err := m.managerHelper.RunCommand("snap", "ack", assertPath)
	if err != nil {
		if m.managerHelper.IsTimeoutError(err) {
			return fmt.Errorf("snap ack: Command timed out")
		}
		return fmt.Errorf("snap install: Failed to ack assert file with output '%s' and error: %v", output, err)
	}

	output, err = m.managerHelper.RunCommand("snap", "install", snapPath, "--classic")
	if err != nil {
		if m.managerHelper.IsTimeoutError(err) {
			return fmt.Errorf("snap install: Command timed out")
		}

		if m.managerHelper.IsExitCodeError(err) && m.managerHelper.GetExitCode(err) == snapAutoRefreshInProgressExitCode {
			// Note: Greengrass install step has a default timeout of 120 seconds
			const maxAttempts = 6
			for i := 1; i < maxAttempts; i++ {
				output, err = m.managerHelper.RunCommand("snap", "install", snapPath, "--classic")
				if err == nil {
					return nil
				}

				if m.managerHelper.IsTimeoutError(err) {
					return fmt.Errorf("snap install: Command timed out")
				}

				isUpdateInProgressError := m.managerHelper.IsExitCodeError(err) && m.managerHelper.GetExitCode(err) == snapAutoRefreshInProgressExitCode
				if !isUpdateInProgressError {
					break
				}

				time.Sleep(waitTimeInterval)
			}

		}

		return fmt.Errorf("snap install: Failed to install snap with output '%s' and error: %v", output, err)
	}

	return nil
}

// UninstallAgent uninstalls the agent using the package manager
func (m *snapManager) UninstallAgent(log log.T, installedAgentVersionPath string) error {
	output, err := m.managerHelper.RunCommand("snap", "remove", "amazon-ssm-agent")

	if err != nil {
		if m.managerHelper.IsTimeoutError(err) {
			return fmt.Errorf("snap uninstall: Command timed out")
		}

		if m.managerHelper.IsExitCodeError(err) && m.managerHelper.GetExitCode(err) == snapAutoRefreshInProgressExitCode {
			// Note: Greengrass install step has a default timeout of 120 seconds
			const maxAttempts = 5
			for i := 1; i < maxAttempts; i++ {
				output, err = m.managerHelper.RunCommand("snap", "remove", "amazon-ssm-agent")
				if err == nil {
					return nil
				}

				if m.managerHelper.IsTimeoutError(err) {
					return fmt.Errorf("snap uninstall: Command timed out")
				}

				isUpdateInProgressError := m.managerHelper.IsExitCodeError(err) && m.managerHelper.GetExitCode(err) == snapAutoRefreshInProgressExitCode
				if !isUpdateInProgressError {
					break
				}

				time.Sleep(waitTimeInterval)
			}

		}

		return fmt.Errorf("snap uninstall: Failed with output '%s' and error: %v", output, err)
	}

	return nil
}

// IsAgentInstalled returns true if agent is installed using package manager, returns error for any unexpected errors
func (m *snapManager) IsAgentInstalled() (bool, error) {
	output, err := m.managerHelper.RunCommand("snap", "list", "amazon-ssm-agent")

	if err == nil {
		return true, nil
	}

	if err != nil {
		if m.managerHelper.IsExitCodeError(err) {
			exitCode := m.managerHelper.GetExitCode(err)
			if exitCode == common.PackageNotInstalledExitCode {
				return false, nil
			}

			if exitCode == snapAutoRefreshInProgressExitCode {
				output, err := m.managerHelper.RunCommand(snapAgentdir, "--version")

				if err != nil {
					return false, fmt.Errorf("agent not installed with snap: %w", err)
				}

				if output != "" {
					return true, nil
				}
			}
			return false, fmt.Errorf("snap isInstalled: Unexpected exit code with output '%s' and exit code: %v", output, exitCode)
		}

		if m.managerHelper.IsTimeoutError(err) {
			return false, fmt.Errorf("snap isInstalled: Command timed out")
		}
	}

	return false, fmt.Errorf("snap isInstalled: Unexpected error with output '%s' and error: %w", output, err)
}

// GetInstalledAgentVersion returns the version of the installed agent
func (m *snapManager) GetInstalledAgentVersion() (string, error) {
	output, err := m.managerHelper.RunCommand("snap", "list", "amazon-ssm-agent")

	if err != nil {
		if m.managerHelper.IsExitCodeError(err) {
			exitCode := m.managerHelper.GetExitCode(err)
			if exitCode == common.PackageNotInstalledExitCode {
				return "", fmt.Errorf("agent not installed with snap")
			}

			if exitCode == snapAutoRefreshInProgressExitCode {
				output, err := m.managerHelper.RunCommand(snapAgentdir, "--version")

				if err != nil {
					return "", fmt.Errorf("agent not installed with snap: %w", err)
				}

				snapInfoVersionOutput := strings.Split(output, ":")
				if len(snapInfoVersionOutput) == 2 {
					return utility.CleanupVersion(strings.TrimSpace(snapInfoVersionOutput[len(snapInfoVersionOutput)-1])), nil
				}
			}

			return "", fmt.Errorf("snap getVersion: Unexpected exit code with output '%s' and exit code: %v", output, exitCode)
		}

		if m.managerHelper.IsTimeoutError(err) {
			return "", fmt.Errorf("snap getVersion: Command timed out")
		}

		return "", fmt.Errorf("snap getVersion: Unexpected error with output '%s' and error: %w", output, err)
	}

	snapInfoLines := strings.Split(strings.TrimSpace(output), "\n")
	if len(snapInfoLines) == 2 {
		headerFields := strings.Fields(snapInfoLines[0])
		agentFields := strings.Fields(snapInfoLines[1])
		for i, header := range headerFields {
			if header == "Version" {
				return utility.CleanupVersion(agentFields[i]), nil
			}
		}

		return "", fmt.Errorf("failed to extract agent version from snap info output")
	}

	return "", fmt.Errorf("failed to extract agent version because of unexpected output from snap info")
}

// IsManagerEnvironment returns true if all commands required by the package manager are available
func (m *snapManager) IsManagerEnvironment() bool {
	return m.managerHelper.IsCommandAvailable("snap") && m.managerHelper.IsCommandAvailable("systemctl")
}

// GetName returns the package manager name
func (m *snapManager) GetName() string {
	return "snap"
}

// GetSupportedServiceManagers returns all the service manager types that the package manager supports
func (m *snapManager) GetSupportedServiceManagers() []servicemanagers.ServiceManager {
	return []servicemanagers.ServiceManager{servicemanagers.Snap}
}

// GetType returns the package manager type
func (m *snapManager) GetType() PackageManager {
	return Snap
}

// GetSupportedVerificationManager returns verification manager types that the package manager supports
func (m *snapManager) GetSupportedVerificationManager() verificationmanagers.VerificationManager {
	return verificationmanagers.Skip
}

// GetFileExtension returns the file extension of the agent using the package manager
func (m *snapManager) GetFileExtension() string {
	return ".snap"
}
