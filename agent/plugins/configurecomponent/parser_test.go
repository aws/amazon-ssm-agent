// Copyright 2016 Amazon.com, Inc. or its affiliates. All Rights Reserved.
//
// Licensed under the Amazon Software License (the "License"). You may not
// use this file except in compliance with the License. A copy of the
// License is located at
//
// http://aws.amazon.com/asl/
//
// or in the "license" file accompanying this file. This file is distributed
// on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND,
// express or implied. See the License for the specific language governing
// permissions and limitations under the License.

// Package configurecomponent implements the ConfigureComponent plugin.
package configurecomponent

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
}

// Invalid manifest files
var errorManifests = []string{
	"testdata/errorManifest_empty.json",
	"testdata/errorManifest_reboot.json",
}

type testCase struct {
	Input  string
	Output *ComponentManifest
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
		result, err := ParseComponentManifest(log, test.Input)

		// check results
		assert.Nil(t, err)
		assert.Equal(t, test.Output, result)
		assert.Equal(t, result.Name, "PVDriver")
		assert.Equal(t, result.Version, "1.0.0")
		assert.Equal(t, result.Install, "AWSPVDriverSetup.msi /quiet /update")
		assert.Equal(t, result.Uninstall, "AWSPVDriverSetup.msi /quiet /uninstall")
		assert.Equal(t, result.Reboot, "true")
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
		result, err := ParseComponentManifest(log, test.Input)

		// check results
		assert.NotNil(t, err)
		assert.Equal(t, test.Output, result)
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
func loadManifestFromFile(t *testing.T, fileName string) (manifest *ComponentManifest) {
	b := loadFile(t, fileName)
	if err := json.Unmarshal(b, &manifest); err != nil {
		t.Fatal(err)
	}

	return manifest
}
