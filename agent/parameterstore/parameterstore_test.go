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

	"github.com/aws/amazon-ssm-agent/agent/contracts"
	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/stretchr/testify/assert"
)

type StringTestCase struct {
	Input             string
	Output            string
	Parameters        []Parameter
	InvalidParameters []string
}

var StringTestCases = []StringTestCase{
	{
		Input:             "This is a test string",
		Output:            "This is a test string",
		Parameters:        []Parameter{},
		InvalidParameters: []string{},
	},
	{
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
	StringTestCase{
		Input:  "This is a {{ssm:test.p1}} with dot string",
		Output: "This is a testvalueWithDot with dot string",
		Parameters: []Parameter{
			{
				Name:  "test.p1",
				Type:  "String",
				Value: "testvalueWithDot",
			},
		},
		InvalidParameters: []string{},
	},
	StringTestCase{
		Input:  "This is a {{ssm:test-p1}} with dash string",
		Output: "This is a testValueWithDash with dash string",
		Parameters: []Parameter{
			{
				Name:  "test-p1",
				Type:  "String",
				Value: "testValueWithDash",
			},
		},
		InvalidParameters: []string{},
	},
	StringTestCase{
		Input:  "This is a {{ssm:test/p1}} with slash string",
		Output: "This is a testValueWithSlash with slash string",
		Parameters: []Parameter{
			{
				Name:  "test/p1",
				Type:  "String",
				Value: "testValueWithSlash",
			},
		},
		InvalidParameters: []string{},
	}, StringTestCase{
		Input:  "This is a {{ssm:test/p5}} that does not exist",
		Output: "This is a {{ssm:test/p5}} that does not exist",
		Parameters: []Parameter{
			{
				Name:  "testp1",
				Type:  "String",
				Value: "testValueWithSlash",
			},
		},
		InvalidParameters: []string{},
	},
}

var logger = log.NewMockLog()

func TestResolve(t *testing.T) {
	for _, testCase := range StringTestCases {
		testResolveMethod(t, testCase)
	}
}

func testResolveMethod(t *testing.T, testCase StringTestCase) {
	callParameterService = func(
		log log.T,
		paramNames []string) (*GetParametersResponse, error) {
		result := GetParametersResponse{}
		result.Parameters = testCase.Parameters
		result.InvalidParameters = testCase.InvalidParameters
		return &result, nil
	}

	result, err := Resolve(logger, testCase.Input)

	assert.Equal(t, testCase.Output, result)
	assert.Nil(t, err)
}

func TestValidateSSMParameters(t *testing.T) {

	// Test case 1 with no SSM parameters
	documentParameters := map[string]*contracts.Parameter{
		"commands": {
			AllowedPattern: "^[a-zA-Z0-9]+$",
		},
		"workingDirectory": {
			AllowedPattern: "",
		},
	}

	parameters := map[string]interface{}{
		"commands":         "test",
		"workingDirectory": "",
	}

	err := ValidateSSMParameters(logger, documentParameters, parameters)
	assert.Nil(t, err)

	// Test case 2 with SSM parameters and secure string SSM parameter type
	documentParameters = map[string]*contracts.Parameter{
		"commands": {
			AllowedPattern: "^[a-zA-Z0-9]+$",
			ParamType:      ParamTypeString,
		},
		"workingDirectory": {
			AllowedPattern: "",
		},
	}

	parameters = map[string]interface{}{
		"commands":         "{{ssm:test}}",
		"workingDirectory": "{{ssm:test2}}",
	}

	ssmParameters := []Parameter{
		{
			Name:  "test",
			Type:  ParamTypeSecureString,
			Value: "test",
		},
		{
			Name:  "test2",
			Type:  ParamTypeSecureString,
			Value: "test2",
		},
	}

	invalidSSMParameters := []string{}

	callParameterService = func(
		log log.T,
		paramNames []string) (*GetParametersResponse, error) {
		result := GetParametersResponse{}
		result.Parameters = ssmParameters
		result.InvalidParameters = invalidSSMParameters
		return &result, nil
	}

	err = ValidateSSMParameters(logger, documentParameters, parameters)
	assert.Equal(t, "Parameters [test test2] of type SecureString are not supported", err.Error())

	// Test case 3 with SSM parameters and SSM parameter value doesn't match allowed pattern
	documentParameters = map[string]*contracts.Parameter{
		"commands": {
			AllowedPattern: "^[a-zA-Z]+$",
			ParamType:      ParamTypeString,
		},
		"workingDirectory": {
			AllowedPattern: "",
		},
	}

	parameters = map[string]interface{}{
		"commands":         "{{ssm:test}}",
		"workingDirectory": "",
	}

	ssmParameters = []Parameter{
		{
			Name:  "test",
			Type:  ParamTypeString,
			Value: "1234",
		},
	}

	callParameterService = func(
		log log.T,
		paramNames []string) (*GetParametersResponse, error) {
		result := GetParametersResponse{}
		result.Parameters = ssmParameters
		result.InvalidParameters = invalidSSMParameters
		return &result, nil
	}

	err = ValidateSSMParameters(logger, documentParameters, parameters)
	assert.Equal(t, "Parameter value for commands does not match the allowed pattern ^[a-zA-Z]+$", err.Error())

}
