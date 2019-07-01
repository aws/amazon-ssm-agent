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

// Package contracts provides model definitions for document state
package contracts

import (
	"time"
)

// DocumentType defines the type of document persists locally.
type DocumentType string

const (
	// SendCommand represents document type for send command
	SendCommand DocumentType = "SendCommand"
	// CancelCommand represents document type for cancel command
	CancelCommand DocumentType = "CancelComamnd"
	// Association represents document type for association
	Association DocumentType = "Association"
	// StartSession represents document type for start session
	StartSession DocumentType = "StartSession"
	// TerminateSession represents document type for terminate session
	TerminateSession DocumentType = "TerminateSession"
	// SendCommandOffline represents document type for send command received from offline service
	SendCommandOffline DocumentType = "SendCommandOffline"
	// CancelCommandOffline represents document type for cancel command received from offline service
	CancelCommandOffline DocumentType = "CancelCommandOffline"
)

// PluginState represents information stored as interim state for any plugin
// This has both the configuration with which a plugin gets executed and a
// corresponding plugin result.
type PluginState struct {
	Configuration Configuration
	Name          string
	//TODO truncate this struct
	Result PluginResult
	Id     string
}

// OSProcInfo represents information about the new process for outofproc
type OSProcInfo struct {
	Pid       int
	StartTime time.Time
}

// DocumentInfo represents information stored as interim state for a document
type DocumentInfo struct {
	// DocumentID is a unique name for file system
	// For Association, DocumentID = AssociationID.RunID
	// For RunCommand, DocumentID = CommandID
	// For Session, DocumentId = SessionId
	DocumentID      string
	CommandID       string
	AssociationID   string
	InstanceID      string
	MessageID       string
	RunID           string
	CreatedDate     string
	DocumentName    string
	DocumentVersion string
	DocumentStatus  ResultStatus
	RunCount        int
	ProcInfo        OSProcInfo
	ClientId        string
	RunAsUser       string
}

//CloudWatchConfiguration represents information relevant to command output in cloudWatch
type CloudWatchConfiguration struct {
	LogGroupName              string
	LogStreamPrefix           string
	LogGroupEncryptionEnabled bool
}

// IOConfiguration represents information relevant to the output sources of a command
type IOConfiguration struct {
	OrchestrationDirectory string
	OutputS3BucketName     string
	OutputS3KeyPrefix      string
	CloudWatchConfig       CloudWatchConfiguration
}

// DocumentState represents information relevant to a command that gets executed by agent
type DocumentState struct {
	DocumentInformation        DocumentInfo
	DocumentType               DocumentType
	SchemaVersion              string
	InstancePluginsInformation []PluginState
	CancelInformation          CancelCommandInfo
	IOConfig                   IOConfiguration
}

// IsRebootRequired returns if reboot is needed
func (c *DocumentState) IsRebootRequired() bool {
	return c.DocumentInformation.DocumentStatus == ResultStatusSuccessAndReboot
}

// IsAssociation returns if documentType is association
func (c *DocumentState) IsAssociation() bool {
	return c.DocumentType == Association
}

// CancelCommandInfo represents information relevant to a cancel-command that agent receives
// TODO  This might be revisited when Agent-cli is written to list previously executed commands
type CancelCommandInfo struct {
	CancelMessageID string
	CancelCommandID string
	Payload         string
	DebugInfo       string
}

// UpdateDocState updates the current document state
func UpdateDocState(docResult *DocumentResult, docState *DocumentState) {
	docState.DocumentInformation.DocumentStatus = docResult.Status
	pluginID := docResult.LastPlugin
	if pluginID != "" {
		for i := 0; i < len(docState.InstancePluginsInformation); i++ {
			if docState.InstancePluginsInformation[i].Id == docResult.LastPlugin {
				docState.InstancePluginsInformation[i].Result = *docResult.PluginResults[pluginID]
			}
		}
	}
}
