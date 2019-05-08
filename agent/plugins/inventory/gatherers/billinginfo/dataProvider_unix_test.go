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

// +build darwin freebsd linux netbsd openbsd

package billinginfo

import (
	"fmt"
	"testing"

	"github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/aws/amazon-ssm-agent/agent/plugins/inventory/model"
	"github.com/stretchr/testify/assert"
)

var (
	sampleDataUnix = []string{
		// Single LicenseIncluded billing product id
		`{ 
 		  "devpayProductCodes" : null,
 		  "marketplaceProductCodes" : null,
 		  "version" : "2017-09-30",
 		  "instanceType" : "t2.micro",
 		  "billingProducts" : [ "bp-123456" ],
 		  "instanceId" : "i-06e5f0529669dc1e6",
 		  "imageId" : "ami-0de53d8956e8dcf80",
 		  "pendingTime" : "2019-04-03T18:56:09Z",
 		  "availabilityZone" : "us-east-1d",
 		  "kernelId" : null,
 		  "ramdiskId" : null,
 		  "architecture" : "x86_64",
 		  "privateIp" : "172.31.82.182",
 		  "region" : "us-east-1"
 		 }`,
		// Multiple LicenseIncluded billing product id
		`{ 
 		  "devpayProductCodes" : null,
 		  "marketplaceProductCodes" : null,
 		  "version" : "2017-09-30",
 		  "instanceType" : "t2.micro",
 		  "billingProducts" : [ "bp-878787", "bp-23478" ],
 		  "instanceId" : "i-06e5f0529669dc1e6",
 		  "imageId" : "ami-0de53d8956e8dcf80",
 		  "pendingTime" : "2019-04-03T18:56:09Z",
 		  "availabilityZone" : "us-east-1d",
 		  "kernelId" : null,
 		  "ramdiskId" : null,
 		  "architecture" : "x86_64",
 		  "privateIp" : "172.31.82.182",
 		  "region" : "us-east-1"
 		 }`,
		// Marketplace product id
		`{ 
 		  "devpayProductCodes" : null,
 		  "marketplaceProductCodes" : [ "89bab4k3h9x4rkojcm2tj8j4l" ],
 		  "version" : "2017-09-30",
 		  "instanceType" : "t2.micro",
 		  "billingProducts" : null,
 		  "instanceId" : "i-06e5f0529669dc1e6",
 		  "imageId" : "ami-0de53d8956e8dcf80",
 		  "pendingTime" : "2019-04-03T18:56:09Z",
 		  "availabilityZone" : "us-east-1d",
 		  "kernelId" : null,
 		  "ramdiskId" : null,
 		  "architecture" : "x86_64",
 		  "privateIp" : "172.31.82.182",
 		  "region" : "us-east-1"
 		 }`,
		// Both LicenseIncluded Marketplace product ids present
		`{ 
 		  "devpayProductCodes" : null,
 		  "marketplaceProductCodes" : [ "89bab4k3h9x4rkojcm2tj8j4l" ],
 		  "version" : "2017-09-30",
 		  "instanceType" : "t2.micro",
 		  "billingProducts" : [ "bp-123456" ],
 		  "instanceId" : "i-06e5f0529669dc1e6",
 		  "imageId" : "ami-0de53d8956e8dcf80",
 		  "pendingTime" : "2019-04-03T18:56:09Z",
 		  "availabilityZone" : "us-east-1d",
 		  "kernelId" : null,
 		  "ramdiskId" : null,
 		  "architecture" : "x86_64",
 		  "privateIp" : "172.31.82.182",
 		  "region" : "us-east-1"
 		 }`,
		// Null LicenseIncluded/Marketplace billing product id
		`{ 
 		  "devpayProductCodes" : null,
 		  "marketplaceProductCodes" : null,
 		  "version" : "2017-09-30",
 		  "instanceType" : "t2.micro",
 		  "billingProducts" : null,
 		  "instanceId" : "i-06e5f0529669dc1e6",
 		  "imageId" : "ami-0de53d8956e8dcf80",
 		  "pendingTime" : "2019-04-03T18:56:09Z",
 		  "availabilityZone" : "us-east-1d",
 		  "kernelId" : null,
 		  "ramdiskId" : null,
 		  "architecture" : "x86_64",
 		  "privateIp" : "172.31.82.182",
 		  "region" : "us-east-1"
 		 }`,
	}
)

var sampleDataUnixParsed = [][]model.BillingInfoData{
	{
		{
			BillingProductId: "bp-123456",
		},
	},
	{
		{
			BillingProductId: "bp-878787",
		},
		{
			BillingProductId: "bp-23478",
		},
	},
	{
		{
			BillingProductId: "89bab4k3h9x4rkojcm2tj8j4l",
		},
	},
	{
		{
			BillingProductId: "bp-123456",
		},
		{
			BillingProductId: "89bab4k3h9x4rkojcm2tj8j4l",
		},
	},
	//  sample data for null billing product id.
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

func TestParseCurlOutput(t *testing.T) {
	mockContext := context.NewMockDefault()
	for i, sampleData := range sampleDataUnix {
		parsedItems := parseCurlOutput(mockContext, sampleData)
		for j := 0; j < len(parsedItems); j++ {
			assert.Equal(t, sampleDataUnixParsed[i][j], parsedItems[j])
		}
		// For nil entry we need to check separately
		if len(parsedItems) == 0 {
			assert.Equal(t, sampleDataUnixParsed[i], parsedItems)
		}
	}
}

func TestCollectBillingInfoData(t *testing.T) {
	mockContext := context.NewMockDefault()
	for i, sampleData := range sampleDataUnix {
		cmdExecutor = createMockExecutor(sampleData)
		parsedItems := CollectBillingInfoData(mockContext)
		for j := 0; j < len(parsedItems); j++ {
			assert.Equal(t, sampleDataUnixParsed[i][j], parsedItems[j])
		}
		// For nil entry we need to check separately
		if len(parsedItems) == 0 {
			assert.Equal(t, sampleDataUnixParsed[i], parsedItems)
		}
	}

	cmdExecutor = MockTestExecutorWithError
	parsedItems := CollectBillingInfoData(mockContext)
	assert.Equal(t, len(parsedItems), 0)
}
