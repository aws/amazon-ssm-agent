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
	"encoding/json"
	"fmt"
	"regexp"
	"runtime/debug"
	"strings"

	"github.com/aws/amazon-ssm-agent/agent/appconfig"
	"github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/aws/amazon-ssm-agent/agent/contracts"
	"github.com/aws/amazon-ssm-agent/agent/framework/docparser/paramvalidator"
	"github.com/aws/amazon-ssm-agent/agent/jsonutil"
	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/amazon-ssm-agent/agent/ssm"
)

const (
	// defaultParamName is used for creating default regex for parameter name
	defaultParamName = ""

	// ParamTypeString represents the Param Type is String
	ParamTypeString = "String"

	// ParamTypeSecureString represents the Param Type is SecureString
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
var resolve = Resolve

// Resolve resolves ssm parameters of the format {{ssm:*}}
func Resolve(context context.T, input interface{}) (interface{}, error) {
	log := context.Log()
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
	resolvedSSMParamMap, err := getSSMParameterValues(context, ssmParams)
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
	context context.T,
	documentParameters map[string]*contracts.Parameter,
	parameters map[string]interface{},
	invokedPlugin string) (err error) {
	log := context.Log()

	/*
		This function validates the following things before the document is sent for execution

		1. Document doesn't contain SecureString SSM Parameters
		2. SSM parameter values match the allowed pattern, allowed values, min/max items and min/max chars in the document
	*/
	var resolvedParameters interface{}
	resolvedParameters, err = resolve(context, parameters)
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

	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("panic during parameter validation: \n%v", r)
			log.Error(err)
			log.Errorf("stacktrace:\n%s", debug.Stack())
		}
	}()

	var validationErrors []string
	paramValidators := paramvalidator.GetMandatoryValidators()
	for paramName, paramObj := range documentParameters {
		for _, paramValidator := range paramValidators {
			if err = paramValidator.Validate(log, reformatResolvedParameters[paramName], paramObj); err != nil {
				mandatoryValidationErr := fmt.Errorf("error thrown in '%v' while validating parameter /%v/: %v", paramValidator.GetName(), paramName, err)
				validationErrors = append(validationErrors, mandatoryValidationErr.Error())
			}
		}
	}

	// currently, optional validators is applicable only for the inner document parameters
	// coming from the document invoked by runDocument plugin
	if invokedPlugin == appconfig.PluginRunDocument {
		paramValidators = paramvalidator.GetOptionalValidators()
		for paramName, paramObj := range documentParameters {
			// skip validations if the text contains SSM parameter store reference
			if val, ok := parameters[paramName]; ok {
				if isParameterResolvedFromSSMParameterStore(log, val) {
					log.Debugf("optional validators ignored for parameter %v", paramName)
					continue
				}
			}
			for _, paramValidator := range paramValidators {
				if err = paramValidator.Validate(log, reformatResolvedParameters[paramName], paramObj); err != nil {
					optionalValidationErr := fmt.Errorf("error thrown in '%v' while validating parameter /%v/: %v", paramValidator.GetName(), paramName, err)
					validationErrors = append(validationErrors, optionalValidationErr.Error())
				}
			}
		}
	}
	if len(validationErrors) > 0 {
		errorVal := fmt.Errorf("all errors during param validation errors: %v", strings.Join(validationErrors, "\n"))
		log.Error(errorVal)
		return errorVal
	}
	return nil
}

// isParameterResolvedFromSSMParameterStore checks whether the parameter is resolved from SSM parameter store
func isParameterResolvedFromSSMParameterStore(log log.T, input interface{}) bool {
	if byteString, err := json.Marshal(input); err == nil {
		if validSSMParam, err := getValidSSMParamRegexCompiler(log, defaultParamName); err == nil {
			if ssmParams := validSSMParam.Find(byteString); ssmParams != nil {
				return true
			}
		}
	}
	return false
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
func getSSMParameterValues(context context.T, ssmParams []string) (map[string]Parameter, error) {
	log := context.Log()
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

	if result, err = callParameterService(context, paramNames); err != nil {
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
func callGetParameters(context context.T, paramNames []string) (*GetParametersResponse, error) {
	log := context.Log()
	finalResult := GetParametersResponse{}

	ssmSvc := ssm.NewService(context)

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
