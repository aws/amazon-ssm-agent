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

// Package documentarchive contains the struct that is called when the package information is stored in birdwatcher
package documentarchive

import (
	"testing"

	"github.com/aws/amazon-ssm-agent/agent/plugins/configurepackage/birdwatcher/facade"

	"github.com/stretchr/testify/assert"
)

func TestGetResourceVersion(t *testing.T) {

	packageName := "Test Package"
	version1 := ""
	version2 := "1.2.3.4"
	latest := "latest"

	data := []struct {
		name         string
		packagename  string
		version      string
		facadeClient facade.FacadeMock
	}{
		{
			"ValidDistributionRule",
			packageName,
			latest,
			facade.FacadeMock{},
		},
		{
			"ValidDistributionRule_2",
			packageName,
			version1,
			facade.FacadeMock{},
		},
		{

			"ValidDistributionRule_3",
			packageName,
			version2,
			facade.FacadeMock{},
		},
	}

	for _, testdata := range data {
		t.Run(testdata.name, func(t *testing.T) {

			bwArchive := New(&testdata.facadeClient)

			names, versions := bwArchive.GetResourceVersion(testdata.packagename, testdata.version)
			assert.Equal(t, names, testdata.packagename)
			assert.Equal(t, versions, testdata.version)
		})
	}
}
