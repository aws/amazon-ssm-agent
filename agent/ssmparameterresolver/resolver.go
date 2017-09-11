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
	"regexp"
	"strings"

	"github.com/aws/amazon-ssm-agent/agent/log"
)

// ExtractParametersFromText takes text document and resolves all parameters in it according to ResolveOptions.
// It will return a map of (parameter references) to SsmParameterInfo.
func ExtractParametersFromText(
	service ISsmParameterService,
	log log.T,
	input string,
	options ResolveOptions) (map[string]SsmParameterInfo, error) {

	uniqueParameterReferences, err := parseParametersFromTextIntoDedupedSlice(input, options.IgnoreSecureParameters)
	if err != nil {
		return nil, err
	}

	parametersWithValues, err := getParametersFromSsmParameterStore(service, log, uniqueParameterReferences)
	if err != nil {
		return nil, err
	}

	prefixValidationError := validateParameterReferencePrefix(&parametersWithValues)
	if prefixValidationError != nil {
		return nil, prefixValidationError
	}

	return parametersWithValues, nil
}

// ResolveParameterReferenceList takes a list of SSM parameter references, resolves them according to ResolveOptions and
// returns a map of (parameter references) to SsmParameterInfo.
func ResolveParameterReferenceList(
	service ISsmParameterService,
	log log.T,
	parameterReferences []string,
	options ResolveOptions) (map[string]SsmParameterInfo, error) {

	uniqueParameterReferences := dedupSlice(parameterReferences)
	parameterReferencesToResolve := []string{}
	if options.IgnoreSecureParameters {
		for _, ref := range uniqueParameterReferences {
			if strings.HasPrefix(ref, ssmNonSecurePrefix) {
				parameterReferencesToResolve = append(parameterReferencesToResolve, ref)
			}
		}
	} else {
		parameterReferencesToResolve = append(parameterReferencesToResolve, uniqueParameterReferences...)
	}

	parametersWithValues, err := getParametersFromSsmParameterStore(service, log, parameterReferencesToResolve)
	if err != nil {
		return nil, err
	}

	prefixValidationError := validateParameterReferencePrefix(&parametersWithValues)
	if prefixValidationError != nil {
		return nil, prefixValidationError
	}

	return parametersWithValues, nil
}

// ResolveParametersInText takes text document, resolves all parameters in it according to ResolveOptions
// and returns resolved document.
func ResolveParametersInText(
	service ISsmParameterService,
	log log.T,
	input string,
	options ResolveOptions) (string, error) {

	resolvedParametersMap, err := ExtractParametersFromText(service, log, input, options)
	if err != nil || resolvedParametersMap == nil || len(resolvedParametersMap) == 0 {
		return input, err
	}

	for ref, param := range resolvedParametersMap {
		var placeholder = regexp.MustCompile("{{\\s*" + ref + "\\s*}}")
		input = placeholder.ReplaceAllString(input, param.Value)
	}

	return input, nil
}

func validateParameterReferencePrefix(resolvedParametersMap *map[string]SsmParameterInfo) error {
	for key, value := range *resolvedParametersMap {
		if strings.HasPrefix(key, ssmSecurePrefix) && value.Type != secureStringType {
			return errors.New("secure prefix " + ssmSecurePrefix + " is used for a non-secure type " + value.Type)
		}

		if strings.HasPrefix(key, ssmNonSecurePrefix) && value.Type == secureStringType {
			return errors.New("non-secure prefix " + ssmNonSecurePrefix + " is used for a secure type " + value.Type)
		}
	}

	return nil
}

func dedupSlice(slice []string) []string {
	ht := map[string]bool{}

	for _, element := range slice {
		ht[element] = true
	}

	keys := make([]string, len(ht))

	i := 0
	for k := range ht {
		keys[i] = k
		i++
	}

	return keys
}

func parseParametersFromTextIntoDedupedSlice(text string, ignoreSecureParameters bool) ([]string, error) {
	matchedPhrases := ssmParameterPlaceholderRegEx.FindAllStringSubmatch(text, -1)

	parameterNamesDeduped := make(map[string]bool)
	for i := 0; i < len(matchedPhrases); i++ {
		parameterNamesDeduped[matchedPhrases[i][1]] = true
	}

	if !ignoreSecureParameters {
		matchedSecurePhrases := secureSsmParameterPlaceholderRegEx.FindAllStringSubmatch(text, -1)
		for i := 0; i < len(matchedSecurePhrases); i++ {
			parameterNamesDeduped[matchedSecurePhrases[i][1]] = true
		}
	}

	result := []string{}
	for key := range parameterNamesDeduped {
		result = append(result, key)
	}

	return result, nil
}
