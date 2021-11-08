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

package registermanager

import (
	"fmt"

	"github.com/aws/amazon-ssm-agent/agent/setupcli/managers/common"
)

func getAgentBinaryPath() string {
	for _, path := range possibleAgentPaths {
		pathExists, err := common.FileExists(path)
		if err != nil {
			// failed to get status
			continue
		} else if pathExists {
			return path
		}
	}

	return ""
}

type registerManager struct {
	managerHelper common.IManagerHelper
	agentBinPath  string
}

func (m *registerManager) RegisterAgent(region, role, tags string) error {
	var err error
	var output string

	if m.agentBinPath == "" {
		return fmt.Errorf("unable to determine path of amazon-ssm-agent executable")
	}

	if tags == "" {
		output, err = m.managerHelper.RunCommand(m.agentBinPath, "-register", "-y", "-region", region, "-role", role)
	} else {
		output, err = m.managerHelper.RunCommand(m.agentBinPath, "-register", "-y", "-region", region, "-role", role, "-tags", tags)
	}

	if err != nil {
		if m.managerHelper.IsExitCodeError(err) {
			return fmt.Errorf("failed with output: %s", output)
		} else if m.managerHelper.IsTimeoutError(err) {
			return fmt.Errorf("timed out with output: %s", output)
		}

		return fmt.Errorf("unexpected error: %v, output was: %s", err, output)
	}

	return nil
}

// New creates new register manager
func New() *registerManager {
	return &registerManager{
		&common.ManagerHelper{},
		getAgentBinaryPath(),
	}
}
