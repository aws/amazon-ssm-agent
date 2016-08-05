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

package times

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func ExampleToIso8601UTC() {
	fmt.Println(ToIso8601UTC(time.Date(2015, 6, 30, 0, 29, 4, 569000000, time.UTC)))
	// Output: 2015-06-30T00:29:04.569Z
}

func ExampleToIso8601UTC_locale() {
	mst := time.FixedZone("MST", -7*3600) // seven hours west of UTC
	fmt.Println(ToIso8601UTC(time.Date(2015, 6, 3, 0, 9, 4, 569100000, mst)))
	// Output: 2015-06-03T07:09:04.569Z
}

func ExampleToIsoDashUTC() {
	fmt.Println(ToIsoDashUTC(time.Date(2015, 6, 30, 0, 29, 3, 148000000, time.UTC)))
	// Output: 2015-06-30T00-29-03.148Z
}

func TestParseIso8601UTC(t *testing.T) {
	date := ParseIso8601UTC("2015-06-30T00:29:04.569Z")
	assert.Equal(t, date, time.Date(2015, 6, 30, 0, 29, 4, 569000000, time.UTC))
}

func TestToIso8601UTC(t *testing.T) {
	expected := "0001-01-01T00:00:00.000Z"
	var tm0 time.Time
	found := ToIso8601UTC(tm0)
	assert.Equal(t, expected, found)
}

func TestToIsoDashUTC(t *testing.T) {
	expected := "0001-01-01T00-00-00.000Z"
	var tm0 time.Time
	found := ToIsoDashUTC(tm0)
	assert.Equal(t, expected, found)
}
