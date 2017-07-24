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

// Package contracts contains all necessary interface and models
// necessary for communication and sharing within the agent.
package contracts

// ResultStatus provides the granular status of a plugin.
// These are internal states maintained by agent during the execution of a command/config
type ResultStatus string

const (
	ResultStatusNotStarted       ResultStatus = "NotStarted"
	ResultStatusInProgress       ResultStatus = "InProgress"
	ResultStatusSuccess          ResultStatus = "Success"
	ResultStatusSuccessAndReboot ResultStatus = "SuccessAndReboot"
	ResultStatusPassedAndReboot  ResultStatus = "PassedAndReboot"
	ResultStatusFailed           ResultStatus = "Failed"
	ResultStatusCancelled        ResultStatus = "Cancelled"
	ResultStatusTimedOut         ResultStatus = "TimedOut"
	ResultStatusSkipped          ResultStatus = "Skipped"
)

func (rs ResultStatus) IsSuccess() bool {
	switch rs {
	case ResultStatusSuccess, ResultStatusPassedAndReboot, ResultStatusSuccessAndReboot:
		return true
	default:
		return false
	}
}

func (rs ResultStatus) IsReboot() bool {
	switch rs {
	case ResultStatusPassedAndReboot, ResultStatusSuccessAndReboot:
		return true
	default:
		return false
	}
}

// MergeResultStatus takes two ResultStatuses (presumably from sub-tasks) and decides what the overall task status should be
func MergeResultStatus(current ResultStatus, new ResultStatus) (merged ResultStatus) {
	orderedResultStatus := [...]ResultStatus{
		ResultStatusSkipped,
		ResultStatusSuccess,
		ResultStatusSuccessAndReboot,
		ResultStatusPassedAndReboot,
		ResultStatusNotStarted,
		ResultStatusInProgress,
		ResultStatusFailed,
		ResultStatusCancelled,
		ResultStatusTimedOut,
	}
	if current == "" {
		return new
	}
	if new == "" {
		return current
	}

	// Return the "greater" ResultStatus - the one with a higher index in OrderedResultStatus
	// We assume both exist in the array and therefore the first one found is at the lower index (so return the other one)
	for _, ResultStatus := range orderedResultStatus {
		if ResultStatus == current {
			return new
		}
		if ResultStatus == new {
			return current
		}
	}
	return new // Default to new ResultStatus if neither is found in the array
}

const (
	/*
		NOTE: Following constants are meant to be used for setting plugin status only
	*/
	// AssociationStatusPending represents Pending status
	AssociationStatusPending = "Pending"
	// AssociationStatusAssociated represents Associated status
	AssociationStatusAssociated = "Associated"
	// AssociationStatusInProgress represents InProgress status
	AssociationStatusInProgress = "InProgress"
	// AssociationStatusSuccess represents Success status
	AssociationStatusSuccess = "Success"
	// AssociationStatusFailed represents Failed status
	AssociationStatusFailed = "Failed"
	// AssociationStatusTimedOut represents TimedOut status
	AssociationStatusTimedOut = "TimedOut"
)

const (
	/*
		NOTE: Following constants are meant to be used for setting error codes in plugin status only. If these are used
		for setting plugin status -> the status will not be appropriately aggregated.
	*/
	// AssociationErrorCodeInvalidAssociation represents InvalidAssociation Error
	AssociationErrorCodeInvalidAssociation = "InvalidAssoc"
	// AssociationErrorCodeInvalidExpression represents InvalidExpression Error
	AssociationErrorCodeInvalidExpression = "InvalidExpression"
	// AssociationErrorCodeExecutionError represents Execution Error
	AssociationErrorCodeExecutionError = "ExecutionError"
	// AssociationErrorCodeListAssociationError represents ListAssociation Error
	AssociationErrorCodeListAssociationError = "ListAssocError"
	// AssociationErrorCodeSubmitAssociationError represents SubmitAssociation Error
	AssociationErrorCodeSubmitAssociationError = "SubmitAssocError"
	// AssociationErrorCodeStuckAtInProgressError represents association stuck in InProgress Error
	AssociationErrorCodeStuckAtInProgressError = "StuckAtInProgress"
	// AssociationErrorCodeNoError represents no error
	AssociationErrorCodeNoError = ""
)

const (
	// DocumentPendingMessages represents the summary message for pending association
	AssociationPendingMessage string = "Association is pending"
	// DocumentInProgressMessage represents the summary message for inprogress association
	AssociationInProgressMessage string = "Executing association"
)

const (
	// ParamTypeString represents the Param Type is String
	ParamTypeString = "String"
	// ParamTypeStringList represents the Param Type is StringList
	ParamTypeStringList = "StringList"
)

type StopType string

const (
	StopTypeSoftStop StopType = "SoftStop"
	StopTypeHardStop StopType = "HardStop"
)

// A Parameter in the DocumentContent of an MDS message.
type Parameter struct {
	DefaultVal     interface{} `json:"default"`
	Description    string      `json:"description"`
	ParamType      string      `json:"type"`
	AllowedVal     []string    `json:"allowedValues"`
	AllowedPattern string      `json:"allowedPattern"`
}

// PluginConfig stores plugin configuration
type PluginConfig struct {
	Settings    interface{} `json:"settings"`
	Properties  interface{} `json:"properties"`
	Description string      `json:"description"`
}

// InstancePluginConfig stores plugin configuration
type InstancePluginConfig struct {
	Action        string              `json:"action"` // plugin name
	Inputs        interface{}         `json:"inputs"` // Properties
	MaxAttempts   int                 `json:"maxAttempts"`
	Name          string              `json:"name"` // unique identifier
	OnFailure     string              `json:"onFailure"`
	Settings      interface{}         `json:"settings"`
	Timeout       int                 `json:"timeoutSeconds"`
	Preconditions map[string][]string `json:"precondition"`
}

// DocumentContent object which represents ssm document content.
type DocumentContent struct {
	SchemaVersion string                   `json:"schemaVersion"`
	Description   string                   `json:"description"`
	RuntimeConfig map[string]*PluginConfig `json:"runtimeConfig"`
	MainSteps     []*InstancePluginConfig  `json:"mainSteps"`
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
	Name               string       `json:"name"`
	Output             string       `json:"output"`
	StartDateTime      string       `json:"startDateTime"`
	EndDateTime        string       `json:"endDateTime"`
	OutputS3BucketName string       `json:"outputS3BucketName"`
	OutputS3KeyPrefix  string       `json:"outputS3KeyPrefix"`
	StandardOutput     string       `json:"standardOutput"`
	StandardError      string       `json:"standardError"`
}

// AgentConfiguration is a struct that stores information about the agent and instance.
type AgentConfiguration struct {
	AgentInfo  AgentInfo
	InstanceID string
}

type DocumentResult struct {
	MessageID     string
	PluginResults map[string]*PluginResult
	Status        ResultStatus
	LastPlugin    string
}
