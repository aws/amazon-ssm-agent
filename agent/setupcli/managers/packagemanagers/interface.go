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

import "github.com/aws/amazon-ssm-agent/agent/setupcli/managers/servicemanagers"

type IPackageManager interface {
	// InstallAgent installs the agent using package manager, folderPath should contain all files required for installation
	InstallAgent(folderPath string) error
	// GetFilesReqForInstall returns all the files the package manager needs to install the agent
	GetFilesReqForInstall() []string
	// UninstallAgent uninstalls the agent using the package manager
	UninstallAgent() error
	// IsAgentInstalled returns true if agent is installed using package manager, returns error for any unexpected errors
	IsAgentInstalled() (bool, error)
	// IsManagerEnvironment returns true if all commands required by the package manager are available
	IsManagerEnvironment() bool
	// GetName returns the package manager name
	GetName() string
	// GetType returns the package manager type
	GetType() PackageManager
	// GetSupportedServiceManagers returns all the service manager types that the package manager supports
	GetSupportedServiceManagers() []servicemanagers.ServiceManager
}
