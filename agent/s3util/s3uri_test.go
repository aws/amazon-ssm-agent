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
)

func runTests(t *testing.T, tests []s3BucketTest) {
	for _, test := range tests {
		fileURL, err := url.Parse(test.url)
		assert.Equal(t, err, nil)
		output := ParseAmazonS3URL(logger, fileURL)
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
