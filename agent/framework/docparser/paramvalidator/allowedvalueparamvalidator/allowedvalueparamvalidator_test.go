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

// Package allowedvalueparamvalidator is responsible for validating parameter value
// with the allowed values given in the document.
package allowedvalueparamvalidator

import (
	"fmt"
	"math"
	"testing"

	"github.com/aws/amazon-ssm-agent/agent/contracts"
	"github.com/aws/amazon-ssm-agent/agent/framework/docparser/paramvalidator/utils"
	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
)

// allowedValueParameterTestSuite executes the test suite
// containing tests related to allowed values param validators
type allowedValueParameterTestSuite struct {
	suite.Suite
	allowedValValidator *allowedValueParamValidator
	testCaseList        []*testCases
	log                 log.T
}
type testStatus string

const (
	success testStatus = "Success"
	failure testStatus = "Failure"
)

type testCases struct {
	allowedVal          []string
	testInput           []interface{}
	testOutput          testStatus
	name                string
	parameterType       utils.DocumentParamType
	defaultVal          interface{}
	errorStringContains string
}

// TestAllowedValueParameterTestSuite executes the test suite containing tests related to allowed values param validators
func TestAllowedValueParameterTestSuite(t *testing.T) {
	allowedValValueParamSuite := new(allowedValueParameterTestSuite)
	allowedValValueParamSuite.allowedValValidator = GetAllowedValueParamValidator()
	suite.Run(t, allowedValValueParamSuite)
}

func (suite *allowedValueParameterTestSuite) SetupTest() {
	suite.log = log.NewMockLog()
	suite.testCaseList = make([]*testCases, 0)
	validAllowedValues := &testCases{
		allowedVal: []string{"testValue1", "testValue2", "testValue3"},
		testInput: []interface{}{
			"testValue1",
			[]string{"testValue1", "testValue2"},
			[]interface{}{"testValue1", "testValue2"}},
		name:       "TestValidAllowedValues",
		testOutput: success}
	suite.testCaseList = append(suite.testCaseList, validAllowedValues)
	mapListInAllowedList := &testCases{
		allowedVal: []string{"{\"path\":\"test1\"}"},
		testInput: []interface{}{
			map[string]interface{}{"path": "test1"},
			"{\"path\":\"test1\"}",
			[]interface{}{map[string]interface{}{"path": "test1"}, map[string]interface{}{"path": "test1"}},
		},
		name:                "TestMapListInAllowedList",
		errorStringContains: "parameter value /testValue1/ is not in the allowed list [testValue2 testValue3]",
		testOutput:          success}
	suite.testCaseList = append(suite.testCaseList, mapListInAllowedList)
	mapListNotInAllowedList := &testCases{
		allowedVal: []string{"{\"path\":\"test2\"}"},
		testInput: []interface{}{
			map[string]interface{}{"path": "test1"},
			"{\"path\":\"test1\"}",
			[]interface{}{map[string]interface{}{"path": "test1"}, map[string]interface{}{"path": "test1"}},
		},
		name:                "TestMapListNotInAllowedList",
		errorStringContains: "is not in the allowed list",
		testOutput:          failure}
	suite.testCaseList = append(suite.testCaseList, mapListNotInAllowedList)
	missingAllowedValues := &testCases{
		allowedVal: []string{"testValue2", "testValue3"},
		testInput: []interface{}{
			"testValue1",
			[]string{"testValue1", "testValue2"},
			[]interface{}{"testValue1", "testValue2"}},
		name:                "TestMissingAllowedValues",
		errorStringContains: "parameter value /testValue1/ is not in the allowed list [testValue2 testValue3]",
		testOutput:          failure}
	suite.testCaseList = append(suite.testCaseList, missingAllowedValues)
	integerAllowedValues := &testCases{
		allowedVal:    []string{"3", "4"},
		defaultVal:    "1",
		testInput:     []interface{}{float64(1)},
		name:          "TestIntegerAllowedValues",
		parameterType: utils.ParamTypeInteger,
		testOutput:    success}
	suite.testCaseList = append(suite.testCaseList, integerAllowedValues)
	integerOutOfBoundaryLimit := &testCases{
		allowedVal:          []string{"1", "2"},
		defaultVal:          "2",
		testInput:           []interface{}{float64(math.MaxInt32 + 1)},
		name:                "TestIntegerAllowedValues",
		parameterType:       utils.ParamTypeInteger,
		errorStringContains: "out-of-boundary error",
		testOutput:          failure}
	suite.testCaseList = append(suite.testCaseList, integerOutOfBoundaryLimit)
	integerNotAllowedValues := &testCases{
		allowedVal:          []string{"1", "2"},
		defaultVal:          "3",
		testInput:           []interface{}{float64(1)},
		name:                "TestIntegerNotAllowedValues",
		parameterType:       utils.ParamTypeInteger,
		testOutput:          failure,
		errorStringContains: "parameter value /1/ not equal to default /3/ for Integer",
	}
	suite.testCaseList = append(suite.testCaseList, integerNotAllowedValues)
	booleanAllowedValues := &testCases{
		allowedVal:    []string{"false"},
		defaultVal:    false,
		testInput:     []interface{}{false},
		name:          "TestBoolAllowedValues",
		parameterType: utils.ParamTypeBoolean,
		testOutput:    success}
	suite.testCaseList = append(suite.testCaseList, booleanAllowedValues)
	booleanNotAllowedValues := &testCases{
		allowedVal:          []string{"false"},
		defaultVal:          "false",
		testInput:           []interface{}{true},
		name:                "TestBoolNotAllowedValues",
		parameterType:       utils.ParamTypeBoolean,
		testOutput:          failure,
		errorStringContains: "parameter value /true/ not equal to default for Boolean",
	}
	suite.testCaseList = append(suite.testCaseList, booleanNotAllowedValues)
}

func (suite *allowedValueParameterTestSuite) TestValidate_MultipleTestCases() {
	log := log.NewMockLog()
	for _, testCase := range suite.testCaseList {
		for _, input := range testCase.testInput {
			paramValue := input
			parameterConfig := contracts.Parameter{
				AllowedVal: testCase.allowedVal,
				DefaultVal: testCase.defaultVal,
				ParamType:  string(testCase.parameterType),
			}
			err := suite.allowedValValidator.Validate(log, paramValue, &parameterConfig)
			if testCase.testOutput == success {
				assert.Nil(suite.T(), err, fmt.Sprintf("Testcase details: parameter %v, paramValue %v, testCaseName: %v", parameterConfig, paramValue, testCase.name))
			} else {
				assert.Contains(suite.T(), err.Error(), testCase.errorStringContains)
				assert.NotNil(suite.T(), err, fmt.Sprintf("Testcase details: parameter %v, paramValue %v, testCaseName: %v", parameterConfig, paramValue, testCase.name))
			}
		}
	}
}

func (suite *allowedValueParameterTestSuite) TestCheckValuesAllowed_Success_ValidInputs() {
	err := suite.allowedValValidator.isValueInAllowedList("test1", []string{"test1", "test2"})
	assert.Nil(suite.T(), err)
	err = suite.allowedValValidator.isValueInAllowedList("test2", []string{"test2"})
	assert.Nil(suite.T(), err)
}

func (suite *allowedValueParameterTestSuite) TestCheckValuesInAllowed_Success_ValidInputs() {
	err := suite.allowedValValidator.isValueInAllowedList("test3", []string{"test1", "test2"})
	assert.NotNil(suite.T(), err)
	assert.Contains(suite.T(), err.Error(), "is not in the allowed list")
	err = suite.allowedValValidator.isValueInAllowedList("test2", []string{"test3"})
	assert.Contains(suite.T(), err.Error(), "is not in the allowed list")
	assert.NotNil(suite.T(), err)
}

func (suite *allowedValueParameterTestSuite) TestVerifyAllowedValuesAfterMarshall_Success_ValidInputs() {
	log := log.NewMockLog()
	input := make(map[string]map[string]struct{})
	input["test"] = make(map[string]struct{})
	err := suite.allowedValValidator.verifyAllowedValuesAfterMarshall(log, input, []string{"{\"test\":{}}"})
	assert.Nil(suite.T(), err)
}

func (suite *allowedValueParameterTestSuite) TestVerifyAllowedValuesAfterMarshall_Failure_InValidInputs() {
	log := log.NewMockLog()
	input := make(map[string]map[string]struct{})
	input["test1"] = make(map[string]struct{})
	err := suite.allowedValValidator.verifyAllowedValuesAfterMarshall(log, input, []string{"{\"testInvalid\":{}}"})
	assert.NotNil(suite.T(), err)
}
