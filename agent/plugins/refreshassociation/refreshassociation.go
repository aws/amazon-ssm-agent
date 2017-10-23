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

// Package refreshassociation implements the refreshassociation plugin.
package refreshassociation

import (
	"fmt"

	"github.com/aws/amazon-ssm-agent/agent/appconfig"
	"github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/aws/amazon-ssm-agent/agent/contracts"
	"github.com/aws/amazon-ssm-agent/agent/framework/processor/executer/iohandler"
	"github.com/aws/amazon-ssm-agent/agent/jsonutil"
	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/amazon-ssm-agent/agent/task"
)

// Plugin is the type for the refreshassociation plugin.
type Plugin struct {
}

// RefreshAssociationPluginInput represents one set of commands executed by the refreshassociation plugin.
type RefreshAssociationPluginInput struct {
	contracts.PluginInput
	ID             string
	AssociationIds []string
}

// NewPlugin returns a new instance of the plugin.
func NewPlugin() (*Plugin, error) {
	var plugin Plugin
	return &plugin, nil
}

// Name returns the name of the plugin
func Name() string {
	return appconfig.PluginNameRefreshAssociation
}

func (p *Plugin) Execute(context context.T, config contracts.Configuration, cancelFlag task.CancelFlag, output iohandler.IOHandler) {
	log := context.Log()
	log.Infof("%v started with configuration %v", Name(), config)

	if cancelFlag.ShutDown() {
		output.MarkAsShutdown()
	} else if cancelFlag.Canceled() {
		output.MarkAsCancelled()
	} else {
		if associationIds, err := p.getAssociationIdsFromPluginInput(log, config.Properties); err != nil {
			output.MarkAsFailed(err)
		} else {
			output.SetOutput(associationIds)
		}
	}

	output.SetStatus(contracts.ResultStatusSuccess)
	return
}

func (p *Plugin) getAssociationIdsFromPluginInput(log log.T, property interface{}) ([]string, error) {
	var pluginInput RefreshAssociationPluginInput
	err := jsonutil.Remarshal(property, &pluginInput)
	log.Debugf("Plugin input %v", pluginInput)
	if err != nil {
		errorString := fmt.Errorf("Invalid format in plugin properties %v;\nerror %v", property, err)
		return nil, errorString
	}

	return pluginInput.AssociationIds, nil
}
