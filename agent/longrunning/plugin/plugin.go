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

// Package plugin contains all essential structs/interfaces for long running plugins
package plugin

import (
	"path/filepath"
	"time"

	"github.com/aws/amazon-ssm-agent/agent/appconfig"
	"github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/aws/amazon-ssm-agent/agent/fileutil"
	"github.com/aws/amazon-ssm-agent/agent/framework/processor/executer/iohandler"
	"github.com/aws/amazon-ssm-agent/agent/jsonutil"
	"github.com/aws/amazon-ssm-agent/agent/longrunning/plugin/rundaemon"
	"github.com/aws/amazon-ssm-agent/agent/task"
)

// PluginState reflects state of a long running plugin
type PluginState struct {
	LastConfigurationModifiedTime time.Time
	IsEnabled                     bool
}

//PluginInfo reflects information about long running plugins
//This is also used by lrpm manager to persisting information & then later use it for reference
type PluginInfo struct {
	Name          string
	Configuration string
	State         PluginState
}

// Plugin reflects a long running plugin
type Plugin struct {
	Info    PluginInfo
	Handler LongRunningPlugin
}

//LongRunningPlugin is the interface that must be implemented by all long running plugins
type LongRunningPlugin interface {
	IsRunning(context context.T) bool
	Start(context context.T, configuration string, orchestrationDir string, cancelFlag task.CancelFlag, out iohandler.IOHandler) error
	Stop(context context.T, cancelFlag task.CancelFlag) error
}

//PluginSettings reflects settings that can be applied to long running plugins like aws:cloudWatch
type PluginSettings struct {
	StartType string
}

//LongRunningPluginInput represents input for long running plugin like aws:cloudWatch
type LongRunningPluginInput struct {
	Settings   PluginSettings
	Properties string
}

// RegisteredPlugins loads all long running plugins in memory
func RegisteredPlugins(context context.T) map[string]Plugin {
	longrunningplugins := make(map[string]Plugin)
	log := context.Log()
	log.Debug("Registering long-running plugins")

	for key, value := range loadPlatformDependentPlugins(context) {
		log.Debugf("Adding platform-specific long-running plugin for %v", key)
		longrunningplugins[key] = value
	}

	for key, value := range loadPlatformIndependentPlugins(context) {
		if _, exists := longrunningplugins[key]; exists {
			log.Errorf("Duplicate long-running plugin - %v already registered", key)
			continue
		}
		context.Log().Debugf("Adding long-running plugin for %v", key)
		longrunningplugins[key] = value
	}

	context.Log().Debugf("Registered %v long-running plugins", len(longrunningplugins))
	return longrunningplugins
}

// loadPlatformIndependentPlugins loads all long running plugins that don't have platform specific implementations
func loadPlatformIndependentPlugins(context context.T) map[string]Plugin {
	//long running plugins that can be started/stopped/configured by long running plugin manager
	longrunningplugins := make(map[string]Plugin)

	for key, value := range loadDaemonPlugins(context) {
		context.Log().Debugf("Adding long-running plugin for %v", key)
		longrunningplugins[key] = value
	}

	return longrunningplugins
}

// loadDaemonPlugins registers long running plugin handlers for ssm daemons
func loadDaemonPlugins(context context.T) map[string]Plugin {
	//long running daemon plugins that can be started/stopped/removed/configured by long running plugin manager
	daemonPlugins := make(map[string]Plugin)

	log := context.Log()
	// find all packages that should run as daemons and register a rundaemon plugin for each
	if err := fileutil.MakeDirs(appconfig.DaemonRoot); err != nil {
		log.Errorf("Unable to create ssm daemon folder %v: %v", appconfig.DaemonRoot, err)
		return daemonPlugins
	}
	daemons, err := fileutil.GetFileNames(appconfig.DaemonRoot)
	if err != nil {
		log.Errorf("error listing registered ssm daemons: %v", err)
		return daemonPlugins
	}
	for _, daemonFile := range daemons {
		// load file
		daemonFilePath := filepath.Join(appconfig.DaemonRoot, daemonFile)
		var input rundaemon.ConfigureDaemonPluginInput
		if err = jsonutil.UnmarshalFile(daemonFilePath, &input); err != nil {
			log.Errorf("Error unmarshalling %v, %v", daemonFilePath, err.Error())
		} else {
			if err := rundaemon.ValidateDaemonInput(input); err != nil {
				log.Errorf("ssm daemon file %v is invalid: %v", daemonFilePath, err.Error())
			} else {
				log.Infof("Registering long-running plugin for ssm daemon %v", input.Name)
				plugin := Plugin{
					Info: PluginInfo{
						Name:          input.Name,
						Configuration: input.Command,
						State:         PluginState{IsEnabled: true},
					},
					Handler: &rundaemon.Plugin{
						ExeLocation: input.PackageLocation,
						Name:        input.Name,
						CommandLine: input.Command,
					},
				}
				if _, exists := daemonPlugins[input.Name]; exists {
					log.Errorf("duplicate registrations exist for ssm daemon %v", input.Name)
					continue
				}
				daemonPlugins[input.Name] = plugin
			}
		}
	}
	return daemonPlugins
}
