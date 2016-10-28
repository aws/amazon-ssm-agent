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
	"testing"

	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/stretchr/testify/assert"
)

type StringTestCase struct {
	Input             string
	Output            string
	Parameters        []Parameter
	InvalidParameters []string
}

type StringListTestCase struct {
	Input             []string
	Output            []string
	Parameters        []Parameter
	InvalidParameters []string
}

var StringTestCases = []StringTestCase{
	StringTestCase{
		Input:             "This is a test string",
		Output:            "This is a test string",
		Parameters:        []Parameter{},
		InvalidParameters: []string{},
	},
	StringTestCase{
		Input:  "This is a {{ssm:test}} string",
		Output: "This is a testvalue string",
		Parameters: []Parameter{
			{
				Name:  "test",
				Type:  "String",
				Value: "testvalue",
			},
		},
		InvalidParameters: []string{},
	},
}

var StringListTestCases = []StringListTestCase{
	StringListTestCase{
		Input:             []string{"This is a test string", "Another test string"},
		Output:            []string{"This is a test string", "Another test string"},
		Parameters:        []Parameter{},
		InvalidParameters: []string{},
	},
	StringListTestCase{
		Input:  []string{"This is a {{ssm:test}} string", "Another parameter {{ ssm:foo }}"},
		Output: []string{"This is a testvalue string", "Another parameter randomvalue"},
		Parameters: []Parameter{
			{
				Name:  "test",
				Type:  "String",
				Value: "testvalue",
			},
			{
				Name:  "foo",
				Type:  "String",
				Value: "randomvalue",
			},
		},
		InvalidParameters: []string{},
	},
}

var logger = log.NewMockLog()

func TestResolveString(t *testing.T) {
	testString(t, StringTestCases[0])
	testString(t, StringTestCases[1])
}

func testString(t *testing.T, testCase StringTestCase) {
	callParameterService = func(
		log log.T,
		paramNames []string) (*GetParametersResponse, error) {
		result := GetParametersResponse{}
		result.Parameters = testCase.Parameters
		result.InvalidParameters = testCase.InvalidParameters
		return &result, nil
	}

	result, err := ResolveString(logger, testCase.Input)

	assert.Equal(t, testCase.Output, result)
	assert.Nil(t, err)
}

func TestResolveStringList(t *testing.T) {
	testStringList(t, StringListTestCases[0])
	testStringList(t, StringListTestCases[1])
}

func testStringList(t *testing.T, testCase StringListTestCase) {
	callParameterService = func(
		log log.T,
		paramNames []string) (*GetParametersResponse, error) {
		result := GetParametersResponse{}
		result.Parameters = testCase.Parameters
		result.InvalidParameters = testCase.InvalidParameters
		return &result, nil
	}

	result, err := ResolveStringList(logger, testCase.Input)

	assert.Equal(t, testCase.Output, result)
	assert.Nil(t, err)
}
