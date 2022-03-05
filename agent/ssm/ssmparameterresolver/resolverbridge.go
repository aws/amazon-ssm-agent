/*
 * Copyright 2019 Amazon.com, Inc. or its affiliates. All Rights Reserved.
 *
 * Licensed under the Apache License, Version 2.0 (the "License"). You may not
 * use this file except in compliance with the License. A copy of the
 * License is located at
 *
 * http://aws.amazon.com/apache2.0/
 *
 * or in the "license" file accompanying this file. This file is distributed
 * on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND,
 * either express or implied. See the License for the specific language governing
 * permissions and limitations under the License.
 */

// Package ssmparameterresolver provides helper methods to detect, validate and extract parameter store parameter references.
package ssmparameterresolver

import (
	"fmt"
	"regexp"

	"github.com/aws/amazon-ssm-agent/agent/log"
)

// The format of a valid secure parameter store parameter reference
var ssmParamReferencePattern = regexp.MustCompile(fmt.Sprintf("{{\\s*((?:%s|%s)[\\w-./]+)\\s*}}", ssmSecurePrefix, ssmNonSecurePrefix))

// ISsmParameterResolverBridge defines methods for validating and resolving parameter store parameter references
// through the ssm parameter store service
type ISsmParameterResolverBridge interface {
	IsValidParameterStoreReference(value string) bool
	GetParameterFromSsmParameterStore(log log.T, parameter string) (string, error)
}

// ssmParameterResolverBridgeImpl moderates the communication to the ssm parameter store service
type ssmParameterResolverBridgeImpl struct {
	ssmParamService ISsmParameterService
}

// NewSsmParameterResolverBridge creates a new ssm parameter resolver bridge object
func NewSsmParameterResolverBridge(ssmParameterService ISsmParameterService) ISsmParameterResolverBridge {
	return &ssmParameterResolverBridgeImpl{
		ssmParamService: ssmParameterService,
	}
}

// IsValidParameterStoreReference determines whether the given value is a valid ssm parameter store parameter reference
func (bridge *ssmParameterResolverBridgeImpl) IsValidParameterStoreReference(value string) bool {
	return ssmParamReferencePattern.MatchString(value)
}

// GetParameterFromSsmParameterStore returns the value of the given parameter store parameter
func (bridge *ssmParameterResolverBridgeImpl) GetParameterFromSsmParameterStore(log log.T, parameter string) (string, error) {
	reference, err := bridge.extractParameterStoreParameterReference(parameter)
	if err != nil {
		return "", err
	}

	parameterInfo, err := bridge.resolveSsmParameterStoreReference(log, reference)
	if err != nil {
		return "", err
	}

	return parameterInfo.Value, nil
}

// extractParameterStoreParameterReference extract the actual parameter name from the reference structure
func (bridge *ssmParameterResolverBridgeImpl) extractParameterStoreParameterReference(parameter string) (string, error) {
	// Regex to extract the contents of the parameter from within {{ }} to get parameter value
	// for. e.g. {{ ssm-secure:parameter-name }} will extract ssm-secure:parameter-name
	matchGroups := ssmParamReferencePattern.FindStringSubmatch(parameter)
	if matchGroups == nil || len(matchGroups) != 2 {
		return "", fmt.Errorf("Invalid SSM parameter store parameter reference format: %s", parameter)
	}

	return matchGroups[1], nil
}

// resolveSsmParameterStoreReference resolves the given parameter store parameter references through the ssm parameter store service
func (bridge *ssmParameterResolverBridgeImpl) resolveSsmParameterStoreReference(log log.T, parameterReference string) (info *SsmParameterInfo, err error) {
	resolverOptions := ResolveOptions{
		IgnoreSecureParameters: false,
	}

	// Get the parameter value from parameter store.
	// NOTE: Do not log the parameter value
	parametersMap, err := ResolveParameterReferenceList(bridge.ssmParamService, log, []string{parameterReference}, resolverOptions)
	if err != nil {
		return nil, err
	}

	// Parameter output must be of size 1. Any other number of tokens returned can lead to undesired behavior
	if len(parametersMap) != 1 {
		return nil, fmt.Errorf("Invalid number of tokens returned - %v", len(parametersMap))
	}

	var parameterInfo SsmParameterInfo
	//Extracting single value of token contained within parametersMap
	for _, token := range parametersMap {
		parameterInfo = token
	}

	return &parameterInfo, nil
}
