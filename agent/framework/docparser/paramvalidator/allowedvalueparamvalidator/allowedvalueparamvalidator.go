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

// Package allowedvalueparamvalidator is responsible for validating parameter value
// with the allowed values given in the document.
package allowedvalueparamvalidator

import (
	"encoding/json"
	"fmt"
	"math"

	"github.com/aws/amazon-ssm-agent/agent/contracts"
	paramvalidatorutils "github.com/aws/amazon-ssm-agent/agent/framework/docparser/paramvalidator/utils"
	"github.com/aws/amazon-ssm-agent/agent/log"
)

type allowedValueParamValidator struct {
}

// GetAllowedValueParamValidator returns the allowedValueParamValidator struct reference
func GetAllowedValueParamValidator() *allowedValueParamValidator {
	return &allowedValueParamValidator{}
}

// Validate validates the parameter value with allowed values list
// This function also validates whether the Integer and Boolean types have values equivalent to default values
func (apv *allowedValueParamValidator) Validate(log log.T, parameterValue interface{}, parameter *contracts.Parameter) error {
	var err error
	log.Debugf("Started %v validation", apv.GetName())
	paramType := paramvalidatorutils.DocumentParamType(parameter.ParamType)
	switch paramType {
	case paramvalidatorutils.ParamTypeInteger:
		// check whether the passed integer value is equivalent to the default value
		if err = apv.isIntegerParamValueSetToDefault(parameter, parameterValue); err != nil {
			return err
		}
		return nil
	case paramvalidatorutils.ParamTypeBoolean:
		// checks whether the passed boolean value is equivalent to the default value
		if err = apv.isBooleanParamValueSetToDefault(parameter, parameterValue); err != nil {
			return err
		}
		return nil
	}
	if parameter.AllowedVal != nil && len(parameter.AllowedVal) > 0 {
		switch input := parameterValue.(type) {
		case string:
			err = apv.isValueInAllowedList(input, parameter.AllowedVal)
		case []string:
			for _, v := range input {
				if err = apv.isValueInAllowedList(v, parameter.AllowedVal); err != nil {
					break
				}
			}
		case []interface{}:
			for _, val := range input {
				if convertedVal, ok := val.(string); ok {
					err = apv.isValueInAllowedList(convertedVal, parameter.AllowedVal)
				} else {
					err = apv.verifyAllowedValuesAfterMarshall(log, val, parameter.AllowedVal)
				}
				if err != nil {
					break
				}
			}
		case map[string]interface{}: // for StringMap parameters sent from CLI and other future cases
			err = apv.verifyAllowedValuesAfterMarshall(log, input, parameter.AllowedVal)
		default:
			err = fmt.Errorf("invalid parameter type %v with parameter value %v", parameter.ParamType, input)
		}
	}
	return err
}

// verifyAllowedValuesAfterMarshall serializes parameter value into json and
// validates the allowed value restriction for the json string
func (apv *allowedValueParamValidator) verifyAllowedValuesAfterMarshall(log log.T, v interface{}, allowedValues []string) error {
	var err error
	var byteString []byte
	if byteString, err = json.Marshal(v); err == nil {
		log.Debugf("json marshalling in %v done: %v", apv.GetName(), string(byteString))
		err = apv.isValueInAllowedList(string(byteString), allowedValues)
	}
	return err
}

func (apv *allowedValueParamValidator) isIntegerParamValueSetToDefault(parameter *contracts.Parameter, parameterValue interface{}) error {
	if input, ok := parameterValue.(float64); ok { // default json unmarshalling for int is float64
		if input >= math.MaxInt32 || input <= math.MinInt32 {
			return fmt.Errorf("out-of-boundary error for parameter value /%v/ for type %v ", parameterValue, paramvalidatorutils.ParamTypeInteger)
		}
		// Unmarshalling sometimes return int type for DefaultVal
		// This behavior can be seen in run document plugin
		parameterValueStr := fmt.Sprintf("%v", parameterValue)
		parameterDefaultValueStr := fmt.Sprintf("%v", parameter.DefaultVal)
		if parameterValueStr != parameterDefaultValueStr {
			return fmt.Errorf("parameter value /%v/ not equal to default /%v/ for %v", parameterValue, parameterDefaultValueStr, paramvalidatorutils.ParamTypeInteger)
		}
	} else {
		return fmt.Errorf("could not convert /%v/ to float64 for %v", parameterValue, paramvalidatorutils.ParamTypeInteger)
	}
	return nil
}

func (apv *allowedValueParamValidator) isBooleanParamValueSetToDefault(parameter *contracts.Parameter, parameterValue interface{}) error {
	if parameterValueInBool, ok := parameterValue.(bool); ok {
		if parameterValueInBool == parameter.DefaultVal {
			return nil
		}
	}
	return fmt.Errorf("parameter value /%v/ not equal to default for %v", parameterValue, paramvalidatorutils.ParamTypeBoolean)
}

// GetName returns the name of param validator
func (apv *allowedValueParamValidator) GetName() string {
	return "AllowedValueParamValidator"
}

// isValueInAllowedList checks whether the value is in allowed list
func (apv *allowedValueParamValidator) isValueInAllowedList(input string, allowedValues []string) error {
	for _, val := range allowedValues {
		if val == input {
			return nil
		}
	}
	return fmt.Errorf("parameter value /%v/ is not in the allowed list %v", input, allowedValues)
}
