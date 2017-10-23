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
//
// Package pluginutil implements some common functions shared by multiple plugins.
package pluginutil

import (
	"bytes"
	"strings"
	"testing"

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
	input = 176400
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

func TestReplaceMarkedFields(t *testing.T) {
	identity := func(a string) string { return a }
	replaceWithDummy := func(a string) string { return "dummy" }
	type testCase struct {
		input       string
		startMarker string
		endMarker   string
		replacer    func(string) string
		output      string
	}
	inOut := []testCase{
		{"a<-tom->s", "<-", "->", identity, "atoms"},
		{"a<-tom->s", "<-", "->", replaceWithDummy, "adummys"},
		{"a<>t</>s", "<>", "</>", strings.ToUpper, "aTs"},
		{`a<tom>abc<de>`, "<", ">", strings.ToUpper, `aTOMabcDE`},
		{`|tom|abc|de|`, "|", "|", strings.ToUpper, `TOMabcDE`},
		{"atoms", "[missingMarker]", "[/missingMarker]", strings.ToUpper, "atoms"},
		{"at<start>oms", "<start>", "</missingEnd>", strings.ToUpper, ""}, // error case
	}
	for _, tst := range inOut {
		result, err := ReplaceMarkedFields(tst.input, tst.startMarker, tst.endMarker, tst.replacer)
		if tst.output != "" {
			assert.Equal(t, tst.output, result)
		} else {
			assert.NotNil(t, err)
		}
	}
}

func TestCleanupNewLines(t *testing.T) {
	inOut := [][]string{
		{"ab\nc", "abc"},
		{"\nab\n\rc\n\r", "abc"},
		{"abc\r", "abc"},
		{"a", "a"},
		{"", ""},
	}
	for _, test := range inOut {
		input, output := test[0], test[1]
		result := CleanupNewLines(input)
		assert.Equal(t, output, result)
	}
}

func TestCleanupJSONField(t *testing.T) {
	inOut := [][]string{
		{"a\nb", `a`},
		{"a\tb\nc", `a\tb`},
		{`a\b`, `a\\b`},
		{`a"b`, `a\"b`},
		{`\"b` + "\n", `\\\"b`},
		{"description\non\nmulti\nline", `description`},
		{"a simple text", `a simple text`},
	}
	for _, test := range inOut {
		input, output := test[0], test[1]
		result := CleanupJSONField(input)
		assert.Equal(t, output, result)
	}
}
