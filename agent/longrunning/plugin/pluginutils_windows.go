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
// +build windows

// Package plugin contains all essential structs/interfaces for long running plugins
package plugin

import (
	"fmt"

	"github.com/aws/amazon-ssm-agent/agent/appconfig"
	"github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/aws/amazon-ssm-agent/agent/framework/processor/executer/iohandler"
	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/amazon-ssm-agent/agent/longrunning/plugin/cloudwatch"
	"github.com/aws/amazon-ssm-agent/agent/platform"
)

// loadPlatformDepedentPlugins loads all registered long running plugins in memory
func loadPlatformDependentPlugins(context context.T) map[string]Plugin {
	log := context.Log()
	//long running plugins that can be started/stopped/configured by long running plugin manager
	longrunningplugins := make(map[string]Plugin)

	//registering cloudwatch plugin
	var cw Plugin
	var cwInfo PluginInfo

	//initializing cloudwatch info
	cwInfo.Name = appconfig.PluginNameCloudWatch
	cwInfo.Configuration = ""
	cwInfo.State = PluginState{}

	if handler, err := cloudwatch.NewPlugin(iohandler.DefaultOutputConfig()); err == nil {
		cw.Info = cwInfo
		cw.Handler = handler

		//add the registered plugin in the map
		longrunningplugins[appconfig.PluginNameCloudWatch] = cw
	} else {
		log.Errorf("failed to create long-running plugin %s %v", appconfig.PluginNameCloudWatch, err)
	}

	return longrunningplugins
}

// IsLongRunningPluginSupportedForCurrentPlatform returns true if current platform supports the plugin with given name.
func IsLongRunningPluginSupportedForCurrentPlatform(log log.T, pluginName string) (bool, string) {
	platformName, _ := platform.PlatformName(log)
	platformVersion, _ := platform.PlatformVersion(log)

	if pluginName == appconfig.PluginNameCloudWatch {
		if isPlatformNanoServer, err := platform.IsPlatformNanoServer(log); err == nil && isPlatformNanoServer {
			//if the current OS is Nano server, SSM Agent doesn't support the following plugins.
			return false, fmt.Sprintf("%s (Nano Server) v%s", platformName, platformVersion)
		} else {
			return true, fmt.Sprintf("%s v%s", platformName, platformVersion)
		}
	}
	return false, fmt.Sprintf("%s v%s", platformName, platformVersion)
}
