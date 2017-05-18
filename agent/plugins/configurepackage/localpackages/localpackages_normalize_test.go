// Copyright 2017 Amazon.com, Inc. or its affiliates. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License"). You may not
// use this file except in compliance with the License. A copy of the
// License is located at
//
// http://aws.amazon.com/apache2.0/
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or
// implied. See the License for the specific language governing
// permissions and limitations under the License.

// Package localpackages implements the local storage for packages managed by the ConfigurePackage plugin.
package localpackages

import (
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

const normalizedRegex = "^(\\_[a-zA-Z0-9-]*)(\\_[a-zA-Z0-9_-]+)(====)$"

var normalizedRegExpValidator = regexp.MustCompile(normalizedRegex)

// Normal names should be unchanged by normalization
func TestNormalNames(t *testing.T) {
	names := []string{"AWSPVDriver",
		"AwsEnaNetworkDriver",
		"IntelSriovDriver",
		"a0",
		"Aa",
		"z_._._",
		"0_-_",
		"A",
		"ABCDEFGHIJKLM-NOPQRSTUVWXYZ.abcdefghijklm-nopqrstuvwxyz.1234567890"}
	for _, original := range names {
		normalized := normalizeDirectory(original)
		assert.True(t, strings.EqualFold(original, normalized))
	}
}

// Normal versions should be unchanged by normalization
func TestNormalVersions(t *testing.T) {
	versions := []string{"1.0.0.0.0.0",
		"1.2.-3",
		"00.000.0000.00000",
		"987654321",
		"The .Quick. brown fox . jumped over.  . the lazy dog",
		"8(67[5{309}])",
		"1.2.3-a.b.c.10.d.5",
	}
	for _, original := range versions {
		normalized := normalizeDirectory(original)
		assert.Equal(t, original, normalized)
	}
}

// Abnormal names should be normalized
func TestAbnormalNames(t *testing.T) {
	normalizedSet := make(map[string]bool)
	names := []string{"_foo_1234",
		"fo..\\bar",
		"~" + longString("q", 250),
		longString("Qq&12", 256),
		".",
		".abc",
		"-",
		"foo ",
		" foo",
		"*abc",
		"abc.",
		"abc:",
		"<0abc",
		"~1234",
		"../foo",
		"abc..def"}
	for _, original := range names {
		normalized := normalizeDirectory(original)
		_, collision := normalizedSet[normalized]
		assert.False(t, collision, normalized)
		normalizedSet[normalized] = true
		assert.False(t, strings.EqualFold(original, normalized))
		assert.True(t, len(normalized) <= dirMaxLength)
		assert.True(t, normalizedRegExpValidator.MatchString(normalized), normalized)
	}
	// there should be no collisions between normalized values
	assert.Equal(t, len(names), len(normalizedSet))
	// name normalization is case insensitive
	assert.Equal(t, normalizeDirectory("*FOO"), normalizeDirectory("*foo"))
}

// Abnormal versions should be normalized
func TestAbnormalVersions(t *testing.T) {
	normalizedSet := make(map[string]bool)
	versions := []string{" ",
		"-",
		"-.-",
		"-12.1",
		"1..2",
		"./foo",
		"~1.0.0",
		"1.\\n0.0",
		"1.2\n3.0",
		"1.2\t3.0",
		"1.2.3 ",
		"1.2.3~",
		"1.2.3&nbsp.4",
		"~" + longString("q", 250),
		longString("Qq&12", 256),
		"12>8<25",
		".",
		".abc",
		"foo ",
		"*abc",
		"abc.",
		"abc:",
		"<0abc",
		"~1234",
		"../foo",
		"abc..def"}
	for _, original := range versions {
		normalized := normalizeDirectory(original)
		_, collision := normalizedSet[normalized]
		assert.False(t, collision, normalized)
		normalizedSet[normalized] = true
		assert.NotEqual(t, original, normalized)
		assert.True(t, len(normalized) <= dirMaxLength)
		assert.True(t, normalizedRegExpValidator.MatchString(normalized), normalized)
	}
	// there should be no collisions between normalized values
	assert.Equal(t, len(versions), len(normalizedSet))
	// version normalization is case insensitive
	assert.Equal(t, normalizeDirectory("*FOO"), normalizeDirectory("*foo"))
}

// Test some specific inputs and ensure exact normalization result
func TestDirectoryNameGeneration(t *testing.T) {
	// For the third input, use the output from the first and ensure that if you attempt to create an input
	// that will collide with a normalized output, it will be normalized
	inputs := []string{"The&quick%brown*fox..jumped\\over/the<lazy>dog. ",
		"~" + longString("q", 250),
		"_Thequickbrownfox--jumpedoverthelazydog-_2F_JMPJKOPIFML5RTJLYW6ISVDNN5MFKAI6UHPMDDIT7FL74AKPTEHA===="}
	outputs := []string{"_Thequickbrownfox--jumpedoverthelazydog-_2F_JMPJKOPIFML5RTJLYW6ISVDNN5MFKAI6UHPMDDIT7FL74AKPTEHA====",
		"_qqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqq_FB_JHNSOBBZKOSTJVCXPCXNOGVCARUAX7CAIGHQGBSCD47CJQKNZQWQ====",
		"_Thequickbrownfox--jumpedoverthelazydog-2FJMPJKOPIFML5RTJLYW6ISVDNN5MFKAI6UHPMDDIT7FL74AKPTEHA_64_C5O2YO4FPXBNTB4NGP7EYRLCPNWOER265QO5TZB5RKZJ4PQFK5RQ===="}
	for index, original := range inputs {
		normalized := generateDirectoryName(original)
		assert.Equal(t, outputs[index], normalized)
	}
}

func longString(pattern string, length int) string {
	var result string
	repeats := int(length / len(pattern))
	remainder := length % len(pattern)
	for i := 0; i < repeats; i++ {
		result = result + pattern
	}
	result = result + pattern[0:remainder]
	return result
}
