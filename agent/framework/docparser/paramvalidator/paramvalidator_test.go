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
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
)

type ParamValidatorTestSuite struct {
	suite.Suite
}

// ParamValidatorTestSuite executes the test suite containing tests related to param validators
func TestParamValidatorTestSuite(t *testing.T) {
	suite.Run(t, new(ParamValidatorTestSuite))
}

func (suite *ParamValidatorTestSuite) TestParamValidator_InitializationCountCheck() {
	currentValidatorCount := 4
	uniqueValidators := 4
	uniqueValidatorMap := make(map[string]struct{})
	allValidators := GetMandatoryValidators()
	assert.Equal(suite.T(), len(allValidators), 1) // mandatory validator count check
	allValidators = append(allValidators, GetOptionalValidators()...)
	for _, paramValidator := range allValidators {
		uniqueValidatorMap[paramValidator.GetName()] = struct{}{}
	}
	assert.Equal(suite.T(), currentValidatorCount, len(allValidators))
	assert.Equal(suite.T(), uniqueValidators, len(uniqueValidatorMap))
}
