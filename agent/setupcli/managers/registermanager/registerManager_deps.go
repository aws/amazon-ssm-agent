// Copyright 2023 Amazon.com, Inc. or its affiliates. All Rights Reserved.
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

// Package registermanager contains functions related to register
package registermanager

// RegisterAgentInputModel represents the input model used for registering agent
type RegisterAgentInputModel struct {
	Region             string
	Role               string
	Tags               string
	ActivationCode     string
	ActivationId       string
	IsFirstTimeInstall bool // will be used only for Windows
}

type IRegisterManager interface {
	// RegisterAgent registers the agent using aws credentials registration,
	// this call will override existing registration using force flag
	RegisterAgent(registerAgentInpModel *RegisterAgentInputModel) error
}
