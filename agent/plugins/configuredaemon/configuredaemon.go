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
	"strings"

	"github.com/aws/amazon-ssm-agent/agent/appconfig"
	"github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/aws/amazon-ssm-agent/agent/contracts"
	"github.com/aws/amazon-ssm-agent/agent/fileutil"
	"github.com/aws/amazon-ssm-agent/agent/framework/runpluginutil"
	"github.com/aws/amazon-ssm-agent/agent/jsonutil"
	"github.com/aws/amazon-ssm-agent/agent/longrunning/manager"
	managerContracts "github.com/aws/amazon-ssm-agent/agent/longrunning/plugin"
	"github.com/aws/amazon-ssm-agent/agent/longrunning/plugin/rundaemon"
	"github.com/aws/amazon-ssm-agent/agent/plugins/pluginutil"
	"github.com/aws/amazon-ssm-agent/agent/rebooter"
	"github.com/aws/amazon-ssm-agent/agent/task"
)

// Plugin is the type for the lrpm invoker plugin.
type Plugin struct {
	pluginutil.DefaultPlugin
	lrpm manager.T
}

// ConfigureDaemonPluginOutput represents the output of the plugin.
type ConfigureDaemonPluginOutput struct {
	contracts.PluginOutput
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
	plugin.Uploader = pluginutil.GetS3Config()
	plugin.ExecuteUploadOutputToS3Bucket = pluginutil.UploadOutputToS3BucketExecuter(plugin.UploadOutputToS3Bucket)

	//getting the reference of LRPM - long running plugin manager - which manages all long running plugins
	plugin.lrpm, err = manager.GetInstance()
	return &plugin, err
}

// Execute runs multiple sets of commands and returns their outputs.
// res.Output will contain a slice of RunCommandPluginOutput.
func (p *Plugin) Execute(context context.T, config contracts.Configuration, cancelFlag task.CancelFlag, subDocumentRunner runpluginutil.PluginRunner) (res contracts.PluginResult) {
	log := context.Log()

	var properties []interface{}
	if properties, res = pluginutil.LoadParametersAsList(log, config.Properties); res.Code != 0 {
		pluginutil.PersistPluginInformationToCurrent(log, Name(), config, res)
		return res
	}

	out := make([]ConfigureDaemonPluginOutput, len(properties))
	for i, prop := range properties {
		// check if a reboot has been requested
		if rebooter.RebootRequested() {
			log.Info("A plugin has requested a reboot.")
			break
		}

		if cancelFlag.ShutDown() {
			out[i] = ConfigureDaemonPluginOutput{}
			out[i].Errors = []string{"Execution canceled due to ShutDown"}
			break
		} else if cancelFlag.Canceled() {
			out[i] = ConfigureDaemonPluginOutput{}
			out[i].Errors = []string{"Execution canceled"}
			break
		}
		out[i] = runConfigureDaemon(p, context, prop, config.OrchestrationDirectory, config.DefaultWorkingDirectory, cancelFlag)
	}

	// TODO: instance here we have to do more result processing, where individual sub properties results are merged smartly into plugin response.
	// Currently assuming we have only one work.
	if len(properties) > 0 {
		res.Code = out[0].ExitCode
		res.Status = out[0].Status
		res.Output = out[0].String()
		res.StandardOutput = contracts.TruncateOutput(out[0].Stdout, "", 24000)
		res.StandardError = contracts.TruncateOutput(out[0].Stderr, "", 8000)
	}

	pluginutil.PersistPluginInformationToCurrent(log, Name(), config, res)

	return res
}

func runConfigureDaemon(
	p *Plugin,
	context context.T,
	rawPluginInput interface{},
	orchestrationDir string,
	daemonWorkingDir string,
	cancelFlag task.CancelFlag) (output ConfigureDaemonPluginOutput) {
	//log := context.Log()

	var input rundaemon.DaemonPluginInput
	var WorkingDir string
	var err error
	if err = jsonutil.Remarshal(rawPluginInput, &input); err != nil {
		output.Status = contracts.ResultStatusFailed
		return
	}

	if input.PackageLocation != "" {
		WorkingDir = input.PackageLocation
	} else {
		WorkingDir = daemonWorkingDir
	}

	// TODO:DAEMON: we're using the command line in a lot of places, we probably only need it in the rundaemon plugin or in the call to startplugin
	// TODO:DAEMON: Validate input
	switch input.Action {
	case "Start":
		// TODO:DAEMON: this creation and registration of the plugins need to also happen
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
				ExeLocation: WorkingDir,
				Name:        input.Name,
				CommandLine: input.Command,
			},
		}
		if strings.HasPrefix(daemonWorkingDir, appconfig.PackageRoot) {
			// TODO:MF: make deps file to support mocking filesystem depedency
			var errDaemonDoc error
			if ssmDaemonDoc, errDaemonDoc := jsonutil.Marshal(input); errDaemonDoc == nil {
				daemonFileName := filepath.Join(daemonWorkingDir, "ssm-daemon.json")
				if fileutil.Exists(daemonFileName) {
					errDaemonDoc = fileutil.DeleteFile(daemonFileName)
				}
				if errDaemonDoc == nil {
					errDaemonDoc = fileutil.WriteAllText(daemonFileName, ssmDaemonDoc)
				}
			}
			if errDaemonDoc != nil {
				output.Stderr = fmt.Sprintf("%v\n%v", output.Stderr, fmt.Sprintf("Failed to register ssm daemon: %v", errDaemonDoc.Error()))
			}
		}
		p.lrpm.EnsurePluginRegistered(input.Name, plugin)
		// TODO need to test this
		//p.lrpm.StopPlugin(input.Name, cancelFlag)
		p.lrpm.StartPlugin(input.Name, input.Command, orchestrationDir, cancelFlag)
	case "Stop":
		p.lrpm.StopPlugin(input.Name, cancelFlag)
	}
	output.Status = contracts.ResultStatusSuccess
	return output

}

// Name returns the plugin name
func Name() string {
	return appconfig.PluginNameAwsConfigureDaemon
}
