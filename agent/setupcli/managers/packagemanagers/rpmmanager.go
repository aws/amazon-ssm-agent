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

//go:build freebsd || linux || netbsd || openbsd
// +build freebsd linux netbsd openbsd

package packagemanagers

import (
	"fmt"
	"path/filepath"

	"github.com/aws/amazon-ssm-agent/agent/setupcli/utility"

	"github.com/aws/amazon-ssm-agent/agent/log"

	"github.com/aws/amazon-ssm-agent/agent/setupcli/managers/common"
	"github.com/aws/amazon-ssm-agent/agent/setupcli/managers/servicemanagers"
	"github.com/aws/amazon-ssm-agent/agent/setupcli/managers/verificationmanagers"
)

type rpmManager struct {
	managerHelper common.IManagerHelper
}

const rpmFile = "amazon-ssm-agent.rpm"

func (m *rpmManager) GetFilesReqForInstall(log log.T) []string {
	return []string{
		rpmFile,
	}
}

func (m *rpmManager) InstallAgent(log log.T, folderPath string) error {
	rpmPath := filepath.Join(folderPath, rpmFile)
	// Using update flag since if agent is installed this flag is required
	// If agent is not installed, rpm will just install the agent
	output, err := m.managerHelper.RunCommand("rpm", "-U", rpmPath)
	if err != nil {
		if m.managerHelper.IsTimeoutError(err) {
			return fmt.Errorf("rpm install: Command timed out")
		}
		return fmt.Errorf("rpm install: Failed with output '%s' and error: %v", output, err)
	}
	return nil
}

func (m *rpmManager) UninstallAgent(log log.T, installedAgentVersionPath string) error {
	output, err := m.managerHelper.RunCommand("rpm", "-e", "amazon-ssm-agent")
	if err != nil {
		if m.managerHelper.IsTimeoutError(err) {
			return fmt.Errorf("rpm uninstall: Command timed out")
		}

		return fmt.Errorf("rpm uninstall: Failed to uninstall agent with output '%s' and error: %v", output, err)
	}
	return nil
}

func (m *rpmManager) IsAgentInstalled() (bool, error) {
	output, err := m.managerHelper.RunCommand("rpm", "-q", "amazon-ssm-agent")

	if err == nil {
		return true, nil
	}

	if m.managerHelper.IsExitCodeError(err) {
		exitCode := m.managerHelper.GetExitCode(err)
		if exitCode == common.PackageNotInstalledExitCode {
			return false, nil
		}

		return false, fmt.Errorf("rpm isInstalled: Unexpected exit code, output '%s' and exit code: %v", output, exitCode)
	}

	if m.managerHelper.IsTimeoutError(err) {
		return false, fmt.Errorf("rpm isInstalled: Command timed out")
	}

	return false, fmt.Errorf("rpm isInstalled: Unexpected error with output '%s' and error: %v", output, err)
}

func (m *rpmManager) GetInstalledAgentVersion() (string, error) {
	output, err := m.managerHelper.RunCommand("rpm", "-q", "--qf", "%{VERSION}", "amazon-ssm-agent")
	if err == nil {
		return utility.CleanupVersion(output), nil
	}

	if m.managerHelper.IsExitCodeError(err) {
		exitCode := m.managerHelper.GetExitCode(err)
		if exitCode == common.PackageNotInstalledExitCode {
			return "", fmt.Errorf("agent not installed with rpm")
		}
		return "", fmt.Errorf("rpm getVersion: Unexpected exit code, output '%s' and exit code: %v", output, exitCode)
	}

	if m.managerHelper.IsTimeoutError(err) {
		return "", fmt.Errorf("rpm getVersion: Command timed out")
	}

	return "", fmt.Errorf("rpm getVersion: Unexpected error with output '%s' and error: %v", output, err)
}

func (m *rpmManager) IsManagerEnvironment() bool {
	return m.managerHelper.IsCommandAvailable("rpm")
}

func (m *rpmManager) GetSupportedServiceManagers() []servicemanagers.ServiceManager {
	return []servicemanagers.ServiceManager{servicemanagers.SystemCtl, servicemanagers.Upstart}
}

func (m *rpmManager) GetName() string {
	return "rpm"
}

func (m *rpmManager) GetType() PackageManager {
	return Rpm
}

func (m *rpmManager) GetFileExtension() string {
	return ".rpm"
}

func (m *rpmManager) GetSupportedVerificationManager() verificationmanagers.VerificationManager {
	return verificationmanagers.Linux
}
