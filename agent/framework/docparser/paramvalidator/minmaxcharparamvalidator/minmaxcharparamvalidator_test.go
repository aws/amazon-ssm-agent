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

// Package minmaxcharparamvalidator is responsible for validating parameter value
// with the min max char restriction given in the document for parameters.
package minmaxcharparamvalidator

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/aws/amazon-ssm-agent/agent/contracts"
	"github.com/aws/amazon-ssm-agent/agent/framework/docparser/paramvalidator/utils"
	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
)

// minMaxCharParamValidatorTestSuite executes the test suite
// containing tests related to min max char param validators
type minMaxCharParamValidatorTestSuite struct {
	suite.Suite
	maxMinCharValidator *minMaxCharParamValidator
	testCaseList        []*testCases
	log                 log.T
}
type testStatus string

const (
	success testStatus = "Success"
	failure testStatus = "Failure"
)

type testCases struct {
	minChar       json.Number
	maxChar       json.Number
	testInput     []interface{}
	testOutput    testStatus
	name          string
	parameterType utils.DocumentParamType
}

// MinMaxCharParamValidatorTestSuite executes the test suite containing tests related to min/max param validators
func TestMinMaxCharParamValidatorTestSuite(t *testing.T) {
	minMaxChar := new(minMaxCharParamValidatorTestSuite)
	minMaxChar.maxMinCharValidator = GetMinMaxCharValidator()
	suite.Run(t, minMaxChar)
}

func (suite *minMaxCharParamValidatorTestSuite) SetupTest() {
	suite.log = log.NewMockLog()
	suite.testCaseList = make([]*testCases, 0)
	minGreaterThanZeroMaxNilCase := &testCases{minChar: "2", maxChar: "",
		testInput: []interface{}{
			"testValue1",
			[]string{"testValue1", "testValue2"},
			[]interface{}{"testValue1", "testValue2"}},
		name:       "TestMinGreaterThanZeroMaxNil",
		testOutput: success}
	suite.testCaseList = append(suite.testCaseList, minGreaterThanZeroMaxNilCase)
	minLessThanZeroMaxNilCase := &testCases{minChar: "-2", maxChar: "",
		testInput: []interface{}{
			"testValue1",
			[]string{"testValue1", "testValue2"},
			[]interface{}{"testValue1", "testValue2"}},
		name:       "TestMinLessThanZeroMaxNil",
		testOutput: success}
	suite.testCaseList = append(suite.testCaseList, minLessThanZeroMaxNilCase)
	minNilMaxNilCase := &testCases{minChar: "", maxChar: "",
		testInput: []interface{}{
			"testValue1",
			[]string{"testValue1", "testValue2"},
			[]interface{}{"testValue1", "testValue2"}},
		name:       "TestMinNilMaxNil",
		testOutput: success}
	suite.testCaseList = append(suite.testCaseList, minNilMaxNilCase)
	minMaxLessThanZeroCase := &testCases{minChar: "-2", maxChar: "-2",
		testInput: []interface{}{
			"testValue1",
			[]string{"testValue1", "testValue2"},
			[]interface{}{"testValue1", "testValue2"}},
		name:       "TestMinMaxLessThanZero",
		testOutput: success}
	suite.testCaseList = append(suite.testCaseList, minMaxLessThanZeroCase)
	testInputMap := make(map[string]interface{})
	testInputMap["test1"] = "testValue1"
	minLessThanZeroMaxGreaterThanIntCase := &testCases{minChar: "-2", maxChar: "2147483650",
		testInput: []interface{}{
			"testValue1",
			[]string{"testValue1", "testValue2"},
			[]interface{}{"testValue1", "testValue2"},
			testInputMap},
		name:       "TestMinLessThanZeroMaxGreaterThanInt",
		testOutput: success}
	suite.testCaseList = append(suite.testCaseList, minLessThanZeroMaxGreaterThanIntCase)
	minLessThanLimitMaxNilCase := &testCases{minChar: "5", maxChar: "",
		testInput: []interface{}{
			"123",
			[]string{"123", "testValue2"},
			[]interface{}{"testValue1", "123"}},
		name:       "TestMinLessThanLimitMaxNil",
		testOutput: failure}
	suite.testCaseList = append(suite.testCaseList, minLessThanLimitMaxNilCase)
	minEmptyMaxGreaterThanLimit := &testCases{minChar: "", maxChar: "5",
		testInput: []interface{}{
			"testValue1",
			[]string{"testValue1", "testValue2"},
			[]interface{}{"testValue1", "testValue2"}},
		name:       "TestMinEmptyMaxGreaterThanLimit",
		testOutput: failure}
	suite.testCaseList = append(suite.testCaseList, minEmptyMaxGreaterThanLimit)
	minLessThanZeroMaxGreaterThanLimit := &testCases{minChar: "-2", maxChar: "5",
		testInput: []interface{}{
			"testValue1",
			[]string{"testValue1", "testValue2"},
			[]interface{}{"testValue1", "testValue2"}},
		name:       "TestMinLessThanZeroMaxGreaterThanLimit",
		testOutput: failure}
	suite.testCaseList = append(suite.testCaseList, minLessThanZeroMaxGreaterThanLimit)
	minGreaterThanMax := &testCases{minChar: "14", maxChar: "10",
		testInput: []interface{}{
			"testValue1",
			[]string{"testValue1", "testValue2"},
			[]interface{}{"testValue1", "testValue2"}},
		name:       "TestMinGreaterThanMax",
		testOutput: failure}
	suite.testCaseList = append(suite.testCaseList, minGreaterThanMax)
	skipInteger := &testCases{minChar: "14", maxChar: "10",
		testInput:     []interface{}{1},
		name:          "TestIntegerSkipForMinMaxChar",
		parameterType: utils.ParamTypeInteger,
		testOutput:    success}
	suite.testCaseList = append(suite.testCaseList, skipInteger)
	skipBoolean := &testCases{minChar: "14", maxChar: "10",
		testInput:     []interface{}{false},
		name:          "TestBooleanSkipForMInMaxChar",
		parameterType: utils.ParamTypeBoolean,
		testOutput:    success}
	suite.testCaseList = append(suite.testCaseList, skipBoolean)
}

func (suite *minMaxCharParamValidatorTestSuite) TestValidate_MultipleTestCases() {
	log := log.NewMockLog()
	for _, testCase := range suite.testCaseList {
		for _, input := range testCase.testInput {
			paramValue := input
			parameterConfig := contracts.Parameter{
				MinChars:  testCase.minChar,
				MaxChars:  testCase.maxChar,
				ParamType: string(testCase.parameterType),
			}
			err := suite.maxMinCharValidator.Validate(log, paramValue, &parameterConfig)
			if testCase.testOutput == success {
				assert.Nil(suite.T(), err, fmt.Sprintf("Testcase details: parameter %v, paramValue %v, testCaseName: %v", parameterConfig, paramValue, testCase.name))
			} else {
				assert.NotNil(suite.T(), err, fmt.Sprintf("Testcase details: parameter %v, paramValue %v, testCaseName: %v", parameterConfig, paramValue, testCase.name))
			}
		}
	}
}

func (suite *minMaxCharParamValidatorTestSuite) TestVerifyMinMaxCharAfterMarshall_Success_ValidInputs() {
	log := log.NewMockLog()
	input := make(map[string]map[string]struct{})
	input["test"] = make(map[string]struct{})
	err := suite.maxMinCharValidator.verifyMinMaxCharAfterMarshall(log, input, 2, 11)
	assert.Nil(suite.T(), err)
}

func (suite *minMaxCharParamValidatorTestSuite) TestVerifyMinMaxCharAfterMarshall_Failure_InValidInputs() {
	log := log.NewMockLog()
	input := make(map[string]map[string]struct{})
	input["test"] = make(map[string]struct{})
	err := suite.maxMinCharValidator.verifyMinMaxCharAfterMarshall(log, input, 2, 10)
	assert.NotNil(suite.T(), err)
}

func (suite *minMaxCharParamValidatorTestSuite) TestCheckMaxMin_Success_ValidInputs() {
	err := suite.maxMinCharValidator.verifyStringLen(1, 6, "123456")
	assert.Nil(suite.T(), err)
	err = suite.maxMinCharValidator.verifyStringLen(-1, 6, "123456")
	assert.Nil(suite.T(), err)
	err = suite.maxMinCharValidator.verifyStringLen(6, -1, "123456")
	assert.Nil(suite.T(), err)
}

func (suite *minMaxCharParamValidatorTestSuite) TestCheckMaxMin_Failed_InValidInputs() {
	err := suite.maxMinCharValidator.verifyStringLen(1, 5, "123456")
	assert.NotNil(suite.T(), err)
	err = suite.maxMinCharValidator.verifyStringLen(7, -1, "123456")
	assert.NotNil(suite.T(), err)
	err = suite.maxMinCharValidator.verifyStringLen(-1, 2, "123456")
	assert.NotNil(suite.T(), err)
}
