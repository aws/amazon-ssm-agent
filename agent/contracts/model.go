// Copyright 2016 Amazon.com, Inc. or its affiliates. All Rights Reserved.
//
// Licensed under the Amazon Software License (the "License"). You may not
// use this file except in compliance with the License. A copy of the
// License is located at
//
// http://aws.amazon.com/asl/
//
// or in the "license" file accompanying this file. This file is distributed
// on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND,
// express or implied. See the License for the specific language governing
// permissions and limitations under the License.

// Package contracts contains all necessary interface and models
// necessary for communication and sharing within the agent.
package contracts

// ResultStatus provides the granular status of a plugin.
// These are internal states maintained by agent during the execution of a command/config
type ResultStatus string

const (
	ResultStatusUnknown          ResultStatus = "Unknown"
	ResultStatusNotStarted       ResultStatus = "NotStarted"
	ResultStatusInProgress       ResultStatus = "InProgress"
	ResultStatusSuccess          ResultStatus = "Success"
	ResultStatusSuccessAndReboot ResultStatus = "SuccessAndReboot"
	ResultStatusFailed           ResultStatus = "Failed"
	ResultStatusCancelled        ResultStatus = "Cancelled"
	ResultStatusTimedOut         ResultStatus = "TimedOut"
)

type StopType string

const (
	StopTypeSoftStop StopType = "SoftStop"
	StopTypeHardStop StopType = "HardStop"
)

// A Parameter in the DocumentContent of an MDS message.
type Parameter struct {
	DefaultVal  string `json:"default"`
	Description string `json:"description"`
	ParamType   string `json:"type"`
}

// PluginConfig stores plugin configuration
type PluginConfig struct {
	Properties  []interface{} `json:"properties"`
	Description string        `json:"description"`
}

// DocumentContent object which represents ssm document content.
type DocumentContent struct {
	SchemaVersion string                   `json:"schemaVersion"`
	Description   string                   `json:"description"`
	RuntimeConfig map[string]*PluginConfig `json:"runtimeConfig"`
	Parameters    map[string]*Parameter    `json:"parameters"`
}

// AdditionalInfo section in agent response
type AdditionalInfo struct {
	Agent               AgentInfo      `json:"agent"`
	DateTime            string         `json:"dateTime"`
	RunID               string         `json:"runId"`
	RuntimeStatusCounts map[string]int `json:"runtimeStatusCounts"`
}

// AgentInfo represents the agent response
type AgentInfo struct {
	Lang      string `json:"lang"`
	Name      string `json:"name"`
	Os        string `json:"os"`
	OsVersion string `json:"osver"`
	Version   string `json:"ver"`
}

// PluginRuntimeStatus represents plugin runtime status section in agent response
type PluginRuntimeStatus struct {
	Status             ResultStatus `json:"status"`
	Code               int          `json:"code"`
	Output             string       `json:"output"`
	StartDateTime      string       `json:"startDateTime"`
	EndDateTime        string       `json:"endDateTime"`
	OutputS3BucketName string       `json:"outputS3BucketName"`
	OutputS3KeyPrefix  string       `json:"outputS3KeyPrefix"`
}

// AgentConfiguration is a struct that stores information about the agent and instance.
type AgentConfiguration struct {
	AgentInfo  AgentInfo
	InstanceID string
}
