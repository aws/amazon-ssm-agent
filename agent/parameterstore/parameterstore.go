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
	"regexp"
	"strings"

	"github.com/aws/amazon-ssm-agent/agent/contracts"
	"github.com/aws/amazon-ssm-agent/agent/jsonutil"
	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/amazon-ssm-agent/agent/ssm"
)

const (
	// defaultParamName is used for creating default regex for parameter name
	defaultParamName = ""

	// ParamTypeString represents the Param Type is SecureString
	ParamTypeSecureString = "SecureString"

	// ParamTypeStringList represents the Param Type is StringList
	ParamTypeStringList = "StringList"

	// ErrorMsg represents the error message to be sent to the customer
	ErrorMsg = "Encountered error while parsing input - internal error"

	// MaxParametersPerCall represents the max number of parameters you can send in one GetParameters call
	MaxParametersPerCall = 10
)

var callParameterService = callGetParameters

// Resolve resolves ssm parameters of the format {{ssm:*}}
func Resolve(log log.T, input interface{}, resolveSecureString bool) (interface{}, error) {
	validSSMParam, err := getValidSSMParamRegexCompiler(log, defaultParamName)
	if err != nil {
		return input, err
	}

	// Extract all SSM parameters from input
	ssmParams := extractSSMParameters(log, input, validSSMParam)

	// Return original string if no ssm params found
	if len(ssmParams) == 0 {
		return input, nil
	}

	// Get ssm parameter values
	resolvedParamMap, err := getSSMParameterValues(log, ssmParams, resolveSecureString)
	if err != nil {
		return input, err
	}

	// Replace ssm parameter names with their values
	input, err = replaceSSMParameters(log, input, resolvedParamMap)
	if err != nil {
		return input, err
	}

	// Return resolved input
	return input, nil
}

// ValidateSSMParameters validates whether the parameter value matches the allowed pattern
func ValidateSSMParameters(
	log log.T,
	documentParameters map[string]*contracts.Parameter,
	parameters map[string]interface{}) error {

	log.Debug("Validating SSM parameters")

	resolvedParameters, err := Resolve(log, parameters, true)
	if err != nil {
		return err
	}

	var resolvedParamMap map[string]interface{}
	err = jsonutil.Remarshal(resolvedParameters, &resolvedParamMap)
	if err != nil {
		log.Debug(err)
		return fmt.Errorf("%v", ErrorMsg)
	}

	for paramName, paramObj := range documentParameters {
		if paramObj.AllowedPattern != "" {
			validParamValue, err := regexp.Compile(paramObj.AllowedPattern)
			if err != nil {
				log.Debug(err)
				return fmt.Errorf("%v", ErrorMsg)
			}

			errorString := fmt.Errorf("Parameter value for %v does not match the allowed pattern %v", paramName, paramObj.AllowedPattern)
			switch input := resolvedParamMap[paramName].(type) {
			case string:
				if !validParamValue.MatchString(input) {
					return errorString
				}

			case []string:
				for _, v := range input {
					if !validParamValue.MatchString(v) {
						return errorString
					}
				}

			case []interface{}:
				for _, v := range input {
					if !validParamValue.MatchString(v.(string)) {
						return errorString
					}
				}

			default:
				return fmt.Errorf("Unable to determine parameter value type for %v", paramName)
			}
		}
	}
	return nil
}

// ResolveSecureString resolves the ssm parameters if present in input string
func ResolveSecureString(log log.T, input string) (string, error) {
	output, err := Resolve(log, input, true)
	if err != nil {
		return input, err
	}

	var reformatOutput string
	err = jsonutil.Remarshal(output, &reformatOutput)
	if err != nil {
		log.Debug(err)
		return input, fmt.Errorf("%v", ErrorMsg)
	}

	return reformatOutput, nil
}

// ResolveSecureStringForStringList resolves the ssm parameters if present in input stringList
func ResolveSecureStringForStringList(log log.T, input []string) ([]string, error) {
	output, err := Resolve(log, input, true)
	if err != nil {
		return input, err
	}

	var reformatOutput []string
	err = jsonutil.Remarshal(output, &reformatOutput)
	if err != nil {
		log.Debug(err)
		return input, fmt.Errorf("%v", ErrorMsg)
	}

	return reformatOutput, nil
}

// getValidSSMParamRegexCompiler returns a regex compiler
func getValidSSMParamRegexCompiler(log log.T, paramName string) (*regexp.Regexp, error) {
	var validSSMParamRegex string

	if strings.Compare(paramName, defaultParamName) == 0 {
		validSSMParamRegex = "\\{\\{ *ssm:([/\\w]+) *}}"
	} else {
		validSSMParamRegex = "\\{\\{ *ssm:" + paramName + " *}}"
	}

	validSSMParam, err := regexp.Compile(validSSMParamRegex)
	if err != nil {
		log.Debug(err)
		return nil, fmt.Errorf("%v", ErrorMsg)
	}
	return validSSMParam, nil
}

// getSSMParameterValues takes a list of strings and resolves them by calling the GetParameters API
func getSSMParameterValues(log log.T, ssmParams []string, resolveSecureString bool) (map[string]Parameter, error) {
	var result *GetParametersResponse
	var err error

	log.Info("Resolving SSM parameters")

	validParamRegex := ":([/\\w]+)*"
	validParam, err := regexp.Compile(validParamRegex)
	if err != nil {
		log.Debug(err)
		return nil, fmt.Errorf("%v", ErrorMsg)
	}

	// Remove duplicates
	paramNames := []string{}
	seen := map[string]bool{}
	for _, value := range ssmParams {
		temp := validParam.FindString(value)
		temp = temp[1:]
		if !seen[temp] {
			seen[temp] = true
			paramNames = append(paramNames, temp)
		}
	}

	if result, err = callParameterService(log, paramNames); err != nil {
		return nil, err
	}

	if len(paramNames) != len(result.Parameters) {
		errorString := fmt.Errorf("Input contains invalid ssm parameters %v", result.InvalidParameters)
		log.Debug(errorString)
		return nil, errorString
	}

	resolvedParamMap := map[string]Parameter{}
	for _, paramObj := range result.Parameters {
		// Skip secure parameters
		if !resolveSecureString && strings.Compare(paramObj.Type, ParamTypeSecureString) == 0 {
			continue
		}

		// get regex compiler
		validSSMParam, err := getValidSSMParamRegexCompiler(log, paramObj.Name)
		if err != nil {
			return nil, err
		}

		for _, value := range ssmParams {
			if validSSMParam.MatchString(value) {
				resolvedParamMap[value] = paramObj
			}
		}
	}

	return resolvedParamMap, nil
}

// callGetParameters makes a GetParameters API call to the service
func callGetParameters(log log.T, paramNames []string) (*GetParametersResponse, error) {
	finalResult := GetParametersResponse{}

	ssmSvc := ssm.NewService()

	for i := 0; i < len(paramNames); i = i + MaxParametersPerCall {
		limit := i + MaxParametersPerCall
		if limit > len(paramNames) {
			limit = len(paramNames)
		}

		result, err := ssmSvc.GetParameters(log, paramNames[i:limit])
		if err != nil {
			return nil, err
		}

		var response GetParametersResponse
		err = jsonutil.Remarshal(result, &response)
		if err != nil {
			log.Debug(err)
			errorString := "Encountered error while parsing GetParameters output"
			return nil, fmt.Errorf("%v", errorString)
		}

		finalResult.Parameters = append(finalResult.Parameters, response.Parameters...)
		finalResult.InvalidParameters = append(finalResult.InvalidParameters, response.InvalidParameters...)
	}

	return &finalResult, nil
}
