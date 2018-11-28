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

// Package runpluginutil run plugin utility functions without referencing the actually plugin impl packages
package runpluginutil

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/aws/amazon-ssm-agent/agent/appconfig"
	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/amazon-ssm-agent/agent/platform"
)

// IsPluginSupportedForCurrentPlatform returns true if current platform supports the plugin with given name.
func IsPluginSupportedForCurrentPlatform(log log.T, pluginName string) (isKnown bool, isSupported bool, message string) {
	platformName, _ := platform.PlatformName(log)
	platformVersion, _ := platform.PlatformVersion(log)

	if _, known := allSessionPlugins[pluginName]; known == true {
		return known, isSupportedSessionPlugin(log, pluginName), fmt.Sprintf("%s v%s", platformName, platformVersion)
	}

	_, known := allPlugins[pluginName]
	if isPlatformNanoServer, err := platform.IsPlatformNanoServer(log); err == nil && isPlatformNanoServer {
		//if the current OS is Nano server, SSM Agent doesn't support the following plugins.
		if pluginName == appconfig.PluginNameDomainJoin ||
			pluginName == appconfig.PluginNameCloudWatch {
			return known, false, fmt.Sprintf("%s (Nano Server) v%s", platformName, platformVersion)
		}
	}
	return known, true, fmt.Sprintf("%s v%s", platformName, platformVersion)
}

// isSupportedSessionPlugin returns  true if given session plugin is supported for current platform, false otherwise
func isSupportedSessionPlugin(log log.T, pluginName string) (isSupported bool) {
	platformVersion, _ := platform.PlatformVersion(log)

	osVersionSplit := strings.Split(platformVersion, ".")
	if osVersionSplit == nil || len(osVersionSplit) < 2 {
		log.Error("Error occurred while parsing OS version")
		return false
	}

	// check if the OS version is 6.1 or higher
	// https://docs.microsoft.com/en-us/windows/desktop/SysInfo/operating-system-version
	osMajorVersion, err := strconv.Atoi(osVersionSplit[0])
	if err != nil {
		return false
	}

	osMinorVersion, err := strconv.Atoi(osVersionSplit[1])
	if err != nil {
		return false
	}

	if osMajorVersion < 6 {
		return false
	}

	if osMajorVersion == 6 && osMinorVersion < 1 {
		return false
	}

	return true
}
