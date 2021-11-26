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

package managers

import (
	"fmt"

	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/amazon-ssm-agent/agent/setupcli/managers/configurationmanager"
	"github.com/aws/amazon-ssm-agent/agent/setupcli/managers/packagemanagers"
	"github.com/aws/amazon-ssm-agent/agent/setupcli/managers/registermanager"
	"github.com/aws/amazon-ssm-agent/agent/setupcli/managers/servicemanagers"
)

var selectedServiceManagerCache = servicemanagers.Undefined
var selectedPackageManagerCache = packagemanagers.Undefined

var getServiceManager = servicemanagers.GetServiceManager
var getPackageManager = packagemanagers.GetPackageManager
var getAllPackageManagers = packagemanagers.GetAllPackageManagers

func setServiceManager(log log.T) error {
	if selectedPackageManagerCache == packagemanagers.Undefined {
		// Should never happen
		panic("Package manager must be selected first before service manager")
	}
	if selectedServiceManagerCache == servicemanagers.Undefined {
		pm, ok := getPackageManager(selectedPackageManagerCache)
		if !ok {
			// Should never happen
			panic("Tried to get service manager when selected package manager does not exist")
		}

		for _, managerType := range pm.GetSupportedServiceManagers() {
			if manager, ok := getServiceManager(managerType); !ok {
				panic(fmt.Sprintf("Failed to get service manager with index %v", managerType))
			} else if manager.IsManagerEnvironment() {
				log.Infof("Selecting %s as service manager", manager.GetName())
				selectedServiceManagerCache = managerType
				return nil
			} else {
				log.Infof("Not selecting %s as service manager", manager.GetName())
			}
		}
	}

	// If we are unable to find service manager we don't want to return error
	// because package managers can support one or more service managers
	return nil
}

func setPackageManager(log log.T) error {
	if selectedPackageManagerCache == packagemanagers.Undefined {
		allPackageManagers := getAllPackageManagers()

		// check if agent is already installed
		packageManagersNames := make([]string, 0, len(allPackageManagers))
		managerName := ""
		for _, manager := range allPackageManagers {
			packageManagersNames = append(packageManagersNames, manager.GetName())
			if manager.IsManagerEnvironment() {
				if selectedPackageManagerCache == packagemanagers.Undefined {
					// if cache is still unset, set to fist package manager that fits the environment
					selectedPackageManagerCache = manager.GetType()
					managerName = manager.GetName()
				}

				log.Debugf("Package manager %s is available, checking if agent is installed", manager.GetName())

				isInstalled, err := manager.IsAgentInstalled()
				if err != nil {
					log.Warnf("Failed to check if agent was installed using %s: %v", manager.GetName(), err)
					continue
				}

				if isInstalled {
					log.Infof("Agent is already installed with %s, selecting it as package manager", manager.GetName())
					selectedPackageManagerCache = manager.GetType()
					return nil
				}

				log.Debugf("Agent is not installed with %s", manager.GetName())
			}
		}

		if selectedPackageManagerCache == packagemanagers.Undefined {
			return fmt.Errorf("no supported package manager found in list: %v", packageManagersNames)
		} else {
			log.Infof("Selecting %s as package manager", managerName)
		}
	}

	return nil
}

// GetPackageManager returns the selected package manager, using cache if already selected
func GetPackageManager(log log.T) (packagemanagers.IPackageManager, error) {
	if err := setPackageManager(log); err != nil {
		return nil, err
	}

	manager, _ := getPackageManager(selectedPackageManagerCache)
	return manager, nil
}

// GetServiceManager returns the selected service manager, using cache if already selected.
// package manager must be selected first
func GetServiceManager(log log.T) (servicemanagers.IServiceManager, error) {
	if err := setServiceManager(log); err != nil {
		return nil, err
	}

	if manager, ok := getServiceManager(selectedServiceManagerCache); ok {
		return manager, nil
	}

	return nil, fmt.Errorf("unable to find service manager with index type %v", selectedServiceManagerCache)
}

// GetRegisterManager returns a new register manager
func GetRegisterManager() registermanager.IRegisterManager {
	return registermanager.New()
}

// GetConfigurationManager returns a new configuration manager
func GetConfigurationManager() configurationmanager.IConfigurationManager {
	return configurationmanager.New()
}
