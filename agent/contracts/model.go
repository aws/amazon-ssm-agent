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

import "encoding/json"

// ResultStatus provides the granular status of a plugin.
// These are internal states maintained by agent during the execution of a command/config
type ResultStatus string

const (
	// ResultStatusNotStarted represents NotStarted status
	ResultStatusNotStarted ResultStatus = "NotStarted"
	// ResultStatusInProgress represents InProgress status
	ResultStatusInProgress ResultStatus = "InProgress"
	// ResultStatusSuccess represents Success status
	ResultStatusSuccess ResultStatus = "Success"
	// ResultStatusSuccessAndReboot represents SuccessAndReboot status
	ResultStatusSuccessAndReboot ResultStatus = "SuccessAndReboot"
	// ResultStatusPassedAndReboot represents PassedAndReboot status
	ResultStatusPassedAndReboot ResultStatus = "PassedAndReboot"
	// ResultStatusFailed represents Failed status
	ResultStatusFailed ResultStatus = "Failed"
	// ResultStatusCancelled represents Cancelled status
	ResultStatusCancelled ResultStatus = "Cancelled"
	// ResultStatusTimedOut represents TimedOut status
	ResultStatusTimedOut ResultStatus = "TimedOut"
	// ResultStatusSkipped represents Skipped status
	ResultStatusSkipped ResultStatus = "Skipped"
	// ResultStatusTestFailure represents test failure
	ResultStatusTestFailure ResultStatus = "TestFailure"
	// ResultStatusTestPass represents test passing
	ResultStatusTestPass ResultStatus = "TestPass"
)

const (
	ExitWithSuccess int = 168
	ExitWithFailure int = 169
)

const (
	OnFailureModifier   string = "onFailure"
	OnSuccessModifier   string = "onSuccess"
	FinallyStepModifier string = "finallyStep"
)

const (
	ModifierValueExit           string = "exit"
	ModifierValueSuccessAndExit string = "successAndExit"
	ModifierValueTrue           string = "true"
)

// IsSuccess checks whether the result is success or not
func (rs ResultStatus) IsSuccess() bool {
	switch rs {
	case ResultStatusSuccess, ResultStatusPassedAndReboot, ResultStatusSuccessAndReboot:
		return true
	default:
		return false
	}
}

// IsReboot checks whether the result is reboot or not
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
	// ParamTypeStringMap represents the param type is StringMap
	ParamTypeStringMap = "StringMap"
)

// A Parameter in the DocumentContent of an MDS message.
type Parameter struct {
	DefaultVal     interface{} `json:"default" yaml:"default"`
	Description    string      `json:"description" yaml:"description"`
	ParamType      string      `json:"type" yaml:"type"`
	AllowedVal     []string    `json:"allowedValues" yaml:"allowedValues"`
	AllowedPattern string      `json:"allowedPattern" yaml:"allowedPattern"`
	MinChars       json.Number `json:"minChars,omitempty" yaml:"minChars,omitempty"`
	MaxChars       json.Number `json:"maxChars,omitempty" yaml:"maxChars,omitempty"`
	MinItems       json.Number `json:"minItems,omitempty" yaml:"minItems,omitempty"`
	MaxItems       json.Number `json:"maxItems,omitempty" yaml:"maxItems,omitempty"`
}

// PluginConfig stores plugin configuration
type PluginConfig struct {
	Settings    interface{} `json:"settings" yaml:"settings"`
	Properties  interface{} `json:"properties" yaml:"properties"`
	Description string      `json:"description" yaml:"description"`
}

// InstancePluginConfig stores plugin configuration
type InstancePluginConfig struct {
	Action        string              `json:"action" yaml:"action"` // plugin name
	Inputs        interface{}         `json:"inputs" yaml:"inputs"` // Properties
	MaxAttempts   int                 `json:"maxAttempts" yaml:"maxAttempts"`
	Name          string              `json:"name" yaml:"name"` // unique identifier
	OnFailure     string              `json:"onFailure" yaml:"onFailure"`
	Settings      interface{}         `json:"settings" yaml:"settings"`
	Timeout       int                 `json:"timeoutSeconds" yaml:"timeoutSeconds"`
	Preconditions map[string][]string `json:"precondition" yaml:"precondition"`
}

// DocumentContent object which represents ssm document content.
type DocumentContent struct {
	SchemaVersion string                   `json:"schemaVersion" yaml:"schemaVersion"`
	Description   string                   `json:"description" yaml:"description"`
	RuntimeConfig map[string]*PluginConfig `json:"runtimeConfig" yaml:"runtimeConfig"`
	MainSteps     []*InstancePluginConfig  `json:"mainSteps" yaml:"mainSteps"`
	Parameters    map[string]*Parameter    `json:"parameters" yaml:"parameters"`

	// InvokedPlugin field is set when document is invoked from any other plugin.
	// Currently, InvokedPlugin is set only in runDocument Plugin
	InvokedPlugin string
}

// SessionInputs stores session configuration
type SessionInputs struct {
	S3BucketName                string             `json:"s3BucketName" yaml:"s3BucketName"`
	S3KeyPrefix                 string             `json:"s3KeyPrefix" yaml:"s3KeyPrefix"`
	S3EncryptionEnabled         interface{}        `json:"s3EncryptionEnabled" yaml:"s3EncryptionEnabled"`
	CloudWatchLogGroupName      string             `json:"cloudWatchLogGroupName" yaml:"cloudWatchLogGroupName"`
	CloudWatchStreamingEnabled  bool               `json:"cloudWatchStreamingEnabled" yaml:"cloudWatchStreamingEnabled"`
	CloudWatchEncryptionEnabled interface{}        `json:"cloudWatchEncryptionEnabled" yaml:"cloudWatchEncryptionEnabled"`
	KmsKeyId                    string             `json:"kmsKeyId" yaml:"kmsKeyId"`
	RunAsEnabled                interface{}        `json:"runAsEnabled" yaml:"runAsEnabled"`
	RunAsDefaultUser            string             `json:"runAsDefaultUser" yaml:"runAsDefaultUser"`
	ShellProfile                ShellProfileConfig `json:"shellProfile" yaml:"shellProfile"`
}

// ShellProfileConfig stores shell profile config
type ShellProfileConfig struct {
	Windows string `json:"windows" yaml:"windows"`
	Linux   string `json:"linux" yaml:"linux"`
}

// SessionDocumentContent object which represents ssm session content.
type SessionDocumentContent struct {
	SchemaVersion string                `json:"schemaVersion" yaml:"schemaVersion"`
	Description   string                `json:"description" yaml:"description"`
	SessionType   string                `json:"sessionType" yaml:"sessionType"`
	Inputs        SessionInputs         `json:"inputs" yaml:"inputs"`
	Parameters    map[string]*Parameter `json:"parameters" yaml:"parameters"`
	Properties    interface{}           `json:"properties" yaml:"properties"`
}

// AdditionalInfo section in agent response
type AdditionalInfo struct {
	Agent                   AgentInfo      `json:"agent"`
	DateTime                string         `json:"dateTime"`
	RunID                   string         `json:"runId"`
	RuntimeStatusCounts     map[string]int `json:"runtimeStatusCounts"`
	AbleToOpenMGSConnection *bool          `json:"ableToOpenMGSConnection,omitempty"`
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
	StepName           string       `json:"stepName"`
	StandardOutput     string       `json:"standardOutput"`
	StandardError      string       `json:"standardError"`
}

// AgentConfiguration is a struct that stores information about the agent and instance
type AgentConfiguration struct {
	AgentInfo  AgentInfo
	InstanceID string
}

// DocumentResult is a struct that stores information about the result of the document
type DocumentResult struct {
	DocumentName        string
	DocumentVersion     string
	MessageID           string
	AssociationID       string
	PluginResults       map[string]*PluginResult
	Status              ResultStatus
	LastPlugin          string
	NPlugins            int
	UpstreamServiceName UpstreamServiceName
	ResultType          ResultType
	RelatedDocumentType DocumentType
}

// ResultType represents document Result types
type ResultType string

const (
	// RunCommandResult represents result sent by document worker which ran runCommand documents
	RunCommandResult ResultType = "RunCommandResult"
	// SessionResult represents result sent by session worker to service
	SessionResult ResultType = "SessionResult"
)

// StatusComm is a struct that holds channels to pass status
// between ssmAgentCore go routine and agent's main go routine
type StatusComm struct {
	TerminationChan chan struct{}
	DoneChan        chan struct{}
}
