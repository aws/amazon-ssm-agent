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

package instancedetailedinformation

import (
	"testing"

	"github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/aws/amazon-ssm-agent/agent/plugins/inventory/model"
	"github.com/stretchr/testify/assert"
)

func DataGenerator(context context.T) []model.InstanceDetailedInformation {
	return []model.InstanceDetailedInformation{
		{
			CPUModel:              "Intel(R) Xeon(R) CPU E5-2686 v4 @ 2.30GHz",
			CPUSpeedMHz:           "1772",
			CPUs:                  "64",
			CPUSockets:            "2",
			CPUCores:              "32",
			CPUHyperThreadEnabled: "true",
		},
	}
}

func TestGatherer(t *testing.T) {
	c := context.NewMockDefault()
	g := Gatherer(c)
	collectData = DataGenerator
	items, err := g.Run(c, model.Config{})
	assert.Nil(t, err, "Unexpected error thrown")
	assert.Equal(t, 1, len(items))
	assert.Equal(t, items[0].Name, g.Name())
	assert.Equal(t, items[0].SchemaVersion, SchemaVersion)
	assert.Equal(t, items[0].Content, DataGenerator(c))
	assert.NotNil(t, items[0].CaptureTime)
}
