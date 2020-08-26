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
	"reflect"
	"sort"
	"testing"

	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/stretchr/testify/assert"
)

func TestExtractParametersFromText(t *testing.T) {
	expectedResult := map[string]SsmParameterInfo{
		"ssm:/a/b/c/param1": {Name: "/a/b/c/param1", Type: stringType, Value: "value_/a/b/c/param1"},
		"ssm-secure:param2": {Name: "param2", Type: secureStringType, Value: "value_param2"},
	}
	serviceObject := newServiceMockedObjectWithExtraRecords(expectedResult)

	log := log.DefaultLogger()
	text := "Some text {{ ssm:/a/b/c/param1}}, some more text {{ssm-secure:param2}}."
	resolvedParameters, err := ExtractParametersFromText(&serviceObject, log, text, ResolveOptions{
		IgnoreSecureParameters: false,
	})

	assert.Nil(t, err)
	assert.NotNil(t, resolvedParameters)
	assert.True(t, reflect.DeepEqual(resolvedParameters, expectedResult))
}

func TestExtractParametersFromTextIgnoreSecureParams(t *testing.T) {
	serviceObject := newServiceMockedObjectWithExtraRecords(map[string]SsmParameterInfo{
		"ssm:/a/b/c/param1":        {Name: "/a/b/c/param1", Type: stringType, Value: "value_/a/b/c/param1"},
		"ssm-secure:/a/b/c/param1": {Name: "param2", Type: secureStringType, Value: "value_/a/b/c/param1"},
		"ssm-secure:param2":        {Name: "param2", Type: secureStringType, Value: "value_param2"},
	})

	log := log.DefaultLogger()
	text := "Some text {{ ssm:/a/b/c/param1}}, some more text {{ssm-secure:param2}} - {{ ssm-secure:/a/b/c/param1}}."
	resolvedParameters, err := ExtractParametersFromText(&serviceObject, log, text, ResolveOptions{
		IgnoreSecureParameters: true,
	})

	expectedResult := map[string]SsmParameterInfo{
		"ssm:/a/b/c/param1": {Name: "/a/b/c/param1", Type: stringType, Value: "value_/a/b/c/param1"},
	}

	assert.Nil(t, err)
	assert.NotNil(t, resolvedParameters)
	assert.True(t, reflect.DeepEqual(resolvedParameters, expectedResult))
}

func TestExtractParametersFromTextWrongPrefix(t *testing.T) {
	expectedResult := map[string]SsmParameterInfo{
		"ssm:/a/b/c/param1": {Name: "/a/b/c/param1", Type: stringType, Value: "value_/a/b/c/param1"},
		"ssm:param2":        {Name: "param2", Type: secureStringType, Value: "value_param2"}, // wrong prefix
	}

	serviceObject := newServiceMockedObjectWithExtraRecords(expectedResult)

	log := log.DefaultLogger()

	text := "Some text {{ ssm:/a/b/c/param1}}, some more text {{ssm:param2}}."
	_, err := ExtractParametersFromText(&serviceObject, log, text, ResolveOptions{
		IgnoreSecureParameters: false,
	})

	assert.NotNil(t, err)
}

func TestResolveParameterReferenceList(t *testing.T) {
	expectedResult := map[string]SsmParameterInfo{
		"ssm:param1":             {Name: "param1", Type: stringType, Value: "value_param1"},
		"ssm:param2":             {Name: "param2", Type: stringType, Value: "value_param2"},
		"ssm-secure:/a/b/param1": {Name: "/a/b/param1", Type: secureStringType, Value: "value_/a/b/param1"},
		"ssm-secure:param4":      {Name: "param4", Type: secureStringType, Value: "value_param4"},
	}
	serviceObject := newServiceMockedObjectWithExtraRecords(expectedResult)

	parameterReferences := []string{
		"ssm:param1",
		"ssm:param2",
		"ssm-secure:/a/b/param1",
		"ssm-secure:param4",
	}

	log := log.DefaultLogger()
	resolvedParameters, err := ResolveParameterReferenceList(&serviceObject, log, parameterReferences, ResolveOptions{
		IgnoreSecureParameters: false,
	})

	assert.Nil(t, err)
	assert.NotNil(t, resolvedParameters)
	assert.True(t, reflect.DeepEqual(resolvedParameters, expectedResult))
}

func TestResolveParameterReferenceListIgnoreSecureParams(t *testing.T) {
	serviceObject := newServiceMockedObjectWithExtraRecords(map[string]SsmParameterInfo{
		"ssm:param1":             {Name: "param1", Type: stringType, Value: "value_param1"},
		"ssm:param2":             {Name: "param2", Type: stringType, Value: "value_param2"},
		"ssm-secure:/a/b/param1": {Name: "/a/b/param1", Type: secureStringType, Value: "value_/a/b/param1"},
		"ssm-secure:param4":      {Name: "param4", Type: secureStringType, Value: "value_param4"},
	})

	parameterReferences := []string{
		"ssm:param1",
		"ssm:param2",
		"ssm-secure:/a/b/param1",
		"ssm-secure:param4",
		"ssm:param2",
	}

	log := log.DefaultLogger()
	resolvedParameters, err := ResolveParameterReferenceList(&serviceObject, log, parameterReferences, ResolveOptions{
		IgnoreSecureParameters: true,
	})

	expectedResult := map[string]SsmParameterInfo{
		"ssm:param1": {Name: "param1", Type: stringType, Value: "value_param1"},
		"ssm:param2": {Name: "param2", Type: stringType, Value: "value_param2"},
	}

	assert.Nil(t, err)
	assert.NotNil(t, resolvedParameters)
	assert.True(t, reflect.DeepEqual(resolvedParameters, expectedResult))
}

func TestParseParametersFromTextIntoMapSecureAllowed(t *testing.T) {
	text := "Some text {{ ssm:/a/b/c/param1}}, some more text {{ssm-secure:param2}}, {{ ssm-secure:/a/b/c/param1  }}."
	expectedList := []string{"ssm:/a/b/c/param1", "ssm-secure:param2", "ssm-secure:/a/b/c/param1"}

	list, err := parseParametersFromTextIntoDedupedSlice(text, false)

	assert.Nil(t, err)
	assert.NotNil(t, list)

	sort.Slice(expectedList, func(i, j int) bool { return expectedList[i] < expectedList[j] })
	sort.Slice(list, func(i, j int) bool { return list[i] < list[j] })
	assert.True(t, reflect.DeepEqual(list, expectedList))
}

func TestParseParametersFromTextIntoMapSecureNotAllowed(t *testing.T) {
	text := "Some text {{ ssm:/a/b/c/param1}}, some more text {{ssm-secure:param2}}, {{ ssm-secure:/a/b/c/param1  }}."
	expectedList := []string{"ssm:/a/b/c/param1"}

	list, err := parseParametersFromTextIntoDedupedSlice(text, true)

	assert.Nil(t, err)
	assert.NotNil(t, list)

	sort.Slice(expectedList, func(i, j int) bool { return expectedList[i] < expectedList[j] })
	sort.Slice(list, func(i, j int) bool { return list[i] < list[j] })
	assert.True(t, reflect.DeepEqual(list, expectedList))
}

func TestResolveParametersInText(t *testing.T) {
	serviceObject := newServiceMockedObjectWithExtraRecords(map[string]SsmParameterInfo{
		"ssm:/a/b/c/param1": {Name: "/a/b/c/param1", Type: stringType, Value: "value_/a/b/c/param1"},
		"ssm-secure:param2": {Name: "param2", Type: secureStringType, Value: "value_param2"},
	})

	log := log.DefaultLogger()
	text := "Some text {{ ssm:/a/b/c/param1}}, some more text {{ssm-secure:param2}}. {{ ssm:/a/b/c/param1  }};"
	output, err := ResolveParametersInText(&serviceObject, log, text, ResolveOptions{
		IgnoreSecureParameters: false,
	})

	expectedOutput := `Some text value_/a/b/c/param1, some more text value_param2. value_/a/b/c/param1;`

	assert.Nil(t, err)
	assert.NotNil(t, output)
	assert.True(t, expectedOutput == output)
}

func TestResolveParametersInTextIgnoreSecureParams(t *testing.T) {
	serviceObject := newServiceMockedObjectWithExtraRecords(map[string]SsmParameterInfo{
		"ssm:/a/b/c/param1": {Name: "/a/b/c/param1", Type: stringType, Value: "value_/a/b/c/param1"},
		"ssm-secure:param2": {Name: "param2", Type: secureStringType, Value: "value_param2"},
	})

	log := log.DefaultLogger()
	text := "Some text {{ ssm:/a/b/c/param1}}, some more text {{ssm-secure:param2}}."
	output, err := ResolveParametersInText(&serviceObject, log, text, ResolveOptions{
		IgnoreSecureParameters: true,
	})

	expectedOutput := `Some text value_/a/b/c/param1, some more text {{ssm-secure:param2}}.`

	assert.Nil(t, err)
	assert.NotNil(t, output)
	assert.True(t, expectedOutput == output)
}
