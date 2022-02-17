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

// Package minmaxcharparamvalidator is responsible for validating parameter value
// with the min max char restriction given in the document for parameters.
package minmaxcharparamvalidator

import (
	"encoding/json"
	"fmt"
	"math"

	"github.com/aws/amazon-ssm-agent/agent/contracts"
	paramvalidatorutils "github.com/aws/amazon-ssm-agent/agent/framework/docparser/paramvalidator/utils"
	"github.com/aws/amazon-ssm-agent/agent/log"
)

type minMaxCharParamValidator struct {
	ignoreList map[paramvalidatorutils.DocumentParamType]struct{}
}

// GetMinMaxCharValidator returns the minMaxCharParamValidator struct reference
func GetMinMaxCharValidator() *minMaxCharParamValidator {
	minMaxCharValidator := minMaxCharParamValidator{
		ignoreList: make(map[paramvalidatorutils.DocumentParamType]struct{}),
	}
	// ignore this validation for integer and boolean types
	// min and max chars cannot be added for integer and boolean types
	minMaxCharValidator.ignoreList[paramvalidatorutils.ParamTypeInteger] = struct{}{}
	minMaxCharValidator.ignoreList[paramvalidatorutils.ParamTypeBoolean] = struct{}{}
	return &minMaxCharValidator
}

// Validate validates the parameter value with min max character restriction given in the document
func (mpv *minMaxCharParamValidator) Validate(log log.T, parameterValue interface{}, parameter *contracts.Parameter) error {
	minChar := -1
	maxChar := -1

	log.Debugf("Started %v validation", mpv.GetName())
	// ignore this validation when min characters and max characters not set in the document parameter
	if parameter.MinChars == "" && parameter.MaxChars == "" {
		return nil
	}
	// ignore this validation when the parameter type is present in ignore list
	if _, ok := mpv.ignoreList[paramvalidatorutils.DocumentParamType(parameter.ParamType)]; ok {
		return nil
	}
	// min and max value for MinChar/MaxChar is 0 and math.MaxInt32 respectively
	if parameter.MinChars != "" {
		// cleanup min char value
		minChar = paramvalidatorutils.GetCleanedUpVal(log, parameter.MinChars, 0)
	}
	if parameter.MaxChars != "" {
		// cleanup max char value
		maxChar = paramvalidatorutils.GetCleanedUpVal(log, parameter.MaxChars, math.MaxInt32)
	}

	// for StringList, data structure will be []interface{"string", "string", ..}
	// for StringMap, data structure will be
	//		* "string" when received from Console
	//      * map[string]interface{} from CLI
	// for MapList, data structure will be []interface{ map[string]interface{}, ..}
	var err error
	switch input := parameterValue.(type) {
	case string:
		// enters when the parameter type is String and StringMap
		err = mpv.verifyStringLen(minChar, maxChar, input)
	case []string:
		// following previous developer for now.
		// document parameters did not visit this case while testing.
		for _, v := range input {
			if err = mpv.verifyStringLen(minChar, maxChar, v); err != nil {
				break
			}
		}
	case []interface{}:
		// enters when the parameter type is StringList and MapList.
		// MapList is not supported for Command documents.
		// Only default values can be passed for MapList and document with this type can be executed only from the CLI
		for _, inp := range input {
			if convertedVal, ok := inp.(string); ok { // for StringList
				err = mpv.verifyStringLen(minChar, maxChar, convertedVal)
			} else { // for MapList
				err = mpv.verifyMinMaxCharAfterMarshall(log, inp, minChar, maxChar)
			}
			if err != nil {
				break
			}
		}
	case map[string]interface{}: // for StringMap parameters sent from CLI and other future cases
		err = mpv.verifyMinMaxCharAfterMarshall(log, input, minChar, maxChar)
	default:
		err = fmt.Errorf("invalid parameter type %v with parameter value %v", parameter.ParamType, input)
	}
	return err
}

// GetName returns the name of param validator
func (mpv *minMaxCharParamValidator) GetName() string {
	return "MinMaxCharParamValidator"
}

// verifyMinMaxCharAfterMarshall serializes parameter value into json and
// validates the min max character restriction for the returned string
func (mpv *minMaxCharParamValidator) verifyMinMaxCharAfterMarshall(log log.T, v interface{}, minChar int, maxChar int) error {
	var err error
	var byteString []byte
	if byteString, err = json.Marshal(v); err == nil {
		log.Debugf("json marshalling in %v done: %v", mpv.GetName(), string(byteString))
		err = mpv.verifyStringLen(minChar, maxChar, string(byteString))
	}
	return err
}

// verifyStringLen verifies the min/max character for a parameter
func (mpv *minMaxCharParamValidator) verifyStringLen(minChar int, maxChar int, input string) error {
	if minChar != -1 && len(input) < minChar {
		return fmt.Errorf("parameter value /%v/ is less than the min char limit /%v/", input, minChar)
	}
	if maxChar != -1 && len(input) > maxChar {
		return fmt.Errorf("parameter value /%v/ is exceeding the max chars /%v/", input, maxChar)
	}
	return nil
}
