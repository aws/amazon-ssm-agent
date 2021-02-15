// Copyright 2020 Amazon.com, Inc. or its affiliates. All Rights Reserved.
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
// +build freebsd linux netbsd openbsd darwin

// Package domainjoin implements the domainjoin plugin.
package domainjoin

import (
	"bytes"
	"fmt"
	"io"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/aws/amazon-ssm-agent/agent/appconfig"
	"github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/aws/amazon-ssm-agent/agent/contracts"
	"github.com/aws/amazon-ssm-agent/agent/fileutil"
	"github.com/aws/amazon-ssm-agent/agent/framework/processor/executer/iohandler"
	"github.com/aws/amazon-ssm-agent/agent/jsonutil"
	"github.com/aws/amazon-ssm-agent/agent/log"
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
	DomainJoinPluginExecutableName = "aws_domainjoin.sh"
	// ProxyAddress represents the url addresses of proxy
	ProxyAddress = " --proxy-address "
	// NoProxy represents addresses that do not use the proxy.
	NoProxy = " --no-proxy "
	// Default folder name for domain join plugin
	DomainJoinFolderName = "awsDomainJoin"
	// KeepHostName is a flag to retain instance hostnames as assigned (by customers).
	KeepHostNameArgs = " --keep-hostname "
)

// Makes command as variables, so that we can mock this for unit tests
var makeDir = fileutil.MakeDirs
var makeArgs = makeArguments
var utilExe convert
var createOrchesDir = createOrchestrationDir

// Plugin is the type for the domain join plugin.
type Plugin struct {
	context context.T
}

// DomainJoinPluginInput represents one set of commands executed by the Domain join plugin.
type DomainJoinPluginInput struct {
	contracts.PluginInput
	DirectoryId    string
	DirectoryName  string
	DirectoryOU    string
	DnsIpAddresses []string
	KeepHostName   bool
}

// NewPlugin returns a new instance of the plugin.
func NewPlugin(context context.T) (*Plugin, error) {
	return &Plugin{
		context: context,
	}, nil
}

// Name returns the plugin name
func Name() string {
	return appconfig.PluginNameDomainJoin
}

type convert func(log.T, string, []string, string, string, io.Writer, io.Writer, bool) (string, error)

func (p *Plugin) Execute(config contracts.Configuration, cancelFlag task.CancelFlag, output iohandler.IOHandler) {
	log := p.context.Log()
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
		p.runCommandsRawInput(config.PluginID, properties, config.OrchestrationDirectory, cancelFlag, output, utilExe)

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
func (p *Plugin) runCommandsRawInput(pluginID string, rawPluginInput map[string]interface{}, orchestrationDirectory string, cancelFlag task.CancelFlag, output iohandler.IOHandler, utilExe convert) {
	var pluginInput DomainJoinPluginInput
	err := jsonutil.Remarshal(rawPluginInput, &pluginInput)
	p.context.Log().Debugf("Plugin input %v", pluginInput)

	if err != nil {
		errorString := fmt.Errorf("Invalid format in plugin properties %v;\nerror %v", rawPluginInput, err)
		output.MarkAsFailed(errorString)
		return
	}
	p.runCommands(pluginID, pluginInput, orchestrationDirectory, cancelFlag, output, utilExe)
}

// createOrchestrationDir Make orchestration dir to copy domainjoin script
func createOrchestrationDir(log log.T, orchestrationDir string, pluginInput DomainJoinPluginInput) (scriptPath string, err error) {

	// create orchestration dir if needed
	if err = fileutil.MakeDirsWithExecuteAccess(orchestrationDir); err != nil {
		return "", fmt.Errorf("failed to create orchestrationDir directory, %v", orchestrationDir)
	}

	// Create script file path
	scriptPath = filepath.Join(orchestrationDir, DomainJoinPluginExecutableName)
	log.Infof("Writing commands %v to file %v", pluginInput, scriptPath)

	// Create script file
	var awsDomainJoinScript = getDomainJoinScript()
	if err = pluginutil.CreateScriptFile(log, scriptPath, awsDomainJoinScript, fileutil.ByteOrderMarkSkip); err != nil {
		return "", fmt.Errorf("failed to create script file. %v", err)
	}

	return scriptPath, nil
}

// runCommands executes the command and returns the output.
func (p *Plugin) runCommands(pluginID string, pluginInput DomainJoinPluginInput, orchestrationDirectory string, cancelFlag task.CancelFlag, out iohandler.IOHandler, utilExe convert) {
	var err error
	var scriptPath string
	log := p.context.Log()
	if scriptPath, err = createOrchesDir(log, orchestrationDirectory, pluginInput); err != nil {
		out.MarkAsFailed(fmt.Errorf("Failed to create orchestration directory because : %v", err.Error()))
		return
	}

	log.Debugf("Running commands in orchestrationDir %v ", orchestrationDirectory)

	// Construct Command line with executable file name and parameters
	var command string
	if command, err = makeArgs(p.context, scriptPath, pluginInput); err != nil {
		out.MarkAsFailed(fmt.Errorf("Failed to build domain join command because : %v", err.Error()))
		return
	}

	log.Infof("command line is : %v", command)
	commandParts := strings.Fields(command)
	out.SetStatus(contracts.ResultStatusInProgress)
	var output string
	output, err = utilExe(log,
		commandParts[0],
		commandParts[1:],
		orchestrationDirectory,
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

	if strings.Contains(output, string(contracts.ResultStatusPassedAndReboot)) ||
		strings.Contains(output, string(contracts.ResultStatusSuccessAndReboot)) {
		out.MarkAsSuccessWithReboot()
		return
	}

	out.MarkAsSucceeded()
	return
}

func isShellInjection(arg string) bool {
	var backtick, _ = regexp.Compile("`")
	matched := backtick.MatchString(arg)
	if matched == true {
		return true
	}

	var shellCmd, _ = regexp.Compile(`\$\(`)
	matched = shellCmd.MatchString(arg)
	if matched == true {
		return true
	}

	return false
}

func isMatchingIPAddress(arg string) bool {
	// Regex from AWS-JoinDirectoryServiceDomain SSM doc

	var ipRegex, _ = regexp.Compile("^((25[0-5]|2[0-4][0-9]|[01]?[0-9][0-9]?)\\.){3}(25[0-5]|2[0-4][0-9]|[01]?[0-9][0-9]?)$")
	err := ipRegex.MatchString(arg)

	return err
}

// makeArguments Build the arguments for domain join plugin
func makeArguments(context context.T, scriptPath string, pluginInput DomainJoinPluginInput) (commandArguments string, err error) {

	var buffer bytes.Buffer
	buffer.WriteString(scriptPath)
	log := context.Log()

	// required parameters for the domain join plugin
	if len(pluginInput.DirectoryId) == 0 {
		return "", fmt.Errorf("directoryId is required")
	}

	if isShellInjection(pluginInput.DirectoryId) {
		return "", fmt.Errorf("Shell command injection string " + pluginInput.DirectoryId)
	}

	buffer.WriteString(DirectoryIdArg)
	buffer.WriteString(pluginInput.DirectoryId)

	if len(pluginInput.DirectoryName) == 0 {
		return "", fmt.Errorf("directoryName is required")
	}

	if isShellInjection(pluginInput.DirectoryName) {
		return "", fmt.Errorf("Shell command injection string " + pluginInput.DirectoryName)
	}
	buffer.WriteString(DirectoryNameArg)
	buffer.WriteString(pluginInput.DirectoryName)

	region, err := context.Identity().Region()
	if err != nil || region == "" {
		return "", fmt.Errorf("cannot get the instance region information")
	} else {
		buffer.WriteString(InstanceRegionArg)
		buffer.WriteString(region)
	}

	if isShellInjection(region) {
		return "", fmt.Errorf("Shell command injection string " + region)
	}

	// check if user provides the directory OU parameter
	if len(pluginInput.DirectoryOU) != 0 {
		log.Debugf("Customized directory OU parameter provided: %v", pluginInput.DirectoryOU)
		buffer.WriteString(DirectoryOUArg)
		buffer.WriteString(pluginInput.DirectoryOU)
	}

	if isShellInjection(pluginInput.DirectoryOU) {
		return "", fmt.Errorf("Shell command injection string " + pluginInput.DirectoryName)
	}

	if len(pluginInput.DnsIpAddresses) == 0 {
		log.Debug("Do not provide dns addresses.")
		return buffer.String(), nil
	}

	buffer.WriteString(DnsAddressesArgs)
	buffer.WriteString(" ")
	for index := 0; index < len(pluginInput.DnsIpAddresses); index++ {
		if index != 0 {
			buffer.WriteString(",")
		}
		if isShellInjection(pluginInput.DnsIpAddresses[index]) {
			return "", fmt.Errorf("Shell command injection string " + pluginInput.DnsIpAddresses[index])
		}
		matchesIPPat := isMatchingIPAddress(pluginInput.DnsIpAddresses[index])
		if matchesIPPat {
			buffer.WriteString(pluginInput.DnsIpAddresses[index])
		} else {
			return "", fmt.Errorf("Invalid DNS IP address " + pluginInput.DnsIpAddresses[index])
		}
	}

	if pluginInput.KeepHostName {
		buffer.WriteString(KeepHostNameArgs)
		buffer.WriteString(" ")
	}

	return buffer.String(), nil
}
