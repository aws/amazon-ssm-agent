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
	"time"

	"github.com/aws/amazon-ssm-agent/agent/appconfig"
	"github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/aws/amazon-ssm-agent/agent/contracts"
	"github.com/aws/amazon-ssm-agent/agent/jsonutil"
	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/amazon-ssm-agent/agent/plugins/pluginutil"
	"github.com/aws/amazon-ssm-agent/agent/task"
)

// Plugin is the type for the refreshassociation plugin.
type Plugin struct {
	pluginutil.DefaultPlugin
}

// RefreshAssociationPluginInput represents one set of commands executed by the refreshassociation plugin.
type RefreshAssociationPluginInput struct {
	contracts.PluginInput
	ID             string
	AssociationIds []string
}

// NewPlugin returns a new instance of the plugin.
func NewPlugin(pluginConfig pluginutil.PluginConfig) (*Plugin, error) {
	var plugin Plugin
	plugin.MaxStdoutLength = pluginConfig.MaxStdoutLength
	plugin.MaxStderrLength = pluginConfig.MaxStderrLength
	plugin.StdoutFileName = pluginConfig.StdoutFileName
	plugin.StderrFileName = pluginConfig.StderrFileName
	plugin.OutputTruncatedSuffix = pluginConfig.OutputTruncatedSuffix
	plugin.ExecuteUploadOutputToS3Bucket = pluginutil.UploadOutputToS3BucketExecuter(plugin.UploadOutputToS3Bucket)

	return &plugin, nil
}

// Name returns the name of the plugin
func Name() string {
	return appconfig.PluginNameRefreshAssociation
}

// Execute runs multiple sets of commands and returns their outputs.
// res.Output will contain a slice of PluginOutput.
func (p *Plugin) Execute(context context.T, config contracts.Configuration, cancelFlag task.CancelFlag) (res contracts.PluginResult) {
	log := context.Log()
	log.Infof("%v started with configuration %v", Name(), config)
	res.StartDateTime = time.Now()
	defer func() { res.EndDateTime = time.Now() }()

	//loading Properties as list since aws:refreshAssociation uses properties as list
	var properties []interface{}
	if properties = pluginutil.LoadParametersAsList(log, config.Properties, &res); res.Code != 0 {
		return res
	}

	var associationIds []string
	var err error

	out := contracts.PluginOutput{}
	for _, prop := range properties {

		if cancelFlag.ShutDown() {
			out.MarkAsShutdown()
			break
		}

		if cancelFlag.Canceled() {
			out.MarkAsCancelled()
			break
		}

		if associationIds, err = p.getAssociationIdsFromPluginInput(log, prop); err != nil {
			out.MarkAsFailed(log, err)
		}

	}

	res.Code = out.ExitCode
	res.Status = contracts.ResultStatusSuccess
	res.Output = associationIds
	res.StandardOutput = pluginutil.StringPrefix(out.Stdout, p.MaxStdoutLength, p.OutputTruncatedSuffix)
	res.StandardError = pluginutil.StringPrefix(out.Stderr, p.MaxStderrLength, p.OutputTruncatedSuffix)

	pluginutil.PersistPluginInformationToCurrent(log, config.PluginID, config, res)
	return res
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
