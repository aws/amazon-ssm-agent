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
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/aws/amazon-ssm-agent/agent/appconfig"
	"github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/aws/amazon-ssm-agent/agent/contracts"
	"github.com/aws/amazon-ssm-agent/agent/fileutil"
	"github.com/aws/amazon-ssm-agent/agent/jsonutil"
	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/amazon-ssm-agent/agent/longrunning/manager"
	managerContracts "github.com/aws/amazon-ssm-agent/agent/longrunning/plugin"
	"github.com/aws/amazon-ssm-agent/agent/plugins/pluginutil"
	"github.com/aws/amazon-ssm-agent/agent/rebooter"
	"github.com/aws/amazon-ssm-agent/agent/task"
)

// Plugin is the type for the lrpm invoker plugin.
type Plugin struct {
	pluginutil.DefaultPlugin
	lrpm manager.T
}

// LongRunningPluginSettings represents startType configuration of long running plugin
type LongRunningPluginSettings struct {
	StartType string
}

// InvokerInput represents input to lrpm invoker
type InvokerInput struct {
	Settings   LongRunningPluginSettings
	Properties string
}

var readFile = ioutil.ReadFile
var getRegisteredPlugins func() map[string]managerContracts.Plugin
var pluginPersister = pluginutil.PersistPluginInformationToCurrent

//todo: add interfaces & dependencies to simplify testing for all calls from lrpminvoker calls to lrpm

// NewPlugin returns lrpminvoker
func NewPlugin(pluginConfig pluginutil.PluginConfig) (*Plugin, error) {
	var plugin Plugin
	var err error
	plugin.MaxStdoutLength = pluginConfig.MaxStdoutLength
	plugin.MaxStderrLength = pluginConfig.MaxStderrLength
	plugin.StdoutFileName = pluginConfig.StdoutFileName
	plugin.StderrFileName = pluginConfig.StderrFileName
	plugin.OutputTruncatedSuffix = pluginConfig.OutputTruncatedSuffix
	plugin.Uploader = pluginutil.GetS3Config()
	plugin.ExecuteUploadOutputToS3Bucket = pluginutil.UploadOutputToS3BucketExecuter(plugin.UploadOutputToS3Bucket)

	//getting the reference of LRPM - long running plugin manager - which manages all long running plugins
	plugin.lrpm, err = manager.GetInstance()
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

	var pluginID string
	var err error

	//grab pluginId from the context
	if pluginID, err = p.GetPluginIdFromContext(context); err != nil {
		log.Errorf("Unable to get plugin name from context - %s", context.CurrentContext())
		res = p.CreateResult("Unable to get plugin name because of unsupported plugin name format",
			contracts.ResultStatusFailed)

		pluginPersister(log, Name(), config, res)
		return
	}

	var pluginsMap = p.lrpm.GetRegisteredPlugins()
	if _, ok := pluginsMap[pluginID]; !ok {
		log.Errorf("Given plugin - %s is not registered", pluginID)
		res = p.CreateResult(fmt.Sprintf("Plugin %s is not registered by agent", pluginID),
			contracts.ResultStatusFailed)

		pluginPersister(log, Name(), config, res)
		return
	}

	//NOTE: All long running plugins have json node similar to aws:cloudWatch as mentioned in SSM document - AWS-ConfigureCloudWatch

	//check if plugin is enabled or not - which would be stored in settings
	jsonB, _ := json.Marshal(&config)
	log.Debugf("Received plugin configuration - %s", jsonutil.Indent(string(jsonB)))

	//load settings from plugin input -> for more details refer to AWS-ConfigureCloudWatch
	var setting LongRunningPluginSettings
	if err = jsonutil.Remarshal(config.Settings, &setting); err != nil {
		log.Errorf(fmt.Sprintf("Invalid format in plugin configuration - %v;\nError %v", config.Settings, err))

		res = p.CreateResult(fmt.Sprintf("Unable to parse Settings for %s", pluginID),
			contracts.ResultStatusFailed)

		pluginPersister(log, Name(), config, res)
		return
	}

	if rebooter.RebootRequested() {
		log.Infof("Stopping execution of %v plugin due to an external reboot request.", Name())
		return
	}

	if cancelFlag.ShutDown() {
		res.Code = 1
		res.Status = contracts.ResultStatusFailed
		pluginPersister(log, Name(), config, res)
		return
	}

	if cancelFlag.Canceled() {
		res.Code = 1
		res.Status = contracts.ResultStatusCancelled
		pluginPersister(log, Name(), config, res)
		return
	}

	switch setting.StartType {
	case "Enabled":
		res = p.enablePlugin(log, config, pluginID, cancelFlag)
		pluginPersister(log, Name(), config, res)
		return

	case "Disabled":

		log.Infof("Disabling %s", pluginID)
		if err = p.lrpm.StopPlugin(pluginID, cancelFlag); err != nil {
			log.Errorf("Unable to stop the plugin - %s: %s", pluginID, err.Error())
			res = p.CreateResult(fmt.Sprintf("Encountered error while stopping the plugin: %s", err.Error()),
				contracts.ResultStatusFailed)
			pluginPersister(log, Name(), config, res)
			return
		} else {
			res = p.CreateResult(fmt.Sprintf("Disabled the plugin - %s successfully", pluginID),
				contracts.ResultStatusSuccess)
			res.Status = contracts.ResultStatusSuccess
			pluginPersister(log, Name(), config, res)
			return
		}

	default:
		log.Errorf("Allowed Values of StartType: Enabled | Disabled")
		res = p.CreateResult("Allowed Values of StartType: Enabled | Disabled",
			contracts.ResultStatusFailed)
		pluginPersister(log, Name(), config, res)
		return res
	}
}

// GetPluginIdFromContext gets pluginId from context
func (p *Plugin) GetPluginIdFromContext(context context.T) (string, error) {

	//last element in context has pluginId in the following format:
	//[pluginID=<pluginName e.g aws:cloudWatch>]
	//finding plugin name from context
	c := context.CurrentContext()
	l := c[len(c)-1]
	temp := strings.Split(l, "=")[1]
	n := strings.Split(temp, "]")[0]

	//verify that pluginName is of format: aws:blah e.g: aws:cloudWatch
	pattern := regexp.MustCompile(`^aws:[aA-zZ]*`)
	if pattern.MatchString(n) {
		return n, nil
	} else {
		return "", errors.New("unable to parse pluginName from context")
	}
}

// CreateResult returns a PluginResult for given message and status
func (p *Plugin) CreateResult(msg string, status contracts.ResultStatus) (res contracts.PluginResult) {
	res.Output = msg

	if status == contracts.ResultStatusFailed {
		res.Code = 1
	} else {
		res.Code = 0
	}

	res.Status = status
	return
}

func (p *Plugin) enablePlugin(log log.T, config contracts.Configuration, pluginID string, cancelFlag task.CancelFlag) (res contracts.PluginResult) {
	log.Infof("Enabling %s", pluginID)

	//loading properties as string since aws:cloudWatch uses properties as string. Properties has new configuration for cloudwatch plugin.
	//For more details refer to AWS-ConfigureCloudWatch
	// TODO cannot check if string is a valid json for cloudwatch
	var property string
	var failed bool
	outputPath := fileutil.BuildPath(config.OrchestrationDirectory, appconfig.PluginNameCloudWatch)
	stdoutFilePath := filepath.Join(outputPath, p.StdoutFileName)
	stderrFilePath := filepath.Join(outputPath, p.StderrFileName)

	res, failed, property = p.prepareForStart(log, config, pluginID, cancelFlag)
	if failed {
		return
	}

	//start the plugin with the new configuration
	if err := p.lrpm.StartPlugin(pluginID, property, config.OrchestrationDirectory, cancelFlag); err != nil {
		log.Errorf("Unable to start the plugin - %s: %s", pluginID, err.Error())
		res = p.CreateResult(fmt.Sprintf("Encountered error while starting the plugin: %s", err.Error()),
			contracts.ResultStatusFailed)
	} else {
		var errData []byte
		var errorReadingFile error
		if errData, errorReadingFile = readFile(stderrFilePath); errorReadingFile != nil {
			log.Errorf("Unable to read the stderr file - %s: %s", stderrFilePath, errorReadingFile.Error())
		}
		serr := string(errData)

		if len(serr) > 0 {
			log.Errorf("Unable to start the plugin - %s: %s", pluginID, serr)

			// Stop the plugin if configuration failed.
			if err := p.lrpm.StopPlugin(pluginID, cancelFlag); err != nil {
				log.Errorf("Unable to start the plugin - %s: %s", pluginID, err.Error())
			}

			res = p.CreateResult(fmt.Sprintf("Encountered error while starting the plugin: %s", serr),
				contracts.ResultStatusFailed)

		} else {
			log.Info("Start Clound Watch successfully.")
			res = p.CreateResult("success", contracts.ResultStatusSuccess)
		}
	}

	// Upload output to S3
	uploadOutputToS3BucketErrors := p.ExecuteUploadOutputToS3Bucket(log, Name(), outputPath, config.OutputS3BucketName, config.OutputS3KeyPrefix, false, "", stdoutFilePath, stderrFilePath)
	if len(uploadOutputToS3BucketErrors) > 0 {
		log.Errorf("Unable to upload the logs - %s: %s", pluginID, uploadOutputToS3BucketErrors)
	}
	return
}

// prepareForStart remalshal the Property and stop the plug if it was running before.
func (p *Plugin) prepareForStart(log log.T, config contracts.Configuration, pluginID string, cancelFlag task.CancelFlag) (res contracts.PluginResult, failed bool, property string) {
	// track if the preparation process succeed.

	failed = false
	var err error
	prop := config.Properties

	switch prop.(type) {
	case string:
		break
	default:
		if prop, err = jsonutil.Marshal(config.Properties); err != nil {
			log.Error("Cannot marshal properties, ", err)
		}
	}

	// config.Properties
	if err = jsonutil.Remarshal(prop, &property); err != nil {
		failed = true
		log.Errorf(fmt.Sprintf("Invalid format in plugin configuration - %v;\nError %v", config.Properties, err))
		res = p.CreateResult(fmt.Sprintf("Invalid format in plugin configuration - expecting property as string - %s", config.Properties),
			contracts.ResultStatusFailed)
		return
	}
	//stop the plugin before reconfiguring it
	log.Debugf("Stopping %s - before applying new configuration", pluginID)
	if err = p.lrpm.StopPlugin(pluginID, cancelFlag); err != nil {
		failed = true
		log.Errorf("Unable to stop the plugin - %s: %s", pluginID, err.Error())
		res = p.CreateResult(fmt.Sprintf("Encountered error while stopping the plugin: %s", err.Error()),
			contracts.ResultStatusFailed)
		return
	}
	return
}
