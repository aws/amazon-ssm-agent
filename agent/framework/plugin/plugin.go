// Copyright 2016 Amazon.com, Inc. or its affiliates. All Rights Reserved.
//
// Licensed under the Amazon Software License (the "License"). You may not
// use this file except in compliance with the License. A copy of the
// License is located at
//
// http://aws.amazon.com/asl/
//
// or in the "license" file accompanying this file. This file is distributed
// on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND,
// express or implied. See the License for the specific language governing
// permissions and limitations under the License.

// Package plugin contains general interfaces and types relevant to plugins.
// It also provides the methods for registering plugins.
package plugin

import (
	"sync"

	"github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/aws/amazon-ssm-agent/agent/contracts"
	"github.com/aws/amazon-ssm-agent/agent/plugins/runcommand"
	"github.com/aws/amazon-ssm-agent/agent/plugins/updatessmagent"
	"github.com/aws/amazon-ssm-agent/agent/task"
)

// T is the interface type for plugins.
type T interface {
	Execute(context context.T, config contracts.Configuration, cancelFlag task.CancelFlag) contracts.PluginResult
}

// PluginRegistry stores a set of plugins, indexed by ID.
type PluginRegistry map[string]T

// registeredExecuters stores the registered plugins.
var registeredExecuters *PluginRegistry

// RegisteredWorkerPlugins returns all registered core plugins.
func RegisteredWorkerPlugins(context context.T) PluginRegistry {
	if !isLoaded() {
		cache(loadWorkerPlugins(context))
	}
	return getCached()
}

// register worker plugins here
func loadWorkerPlugins(context context.T) PluginRegistry {
	log := context.Log()
	var workerPlugins = PluginRegistry{}

	// registering runcommand plugin
	runcommandPluginName := runcommand.Name()
	runcommandPlugin, err := runcommand.NewPlugin()
	if err != nil {
		log.Errorf("failed to create plugin %s %v", runcommandPluginName, err)
	}
	workerPlugins[runcommandPluginName] = runcommandPlugin

	// registering updateagent plugin
	updateAgentPluginName := updatessmagent.Name()
	updateAgentPlugin, err := updatessmagent.NewPlugin()
	if err != nil {
		log.Errorf("failed to create plugin %s %v", updateAgentPluginName, err)
	}
	workerPlugins[updateAgentPluginName] = updateAgentPlugin

	return workerPlugins
}

var lock sync.RWMutex

func isLoaded() bool {
	lock.RLock()
	defer lock.RUnlock()
	return registeredExecuters != nil
}

func cache(plugins PluginRegistry) {
	lock.Lock()
	defer lock.Unlock()
	registeredExecuters = &plugins
}

func getCached() PluginRegistry {
	lock.RLock()
	defer lock.RUnlock()
	return *registeredExecuters
}
