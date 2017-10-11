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
	"github.com/aws/amazon-ssm-agent/agent/appconfig"
	"github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/aws/amazon-ssm-agent/agent/framework/runpluginutil"
	"github.com/aws/amazon-ssm-agent/agent/plugins/runscript"
)

type RunShellScriptFactory struct {
}

func (f RunShellScriptFactory) Create(context context.T) (runpluginutil.T, error) {
	return runscript.NewRunShellPlugin(context.Log())
}

// loadPlatformDependentPlugins registers platform dependent plugins
func loadPlatformDependentPlugins(context context.T) runpluginutil.PluginRegistry {
	var workerPlugins = runpluginutil.PluginRegistry{}

	workerPlugins[appconfig.PluginNameAwsRunShellScript] = RunShellScriptFactory{}
	return workerPlugins
}
