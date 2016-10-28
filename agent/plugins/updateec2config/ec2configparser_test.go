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

// +build windows

// Package updateec2config implements the UpdateEC2Config plugin.
package updateec2config

import (
	"encoding/json"
	"io/ioutil"
	"testing"

	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/stretchr/testify/assert"
)

//Valid manifest file
var testManifests = []string{
	"testData/testManifest.json",
}

//Invalid manifest file
var errorManifests = []string{
	"testData/invalidManifest.json",
}

//testCase is a struct depicting a test case
type testCase struct {
	Input  string
	Output *Manifest
}

//TestParseManifest tests the function parse manifest file
func TestParseManifest(t *testing.T) {
	//generate test cases
	var testCases []testCase
	for _, manifestFile := range testManifests {
		testCases = append(testCases, testCase{
			Input:  string(manifestFile),
			Output: loadManifestFromFile(t, manifestFile),
		})
	}
	agentName := EC2ConfigAgentName
	log := log.NewMockLog()

	// run tests
	for _, tst := range testCases {
		// call method
		parsedMsg, err := ParseManifest(log, tst.Input)

		// check results
		assert.Nil(t, err)
		assert.Equal(t, tst.Output, parsedMsg)
		assert.Equal(t, parsedMsg.Packages[0].Name, agentName)
		assert.Equal(
			t,
			parsedMsg.Packages[0].AvailableVersions[0].Version,
			"1.0.0")
		assert.Equal(
			t,
			parsedMsg.Packages[0].AvailableVersions[0].Checksum,
			"f03a4d3393c91cdff03a465232508952ae9bd3d94b78a8ee198402fe828c3830")
	}

}

//Test ParseManifest With Invalid manifest files
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
	for _, tst := range testCases {
		// call method
		parsedMsg, err := ParseManifest(log, tst.Input)

		// check results
		assert.NotNil(t, err)
		assert.Equal(t, tst.Output, parsedMsg)
	}
}

//loadManifestFromFile is a helper function load manifest file
func loadManifestFromFile(t *testing.T, fileName string) (manifest *Manifest) {
	b := loadFile(t, fileName)
	if err := json.Unmarshal(b, &manifest); err != nil {
		t.Fatal(err)
	}

	return manifest
}

//loadfile is a helper function to load specified file from file system
func loadFile(t *testing.T, fileName string) (result []byte) {
	var err error
	if result, err = ioutil.ReadFile(fileName); err != nil {
		t.Fatal(err)
	}
	return
}
