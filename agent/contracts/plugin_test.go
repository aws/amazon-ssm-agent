// Copyright 2016 Amazon.com, Inc. or its affiliates. All Rights Reserved.
//
// Licensed under the Amazon Software License (the "License"). You may not
// use this file except in compliance with the License. A copy of the
// License is located at
//
// http://aws.amazon.com/asl/
//
// or in the "license" file accompanying this file. This file is distributed
// on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND,
// express or implied. See the License for the specific language governing
// permissions and limitations under the License.

package contracts

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

type truncateOutputTest struct {
	stdout   string
	stderr   string
	capacity int
	expected string
}

const (
	sampleSize  = 100
	longMessage = `This is a sample text. This is a sample text. This is a sample text. This is a sample text. This is a sample text.
This is a sample text. This is a sample text. This is a sample text. This is a sample text. This is a sample text.
This is a sample text. This is a sample text. This is a sample text. This is a sample text. This is a sample text.
This is a sample text. This is a sample text. This is a sample text. This is a sample text. This is a sample text.
This is a sample text. This is a sample text. This is a sample text. This is a sample text. This is a sample text.
This is a sample text. This is a sample text. This is a sample text. This is a sample text. This is a sample text.
This is a sample text. This is a sample text. This is a sample text. This is a sample text. This is a sample text.
This is a sample text. This is a sample text. This is a sample text. This is a sample text. This is a sample text.
This is a sample text. This is a sample text. This is a sample text. This is a sample text. This is a sample text.
This is a sample text. This is a sample text. This is a sample text. This is a sample text. This is a sample text.
1234567890. This is a sample text. This is a sample text`
)

var testData = []truncateOutputTest{
	//{stdout, stderr, capacity, expected}
	{"", "", sampleSize, ""},
	{"sample output", "", sampleSize, "sample output"},
	{"", "sample error", sampleSize, "\n----------ERROR-------\nsample error"},
	{"sample output", "sample error", sampleSize, "sample output\n----------ERROR-------\nsample error"},
	{longMessage, "", sampleSize, "This is a sample text. This is a sample text. This is a sample text. This is \n---Output truncated---"},
	{"", longMessage, sampleSize, "\n----------ERROR-------\nThis is a sample text. This is a sample text. This is\n---Error truncated----"},
	{longMessage, longMessage, sampleSize, "This is a sampl\n---Output truncated---\n----------ERROR-------\nThis is a sampl\n---Error truncated----"},
}

func TestTruncateOutput(t *testing.T) {
	for i, test := range testData {
		actual := TruncateOutput(test.stdout, test.stderr, test.capacity)
		assert.Equal(t, test.expected, actual, "failed test case: %v", i)
	}
}
