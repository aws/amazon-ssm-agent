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

package packagemanagers

import (
	"fmt"
	"path/filepath"

	"github.com/aws/amazon-ssm-agent/agent/setupcli/managers/common"
	"github.com/aws/amazon-ssm-agent/agent/setupcli/managers/servicemanagers"
)

type snapManager struct {
	managerHelper common.IManagerHelper
}

const (
	assertFile = "amazon-ssm-agent.assert"
	snapFile   = "amazon-ssm-agent.snap"
)

func (m *snapManager) GetFilesReqForInstall() []string {
	return []string{
		assertFile,
		snapFile,
	}
}

func (m *snapManager) InstallAgent(folderPath string) error {
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
		return fmt.Errorf("snap install: Failed to install snap with output '%s' and error: %v", output, err)
	}

	return nil
}

func (m *snapManager) UninstallAgent() error {
	output, err := m.managerHelper.RunCommand("snap", "remove", "amazon-ssm-agent")

	if err != nil {
		if m.managerHelper.IsTimeoutError(err) {
			return fmt.Errorf("snap uninstall: Command timed out")
		}

		return fmt.Errorf("snap uninstall: Failed with output '%s' and error: %v", output, err)
	}

	return nil
}

func (m *snapManager) IsAgentInstalled() (bool, error) {
	output, err := m.managerHelper.RunCommand("snap", "list", "amazon-ssm-agent")

	if err == nil {
		return true, nil
	}

	if m.managerHelper.IsExitCodeError(err) {
		exitCode := m.managerHelper.GetExitCode(err)
		if exitCode == packageNotInstalledExitCode {
			return false, nil
		}

		return false, fmt.Errorf("snap isInstalled: Unexpected exit code with output '%s' and exit code: %v", output, exitCode)
	}

	if m.managerHelper.IsTimeoutError(err) {
		return false, fmt.Errorf("snap isInstalled: Command timed out")
	}

	return false, fmt.Errorf("snap isInstalled: Unexpected error with output '%s' and error: %v", output, err)
}

func (m *snapManager) IsManagerEnvironment() bool {
	return m.managerHelper.IsCommandAvailable("snap") && m.managerHelper.IsCommandAvailable("systemctl")
}

func (m *snapManager) GetName() string {
	return "snap"
}

func (m *snapManager) GetSupportedServiceManagers() []servicemanagers.ServiceManager {
	return []servicemanagers.ServiceManager{servicemanagers.Snap}
}

func (m *snapManager) GetType() PackageManager {
	return Snap
}
