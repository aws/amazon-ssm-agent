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

// Package runpluginutil provides Plugins factory as PluginRegistry interface and other utility functions for running plugins
package runpluginutil

import (
	"fmt"

	"github.com/aws/amazon-ssm-agent/agent/appconfig"
	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/amazon-ssm-agent/agent/platform"
)

// IsPluginSupportedForCurrentPlatform returns true if current platform supports the plugin with given name.
func IsPluginSupportedForCurrentPlatform(log log.T, pluginName string) (isKnown bool, isSupported bool, message string) {
	_, known := allPlugins[pluginName]
	platformName, _ := platform.PlatformName(log)
	platformVersion, _ := platform.PlatformVersion(log)

	if isPlatformNanoServer, err := platform.IsPlatformNanoServer(log); err == nil && isPlatformNanoServer {
		//if the current OS is Nano server, SSM Agent doesn't support the following plugins.
		if pluginName == appconfig.PluginNameDomainJoin ||
			pluginName == appconfig.PluginNameCloudWatch {
			return known, false, fmt.Sprintf("%s (Nano Server) v%s", platformName, platformVersion)
		}
	}
	return known, true, fmt.Sprintf("%s v%s", platformName, platformVersion)
}
