// Copyright 2016 Amazon.com, Inc. or its affiliates. All Rights Reserved.
//
// Licensed under the AWS Customer Agreement (the "License"). You may
// not use this file except in compliance with the License. A copy of the
// License is located at
//
// http://aws.amazon.com/agreement/
//
// Package pluginutil implements some common functions shared by multiple plugins.
package pluginutil

import (
	"bytes"
	"fmt"
	"strings"
	"testing"

	"github.com/aws/amazon-ssm-agent/agent/contracts"
	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// TestReadPrefix tests that readPrefix works correctly.
func TestReadPrefix(t *testing.T) {
	inputs := []string{"a string to truncate", ""}
	suffix := "-z-"

	for _, input := range inputs {
		testReadPrefix(t, input, suffix, true)
		testReadPrefix(t, input, suffix, false)
	}
}

func testReadPrefix(t *testing.T, input string, suffix string, truncate bool) {
	// setup inputs
	var maxLength int
	if truncate {
		maxLength = len(input) / 2
	} else {
		maxLength = len(input) + 1
	}
	reader := bytes.NewReader([]byte(input))

	// call method under test
	output, err := ReadPrefix(reader, maxLength, suffix)

	// test results
	assert.Nil(t, err)
	if truncate {
		testTruncatedString(t, input, output, maxLength, suffix)
	} else {
		assert.Equal(t, input, output)
	}
}

// testTruncatedString tests that truncated is obtained from original, truncated has the expected length and ends with the expected suffix.
func testTruncatedString(t *testing.T, original string, truncated string, truncatedLength int, truncatedSuffix string) {
	assert.Equal(t, truncatedLength, len(truncated))
	if truncatedLength >= len(truncatedSuffix) {
		// enough room to fit the suffix
		assert.True(t, strings.HasSuffix(truncated, truncatedSuffix))
		assert.True(t, strings.HasPrefix(original, truncated[:truncatedLength-len(truncatedSuffix)]))
	} else {
		// suffix doesn't fir, expect a prefix of the suffix
		assert.Equal(t, truncated, truncatedSuffix[:truncatedLength])
	}
}

func TestOutputTruncation(t *testing.T) {
	out := contracts.PluginOutput{
		Stdout:   "standard output of test case",
		Stderr:   "standard error of test case",
		ExitCode: 0,
		Status:   "Success",
	}
	response := contracts.TruncateOutput(out.Stdout, out.Stderr, 200)
	fmt.Printf("response=\n%v\n", response)
	assert.Equal(t, out.String(), response)

}

func TestValidateExecutionTimeout(t *testing.T) {
	logger := log.NewMockLog()
	logger.On("Error", mock.Anything).Return(nil)
	logger.On("Info", mock.Anything).Return(nil)

	// Run tests
	var input interface{}
	var num int

	// Check with a value less than min value
	input = 3
	num = ValidateExecutionTimeout(logger, input)
	assert.Equal(t, defaultExecutionTimeoutInSeconds, num)

	// Check with a value more than max value
	input = 28900
	num = ValidateExecutionTimeout(logger, input)
	assert.Equal(t, defaultExecutionTimeoutInSeconds, num)

	// Check with a float64 value
	input = 5.0
	num = ValidateExecutionTimeout(logger, input)
	assert.Equal(t, 5, num)

	// Check with int in a string
	input = "10"
	num = ValidateExecutionTimeout(logger, input)
	assert.Equal(t, 10, num)

	// Check with float64 in a string
	input = "10.5"
	num = ValidateExecutionTimeout(logger, input)
	assert.Equal(t, 10, num)

	// Check with character string
	input = "test"
	num = ValidateExecutionTimeout(logger, input)
	assert.Equal(t, defaultExecutionTimeoutInSeconds, num)
}

func TestGetProxySetting(t *testing.T) {
	var input []string
	var outUrl, outNoProxy string

	defaultProxyUrl := "hostname:port"
	defaultNoProxy := "169.254.169.254"

	// Check Environment contains both http_proxy=hostname:port and no_proxy=169.254.169.254 values
	input = []string{"http_proxy=hostname:port", "no_proxy=169.254.169.254"}
	outUrl, outNoProxy = GetProxySetting(input)
	assert.Equal(t, defaultProxyUrl, outUrl)
	assert.Equal(t, defaultNoProxy, outNoProxy)

	// Check Environment contains only http_proxy=hostname:port value
	input = []string{"http_proxy=hostname:port"}
	outUrl, outNoProxy = GetProxySetting(input)
	assert.Equal(t, defaultProxyUrl, outUrl)
	assert.Equal(t, "", outNoProxy)
}
