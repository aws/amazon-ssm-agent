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

package appconfig

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// getStringValue Tests

type GetStringValueTest struct {
	Input        string
	DefaultValue string
	Output       string
}

var (
	getStringValueTests = []GetStringValueTest{
		{"", "test", "test"},
		{"val", "test", "val"},
	}
)

func TestGetStringValue(t *testing.T) {
	for _, test := range getStringValueTests {
		output := getStringValue(test.Input, test.DefaultValue)
		assert.Equal(t, test.Output, output)
	}
}

//GetDefaultEndpointTests

type GetDefaultEndPointTest struct {
	Region  string
	Service string
	Output  string
}

var (
	getDefaultEndPointTests = []GetDefaultEndPointTest{
		{"", "", ""},
		{"val", "test", ""},
		{"us-east-1", "ssm", ""},
		{"cn-north-1", "ssm", "ssm.cn-north-1.amazonaws.com.cn"},
	}
)

func TestGetDefaultEndPoint(t *testing.T) {
	for _, test := range getDefaultEndPointTests {
		output := GetDefaultEndPoint(test.Region, test.Service)
		assert.Equal(t, test.Output, output)
	}
}

// getNumericValue Tests

type GetNumericValueTest struct {
	Input        int
	MinValue     int
	MaxValue     int
	DefaultValue int
	Output       int
}

var (
	getNumericValueTests = []GetNumericValueTest{
		{0, 10, 100, 50, 50},   // empty
		{1, 10, 100, 50, 50},   // less than min
		{200, 10, 100, 50, 50}, // greater than max
		{20, 10, 100, 50, 20},  // within range
	}
)

func TestGetNumericValue(t *testing.T) {
	for _, test := range getNumericValueTests {
		output := getNumericValue(test.Input, test.MinValue, test.MaxValue, test.DefaultValue)
		assert.Equal(t, test.Output, output)
	}
}

// getNumeric64Value Tests

type GetNumeric64ValueTest struct {
	Input        int64
	MinValue     int64
	MaxValue     int64
	DefaultValue int64
	Output       int64
}

var (
	getNumeric64ValueTests = []GetNumeric64ValueTest{
		{0, 10, 100000000000000000, 50, 50},                                // empty
		{1, 10, 100000000000000000, 50, 50},                                // less than min
		{200000000000000000, 10, 100000000000000000, 50, 50},               // greater than max
		{30000000000000000, 10, 100000000000000000, 50, 30000000000000000}, // within range
	}
)

func TestGetNumeric64Value(t *testing.T) {
	for _, test := range getNumeric64ValueTests {
		output := getNumeric64Value(test.Input, test.MinValue, test.MaxValue, test.DefaultValue)
		assert.Equal(t, test.Output, output)
	}
}
