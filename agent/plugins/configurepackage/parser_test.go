// Copyright 2016 Amazon.com, Inc. or its affiliates. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License"). You may not
// use this file except in compliance with the License. A copy of the
// License is located at
//
// http://aws.amazon.com/apache2.0/
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or
// implied. See the License for the specific language governing
// permissions and limitations under the License.

// Package configurepackage implements the ConfigurePackage plugin.
package configurepackage

import (
	"encoding/json"
	"io/ioutil"
	"testing"

	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/stretchr/testify/assert"
)

// Valid manifest files
var sampleManifests = []string{
	"testdata/sampleManifest.json",
	"testdata/daemonManifest.json",
	"testdata/simpleManifest.json",
}

// Invalid manifest files
var errorManifests = []string{
	"testdata/errorManifest_empty.json",
	"testdata/errorManifest_reboot.json",
	"testdata/errorManifest_version.json",
	"testdata/errorManifest_versionempty.json",
}

// Malformed manifest files
var malformedManifests = []string{
	"testdata/errorManifest_nonexistent.json",
	"testdata/errorManifest_malformed.json",
}

type testCase struct {
	Input  string
	Output *PackageManifest
}

// TestParseManifest testing parses valid manifest files
func TestParseManifest(t *testing.T) {
	// generate test cases
	var testCases []testCase
	for _, manifestFile := range sampleManifests {
		testCases = append(testCases, testCase{
			Input:  string(manifestFile),
			Output: loadManifestFromFile(t, manifestFile),
		})
	}

	log := log.NewMockLog()

	// run tests
	for _, test := range testCases {
		// call method
		result, err := parsePackageManifest(log, test.Input)

		// check results
		assert.Nil(t, err)
		assert.Equal(t, test.Output.Name, result.Name)
		assert.Equal(t, test.Output.Version, result.Version)
		assert.Equal(t, test.Output.Install, result.Install)
		assert.Equal(t, test.Output.Uninstall, result.Uninstall)
		assert.Equal(t, test.Output.Launch, result.Launch)
		assert.Equal(t, test.Output.Platform, result.Platform)
		assert.Equal(t, test.Output.Architecture, result.Architecture)
		assert.True(t, test.Output.Reboot == result.Reboot || (test.Output.Reboot == "" && result.Reboot == "false"))
	}
}

//Test ParseManifest with invalid manifest files
func TestParseManifestWithError(t *testing.T) {
	// generate test cases
	var testCases []testCase
	for _, manifestFile := range errorManifests {
		testCases = append(testCases, testCase{
			Input:  string(manifestFile),
			Output: loadManifestFromFile(t, manifestFile),
		})
	}

	log := log.NewMockLog()

	// run tests
	for _, test := range testCases {
		// call method
		result, err := parsePackageManifest(log, test.Input)

		// check results
		assert.NotNil(t, err)
		assert.Equal(t, test.Output, result)
	}
}

//Test ParseManifest with manifest files that cannot be loaded or parsed
func TestParseMalformedManifest(t *testing.T) {
	log := log.NewMockLog()

	// run tests
	for _, manifestFile := range malformedManifests {
		// call method
		result, err := parsePackageManifest(log, string(manifestFile))

		// check results
		assert.NotNil(t, err)
		assert.Nil(t, result)
	}
}

// Load specified file from file system
func loadFile(t *testing.T, fileName string) (result []byte) {
	var err error
	if result, err = ioutil.ReadFile(fileName); err != nil {
		t.Fatal(err)
	}
	return
}

// Parse manifest file
func loadManifestFromFile(t *testing.T, fileName string) (manifest *PackageManifest) {
	b := loadFile(t, fileName)
	if err := json.Unmarshal(b, &manifest); err != nil {
		t.Fatal(err)
	}

	return manifest
}
