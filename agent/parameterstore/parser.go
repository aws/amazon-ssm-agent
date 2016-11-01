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

	"github.com/aws/amazon-ssm-agent/agent/jsonutil"
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
func replaceSSMParameters(log log.T, input interface{}, ssmParameters map[string]Parameter) (interface{}, error) {
	switch input := input.(type) {
	case string:
		// replace param names with actual values
		for paramName, paramObj := range ssmParameters {
			input = strings.Replace(input, paramName, paramObj.Value, -1)
		}
		return input, nil

	case []string:
		out, err := parseStringList(log, input, ssmParameters)
		if err != nil {
			return nil, err
		}
		return out, nil

	case []interface{}:
		switch input[0].(type) {
		case string:
			out, err := parseStringList(log, input, ssmParameters)
			if err != nil {
				return nil, err
			}
			return out, nil

		default:
			var err error
			// for slices, recursively replace parameters on each element of the slice
			out := make([]interface{}, len(input))
			for i, v := range input {
				out[i], err = replaceSSMParameters(log, v, ssmParameters)
				if err != nil {
					return nil, err
				}
			}
			return out, nil
		}

	case []map[string]interface{}:
		// this case is not caught by the one above because map cannot be converted to interface{}
		out := make([]map[string]interface{}, len(input))
		for i, v := range input {
			temp, err := replaceSSMParameters(log, v, ssmParameters)
			if err != nil {
				return nil, err
			}
			out[i] = temp.(map[string]interface{})
		}
		return out, nil

	case map[string]interface{}:
		var err error
		// for maps, recursively replace parameters on each value in the map
		out := make(map[string]interface{})
		for k, v := range input {
			out[k], err = replaceSSMParameters(log, v, ssmParameters)
			if err != nil {
				return nil, err
			}
		}
		return out, nil

	default:
		// any other type, return as is
		return input, nil
	}
}

func parseStringList(log log.T, input interface{}, ssmParameters map[string]Parameter) (interface{}, error) {
	/*
		This method parses the input and replaces ssm parameters of the format {{ssm:*}} with their
		actual values. It includes a special case where if the ssm parameter is of type StringList then
		split it into a string array and return.

		Sample:

		SSM parameter
		{{ssm:commands}} = "ls,date,dir"
		Type = StringList

		Input:
		["{{ssm:commands}}"]

		Output:
		["ls","date","dir"]

		Edge case:
		For input {{ssm:commands}} = "echo "a,b",ls,date,dir", output will be
		echo "a
		b"
		ls
		date
		dir

		We assume this use case to be validated at the service side. Even if the service doesn't validate it,
		the agent will continue to split the input based on ","
	*/

	var reformatInput []string
	err := jsonutil.Remarshal(input, &reformatInput)
	if err != nil {
		log.Debug(err)
		return nil, fmt.Errorf("%v", ErrorMsg)
	}

	out := []string{}
	for _, value := range reformatInput {
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
	return out, nil
}
