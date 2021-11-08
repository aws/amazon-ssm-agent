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
	"github.com/aws/amazon-ssm-agent/agent/setupcli/managers/common"
)

type IServiceManager interface {
	// StartAgent starts the agent
	StartAgent() error
	// StopAgent stops the agent
	StopAgent() error
	// GetAgentStatus returns the status of the agent from the perspective of the service manager
	GetAgentStatus() (common.AgentStatus, error)
	// ReloadManager reloads the service manager configuration files
	ReloadManager() error
	// IsManagerEnvironment returns true if all commands required by the package manager are available
	IsManagerEnvironment() bool
	// GetName returns the service manager name
	GetName() string
	// GetType returns the service manage type
	GetType() ServiceManager
}
