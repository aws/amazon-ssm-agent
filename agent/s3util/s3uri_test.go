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
	"net/url"
	"testing"

	"github.com/aws/amazon-ssm-agent/agent/mocks/s3util"
	"github.com/stretchr/testify/assert"
)

type s3BucketTest struct {
	bucket string
	url    string
	output AmazonS3URL
}

var (
	sslTests = []s3BucketTest{
		// {bucket, url, AmazonS3URL{IsValidS3URI, IsPathStyle, Bucket, Key, Region}},
		{"abc", "https://abc.s3.mock-region.amazonaws.com/", AmazonS3URL{true, false, "abc", "", "mock-region"}},
		{"a$b$c", "https://s3.mock-region.amazonaws.com/a%24b%24c", AmazonS3URL{true, true, "a$b$c", "", "mock-region"}},
		{"a.b.c", "https://s3.mock-region.amazonaws.com/a.b.c", AmazonS3URL{true, true, "a.b.c", "", "mock-region"}},
		{"a..bc", "https://s3.mock-region.amazonaws.com/a..bc", AmazonS3URL{true, true, "a..bc", "", "mock-region"}},
		{"a..bc", "https://s3.mock-region.amazonaws.com/a..bc/mykey", AmazonS3URL{true, true, "a..bc", "mykey", "mock-region"}},
		{"a..bc", "https://s3.mock-region.amazonaws.com/a..bc/mykey/mykey", AmazonS3URL{true, true, "a..bc", "mykey/mykey", "mock-region"}},
		{"johnsmith", "http://johnsmith.eu.s3-eu-west-1.amazonaws.com/homepage.html", AmazonS3URL{true, false, "johnsmith.eu", "homepage.html", "eu-west-1"}},
		{"amazon-ssm-us-west-2", "https://s3-us-west-2.amazonaws.com/amazon-ssm-us-west-2/ssm-agent-manifest.json", AmazonS3URL{true, true, "amazon-ssm-us-west-2", "ssm-agent-manifest.json", "us-west-2"}},
		{"amazon-ssm-us-west-2", "https://s3.amazonaws.com/amazon-ssm-us-west-2/ssm-agent-manifest.json", AmazonS3URL{true, true, "amazon-ssm-us-west-2", "ssm-agent-manifest.json", "us-east-1"}},
		{"amazon-ssm-us-west-2", "https://amazon-ssm-us-west-2.s3.amazonaws.com/ssm-agent-manifest.json", AmazonS3URL{true, false, "amazon-ssm-us-west-2", "ssm-agent-manifest.json", "us-east-1"}},
	}

	noSslTests = []s3BucketTest{
		{"a.b.c", "http://a.b.c.s3.mock-region.amazonaws.com/", AmazonS3URL{true, false, "a.b.c", "", "mock-region"}},
		{"a..bc", "http://s3.mock-region.amazonaws.com/a..bc", AmazonS3URL{true, true, "a..bc", "", "mock-region"}},
		{"a..bc", "http://s3.mock-region.amazonaws.com/a..bc/mykey", AmazonS3URL{true, true, "a..bc", "mykey", "mock-region"}},
		{"a..bc", "http://s3.mock-region.amazonaws.com/a..bc/mykey/mykey", AmazonS3URL{true, true, "a..bc", "mykey/mykey", "mock-region"}},
	}

	forcePathTests = []s3BucketTest{
		{"abc", "https://s3.mock-region.amazonaws.com/abc", AmazonS3URL{true, true, "abc", "", "mock-region"}},
		{"a$b$c", "https://s3.mock-region.amazonaws.com/a%24b%24c", AmazonS3URL{true, true, "a$b$c", "", "mock-region"}},
		{"a.b.c", "https://s3.mock-region.amazonaws.com/a.b.c", AmazonS3URL{true, true, "a.b.c", "", "mock-region"}},
		{"a..bc", "https://s3.mock-region.amazonaws.com/a..bc", AmazonS3URL{true, true, "a..bc", "", "mock-region"}},
		{"ssmagent", "https://s3.amazonaws.com/ssmagent/test1%20test2%20test3/stderr.txt", AmazonS3URL{true, true, "ssmagent", "test1 test2 test3/stderr.txt", "us-east-1"}},
	}

	invalidTests = []s3BucketTest{
		{"abc", "https://abcd/pqr/xyz.txt", AmazonS3URL{false, false, "", "", ""}},
	}

	websiteTests = []s3BucketTest{
		{
			"mybucket",
			"http://mybucket.s3-website-us-west-1.amazonaws.com/mykey", // dash-region format
			AmazonS3URL{
				true,
				false,
				"mybucket",
				"mykey",
				"us-west-1",
			},
		},
		{
			"mybucket",
			"http://mybucket.s3-website.us-west-1.amazonaws.com/mykey", // dot-region format
			AmazonS3URL{
				true,
				false,
				"mybucket",
				"mykey",
				"us-west-1",
			},
		},
	}

	// Dualstack works with both virtual-hosted and path-style URLs
	dualstackTests = []s3BucketTest{
		{
			"mybucket",
			"https://mybucket.s3.dualstack.us-west-1.amazonaws.com/mykey",
			AmazonS3URL{
				true,
				false,
				"mybucket",
				"mykey",
				"us-west-1",
			},
		},
		{
			"mybucket",
			"https://s3.dualstack.us-west-1.amazonaws.com/mybucket/mykey",
			AmazonS3URL{
				true,
				true,
				"mybucket",
				"mykey",
				"us-west-1",
			},
		},
	}

	// Transfer acceleration uses global endpoint, so no region in the URL
	accelerateTests = []s3BucketTest{
		{
			"mybucket",
			"https://mybucket.s3-accelerate.amazonaws.com/mykey",
			AmazonS3URL{
				true,
				false,
				"mybucket",
				"mykey",
				"us-east-1",
			},
		},
		{
			"mybucket",
			"https://mybucket.s3-accelerate.dualstack.amazonaws.com/mykey",
			AmazonS3URL{
				true,
				false,
				"mybucket",
				"mykey",
				"us-east-1",
			},
		},
		{
			"mybucket",
			"https://s3-accelerate.amazonaws.com/mybucket/mykey",
			AmazonS3URL{
				true,
				true,
				"mybucket",
				"mykey",
				"us-east-1",
			},
		},
	}

	vpcEndpointTests = []s3BucketTest{
		{
			"mybucket",
			"https://bucket.vpce-05a18c86214d4f28c-6p280e25.s3.us-west-1.vpce.amazonaws.com/mybucket/mykey",
			AmazonS3URL{
				true,
				true,
				"mybucket",
				"mykey",
				"us-west-1",
			},
		},
		{
			"mybucket",
			"https://mybucket.bucket.vpce-07dd6fec74b812c52-2gqlpwuc.s3.us-west-1.vpce.amazonaws.com/mykey",
			AmazonS3URL{
				true,
				false,
				"mybucket",
				"mykey",
				"us-west-1",
			},
		},
		{
			"mybucket",
			"https://bucket.vpce-0e3580b5f3cb40b34-tr39ydlu.s3.cn-northwest-1.vpce.amazonaws.com.cn/mybucket/mykey",
			AmazonS3URL{
				true,
				true,
				"mybucket",
				"mykey",
				"cn-northwest-1",
			},
		},
		{
			"mybucket",
			"https://bucket.vpce-07dd6fec74b812c52-2gqlpwuc.s3.us-gov-west-1.vpce.amazonaws.com/mybucket/mykey",
			AmazonS3URL{
				true,
				true,
				"mybucket",
				"mykey",
				"us-gov-west-1",
			},
		},
	}
)

func runTests(t *testing.T, tests []s3BucketTest) {
	for _, test := range tests {
		fileURL, err := url.Parse(test.url)
		assert.Equal(t, err, nil)
		output := ParseAmazonS3URL(s3util.MockLog, fileURL)
		assert.Equal(t, test.output, output, test.url)
	}
}

func TestHostStyleBucketBuild(t *testing.T) {
	runTests(t, sslTests)
}

func TestHostStyleBucketBuildNoSSL(t *testing.T) {
	runTests(t, noSslTests)
}

func TestPathStyleBucketBuild(t *testing.T) {
	runTests(t, forcePathTests)
}

func TestInValidS3PathStyle(t *testing.T) {
	runTests(t, invalidTests)
}

func TestWebsite(t *testing.T) {
	runTests(t, websiteTests)
}

func TestDualstack(t *testing.T) {
	runTests(t, dualstackTests)
}

func TestAccelerate(t *testing.T) {
	runTests(t, accelerateTests)
}

func TestVpcEndpoint(t *testing.T) {
	runTests(t, vpcEndpointTests)
}
