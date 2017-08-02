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

//TODO remove the legacy RunPlugins and SendResponse method
// Package runpluginutil provides Plugins factory as PluginRegistry interface and other utility functions for running plugins
package runpluginutil

import (
	"github.com/aws/amazon-ssm-agent/agent/appconfig"
	"github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/aws/amazon-ssm-agent/agent/contracts"
	"github.com/aws/amazon-ssm-agent/agent/docmanager/model"
	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/amazon-ssm-agent/agent/task"
)

// SendResponse is used to send response on plugin completion.
// If pluginID is empty it will send responses of all plugins.
// If pluginID is specified, response will be sent of that particular plugin.
type SendResponseLegacy func(messageID string, pluginID string, results map[string]*contracts.PluginResult)

func NoReply(messageID string, pluginID string, results map[string]*contracts.PluginResult) {}

// SendDocumentLevelResponse is used to send status response before plugin begins
type SendDocumentLevelResponse func(messageID string, resultStatus contracts.ResultStatus, documentTraceOutput string)

// UpdateAssociation updates association status
type UpdateAssociation func(log log.T, documentID string, documentCreatedDate string, pluginOutputs map[string]*contracts.PluginResult, totalNumberOfPlugins int)

func NoUpdate(log log.T, documentID string, documentCreatedDate string, pluginOutputs map[string]*contracts.PluginResult, totalNumberOfPlugins int) {
}

// T is the interface type for plugins.
type T interface {
	Execute(context context.T, config contracts.Configuration, cancelFlag task.CancelFlag, subDocumentRunner PluginRunner) contracts.PluginResult
}

// PluginRegistry stores a set of plugins (both worker and long running plugins), indexed by ID.
type PluginRegistry map[string]T

type PluginRunner struct {
	RunPlugins func(
		context context.T,
		documentID string,
		documentCreatedDate string,
		plugins []model.PluginState,
		pluginRegistry PluginRegistry,
		sendReply SendResponseLegacy,
		updateAssoc UpdateAssociation,
		cancelFlag task.CancelFlag,
	) (pluginOutputs map[string]*contracts.PluginResult)
	Plugins     PluginRegistry
	SendReply   SendResponseLegacy
	UpdateAssoc UpdateAssociation
	CancelFlag  task.CancelFlag
}

func (r *PluginRunner) ExecuteDocument(context context.T, pluginInput []model.PluginState, documentID string, documentCreatedDate string) (pluginOutputs map[string]*contracts.PluginResult) {
	log := context.Log()
	for _, state := range pluginInput {
		log.Debugf("Executing document contains input for plugin %v", state.Name)
	}

	return r.RunPlugins(context, documentID, documentCreatedDate, pluginInput, r.Plugins, r.SendReply, r.UpdateAssoc, r.CancelFlag)
}

// allPlugins is the list of all known plugins.
// This allows us to differentiate between the case where a document asks for a plugin that exists but isn't supported on this platform
// and the case where a plugin name isn't known at all to this version of the agent (and the user should probably upgrade their agent)
var allPlugins = map[string]struct{}{
	appconfig.PluginNameAwsAgentUpdate:         {},
	appconfig.PluginNameAwsApplications:        {},
	appconfig.PluginNameAwsConfigureDaemon:     {},
	appconfig.PluginNameAwsConfigurePackage:    {},
	appconfig.PluginNameAwsPowerShellModule:    {},
	appconfig.PluginNameAwsRunPowerShellScript: {},
	appconfig.PluginNameAwsRunShellScript:      {},
	appconfig.PluginNameAwsSoftwareInventory:   {},
	appconfig.PluginNameCloudWatch:             {},
	appconfig.PluginNameConfigureDocker:        {},
	appconfig.PluginNameDockerContainer:        {},
	appconfig.PluginNameDomainJoin:             {},
	appconfig.PluginEC2ConfigUpdate:            {},
	appconfig.PluginNameRefreshAssociation:     {},
}
