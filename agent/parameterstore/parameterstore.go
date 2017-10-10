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

	// ParamTypeString represents the Param Type is String
	ParamTypeString = "String"

	// ParamTypeString represents the Param Type is SecureString
	ParamTypeSecureString = "SecureString"

	// ParamTypeStringList represents the Param Type is StringList
	ParamTypeStringList = "StringList"

	// ErrorMsg represents the error message to be sent to the customer
	ErrorMsg = "Encountered error while parsing input - internal error"

	// MaxParametersPerCall represents the max number of parameters you can send in one GetParameters call
	MaxParametersPerCall = 10

	// Delimiter used for splitting StringList type SSM parameters.
	StringListDelimiter = ","
)

var callParameterService = callGetParameters

// Resolve resolves ssm parameters of the format {{ssm:*}}
func Resolve(log log.T, input interface{}) (interface{}, error) {
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
	resolvedSSMParamMap, err := getSSMParameterValues(log, ssmParams)
	if err != nil {
		return input, err
	}

	// Replace ssm parameter names with their values
	input, err = replaceSSMParameters(log, input, resolvedSSMParamMap)
	if err != nil {
		return input, err
	}

	// Return resolved input
	return input, nil
}

// ValidateSSMParameters validates SSM parameters
func ValidateSSMParameters(
	log log.T,
	documentParameters map[string]*contracts.Parameter,
	parameters map[string]interface{}) error {

	/*
		This function validates the following things before the document is sent for execution

		1. Document doesn't contain SecureString SSM Parameters
		2. SSM parameter values match the allowed pattern in the document
	*/

	resolvedParameters, err := Resolve(log, parameters)
	if err != nil {
		return err
	}

	// Reformat resolvedParameters to type map[string]interface{}
	var reformatResolvedParameters map[string]interface{}
	err = jsonutil.Remarshal(resolvedParameters, &reformatResolvedParameters)
	if err != nil {
		log.Debug(err)
		return fmt.Errorf("%v", ErrorMsg)
	}

	for paramName, paramObj := range documentParameters {
		// Check SSM parameter values match the allowed pattern in the document
		if paramObj.AllowedPattern != "" {
			validParamValue, err := regexp.Compile(paramObj.AllowedPattern)
			if err != nil {
				log.Debug(err)
				return fmt.Errorf("%v", ErrorMsg)
			}

			errorString := fmt.Errorf("Parameter value for %v does not match the allowed pattern %v", paramName, paramObj.AllowedPattern)
			switch input := reformatResolvedParameters[paramName].(type) {
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

// getValidSSMParamRegexCompiler returns a regex compiler
func getValidSSMParamRegexCompiler(log log.T, paramName string) (*regexp.Regexp, error) {
	var validSSMParamRegex string
	if strings.Compare(paramName, defaultParamName) == 0 {
		validSSMParamRegex = "\\{\\{ *ssm:[/\\w.:-]+ *\\}\\}"
	} else {
		//[BUG FIX] escape . in the paramName
		validSSMParamRegex = "\\{\\{ *ssm:" + strings.Replace(paramName, ".", "\\.", -1) + " *\\}\\}"
	}

	validSSMParam, err := regexp.Compile(validSSMParamRegex)
	if err != nil {
		log.Debug(err)
		return nil, fmt.Errorf("%v", ErrorMsg)
	}

	return validSSMParam, nil
}

// getSSMParameterValues takes a list of strings and resolves them by calling the GetParameters API
func getSSMParameterValues(log log.T, ssmParams []string) (map[string]Parameter, error) {
	var result *GetParametersResponse
	var err error

	validParamRegex := ":([/\\w.:-]+)*"
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
		errorString := fmt.Errorf("Input contains invalid parameters %v", result.InvalidParameters)
		log.Debug(errorString)
		return nil, errorString
	}

	resolvedParamMap := map[string]Parameter{}
	secureStringParams := []string{}
	seen = map[string]bool{}
	for _, paramObj := range result.Parameters {
		// Populate all the secure string parameters used in the document
		if paramObj.Type == ParamTypeSecureString {
			secureStringParams = append(secureStringParams, paramObj.Name)
		}

		// get regex compiler
		// Let's try to get an exact match for the ssm parameter with version
		validSSMParam, err := getValidSSMParamRegexCompiler(log, fmt.Sprintf("%v:%d", paramObj.Name, paramObj.Version))
		if err != nil {
			return nil, err
		}

		found := false
		for _, value := range ssmParams {
			if !seen[value] && validSSMParam.MatchString(value) {
				resolvedParamMap[value] = paramObj
				found = true
				seen[value] = true
			}
		}

		// If not found, lets try to search without the version
		// This is to support the existing use case of referring parameters without versions.
		//
		// Example:
		// SSM Parameter references of type {{ssm:test}} which do not have any versions associated with it
		// will not match with regex compiler of type 'ssm:test:4' which we do in the previous step.
		// So, in order to resolve parameters without versions we perform another step of regex match, but
		// with parameter name only. Once, we find a match, we make sure that the parameter referred
		// in the document doesn't have any version associated with it before replacing it.
		if !found {
			// Recompile the regex without the version for a match
			validSSMParam, err = getValidSSMParamRegexCompiler(log, paramObj.Name)
			if err != nil {
				return nil, err
			}

			for _, value := range ssmParams {
				if !seen[value] && validSSMParam.MatchString(value) {
					// Need to make sure that the ssm parameter referenced in the document doesn't have a version
					if strings.Count(value, ":") == 1 {
						resolvedParamMap[value] = paramObj
						seen[value] = true
						found = true
					}
				}
			}
		}

		if !found {
			return nil, fmt.Errorf("%v", ErrorMsg)
		}
	}

	if len(secureStringParams) > 0 {
		return nil, fmt.Errorf("Parameters %v of type %v are not supported", secureStringParams, ParamTypeSecureString)
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
			return nil, fmt.Errorf("%v", ErrorMsg)
		}

		finalResult.Parameters = append(finalResult.Parameters, response.Parameters...)
		finalResult.InvalidParameters = append(finalResult.InvalidParameters, response.InvalidParameters...)
	}

	return &finalResult, nil
}
