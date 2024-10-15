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

// Package registermanager contains functions related to register
package registermanager

import (
	"fmt"

	"github.com/aws/amazon-ssm-agent/agent/setupcli/managers/common"
	"github.com/aws/amazon-ssm-agent/agent/setupcli/utility"
)

var (
	utilFileExists = utility.FileExists
)

type registerManager struct {
	managerHelper common.IManagerHelper
	agentBinPath  string
}

// RegisterAgent registers SSM Agent with SSM
func (m *registerManager) RegisterAgent(registerAgentInpModel *RegisterAgentInputModel) error {
	var err error
	var output string

	if m.agentBinPath == "" {
		return fmt.Errorf("unable to determine path of amazon-ssm-agent executable")
	}

	if registerAgentInpModel.ActivationCode != "" || registerAgentInpModel.ActivationId != "" {
		if registerAgentInpModel.ActivationCode == "" {
			return fmt.Errorf("failed with empty activation code")
		}
		if registerAgentInpModel.ActivationId == "" {
			return fmt.Errorf("failed with empty activation id")
		}
		output, err = m.generateMIRegisterCommand(registerAgentInpModel)
	} else if registerAgentInpModel.Tags == "" {
		output, err = m.managerHelper.RunCommand(m.agentBinPath, "-register", "-y",
			"-region", registerAgentInpModel.Region,
			"-role", registerAgentInpModel.Role)
	} else {
		output, err = m.managerHelper.RunCommand(m.agentBinPath, "-register", "-y",
			"-region", registerAgentInpModel.Region,
			"-role", registerAgentInpModel.Role,
			"-tags", registerAgentInpModel.Tags)
	}

	if err != nil {
		if m.managerHelper.IsExitCodeError(err) {
			return fmt.Errorf("registration command failed with output: %s", output)
		} else if m.managerHelper.IsTimeoutError(err) {
			return fmt.Errorf("registration command timed out with output: %s", output)
		}

		return fmt.Errorf("unexpected error during agent registration: %v, output: %v", err, output)
	}

	return nil
}

// New creates new register manager
func New() *registerManager {
	return &registerManager{&common.ManagerHelper{}, getAgentBinaryPath()}
}

func getAgentBinaryPath() string {
	for _, path := range possibleAgentPaths {
		pathExists, err := utilFileExists(path)
		if err != nil {
			continue
		} else if pathExists {
			return path
		}
	}
	return ""
}

func (m *registerManager) generateMIRegisterCommand(registerAgentInpModel *RegisterAgentInputModel) (string, error) {
	return m.managerHelper.RunCommand(m.agentBinPath, "-register", "-y",
		"-region", registerAgentInpModel.Region,
		"-code", registerAgentInpModel.ActivationCode,
		"-id", registerAgentInpModel.ActivationId)
}
