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
// +build darwin freebsd linux netbsd openbsd

package plugin

import (
	"fmt"

	"github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/aws/amazon-ssm-agent/agent/framework/runpluginutil"
	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/amazon-ssm-agent/agent/platform"
	"github.com/aws/amazon-ssm-agent/agent/plugins/pluginutil"
	"github.com/aws/amazon-ssm-agent/agent/plugins/runscript"
)

// IsPluginSupportedForCurrentPlatform always returns true for plugins that exist for linux because currently there
// are no plugins that are supported on only one distribution or version of linux.
func IsPluginSupportedForCurrentPlatform(log log.T, pluginName string) (isKnown bool, isSupported bool, message string) {
	_, known := allPlugins[pluginName]
	platformName, _ := platform.PlatformName(log)
	platformVersion, _ := platform.PlatformVersion(log)

	return known, true, fmt.Sprintf("%s v%s", platformName, platformVersion)
}

// loadPlatformDependentPlugins registers platform dependent plugins
func loadPlatformDependentPlugins(context context.T) runpluginutil.PluginRegistry {
	log := context.Log()
	var workerPlugins = runpluginutil.PluginRegistry{}

	// registering aws:runShellScript plugin
	shellPlugin, err := runscript.NewRunShellPlugin(log, pluginutil.DefaultPluginConfig())
	shellPluginName := shellPlugin.Name
	if err != nil {
		log.Errorf("failed to create plugin %s %v", shellPluginName, err)
	} else {
		workerPlugins[shellPluginName] = shellPlugin
	}

	return workerPlugins
}
