// Copyright 2016 Amazon.com, Inc. or its affiliates. All Rights Reserved.
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

// Package coremodules contains a list of implemented core modules.
package coremodules

import (
	"github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/aws/amazon-ssm-agent/agent/contracts"
	"github.com/aws/amazon-ssm-agent/agent/health"
	"github.com/aws/amazon-ssm-agent/agent/longrunning/manager"
	"github.com/aws/amazon-ssm-agent/agent/messageservice"
	"github.com/aws/amazon-ssm-agent/agent/runcommand"
	"github.com/aws/amazon-ssm-agent/agent/ssm"
)

// ModuleRegistry stores a set of core modules.
type ModuleRegistry []contracts.ICoreModuleWrapper

// registeredCoreModules stores the registered core modules.
var registeredCoreModules ModuleRegistry

// RegisteredCoreModules returns all registered core modules.
func RegisteredCoreModules(context context.T) *ModuleRegistry {
	if registeredCoreModules == nil {
		loadCoreModules(context)
	}
	return &registeredCoreModules
}

// register core modules here
func loadCoreModules(context context.T) {
	if !context.AppConfig().Agent.ContainerMode {
		registeredCoreModules = append(registeredCoreModules, NewCoreModuleWrapper(context.Log(), health.NewHealthCheck(context, ssm.NewService(context))))
	}

	messageServiceCoreModule := messageservice.NewService(context)
	if messageServiceCoreModule != nil {
		registeredCoreModules = append(registeredCoreModules, NewCoreModuleWrapper(context.Log(), messageServiceCoreModule))
	}

	if !context.AppConfig().Agent.ContainerMode {
		if offlineProcessor, err := runcommand.NewOfflineService(context); err == nil {
			registeredCoreModules = append(registeredCoreModules, NewCoreModuleWrapper(context.Log(), offlineProcessor))
		} else {
			context.Log().Errorf("Failed to start offline command document processor")
		}

		// registering the long running plugin manager as a core module
		manager.EnsureInitialization(context)
		if lrpm, err := manager.GetInstance(); err == nil {
			registeredCoreModules = append(registeredCoreModules, NewCoreModuleWrapper(context.Log(), lrpm))
		} else {
			context.Log().Errorf("Something went wrong during initialization of long running plugin manager")
		}
	}
}
