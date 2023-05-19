// Copyright 2020 Amazon.com, Inc. or its affiliates. All Rights Reserved.
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

// Package model contains data objects for long running container
package model

// WorkerConfig contains worker configuration information, such as path and args.
type WorkerConfig struct {
	Name       string
	BinaryName string
	Path       string
	Args       []string
}

// Worker represents the worker information, one worker could have multiple instances running
type Worker struct {
	Name      string           `json:"name"`
	Processes map[int]*Process `json:"processes"`
	Config    *WorkerConfig    `json:"-"`
}

// Process represent the process information for the worker, such as pid
type Process struct {
	Pid    int           `json:"pid"`
	Status ProcessStatus `json:"status"`
}

const (
	SSMAgentName       = "amazon-ssm-agent"
	SSMAgentWorkerName = "ssm-agent-worker"
)

// ProcessStatus represents status for process: active or unknown
type ProcessStatus string

const (
	Active  ProcessStatus = "Active"
	Unknown ProcessStatus = "Unknown"
)
