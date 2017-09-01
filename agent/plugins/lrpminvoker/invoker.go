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

// Package lrpminvoker contains implementation of lrpm-invoker plugin. (lrpm - long running plugin manager)
// lrpminvoker is an ondemand worker plugin - which can be called by SSM config or SSM Command.
package lrpminvoker

import (
	"fmt"

	"github.com/aws/amazon-ssm-agent/agent/appconfig"
	"github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/aws/amazon-ssm-agent/agent/contracts"
	"github.com/aws/amazon-ssm-agent/agent/jsonutil"
	logger "github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/amazon-ssm-agent/agent/plugins/pluginutil"
	"github.com/aws/amazon-ssm-agent/agent/task"
)

// Plugin is the type for the lrpm invoker plugin.
type Plugin struct {
	pluginutil.DefaultPlugin
	lrpName string
}

// LongRunningPluginSettings represents startType configuration of long running plugin
type LongRunningPluginSettings struct {
	StartType string
}

// InvokerInput represents input to lrpm invoker
type InvokerInput struct {
	ID         string      `json:"id"`
	Properties interface{} `json:"properties"`
}

//todo: add interfaces & dependencies to simplify testing for all calls from lrpminvoker calls to lrpm

// NewPlugin returns an instance of lrpminvoker for a given long running plugin name
func NewPlugin(pluginConfig pluginutil.PluginConfig, lrpName string) (*Plugin, error) {
	var plugin Plugin
	var err error
	plugin.MaxStdoutLength = pluginConfig.MaxStdoutLength
	plugin.MaxStderrLength = pluginConfig.MaxStderrLength
	plugin.StdoutFileName = pluginConfig.StdoutFileName
	plugin.StderrFileName = pluginConfig.StderrFileName
	plugin.OutputTruncatedSuffix = pluginConfig.OutputTruncatedSuffix
	plugin.ExecuteUploadOutputToS3Bucket = pluginutil.UploadOutputToS3BucketExecuter(plugin.UploadOutputToS3Bucket)

	//name of the long running plugin that this instance of lrpminvoker interacts with - this is the name the lrpminvoker plugin instance is registered under
	plugin.lrpName = lrpName

	return &plugin, err
}

// Name returns the plugin name
func Name() string {
	return appconfig.PluginNameLongRunningPluginInvoker
}

// Execute sends commands specific to long running plugins to long running plugin manager and accordingly sends reply back
func (p *Plugin) Execute(context context.T, config contracts.Configuration, cancelFlag task.CancelFlag) (res contracts.PluginResult) {
	log := context.Log()
	log.Infof("long running plugin invoker has been invoked")

	var err error

	//NOTE: All long running plugins have json node similar to aws:cloudWatch as mentioned in SSM document - AWS-ConfigureCloudWatch

	//check if plugin is enabled or not - which would be stored in settings
	if configJson, ok := config.Properties.(string); ok {
		log.Debugf("Received plugin configuration - Setting: %s\n Properties: %s\n OutputS3BucketName: %s\n OutputS3Prefix: %s\n "+
			"OrchestrationDirectory: %s\n MessageId: %s\n BookKeepingFileName: %s\n PluginName: %s\n PluginID: %s\n DefaultWorkingDirectory: %s",
			config.Settings, logger.PrintCWConfig(configJson, log), config.OutputS3BucketName, config.OutputS3KeyPrefix, config.OrchestrationDirectory,
			config.MessageId, config.BookKeepingFileName, config.PluginName, config.PluginID, config.DefaultWorkingDirectory)
	}

	//load settings from plugin input -> for more details refer to AWS-ConfigureCloudWatch
	var setting LongRunningPluginSettings
	if err = jsonutil.Remarshal(config.Settings, &setting); err != nil {
		log.Errorf(fmt.Sprintf("Invalid format in plugin configuration - %v;\nError %v", config.Settings, err))

		res = p.CreateResult(fmt.Sprintf("Unable to parse Settings for %s", p.lrpName),
			contracts.ResultStatusFailed)

		return
	}
	var property string

	if cancelFlag.ShutDown() {
		res.Code = 1
		res.Status = contracts.ResultStatusFailed
	} else if cancelFlag.Canceled() {
		res.Code = 1
		res.Status = contracts.ResultStatusCancelled
	} else {
		res, property = p.prepareForStart(log, config, cancelFlag)
		res.Output = property
		res.StandardOutput = setting.StartType
	}
	return
}

// CreateResult returns a PluginResult for given message and status
func (p *Plugin) CreateResult(msg string, status contracts.ResultStatus) (res contracts.PluginResult) {
	res.Output = msg

	if status == contracts.ResultStatusFailed {
		res.StandardError = msg
		res.Code = 1
	} else {
		res.StandardOutput = msg
		res.Code = 0
	}

	res.Status = status
	return
}

// prepareForStart remalshal the Property and stop the plug if it was running before.
func (p *Plugin) prepareForStart(log logger.T, config contracts.Configuration, cancelFlag task.CancelFlag) (res contracts.PluginResult, property string) {
	// track if the preparation process succeed.
	var err error
	prop := config.Properties

	switch prop.(type) {
	// cloudwatch triggered by run command
	case string:
		break
	// cloudwatch triggered by create association
	case *string:
		temp := prop.(*string)
		prop = *temp
		break
	// cloudwatch triggered by association document
	default:
		var inputs InvokerInput
		if err = jsonutil.Remarshal(config.Properties, &inputs); err != nil {
			log.Errorf(fmt.Sprintf("Invalid format in plugin configuration - %v;\nError %v", config.Properties, err))
			res = p.CreateResult(fmt.Sprintf("Invalid format in plugin configuration - expecting property as string - %s", config.Properties),
				contracts.ResultStatusFailed)
			return
		}
		log.Debug(inputs)
		// If the docuemnt type is 2.0, there is no Properties field in the docuemnt.
		// The whole config.Properties is the Properties we want.
		// So just need to marshal the whole Properties
		if inputs.Properties == nil {
			inputs.Properties = config.Properties
		}

		if prop, err = jsonutil.Marshal(inputs.Properties); err != nil {
			log.Error("Cannot marshal properties, ", err)
		}
	}

	// config.Properties
	if err = jsonutil.Remarshal(prop, &property); err != nil {
		log.Errorf(fmt.Sprintf("Invalid format in plugin configuration - %v;\nError %v", config.Properties, err))
		res = p.CreateResult(fmt.Sprintf("Invalid format in plugin configuration - expecting property as string - %s", config.Properties),
			contracts.ResultStatusFailed)
		return
	}
	res = p.CreateResult("success", contracts.ResultStatusSuccess)
	return
}
