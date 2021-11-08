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
	"strings"

	"github.com/aws/amazon-ssm-agent/agent/setupcli/managers/common"
)

type upstartManager struct {
	managerHelper common.IManagerHelper
}

func (m *upstartManager) StartAgent() error {
	output, err := m.managerHelper.RunCommand("start", "amazon-ssm-agent")
	if err != nil {
		return fmt.Errorf("upstart: failed to start agent with output '%s' and error: %v", output, err)
	}

	return nil
}

func (m *upstartManager) StopAgent() error {
	output, err := m.managerHelper.RunCommand("stop", "amazon-ssm-agent")
	if err != nil {
		return fmt.Errorf("upstart: failed to stop agent with output '%s' and error: %v", output, err)
	}

	return nil
}

func (m *upstartManager) GetAgentStatus() (common.AgentStatus, error) {
	output, err := m.managerHelper.RunCommand("status", "amazon-ssm-agent")

	if err != nil {
		if m.managerHelper.IsExitCodeError(err) {
			exitCode := m.managerHelper.GetExitCode(err)
			if exitCode == upstartServiceNotFoundExitCode {
				return common.NotInstalled, nil
			}

			return common.UndefinedStatus, fmt.Errorf("upstart agentStatus: Unexpected exit code from upstart 'status' with output '%s' and exit code '%v'", output, exitCode)
		} else if m.managerHelper.IsTimeoutError(err) {
			return common.UndefinedStatus, fmt.Errorf("upstart agentStatus: 'status' command timed out")
		}
		return common.UndefinedStatus, fmt.Errorf("upstart agentStatus: Unexpected error from upstart 'status': %v", err)
	}

	// upstart returns exit code 0 when service is running or stopped
	if strings.Contains(output, "start/running") {
		// Agent is running as upstart service
		return common.Running, nil
	} else if strings.Contains(output, "stop/waiting") {
		return common.Stopped, nil
	}

	return common.UndefinedStatus, fmt.Errorf("upstart agentStatus: unexpected output from 'status': %v", output)
}

func (m *upstartManager) ReloadManager() error {
	output, err := m.managerHelper.RunCommand("initctl", "reload-configuration")

	if err != nil {
		return fmt.Errorf("upstart reload: Failed with output '%s' and error: %v", output, err)
	}

	return nil
}

func (m *upstartManager) IsManagerEnvironment() bool {
	return m.managerHelper.IsCommandAvailable("start") &&
		m.managerHelper.IsCommandAvailable("status") &&
		m.managerHelper.IsCommandAvailable("stop") &&
		m.managerHelper.IsCommandAvailable("initctl")
}

func (m *upstartManager) GetName() string {
	return "upstart"
}

func (m *upstartManager) GetType() ServiceManager {
	return Upstart
}
