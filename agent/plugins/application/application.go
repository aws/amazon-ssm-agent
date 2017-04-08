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
	"github.com/aws/amazon-ssm-agent/agent/plugins/pluginutil"
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
	DefaultWorkingDirectory string
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

// NewPlugin returns a new instance of the plugin.
func NewPlugin(pluginConfig pluginutil.PluginConfig) (*Plugin, error) {
	var plugin Plugin
	plugin.MaxStdoutLength = pluginConfig.MaxStdoutLength
	plugin.MaxStderrLength = pluginConfig.MaxStderrLength
	plugin.StdoutFileName = pluginConfig.StdoutFileName
	plugin.StderrFileName = pluginConfig.StderrFileName
	plugin.OutputTruncatedSuffix = pluginConfig.OutputTruncatedSuffix
	plugin.ExecuteUploadOutputToS3Bucket = pluginutil.UploadOutputToS3BucketExecuter(plugin.UploadOutputToS3Bucket)
	plugin.CommandExecuter = executers.ShellCommandExecuter{}

	return &plugin, nil
}

// Name returns the name of the plugin
func Name() string {
	return appconfig.PluginNameAwsApplications
}

// Execute runs multiple sets of commands and returns their outputs.
// res.Output will contain a slice of PluginOutput.
func (p *Plugin) Execute(context context.T, config contracts.Configuration, cancelFlag task.CancelFlag, subDocumentRunner runpluginutil.PluginRunner) (res contracts.PluginResult) {
	log := context.Log()
	log.Infof("%v started with configuration %v", Name(), config)
	res.StartDateTime = time.Now()
	p.DefaultWorkingDirectory = config.DefaultWorkingDirectory
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
	out := make([]contracts.PluginOutput, len(properties))
	for i, prop := range properties {

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

		out[i] = p.runCommandsRawInput(log, config.PluginID, prop, config.OrchestrationDirectory, cancelFlag, config.OutputS3BucketName, config.OutputS3KeyPrefix)

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
func (p *Plugin) runCommandsRawInput(log log.T, pluginID string, rawPluginInput interface{}, orchestrationDirectory string, cancelFlag task.CancelFlag, outputS3BucketName string, outputS3KeyPrefix string) (out contracts.PluginOutput) {
	var pluginInput ApplicationPluginInput
	err := jsonutil.Remarshal(rawPluginInput, &pluginInput)
	log.Debugf("Plugin input %v", pluginInput)
	if err != nil {
		errorString := fmt.Errorf("Invalid format in plugin properties %v;\nerror %v", rawPluginInput, err)
		out.MarkAsFailed(log, errorString)
		return
	}
	return p.runCommands(log, pluginID, pluginInput, orchestrationDirectory, cancelFlag, outputS3BucketName, outputS3KeyPrefix)
}

// runCommands executes one set of commands and returns their output.
func (p *Plugin) runCommands(log log.T, pluginID string, pluginInput ApplicationPluginInput, orchestrationDirectory string, cancelFlag task.CancelFlag, outputS3BucketName string, outputS3KeyPrefix string) (out contracts.PluginOutput) {
	var err error

	// TODO:MF: This subdirectory is only needed because we could be running multiple sets of properties for the same plugin - otherwise the orchestration directory would already be unique
	orchestrationDir := fileutil.BuildPath(orchestrationDirectory, pluginInput.ID)
	log.Debugf("OrchestrationDir %v ", orchestrationDir)

	// create orchestration dir if needed
	if err = fileutil.MakeDirs(orchestrationDir); err != nil {
		log.Debug("failed to create orchestrationDir directory", orchestrationDir, err)
		out.MarkAsFailed(log, err)
		return
	}

	// Get application mode
	mode, err := getMsiApplicationMode(log, pluginInput)
	if err != nil {
		out.MarkAsFailed(log, err)
		return
	}
	log.Debugf("mode is %v", mode)

	var localFilePath string
	// Download file from source if available
	downloadOutput, err := pluginutil.DownloadFileFromSource(log, pluginInput.Source, pluginInput.SourceHash, pluginInput.SourceHashType)
	if err != nil || downloadOutput.IsHashMatched == false || downloadOutput.LocalFilePath == "" {
		errorString := fmt.Errorf("failed to download file reliably %v", pluginInput.Source)
		out.MarkAsFailed(log, errorString)
		return
	}
	localFilePath = downloadOutput.LocalFilePath
	log.Debugf("local path to file is %v", localFilePath)

	// Create msi related log file
	localSourceLogFilePath := localFilePath + ".msiexec.log.txt"
	log.Debugf("log path is %v", localSourceLogFilePath)

	// TODO: This needs to be pulled out of this function as it runs multiple times getting initialized with the same values
	// Create output file paths
	stdoutFilePath := filepath.Join(orchestrationDir, p.StdoutFileName)
	stderrFilePath := filepath.Join(orchestrationDir, p.StderrFileName)
	log.Debugf("stdout file %v, stderr file %v", stdoutFilePath, stderrFilePath)

	// Construct Command Name and Arguments
	commandName := msiExecCommand
	commandArguments := []string{mode, localFilePath, "/quiet", "/norestart", "/log", localSourceLogFilePath}
	if pluginInput.Parameters != "" {
		log.Debugf("Got Parameters \"%v\"", pluginInput.Parameters)
		params := processParams(log, pluginInput.Parameters)
		commandArguments = append(commandArguments, params...)
	}

	// Execute Command
	_, _, exitCode, errs := p.CommandExecuter.Execute(log, defaultWorkingDirectory, stdoutFilePath, stderrFilePath, cancelFlag, defaultApplicationExecutionTimeoutInSeconds, commandName, commandArguments)

	// Set output status
	out.ExitCode = exitCode
	setMsiExecStatus(log, pluginInput, cancelFlag, &out)

	if len(errs) > 0 {
		for _, err := range errs {
			out.MarkAsFailed(log, fmt.Errorf("failed to run commands: ", err))
		}
		return
	}

	// Upload output to S3
	s3PluginID := pluginInput.ID
	if s3PluginID == "" {
		s3PluginID = pluginID
	}
	uploadOutputToS3BucketErrors := p.ExecuteUploadOutputToS3Bucket(log, s3PluginID, orchestrationDir, outputS3BucketName, outputS3KeyPrefix, false, "", out.Stdout, out.Stderr)
	if len(uploadOutputToS3BucketErrors) > 0 {
		log.Errorf("Unable to upload the logs: %s", uploadOutputToS3BucketErrors)
	}

	// Return Json indented response
	responseContent, _ := jsonutil.Marshal(out)
	log.Debug("Returning response:\n", jsonutil.Indent(responseContent))
	return
}
