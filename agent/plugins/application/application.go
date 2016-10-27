// Copyright 2016 Amazon.com, Inc. or its affiliates. All Rights Reserved.
//
// Licensed under the AWS Customer Agreement (the "License"). You may
// not use this file except in compliance with the License. A copy of the
// License is located at
//
// http://aws.amazon.com/agreement/

// Package application implements the application plugin.
//
// +build windows

package application

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
	"github.com/aws/amazon-ssm-agent/agent/jsonutil"
	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/amazon-ssm-agent/agent/parameterstore"
	"github.com/aws/amazon-ssm-agent/agent/plugins/pluginutil"
	"github.com/aws/amazon-ssm-agent/agent/rebooter"
	"github.com/aws/amazon-ssm-agent/agent/task"
)

const (
	// defaultApplicationExecutionTimeoutInSeconds represents default timeout time for execution of applications in seconds
	defaultApplicationExecutionTimeoutInSeconds = 3600

	// defaultWorkingDirectory represents the default working directory
	defaultWorkingDirectory = ""
)

// msiExecCommand is the command for installing msi applications
var msiExecCommand = filepath.Join(os.Getenv("SystemRoot"), "System32", "msiexec.exe")

// Plugin is the type for the applications plugin.
type Plugin struct {
	pluginutil.DefaultPlugin
}

// ApplicationPluginInput represents one set of commands executed by the Applications plugin.
type ApplicationPluginInput struct {
	contracts.PluginInput
	ID             string
	Action         string
	Parameters     string
	Source         string
	SourceHash     string
	SourceHashType string
}

// ApplicationPluginOutput represents the output of the plugin
type ApplicationPluginOutput struct {
	contracts.PluginOutput
}

// Failed marks plugin as Failed
func (out *ApplicationPluginOutput) MarkAsFailed(log log.T, err error) {
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
	return appconfig.PluginNameAwsApplications
}

// Execute runs multiple sets of commands and returns their outputs.
// res.Output will contain a slice of PluginOutput.
func (p *Plugin) Execute(context context.T, config contracts.Configuration, cancelFlag task.CancelFlag) (res contracts.PluginResult) {
	log := context.Log()
	log.Infof("%v started with configuration %v", Name(), config)
	res.StartDateTime = time.Now()
	defer func() { res.EndDateTime = time.Now() }()

	//loading Properties as list since aws:applications uses properties as list
	var properties []interface{}
	if properties, res = pluginutil.LoadParametersAsList(log, config.Properties); res.Code != 0 {

		pluginutil.PersistPluginInformationToCurrent(log, config.PluginID, config, res)
		return res
	}

	msiFailureCount := 0
	atleastOneRequestedReboot := false
	finalStdOut := ""
	finalStdErr := ""
	out := make([]ApplicationPluginOutput, len(properties))
	for i, prop := range properties {
		// check if a reboot has been requested
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

		out[i] = p.runCommandsRawInput(log, prop, config.OrchestrationDirectory, cancelFlag, config.OutputS3BucketName, config.OutputS3KeyPrefix)

		if out[i].Status == contracts.ResultStatusFailed {
			msiFailureCount++

			if out[i].Stdout != "" {
				finalStdOut = fmt.Sprintf("%v\n%v", finalStdOut, out[i].Stdout)
			}

			if out[i].Stderr != "" {
				finalStdErr = fmt.Sprintf("%v\n%v", finalStdErr, out[i].Stderr)
			}
		}

		if out[i].Status == contracts.ResultStatusSuccessAndReboot {
			atleastOneRequestedReboot = true
			res.Code = out[i].ExitCode
		}
	}

	if atleastOneRequestedReboot {
		res.Status = contracts.ResultStatusSuccessAndReboot
	} else {
		res.Status = contracts.ResultStatusSuccess
		res.Code = 0
	}

	if msiFailureCount > 0 {
		finalStdOut = fmt.Sprintf("Number of Failures: %v\n%v", msiFailureCount, finalStdOut)
		res.Status = contracts.ResultStatusFailed
		res.Code = 1
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
func (p *Plugin) runCommandsRawInput(log log.T, rawPluginInput interface{}, orchestrationDirectory string, cancelFlag task.CancelFlag, outputS3BucketName string, outputS3KeyPrefix string) (out ApplicationPluginOutput) {
	var pluginInput ApplicationPluginInput
	err := jsonutil.Remarshal(rawPluginInput, &pluginInput)
	log.Debugf("Plugin input %v", pluginInput)
	if err != nil {
		errorString := fmt.Errorf("Invalid format in plugin properties %v;\nerror %v", rawPluginInput, err)
		out.MarkAsFailed(log, errorString)
		return
	}
	return p.runCommands(log, pluginInput, orchestrationDirectory, cancelFlag, outputS3BucketName, outputS3KeyPrefix)
}

// runCommands executes one set of commands and returns their output.
func (p *Plugin) runCommands(log log.T, pluginInput ApplicationPluginInput, orchestrationDirectory string, cancelFlag task.CancelFlag, outputS3BucketName string, outputS3KeyPrefix string) (out ApplicationPluginOutput) {
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
	log.Debugf("OrchestrationDir %v ", orchestrationDir)

	// create orchestration dir if needed
	if err = fileutil.MakeDirs(orchestrationDir); err != nil {
		log.Debug("failed to create orchestrationDir directory", orchestrationDir, err)
		out.Errors = append(out.Errors, err.Error())
		return
	}

	// Get application mode
	mode, err := getMsiApplicationMode(log, pluginInput)
	if err != nil {
		out.MarkAsFailed(log, err)
		return
	}
	log.Debugf("mode is %v", mode)

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

	// Download file from source if available
	downloadOutput, err := pluginutil.DownloadFileFromSource(log, pluginInput.Source, pluginInput.SourceHash, pluginInput.SourceHashType)
	if err != nil || downloadOutput.IsHashMatched == false || downloadOutput.LocalFilePath == "" {
		errorString := fmt.Errorf("failed to download file reliably %v", pluginInput.Source)
		out.MarkAsFailed(log, errorString)
		return
	}
	log.Debugf("local path to file is %v", downloadOutput.LocalFilePath)

	// Create msi related log file
	localSourceLogFilePath := downloadOutput.LocalFilePath + ".msiexec.log.txt"
	log.Debugf("log path is %v", localSourceLogFilePath)

	// TODO: This needs to be pulled out of this function as it runs multiple times getting initialized with the same values
	// Create output file paths
	stdoutFilePath := filepath.Join(orchestrationDir, p.StdoutFileName)
	stderrFilePath := filepath.Join(orchestrationDir, p.StderrFileName)
	log.Debugf("stdout file %v, stderr file %v", stdoutFilePath, stderrFilePath)

	// Resolve ssm parameters
	// This may contain sensitive information, do not log this data after resolving.
	if pluginInput.Parameters, err = parameterstore.ResolveString(log, pluginInput.Parameters); err != nil {
		out.Errors = append(out.Errors, err.Error())
		log.Errorf("Failed to resolve ssm parameters. Error: - %v", err)
		return
	}

	// Construct Command Name and Arguments
	commandName := msiExecCommand
	commandArguments := []string{mode, downloadOutput.LocalFilePath, "/quiet", "/norestart", "/log", localSourceLogFilePath}
	if pluginInput.Parameters != "" {
		log.Debugf("Got Parameters \"%v\"", pluginInput.Parameters)
		params := processParams(log, pluginInput.Parameters)
		commandArguments = append(commandArguments, params...)
	}

	// Execute Command
	_, _, exitCode, errs := p.ExecuteCommand(log, defaultWorkingDirectory, stdoutFilePath, stderrFilePath, cancelFlag, defaultApplicationExecutionTimeoutInSeconds, commandName, commandArguments)

	// Set output status
	out.ExitCode = exitCode
	setMsiExecStatus(log, pluginInput, cancelFlag, &out)

	if len(errs) > 0 {
		for _, err := range errs {
			out.Errors = append(out.Errors, err.Error())
			log.Error("failed to run commands: ", err)
			out.Status = contracts.ResultStatusFailed
		}
		return
	}

	// Upload output to S3
	uploadOutputToS3BucketErrors := p.ExecuteUploadOutputToS3Bucket(log, pluginInput.ID, orchestrationDir, outputS3BucketName, outputS3KeyPrefix, useTempDirectory, tempDir, out.Stdout, out.Stderr)
	out.Errors = append(out.Errors, uploadOutputToS3BucketErrors...)

	// Return Json indented response
	responseContent, _ := jsonutil.Marshal(out)
	log.Debug("Returning response:\n", jsonutil.Indent(responseContent))
	return
}
