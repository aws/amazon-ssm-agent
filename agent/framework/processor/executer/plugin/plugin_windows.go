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
//
// Package plugin contains general interfaces and types relevant to plugins.
// It also provides the methods for registering plugins.
//
// +build windows

package plugin

import (
	"github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/aws/amazon-ssm-agent/agent/framework/runpluginutil"
	"github.com/aws/amazon-ssm-agent/agent/plugins/application"
	"github.com/aws/amazon-ssm-agent/agent/plugins/configuredaemon"
	"github.com/aws/amazon-ssm-agent/agent/plugins/domainjoin"
	"github.com/aws/amazon-ssm-agent/agent/plugins/pluginutil"
	"github.com/aws/amazon-ssm-agent/agent/plugins/psmodule"
	"github.com/aws/amazon-ssm-agent/agent/plugins/updateec2config"
)

// loadPlatformDependentPlugins registers platform dependent plugins
func loadPlatformDependentPlugins(context context.T) runpluginutil.PluginRegistry {
	log := context.Log()
	var workerPlugins = runpluginutil.PluginRegistry{}

	// registering aws:psModule plugin
	psModulePluginName := psmodule.Name()
	psModulePlugin, err := psmodule.NewPlugin(pluginutil.DefaultPluginConfig())
	if err != nil {
		log.Errorf("failed to create plugin %s %v", psModulePluginName, err)
	} else {
		workerPlugins[psModulePluginName] = psModulePlugin
	}

	// registering aws:applications plugin
	applicationPluginName := application.Name()
	applicationPlugin, err := application.NewPlugin(pluginutil.DefaultPluginConfig())
	if err != nil {
		log.Errorf("failed to create plugin %s %v", applicationPluginName, err)
	} else {
		workerPlugins[applicationPluginName] = applicationPlugin
	}

	// registering aws:domainJoin plugin
	domainJoinPluginName := domainjoin.Name()
	domainJoinPlugin, err := domainjoin.NewPlugin(pluginutil.DefaultPluginConfig())
	if err != nil {
		log.Errorf("failed to create plugin %s %v", domainJoinPluginName, err)
	} else {
		workerPlugins[domainJoinPluginName] = domainJoinPlugin
	}

	// registering aws:updateAgent plugin.
	updateEC2AgentPluginName := updateec2config.Name()
	log.Debugf("Registering EC2 updateAgent plugin, %s", updateEC2AgentPluginName)
	updateEC2Agent, err := updateec2config.NewPlugin(updateec2config.GetUpdatePluginConfig(context))
	if err != nil {
		log.Errorf("failed to create plugin %s %v", updateEC2AgentPluginName, err)
	} else {
		workerPlugins[updateEC2AgentPluginName] = updateEC2Agent
	}

	// registering aws:configureDaemon
	configureDaemonPluginName := configuredaemon.Name()
	configureDaemonPlugin, err := configuredaemon.NewPlugin(pluginutil.DefaultPluginConfig())
	if err != nil {
		log.Errorf("failed to create plugin %s %v", configureDaemonPluginName, err)
	} else {
		workerPlugins[configureDaemonPluginName] = configureDaemonPlugin
	}

	return workerPlugins
}
