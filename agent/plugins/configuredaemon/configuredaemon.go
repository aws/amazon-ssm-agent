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
	"github.com/aws/amazon-ssm-agent/agent/framework/processor/executer/iohandler"
	"github.com/aws/amazon-ssm-agent/agent/jsonutil"
	"github.com/aws/amazon-ssm-agent/agent/longrunning/manager"
	managerContracts "github.com/aws/amazon-ssm-agent/agent/longrunning/plugin"
	"github.com/aws/amazon-ssm-agent/agent/longrunning/plugin/rundaemon"
	"github.com/aws/amazon-ssm-agent/agent/task"
)

// Plugin is the type for the lrpm invoker plugin.
type Plugin struct {
	lrpm manager.T
}

// NewPlugin returns lrpminvoker
func NewPlugin() (*Plugin, error) {
	var plugin Plugin
	var err error
	//getting the reference of LRPM - long running plugin manager - which manages all long running plugins
	plugin.lrpm, err = manager.GetInstance()
	return &plugin, err
}

func (p *Plugin) Execute(context context.T, config contracts.Configuration, cancelFlag task.CancelFlag, output iohandler.IOHandler) {
	log := context.Log()
	log.Infof("%v started with configuration %v", Name(), config)
	if cancelFlag.ShutDown() {
		output.MarkAsShutdown()
	} else if cancelFlag.Canceled() {
		output.MarkAsCancelled()
	} else {
		runConfigureDaemon(p, context, config.Properties, config.OrchestrationDirectory, config.DefaultWorkingDirectory, cancelFlag, output)
	}
	return
}

func runConfigureDaemon(
	p *Plugin,
	context context.T,
	rawPluginInput interface{},
	orchestrationDir string,
	daemonWorkingDir string,
	cancelFlag task.CancelFlag,
	output iohandler.IOHandler) {

	// TODO:DAEMON: we're using the command line in a lot of places, we probably only need it in the rundaemon plugin or in the call to startplugin
	var input rundaemon.ConfigureDaemonPluginInput
	var err error
	if err = jsonutil.Remarshal(rawPluginInput, &input); err != nil {
		output.SetStatus(contracts.ResultStatusFailed)
		return
	}

	if input.PackageLocation == "" {
		input.PackageLocation = daemonWorkingDir
	}

	if err = rundaemon.ValidateDaemonInput(input); err != nil {
		output.AppendErrorf("\nconfigureDaemon input invalid: %v", err.Error())
		output.SetStatus(contracts.ResultStatusFailed)
		return
	}

	daemonFilePath := filepath.Join(appconfig.DaemonRoot, fmt.Sprintf("%v.json", input.Name))
	// make sure directory for ssm daemons exists
	if err := fileutil.MakeDirs(appconfig.DaemonRoot); err != nil {
		output.AppendErrorf("\nUnable to create ssm daemon folder %v: %v", appconfig.DaemonRoot, err.Error())
		output.SetStatus(contracts.ResultStatusFailed)
		return
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
			output.AppendErrorf("\nFailed to register ssm daemon %v: %v", input.Name, errDaemonDoc.Error())
			output.SetStatus(contracts.ResultStatusFailed)
			return
		}

		p.lrpm.EnsurePluginRegistered(input.Name, plugin)
		p.lrpm.StopPlugin(input.Name, cancelFlag)
		if errStart := p.lrpm.StartPlugin(input.Name, input.Command, orchestrationDir, cancelFlag, output); errStart != nil {
			output.AppendErrorf("\nFailed to start ssm daemon %v: %v", input.Name, err.Error())
			output.SetStatus(contracts.ResultStatusFailed)
			return
		}
	case "Stop":
		if !fileutil.Exists(daemonFilePath) {
			output.AppendErrorf("\nNo ssm daemon %v exists", input.Name)
			output.SetStatus(contracts.ResultStatusFailed)
			return
		}
		err := p.lrpm.StopPlugin(input.Name, cancelFlag)
		if err != nil {
			output.AppendErrorf("\nFailed to stop ssm daemon %v: %v", input.Name, err.Error())
			output.SetStatus(contracts.ResultStatusFailed)
			return
		}
		output.AppendInfof("\nDaemon %v stopped", input.Name)
	case "Remove":
		if fileutil.Exists(daemonFilePath) {
			p.lrpm.StopPlugin(input.Name, cancelFlag)
			fileutil.DeleteFile(daemonFilePath)
			output.AppendInfof("\nDaemon %v removed", input.Name)
		} else {
			output.AppendInfof("\nDaemon %v is not installed", input.Name)
		}
	default:
		output.AppendErrorf("\nUnsupported action %v", input.Action)
		output.SetStatus(contracts.ResultStatusFailed)
		return
	}

	output.SetStatus(contracts.ResultStatusSuccess)
	return

}

// Name returns the plugin name
func Name() string {
	return appconfig.PluginNameAwsConfigureDaemon
}
