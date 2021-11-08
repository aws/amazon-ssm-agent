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
	"strings"

	"github.com/aws/amazon-ssm-agent/agent/setupcli/managers/common"
	"github.com/aws/amazon-ssm-agent/agent/setupcli/managers/servicemanagers"
)

type dpkgManager struct {
	managerHelper common.IManagerHelper
}

const debFile = "amazon-ssm-agent.deb"

func (m *dpkgManager) GetFilesReqForInstall() []string {
	return []string{
		debFile,
	}
}

func (m *dpkgManager) InstallAgent(folderPath string) error {
	debPath := filepath.Join(folderPath, debFile)

	output, err := m.managerHelper.RunCommand("dpkg", "-i", debPath)
	if err != nil {
		if m.managerHelper.IsTimeoutError(err) {
			return fmt.Errorf("dpkg install: Command timed out")
		}

		return fmt.Errorf("dpkg install: Failed to install deb package with output '%s' and error: %v", output, err)
	}

	return nil
}

func (m *dpkgManager) UninstallAgent() error {
	output, err := m.managerHelper.RunCommand("dpkg", "-P", "amazon-ssm-agent")

	if err != nil {
		if m.managerHelper.IsTimeoutError(err) {
			return fmt.Errorf("dpkg uninstall: Command timed out")
		}

		return fmt.Errorf("dpkg uninstall: failed to uninstall agent with output '%s' and error: %v", output, err)
	}

	return nil
}

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
		if exitCode == packageNotInstalledExitCode {
			return false, nil
		}

		return false, fmt.Errorf("dpkg isInstalled: Unexpected exit code, output '%s' and exit code: %v", output, exitCode)
	}

	if m.managerHelper.IsTimeoutError(err) {
		return false, fmt.Errorf("dpkg isInstalled: Command timed out")
	}

	return false, fmt.Errorf("dpkg isInstalled: Unexpected error with output '%s' and error: %v", output, err)
}

func (m *dpkgManager) IsManagerEnvironment() bool {
	return m.managerHelper.IsCommandAvailable("dpkg")
}

func (m *dpkgManager) GetName() string {
	return "dpkg"
}

func (m *dpkgManager) GetSupportedServiceManagers() []servicemanagers.ServiceManager {
	return []servicemanagers.ServiceManager{servicemanagers.SystemCtl, servicemanagers.Upstart}
}

func (m *dpkgManager) GetType() PackageManager {
	return Dpkg
}
