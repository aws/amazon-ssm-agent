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
//
// Package plugin contains all essential structs/interfaces for long running plugins
package plugin

import (
	"fmt"

	"github.com/aws/amazon-ssm-agent/agent/appconfig"
	"github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/amazon-ssm-agent/agent/longrunning/plugin/cloudwatch"
	"github.com/aws/amazon-ssm-agent/agent/platform"
	"github.com/aws/amazon-ssm-agent/agent/plugins/pluginutil"
)

const (
	PluginNameAwsCloudwatch = "aws:cloudWatch"
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
	cwInfo.Name = PluginNameAwsCloudwatch
	cwInfo.Configuration = ""
	cwInfo.State = PluginState{}

	if handler, err := cloudwatch.NewPlugin(pluginutil.DefaultPluginConfig()); err == nil {
		cw.Info = cwInfo
		cw.Handler = handler

		//add the registered plugin in the map
		longrunningplugins[PluginNameAwsCloudwatch] = cw
	} else {
		log.Errorf("failed to create long-running plugin %s %v", PluginNameAwsCloudwatch, err)
	}

	return longrunningplugins
}

// IsPluginSupportedForCurrentPlatform returns true if current platform supports the plugin with given name.
func IsPluginSupportedForCurrentPlatform(log log.T, pluginID string) (bool, string) {
	platformName, _ := platform.PlatformName(log)
	platformVersion, _ := platform.PlatformVersion(log)

	if isPlatformNanoServer, err := platform.IsPlatformNanoServer(log); err == nil && isPlatformNanoServer {
		//if the current OS is Nano server, SSM Agent doesn't support the following plugins.
		if pluginID == appconfig.PluginNameDomainJoin ||
			pluginID == appconfig.PluginNameCloudWatch {
			return false, fmt.Sprintf("%s (Nano Server) v%s", platformName, platformVersion)
		}
	}
	return true, fmt.Sprintf("%s v%s", platformName, platformVersion)
}
