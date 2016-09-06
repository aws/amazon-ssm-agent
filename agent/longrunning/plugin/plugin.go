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

// Package plugin contains all essential structs/interfaces for long running plugins

package plugin

import (
	"time"

	"github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/aws/amazon-ssm-agent/agent/task"
)

// PluginState reflects state of a long running plugin
type PluginState struct {
	LastConfigurationModifiedTime time.Time
	IsEnabled                     bool
}

//PluginInfo reflects information about long running plugins
//This is also used by lrpm manager to persisting information & then later use it for reference
type PluginInfo struct {
	Name          string
	Configuration string
	State         PluginState
}

// Plugin reflects a long running plugin
type Plugin struct {
	Info    PluginInfo
	Handler LongRunningPlugin
}

//LongRunningPlugin is the interface that must be implemented by all long running plugins
type LongRunningPlugin interface {
	IsRunning(context context.T) bool
	Start(context context.T, configuration string, orchestrationDir string, cancelFlag task.CancelFlag) error
	Stop(context context.T, cancelFlag task.CancelFlag) error
}

//PluginSettings reflects settings that can be applied to long running plugins like aws:cloudWatch
type PluginSettings struct {
	StartType string
}

//LongRunningPluginInput represents input for long running plugin like aws:cloudWatch
type LongRunningPluginInput struct {
	Settings   PluginSettings
	Properties string
}
