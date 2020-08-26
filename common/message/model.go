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

// Package message contains information for the IPC messages.
package message

import (
	"encoding/json"
)

// HealthResultPayload contains information required by Core Agent to decide if a worker is healthy
type HealthResultPayload struct {
	SchemaVersion int
	Name          string
	WorkerType    WorkerType
	Pid           int
}

// TerminateWorkerResultPayload contains worker termination result
type TerminateWorkerResultPayload struct {
	SchemaVersion int
	Name          string
	WorkerType    WorkerType
	Pid           int
	IsTerminating bool
}

type Message struct {
	SchemaVersion int
	Topic         TopicType
	Payload       []byte
}

// WorkerType is the type of the worker
type WorkerType string

// TopicType is the message type for IPC messages
type TopicType string

const (
	LongRunning WorkerType = "LongRunning"
	OnDemand    WorkerType = "OnDemand"

	SchemaVersion = 1

	GetWorkerHealthRequest TopicType = "GetWorkerHealthRequest"
	GetWorkerHealthResult  TopicType = "GetWorkerHealthResult"
	TerminateWorkerRequest TopicType = "TerminateWorkerRequest"
	TerminateWorkerResult  TopicType = "TerminateWorkerResult"
)

// CreateHealthRequest creates an instance of health request message
func CreateHealthRequest() *Message {
	return &Message{
		SchemaVersion: SchemaVersion,
		Topic:         GetWorkerHealthRequest,
	}
}

// CreateHealthResult creates an instance of health result message
func CreateHealthResult(workerName string, workerType WorkerType, pid int) (*Message, error) {
	var message *Message

	payload := HealthResultPayload{
		SchemaVersion: SchemaVersion,
		Name:          workerName,
		WorkerType:    workerType,
		Pid:           pid,
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return message, err
	}

	message = &Message{
		SchemaVersion: payload.SchemaVersion,
		Topic:         GetWorkerHealthResult,
		Payload:       payloadBytes,
	}

	return message, nil
}

// CreateTerminateWorkerRequest creates an instance of terminate worker request message
func CreateTerminateWorkerRequest() *Message {
	return &Message{
		SchemaVersion: SchemaVersion,
		Topic:         TerminateWorkerRequest,
	}
}

// CreateTerminateWorkerRequest creates an instance of terminate worker result message
func CreateTerminateWorkerResult(
	workerName string,
	workerType WorkerType,
	pid int,
	isTerminating bool) (*Message, error) {

	payload := TerminateWorkerResultPayload{
		SchemaVersion: SchemaVersion,
		Name:          workerName,
		WorkerType:    workerType,
		Pid:           pid,
		IsTerminating: isTerminating,
	}
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	return &Message{
		SchemaVersion: payload.SchemaVersion,
		Topic:         GetWorkerHealthResult,
		Payload:       payloadBytes,
	}, err
}
