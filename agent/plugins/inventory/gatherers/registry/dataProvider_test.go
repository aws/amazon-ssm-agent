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
	"testing"

	"github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/aws/amazon-ssm-agent/agent/plugins/inventory/model"
	"github.com/stretchr/testify/assert"
)

var testRegistryOutput = "[{\"ValueName\":\"AMIName\",\"ValueType\":\"REG_SZ\",\"KeyPath\":\"<start1234>HKEY_LOCAL_MACHINE\\SOFTWARE\\Amazon\\MachineImage.Name<end1234>\", \"Value\":\"Windows_Server-2016-English-Full-Base\"},{\"ValueName\": \"AMIVersion\", \"ValueType\": \"REG_SZ\", \"KeyPath\":  \"<start1234>HKEY_LOCAL_MACHINE\\SOFTWARE\\Amazon\\MachineImage.Name<end1234>\",\"Value\": \"2017.08.09\"}]"

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

func testExecuteCommand(command string, args ...string) ([]byte, error) {
	return []byte(testRegistryOutput), nil
}

func testExecuteCommandValueLimit(command string, args ...string) ([]byte, error) {
	return []byte(ValueLimitExceeded), nil
}

func TestGetRegistryData(t *testing.T) {

	contextMock := context.NewMockDefault()
	cmdExecutor = testExecuteCommand
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
	cmdExecutor = testExecuteCommandValueLimit
	startMarker = "<start1234>"
	endMarker = "<end1234>"
	mockFilters := `[{"Path": "HKEY_LOCAL_MACHINE\\SOFTWARE\\Amazon","Recursive": true}, {"Path": "HKEY_LOCAL_MACHINE\\SOFTWARE\\Amazon","Recursive": true, "RegScanLimit": 100}]`
	mockConfig := model.Config{Collection: "Enabled", Filters: mockFilters, Location: ""}
	data, err := collectRegistryData(contextMock, mockConfig)

	assert.NotNil(t, err)
	assert.Nil(t, data)
}
