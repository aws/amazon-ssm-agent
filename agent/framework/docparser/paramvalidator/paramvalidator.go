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

// Package paramvalidator is responsible for registering all the param validators for a document available
// and exposes getter functions to be utilized by other modules
package paramvalidator

import (
	"github.com/aws/amazon-ssm-agent/agent/framework/docparser/paramvalidator/allowedregexparamvalidator"
	"github.com/aws/amazon-ssm-agent/agent/framework/docparser/paramvalidator/allowedvalueparamvalidator"
	"github.com/aws/amazon-ssm-agent/agent/framework/docparser/paramvalidator/minmaxcharparamvalidator"
	"github.com/aws/amazon-ssm-agent/agent/framework/docparser/paramvalidator/minmaxitemparamvalidator"
)

var mandatoryValidators []ParameterValidator
var optionalValidators []ParameterValidator

// GetMandatoryValidators returns all the registered mandatory parameter validators
func GetMandatoryValidators() []ParameterValidator {
	if mandatoryValidators == nil {
		mandatoryValidators = make([]ParameterValidator, 1)
		mandatoryValidators[0] = allowedregexparamvalidator.GetAllowedRegexValidator()
	}
	return mandatoryValidators
}

// GetOptionalValidators returns all the registered optional parameter validators
func GetOptionalValidators() []ParameterValidator {
	if optionalValidators == nil {
		optionalValidators = make([]ParameterValidator, 3)
		optionalValidators[0] = minmaxcharparamvalidator.GetMinMaxCharValidator()
		optionalValidators[1] = minmaxitemparamvalidator.GetMinMaxItemValidator()
		optionalValidators[2] = allowedvalueparamvalidator.GetAllowedValueParamValidator()
	}
	return optionalValidators
}
