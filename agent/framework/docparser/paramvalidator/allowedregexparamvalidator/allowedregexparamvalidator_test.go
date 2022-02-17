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

// Package allowedregexparamvalidator is responsible for validating parameter value
// with regex pattern given in the document.
package allowedregexparamvalidator

import (
	"fmt"
	"testing"

	"github.com/aws/amazon-ssm-agent/agent/contracts"
	"github.com/aws/amazon-ssm-agent/agent/framework/docparser/paramvalidator/utils"
	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
)

// allowedPatternParameterTestSuite executes the test suite
// containing tests related to allowed regex param validators
type allowedPatternParameterTestSuite struct {
	suite.Suite
	allowedRegexValidator *allowedRegexParamValidator
	testCaseList          []*testCases
	log                   log.T
}
type testStatus string

const (
	success testStatus = "Success"
	failure testStatus = "Failure"
)

type testCases struct {
	allowedRegex        string
	testInput           []interface{}
	testOutput          testStatus
	name                string
	parameterType       utils.DocumentParamType
	errorStringContains string
}

// TestAllowedPatternParameterTestSuite executes the test suite containing tests related to allowed pattern param validators
func TestAllowedPatternParameterTestSuite(t *testing.T) {
	allowedRegexParamSuite := new(allowedPatternParameterTestSuite)
	allowedRegexParamSuite.allowedRegexValidator = GetAllowedRegexValidator()
	suite.Run(t, allowedRegexParamSuite)
}

func (suite *allowedPatternParameterTestSuite) SetupTest() {
	suite.log = log.NewMockLog()
	suite.testCaseList = make([]*testCases, 0)
	validRegexValues := &testCases{
		allowedRegex: "^([\"{:}A-Za-z0-9]*)$",
		testInput: []interface{}{
			"testValue1",
			[]string{"testValue1", "testValue2"},
			[]interface{}{"testValue1", "testValue2"},
			"{\"testKey\":\"testVal\"}",
			map[string]interface{}{"testKey": "testVal"},
		},
		name:       "TestValidRegexValues",
		testOutput: success}
	suite.testCaseList = append(suite.testCaseList, validRegexValues)
	inValidRegexValues := &testCases{
		allowedRegex: "^([A-Za-z0-9]*)$",
		testInput: []interface{}{
			"test-Value1",
			[]string{"test-Value1", "testValue2"},
			[]interface{}{"test-Value1", "testValue2"},
			"{\"test-key\":\"test-value\"}",
			map[string]interface{}{"test-key": "test-val"},
			2,
			true,
		},
		name:       "TestInValidRegexValues",
		testOutput: failure}
	suite.testCaseList = append(suite.testCaseList, inValidRegexValues)
}

func (suite *allowedPatternParameterTestSuite) TestValidate_MultipleTestCases() {
	log := log.NewMockLog()
	for _, testCase := range suite.testCaseList {
		for _, input := range testCase.testInput {
			paramValue := input
			parameterConfig := contracts.Parameter{
				AllowedPattern: testCase.allowedRegex,
				ParamType:      string(testCase.parameterType),
			}
			err := suite.allowedRegexValidator.Validate(log, paramValue, &parameterConfig)
			if testCase.testOutput == success {
				assert.Nil(suite.T(), err, fmt.Sprintf("Testcase details: parameter %v, paramValue %v, testCaseName: %v", parameterConfig, paramValue, testCase.name))
			} else {
				assert.Contains(suite.T(), err.Error(), testCase.errorStringContains)
				assert.NotNil(suite.T(), err, fmt.Sprintf("Testcase details: parameter %v, paramValue %v, testCaseName: %v", parameterConfig, paramValue, testCase.name))
			}
		}
	}
}
