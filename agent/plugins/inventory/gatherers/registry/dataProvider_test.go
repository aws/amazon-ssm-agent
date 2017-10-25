// Copyright 2017 Amazon.com, Inc. or its affiliates. All Rights Reserved.
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

package registry

import (
	"errors"
	"testing"

	"github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/aws/amazon-ssm-agent/agent/plugins/inventory/model"
	"github.com/stretchr/testify/assert"
)

var testRegistryOutput = "[{\"ValueName\":\"AMIName\",\"ValueType\":\"REG_SZ\",\"KeyPath\":\"<start1234>HKEY_LOCAL_MACHINE\\SOFTWARE\\Amazon\\MachineImage.Name<end1234>\", \"Value\":\"Windows_Server-2016-English-Full-Base\"},{\"ValueName\": \"AMIVersion\", \"ValueType\": \"REG_SZ\", \"KeyPath\":  \"<start1234>HKEY_LOCAL_MACHINE\\SOFTWARE\\Amazon\\MachineImage.Name<end1234>\",\"Value\": \"2017.08.09\"}]"
var testRegistryOutputInvalid = "[{\"ValueName\":\"<start1234>AMIName\",\"ValueType\":\"REG_SZ\",\"KeyPath\":\"<start1234>HKEY_LOCAL_MACHINE\\SOFTWARE\\Amazon\\MachineImage.Name<end1234>\", \"Value\":\"Windows_Server-2016-English-Full-Base\"},{\"ValueName\": \"AMIVersion\", \"ValueType\": \"REG_SZ\", \"KeyPath\":  \"<start1234>HKEY_LOCAL_MACHINE\\SOFTWARE\\Amazon\\MachineImage.Name\",\"Value\": \"2017.08.09\"}]"

var testRegistryOutputData = []model.RegistryData{
	{
		ValueName: "AMIName",
		ValueType: "REG_SZ",
		KeyPath:   "HKEY_LOCAL_MACHINE\\SOFTWARE\\Amazon\\MachineImage.Name",
		Value:     "Windows_Server-2016-English-Full-Base",
	},
	{
		ValueName: "AMIVersion",
		ValueType: "REG_SZ",
		KeyPath:   "HKEY_LOCAL_MACHINE\\SOFTWARE\\Amazon\\MachineImage.Name",
		Value:     "2017.08.09",
	},
	{
		ValueName: "AMIName",
		ValueType: "REG_SZ",
		KeyPath:   "HKEY_LOCAL_MACHINE\\SOFTWARE\\Amazon\\MachineImage.Name",
		Value:     "Windows_Server-2016-English-Full-Base",
	},
	{
		ValueName: "AMIVersion",
		ValueType: "REG_SZ",
		KeyPath:   "HKEY_LOCAL_MACHINE\\SOFTWARE\\Amazon\\MachineImage.Name",
		Value:     "2017.08.09",
	},
}

var testRegistryOutputDataSingleCall = []model.RegistryData{
	{
		ValueName: "AMIName",
		ValueType: "REG_SZ",
		KeyPath:   "HKEY_LOCAL_MACHINE\\SOFTWARE\\Amazon\\MachineImage.Name",
		Value:     "Windows_Server-2016-English-Full-Base",
	},
	{
		ValueName: "AMIVersion",
		ValueType: "REG_SZ",
		KeyPath:   "HKEY_LOCAL_MACHINE\\SOFTWARE\\Amazon\\MachineImage.Name",
		Value:     "2017.08.09",
	},
}

func createMockExecutor(output []string, err []error) func(string, ...string) ([]byte, error) {
	var index = 0
	return func(string, ...string) ([]byte, error) {
		if index < len(output) {
			index += 1
		}
		return []byte(output[index-1]), err[index-1]
	}
}

func TestGetRegistryData(t *testing.T) {

	contextMock := context.NewMockDefault()
	cmdExecutor = createMockExecutor([]string{testRegistryOutput}, []error{nil})
	startMarker = "<start1234>"
	endMarker = "<end1234>"
	mockFilters := `[{"Path": "HKEY_LOCAL_MACHINE\\SOFTWARE\\Amazon","Recursive": true}, {"Path": "HKEY_LOCAL_MACHINE\\SOFTWARE\\Amazon","Recursive": true, "RegScanLimit": 100}]`
	mockConfig := model.Config{Collection: "Enabled", Filters: mockFilters, Location: ""}
	data, err := collectRegistryData(contextMock, mockConfig)

	assert.Nil(t, err)
	assert.Equal(t, testRegistryOutputData, data)
}

func TestGetRegistryDataValueError(t *testing.T) {

	contextMock := context.NewMockDefault()
	cmdExecutor = createMockExecutor([]string{testRegistryOutput, ValueLimitExceeded}, []error{nil, nil})
	startMarker = "<start1234>"
	endMarker = "<end1234>"
	mockFilters := `[{"Path": "HKEY_LOCAL_MACHINE\\SOFTWARE\\Amazon","Recursive": true}, {"Path": "HKEY_LOCAL_MACHINE\\SOFTWARE\\Amazon","Recursive": true, "RegScanLimit": 100}]`
	mockConfig := model.Config{Collection: "Enabled", Filters: mockFilters, Location: ""}
	data, err := collectRegistryData(contextMock, mockConfig)

	assert.NotNil(t, err)
	assert.Equal(t, testRegistryOutputDataSingleCall, data)
}

func TestGetRegistryDataCmdErr(t *testing.T) {

	contextMock := context.NewMockDefault()
	cmdExecutor = createMockExecutor([]string{testRegistryOutput, ""}, []error{nil, errors.New("error")})
	startMarker = "<start1234>"
	endMarker = "<end1234>"
	mockFilters := `[{"Path": "HKEY_LOCAL_MACHINE\\SOFTWARE\\Amazon","Recursive": true}, {"Path": "HKEY_LOCAL_MACHINE\\SOFTWARE\\Amazon","Recursive": true, "RegScanLimit": 100}]`
	mockConfig := model.Config{Collection: "Enabled", Filters: mockFilters, Location: ""}
	data, err := collectRegistryData(contextMock, mockConfig)

	assert.Nil(t, err)
	assert.Equal(t, testRegistryOutputDataSingleCall, data)
}

func TestGetRegistryDataInvalidOutput(t *testing.T) {

	contextMock := context.NewMockDefault()
	cmdExecutor = createMockExecutor([]string{testRegistryOutput, "InvalidOutput"}, []error{nil, nil})
	startMarker = "<start1234>"
	endMarker = "<end1234>"
	mockFilters := `[{"Path": "HKEY_LOCAL_MACHINE\\SOFTWARE\\Amazon","Recursive": true}, {"Path": "HKEY_LOCAL_MACHINE\\SOFTWARE\\Amazon","Recursive": true, "RegScanLimit": 100}]`
	mockConfig := model.Config{Collection: "Enabled", Filters: mockFilters, Location: ""}
	data, err := collectRegistryData(contextMock, mockConfig)

	assert.Nil(t, err)
	assert.Equal(t, testRegistryOutputDataSingleCall, data)
}

func TestGetRegistryDataInvalidMarker(t *testing.T) {

	contextMock := context.NewMockDefault()
	cmdExecutor = createMockExecutor([]string{testRegistryOutput, testRegistryOutputInvalid}, []error{nil, nil})
	startMarker = "<start1234>"
	endMarker = "<end1234>"
	mockFilters := `[{"Path": "HKEY_LOCAL_MACHINE\\SOFTWARE\\Amazon","Recursive": true}, {"Path": "HKEY_LOCAL_MACHINE\\SOFTWARE\\Amazon","Recursive": true, "RegScanLimit": 100}]`
	mockConfig := model.Config{Collection: "Enabled", Filters: mockFilters, Location: ""}
	data, err := collectRegistryData(contextMock, mockConfig)

	assert.Nil(t, err)
	assert.Equal(t, testRegistryOutputDataSingleCall, data)
}
