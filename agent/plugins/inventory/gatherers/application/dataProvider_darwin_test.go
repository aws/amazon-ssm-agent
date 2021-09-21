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

//go:build darwin
// +build darwin

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
	sampleData = `<?xml version="1.0" encoding="UTF-8"?>
                  <!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
                  <plist version="1.0">
					<dict>
						<key>install-location</key>
						<string>private/tmp</string>
						<key>install-time</key>
						<integer>1581382444</integer>
						<key>pkg-version</key>
						<string>16.17.18080304</string>
						<key>pkgid</key>
						<string>randomPackage-1</string>
						<key>receipt-plist-version</key>
						<real>1</real>
						<key>volume</key>
						<string>/</string>
					</dict>
					<dict>
						<key>install-location</key>
						<string>Applications</string>
						<key>install-time</key>
						<integer>1581382096</integer>
						<key>pkg-version</key>
						<string>16.31.19111002</string>
						<key>pkgid</key>
						<string>randomPackage-2</string>
						<key>receipt-plist-version</key>
						<real>1</real>
						<key>volume</key>
						<string>/</string>
					</dict>
                  </plist>`
	unexpectedSampleData = `<a><b>test</b></a>`
	sampleDataPackages   = `<WrapperXMLTag>
	                       <plist version="1.0">
                             <dict>
                                <key>install-location</key>
                                <string>private/tmp</string>
                                <key>install-time</key>
                                <integer>1581382444</integer>
                                <key>pkg-version</key>
                                <string>16.17.18080304</string>
                                <key>pkgid</key>
                                <string>randomPackage-1</string>
                                <key>receipt-plist-version</key>
                                <real>1</real>
                                <key>volume</key>
                                <string>/</string>
                             </dict>
                             </plist>

                             <plist version="1.0">
                             <dict>
                                <key>install-location</key>
                                <string>Applications</string>
                                <key>install-time</key>
                                <integer>1581382096</integer>
                                <key>pkg-version</key>
                                <string>16.31.19111002</string>
                                <key>pkgid</key>
                                <string>randomPackage-2</string>
                                <key>receipt-plist-version</key>
                                <real>1</real>
                                <key>volume</key>
                                <string>/</string>
                             </dict>
                             </plist>
                           </WrapperXMLTag>`
	applicationSampleData = `
                            <key>_name</key>
                            <string>Calendar</string>
                            <key>has64BitIntelCode</key>
                            <string>yes</string>
                            <key>lastModified</key>
                            <date>2019-04-03T07:20:22Z</date>
                            <key>obtained_from</key>
                            <string>apple</string>
                            <key>path</key>
                            <string>/Applications/Calendar.app</string>
                            <key>runtime_environment</key>
                            <string>arch_x86</string>
                            <key>test-key</key>
                            <key>signed_by</key>
                            <array>
                                <string>Software Signing</string>
                                <string>Apple Code Signing Certification Authority</string>
                                <string>Apple Root CA</string>
                            </array>
                            <key>version</key>
                            <string>11.0</string>`

	applicationSampleDataWrapper = `<?xml version="1.0" encoding="UTF-8"?>
                  <!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
                  <plist version="1.0">
                  <array>
                  	<dict>
                  		<key>_SPCommandLineArguments</key>
                  		<array>
                  			<string>/usr/sbin/system_profiler</string>
                  			<string>-nospawn</string>
                  			<string>-xml</string>
                  			<string>SPApplicationsDataType</string>
                  			<string>-detailLevel</string>
                  			<string>full</string>
                  		</array>
                  		<key>_SPCompletionInterval</key>
                  		<real>2.8108129501342773</real>
                  		<key>_SPResponseTime</key>
                  		<real>2.9170479774475098</real>
                  		<key>_dataType</key>
                  		<string>SPApplicationsDataType</string>
                  		<key>_detailLevel</key>
                  		<integer>1</integer>
                  		<key>_items</key>
                  		<array>
                  			<dict>
                  				<key>_name</key>
                  				<string>Calendar</string>
                  				<key>has64BitIntelCode</key>
                  				<string>yes</string>
                  				<key>lastModified</key>
                  				<date>2019-04-03T07:20:22Z</date>
                  				<key>obtained_from</key>
                  				<string>apple</string>
                  				<key>path</key>
                  				<string>/Applications/Calendar.app</string>
                  				<key>runtime_environment</key>
                  				<string>arch_x86</string>
                  				<key>signed_by</key>
                  				<array>
                  					<string>Software Signing</string>
                  					<string>Apple Code Signing Certification Authority</string>
                  					<string>Apple Root CA</string>
                  				</array>
                  				<key>version</key>
                  				<string>11.0</string>
                  			</dict>
                  			<dict>
                  				<key>_name</key>
                  				<string>Amazon Chime</string>
                  				<key>has64BitIntelCode</key>
                  				<string>yes</string>
                  				<key>lastModified</key>
                  				<date>2020-02-06T22:52:21Z</date>
                  				<key>obtained_from</key>
                  				<string>identified_developer</string>
                  				<key>path</key>
                  				<string>/Applications/Amazon Chime.app</string>
                  				<key>runtime_environment</key>
                  				<string>arch_x86</string>
                  				<key>signed_by</key>
                  				<array>
                  					<string>Developer ID Application: AMZN Mobile LLC (94KV3E626L)</string>
                  					<string>Developer ID Certification Authority</string>
                  					<string>Apple Root CA</string>
                  				</array>
                  				<key>version</key>
                  				<string>4.28.7255</string>
                  			</dict>
                  		</array>
                  	</dict>
                  </array>
                  </plist>`
)

var sampleDataParsed = []model.ApplicationData{
	{
		Name:            "Calendar",
		Version:         "11.0",
		Release:         "",
		Epoch:           "",
		Publisher:       "apple",
		ApplicationType: "",
		Architecture:    "arch_x86",
		URL:             "",
		Summary:         "",
		PackageId:       "",
	},
	{
		Name:            "Amazon Chime",
		Version:         "4.28.7255",
		Release:         "",
		Epoch:           "",
		Publisher:       "identified_developer",
		ApplicationType: "",
		Architecture:    "arch_x86",
		URL:             "",
		Summary:         "",
		PackageId:       "",
	},
}

var sampleDataPackagesParsed = []model.ApplicationData{
	{
		Name:            "randomPackage-1",
		Version:         "16.17.18080304",
		InstalledTime:   "2020-02-11T00:54:04Z",
		Release:         "",
		Epoch:           "",
		Publisher:       "",
		ApplicationType: "",
		Architecture:    "",
		URL:             "",
		Summary:         "",
		PackageId:       "",
	},
	{
		Name:            "randomPackage-2",
		Version:         "16.31.19111002",
		InstalledTime:   "2020-02-11T00:48:16Z",
		Release:         "",
		Epoch:           "",
		Publisher:       "",
		ApplicationType: "",
		Architecture:    "",
		URL:             "",
		Summary:         "",
		PackageId:       "",
	},
}

var unexpectedSampleDataParsed = []model.ApplicationData{}

func MockTestExecutorWithError(command string, args ...string) ([]byte, error) {
	var result []byte
	return result, fmt.Errorf("random error")
}

func MockTestExecutorWithoutError(command string, args ...string) ([]byte, error) {
	return []byte(sampleData), nil
}

func TestConvertToApplicationData(t *testing.T) {
	data, err := convertToApplicationData(applicationSampleDataWrapper)

	assert.Nil(t, err, "Check conversion logic - since sample data in unit test is tied to implementation")
	assertEqual(t, sampleDataParsed, data)

	data, err = convertToApplicationData(unexpectedSampleData)
	assertEqual(t, unexpectedSampleDataParsed, data)
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
	cmdExecutor = func(command string, args ...string) ([]byte, error) {
		return []byte(applicationSampleDataWrapper), nil
	}

	data, err = getApplicationData(mockContext, mockCommand, mockArgs)

	assert.Nil(t, err, "Error must not be thrown with MockTestExecutorWithoutError")
	assertEqual(t, sampleDataParsed, data)
}

func TestCollectApplicationData(t *testing.T) {
	mockContext := context.NewMockDefault()

	// sysctl return result without error
	cmdExecutor = func(command string, args ...string) ([]byte, error) {
		return []byte(applicationSampleDataWrapper), nil
	}

	data := collectPlatformDependentApplicationData(mockContext)
	assertEqual(t, sampleDataParsed, data)

	// sysctl return errors
	cmdExecutor = MockTestExecutorWithError
	data = collectPlatformDependentApplicationData(mockContext)
	assert.Equal(t, 0, len(data), "When command execution fails - application dataset must be empty")
}

func TestConvertToApplicationDataFromInstalledPkg(t *testing.T) {
	data, err := convertToApplicationDataFromInstalledPkg(sampleDataPackages)

	assert.Nil(t, err, "Check conversion logic - since sample data in unit test is tied to implementation")
	assertEqual(t, sampleDataPackagesParsed, data)

	data, err = convertToApplicationDataFromInstalledPkg(unexpectedSampleData)
	assertEqual(t, unexpectedSampleDataParsed, data)
}

func TestGetInstalledPackages(t *testing.T) {

	var data []model.ApplicationData
	var err error

	//setup
	mockContext := context.NewMockDefault()
	mockCommand := "RandomCommand"

	//testing with error
	cmdExecutor = MockTestExecutorWithError

	data, err = getInstalledPackages(mockContext, mockCommand)

	assert.NotNil(t, err, "Error must be thrown when command execution fails")
	assert.Equal(t, 0, len(data), "When command execution fails - application dataset must be empty")

	//testing without error
	cmdExecutor = MockTestExecutorWithoutError

	data, err = getInstalledPackages(mockContext, mockCommand)

	assert.Nil(t, err, "Error must not be thrown with MockTestExecutorWithoutError")
	assertEqual(t, sampleDataPackagesParsed, data)
}

func TestGetFieldValue(t *testing.T) {
	name := getFieldValue(applicationSampleData, "_name", "string")
	assert.Equal(t, "Calendar", name)
	version := getFieldValue(applicationSampleData, "version", "string")
	assert.Equal(t, "11.0", version)
	publisher := getFieldValue(applicationSampleData, "obtained_from", "string")
	assert.Equal(t, "apple", publisher)
	architecture := getFieldValue(applicationSampleData, "runtime_environment", "string")
	assert.Equal(t, "arch_x86", architecture)

	// test: if key doesn't exist return empty string
	randomField := getFieldValue(applicationSampleData, "random_field", "string")
	assert.Equal(t, "", randomField)

	// if value doesn't exist for the key, return empty string
	negTestVal := getFieldValue(applicationSampleData, "test-key", "string")
	assert.Equal(t, "", negTestVal)
}
