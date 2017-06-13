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
	"testing"

	"github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/aws/amazon-ssm-agent/agent/plugins/inventory/model"
	"github.com/stretchr/testify/assert"
)

func DataGenerator(context context.T) (appData []model.ApplicationData) {
	return []model.ApplicationData{
		{
			ApplicationType: "System Environment/Libraries",
			InstalledTime:   "1461974300",
			Architecture:    "x86_64",
			Version:         "3.16.2.3",
			Publisher:       "Amazon.com",
			URL:             "http://www.mozilla.org/projects/security/pki/nss/",
			Name:            "nss-softokn",
		},
		{
			ApplicationType: "System Environment/Base",
			InstalledTime:   "1461974291",
			Architecture:    "noarch",
			Version:         "10.0",
			Publisher:       "Amazon.com",
			URL:             "(none)",
			Name:            "basesystem",
		},
	}
}

func TestGatherer(t *testing.T) {
	c := context.NewMockDefault()
	g := Gatherer(c)
	collectData = DataGenerator
	items, err := g.Run(c, model.Config{})
	assert.Nil(t, err, "Unexpected error thrown")
	assert.Equal(t, 1, len(items), "ApplicationGatherer always returns 1 inventory type data - which is why number of entries must be 1.")
	item := items[0]
	assert.Equal(t, GathererName, item.Name)
	assert.Equal(t, SchemaVersionOfApplication, item.SchemaVersion)
	assert.Equal(t, collectData(c), item.Content)
	assert.NotNil(t, item.CaptureTime)
}
