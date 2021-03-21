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
	"time"

	"github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/amazon-ssm-agent/agent/versionutil"
)

const (
	preconditionSchemaVersion string = "2.2"
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
	StepName           string       `json:"stepName"`
	Error              string       `json:"error"`
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
	ModuleExecute() (err error)
	ModuleRequestStop(stopType StopType) (err error)
}

// IWorkerPlugin is the plugins which do not form part of core
// These plugins are invoked on demand.
type IWorkerPlugin IPlugin

// PreconditionArgument represents a single input value for the plugin precondition operators
// InitialArgumentValue contains the original value of the argument as specified by the user (e.g. "parameter: {{ paramName }}")
// ResolvedArgumentValue contains the value of the argument with resolved document parameters (e.g. "parameter: paramValue")
type PreconditionArgument struct {
	InitialArgumentValue  string
	ResolvedArgumentValue string
}

// Configuration represents a plugin configuration as in the json format.
type Configuration struct {
	Settings                    interface{}
	Properties                  interface{}
	OutputS3KeyPrefix           string
	OutputS3BucketName          string
	S3EncryptionEnabled         bool
	CloudWatchLogGroup          string
	CloudWatchEncryptionEnabled bool
	CloudWatchStreamingEnabled  bool
	OrchestrationDirectory      string
	MessageId                   string
	BookKeepingFileName         string
	PluginName                  string
	PluginID                    string
	DefaultWorkingDirectory     string
	Preconditions               map[string][]PreconditionArgument
	IsPreconditionEnabled       bool
	CurrentAssociations         []string
	SessionId                   string
	ClientId                    string
	KmsKeyId                    string
	RunAsEnabled                bool
	RunAsUser                   string
	ShellProfile                ShellProfileConfig
	SessionOwner                string
}

// Plugin wraps the plugin configuration and plugin result.
type Plugin struct {
	Configuration
	PluginResult
}

// PluginInput represents the input of the plugin.
type PluginInput struct {
}

// PluginOutputter defines interface for PluginOutput type
type PluginOutputter interface {
	String() string
	MarkAsFailed(log log.T, err error)
	MarkAsSucceeded()
	MarkAsInProgress()
	MarkAsSuccessWithReboot()
	MarkAsCancelled()
	MarkAsShutdown()

	AppendInfo(log log.T, message string)
	AppendInfof(log log.T, format string, params ...interface{})
	AppendError(log log.T, message string)
	AppendErrorf(log log.T, format string, params ...interface{})

	// getters/setters
	GetStatus() ResultStatus
	GetStdout() string
	GetStderr() string
	GetExitCode() int

	SetStatus(ResultStatus)
	SetExitCode(int)
}

// IsPreconditionEnabled checks if precondition support is enabled by checking document schema version
func IsPreconditionEnabled(schemaVersion string) (response bool) {
	response = false

	// set precondition flag based on schema version
	versionCompare, err := versionutil.VersionCompare(schemaVersion, preconditionSchemaVersion)
	if err == nil && versionCompare >= 0 {
		response = true
	}

	return response
}
