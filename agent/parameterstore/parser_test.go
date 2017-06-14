// Copyright 2016 Amazon.com, Inc. or its affiliates. All Rights Reserved.
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

// Package parameterstore contains modules to resolve ssm parameters present in the document.
package parameterstore

import (
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

type ReplaceSSMParamTestCase struct {
	Input  interface{}
	Params map[string]Parameter
	Output interface{}
}

func TestReplaceSSMParameters(t *testing.T) {
	testCases := generateReplaceSSMParamTestCases()
	for _, testCase := range testCases {
		temp, err := replaceSSMParameters(logger, testCase.Input, testCase.Params)
		assert.Equal(t, testCase.Output, temp)
		assert.Nil(t, err)
	}
}

func generateReplaceSSMParamTestCases() []ReplaceSSMParamTestCase {
	params := map[string]Parameter{
		"{{ssm:param1}}": {
			Value: "a parameter",
			Type:  "String",
			Name:  "param1",
		},
		"{{ssm:param2}}": {
			Value: "5",
			Type:  "String",
			Name:  "param2",
		},
		"{{ssm:param3.p1}}": {
			Value: "a parameter with dot",
			Type:  "String",
			Name:  "param3.p1",
		},
		"{{ssm:param4.p1}}": {
			Value: "5",
			Type:  "String",
			Name:  "param4.p1",
		},
	}

	stringListParams := map[string]Parameter{
		"{{ssm:param5}}": {
			Value: "a,b,c",
			Type:  "StringList",
			Name:  "param5",
		},
		"{{ssm:param6.p1}}": {
			Value: "'a,b',c",
			Type:  "StringList",
			Name:  "param6.p1",
		},
	}

	var testCases []ReplaceSSMParamTestCase

	// test cases for replacement in parameter-only strings
	for paramName, paramValue := range params {
		testCases = append(testCases, ReplaceSSMParamTestCase{
			Input:  fmt.Sprintf("%v", paramName),
			Params: params,
			Output: paramValue.Value,
		})
	}

	// test cases for StringList params
	for paramName, paramValue := range stringListParams {
		testCases = append(testCases, ReplaceSSMParamTestCase{
			Input:  []string{paramName},
			Params: stringListParams,
			Output: strings.Split(paramValue.Value, StringListDelimiter),
		})
	}

	// test cases for replacement in strings that have parameter inside other text
	for paramName, paramValue := range params {
		v := paramValue.Value
		testCases = append(testCases, ReplaceSSMParamTestCase{
			Input:  fmt.Sprintf("put %v here", paramName),
			Params: params,
			Output: fmt.Sprintf("put %v here", v),
		})
	}

	// test case for unexpected types
	unexp := struct{ Field string }{Field: "no {{ param1 }} replacement here"}
	testCases = append(testCases, ReplaceSSMParamTestCase{
		Input:  unexp,
		Params: params,
		Output: unexp,
	})

	return testCases
}

type ExtractSSMParamTestCase struct {
	Input  interface{}
	Output interface{}
}

func TestExtractSSMParamTestCase(t *testing.T) {
	testCases := generateExtractSSMParamTestCases()
	validSSMParam, _ := getValidSSMParamRegexCompiler(logger, defaultParamName)

	for _, testCase := range testCases {
		assert.Equal(t, testCase.Output, extractSSMParameters(logger, testCase.Input, validSSMParam))
	}
}

func generateExtractSSMParamTestCases() []ExtractSSMParamTestCase {
	params := map[string]Parameter{
		"{{ssm:param1}}": {
			Value: "a parameter",
			Type:  "String",
			Name:  "param1",
		},
		"{{ssm:param2}}": {
			Value: "5",
			Type:  "String",
			Name:  "param2",
		},
		"{{ssm:param3/p1}}": {
			Value: "a parameter with slash",
			Type:  "String",
			Name:  "param3/p1",
		}, "{{ssm:param4-p1}}": {
			Value: "a parameter with dash",
			Type:  "String",
			Name:  "ssm:param4-p1",
		},
		"{{ssm:param5.p1}}": {
			Value: "a parameter with dot",
			Type:  "String",
			Name:  "param5.p1",
		},
	}

	var testCases []ExtractSSMParamTestCase

	// test cases for replacement in parameter-only strings
	for paramName := range params {
		testCases = append(testCases, ExtractSSMParamTestCase{
			Input:  fmt.Sprintf("%v", paramName),
			Output: []string{paramName},
		})
	}

	// test cases for replacement in strings that have parameter inside other text
	for paramName := range params {
		testCases = append(testCases, ExtractSSMParamTestCase{
			Input:  fmt.Sprintf("put %v here", paramName),
			Output: []string{paramName},
		})
	}

	return testCases
}
