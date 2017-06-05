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

// Package updatessmagent implements the UpdateSsmAgent plugin.
package updatessmagent

import (
	"encoding/json"
	"io/ioutil"
	"testing"

	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/amazon-ssm-agent/agent/updateutil"
	"github.com/stretchr/testify/assert"
)

//Valid manifest files
var sampleManifests = []string{
	"testdata/sampleManifest.json",
}

//Invalid manifest files
var errorManifests = []string{
	"testdata/errorManifest.json",
}

type testCase struct {
	Input  string
	Output *Manifest
}

//TestParseManifest testing parses valid manifest files
func TestParseManifest(t *testing.T) {
	// generate test cases
	var testCases []testCase
	for _, manifestFile := range sampleManifests {
		testCases = append(testCases, testCase{
			Input:  string(manifestFile),
			Output: loadManifestFromFile(t, manifestFile),
		})
	}
	agentName := "amazon-ssm-agent"
	log := log.NewMockLog()
	context := mockInstanceContext()
	// run tests
	for _, tst := range testCases {
		// call method
		parsedMsg, err := ParseManifest(log, tst.Input, context, agentName)

		// check results
		assert.Nil(t, err)
		assert.Equal(t, tst.Output, parsedMsg)
		assert.Equal(t, parsedMsg.Packages[0].Name, agentName)
		assert.Equal(t, parsedMsg.Packages[0].Files[0].Name, "amazon-ssm-agent-linux-amd64.tar.gz")
		assert.Equal(
			t,
			parsedMsg.Packages[0].Files[0].AvailableVersions[0].Version,
			"1.0.178.0")
		assert.Equal(
			t,
			parsedMsg.Packages[0].Files[0].AvailableVersions[0].Checksum,
			"d2b67b804e0c3d3d83d09992a6a62b9e6a79fa3214b00685b07998b4e548870e")
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
	agentName := "amazon-ssm-agent"
	log := log.NewMockLog()
	context := mockInstanceContext()
	// run tests
	for _, tst := range testCases {
		// call method
		parsedMsg, err := ParseManifest(log, tst.Input, context, agentName)

		// check results
		assert.NotNil(t, err)
		assert.Equal(t, tst.Output, parsedMsg)
	}
}

//Load specified file from file system
func loadFile(t *testing.T, fileName string) (result []byte) {
	var err error
	if result, err = ioutil.ReadFile(fileName); err != nil {
		t.Fatal(err)
	}
	return
}

//Parse manifest file
func loadManifestFromFile(t *testing.T, fileName string) (manifest *Manifest) {
	b := loadFile(t, fileName)
	if err := json.Unmarshal(b, &manifest); err != nil {
		t.Fatal(err)
	}

	return manifest
}

func mockInstanceContext() *updateutil.InstanceContext {
	return &updateutil.InstanceContext{
		Region:         "us-east-1",
		Platform:       "linux",
		InstallerName:  "linux",
		Arch:           "amd64",
		CompressFormat: "tar.gz",
	}
}
