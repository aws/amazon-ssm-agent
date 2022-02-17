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

// Package minmaxitemparamvalidator is responsible for validating parameter value
// with the min max item restriction given in the document for parameters.
package minmaxitemparamvalidator

import (
	"fmt"
	"math"

	"github.com/aws/amazon-ssm-agent/agent/contracts"
	paramvalidatorutils "github.com/aws/amazon-ssm-agent/agent/framework/docparser/paramvalidator/utils"
	"github.com/aws/amazon-ssm-agent/agent/log"
)

type minMaxItemParamValidator struct {
	ignoreList map[paramvalidatorutils.DocumentParamType]struct{}
}

// GetMinMaxItemValidator returns the minMaxItemParamValidator struct reference
func GetMinMaxItemValidator() *minMaxItemParamValidator {
	minMaxCharValidator := minMaxItemParamValidator{
		ignoreList: make(map[paramvalidatorutils.DocumentParamType]struct{}),
	}
	// ignore this validation for string, integer and boolean types
	// min and max item restriction cannot be added for integer and boolean parameter type
	// not applicable for string too
	minMaxCharValidator.ignoreList[paramvalidatorutils.ParamTypeInteger] = struct{}{}
	minMaxCharValidator.ignoreList[paramvalidatorutils.ParamTypeBoolean] = struct{}{}
	minMaxCharValidator.ignoreList[paramvalidatorutils.ParamTypeString] = struct{}{}
	return &minMaxCharValidator
}

// Validate validates the parameter value with min-max item restriction given in the document
func (mpv *minMaxItemParamValidator) Validate(log log.T, parameterValue interface{}, parameter *contracts.Parameter) error {
	minItem := -1
	maxItem := -1

	log.Debugf("Started %v validation", mpv.GetName())

	// pass when min item and max item not set in the document parameter
	if parameter.MinItems == "" && parameter.MaxItems == "" {
		return nil
	}
	// ignore this validation when the parameter type is present in ignore list
	if _, ok := mpv.ignoreList[paramvalidatorutils.DocumentParamType(parameter.ParamType)]; ok {
		return nil
	}
	if parameter.MinItems != "" {
		// cleanup min item value
		minItem = paramvalidatorutils.GetCleanedUpVal(log, parameter.MinItems, 0)
	}
	if parameter.MaxItems != "" {
		// cleanup max item value
		maxItem = paramvalidatorutils.GetCleanedUpVal(log, parameter.MaxItems, math.MaxInt32)
	}

	var err error
	switch input := parameterValue.(type) {
	case string: // For StringMap, when passed through Console, this will be a string. Just pass through this type
	case []string:
		// following previous developer for now.
		// document parameters did not visit this case while testing.
		err = mpv.verifyItemCount(minItem, maxItem, len(input))
	case []interface{}:
		// enters when the parameter type is StringList and MapList.
		// MapList is not supported for Command documents.
		// Only default values can be passed for MapList and document with this type can be executed only from the CLI
		err = mpv.verifyItemCount(minItem, maxItem, len(input))
	case map[string]interface{}:
		// For StringMap, when passed through Console, this will be a string
		// From CLI, the translation will be to map[string]interface{}
		err = mpv.verifyItemCount(minItem, maxItem, len(input))
	default:
		err = fmt.Errorf("invalid parameter type %v with parameter value %v", parameter.ParamType, input)
	}
	return err
}

// GetName returns the name of param validator
func (mpv *minMaxItemParamValidator) GetName() string {
	return "MinMaxItemParamValidator"
}

// verifyItemCount verifies the parameter value length with Min/Max item length given in parameter definition
func (mpv *minMaxItemParamValidator) verifyItemCount(minItem int, maxItem int, itemLen int) error {
	if minItem != -1 && itemLen < minItem {
		return fmt.Errorf("parameter value list length /%v/ is less the min item count", itemLen)
	}
	if maxItem != -1 && itemLen > maxItem {
		return fmt.Errorf("parameter value list length /%v/ has exceeded the max item count", maxItem)
	}
	return nil
}
