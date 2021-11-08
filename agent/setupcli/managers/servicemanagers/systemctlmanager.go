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

package servicemanagers

import (
	"fmt"

	"github.com/aws/amazon-ssm-agent/agent/setupcli/managers/common"
)

type systemCtlManager struct {
	managerHelper common.IManagerHelper
	serviceName   string
	managerName   string
	managerType   ServiceManager

	dependentBinaries []string
}

func (m *systemCtlManager) StartAgent() error {
	output, err := m.managerHelper.RunCommand("systemctl", "start", m.serviceName)
	if err != nil {
		return fmt.Errorf("systemctl start: Failed to start agent with output '%s' and error: %v", output, err)
	}

	return nil
}

func (m *systemCtlManager) StopAgent() error {
	output, err := m.managerHelper.RunCommand("systemctl", "stop", m.serviceName)
	if err != nil {
		return fmt.Errorf("systemctl stop: Failed to start agent with output '%s' and error: %v", output, err)
	}

	return nil
}

func (m *systemCtlManager) GetAgentStatus() (common.AgentStatus, error) {
	_, err := m.managerHelper.RunCommand("systemctl", "is-active", m.serviceName)
	if err == nil {
		return common.Running, nil
	}

	// Fallback to status if is-active is not true
	output, err := m.managerHelper.RunCommand("systemctl", "status", m.serviceName)
	if err == nil {
		return common.Running, nil
	}

	if m.managerHelper.IsExitCodeError(err) {
		exitCode := m.managerHelper.GetExitCode(err)
		if exitCode == systemCtlServiceStoppedExitCode {
			return common.Stopped, nil
		} else if exitCode == systemCtlServiceNotFoundExitCode {
			return common.NotInstalled, nil
		}

		return common.UndefinedStatus, fmt.Errorf("systemctl agentStatus: Unexpected exit code with output '%s' and exit code: %v", output, exitCode)
	} else if m.managerHelper.IsTimeoutError(err) {
		return common.UndefinedStatus, fmt.Errorf("systemctl agentStatus: command timed out")
	}

	return common.UndefinedStatus, fmt.Errorf("systemctl agentStatus:  Unexpected error with output '%s' and error: %v", output, err)
}

func (m *systemCtlManager) ReloadManager() error {
	output, err := m.managerHelper.RunCommand("systemctl", "daemon-reload")

	if err != nil {
		return fmt.Errorf("systemctl reload: Failed with output '%s' and error: %v", output, err)
	}

	return nil
}

func (m *systemCtlManager) IsManagerEnvironment() bool {
	for _, cmd := range m.dependentBinaries {
		if !m.managerHelper.IsCommandAvailable(cmd) {
			return false
		}
	}
	return true
}

func (m *systemCtlManager) GetName() string {
	return m.managerName
}

func (m *systemCtlManager) GetType() ServiceManager {
	return m.managerType
}
