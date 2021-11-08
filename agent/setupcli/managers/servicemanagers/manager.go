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

	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/amazon-ssm-agent/agent/setupcli/managers/common"
)

const numRetries = 4

type ServiceManager int

const (
	Undefined ServiceManager = iota
	Snap
	SystemCtl
	Upstart
)

var serviceManagers = map[ServiceManager]IServiceManager{}

func registerServiceManager(managerType ServiceManager, manager IServiceManager) {
	serviceManagers[managerType] = manager
}

// GetServiceManager returns the service manager instance for a specific service manager type
func GetServiceManager(managerType ServiceManager) (IServiceManager, bool) {
	manager, ok := serviceManagers[managerType]
	return manager, ok
}

// StopAgent attempts to stop the agent and verifies it is stopped using retries
func StopAgent(manager IServiceManager, log log.T) error {
	var status common.AgentStatus
	var err error
	log.Infof("Stopping agent using %s service manager", manager.GetName())
	for i := 1; i <= numRetries; i++ {
		if err = manager.StopAgent(); err != nil {
			log.Warnf("attempt %v/%v failed to stop agent: %v", i, numRetries, err)
			continue
		}

		if status, err = manager.GetAgentStatus(); err != nil {
			log.Warnf("attempt %v/%v failed to get agent status after stopping: %v", i, numRetries, err)
			continue
		} else if status == common.Stopped {
			log.Info("Successfully stopped agent")
			return nil
		}

		log.Infof("attempt %v/%v: agent status was %v when expected status was %v", status, common.Stopped)
	}

	return fmt.Errorf("retries exhausted")
}

// StartAgent attempts to start the agent and verifies it is running using retries
func StartAgent(manager IServiceManager, log log.T) error {
	var status common.AgentStatus
	var err error
	log.Infof("Starting agent using %s service manager", manager.GetName())
	for i := 1; i <= numRetries; i++ {
		if err = manager.StartAgent(); err != nil {
			log.Warnf("attempt %v/%v failed to start agent: %v", i, numRetries, err)
			continue
		}

		if status, err = manager.GetAgentStatus(); err != nil {
			log.Warnf("attempt %v/%v failed to get agent status after starting: %v", i, numRetries, err)
			continue
		} else if status == common.Running {
			log.Info("Agent is running")
			return nil
		}

		log.Infof("attempt %v/%v: agent status was %v when expected status was %v", status, common.Running)
	}

	return fmt.Errorf("retries exhausted")
}
