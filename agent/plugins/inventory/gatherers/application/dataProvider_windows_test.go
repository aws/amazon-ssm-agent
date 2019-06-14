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

// Package application contains a application gatherer.

// +build windows

package application

import (
	"fmt"
	"testing"

	"github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/aws/amazon-ssm-agent/agent/plugins/inventory/model"
	"github.com/stretchr/testify/assert"
)

var (
	sampleDataSets = []string{
		`{"Name":"Notepad++","Version":"6.9.2","Publisher":"Notepad++ Team","InstalledTime":null},
		{"Name":"AWS Tools for Windows","Version":"3.9.344.0","Publisher":"Amazon Web Services Developer Relations","InstalledTime":"20160512"},
		{"Name":"EC2ConfigService","Version":"3.16.930.0","Publisher":"Amazon Web Services","InstalledTime":null},` +
			// Windows 2008 samples:
			`{
			"Name":  "Microsoft Visual C++ 2008 Redistributable - x64 9.0.30729",
			"PackageId":  "{4FFA2088-8317-3B14-93CD-4C699DB37843}",
			"Version":  "9.0.30729",
			"Publisher":  "Microsoft Corporation",
			"InstalledTime":  "2011-03-05T00:00:00Z"
		},
		{
			"Name":  "Microsoft .NET Framework 4.5.2",
			"PackageId":  "{92FB6C44-E685-45AD-9B20-CADF4CABA132} - 1033",
			"Version":  "4.5.51209",
			"Publisher":  "Microsoft Corporation",
			"InstalledTime":  null
		},` +
			// Windows 2016 samples:
			`{
			"Name":  "Mozilla Firefox 53.0.3 (x64 en-US)",
			"PackageId":  "Mozilla Firefox 53.0.3 (x64 en-US)",
			"Version":  "53.0.3",
			"Publisher":  "Mozilla",
			"InstalledTime":  null
		},
		{
			"Name":  "Go Programming Language amd64 go1.8.3",
			"PackageId":  "{854BC448-6940-4253-9E50-E433E8C2E96A}",
			"Version":  "1.8.3",
			"Publisher":  "https://golang.org",
			"InstalledTime":  "2017-05-31T00:00:00Z"
		},`,
		// single entry testcase
		`{"Name":"Notepad++","Version":"6.9.2","Publisher":"Notepad++ Team","InstalledTime":null},`,
		// no result testcase
		``,
	}
	mockArch     = "randomArch"
	randomString = "blahblah"
)

var sampleDataSetsParsed = [][]model.ApplicationData{
	{
		{Name: "Notepad++", Version: "6.9.2", Publisher: "Notepad++ Team", InstalledTime: ""},
		{Name: "AWS Tools for Windows", Version: "3.9.344.0", Publisher: "Amazon Web Services Developer Relations", InstalledTime: "20160512"},
		{Name: "EC2ConfigService", Version: "3.16.930.0", Publisher: "Amazon Web Services", InstalledTime: ""},
		// Windows 2008 samples:
		{
			Name:          "Microsoft Visual C++ 2008 Redistributable - x64 9.0.30729",
			PackageId:     "{4FFA2088-8317-3B14-93CD-4C699DB37843}",
			Version:       "9.0.30729",
			Publisher:     "Microsoft Corporation",
			InstalledTime: "2011-03-05T00:00:00Z",
		},
		{
			Name:          "Microsoft .NET Framework 4.5.2",
			PackageId:     "{92FB6C44-E685-45AD-9B20-CADF4CABA132} - 1033",
			Version:       "4.5.51209",
			Publisher:     "Microsoft Corporation",
			InstalledTime: "",
		},
		// Windows 2016 samples:
		{
			Name:          "Mozilla Firefox 53.0.3 (x64 en-US)",
			PackageId:     "Mozilla Firefox 53.0.3 (x64 en-US)",
			Version:       "53.0.3",
			Publisher:     "Mozilla",
			InstalledTime: "",
		},
		{
			Name:          "Go Programming Language amd64 go1.8.3",
			PackageId:     "{854BC448-6940-4253-9E50-E433E8C2E96A}",
			Version:       "1.8.3",
			Publisher:     "https://golang.org",
			InstalledTime: "2017-05-31T00:00:00Z",
		},
	},
	{
		{Name: "Notepad++", Version: "6.9.2", Publisher: "Notepad++ Team", InstalledTime: ""},
	},
	{},
}

func MockTestExecutorWithError(command string, args ...string) ([]byte, error) {
	var result []byte
	return result, fmt.Errorf("Random Error")
}

func MockTestExecutorWithConvertToApplicationDataReturningRandomString(command string, args ...string) ([]byte, error) {
	return []byte(randomString), nil
}

func TestConvertToApplicationData(t *testing.T) {

	var data []model.ApplicationData
	var err error

	for i, sampleData := range sampleDataSets {
		data, err = convertToApplicationData(sampleData, mockArch)

		assert.Nil(t, err, "Error is not expected for processing sample data - %v", sampleData)
		assertEqual(t, getDataWithArchitecture(sampleDataSetsParsed[i], mockArch), data)
	}
}

func getDataWithArchitecture(data []model.ApplicationData, architecture string) (dataWithArchitecture []model.ApplicationData) {
	dataWithArchitecture = append(dataWithArchitecture, data...)
	for i := range dataWithArchitecture {
		dataWithArchitecture[i].Architecture = architecture
	}
	return
}

func TestExecutePowershellCommands(t *testing.T) {

	var data []model.ApplicationData
	c := context.NewMockDefault()
	packageRepository = MockPackageRepositoryEmpty()
	mockCmd := "RandomCommand"
	mockArgs := "RandomCommandArgs"

	//testing command executor without errors
	for i, sampleData := range sampleDataSets {
		cmdExecutor = createMockExecutor(sampleData)
		data = executePowershellCommands(c, mockCmd, mockArgs, mockArch)

		assertEqual(t, getDataWithArchitecture(sampleDataSetsParsed[i], mockArch), data)
	}

	//testing command executor with errors
	cmdExecutor = MockTestExecutorWithError
	data = executePowershellCommands(c, mockCmd, mockArgs, mockArch)

	assert.Equal(t, 0, len(data), "On encountering error - application dataset must be empty")

	//testing command executor with ConvertToApplicationData throwing errors
	cmdExecutor = MockTestExecutorWithConvertToApplicationDataReturningRandomString
	data = executePowershellCommands(c, mockCmd, mockArgs, mockArch)

	assert.Equal(t, 0, len(data), "On encountering error during json conversion - application dataset must be empty")
}

func TestCollectApplicationData(t *testing.T) {

	var data []model.ApplicationData
	c := context.NewMockDefault()
	packageRepository = MockPackageRepositoryEmpty()

	// mock OS arch
	detectOSArch = func(context context.T, command, args string) (osArch string) {
		return KeywordFor64BitArchitectureReportedByPowershell
	}

	//testing command executor without errors
	for i, sampleData := range sampleDataSets {
		cmdExecutor = createMockExecutor(sampleData)
		data = collectPlatformDependentApplicationData(c)

		// MockExecutor will be called 2 times: once for i386, once for amd64, hence total entries must be twice the sample data
		var doubleResult []model.ApplicationData
		doubleResult = append(doubleResult, getDataWithArchitecture(sampleDataSetsParsed[i], model.Arch32Bit)...)
		doubleResult = append(doubleResult, getDataWithArchitecture(sampleDataSetsParsed[i], model.Arch64Bit)...)
		assertEqual(t, doubleResult, data)
	}

	//testing command executor with errors
	cmdExecutor = MockTestExecutorWithError
	data = collectPlatformDependentApplicationData(c)

	assert.Equal(t, 0, len(data), "If MockExecutor throws error, application dataset must be empty")
}

func TestCollectAndMergePackages(t *testing.T) {
	mockContext := context.NewMockDefault()
	packageRepository = MockPackageRepository([]model.ApplicationData{
		{Name: "AWS Tools for Windows", Version: "3.9.344.0", Architecture: model.Arch64Bit, CompType: model.AWSComponent},
		{Name: "IntelSriovDriver", Version: "1.2.3", Architecture: model.Arch64Bit},
	})

	for i, sampleData := range sampleDataSets {
		cmdExecutor = createMockExecutor(sampleData)
		data := CollectApplicationData(mockContext)
		// MockExecutor will be called 2 times and there are one or two extra application from the merge
		assert.True(t, 2*len(sampleDataSetsParsed[i])+1 <= len(data))
	}
}
