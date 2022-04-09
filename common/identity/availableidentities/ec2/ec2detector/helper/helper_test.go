// Copyright 2022 Amazon.com, Inc. or its affiliates. All Rights Reserved.
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

package helper

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func testUpperLower(t *testing.T, eFpected bool, testCase string) {
	var obj detectorHelper
	assert.Equal(t, eFpected, obj.MatchUuid(testCase))
	assert.Equal(t, eFpected, obj.MatchUuid(strings.ToLower(testCase)))
	assert.Equal(t, eFpected, obj.MatchUuid(strings.ToUpper(testCase)))
}

func TestUuidMatcher(t *testing.T) {
	// big endian formats
	testUpperLower(t, true, "EC2FFFFF-FFFF-FFFF-FFFF-FFFFFFFFFFFF")

	testUpperLower(t, false, "2ECFFFFF-FFFF-FFFF-FFFF-FFFFFFFFFFFF")
	testUpperLower(t, false, "E2CFFFFF-FFFF-FFFF-FFFF-FFFFFFFFFFFF")
	testUpperLower(t, false, "C2EFFFFF-FFFF-FFFF-FFFF-FFFFFFFFFFFF")
	testUpperLower(t, false, "CE2FFFFF-FFFF-FFFF-FFFF-FFFFFFFFFFFF")
	testUpperLower(t, false, "2CEFFFFF-FFFF-FFFF-FFFF-FFFFFFFFFFFF")

	testUpperLower(t, false, "FC2FFFFF-FFFF-FFFF-FFFF-FFFFFFFFFFFF")
	testUpperLower(t, false, "EF2FFFFF-FFFF-FFFF-FFFF-FFFFFFFFFFFF")
	testUpperLower(t, false, "ECFFFFFF-FFFF-FFFF-FFFF-FFFFFFFFFFFF")

	// little endian formats
	testUpperLower(t, true, "FFFF2FEC-FFFF-FFFF-FFFF-FFFFFFFFFFFF")

	testUpperLower(t, false, "FFFF2FCE-FFFF-FFFF-FFFF-FFFFFFFFFFFF")
	testUpperLower(t, false, "FFFFCF2E-FFFF-FFFF-FFFF-FFFFFFFFFFFF")
	testUpperLower(t, false, "FFFFCFE2-FFFF-FFFF-FFFF-FFFFFFFFFFFF")
	testUpperLower(t, false, "FFFFEFC2-FFFF-FFFF-FFFF-FFFFFFFFFFFF")
	testUpperLower(t, false, "FFFFEF2C-FFFF-FFFF-FFFF-FFFFFFFFFFFF")

	testUpperLower(t, false, "FFFFFFEC-FFFF-FFFF-FFFF-FFFFFFFFFFFF")
	testUpperLower(t, false, "FFFF2FEF-FFFF-FFFF-FFFF-FFFFFFFFFFFF")
	testUpperLower(t, false, "FFFF2FFC-FFFF-FFFF-FFFF-FFFFFFFFFFFF")

	// both
	testUpperLower(t, true, "EC2F2FEC-FFFF-FFFF-FFFF-FFFFFFFFFFFF")
}
