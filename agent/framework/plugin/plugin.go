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

// Package plugin contains general interfaces and types relevant to plugins.
// It also provides the methods for registering plugins.
package plugin

import (
	"sync"

	"github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/aws/amazon-ssm-agent/agent/framework/runpluginutil"
	"github.com/aws/amazon-ssm-agent/agent/plugins/lrpminvoker"
	"github.com/aws/amazon-ssm-agent/agent/plugins/pluginutil"
	"github.com/aws/amazon-ssm-agent/agent/plugins/refreshassociation"
	"github.com/aws/amazon-ssm-agent/agent/plugins/runcommand"
	"github.com/aws/amazon-ssm-agent/agent/plugins/updatessmagent"
	"github.com/aws/amazon-ssm-agent/agent/task"
	"github.com/aws/amazon-ssm-agent/agent/plugins/dockercontainer"
)

// registeredExecuters stores the registered plugins.
var registeredExecuters, registeredLongRunningPlugins *runpluginutil.PluginRegistry

// RegisteredWorkerPlugins returns all registered core plugins.
func RegisteredWorkerPlugins(context context.T) runpluginutil.PluginRegistry {
	if !isLoaded() {
		cache(loadWorkerPlugins(context), loadLongRunningPlugins(context))
	}
	return getCachedWorkerPlugins()
}

// LongRunningPlugins returns a map of long running plugins and their respective handlers
func RegisteredLongRunningPlugins(context context.T) runpluginutil.PluginRegistry {
	if !isLoaded() {
		cache(loadWorkerPlugins(context), loadLongRunningPlugins(context))
	}
	return getCachedLongRunningPlugins()
}

var lock sync.RWMutex

func isLoaded() bool {
	lock.RLock()
	defer lock.RUnlock()
	return registeredExecuters != nil
}

func cache(workerPlugins, longRunningPlugins runpluginutil.PluginRegistry) {
	lock.Lock()
	defer lock.Unlock()
	registeredExecuters = &workerPlugins
	registeredLongRunningPlugins = &longRunningPlugins
}

func getCachedWorkerPlugins() runpluginutil.PluginRegistry {
	lock.RLock()
	defer lock.RUnlock()
	return *registeredExecuters
}

func getCachedLongRunningPlugins() runpluginutil.PluginRegistry {
	lock.RLock()
	defer lock.RUnlock()
	return *registeredLongRunningPlugins
}

// loadLongRunningPlugins loads all long running plugins
func loadLongRunningPlugins(context context.T) runpluginutil.PluginRegistry {
	log := context.Log()
	var longRunningPlugins = runpluginutil.PluginRegistry{}

	//Long running plugins are handled by lrpm. lrpminvoker is a worker plugin that can communicate with lrpm.
	//that's why all long running plugins are first handled by lrpminvoker - which then hands off the work to lrpm.

	if handler, err := lrpminvoker.NewPlugin(pluginutil.DefaultPluginConfig()); err != nil {
		log.Errorf("Failed to load lrpminvoker that will handle all long running plugins - %v", err)
	} else {
		//NOTE: register all long running plugins here

		//registering handler for aws:cloudWatch plugin
		cloudwatchPluginName := "aws:cloudWatch"
		longRunningPlugins[cloudwatchPluginName] = handler
	}

	return longRunningPlugins
}

// loadWorkerPlugins loads all plugins
func loadWorkerPlugins(context context.T) runpluginutil.PluginRegistry {
	var workerPlugins = runpluginutil.PluginRegistry{}

	for key, value := range loadPlatformIndependentPlugins(context) {
		workerPlugins[key] = value
	}

	for key, value := range loadPlatformDependentPlugins(context) {
		workerPlugins[key] = value
	}

	return workerPlugins
}

// loadPlatformIndependentPlugins registers plugins common to all platforms
func loadPlatformIndependentPlugins(context context.T) runpluginutil.PluginRegistry {
	log := context.Log()
	var workerPlugins = runpluginutil.PluginRegistry{}

	// registering aws:runPowerShellScript & aws:runShellScript plugin
	runcommandPluginName := runcommand.Name()
	runcommandPlugin, err := runcommand.NewPlugin(pluginutil.DefaultPluginConfig())
	if err != nil {
		log.Errorf("failed to create plugin %s %v", runcommandPluginName, err)
	} else {
		workerPlugins[runcommandPluginName] = runcommandPlugin
	}

	// registering aws:updateSsmAgent plugin
	updateAgentPluginName := updatessmagent.Name()
	updateAgentPlugin, err := updatessmagent.NewPlugin(updatessmagent.GetUpdatePluginConfig(context))
	if err != nil {
		log.Errorf("failed to create plugin %s %v", updateAgentPluginName, err)
	} else {
		workerPlugins[updateAgentPluginName] = updateAgentPlugin
	}

	// registering aws:runDockerAction plugin
	runDockerPluginName := dockercontainer.Name()
	runDockerPlugin, err := dockercontainer.NewPlugin(pluginutil.DefaultPluginConfig())
	if err != nil {
		log.Errorf("failed to create plugin %s %v", runDockerPluginName, err)
	} else {
		workerPlugins[runDockerPluginName] = runDockerPlugin
	}

	// registering aws:refreshAssociation plugin
	refreshAssociationPluginName := refreshassociation.Name()
	refreshAssociationPlugin, err := refreshassociation.NewPlugin(pluginutil.DefaultPluginConfig())
	if err != nil {
		log.Errorf("failed to create plugin %s %v", refreshAssociationPluginName, err)
	} else {
		workerPlugins[refreshAssociationPluginName] = refreshAssociationPlugin
	}

	return workerPlugins
}
