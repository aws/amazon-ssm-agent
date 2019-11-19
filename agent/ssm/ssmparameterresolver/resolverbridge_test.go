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
	"errors"
	"fmt"
	"testing"

	"github.com/aws/amazon-ssm-agent/agent/framework/docparser/parameterstore"
	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/stretchr/testify/assert"
)

var logMock = log.NewMockLog()

var parameterServiceMock = newServiceMockedObjectWithExtraRecords(
	map[string]SsmParameterInfo{
		"ssm:param":              {Name: "param", Type: parameterstore.ParamTypeString, Value: "plainValue"},
		"ssm-secure:secureParam": {Name: "secureParam", Type: parameterstore.ParamTypeSecureString, Value: "secureValue"},
	})

var ssmParameterResolverBridge = ssmParameterResolverBridgeImpl{
	ssmParamService: &parameterServiceMock,
}

func getString(obj interface{}) string {
	return fmt.Sprintf("%v", obj)
}

func TestIsValidParameterStoreReference(t *testing.T) {
	references := []string{
		"{{ssm-secure:test}}",
		"{{ssm:test}}",
		"{{ssm-secure:p-a.r/a_m}}",
	}
	for _, reference := range references {
		assert.True(t, ssmParameterResolverBridge.IsValidParameterStoreReference(reference), reference)
	}

	references = []string{
		"{{ds:test.}}",
		"test",
		"{{ssm-secure:}}",
	}
	for _, reference := range references {
		assert.False(t, ssmParameterResolverBridge.IsValidParameterStoreReference(reference), reference)
	}
}

func TestExtractParameterStoreParameterReference(t *testing.T) {
	tests := []struct {
		rawParam       string
		paramReference string
		err            error
	}{
		{
			"{{ssm-secure=test}}",
			"",
			fmt.Errorf("Invalid SSM parameter store parameter reference format: %s", "{{ssm-secure=test}}"),
		},
		{
			"{{ssm-secure:test}}",
			"ssm-secure:test",
			nil,
		},
	}

	for _, test := range tests {
		reference, err := ssmParameterResolverBridge.extractParameterStoreParameterReference(test.rawParam)

		if test.err != nil {
			assert.Error(t, err, getString(test))
			assert.EqualError(t, err, test.err.Error(), getString(test))
		} else {
			assert.NoError(t, err, getString(test))
			assert.Equal(t, test.paramReference, reference)
		}
	}
}

func TestResolveSsmParameterStoreReference(t *testing.T) {
	tests := []struct {
		paramReference string
		paramValue     string
		err            error
	}{
		{
			"test",
			"",
			errors.New("error: test cannot be resolved"),
		},
		{
			"ssm:param",
			"plainValue",
			nil,
		},
		{
			"ssm-secure:secureParam",
			"secureValue",
			nil,
		},
	}

	for _, test := range tests {
		value, err := ssmParameterResolverBridge.resolveSsmParameterStoreReference(logMock, test.paramReference)

		if test.err != nil {
			assert.Error(t, err, getString(test))
			assert.EqualError(t, err, test.err.Error(), getString(test))
		} else {
			assert.NoError(t, err, getString(test))
			assert.Equal(t, test.paramValue, value.Value)
		}
	}
}

func TestGetParameterFromSsmParameterStore(t *testing.T) {
	tests := []struct {
		rawParam   string
		paramValue string
		err        error
	}{
		{
			"{{ssm-secure=test}}",
			"",
			fmt.Errorf("Invalid SSM parameter store parameter reference format: %s", "{{ssm-secure=test}}"),
		},
		{
			"{{ssm-secure:test}}",
			"",
			errors.New("error: ssm-secure:test cannot be resolved"),
		},
		{
			"{{ssm-secure:secureParam}}",
			"secureValue",
			nil,
		},
	}

	for _, test := range tests {
		reference, err := ssmParameterResolverBridge.GetParameterFromSsmParameterStore(logMock, test.rawParam)

		if test.err != nil {
			assert.Error(t, err, getString(test))
			assert.EqualError(t, err, test.err.Error(), getString(test))
		} else {
			assert.NoError(t, err, getString(test))
			assert.Equal(t, test.paramValue, reference)
		}
	}
}
