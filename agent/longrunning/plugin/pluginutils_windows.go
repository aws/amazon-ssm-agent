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
	"github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/aws/amazon-ssm-agent/agent/longrunning/plugin/cloudwatch"
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
