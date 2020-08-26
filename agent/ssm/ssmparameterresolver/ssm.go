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

// Package ssmparameterresolver contains types and methods for resolving SSM Parameter references.
package ssmparameterresolver

import (
	"errors"
	"strings"

	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/amazon-ssm-agent/agent/ssm"
)

// ISsmParameterService interface represents SSM Parameter service API.
type ISsmParameterService interface {
	getParameters(log log.T, parameterReferences []string) (map[string]SsmParameterInfo, error)
}

// SsmParameterService structure represents an SSM parameter service and implements ISsmParameterService interface.
type SsmParameterService struct {
	ISsmParameterService
	sdk ssm.Service
}

// NewService creates an instance of the SsmParameterService.
func NewService() (service SsmParameterService) {
	return SsmParameterService{sdk: ssm.NewService()}
}

// This function takes a list of at most maxParametersRetrievedFromSsm(=10) ssm parameter name references like (ssm:name).
// It returns a map<param-ref, SsmParameterInfo>.
func (s *SsmParameterService) getParameters(
	log log.T,
	parameterReferences []string) (map[string]SsmParameterInfo, error) {

	ref2NameMapper := make(map[string]string)

	for i := 0; i < len(parameterReferences); i++ {
		nameWithoutPrefix := extractParameterNameFromReference(parameterReferences[i])
		ref2NameMapper[nameWithoutPrefix] = parameterReferences[i]
		parameterReferences[i] = nameWithoutPrefix
	}

	parametersOutput, err := s.sdk.GetDecryptedParameters(log, parameterReferences)
	if err != nil {
		return nil, err
	}

	if len(parametersOutput.InvalidParameters) > 0 {
		invalidParameters := []string{}
		for _, p := range parametersOutput.InvalidParameters {
			invalidParameters = append(invalidParameters, *p)
		}
		return nil, errors.New("The following parameter(s) cannot be resolved: " + strings.Join(invalidParameters, ","))
	}

	resolvedParametersMap := map[string]SsmParameterInfo{}
	for i := 0; i < len(parametersOutput.Parameters); i++ {
		param := parametersOutput.Parameters[i]
		resolvedParametersMap[ref2NameMapper[*param.Name]] = SsmParameterInfo{
			Name:  *param.Name,
			Type:  *param.Type,
			Value: *param.Value,
		}
	}

	return resolvedParametersMap, nil
}

// getParametersFromSsmParameterStore takes as an input a list of references
// to the SSMParameterService and return a map <reference, SSMParameterInfo>
func getParametersFromSsmParameterStore(
	s ISsmParameterService,
	log log.T,
	parametersToFetch []string) (map[string]SsmParameterInfo, error) {

	outputMap := make(map[string]SsmParameterInfo)

	var totalParams = len(parametersToFetch)
	var startPos = 0
	for totalParams > 0 {

		var paramsBatch []string

		var count = 0
		for i := startPos; i < len(parametersToFetch) && count < maxParametersRetrievedFromSsm; i++ {
			paramsBatch = append(paramsBatch, parametersToFetch[i])

			totalParams--
			count++
			startPos++
		}

		results, err := s.getParameters(log, paramsBatch)
		if err != nil {
			return nil, err
		}

		for name, value := range results {
			outputMap[name] = value
		}
	}

	return outputMap, nil
}

func extractParameterNameFromReference(parameterReference string) string {
	return parameterReference[strings.Index(parameterReference, ":")+1:]
}
