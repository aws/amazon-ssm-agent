// Copyright 2016 Amazon.com, Inc. or its affiliates. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License"). You may not
// use this file except in compliance with the License. A copy of the
// License is located at
//
// http://aws.amazon.com/apache2.0/
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or
// implied. See the License for the specific language governing
// permissions and limitations under the License.

// Package configuredaemon implements the ConfigureDaemon plugin.
package configuredaemon

import (
	"fmt"
	"path/filepath"

	"github.com/aws/amazon-ssm-agent/agent/appconfig"
	"github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/aws/amazon-ssm-agent/agent/contracts"
	"github.com/aws/amazon-ssm-agent/agent/fileutil"
	"github.com/aws/amazon-ssm-agent/agent/jsonutil"
	"github.com/aws/amazon-ssm-agent/agent/longrunning/manager"
	managerContracts "github.com/aws/amazon-ssm-agent/agent/longrunning/plugin"
	"github.com/aws/amazon-ssm-agent/agent/longrunning/plugin/rundaemon"
	"github.com/aws/amazon-ssm-agent/agent/plugins/pluginutil"
	"github.com/aws/amazon-ssm-agent/agent/task"
)

// Plugin is the type for the lrpm invoker plugin.
type Plugin struct {
	pluginutil.DefaultPlugin
	lrpm manager.T
}

// NewPlugin returns lrpminvoker
func NewPlugin(pluginConfig pluginutil.PluginConfig) (*Plugin, error) {
	var plugin Plugin
	var err error
	plugin.MaxStdoutLength = pluginConfig.MaxStdoutLength
	plugin.MaxStderrLength = pluginConfig.MaxStderrLength
	plugin.StdoutFileName = pluginConfig.StdoutFileName
	plugin.StderrFileName = pluginConfig.StderrFileName
	plugin.OutputTruncatedSuffix = pluginConfig.OutputTruncatedSuffix
	plugin.ExecuteUploadOutputToS3Bucket = pluginutil.UploadOutputToS3BucketExecuter(plugin.UploadOutputToS3Bucket)

	//getting the reference of LRPM - long running plugin manager - which manages all long running plugins
	plugin.lrpm, err = manager.GetInstance()
	return &plugin, err
}

// Execute runs multiple sets of commands and returns their outputs.
// res.Output will contain a slice of RunCommandPluginOutput.
func (p *Plugin) Execute(context context.T, config contracts.Configuration, cancelFlag task.CancelFlag) (res contracts.PluginResult) {
	log := context.Log()

	var properties []interface{}
	if properties = pluginutil.LoadParametersAsList(log, config.Properties, &res); res.Code != 0 {
		return res
	}

	out := contracts.PluginOutput{}
	for _, prop := range properties {

		if cancelFlag.ShutDown() {
			out.MarkAsShutdown()
			break
		} else if cancelFlag.Canceled() {
			out.MarkAsCancelled()
			break
		}
		out.Merge(log, runConfigureDaemon(p, context, prop, config.OrchestrationDirectory, config.DefaultWorkingDirectory, cancelFlag))
	}

	res.Code = out.ExitCode
	res.Status = out.Status
	res.Output = out.String()
	res.StandardOutput = pluginutil.StringPrefix(out.Stdout, p.MaxStdoutLength, p.OutputTruncatedSuffix)
	res.StandardError = pluginutil.StringPrefix(out.Stderr, p.MaxStderrLength, p.OutputTruncatedSuffix)

	return res
}

func runConfigureDaemon(
	p *Plugin,
	context context.T,
	rawPluginInput interface{},
	orchestrationDir string,
	daemonWorkingDir string,
	cancelFlag task.CancelFlag) (output contracts.PluginOutput) {
	//log := context.Log()

	// TODO:DAEMON: we're using the command line in a lot of places, we probably only need it in the rundaemon plugin or in the call to startplugin
	var input rundaemon.ConfigureDaemonPluginInput
	var err error
	if err = jsonutil.Remarshal(rawPluginInput, &input); err != nil {
		output.Status = contracts.ResultStatusFailed
		return
	}

	if input.PackageLocation == "" {
		input.PackageLocation = daemonWorkingDir
	}

	if err = rundaemon.ValidateDaemonInput(input); err != nil {
		output.Stderr = fmt.Sprintf("%v\nconfigureDaemon input invalid: %v", output.Stderr, err.Error())
		output.Status = contracts.ResultStatusFailed
		return output
	}

	daemonFilePath := filepath.Join(appconfig.DaemonRoot, fmt.Sprintf("%v.json", input.Name))
	// make sure directory for ssm daemons exists
	if err := fileutil.MakeDirs(appconfig.DaemonRoot); err != nil {
		output.Stderr = fmt.Sprintf("%v\nUnable to create ssm daemon folder %v: %v", output.Stderr, appconfig.DaemonRoot, err.Error())
		output.Status = contracts.ResultStatusFailed
		return output
	}

	switch input.Action {
	case "Start":
		// in longrunning/plugin/plugin.go (which should have the RegisteredPlugins method
		// and call a platform specific helper) as part of exploration of the appconfig.PackageRoot
		// directory tree looking for start.json (or whatever we call the daemon action)
		plugin := managerContracts.Plugin{
			Info: managerContracts.PluginInfo{
				Name:          input.Name,
				Configuration: input.Command,
				State:         managerContracts.PluginState{IsEnabled: true},
			},
			Handler: &rundaemon.Plugin{
				ExeLocation: input.PackageLocation,
				Name:        input.Name,
				CommandLine: input.Command,
			},
		}

		// TODO:MF: make deps file to support mocking filesystem dependency
		// Save daemon configuration file
		var errDaemonDoc error
		var ssmDaemonDoc string
		if ssmDaemonDoc, errDaemonDoc = jsonutil.Marshal(input); errDaemonDoc == nil {
			if fileutil.Exists(daemonFilePath) {
				errDaemonDoc = fileutil.DeleteFile(daemonFilePath)
			}
			if errDaemonDoc == nil {
				errDaemonDoc = fileutil.WriteAllText(daemonFilePath, ssmDaemonDoc)
			}
		}
		if errDaemonDoc != nil {
			output.Stderr = fmt.Sprintf("%v\nFailed to register ssm daemon %v: %v", output.Stderr, input.Name, errDaemonDoc.Error())
			output.Status = contracts.ResultStatusFailed
			return output
		}

		p.lrpm.EnsurePluginRegistered(input.Name, plugin)
		p.lrpm.StopPlugin(input.Name, cancelFlag)
		if errStart := p.lrpm.StartPlugin(input.Name, input.Command, orchestrationDir, cancelFlag); errStart != nil {
			output.Stderr = fmt.Sprintf("%v\nFailed to start ssm daemon %v: %v", output.Stderr, input.Name, err.Error())
			output.Status = contracts.ResultStatusFailed
			return output
		}
	case "Stop":
		if !fileutil.Exists(daemonFilePath) {
			output.Stderr = fmt.Sprintf("%v\nNo ssm daemon %v exists", output.Stderr, input.Name)
			output.Status = contracts.ResultStatusFailed
			return output
		}
		err := p.lrpm.StopPlugin(input.Name, cancelFlag)
		if err != nil {
			output.Stderr = fmt.Sprintf("%v\nFailed to stop ssm daemon %v: %v", output.Stderr, input.Name, err.Error())
			output.Status = contracts.ResultStatusFailed
			return output
		}
		output.Stdout = fmt.Sprintf("%v\nDaemon %v stopped", output.Stdout, input.Name)
	case "Remove":
		if fileutil.Exists(daemonFilePath) {
			p.lrpm.StopPlugin(input.Name, cancelFlag)
			fileutil.DeleteFile(daemonFilePath)
			output.Stdout = fmt.Sprintf("%v\nDaemon %v removed", output.Stdout, input.Name)
		} else {
			output.Stdout = fmt.Sprintf("%v\nDaemon %v is not installed", output.Stdout, input.Name)
		}
	default:
		output.Stderr = fmt.Sprintf("%v\nUnsupported action %v", output.Stderr, input.Action)
		output.Status = contracts.ResultStatusFailed
		return output
	}

	output.Status = contracts.ResultStatusSuccess
	return output

}

// Name returns the plugin name
func Name() string {
	return appconfig.PluginNameAwsConfigureDaemon
}
