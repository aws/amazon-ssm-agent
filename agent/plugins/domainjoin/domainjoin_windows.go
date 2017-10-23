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
// +build windows

// Package domainjoin implements the domainjoin plugin.
package domainjoin

import (
	"bytes"
	"fmt"
	"io"
	"strings"

	"github.com/aws/amazon-ssm-agent/agent/appconfig"
	"github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/aws/amazon-ssm-agent/agent/contracts"
	"github.com/aws/amazon-ssm-agent/agent/fileutil"
	"github.com/aws/amazon-ssm-agent/agent/framework/processor/executer/iohandler"
	"github.com/aws/amazon-ssm-agent/agent/jsonutil"
	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/amazon-ssm-agent/agent/platform"
	"github.com/aws/amazon-ssm-agent/agent/plugins/pluginutil"
	"github.com/aws/amazon-ssm-agent/agent/task"
	"github.com/aws/amazon-ssm-agent/agent/updateutil"
)

const (
	// UpdateExecutionTimeoutInSeconds represents default timeout time for execution update domain join in seconds
	UpdateExecutionTimeoutInSeconds = 60
	// Domain join plugin command arguments
	// DirectoryIdArg represents the directory id for domain join
	DirectoryIdArg = " --directory-id "
	// DirectoryNameArg represents the directory name for domain join
	DirectoryNameArg = " --directory-name "
	// DirectoryOUArg represents the directory OU for domain join
	DirectoryOUArg = " --directory-ou "
	// InstanceRegionArg represents the region of the instance for domain join
	InstanceRegionArg = " --instance-region "
	// DirectoryNameArg represents the dns ip addresses of directory for domain join
	DnsAddressesArgs = " --dns-addresses"
	// DomainJoinPluginName is the name of the executable file of domain join plugin
	DomainJoinPluginExecutableName = "AWS.DomainJoin.exe"
	// ProxyAddress represents the url addresses of proxy
	ProxyAddress = " --proxy-address "
	// NoProxy represents addresses that do not use the proxy.
	NoProxy = " --no-proxy "
	// Default folder name for domain join plugin
	DomainJoinFolderName = "awsDomainJoin"
)

// Makes command as variables, so that we can mock this for unit tests
var makeDir = fileutil.MakeDirs
var makeArgs = makeArguments
var getRegion = platform.Region
var utilExe convert

// Plugin is the type for the domain join plugin.
type Plugin struct {
}

// DomainJoinPluginInput represents one set of commands executed by the Domain join plugin.
type DomainJoinPluginInput struct {
	contracts.PluginInput
	DirectoryId    string
	DirectoryName  string
	DirectoryOU    string
	DnsIpAddresses []string
}

// NewPlugin returns a new instance of the plugin.
func NewPlugin() (*Plugin, error) {
	var plugin Plugin
	return &plugin, nil
}

// Name returns the plugin name
func Name() string {
	return appconfig.PluginNameDomainJoin
}

type convert func(log.T, string, []string, string, string, io.Writer, io.Writer, bool) (string, error)

func (p *Plugin) Execute(context context.T, config contracts.Configuration, cancelFlag task.CancelFlag, output iohandler.IOHandler) {
	log := context.Log()
	log.Infof("%v started with configuration %v", Name(), config)

	var properties map[string]interface{}
	if properties = pluginutil.LoadParametersAsMap(log, config.Properties, output); output.GetExitCode() != 0 {
		return
	}

	if cancelFlag.ShutDown() {
		output.MarkAsShutdown()
	} else if cancelFlag.Canceled() {
		output.MarkAsCancelled()
	} else {
		util := updateutil.Utility{CustomUpdateExecutionTimeoutInSeconds: UpdateExecutionTimeoutInSeconds}
		utilExe = util.NewExeCommandOutput
		p.runCommandsRawInput(log, config.PluginID, properties, config.OrchestrationDirectory, cancelFlag, output, utilExe)

		if output.GetStatus() == contracts.ResultStatusFailed {
			output.AppendInfo("Domain join failed.")
		} else if output.GetStatus() == contracts.ResultStatusSuccess {
			output.AppendInfo("Domain join succeeded.")
		}
	}

	return
}

// runCommandsRawInput executes one set of commands and returns their output.
// The input is in the default json unmarshal format (e.g. map[string]interface{}).
func (p *Plugin) runCommandsRawInput(log log.T, pluginID string, rawPluginInput map[string]interface{}, orchestrationDirectory string, cancelFlag task.CancelFlag, output iohandler.IOHandler, utilExe convert) {
	var pluginInput DomainJoinPluginInput
	err := jsonutil.Remarshal(rawPluginInput, &pluginInput)
	log.Debugf("Plugin input %v", pluginInput)

	if err != nil {
		errorString := fmt.Errorf("Invalid format in plugin properties %v;\nerror %v", rawPluginInput, err)
		output.MarkAsFailed(errorString)
		return
	}
	p.runCommands(log, pluginID, pluginInput, orchestrationDirectory, cancelFlag, output, utilExe)
}

// runCommands executes the command and returns the output.
func (p *Plugin) runCommands(log log.T, pluginID string, pluginInput DomainJoinPluginInput, orchestrationDirectory string, cancelFlag task.CancelFlag, out iohandler.IOHandler, utilExe convert) {
	var err error

	// create orchestration dir if needed
	if err = makeDir(orchestrationDirectory); err != nil {
		log.Debug("failed to create orchestration directory", orchestrationDirectory, err)
		out.MarkAsFailed(err)
		return
	}

	// Construct Command line with executable file name and parameters
	var command string
	if command, err = makeArgs(log, pluginInput); err != nil {
		out.MarkAsFailed(fmt.Errorf("Failed to build domain join command because : %v", err.Error()))
		return
	}

	log.Debugf("command line is : %v", command)
	workingDir := fileutil.BuildPath(appconfig.DefaultPluginPath, DomainJoinFolderName)
	commandParts := strings.Fields(command)
	out.SetStatus(contracts.ResultStatusInProgress)
	var output string
	output, err = utilExe(log,
		commandParts[0],
		commandParts[1:],
		workingDir,
		orchestrationDirectory,
		out.GetStdoutWriter(),
		out.GetStderrWriter(),
		true)

	log.Debugf("code is: %v", output)
	log.Debugf("err is: %v", err)

	if err != nil {
		out.MarkAsFailed(err)
		return
	}

	// TODO:MF: Why is output a string that we parse to determine if a reboot is needed?  Can we shell out and run a command instead of using the updateutil approach?
	// check output to see if the machine needs reboot
	if strings.Contains(output, string(contracts.ResultStatusPassedAndReboot)) ||
		strings.Contains(output, string(contracts.ResultStatusSuccessAndReboot)) {
		out.MarkAsSuccessWithReboot()
		return
	}

	out.MarkAsSucceeded()
	return
}

// makeArguments Build the arguments for domain join plugin
func makeArguments(log log.T, pluginInput DomainJoinPluginInput) (commandArguments string, err error) {

	var buffer bytes.Buffer
	buffer.WriteString("./")
	buffer.WriteString(DomainJoinPluginExecutableName)

	// required parameters for the domain join plugin
	if len(pluginInput.DirectoryId) == 0 {
		return "", fmt.Errorf("directoryId is required")
	}
	buffer.WriteString(DirectoryIdArg)
	buffer.WriteString(pluginInput.DirectoryId)

	if len(pluginInput.DirectoryName) == 0 {
		return "", fmt.Errorf("directoryName is required")
	}
	buffer.WriteString(DirectoryNameArg)
	buffer.WriteString(pluginInput.DirectoryName)

	buffer.WriteString(InstanceRegionArg)
	region, err := getRegion()
	if err != nil {
		return "", fmt.Errorf("cannot get the instance region information")
	}
	buffer.WriteString(region)

	// check if user provides the directory OU parameter
	if len(pluginInput.DirectoryOU) != 0 {
		log.Debugf("Customized directory OU parameter provided: %v", pluginInput.DirectoryOU)
		buffer.WriteString(DirectoryOUArg)
		buffer.WriteString("'")
		buffer.WriteString(pluginInput.DirectoryOU)
		buffer.WriteString("'")
	}

	value, _, err := pluginutil.LocalRegistryKeyGetStringsValue(appconfig.ItemPropertyPath, appconfig.ItemPropertyName)
	if err != nil {
		log.Debug("Cannot find customized proxy setting.")
	}
	// if user has customized proxy setting
	if (err == nil) && (len(value) != 0) {
		url, noProxy := pluginutil.GetProxySetting(value)
		if len(url) != 0 {
			buffer.WriteString(ProxyAddress)
			buffer.WriteString(url)
		}

		if len(noProxy) != 0 {
			buffer.WriteString(NoProxy)
			buffer.WriteString(noProxy)
		}
	}

	if len(pluginInput.DnsIpAddresses) == 0 {
		log.Debug("Do not provide dns addresses.")
		return buffer.String(), nil
	}

	buffer.WriteString(DnsAddressesArgs)
	for index := 0; index < len(pluginInput.DnsIpAddresses); index++ {
		buffer.WriteString(" ")
		buffer.WriteString(pluginInput.DnsIpAddresses[index])
	}

	return buffer.String(), nil
}
