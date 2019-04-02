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

// Package parameters provides utilities to parse ssm document parameters
package parameters

import (
	"encoding/json"
	"fmt"
	"reflect"
	"regexp"

	"github.com/aws/amazon-ssm-agent/agent/log"
)

const paramNameRegex = "^[a-zA-Z0-9]+$"

// ReplaceParameters traverses an arbitrarily complex input object (maps/slices/strings/etc.)
// and tries to replace parameters given as {{parameter}} with their values from the parameters map.
//
// Strings like "{{ parameter }}" are replaced directly with the value associated with
// the parameter. That value need not be a string.
//
// Strings like "a {{ parameter1 }} within a string" are replaced with strings where the parameters
// are replaced by a marshaled version of their values. In this case, the resulting object is always a string.
//
// Note: this only works on composite types []interface{} and map[string]interface{} which are what json.Unmarshal
// produces by default. If your object contains []string, for example, the object will be returned as is.
//
// Returns a new object with replaced parameters.
func ReplaceParameters(input interface{}, parameters map[string]interface{}, logger log.T) interface{} {
	switch input := input.(type) {
	case string:
		// handle single parameter case first
		for parameterName, parameterValue := range parameters {
			if isSingleParameterString(input, parameterName) {
				return parameterValue
			}
		}

		// look for multiple parameter strings
		for parameterName, parameterValue := range parameters {
			var parameterValueString string
			var err error
			if parameterValueString, err = convertToString(parameterValue); err != nil {
				logger.Error(err)
			}

			input = ReplaceParameter(input, parameterName, parameterValueString)
		}
		return input

	case []interface{}:
		// for slices, recursively replace parameters on each element of the slice
		out := make([]interface{}, len(input))
		for i, v := range input {
			out[i] = ReplaceParameters(v, parameters, logger)
		}
		return out

	case []map[string]interface{}:
		// this case is not caught by the one above because map cannot be converted to interface{}
		out := make([]map[string]interface{}, len(input))
		for i, v := range input {
			out[i] = ReplaceParameters(v, parameters, logger).(map[string]interface{})
		}
		return out

	case map[string]interface{}:
		// for maps, recursively replace parameters on each value in the map
		out := make(map[string]interface{})
		for k, v := range input {
			out[k] = ReplaceParameters(v, parameters, logger)
		}
		return out

	case map[interface{}]interface{}:
		out := make(map[string]interface{})
		for k, v := range input {
			switch k := k.(type) {
			case string:
				out[k] = ReplaceParameters(v, parameters, logger)
			}
		}
		return out
	default:
		// any other type, return as is
		logger.Debugf("Type is - %v which was not found. Returning parameter without replacement", reflect.TypeOf(input))
		return input
	}
}

var singleParamRegex = regexp.MustCompile(paramNameRegex)

// isSingleParameterString returns true if the given string has the form "{{ paramName }}" with
// some spaces but nothing else.
func isSingleParameterString(input string, paramName string) bool {
	if singleParamRegex.MatchString(paramName) {
		// this method should be called only on parameter names that have been validated first
		r := regexp.MustCompile(fmt.Sprintf(`^{{\s*%v\s*}}$`, paramName))
		return r.MatchString(input)
	}
	return false
}

// ReplaceParameter replaces all occurrences of "{{ paramName }}" in the input by paramValue.
func ReplaceParameter(input string, paramName string, paramValue string) string {
	// this method should be called only on parameter names that have been validated first
	r := regexp.MustCompile(fmt.Sprintf(`{{\s*%v\s*}}`, paramName))
	return r.ReplaceAllString(input, paramValue)
}

// ValidParameters checks if parameter names are valid. Returns valid parameters only.
func ValidParameters(log log.T, params map[string]interface{}) map[string]interface{} {
	validParams := make(map[string]interface{})
	for paramName, paramValue := range params {
		if validName(paramName) {
			validParams[paramName] = paramValue
		} else {
			log.Errorf("invalid parameter name %v", paramName)
		}
	}
	return validParams
}

// validName checks whether the given parameter name is valid.
func validName(paramName string) bool {
	paramNameValidator := regexp.MustCompile(paramNameRegex)
	return paramNameValidator.MatchString(paramName)
}

// convertToString converts the input to a string form: if already a string,
// returns the same object, otherwise uses json.Marshal
func convertToString(input interface{}) (result string, err error) {
	switch input := input.(type) {
	case string:
		result = input
		return
	default:
		var resultBytes []byte
		resultBytes, err = json.Marshal(input)
		if err == nil {
			result = string(resultBytes)
			return
		}
		// in the unlikely event that we cannot Marshal return empty string
		// (not likely since this method is called on data unmarshalled from string!)
		err = fmt.Errorf("marshal object returned %v", err)
		return
	}
}
