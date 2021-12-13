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
// either express or implied. See the License for the specific language governing
// permissions and limitations under the License.

// Package interactor contains the logic to communicate with upstream core services MGS & MDS
package interactor

import "github.com/aws/amazon-ssm-agent/agent/messageservice/utils"

// IInteractor defines the interface for interactors
type IInteractor interface {
	// Initialize initializes interactor
	Initialize() error
	// PostProcessorInitialization sets values needed post initialization
	// This should be called after processor initialization call
	PostProcessorInitialization(name utils.WorkerName)
	// GetName used to get the name of interactor
	GetName() string
	// GetSupportedWorkers returns the workers supported by the interactors
	GetSupportedWorkers() []utils.WorkerName
	// PreProcessorClose lists the actions to do before processor stop
	// This should be called before processor close
	PreProcessorClose()
	// Close closes interactor
	Close() error
}
