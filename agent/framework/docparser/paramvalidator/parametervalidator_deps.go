// Copyright 2022 Amazon.com, Inc. or its affiliates. All Rights Reserved.
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

// Package paramvalidator is responsible for registering all the param validators available
// and exposes getter functions to be utilized by other modules
package paramvalidator

import (
	"github.com/aws/amazon-ssm-agent/agent/contracts"
	"github.com/aws/amazon-ssm-agent/agent/log"
)

// ParameterValidator is the interface for various Parameter validators
type ParameterValidator interface {
	// Validate validates the parameter value based on the parameter definition
	Validate(log log.T, parameterValue interface{}, parameter *contracts.Parameter) error
	// GetName returns the name of param validator
	GetName() string
}
