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
	"reflect"
	"strconv"
	"testing"

	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/stretchr/testify/assert"
)

type ServiceMockedObjectWithRecords struct {
	ISsmParameterService
	records map[string]SsmParameterInfo
}

func newServiceMockedObjectWithExtraRecords(
	records map[string]SsmParameterInfo) ServiceMockedObjectWithRecords {
	return ServiceMockedObjectWithRecords{
		records: records,
	}
}

func (m *ServiceMockedObjectWithRecords) getParameters(log log.T, parameterReferences []string) (map[string]SsmParameterInfo, error) {
	parameters := make(map[string]SsmParameterInfo)

	for i := 0; i < len(parameterReferences); i++ {

		value, contains := m.records[parameterReferences[i]]
		if !contains {
			return nil, errors.New("error: " + parameterReferences[i] + " cannot be resolved")
		}

		parameters[parameterReferences[i]] = value
	}

	return parameters, nil
}

func TestGetParametersFromSsmParameterStoreWithAllResolvedNoPaging(t *testing.T) {
	parametersList := []string{}
	expectedValues := map[string]SsmParameterInfo{}

	for i := 0; i < maxParametersRetrievedFromSsm/2; i++ {
		name := "name_" + strconv.Itoa(i)
		key := ssmNonSecurePrefix + name
		parametersList = append(parametersList, key)

		expectedValues[key] = SsmParameterInfo{
			Name:  name,
			Value: "value_" + name,
			Type:  "String",
		}
	}

	serviceObject := newServiceMockedObjectWithExtraRecords(expectedValues)

	log := log.DefaultLogger()
	t.Log("Testing getParametersFromSsmParameterStore API for all parameters present without paging...")
	retrievedValues, err := getParametersFromSsmParameterStore(&serviceObject, log, parametersList)
	assert.Nil(t, err)
	assert.True(t, reflect.DeepEqual(expectedValues, retrievedValues))
}

func TestGetParametersFromSsmParameterStoreWithAllResolvedWithPaging(t *testing.T) {
	parametersList := []string{}
	expectedValues := map[string]SsmParameterInfo{}

	for i := 0; i < maxParametersRetrievedFromSsm/5; i++ {
		name := "name_" + strconv.Itoa(i)
		key := ssmSecurePrefix + name
		parametersList = append(parametersList, key)

		expectedValues[key] = SsmParameterInfo{
			Name:  name,
			Value: "value_" + name,
			Type:  secureStringType,
		}
	}

	serviceObject := newServiceMockedObjectWithExtraRecords(expectedValues)

	log := log.DefaultLogger()

	t.Log("Testing getParametersFromSsmParameterStore API for all parameters present with paging...")
	retrievedValues, err := getParametersFromSsmParameterStore(&serviceObject, log, parametersList)
	assert.Nil(t, err)
	assert.True(t, reflect.DeepEqual(expectedValues, retrievedValues))
}

func TestGetParametersFromSsmParameterStoreWithUnresolvedIgnoreNoPaging(t *testing.T) {
	parametersList := []string{}
	for i := 0; i < 2; i++ {
		key := "{{ssm:name_" + strconv.Itoa(i) + "}}"
		parametersList = append(parametersList, key)
	}

	serviceObject := newServiceMockedObjectWithExtraRecords(map[string]SsmParameterInfo{})

	log := log.DefaultLogger()

	t.Log("Testing getParametersFromSsmParameterStore API for all unresolved parameters...")
	_, err := getParametersFromSsmParameterStore(&serviceObject, log, parametersList)
	assert.NotNil(t, err)
}
