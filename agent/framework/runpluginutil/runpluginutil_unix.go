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

// Package runpluginutil run plugin utility functions without referencing the actually plugin impl packages
package runpluginutil

import (
	"fmt"

	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/amazon-ssm-agent/agent/platform"
)

// IsPluginSupportedForCurrentPlatform always returns true for plugins that exist for linux because currently there
// are no plugins that are supported on only one distribution or version of linux.
func IsPluginSupportedForCurrentPlatform(log log.T, pluginName string) (isKnown bool, isSupported bool, message string) {
	platformName, _ := platform.PlatformName(log)
	platformVersion, _ := platform.PlatformVersion(log)

	if _, known := allSessionPlugins[pluginName]; known == true {
		return known, true, fmt.Sprintf("%s v%s", platformName, platformVersion)
	}
	_, known := allPlugins[pluginName]
	return known, true, fmt.Sprintf("%s v%s", platformName, platformVersion)
}
