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

// Package minmaxitemparamvalidator is responsible for validating parameter value
// with the min max item restriction given in the document for parameters.
package minmaxitemparamvalidator

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

type testStatus string

const (
	success testStatus = "Success"
	failure testStatus = "Failure"
)

type testCases struct {
	minItem       json.Number
	maxItem       json.Number
	testInput     []interface{}
	testOutput    testStatus
	name          string
	parameterType utils.DocumentParamType
}

type minMaxItemParamValidatorTestSuite struct {
	suite.Suite
	testCaseList        []*testCases
	log                 log.T
	maxMinItemValidator *minMaxItemParamValidator
}

// TestMinMaxItemParamValidatorTestSuite executes the test suite containing tests related to min/max item param validators
func TestMinMaxItemParamValidatorTestSuite(t *testing.T) {
	minMaxItemSuite := new(minMaxItemParamValidatorTestSuite)
	minMaxItemSuite.maxMinItemValidator = GetMinMaxItemValidator()
	suite.Run(t, minMaxItemSuite)
}

func (suite *minMaxItemParamValidatorTestSuite) SetupTest() {
	suite.log = log.NewMockLog()
	suite.testCaseList = make([]*testCases, 0)

	testInputMapOneVal := make(map[string]interface{})
	testInputMapOneVal["test1"] = "testValue1"
	testInputMapTwoVal := make(map[string]interface{})
	testInputMapTwoVal["test1"] = "testValue1"
	testInputMapTwoVal["test2"] = "testValue2"

	minItemGreaterThanZeroMaxNilCase := &testCases{minItem: "2", maxItem: "",
		testInput: []interface{}{
			[]string{"testValue1", "testValue2"},
			[]interface{}{"testValue1", "testValue2", "testValue3"},
			[]interface{}{
				map[string]interface{}{"testKey1": "testValue1"},
				map[string]interface{}{"testKey2": "testValue2"},
				map[string]interface{}{"testKey2": "testValue3"},
			},
		},
		name:       "TestMinItemGreaterThanZeroMaxItemNil",
		testOutput: success}
	suite.testCaseList = append(suite.testCaseList, minItemGreaterThanZeroMaxNilCase)
	minItemGreaterThanZeroMaxNilForMapCase := &testCases{minItem: "2", maxItem: "",
		testInput:  []interface{}{testInputMapTwoVal},
		name:       "TestMinItemGreaterThanZeroMaxNilForMap",
		testOutput: success}
	suite.testCaseList = append(suite.testCaseList, minItemGreaterThanZeroMaxNilForMapCase)
	minItemLessThanZeroMaxItemNilCase := &testCases{minItem: "-2", maxItem: "",
		testInput: []interface{}{
			[]string{"testValue1", "testValue2"},
			[]interface{}{"testValue1", "testValue2"},
			[]interface{}{
				map[string]interface{}{"testKey1": "testValue1"}, map[string]interface{}{"testKey2": "testValue2"}},
			testInputMapTwoVal},
		name:       "TestMinItemLessThanZeroMaxItemNil",
		testOutput: success}
	suite.testCaseList = append(suite.testCaseList, minItemLessThanZeroMaxItemNilCase)
	minItemNilMaxItemNilCase := &testCases{minItem: "", maxItem: "",
		testInput: []interface{}{
			[]string{"testValue1", "testValue2"},
			[]interface{}{"testValue1", "testValue2"},
			testInputMapTwoVal},
		name:       "TestMinItemNilMaxItemNil",
		testOutput: success}
	suite.testCaseList = append(suite.testCaseList, minItemNilMaxItemNilCase)
	minMaxLessThanZeroCase := &testCases{minItem: "-2", maxItem: "-2",
		testInput: []interface{}{
			[]string{"testValue1", "testValue2"},
			[]interface{}{"testValue1", "testValue2"},
			testInputMapTwoVal},
		name:       "TestMinMaxItemLessThanZero",
		testOutput: success}
	suite.testCaseList = append(suite.testCaseList, minMaxLessThanZeroCase)
	minLessThanZeroMaxGreaterThanIntCase := &testCases{minItem: "-2", maxItem: "2147483650",
		testInput: []interface{}{
			[]string{"testValue1", "testValue2"},
			[]interface{}{"testValue1", "testValue2"},
			testInputMapTwoVal,
			[]interface{}{map[string]interface{}{"testKey1": "testValue1"}, map[string]interface{}{"testKey2": "testValue2"}},
		},
		name:       "TestMinLessThanZeroMaxGreaterThanInt",
		testOutput: success}
	suite.testCaseList = append(suite.testCaseList, minLessThanZeroMaxGreaterThanIntCase)
	minItemLessThanLimitMaxItemNilCase := &testCases{minItem: "5", maxItem: "",
		testInput: []interface{}{
			[]string{"123", "testValue2"},
			[]interface{}{"testValue1", "123"}},
		name:       "TestMinLessThanLimitMaxNil",
		testOutput: failure}
	suite.testCaseList = append(suite.testCaseList, minItemLessThanLimitMaxItemNilCase)
	minItemLessThanLimitMaxItemNilCaseForMapInput := &testCases{minItem: "2", maxItem: "",
		testInput:  []interface{}{testInputMapOneVal},
		name:       "TestMinItemLessThanLimitMaxItemNilForMap",
		testOutput: failure}
	suite.testCaseList = append(suite.testCaseList, minItemLessThanLimitMaxItemNilCaseForMapInput)
	minEmptyMaxGreaterThanLimit := &testCases{minItem: "", maxItem: "1",
		testInput: []interface{}{
			[]string{"testValue1", "testValue2"},
			[]interface{}{"testValue1", "testValue2"},
			[]interface{}{map[string]interface{}{"testKey1": "testValue1"}, map[string]interface{}{"testKey2": "testValue2"}}},
		name:       "TestMinEmptyMaxGreaterThanLimit",
		testOutput: failure}
	suite.testCaseList = append(suite.testCaseList, minEmptyMaxGreaterThanLimit)
	minItemEmptyMaxItemGreaterThanLimitForMapInput := &testCases{minItem: "", maxItem: "1",
		testInput:  []interface{}{testInputMapTwoVal},
		name:       "TestMinItemEmptyMaxItemGreaterThanLimitForMapInput",
		testOutput: failure}
	suite.testCaseList = append(suite.testCaseList, minItemEmptyMaxItemGreaterThanLimitForMapInput)
	minItemLessThanZeroMaxGreaterThanLimit := &testCases{minItem: "-2", maxItem: "1",
		testInput: []interface{}{
			[]string{"testValue1", "testValue2"},
			[]interface{}{"testValue1", "testValue2"}},
		name:       "TestMinItemLessThanZeroMaxGreaterThanLimit",
		testOutput: failure}
	suite.testCaseList = append(suite.testCaseList, minItemLessThanZeroMaxGreaterThanLimit)
	minItemGreaterThanMax := &testCases{minItem: "14", maxItem: "10",
		testInput: []interface{}{
			[]string{"testValue1", "testValue2"},
			[]interface{}{"testValue1", "testValue2"}},
		name:       "TestMinItemGreaterThanMax",
		testOutput: failure}
	suite.testCaseList = append(suite.testCaseList, minItemGreaterThanMax)
	skipStringMapFromConsole := &testCases{minItem: "14", maxItem: "10",
		testInput:     []interface{}{"{\"x\":\"y\"}"},
		name:          "TestSkipStringMapFromConsole",
		parameterType: utils.ParamTypeStringMap,
		testOutput:    success}
	suite.testCaseList = append(suite.testCaseList, skipStringMapFromConsole)
	skipInteger := &testCases{minItem: "14", maxItem: "10",
		testInput:     []interface{}{1},
		name:          "TestIntegerTypeSkipForMinMaxItem",
		parameterType: utils.ParamTypeInteger,
		testOutput:    success}
	suite.testCaseList = append(suite.testCaseList, skipInteger)
	skipString := &testCases{minItem: "14", maxItem: "10",
		testInput:     []interface{}{"sample"},
		name:          "TestStringTypeSkipForMinMaxItem",
		parameterType: utils.ParamTypeString,
		testOutput:    success}
	suite.testCaseList = append(suite.testCaseList, skipString)
	skipBoolean := &testCases{minItem: "14", maxItem: "10",
		testInput:     []interface{}{false},
		name:          "TestBooleanTypeSkipForMinMaxItem",
		parameterType: utils.ParamTypeBoolean,
		testOutput:    success}
	suite.testCaseList = append(suite.testCaseList, skipBoolean)
}

func (suite *minMaxItemParamValidatorTestSuite) TestValidate_MultipleTestCases() {
	log := log.NewMockLog()
	for _, testCase := range suite.testCaseList {
		for _, input := range testCase.testInput {
			paramValue := input
			parameterConfig := contracts.Parameter{
				MinItems:  testCase.minItem,
				MaxItems:  testCase.maxItem,
				ParamType: string(testCase.parameterType),
			}
			err := suite.maxMinItemValidator.Validate(log, paramValue, &parameterConfig)
			if testCase.testOutput == success {
				assert.Nil(suite.T(), err, fmt.Sprintf("Testcase details: parameter %v, paramValue %v, testCaseName: %v", parameterConfig, paramValue, testCase.name))
			} else {
				assert.NotNil(suite.T(), err, fmt.Sprintf("Testcase details: parameter %v, paramValue %v, testCaseName: %v", parameterConfig, paramValue, testCase.name))
			}
		}
	}
}

func (suite *minMaxItemParamValidatorTestSuite) TestCheckMaxMinItem_Success_ValidInputs() {
	// between min and max limit
	// choosing random valid values
	err := suite.maxMinItemValidator.verifyItemCount(1, 6, 2)
	assert.Nil(suite.T(), err)
	err = suite.maxMinItemValidator.verifyItemCount(-1, 6, 5)
	assert.Nil(suite.T(), err)
	err = suite.maxMinItemValidator.verifyItemCount(6, -1, 14)
	assert.Nil(suite.T(), err)
}

func (suite *minMaxItemParamValidatorTestSuite) TestCheckMaxMinItem_Failed_InValidInputs() {
	// outside min and max limit
	// choosing random invalid values
	err := suite.maxMinItemValidator.verifyItemCount(1, 5, 7)
	assert.NotNil(suite.T(), err)
	err = suite.maxMinItemValidator.verifyItemCount(7, -1, 5)
	assert.NotNil(suite.T(), err)
	err = suite.maxMinItemValidator.verifyItemCount(-1, 2, 5)
	assert.NotNil(suite.T(), err)
}
