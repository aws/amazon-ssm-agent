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

package contracts

import (
	"fmt"
	"testing"

	"github.com/aws/amazon-ssm-agent/agent/log"
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

var logger = log.NewMockLog()

func TestSucceeded(t *testing.T) {
	output := PluginOutput{}

	output.MarkAsSucceeded()

	assert.Equal(t, output.ExitCode, 0)
	assert.Equal(t, output.Status, ResultStatusSuccess)
}

func TestFailed(t *testing.T) {
	output := PluginOutput{}

	output.MarkAsFailed(logger, fmt.Errorf("Error message"))

	assert.Equal(t, output.ExitCode, 1)
	assert.Equal(t, output.Status, ResultStatusFailed)
	assert.Contains(t, output.Stderr, "Error message")
}

func TestPending(t *testing.T) {
	output := PluginOutput{}

	output.MarkAsInProgress()

	assert.Equal(t, output.ExitCode, 0)
	assert.Equal(t, output.Status, ResultStatusInProgress)
}

func TestAppendInfo(t *testing.T) {
	output := PluginOutput{}

	output.AppendInfo(logger, "Info message")
	output.AppendInfo(logger, "Second entry")

	assert.Contains(t, output.Stdout, "Info message")
	assert.Contains(t, output.Stdout, "Second entry")
}

func TestAppendSpecialChars(t *testing.T) {
	output := PluginOutput{}

	var testString = "%v`~!@#$%^&*()-_=+[{]}|\\;:'\",<.>/?"
	output.AppendInfo(logger, testString)
	output.AppendError(logger, testString)

	assert.Contains(t, output.Stdout, testString)
	assert.Contains(t, output.Stderr, testString)
}

func TestAppendFormat(t *testing.T) {
	output := PluginOutput{}

	var testString = "%v`~!@#$%^&*()-_=+[{]}|\\;:'\",<.>/?%%"

	// The first % is a %v - a variable to be replaced and we provided a value for it.
	// The second % isn't escaped and is treated as a fmt parameter, but no value is provided for it.
	// The double %% is an escaped single literal %.
	var testStringFormatted = "foo`~!@#$%!^(MISSING)&*()-_=+[{]}|\\;:'\",<.>/?%"
	output.AppendInfof(logger, testString, "foo")
	output.AppendErrorf(logger, testString, "foo")

	assert.Contains(t, output.Stdout, testStringFormatted)
	assert.Contains(t, output.Stderr, testStringFormatted)
}

func TestIsCrossPlatformEnabledForSchema20(t *testing.T) {
	var schemaVersion = "2.0"
	isCrossPlatformEnabled := IsPreconditionEnabled(schemaVersion)

	// isCrossPlatformEnabled should be false for 2.0 document
	assert.False(t, isCrossPlatformEnabled)
}

func TestIsCrossPlatformEnabledForSchema21(t *testing.T) {
	var schemaVersion = "2.1"
	isCrossPlatformEnabled := IsPreconditionEnabled(schemaVersion)

	// isCrossPlatformEnabled should be true for 2.1 document
	assert.True(t, isCrossPlatformEnabled)
}
