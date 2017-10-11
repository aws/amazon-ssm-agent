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
	"github.com/aws/amazon-ssm-agent/agent/plugins/domainjoin"
	"github.com/aws/amazon-ssm-agent/agent/plugins/psmodule"
	"github.com/aws/amazon-ssm-agent/agent/plugins/updateec2config"
)

type PsModuleFactory struct {
}

func (f PsModuleFactory) Create(context context.T) (runpluginutil.T, error) {
	return psmodule.NewPlugin()
}

type ApplicationFactory struct {
}

func (f ApplicationFactory) Create(context context.T) (runpluginutil.T, error) {
	return application.NewPlugin()
}

type DomainJoinFactory struct {
}

func (f DomainJoinFactory) Create(context context.T) (runpluginutil.T, error) {
	return domainjoin.NewPlugin()
}

type UpdateEc2ConfigFactory struct {
}

func (f UpdateEc2ConfigFactory) Create(context context.T) (runpluginutil.T, error) {
	return updateec2config.NewPlugin(updateec2config.GetUpdatePluginConfig(context))
}

// loadPlatformDependentPlugins registers platform dependent plugins
func loadPlatformDependentPlugins(context context.T) runpluginutil.PluginRegistry {
	var workerPlugins = runpluginutil.PluginRegistry{}

	// registering aws:psModule plugin
	psModulePluginName := psmodule.Name()
	workerPlugins[psModulePluginName] = PsModuleFactory{}

	// registering aws:applications plugin
	applicationPluginName := application.Name()
	workerPlugins[applicationPluginName] = ApplicationFactory{}

	// registering aws:domainJoin plugin
	domainJoinPluginName := domainjoin.Name()
	workerPlugins[domainJoinPluginName] = DomainJoinFactory{}

	// registering aws:updateAgent plugin.
	updateEC2AgentPluginName := updateec2config.Name()
	workerPlugins[updateEC2AgentPluginName] = UpdateEc2ConfigFactory{}

	//// registering aws:configureDaemon
	//configureDaemonPluginName := configuredaemon.Name()
	//configureDaemonPlugin, err := configuredaemon.NewPlugin(pluginutil.DefaultPluginConfig())
	//if err != nil {
	//	log.Errorf("failed to create plugin %s %v", configureDaemonPluginName, err)
	//} else {
	//	workerPlugins[configureDaemonPluginName] = configureDaemonPlugin
	//}

	return workerPlugins
}
