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
	"regexp"
	"strings"

	"github.com/aws/amazon-ssm-agent/agent/log"
)

// extractSSMParameters extracts parameters of the format {{ssm:*}} from input
func extractSSMParameters(log log.T, input interface{}, validSSMParam *regexp.Regexp) []string {
	switch input := input.(type) {
	case string:
		// find all matching strings
		ssmParams := validSSMParam.FindAllString(input, -1)
		return ssmParams

	case []string:
		ssmParams := []string{}
		// for slices, recursively replace parameters on each element of the slice
		for _, v := range input {
			ssmParams = append(ssmParams, extractSSMParameters(log, v, validSSMParam)...)
		}
		return ssmParams

	case []interface{}:
		ssmParams := []string{}
		// for slices, recursively replace parameters on each element of the slice
		for _, v := range input {
			ssmParams = append(ssmParams, extractSSMParameters(log, v, validSSMParam)...)
		}
		return ssmParams

	case []map[string]interface{}:
		ssmParams := []string{}
		// this case is not caught by the one above because map cannot be converted to interface{}
		for _, v := range input {
			ssmParams = append(ssmParams, extractSSMParameters(log, v, validSSMParam)...)
		}
		return ssmParams

	case map[string]interface{}:
		ssmParams := []string{}
		// for maps, recursively replace parameters on each value in the map
		for _, v := range input {
			ssmParams = append(ssmParams, extractSSMParameters(log, v, validSSMParam)...)
		}
		return ssmParams

	default:
		// return empty string array
		return []string{}
	}
}

// replaceSSMParameters replaces parameters of the format {{ssm:*}} with their actual values
func replaceSSMParameters(log log.T, input interface{}, ssmParameters map[string]Parameter) interface{} {
	switch input := input.(type) {
	case string:
		// replace param names with actual values
		for paramName, paramObj := range ssmParameters {
			input = strings.Replace(input, paramName, paramObj.Value, -1)
		}
		return input

	case []string:
		out := []string{}
		for _, value := range input {
			temp := value
			found := false
			for paramName, paramObj := range ssmParameters {
				if strings.Compare(paramObj.Type, ParamTypeStringList) == 0 &&
					strings.Compare(paramName, strings.TrimSpace(temp)) == 0 {
					out = append(out, strings.Split(paramObj.Value, ",")...)
					found = true
					break
				}
				temp = strings.Replace(temp, paramName, paramObj.Value, -1)
			}

			// If value not found then add the string as it is
			if !found {
				out = append(out, temp)
			}
		}
		return out

	case []interface{}:
		switch input[0].(type) {
		case string:
			out := []string{}
			for _, value := range input {
				temp := value.(string)
				found := false
				for paramName, paramObj := range ssmParameters {
					if strings.Compare(paramObj.Type, ParamTypeStringList) == 0 &&
						strings.Compare(paramName, strings.TrimSpace(temp)) == 0 {
						out = append(out, strings.Split(paramObj.Value, ",")...)
						found = true
						break
					}
					temp = strings.Replace(temp, paramName, paramObj.Value, -1)
				}

				// If value not found then add the string as it is
				if !found {
					out = append(out, temp)
				}
			}
			return out

		default:
			// for slices, recursively replace parameters on each element of the slice
			out := make([]interface{}, len(input))
			for i, v := range input {
				out[i] = replaceSSMParameters(log, v, ssmParameters)
			}
			return out
		}

	case []map[string]interface{}:
		// this case is not caught by the one above because map cannot be converted to interface{}
		out := make([]map[string]interface{}, len(input))
		for i, v := range input {
			out[i] = replaceSSMParameters(log, v, ssmParameters).(map[string]interface{})
		}
		return out

	case map[string]interface{}:
		// for maps, recursively replace parameters on each value in the map
		out := make(map[string]interface{})
		for k, v := range input {
			out[k] = replaceSSMParameters(log, v, ssmParameters)
		}
		return out

	default:
		// any other type, return as is
		return input
	}
}
