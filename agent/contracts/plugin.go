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

// Package contracts contains objects for parsing and encoding MDS/SSM messages.
package contracts

import (
	"fmt"
	"time"

	"github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/amazon-ssm-agent/agent/updateutil"
)

const (
	//MaximumPluginOutputSize represents the maximum output size that agent supports
	MaximumPluginOutputSize = 2400
	truncOut                = "\n---Output truncated---"
	truncError              = "\n---Error truncated----"
)

const (
	preconditionSchemaVersion string = "2.2"
)

var (
	lenTruncOut   = len(truncOut)
	lenTruncError = len(truncError)
)

// PluginResult represents a plugin execution result.
type PluginResult struct {
	PluginID           string       `json:"pluginID"`
	PluginName         string       `json:"pluginName"`
	Status             ResultStatus `json:"status"`
	Code               int          `json:"code"`
	Output             interface{}  `json:"output"`
	StartDateTime      time.Time    `json:"startDateTime"`
	EndDateTime        time.Time    `json:"endDateTime"`
	OutputS3BucketName string       `json:"outputS3BucketName"`
	OutputS3KeyPrefix  string       `json:"outputS3KeyPrefix"`
	Error              error        `json:"-"`
	StandardOutput     string       `json:"standardOutput"`
	StandardError      string       `json:"standardError"`
}

// IPlugin is interface for authoring a functionality of work.
// Every functionality of work is implemented as a plugin.
type IPlugin interface {
	Name() string
	Execute(context context.T, input PluginConfig) (output PluginResult, err error)
	RequestStop(stopType StopType) (err error)
}

// ICoreModule is the very much of core itself will be implemented as plugins
// that are simply hardcoded to run with agent framework.
// The hardcoded plugins will implement the ICoreModule
type ICoreModule interface {
	ModuleName() string
	ModuleExecute(context context.T) (err error)
	ModuleRequestStop(stopType StopType) (err error)
}

// IWorkerPlugin is the plugins which do not form part of core
// These plugins are invoked on demand.
type IWorkerPlugin IPlugin

// Configuration represents a plugin configuration as in the json format.
type Configuration struct {
	Settings                interface{}
	Properties              interface{}
	OutputS3KeyPrefix       string
	OutputS3BucketName      string
	OrchestrationDirectory  string
	MessageId               string
	BookKeepingFileName     string
	PluginName              string
	PluginID                string
	DefaultWorkingDirectory string
	Preconditions           map[string][]string
	IsPreconditionEnabled   bool
}

// Plugin wraps the plugin configuration and plugin result.
type Plugin struct {
	Configuration
	PluginResult
}

// PluginInput represents the input of the plugin.
type PluginInput struct {
}

// PluginOutput represents the output of the plugin.
type PluginOutput struct {
	ExitCode int
	Status   ResultStatus
	Stdout   string
	Stderr   string
}

func (p *PluginOutput) Merge(log log.T, mergeOutput PluginOutput) {
	p.AppendInfo(log, mergeOutput.Stdout)
	p.AppendError(log, mergeOutput.Stderr)
	if p.ExitCode == 0 {
		p.ExitCode = mergeOutput.ExitCode
	}
	p.Status = MergeResultStatus(p.Status, mergeOutput.Status)
}

func (p *PluginOutput) String() (response string) {
	return TruncateOutput(p.Stdout, p.Stderr, MaximumPluginOutputSize)
}

// MarkAsFailed Failed marks plugin as Failed
func (out *PluginOutput) MarkAsFailed(log log.T, err error) {
	// Update the error exit code
	if out.ExitCode == 0 {
		out.ExitCode = 1
	}
	out.Status = ResultStatusFailed
	if err != nil {
		out.AppendError(log, err.Error())
	}
}

// MarkAsSucceeded marks plugin as Successful.
func (out *PluginOutput) MarkAsSucceeded() {
	out.ExitCode = 0
	out.Status = ResultStatusSuccess
}

// MarkAsInProgress marks plugin as In Progress.
func (out *PluginOutput) MarkAsInProgress() {
	out.ExitCode = 0
	out.Status = ResultStatusInProgress
}

// MarkAsSuccessWithReboot marks plugin as Successful and requests a reboot.
func (out *PluginOutput) MarkAsSuccessWithReboot() {
	out.ExitCode = 0
	out.Status = ResultStatusSuccessAndReboot
}

// MarkAsCancelled marks a plugin as Cancelled.
func (out *PluginOutput) MarkAsCancelled() {
	out.ExitCode = 1
	out.Status = ResultStatusCancelled
}

// MarkAsShutdown marks a plugin as Failed in the case of interruption due to shutdown signal.
func (out *PluginOutput) MarkAsShutdown() {
	out.ExitCode = 1
	out.Status = ResultStatusCancelled
}

// AppendInfo adds info to PluginOutput StandardOut.
func (out *PluginOutput) AppendInfo(log log.T, message string) {
	if len(message) > 0 {
		log.Info(message)
		if len(out.Stdout) > 0 {
			out.Stdout = fmt.Sprintf("%v\n%v", out.Stdout, message)
		} else {
			out.Stdout = message
		}
	}
}

// AppendInfof adds info to PluginOutput StandardOut with formatting parameters.
func (out *PluginOutput) AppendInfof(log log.T, format string, params ...interface{}) {
	if len(format) > 0 {
		message := fmt.Sprintf(format, params...)
		out.AppendInfo(log, message)
	}
}

// AppendError adds errors to PluginOutput StandardErr.
func (out *PluginOutput) AppendError(log log.T, message string) {
	if len(message) > 0 {
		log.Error(message)
		if len(out.Stderr) > 0 {
			out.Stderr = fmt.Sprintf("%v\n%v", out.Stderr, message)
		} else {
			out.Stderr = message
		}
	}
}

// AppendErrorf adds errors to PluginOutput StandardErr with formatting parameters.
func (out *PluginOutput) AppendErrorf(log log.T, format string, params ...interface{}) {
	if len(format) > 0 {
		message := fmt.Sprintf(format, params...)
		out.AppendError(log, message)
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

	// trunc out and error when both exceed the size
	if outputSize > availableSpace/2 && errorSize > availableSpace/2 {
		truncSize := availableSpace - lenTruncError - lenTruncOut
		return fmt.Sprint(stdout[:truncSize/2], truncOut, errorTitle, stderr[:truncSize/2], truncError)
	}

	// trunc error when output is short
	if outputSize < availableSpace/2 {
		truncSize := availableSpace - lenTruncError
		return fmt.Sprint(stdout, errorTitle, stderr[:truncSize-outputSize], truncError)
	}

	// trunc output when error is short
	truncSize := availableSpace - lenTruncOut
	return fmt.Sprint(stdout[:truncSize-errorSize], truncOut, errorTitle, stderr)
}

// Check if precondition support is enabled by checking document schema version
func IsPreconditionEnabled(schemaVersion string) (response bool) {
	response = false

	// set precondition flag based on schema version
	versionCompare, err := updateutil.VersionCompare(schemaVersion, preconditionSchemaVersion)
	if err == nil && versionCompare >= 0 {
		response = true
	}

	return response
}
