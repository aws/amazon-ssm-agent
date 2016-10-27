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

	"io/ioutil"

	"github.com/aws/amazon-ssm-agent/agent/appconfig"
	"github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/aws/amazon-ssm-agent/agent/fileutil"
	"github.com/aws/amazon-ssm-agent/agent/framework/runpluginutil"
	"github.com/aws/amazon-ssm-agent/agent/jsonutil"
	"github.com/aws/amazon-ssm-agent/agent/longrunning/plugin/rundaemon"
	"github.com/aws/amazon-ssm-agent/agent/plugins/pluginutil"
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
	Start(context context.T, configuration string, orchestrationDir string, cancelFlag task.CancelFlag) error
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

func RegisteredPlugins(context context.T) map[string]Plugin {
	longrunningplugins := make(map[string]Plugin)
	context.Log().Debug("Registering long-running plugins")

	for key, value := range loadPlatformIndependentPlugins(context) {
		longrunningplugins[key] = value
	}

	for key, value := range loadPlatformDependentPlugins(context) {
		longrunningplugins[key] = value
	}

	context.Log().Debugf("Registered %v long-running plugins", len(longrunningplugins))
	return longrunningplugins
}

// loadPlatformIndependentPlugins loads all long running plugins in memory
func loadPlatformIndependentPlugins(context context.T) map[string]Plugin {
	log := context.Log()
	//long running plugins that can be started/stopped/configured by long running plugin manager
	longrunningplugins := make(map[string]Plugin)

	// find all packages that should run as daemons and register a rundaemon plugin for each
	if pkgdirs, err := fileutil.GetDirectoryNames(appconfig.PackageRoot); err == nil {
		for _, pkgdir := range pkgdirs {
			if verdirs, err := fileutil.GetDirectoryNames(filepath.Join(appconfig.PackageRoot, pkgdir)); err == nil {
				for _, verdir := range verdirs {
					daemonWorkingDir := filepath.Join(appconfig.PackageRoot, pkgdir, verdir)
					daemonStartFile := filepath.Join(daemonWorkingDir, "start.json")
					if fileutil.Exists(daemonStartFile) {
						// load file
						var input rundaemon.DaemonPluginInput
						var err error
						filedata, _ := ioutil.ReadFile(daemonStartFile)
						pluginsInfo, parseErr := runpluginutil.ParseDocument(
							context,
							filedata,
							"", "", "", "", "", daemonWorkingDir)
						pluginInfo, exists := pluginsInfo["aws:configureDaemon"]
						if !exists {
							log.Debugf("Daemon configuration file %v did not unmarshal as expected.  Contains %v entries, parse error: %v", daemonStartFile, len(pluginsInfo), parseErr)
							continue
						}
						properties, res := pluginutil.LoadParametersAsList(log, pluginInfo.Configuration.Properties)
						if res.Code != 0 || len(properties) == 0 {
							log.Debugf("Daemon properties did not load as expected, %v", res.Error.Error())
							continue
						}
						// TODO:MF: We assume in a lot of places that documents can contain a list of actions and then we only deal with the first
						if err = jsonutil.Remarshal(properties[0], &input); err == nil {
							log.Infof("Registering long-running plugin for daemon %v", input.Name)
							plugin := Plugin{
								Info: PluginInfo{
									Name:          input.Name,
									Configuration: input.Command,
									State:         PluginState{IsEnabled: true},
								},
								Handler: &rundaemon.Plugin{
									ExeLocation: daemonWorkingDir,
									Name:        input.Name,
									CommandLine: input.Command,
								},
							}
							// TODO:MF: if there are multiple version folders, use the latest that isn't installing?  Shouldn't be an issue because if there are multiple we SHOULD be mid-install but it would be good to be safe
							longrunningplugins[input.Name] = plugin
						} else {
							log.Debugf("Error unmarshalling %v, %v", daemonStartFile, err.Error())
						}
					}
				}
			} else {
				log.Debugf("Error getting directory names under %v, %v", pkgdir, err.Error())
			}
		}
	} else {
		log.Debugf("Error getting directory names under %v, %v", appconfig.PackageRoot, err.Error())
	}
	return longrunningplugins
}
