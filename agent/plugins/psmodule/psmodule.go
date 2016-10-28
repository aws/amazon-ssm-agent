// Copyright 2016 Amazon.com, Inc. or its affiliates. All Rights Reserved.
//
// Licensed under the AWS Customer Agreement (the "License"). You may
// not use this file except in compliance with the License. A copy of the
// License is located at
//
// http://aws.amazon.com/agreement/

// Package psmodule implements the power shell module plugin.
//
// +build windows

package psmodule

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"time"

	"github.com/aws/amazon-ssm-agent/agent/appconfig"
	"github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/aws/amazon-ssm-agent/agent/contracts"
	"github.com/aws/amazon-ssm-agent/agent/executers"
	"github.com/aws/amazon-ssm-agent/agent/fileutil"
	"github.com/aws/amazon-ssm-agent/agent/framework/runpluginutil"
	"github.com/aws/amazon-ssm-agent/agent/jsonutil"
	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/amazon-ssm-agent/agent/parameterstore"
	"github.com/aws/amazon-ssm-agent/agent/plugins/pluginutil"
	"github.com/aws/amazon-ssm-agent/agent/rebooter"
	"github.com/aws/amazon-ssm-agent/agent/task"
)

// PowerShellModulesDirectory is the directory where PowerShell Modules are installed
var PowerShellModulesDirectory = filepath.Join(os.Getenv("SystemRoot"), "System32", "WindowsPowerShell", "v1.0", "Modules")

// Plugin is the type for the psmodule plugin.
type Plugin struct {
	pluginutil.DefaultPlugin
}

// RunCommandPluginInput represents one set of commands executed by the RunCommand plugin.
type PSModulePluginInput struct {
	contracts.PluginInput
	RunCommand       interface{}
	ParsedCommands   []string
	ID               string
	WorkingDirectory string
	TimeoutSeconds   interface{}
	Source           string
	SourceHash       string
	SourceHashType   string
}

// PSModulePluginOutput represents the output of the plugin
type PSModulePluginOutput struct {
	contracts.PluginOutput
}

// Failed marks plugin as Failed
func (out *PSModulePluginOutput) MarkAsFailed(log log.T, err error) {
	out.ExitCode = 1
	out.Status = contracts.ResultStatusFailed
	if len(out.Stderr) != 0 {
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

	exec := executers.ShellCommandExecuter{}
	plugin.ExecuteCommand = pluginutil.CommandExecuter(exec.Execute)

	return &plugin, nil
}

// Name returns the name of the plugin
func Name() string {
	return appconfig.PluginNameAwsPowerShellModule
}

// Execute runs multiple sets of commands and returns their outputs.
// res.Output will contain a slice of RunCommandPluginOutput.
func (p *Plugin) Execute(context context.T, config contracts.Configuration, cancelFlag task.CancelFlag, subDocumentRunner runpluginutil.PluginRunner) (res contracts.PluginResult) {
	log := context.Log()
	log.Infof("%v started with configuration %v", Name(), config)
	res.StartDateTime = time.Now()
	defer func() { res.EndDateTime = time.Now() }()

	//loading Properties as list since aws:psModule uses properties as list
	var properties []interface{}
	if properties, res = pluginutil.LoadParametersAsList(log, config.Properties); res.Code != 0 {

		pluginutil.PersistPluginInformationToCurrent(log, config.PluginID, config, res)
		return res
	}

	out := make([]PSModulePluginOutput, len(properties))
	for i, prop := range properties {
		// check if a reboot has been requested
		if rebooter.RebootRequested() {
			log.Info("A plugin has requested a reboot.")
			return
		}

		if cancelFlag.ShutDown() {
			out[i].ExitCode = 1
			out[i].Status = contracts.ResultStatusFailed
			break
		}

		if cancelFlag.Canceled() {
			out[i].ExitCode = 1
			out[i].Status = contracts.ResultStatusCancelled
			break
		}

		out[i] = p.runCommandsRawInput(log, prop, config.OrchestrationDirectory, cancelFlag, config.OutputS3BucketName, config.OutputS3KeyPrefix)
	}

	// TODO: (manoghos) here we have to do more result processing, where individual sub properties results are merged smartly into plugin response.
	// Currently we have only one work.
	if len(properties) > 0 {
		res.Code = out[0].ExitCode
		res.Status = out[0].Status
		res.Output = out[0].String()
	}

	pluginutil.PersistPluginInformationToCurrent(log, config.PluginID, config, res)

	return res
}

// runCommandsRawInput executes one set of commands and returns their output.
// The input is in the default json unmarshal format (e.g. map[string]interface{}).
func (p *Plugin) runCommandsRawInput(log log.T, rawPluginInput interface{}, orchestrationDirectory string, cancelFlag task.CancelFlag, outputS3BucketName string, outputS3KeyPrefix string) (out PSModulePluginOutput) {
	var pluginInput PSModulePluginInput
	err := jsonutil.Remarshal(rawPluginInput, &pluginInput)
	log.Debugf("Plugin input %v", pluginInput)
	if err != nil {
		errorString := fmt.Errorf("Invalid format in plugin properties %v;\nerror %v", rawPluginInput, err)
		out.MarkAsFailed(log, errorString)
		return
	}

	pluginInput.ParsedCommands = pluginutil.ParseRunCommand(pluginInput.RunCommand, pluginInput.ParsedCommands)
	return p.runCommands(log, pluginInput, orchestrationDirectory, cancelFlag, outputS3BucketName, outputS3KeyPrefix)
}

// runCommands executes one set of commands and returns their output.
func (p *Plugin) runCommands(log log.T, pluginInput PSModulePluginInput, orchestrationDirectory string, cancelFlag task.CancelFlag, outputS3BucketName string, outputS3KeyPrefix string) (out PSModulePluginOutput) {
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

	orchestrationDir := fileutil.BuildPath(orchestrationDirectory, pluginInput.ID)
	log.Debugf("Running commands %v in workingDirectory %v; orchestrationDir %v ", pluginInput.ParsedCommands, pluginInput.WorkingDirectory, orchestrationDir)

	// create orchestration dir if needed
	if err = fileutil.MakeDirsWithExecuteAccess(orchestrationDir); err != nil {
		log.Debug("failed to create orchestrationDir directory", orchestrationDir)
		out.Errors = append(out.Errors, err.Error())
		return
	}

	// Create script file path
	scriptPath := filepath.Join(orchestrationDir, pluginutil.RunCommandScriptName)
	log.Debugf("Writing commands %v to file %v", pluginInput, scriptPath)

	// Create script file
	// Resolve ssm parameters
	// This may contain sensitive information, do not log this data after resolving.
	if pluginInput.ParsedCommands, err = parameterstore.ResolveStringList(log, pluginInput.ParsedCommands); err != nil {
		out.Errors = append(out.Errors, err.Error())
		log.Errorf("Failed to resolve ssm parameters. Error: - %v", err)
		return
	}
	if err = pluginutil.CreateScriptFile(log, scriptPath, pluginInput.ParsedCommands); err != nil {
		out.Errors = append(out.Errors, err.Error())
		log.Errorf("failed to create script file. %v", err)
		return
	}

	// Resolve ssm parameters
	// This may contain sensitive information, do not log this data after resolving.
	if pluginInput.Source, err = parameterstore.ResolveString(log, pluginInput.Source); err != nil {
		out.Errors = append(out.Errors, err.Error())
		log.Errorf("Failed to resolve ssm parameters. Error: - %v", err)
		return
	}

	// Resolve ssm parameters
	// This may contain sensitive information, do not log this data after resolving.
	if pluginInput.SourceHash, err = parameterstore.ResolveString(log, pluginInput.SourceHash); err != nil {
		out.Errors = append(out.Errors, err.Error())
		log.Errorf("Failed to resolve ssm parameters. Error: - %v", err)
		return
	}

	// Resolve ssm parameters
	// This may contain sensitive information, do not log this data after resolving.
	if pluginInput.SourceHashType, err = parameterstore.ResolveString(log, pluginInput.SourceHashType); err != nil {
		out.Errors = append(out.Errors, err.Error())
		log.Errorf("Failed to resolve ssm parameters. Error: - %v", err)
		return
	}

	if pluginInput.Source != "" {
		// Download file from source if available
		downloadOutput, err := pluginutil.DownloadFileFromSource(log, pluginInput.Source, pluginInput.SourceHash, pluginInput.SourceHashType)
		if err != nil || downloadOutput.IsHashMatched == false || downloadOutput.LocalFilePath == "" {
			errorString := fmt.Errorf("failed to download file reliably %v", pluginInput.Source)
			out.MarkAsFailed(log, errorString)
			return
		} else {
			// Uncompress the zip file received
			fileutil.Uncompress(downloadOutput.LocalFilePath, PowerShellModulesDirectory)
		}
	}

	// Set execution time
	executionTimeout := pluginutil.ValidateExecutionTimeout(log, pluginInput.TimeoutSeconds)

	// Create output file paths
	stdoutFilePath := filepath.Join(orchestrationDir, p.StdoutFileName)
	stderrFilePath := filepath.Join(orchestrationDir, p.StderrFileName)
	log.Debugf("stdout file %v, stderr file %v", stdoutFilePath, stderrFilePath)

	// Construct Command Name and Arguments
	commandName := pluginutil.GetShellCommand()
	commandArguments := append(pluginutil.GetShellArguments(), scriptPath, pluginutil.ExitCodeTrap)

	// Resolve ssm parameters
	// This may contain sensitive information, do not log this data after resolving.
	if pluginInput.WorkingDirectory, err = parameterstore.ResolveString(log, pluginInput.WorkingDirectory); err != nil {
		out.Errors = append(out.Errors, err.Error())
		log.Errorf("Failed to resolve ssm parameters. Error: - %v", err)
		return
	}

	// Execute Command
	stdout, stderr, exitCode, errs := p.ExecuteCommand(log, pluginInput.WorkingDirectory, stdoutFilePath, stderrFilePath, cancelFlag, executionTimeout, commandName, commandArguments)

	// Set output status
	out.ExitCode = exitCode
	out.Status = pluginutil.GetStatus(out.ExitCode, cancelFlag)

	if len(errs) > 0 {
		for _, err := range errs {
			out.Errors = append(out.Errors, err.Error())
			if out.Status != contracts.ResultStatusCancelled &&
				out.Status != contracts.ResultStatusTimedOut &&
				out.Status != contracts.ResultStatusSuccessAndReboot {
				log.Error("failed to run commands: ", err)
				out.Status = contracts.ResultStatusFailed
			}
		}
	}

	// read (a prefix of) the standard output/error
	out.Stdout, err = pluginutil.ReadPrefix(stdout, p.MaxStdoutLength, p.OutputTruncatedSuffix)
	if err != nil {
		out.Errors = append(out.Errors, err.Error())
		log.Error(err)
	}
	out.Stderr, err = pluginutil.ReadPrefix(stderr, p.MaxStderrLength, p.OutputTruncatedSuffix)
	if err != nil {
		out.Errors = append(out.Errors, err.Error())
		log.Error(err)
	}

	// Upload output to S3
	uploadOutputToS3BucketErrors := p.ExecuteUploadOutputToS3Bucket(log, pluginInput.ID, orchestrationDir, outputS3BucketName, outputS3KeyPrefix, useTempDirectory, tempDir, out.Stdout, out.Stderr)
	out.Errors = append(out.Errors, uploadOutputToS3BucketErrors...)

	// Return Json indented response
	responseContent, _ := jsonutil.Marshal(out)
	log.Debug("Returning response:\n", jsonutil.Indent(responseContent))
	return
}
