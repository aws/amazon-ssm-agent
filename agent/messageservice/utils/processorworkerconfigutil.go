// Copyright 2021 Amazon.com, Inc. or its affiliates. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License"). You may not
// use this file except in compliance with the License. A copy of the
// License is located at
//
// http://aws.amazon.com/apache2.0/
//
// or in the "license" file accompanying this file. This file is distributed
// on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND,
// either express or implied. See the License for the specific language governing`
// permissions and limitations under the License.

// Package utils provides utility functions to be used by interactors
package utils

import (
	"path/filepath"

	"github.com/aws/amazon-ssm-agent/agent/appconfig"
	"github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/aws/amazon-ssm-agent/agent/contracts"
)

// ProcessorName represents the currently supported processor name
type ProcessorName string

const (
	// CommandProcessor represents the command processor wrapper which handles send command and cancel command
	CommandProcessor ProcessorName = "CommandProcessor"

	// SessionProcessor represents the session processor wrapper which handles start session and terminate session
	SessionProcessor ProcessorName = "SessionProcessor"
)

// WorkerName represents the currently supported workers
type WorkerName string

var (
	// DocumentWorkerName represents document worker name
	DocumentWorkerName = WorkerName(filepath.Base(appconfig.DefaultDocumentWorker))

	// SessionWorkerName represents session worker name
	SessionWorkerName = WorkerName(filepath.Base(appconfig.DefaultSessionWorker))
)

// ProcessorWorkerConfig defines the different processor configurations along with worker details
type ProcessorWorkerConfig struct {
	ProcessorName           ProcessorName
	WorkerName              WorkerName
	StartWorkerLimit        int
	CancelWorkerLimit       int
	StartWorkerDocType      contracts.DocumentType
	CancelWorkerDocType     contracts.DocumentType
	StartWorkerBufferLimit  int
	CancelWorkerBufferLimit int
}

// LoadProcessorWorkerConfig loads all the processor and worker configurations used for initializing processors
// We launch equal number of processors returned by this function in message handler
func LoadProcessorWorkerConfig(context context.T) map[WorkerName]*ProcessorWorkerConfig {
	appConfigVal := context.AppConfig()
	config := make(map[WorkerName]*ProcessorWorkerConfig)
	docProcWorkConfig := &ProcessorWorkerConfig{
		ProcessorName:           CommandProcessor,
		WorkerName:              DocumentWorkerName,
		StartWorkerLimit:        appConfigVal.Mds.CommandWorkersLimit,
		CancelWorkerLimit:       appconfig.DefaultCancelWorkersLimit,
		StartWorkerDocType:      contracts.SendCommand,
		CancelWorkerDocType:     contracts.CancelCommand,
		StartWorkerBufferLimit:  appConfigVal.Mds.CommandWorkerBufferLimit, // provides buffer limit equivalent to worker limit
		CancelWorkerBufferLimit: 1,
	}
	sessionProcWorkConfig := &ProcessorWorkerConfig{
		ProcessorName:           SessionProcessor,
		WorkerName:              SessionWorkerName,
		StartWorkerLimit:        appConfigVal.Mgs.SessionWorkersLimit,
		CancelWorkerLimit:       appconfig.DefaultCancelWorkersLimit,
		StartWorkerDocType:      contracts.StartSession,
		CancelWorkerDocType:     contracts.TerminateSession,
		StartWorkerBufferLimit:  appConfigVal.Mgs.SessionWorkerBufferLimit, // providing just 1 as buffer
		CancelWorkerBufferLimit: 1,                                         // providing just 1 as buffer
	}
	config[docProcWorkConfig.WorkerName] = docProcWorkConfig
	config[sessionProcWorkConfig.WorkerName] = sessionProcWorkConfig
	return config
}
