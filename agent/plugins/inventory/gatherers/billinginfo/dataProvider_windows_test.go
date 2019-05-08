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

// + build windows

package billinginfo

import (
	"fmt"
	"testing"

	"github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/aws/amazon-ssm-agent/agent/plugins/inventory/model"
	"github.com/stretchr/testify/assert"
)

var (
	sampleDataBillingInfo = []string{
		// single billing product id
		`{
			"BillingProductId": "bp-123456"
		}`,
		// Multiple billing product id
		`[
			{
				"BillingProductId": "bp-6ba54002"
			},
			{
				"BillingProductId": "bp-62a5400b"
			}
	    ]`,
		// Market place billing product id
		`[
			{
				"BillingProductId": "89bab4k3h9x4rkojcm2tj8j4l"
			}
	    ]`,
		// Billing product id null case
		`[]`,
	}
)

var sampleDataBillingInfoParsed = [][]model.BillingInfoData{
	{
		{
			BillingProductId: "bp-123456",
		},
	},
	{
		{
			BillingProductId: "bp-6ba54002",
		},
		{
			BillingProductId: "bp-62a5400b",
		},
	},
	{
		{
			BillingProductId: "89bab4k3h9x4rkojcm2tj8j4l",
		},
	},
	nil,
}

// createMockExecutor creates an executor that returns the given stdout values on subsequent invocations.
// If the number of invocations exceeds the number of outputs provided, the executor will return the last output.
// For example createMockExecutor("a", "b", "c") will return an executor that returns the following values:
// on first call -> "a"
// on second call -> "b"
// on third call -> "c"
// on every call after that -> "c"
func createMockExecutor(stdout ...string) func(string, ...string) ([]byte, error) {
	var index = 0
	return func(string, ...string) ([]byte, error) {
		if index < len(stdout) {
			index += 1
		}
		return []byte(stdout[index-1]), nil
	}
}

func MockTestExecutorWithError(command string, args ...string) ([]byte, error) {
	var result []byte
	return result, fmt.Errorf("Random Error")
}

func TestCollectBillingInfoData(t *testing.T) {
	mockContext := context.NewMockDefault()
	for i, sampleBillingInfoData := range sampleDataBillingInfo {
		cmdExecutor = createMockExecutor(sampleBillingInfoData)
		parsedItems := CollectBillingInfoData(mockContext)
		for j := 0; j < len(parsedItems); j++ {
			assert.Equal(t, sampleDataBillingInfoParsed[i][j], parsedItems[j])
		}

		// For nil entry we need to check separately
		if len(parsedItems) == 0 {
			assert.Equal(t, sampleDataBillingInfoParsed[i], parsedItems)
		}
	}
}

func TestCollectBillingInfoDataWithError(t *testing.T) {
	mockContext := context.NewMockDefault()
	cmdExecutor = MockTestExecutorWithError
	parsedItems := CollectBillingInfoData(mockContext)
	assert.Equal(t, len(parsedItems), 0)
}
