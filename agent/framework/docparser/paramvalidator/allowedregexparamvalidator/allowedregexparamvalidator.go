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

// Package allowedregexparamvalidator is responsible for validating parameter value
// with regex pattern given in the document.
package allowedregexparamvalidator

import (
	"encoding/json"
	"fmt"
	"regexp"

	"github.com/aws/amazon-ssm-agent/agent/contracts"
	"github.com/aws/amazon-ssm-agent/agent/log"
)

type allowedRegexParamValidator struct {
}

// GetAllowedRegexValidator returns the allowedRegexParamValidator struct reference
func GetAllowedRegexValidator() *allowedRegexParamValidator {
	return &allowedRegexParamValidator{}
}

// Validate validates the parameter value with regex pattern
func (apv *allowedRegexParamValidator) Validate(log log.T, parameterValue interface{}, parameter *contracts.Parameter) error {

	// Check SSM parameter values match the allowed pattern in the document
	if parameter.AllowedPattern == "" {
		return nil
	}

	log.Debugf("Started %v validation", apv.GetName())
	validParamValue, err := regexp.Compile(parameter.AllowedPattern)
	if err != nil {
		return fmt.Errorf("encountered error while compiling allowed pattern - internal error: %v", err)
	}

	errorString := "parameter value /%v/ does not match the allowed pattern /%v/"
	switch input := parameterValue.(type) {
	case string:
		if !validParamValue.MatchString(input) {
			return fmt.Errorf(errorString, input, parameter.AllowedPattern)
		}
	case []string:
		for _, v := range input {
			if !validParamValue.MatchString(v) {
				return fmt.Errorf(errorString, v, parameter.AllowedPattern)
			}
		}
	case []interface{}:
		for _, val := range input {
			if convertedVal, ok := val.(string); ok {
				if !validParamValue.MatchString(convertedVal) {
					return fmt.Errorf(errorString, convertedVal, parameter.AllowedPattern)
				}
			} else if err = apv.verifyRegexAfterMarshall(val, validParamValue); err != nil {
				return fmt.Errorf(errorString, val, parameter.AllowedPattern)
			}
		}
	case map[string]interface{}:
		// For StringMap, when passed through Console, this will be a string
		// From CLI, the translation will be to map[string]interface{}
		if err = apv.verifyRegexAfterMarshall(input, validParamValue); err != nil {
			return fmt.Errorf(errorString, input, parameter.AllowedPattern)
		}
	default:
		return fmt.Errorf("invalid parameter type %v with parameter value %v", parameter.ParamType, input)
	}
	return nil
}

// GetName returns the name of param validator
func (apv *allowedRegexParamValidator) GetName() string {
	return "AllowedRegexParamValidator"
}

func (apv *allowedRegexParamValidator) verifyRegexAfterMarshall(v interface{}, validParamValue *regexp.Regexp) error {
	var err error
	var byteString []byte
	if byteString, err = json.Marshal(v); err == nil {
		if !validParamValue.MatchString(string(byteString)) {
			return fmt.Errorf("pattern is not matching")
		}
	}
	return err
}
