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
//
// Package domainjoin implements the domainjoin plugin.
package domainjoin

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"path/filepath"
	"strings"
	"time"

	"github.com/aws/amazon-ssm-agent/agent/appconfig"
	"github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/aws/amazon-ssm-agent/agent/contracts"
	"github.com/aws/amazon-ssm-agent/agent/fileutil"
	"github.com/aws/amazon-ssm-agent/agent/framework/runpluginutil"
	"github.com/aws/amazon-ssm-agent/agent/jsonutil"
	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/amazon-ssm-agent/agent/parameterstore"
	"github.com/aws/amazon-ssm-agent/agent/platform"
	"github.com/aws/amazon-ssm-agent/agent/plugins/pluginutil"
	"github.com/aws/amazon-ssm-agent/agent/rebooter"
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
	// InstanceRegionArg represents the region of the instance for domain join
	InstanceRegionArg = " --instance-region "
	// DirectoryNameArg represents the dns ip addresses of directory for domain join
	DnsAddressesArgs = " --dns-addresses"
	// DomainJoinPluginName is the name of the executable file of domain join plugin
	DomainJoinPluginExecutableName = "AWS.DomainJoin.exe"
	// Default folder name for logs
	OutputFolder = "output"
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
	pluginutil.DefaultPlugin
}

// DomainJoinPluginInput represents one set of commands executed by the Domain join plugin.
type DomainJoinPluginInput struct {
	contracts.PluginInput
	DirectoryId    string
	DirectoryName  string
	DirectoryOU    string
	DnsIpAddresses []string
}

// DomainJoinPluginOutput represents the output of the plugin
type DomainJoinPluginOutput struct {
	contracts.PluginOutput
}

// MarkAsFailed marks plugin as Failed
func (out *DomainJoinPluginOutput) MarkAsFailed(log log.T, err error) {
	out.ExitCode = 1
	out.Status = contracts.ResultStatusFailed
	if out.Stderr != "" {
		out.Stderr = fmt.Sprintf("\n%v\n%v", out.Stderr, err.Error())
	} else {
		out.Stderr = fmt.Sprintf("\n%v", err.Error())
	}
	log.Error(err.Error())
	out.Errors = append(out.Errors, err.Error())
}

// NewPlugin returns a new instance of the plugin.
func NewPlugin(pluginConfig pluginutil.PluginConfig) (*Plugin, error) {
	var plugin Plugin
	plugin.MaxStdoutLength = pluginConfig.MaxStdoutLength
	plugin.MaxStderrLength = pluginConfig.MaxStderrLength
	plugin.StdoutFileName = pluginConfig.StdoutFileName
	plugin.StderrFileName = pluginConfig.StderrFileName
	plugin.OutputTruncatedSuffix = pluginConfig.OutputTruncatedSuffix
	plugin.Uploader = pluginutil.GetS3Config()
	plugin.ExecuteUploadOutputToS3Bucket = pluginutil.UploadOutputToS3BucketExecuter(plugin.UploadOutputToS3Bucket)

	return &plugin, nil
}

// Name returns the plugin name
func Name() string {
	return appconfig.PluginNameDomainJoin
}

type convert func(log.T, string, []string, string, string, string, string, bool) (string, error)

// Execute runs multiple sets of commands and returns their outputs.
// res.Output will contain a slice of RunCommandPluginOutput.
func (p *Plugin) Execute(context context.T, config contracts.Configuration, cancelFlag task.CancelFlag, subDocumentRunner runpluginutil.PluginRunner) (res contracts.PluginResult) {
	log := context.Log()
	log.Infof("%v started with configuration %v", Name(), config)
	res.StartDateTime = time.Now()
	defer func() { res.EndDateTime = time.Now() }()

	//loading Properties as map since aws:domainJoin uses properties as map
	var properties map[string]interface{}
	if properties, res = pluginutil.LoadParametersAsMap(log, config.Properties); res.Code != 0 {
		pluginutil.PersistPluginInformationToCurrent(log, config.PluginID, config, res)
		return res
	}

	msiFailureCount := 0
	atleastOneRequestedReboot := false
	finalStdOut := ""
	finalStdErr := ""
	var out DomainJoinPluginOutput

	if rebooter.RebootRequested() {
		log.Infof("Stopping execution of %v plugin due to an external reboot request.", Name())
		return
	}

	if cancelFlag.ShutDown() {
		res.Code = 1
		res.Status = contracts.ResultStatusFailed
		pluginutil.PersistPluginInformationToCurrent(log, config.PluginID, config, res)
		return
	}

	if cancelFlag.Canceled() {
		res.Code = 1
		res.Status = contracts.ResultStatusCancelled
		pluginutil.PersistPluginInformationToCurrent(log, config.PluginID, config, res)
		return
	}

	util := updateutil.Utility{CustomUpdateExecutionTimeoutInSeconds: UpdateExecutionTimeoutInSeconds}
	utilExe = util.ExeCommandOutput
	out = p.runCommandsRawInput(log, properties, config.OrchestrationDirectory, cancelFlag, config.OutputS3BucketName, config.OutputS3KeyPrefix, utilExe)

	if out.Status == contracts.ResultStatusFailed {
		msiFailureCount++
		if out.Stdout != "" {
			finalStdOut = fmt.Sprintf("%v\n%v", finalStdOut, out.Stdout)
		}

		if out.Stderr != "" {
			finalStdErr = fmt.Sprintf("%v\n%v", finalStdErr, out.Stderr)
		}
	}

	if out.Status == contracts.ResultStatusSuccessAndReboot || out.Status == contracts.ResultStatusPassedAndReboot {
		atleastOneRequestedReboot = true
		res.Code = out.ExitCode
	}

	if atleastOneRequestedReboot {
		res.Status = contracts.ResultStatusSuccessAndReboot
	} else {
		res.Status = contracts.ResultStatusSuccess
		res.Code = 0
	}

	if msiFailureCount > 0 {
		finalStdOut = fmt.Sprintf("Domain join failed. Number of Failures: %v\n%v", msiFailureCount, finalStdOut)
		res.Status = contracts.ResultStatusFailed
		res.Code = 1
	} else {
		finalStdOut = fmt.Sprint("Domain join succeeded.")
	}

	finalOut := contracts.PluginOutput{
		Stdout: finalStdOut,
		Stderr: finalStdErr,
	}

	res.Output = finalOut.String()
	pluginutil.PersistPluginInformationToCurrent(log, config.PluginID, config, res)

	return res

}

// runCommandsRawInput executes one set of commands and returns their output.
// The input is in the default json unmarshal format (e.g. map[string]interface{}).
func (p *Plugin) runCommandsRawInput(log log.T, rawPluginInput map[string]interface{}, orchestrationDirectory string, cancelFlag task.CancelFlag, outputS3BucketName string, outputS3KeyPrefix string, utilExe convert) (out DomainJoinPluginOutput) {
	var pluginInput DomainJoinPluginInput
	err := jsonutil.Remarshal(rawPluginInput, &pluginInput)
	log.Debugf("Plugin input %v", pluginInput)

	if err != nil {
		errorString := fmt.Errorf("Invalid format in plugin properties %v;\nerror %v", rawPluginInput, err)
		out.MarkAsFailed(log, errorString)
		return
	}
	return p.runCommands(log, pluginInput, orchestrationDirectory, cancelFlag, outputS3BucketName, outputS3KeyPrefix, utilExe)
}

// runCommands executes the command and returns the output.
func (p *Plugin) runCommands(log log.T, pluginInput DomainJoinPluginInput, orchestrationDirectory string, cancelFlag task.CancelFlag, outputS3BucketName string, outputS3KeyPrefix string, utilExe convert) (out DomainJoinPluginOutput) {
	var err error

	// if no orchestration directory specified, create temp directory
	var useTempDirectory = (orchestrationDirectory == "")
	var tempDir string
	if useTempDirectory {
		if tempDir, err = ioutil.TempDir("", "Ec2RunCommand"); err != nil {
			out.Errors = append(out.Errors, err.Error())
			log.Error(err)
			return
		}
		orchestrationDirectory = tempDir
	}
	// create orchestration dir if needed
	if err = makeDir(orchestrationDirectory); err != nil {
		log.Debug("failed to create orchestration directory", orchestrationDirectory, err)
		out.Errors = append(out.Errors, err.Error())
		return
	}

	// Create output file paths
	stdoutFilePath := filepath.Join(orchestrationDirectory, OutputFolder, p.StdoutFileName)
	stderrFilePath := filepath.Join(orchestrationDirectory, OutputFolder, p.StderrFileName)
	log.Debugf("stdout file %v, stderr file %v", stdoutFilePath, stderrFilePath)
	log.Debugf("stdoutFilePath file %v, stderrFilePath file %v", stdoutFilePath, stderrFilePath) //check if this is log path

	// Construct Command line with executable file name and parameters
	var command string
	if command, err = makeArgs(log, pluginInput); err != nil {
		log.Errorf("Failed to build domain join command because : %v", err)
		out.ExitCode = 1
		out.Status = contracts.ResultStatusFailed
		out.Stderr = fmt.Sprintf("Failed to build domain join command because : %v", err.Error())
		return
	}

	log.Debugf("command line is : %v", command)
	workingDir := fileutil.BuildPath(appconfig.DefaultPluginPath, DomainJoinFolderName)
	commandParts := strings.Fields(command)
	out.Status = contracts.ResultStatusInProgress
	var output string
	output, err = utilExe(log,
		commandParts[0],
		commandParts[1:],
		workingDir,
		orchestrationDirectory,
		out.Stdout,
		out.Stderr,
		true)

	log.Debugf("stdout is: %v", out.Stdout)
	log.Debugf("stderr is: %v", out.Stderr)
	log.Debugf("stdoutFilePath is: %v", stdoutFilePath)
	log.Debugf("code is: %v", output)
	log.Debugf("err is: %v", err)

	// Upload output to S3
	outputPath := filepath.Join(orchestrationDirectory, OutputFolder)
	uploadOutputToS3BucketErrors := p.ExecuteUploadOutputToS3Bucket(log, Name(), outputPath, outputS3BucketName, outputS3KeyPrefix, useTempDirectory, tempDir, stdoutFilePath, stderrFilePath) // should use out.Stdout and out.Stderr
	out.Errors = append(out.Errors, uploadOutputToS3BucketErrors...)

	if err != nil {
		out.ExitCode = 1
		out.Status = contracts.ResultStatusFailed
		return
	}

	// check output to see if the machine needs reboot
	if strings.Contains(output, string(contracts.ResultStatusPassedAndReboot)) ||
		strings.Contains(output, string(contracts.ResultStatusSuccessAndReboot)) {
		out.ExitCode = 0
		out.Status = contracts.ResultStatusSuccessAndReboot
		return
	}

	out.Status = contracts.ResultStatusSuccess
	out.ExitCode = 0
	return
}

// makeArguments Build the arguments for domain join plugin
func makeArguments(log log.T, pluginInput DomainJoinPluginInput) (commandArguments string, err error) {

	var buffer bytes.Buffer
	buffer.WriteString("./")
	buffer.WriteString(DomainJoinPluginExecutableName)

	// Resolve ssm parameters
	// This may contain sensitive information, do not log this data after resolving.
	if pluginInput.DirectoryId, err = parameterstore.ResolveString(log, pluginInput.DirectoryId); err != nil {
		log.Errorf("Failed to resolve ssm parameters. Error: - %v", err)
		return
	}

	// required parameters for the domain join plugin
	if len(pluginInput.DirectoryId) == 0 {
		return "", fmt.Errorf("directoryId is required")
	}
	buffer.WriteString(DirectoryIdArg)
	buffer.WriteString(pluginInput.DirectoryId)

	// Resolve ssm parameters
	// This may contain sensitive information, do not log this data after resolving.
	if pluginInput.DirectoryName, err = parameterstore.ResolveString(log, pluginInput.DirectoryName); err != nil {
		log.Errorf("Failed to resolve ssm parameters. Error: - %v", err)
		return
	}

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

	// Resolve ssm parameters
	// This may contain sensitive information, do not log this data after resolving.
	if pluginInput.DnsIpAddresses, err = parameterstore.ResolveStringList(log, pluginInput.DnsIpAddresses); err != nil {
		log.Errorf("Failed to resolve ssm parameters. Error: - %v", err)
		return
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
