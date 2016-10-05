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

// Package coreplugins contains a list of implemented core plugins.
package coreplugins

import (
	"github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/aws/amazon-ssm-agent/agent/contracts"
	"github.com/aws/amazon-ssm-agent/agent/health"
	message "github.com/aws/amazon-ssm-agent/agent/message/processor"
)

// PluginRegistry stores a set of core plugins.
type PluginRegistry []contracts.ICorePlugin

// registeredCorePlugins stores the registered core plugins.
var registeredCorePlugins PluginRegistry

// RegisteredCorePlugins returns all registered core plugins.
func RegisteredCorePlugins(context context.T) *PluginRegistry {
	if registeredCorePlugins == nil {
		loadCorePlugins(context)
	}
	return &registeredCorePlugins
}

// register core plugins here
func loadCorePlugins(context context.T) {
	registeredCorePlugins = make([]contracts.ICorePlugin, 2)

	// registering the health core plugin
	registeredCorePlugins[0] = health.NewHealthCheck(context)

	// registering the messages core plugin
	registeredCorePlugins[1] = message.NewProcessor(context)

	//registeredCorePlugins[2] = config
}
