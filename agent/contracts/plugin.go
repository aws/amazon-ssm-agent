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
)

const (
	//MaximumPluginOutputSize represents the maximum output size that agent supports
	MaximumPluginOutputSize = 2400

	truncOut   = "\n---Output truncated---"
	truncError = "\n---Error truncated----"
)

var (
	lenTruncOut   = len(truncOut)
	lenTruncError = len(truncError)
)

// PluginResult represents a plugin execution result.
type PluginResult struct {
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

// ICorePlugin is the very much of core itself will be implemented as plugins
// that are simply hardcoded to run with agent framework.
// The hardcoded plugins will implement the ICorePlugin
type ICorePlugin interface {
	Name() string
	Execute(context context.T) (err error)
	RequestStop(stopType StopType) (err error)
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
	if len(out.Stderr) != 0 {
		out.Stderr = fmt.Sprintf("\n%v\n%v", out.Stderr, err.Error())
	} else {
		out.Stderr = fmt.Sprintf("\n%v", err.Error())
	}
	log.Error(err.Error())
}

// AppendInfo adds info to PluginOutput StandardOut.
func (result *PluginOutput) AppendInfo(log log.T, format string, params ...interface{}) {
	message := fmt.Sprintf(format, params...)
	log.Info(message)
	result.Stdout = fmt.Sprintf("%v\n%v", result.Stdout, message)
}

// AppendError adds errors to PluginOutput StandardErr.
func (result *PluginOutput) AppendError(log log.T, format string, params ...interface{}) {
	message := fmt.Sprintf(format, params...)
	log.Error(message)
	result.Stderr = fmt.Sprintf("%v\n%v", result.Stderr, message)
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
