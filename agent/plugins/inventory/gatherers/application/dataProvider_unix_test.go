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
package application

import (
	"fmt"
	"testing"

	"github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/aws/amazon-ssm-agent/agent/plugins/inventory/model"
	"github.com/stretchr/testify/assert"
)

var (
	sampleData = `{"Name":"amazon-ssm-agent","Version":"1.2.0.0-1","Publisher":"Amazon.com, Inc. <ec2-ssm-feedback@amazon.com>",` +
		`"ApplicationType":"admin","Architecture":"amd64","Url":"","Summary":"` +
		mark(`Description with "quotes" 'and' `+"tabs\t"+` and
		new lines`) + `","PackageID":"amazon-ssm-agent_1.2_amd64.rpm"},` +

		`{"Name":"adduser","Version":"3.113+nmu3ubuntu3","Publisher":"Ubuntu Core Developers <ubuntu-devel-discuss@lists.ubuntu.com>",` +
		`"ApplicationType":"admin","Architecture":"all","Url":"http://alioth.debian.org/projects/adduser/",` +
		`"Summary":"` + mark(`add and remove users and groups
 This package includes the 'adduser' and 'deluser' commands for creating
 and removing users.`) + `","PackageID":"adduser_3.113+nmu3ubuntu4_all.deb"},` +

		`{"Name":"sed","Publisher":"Amazon.com","Version":"4.2.1","InstalledTime":"1454346676",` +
		`"ApplicationType":"Applications/Text","Architecture":"x86_64","Url":"http://sed.sourceforge.net/",` +
		`"Summary":"` + mark(`A GNU stream text editor`) + `","PackageID":"sed-4.2.1-7.9.amzn1.src.rpm"},` +

		`{"Name":"sed","Version":"4.2.2-7","Publisher":"Ubuntu Developers <ubuntu-devel-discuss@lists.ubuntu.com>",` +
		`"ApplicationType":"utils","Architecture":"amd64","Url":"http://www.gnu.org/software/sed/",` +
		`"Summary":"` + mark(`The GNU sed stream editor
sed reads the specified files or the standard input if no
files are specified, makes editing changes according to a
list of commands, and writes the results to the standard
output.`) + `","PackageID":"sed_4.2.2-7_amd64.deb"},`
)

var sampleDataParsed = []model.ApplicationData{
	{
		Name:            "amazon-ssm-agent",
		Version:         "1.2.0.0-1",
		Publisher:       "Amazon.com, Inc. <ec2-ssm-feedback@amazon.com>",
		ApplicationType: "admin",
		Architecture:    "x86_64",
		URL:             "",
		Summary:         "Description with \"quotes\" 'and' tabs\t and",
		PackageID:       "amazon-ssm-agent_1.2_amd64.rpm",
	},
	{
		Name:            "adduser",
		Version:         "3.113+nmu3ubuntu3",
		Publisher:       "Ubuntu Core Developers <ubuntu-devel-discuss@lists.ubuntu.com>",
		ApplicationType: "admin",
		Architecture:    "all",
		URL:             "http://alioth.debian.org/projects/adduser/",
		Summary:         "add and remove users and groups",
		PackageID:       "adduser_3.113+nmu3ubuntu4_all.deb",
	},
	{
		Name:            "sed",
		Version:         "4.2.1",
		Publisher:       "Amazon.com",
		InstalledTime:   "2016-02-01T17:11:16Z",
		ApplicationType: "Applications/Text",
		Architecture:    "x86_64",
		URL:             "http://sed.sourceforge.net/",
		Summary:         "A GNU stream text editor",
		PackageID:       "sed-4.2.1-7.9.amzn1.src.rpm",
	},
	{
		Name:            "sed",
		Version:         "4.2.2-7",
		Publisher:       "Ubuntu Developers <ubuntu-devel-discuss@lists.ubuntu.com>",
		ApplicationType: "utils",
		Architecture:    "x86_64",
		URL:             "http://www.gnu.org/software/sed/",
		Summary:         "The GNU sed stream editor",
		PackageID:       "sed_4.2.2-7_amd64.deb",
	},
}

func MockTestExecutorWithError(command string, args ...string) ([]byte, error) {
	var result []byte
	return result, fmt.Errorf("Random Error")
}

func MockTestExecutorWithoutError(command string, args ...string) ([]byte, error) {
	return []byte(sampleData), nil
}

var i = 0

// cmdExecutor returns error first (dpkg) and returns some valid result (rpm)
func MockTestExecutorWithAndWithoutError(command string, args ...string) ([]byte, error) {
	if i == 0 {
		i++
		return MockTestExecutorWithError(command, args...)
	} else {
		return MockTestExecutorWithoutError(command, args...)
	}
}

func TestConvertToApplicationData(t *testing.T) {
	data, err := convertToApplicationData(sampleData)

	assert.Nil(t, err, "Check conversion logic - since sample data in unit test is tied to implementation")
	assertEqual(t, sampleDataParsed, data)
}

func TestGetApplicationData(t *testing.T) {

	var data []model.ApplicationData
	var err error

	//setup
	mockContext := context.NewMockDefault()
	mockCommand := "RandomCommand"
	mockArgs := []string{
		"RandomArgument-1",
		"RandomArgument-2",
	}

	//testing with error
	cmdExecutor = MockTestExecutorWithError

	data, err = getApplicationData(mockContext, mockCommand, mockArgs)

	assert.NotNil(t, err, "Error must be thrown when command execution fails")
	assert.Equal(t, 0, len(data), "When command execution fails - application dataset must be empty")

	//testing without error
	cmdExecutor = MockTestExecutorWithoutError

	data, err = getApplicationData(mockContext, mockCommand, mockArgs)

	assert.Nil(t, err, "Error must not be thrown with MockTestExecutorWithoutError")
	assertEqual(t, sampleDataParsed, data)
}

func TestCollectApplicationData(t *testing.T) {
	mockContext := context.NewMockDefault()

	// both dpkg and rpm return result without error
	cmdExecutor = MockTestExecutorWithoutError
	data := collectPlatformDependentApplicationData(mockContext)
	assertEqual(t, sampleDataParsed, data)

	// both dpkg and rpm return errors
	cmdExecutor = MockTestExecutorWithError
	data = collectPlatformDependentApplicationData(mockContext)
	assert.Equal(t, 0, len(data), "When command execution fails - application dataset must be empty")

	// dpkg returns error and rpm return some result
	cmdExecutor = MockTestExecutorWithAndWithoutError
	data = collectPlatformDependentApplicationData(mockContext)
	assertEqual(t, sampleDataParsed, data)
}

func TestCollectAndMergePackages(t *testing.T) {
	mockContext := context.NewMockDefault()
	packageRepository = MockPackageRepository([]model.ApplicationData{
		{Name: "amazon-ssm-agent", Version: "1.2.0.0-1", Architecture: model.Arch64Bit, CompType: model.AWSComponent},
		{Name: "AwsXRayDaemon", Version: "1.2.3", Architecture: model.Arch64Bit, CompType: model.AWSComponent},
	})

	// both dpkg and rpm return result without error
	cmdExecutor = MockTestExecutorWithoutError
	data := CollectApplicationData(mockContext)
	assert.Equal(t, len(sampleDataParsed)+1, len(data), "Wrong nuber of entries parsed")
}

func TestCollectAndMergePackagesEmpty(t *testing.T) {
	mockContext := context.NewMockDefault()
	packageRepository = MockPackageRepositoryEmpty()

	// both dpkg and rpm return result without error
	cmdExecutor = MockTestExecutorWithoutError
	data := CollectApplicationData(mockContext)
	assert.Equal(t, len(sampleDataParsed), len(data), "Wrong nuber of entries parsed")
}

func TestCollectAndMergePackagesPlatformError(t *testing.T) {
	mockContext := context.NewMockDefault()
	mockData := []model.ApplicationData{
		{Name: "amazon-ssm-agent", Version: "1.2.0.0-1", Architecture: model.Arch64Bit, CompType: model.AWSComponent},
		{Name: "AwsXRayDaemon", Version: "1.2.3", Architecture: model.Arch64Bit, CompType: model.AWSComponent},
	}
	packageRepository = MockPackageRepository(mockData)

	// both dpkg and rpm return result without error
	cmdExecutor = MockTestExecutorWithError
	data := CollectApplicationData(mockContext)
	assert.Equal(t, len(mockData), len(data), "Wrong number of entries")
}

func assertEqual(t *testing.T, expected []model.ApplicationData, found []model.ApplicationData) {
	assert.Equal(t, len(expected), len(found))
	for i, expectedApp := range expected {
		foundApp := found[i]
		assertEqualApps(t, expectedApp, foundApp)
	}
}

func assertEqualApps(t *testing.T, a model.ApplicationData, b model.ApplicationData) {
	assert.Equal(t, a.Name, b.Name)
	assert.Equal(t, a.Publisher, b.Publisher)
	assert.Equal(t, a.Version, b.Version)
	assert.Equal(t, a.InstalledTime, b.InstalledTime)
	assert.Equal(t, a.ApplicationType, b.ApplicationType)
	assert.Equal(t, a.Architecture, b.Architecture)
	assert.Equal(t, a.URL, b.URL)
	assert.Equal(t, a.Summary, b.Summary)
	assert.Equal(t, a.PackageID, b.PackageID)
}
