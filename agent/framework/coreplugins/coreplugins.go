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

// Package coreplugins contains a list of implemented core modules.
package coreplugins

import (
	"github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/aws/amazon-ssm-agent/agent/contracts"
	"github.com/aws/amazon-ssm-agent/agent/health"
	"github.com/aws/amazon-ssm-agent/agent/longrunning/manager"
	message "github.com/aws/amazon-ssm-agent/agent/message/processor"
	"github.com/aws/amazon-ssm-agent/agent/startup"
)

// ModuleRegistry stores a set of core modules.
type ModuleRegistry []contracts.ICoreModule

// registeredCorePlugins stores the registered core modules.
var registeredModules ModuleRegistry

// RegisteredCoreModules returns all registered core modules.
func RegisteredCoreModules(context context.T) *ModuleRegistry {
	if registeredModules == nil {
		loadCorePlugins(context)
	}
	return &registeredModules
}

// register core modules here
func loadCorePlugins(context context.T) {
	registeredModules = append(registeredModules, health.NewHealthCheck(context))
	registeredModules = append(registeredModules, message.NewMdsProcessor(context))

	if offlineProcessor, err := message.NewOfflineProcessor(context); err == nil {
		registeredModules = append(registeredModules, offlineProcessor)
	} else {
		context.Log().Errorf("Failed to start offline command document processor")
	}

	registeredModules = append(registeredModules, startup.NewProcessor(context))

	// registering the long running plugin manager as a core module
	manager.EnsureInitialization(context)
	if lrpm, err := manager.GetInstance(); err == nil {
		registeredModules = append(registeredModules, lrpm)
	} else {
		context.Log().Errorf("Something went wrong during initialization of long running plugin manager")
	}
}
