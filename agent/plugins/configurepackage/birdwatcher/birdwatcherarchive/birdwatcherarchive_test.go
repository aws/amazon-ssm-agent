// Copyright 2018 Amazon.com, Inc. or its affiliates. All Rights Reserved.
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

// Package birdwatcherarchive contains the struct that is called when the package information is stored in birdwatcher
package birdwatcherarchive

import (
	"testing"

	"github.com/aws/amazon-ssm-agent/agent/plugins/configurepackage/birdwatcher/facade"

	"github.com/stretchr/testify/assert"
)

func TestGetPackageArnAndVersion(t *testing.T) {

	data := []struct {
		name    string
		version string
	}{
		{
			"PVDriver",
			"latest",
		},
		{
			"PVDriver",
			"",
		},
		{
			"PVDriver",
			"1.2.3.4",
		},
	}

	for _, testdata := range data {
		t.Run(testdata.name, func(t *testing.T) {

			mockBWFacade := facade.FacadeMock{}

			bwArchive := New(&mockBWFacade)

			names, versions := bwArchive.GetResourceVersion(testdata.name, testdata.version)
			assert.Equal(t, names[0], testdata.name)
			if testdata.version == "" {
				assert.Equal(t, versions[0], "latest")
			} else {
				assert.Equal(t, versions[0], testdata.version)
			}

		})
	}
}
