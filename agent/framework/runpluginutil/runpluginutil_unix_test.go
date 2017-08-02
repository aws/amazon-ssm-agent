// Copyright 2017 Amazon.com, Inc. or its affiliates. All Rights Reserved.
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
	"testing"

	"github.com/aws/amazon-ssm-agent/agent/appconfig"
	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/stretchr/testify/assert"
)

var mockLog = log.NewMockLog()

func TestKnownSupported(t *testing.T) {
	isKnown, isSupported, _ := IsPluginSupportedForCurrentPlatform(mockLog, appconfig.PluginNameAwsRunShellScript)
	assert.True(t, isKnown)
	assert.True(t, isSupported)
}

/*
func TestKnownUnsupported(t *testing.T) {
	isKnown, isSupported, _ := IsPluginSupportedForCurrentPlatform(mockLog, appconfig.PluginEC2ConfigUpdate)
	assert.True(t, isKnown)
	assert.False(t, isSupported)
}
*/

func TestUnknown(t *testing.T) {
	isKnown, isSupported, _ := IsPluginSupportedForCurrentPlatform(mockLog, "FOO")
	assert.False(t, isKnown)
	assert.True(t, isSupported)
}
