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

package s3util

import (
	"testing"

	"github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/stretchr/testify/assert"
)

type s3EndpointTest struct {
	region string
	output string
}

var (
	getFallbackS3EndpointTests = []s3EndpointTest{
		// {region, output},
		{"us-east-1", "s3.amazonaws.com"},
		{"us-west-1", "s3.amazonaws.com"},
		{"af-south-1", "s3.amazonaws.com"},
		{"eu-south-1", "s3.amazonaws.com"},
		{"ap-southeast-2", "s3.amazonaws.com"},
		{"us-gov-east-1", "s3.us-gov-west-1.amazonaws.com"},
		{"us-gov-west-1", "s3.us-gov-east-1.amazonaws.com"},
		{"cn-north-1", "s3.cn-northwest-1.amazonaws.com.cn"},
		{"cn-northwest-1", "s3.cn-north-1.amazonaws.com.cn"},
	}
)

func TestGetFallbackS3Endpoint(t *testing.T) {
	for _, test := range getFallbackS3EndpointTests {
		output := getFallbackS3Endpoint(context.NewMockDefault(), test.region)
		assert.Equal(t, test.output, output, "The two urls should be the same")
	}
}
