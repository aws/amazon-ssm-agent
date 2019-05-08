// Copyright 2019 Amazon.com, Inc. or its affiliates. All Rights Reserved.
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

package billinginfo

import (
	"testing"

	"github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/aws/amazon-ssm-agent/agent/plugins/inventory/model"
	"github.com/stretchr/testify/assert"
)

var testBillingInfo = []model.BillingInfoData{
	{
		BillingProductId: "bp-1231231",
	},
	{
		BillingProductId: "bp-1231",
	},
}

func testCollectBillingInfoData(context context.T) (data []model.BillingInfoData) {
	return testBillingInfo
}

func TestGatherer(t *testing.T) {
	contextMock := context.NewMockDefault()
	gatherer := Gatherer(contextMock)
	collectData = testCollectBillingInfoData
	item, err := gatherer.Run(contextMock, model.Config{})
	assert.Nil(t, err)
	assert.Equal(t, 1, len(item))
	assert.Equal(t, GathererName, item[0].Name)
	assert.Equal(t, SchemaVersionOfBillingInfo, item[0].SchemaVersion)
	assert.Equal(t, testBillingInfo, item[0].Content)
}
