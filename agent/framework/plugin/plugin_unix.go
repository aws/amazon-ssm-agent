// Copyright 2016 Amazon.com, Inc. or its affiliates. All Rights Reserved.
//
// Licensed under the AWS Customer Agreement (the "License"). You may
// not use this file except in compliance with the License. A copy of the
// License is located at
//
// http://aws.amazon.com/agreement/

// Package plugin contains general interfaces and types relevant to plugins.
// It also provides the methods for registering plugins.
//
// +build darwin freebsd linux netbsd openbsd

package plugin

import (
	"github.com/aws/amazon-ssm-agent/agent/context"
)

// loadPlatformDependentPlugins registers platform dependent plugins
func loadPlatformDependentPlugins(context context.T) PluginRegistry {
	return PluginRegistry{}
}
