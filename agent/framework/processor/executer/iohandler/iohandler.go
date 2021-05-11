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

// Package iohandler implements the iohandler for the plugins
package iohandler

import (
	"bytes"
	"fmt"
	"io"
	"runtime/debug"

	"github.com/aws/amazon-ssm-agent/agent/agentlogstocloudwatch/cloudwatchlogspublisher"
	"github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/aws/amazon-ssm-agent/agent/contracts"
	"github.com/aws/amazon-ssm-agent/agent/fileutil"
	"github.com/aws/amazon-ssm-agent/agent/framework/processor/executer/iohandler/iomodule"
	"github.com/aws/amazon-ssm-agent/agent/framework/processor/executer/iohandler/multiwriter"
)

const (
	// maximumPluginOutputSize represents the maximum output size that agent supports
	MaximumPluginOutputSize = 2500
	// truncateOut represents the string appended when output is truncated
	truncateOut = "\n---Output truncated---"
	// truncateError represents the string appended when error is truncated
	truncateError = "\n---Error truncated----"
)

// PluginConfig is used for initializing plugins with default values
type PluginConfig struct {
	StdoutFileName        string
	StderrFileName        string
	StdoutConsoleFileName string
	StderrConsoleFileName string
	MaxStdoutLength       int
	MaxStderrLength       int
	OutputTruncatedSuffix string
}

// DefaultOutputConfig returns the default values for the plugin
func DefaultOutputConfig() PluginConfig {
	return PluginConfig{
		StdoutFileName:        "stdout",
		StderrFileName:        "stderr",
		StdoutConsoleFileName: "stdoutConsole",
		StderrConsoleFileName: "stderrConsole",
		MaxStdoutLength:       24000,
		MaxStderrLength:       8000,
		OutputTruncatedSuffix: "--output truncated--",
	}
}

// IOHandler Interface defines interface for IOHandler type
type IOHandler interface {
	Init(...string)
	RegisterOutputSource(multiwriter.DocumentIOMultiWriter, ...iomodule.IOModule)
	Close()
	String() string
	MarkAsFailed(err error)
	MarkAsSucceeded()
	MarkAsInProgress()
	MarkAsSuccessWithReboot()
	MarkAsCancelled()
	MarkAsShutdown()

	AppendInfo(message string)
	AppendInfof(format string, params ...interface{})
	AppendError(message string)
	AppendErrorf(format string, params ...interface{})

	// getters/setters
	GetStatus() contracts.ResultStatus
	GetStdout() string
	GetStderr() string
	GetExitCode() int
	GetStdoutWriter() multiwriter.DocumentIOMultiWriter
	GetStderrWriter() multiwriter.DocumentIOMultiWriter
	GetIOConfig() contracts.IOConfiguration

	SetStatus(contracts.ResultStatus)
	SetExitCode(int)
	SetOutput(interface{})
	SetStdout(string)
	SetStderr(string)
}

// DefaultIOHandler is used for writing output by the plugins
type DefaultIOHandler struct {
	context  context.T
	ExitCode int
	Status   contracts.ResultStatus
	//private members - not exposed directly to plugins because they shouldn't write to these
	stdout   string
	stderr   string
	ioConfig contracts.IOConfiguration
	//refreshassociation and invoker write a different output rather than merging stdout and stderr
	output interface{}

	// List of Writers attached to the IOHandler instance
	StdoutWriter multiwriter.DocumentIOMultiWriter
	StderrWriter multiwriter.DocumentIOMultiWriter
}

// NewDefaultIOHandler returns a new instance of the IOHandler
func NewDefaultIOHandler(context context.T, ioConfig contracts.IOConfiguration) *DefaultIOHandler {

	context.Log().Debugf("IOHandler Initialization with config: %v", ioConfig)
	out := new(DefaultIOHandler)
	out.context = context
	out.ioConfig = ioConfig

	return out
}

// Init initializes the plugin output object by creating the necessary writers
func (out *DefaultIOHandler) Init(filePath ...string) {
	log := out.context.Log()
	pluginConfig := DefaultOutputConfig()
	// Create path to output location for file and s3
	fullPath := out.ioConfig.OrchestrationDirectory
	s3KeyPrefix := out.ioConfig.OutputS3KeyPrefix
	for _, element := range filePath {
		fullPath = fileutil.BuildPath(fullPath, element)
		s3KeyPrefix = fileutil.BuildS3Path(s3KeyPrefix, element)
	}

	stdOutLogStreamName := ""
	stdErrLogStreamName := ""
	if out.ioConfig.CloudWatchConfig.LogGroupName != "" {
		cwl := cloudwatchlogspublisher.NewCloudWatchLogsService(out.context)
		if err := cwl.CreateLogGroup(out.ioConfig.CloudWatchConfig.LogGroupName); err != nil {
			log.Errorf("Error Creating Log Group for CloudWatchLogs output: %v", err)
			//Stop CloudWatch Streaming on Error
			out.ioConfig.CloudWatchConfig.LogGroupName = ""
		}
		stdOutLogStreamName = fmt.Sprintf("%s/%s", out.ioConfig.CloudWatchConfig.LogStreamPrefix, pluginConfig.StdoutFileName)
		stdErrLogStreamName = fmt.Sprintf("%s/%s", out.ioConfig.CloudWatchConfig.LogStreamPrefix, pluginConfig.StderrFileName)
	}

	// Initialize file output module
	stdoutFile := iomodule.File{
		FileName:               pluginConfig.StdoutFileName,
		OrchestrationDirectory: fullPath,
		OutputS3BucketName:     out.ioConfig.OutputS3BucketName,
		OutputS3KeyPrefix:      s3KeyPrefix,
		LogGroupName:           out.ioConfig.CloudWatchConfig.LogGroupName,
		LogStreamName:          stdOutLogStreamName,
	}

	// Initialize console output module
	stdoutConsole := iomodule.CommandOutput{
		OutputString:           &out.stdout,
		FileName:               pluginConfig.StdoutConsoleFileName,
		OrchestrationDirectory: fullPath,
	}

	log.Debug("Initializing the Stdout Multi-writer with file and console listeners")
	// Get a multi-writer for standard output
	out.StdoutWriter = multiwriter.NewDocumentIOMultiWriter()
	out.RegisterOutputSource(out.StdoutWriter, stdoutFile, stdoutConsole)

	// Initialize file error module
	stderrFile := iomodule.File{
		FileName:               pluginConfig.StderrFileName,
		OrchestrationDirectory: fullPath,
		OutputS3BucketName:     out.ioConfig.OutputS3BucketName,
		OutputS3KeyPrefix:      s3KeyPrefix,
		LogGroupName:           out.ioConfig.CloudWatchConfig.LogGroupName,
		LogStreamName:          stdErrLogStreamName,
	}

	// Initialize console error module
	stderrConsole := iomodule.CommandOutput{
		OutputString:           &out.stderr,
		FileName:               pluginConfig.StderrConsoleFileName,
		OrchestrationDirectory: fullPath,
	}

	log.Debug("Initializing the Stderr Multi-writer with file and console listeners")
	// Get a multi-writer for standard error
	out.StderrWriter = multiwriter.NewDocumentIOMultiWriter()
	out.RegisterOutputSource(out.StderrWriter, stderrFile, stderrConsole)
}

// RegisterOutputSource returns a new output source by creating a multiwriter for the output modules.
func (out *DefaultIOHandler) RegisterOutputSource(multiWriter multiwriter.DocumentIOMultiWriter, IOModules ...iomodule.IOModule) {
	if len(IOModules) == 0 {
		return
	}

	log := out.context.Log()
	wg := multiWriter.GetWaitGroup()
	// Create a Pipe for each IO Module and add it to the multi-writer.
	for _, module := range IOModules {
		r, w := io.Pipe()
		multiWriter.AddWriter(w)
		// Run the reader for each module
		log.Debug("Starting a new stream reader go routing")
		go func(module iomodule.IOModule, r *io.PipeReader) {
			defer func() {
				if r := recover(); r != nil {
					log.Errorf("Stream reader panic: %v", r)
					log.Errorf("Stacktrace:\n%s", debug.Stack())
				}
			}()
			defer wg.Done()
			module.Read(out.context, r, out.ExitCode)
		}(module, r)
	}

	return
}

// Close closes all the attached writers.
func (out *DefaultIOHandler) Close() {
	log := out.context.Log()
	log.Debug("IOHandler closing all subscribed writers.")
	if out.StdoutWriter != nil {
		out.StdoutWriter.Close()
	}

	if out.StderrWriter != nil {
		out.StderrWriter.Close()
	}
}

// String returns the output by concatenating stdout and stderr
func (out DefaultIOHandler) String() (response string) {
	// exit code 168 is a successful execution signaling all future steps should be ignored
	// therefore, we remove the error message of exit status 168 which would be confusing in the output
	if out.ExitCode == contracts.ExitWithSuccess {
		out.stderr = ""
	}
	return TruncateOutput(out.stdout, out.stderr, MaximumPluginOutputSize)
}

// GetOutput returns the output to be appended to the response
func (out DefaultIOHandler) GetOutput() interface{} {
	// Return output if assigned. Otherwise, return stdout + stderr
	if out.output == nil {
		return out.String()
	}
	return out.output
}

// GetStatus returns the status
func (out DefaultIOHandler) GetStatus() contracts.ResultStatus {
	return out.Status
}

// GetStdout returns the stdout
func (out DefaultIOHandler) GetStdout() string {
	return out.stdout
}

// GetExitCode returns the exit code
func (out DefaultIOHandler) GetExitCode() int {
	return out.ExitCode
}

// GetStderr returns the stderr
func (out DefaultIOHandler) GetStderr() string {
	return out.stderr
}

// GetIOConfig returns the io configuration
func (out DefaultIOHandler) GetIOConfig() contracts.IOConfiguration {
	return out.ioConfig
}

// GetStdoutWriter returns the stdout writer
func (out DefaultIOHandler) GetStdoutWriter() multiwriter.DocumentIOMultiWriter {
	return out.StdoutWriter
}

// GetStderrWriter returns the stderr writer
func (out DefaultIOHandler) GetStderrWriter() multiwriter.DocumentIOMultiWriter {
	return out.StderrWriter
}

// SetStatus sets the status
func (out *DefaultIOHandler) SetStatus(status contracts.ResultStatus) {
	out.Status = status
}

// SetStdout sets the stdout
func (out *DefaultIOHandler) SetStdout(stdout string) {
	out.stdout = stdout
}

// SetStderr sets the stderr
func (out *DefaultIOHandler) SetStderr(stderr string) {
	out.stderr = stderr
}

// SetExitCode sets the exit code
func (out *DefaultIOHandler) SetExitCode(exitCode int) {
	out.ExitCode = exitCode
}

// SetOutput sets the output
func (out *DefaultIOHandler) SetOutput(output interface{}) {
	out.output = output
}

// Merge plugin output objects
func (out *DefaultIOHandler) Merge(mergeOutput *DefaultIOHandler) {

	// Append Info
	var stdoutBuffer bytes.Buffer
	if len(out.stdout) > 0 {
		stdoutBuffer.WriteString(out.stdout + "\n")
	}
	stdoutBuffer.WriteString(mergeOutput.GetStdout())
	out.stdout = stdoutBuffer.String()

	// Append Error
	var stderrBuffer bytes.Buffer
	if len(out.stderr) > 0 {
		stderrBuffer.WriteString(out.stderr + "\n")
	}
	stderrBuffer.WriteString(mergeOutput.GetStderr())
	out.stderr = stderrBuffer.String()

	if out.ExitCode == 0 {
		out.ExitCode = mergeOutput.GetExitCode()
	}
	out.Status = contracts.MergeResultStatus(out.Status, mergeOutput.GetStatus())
}

// MarkAsFailed Failed marks plugin as Failed
func (out *DefaultIOHandler) MarkAsFailed(err error) {
	// Update the error exit code
	if out.ExitCode == 0 {
		out.ExitCode = 1
	}
	out.Status = contracts.ResultStatusFailed
	if err != nil {
		out.AppendError(err.Error())
	}
}

// MarkAsSucceeded marks plugin as Successful.
func (out *DefaultIOHandler) MarkAsSucceeded() {
	out.ExitCode = 0
	out.Status = contracts.ResultStatusSuccess
}

// MarkAsInProgress marks plugin as In Progress.
func (out *DefaultIOHandler) MarkAsInProgress() {
	out.ExitCode = 0
	out.Status = contracts.ResultStatusInProgress
}

// MarkAsSuccessWithReboot marks plugin as Successful and requests a reboot.
func (out *DefaultIOHandler) MarkAsSuccessWithReboot() {
	out.ExitCode = 0
	out.Status = contracts.ResultStatusSuccessAndReboot
}

// MarkAsCancelled marks a plugin as Cancelled.
func (out *DefaultIOHandler) MarkAsCancelled() {
	out.ExitCode = 1
	out.Status = contracts.ResultStatusCancelled
}

// MarkAsShutdown marks a plugin as Failed in the case of interruption due to shutdown signal.
func (out *DefaultIOHandler) MarkAsShutdown() {
	out.ExitCode = 1
	out.Status = contracts.ResultStatusCancelled
}

// AppendInfo adds info to IOHandler StandardOut.
func (out *DefaultIOHandler) AppendInfo(message string) {
	if len(message) > 0 && out.StdoutWriter != nil {
		if len(out.stdout) > 0 {
			out.StdoutWriter.WriteString("\n")
		}
		out.StdoutWriter.WriteString(message)
	} else {
		// Write to stdout if the writer is not defined.
		if len(out.stdout) > 0 {
			out.stdout = fmt.Sprintf("%v\n%v", out.stdout, message)
		} else {
			out.stdout = message
		}
	}
}

// AppendInfof adds info to DefaultIOHandler StandardOut with formatting parameters.
func (out *DefaultIOHandler) AppendInfof(format string, params ...interface{}) {
	if len(format) > 0 {
		message := fmt.Sprintf(format, params...)
		out.AppendInfo(message)
	}
}

// AppendError adds errors to DefaultIOHandler StandardErr.
func (out *DefaultIOHandler) AppendError(message string) {
	if len(message) > 0 && out.StderrWriter != nil {
		if len(out.stderr) > 0 {
			out.StderrWriter.WriteString("\n")
		}
		out.StderrWriter.WriteString(message)
	} else {
		// Write to stderr if the writer is not defined.
		if len(out.stderr) > 0 {
			out.stderr = fmt.Sprintf("%v\n%v", out.stderr, message)
		} else {
			out.stderr = message
		}
	}
}

// AppendErrorf adds errors to DefaultIOHandler StandardErr with formatting parameters.
func (out *DefaultIOHandler) AppendErrorf(format string, params ...interface{}) {
	if len(format) > 0 {
		message := fmt.Sprintf(format, params...)
		out.AppendError(message)
	}
}

// TruncateOutput truncates the output
func TruncateOutput(stdout string, stderr string, capacity int) (response string) {
	outputSize := len(stdout)
	errorSize := len(stderr)

	// prepare error title
	errorTitle := ""
	lenErrorTitle := 0
	if errorSize > 0 {
		errorTitle = "\n----------ERROR-------\n"
		lenErrorTitle = len(errorTitle)
	}

	// calculate available space
	availableSpace := capacity - lenErrorTitle

	// all fits within availableSpace
	if (outputSize + errorSize) < availableSpace {
		return fmt.Sprint(stdout, errorTitle, stderr)
	}

	// truncate out and error when both exceed the size
	if outputSize > availableSpace/2 && errorSize > availableSpace/2 {
		truncateSize := availableSpace - len(truncateError) - len(truncateOut)
		return fmt.Sprint(stdout[:truncateSize/2], truncateOut, errorTitle, stderr[:truncateSize/2], truncateError)
	}

	// truncate error when output is short
	if outputSize < availableSpace/2 {
		truncateSize := availableSpace - len(truncateError)
		return fmt.Sprint(stdout, errorTitle, stderr[:truncateSize-outputSize], truncateError)
	}

	// truncate output when error is short
	truncateSize := availableSpace - len(truncateOut)
	return fmt.Sprint(stdout[:truncateSize-errorSize], truncateOut, errorTitle, stderr)
}
