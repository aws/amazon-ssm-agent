package versionutil

import (
	"fmt"
	"sort"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCompareSemVer(t *testing.T) {
	assert.True(t, Compare("0.0.0", "0.0.0-foo", false) > 0)
	assert.True(t, Compare("1.2.3", "1.2.3-4", false) > 0)
	assert.True(t, Compare("1.2.3-a.b.c.10.d.5", "1.2.3-a.b.c.5.d.100", false) > 0)
	assert.True(t, Compare("3.0.0+foo", "2.9.9", false) > 0)
	assert.True(t, Compare("1.0.0", "1.0.0-rc.1", false) > 0)
	assert.True(t, Compare("3.0.0-foo+bar", "3.0.0-bar+foo", false) > 0)

	assert.Equal(t, 0, Compare("3.0.0+foo", "3.0.0", false))
	assert.Equal(t, 0, Compare("3.0.0+foo", "3.0.0+bar", false))
	assert.Equal(t, 0, Compare("3.0.0+foo-bar", "3.0.0+bar-foo", false))

	// SemVer and non-SemVer compliant versions
	assert.True(t, Compare("3.0.0+foo", "3.0", false) > 0)
	assert.True(t, Compare("3.0.0+foo", "3.0.0.1", false) > 0)
}

func TestCompareVersion(t *testing.T) {
	assert.True(t, Compare("2", "10", false) < 0)
	assert.True(t, Compare("a", "z", false) < 0)
	assert.True(t, Compare("1.0.0", "2.0.0", false) < 0)
	assert.True(t, Compare("1.0.0", "1.1.0", false) < 0)
	assert.True(t, Compare("1.0.0", "1.0.1", false) < 0)
	assert.True(t, Compare("1.0.0", "1.0.0.1", false) < 0)
	assert.True(t, Compare("1.0.0", "1.0.0.0", true) < 0)
	assert.True(t, Compare("1.1.1", "1.2.0", false) < 0)
	assert.True(t, Compare("1.0.0", "1.0.a", false) < 0)

	assert.True(t, Compare("10", "2", false) > 0)
	assert.True(t, Compare("z", "a", false) > 0)
	assert.True(t, Compare("1.0.a", "1.0.A", false) > 0)
	assert.True(t, Compare("1.1.0", "1.0.1", false) > 0)
	assert.True(t, Compare("1.1", "1.-1", false) > 0)
	assert.True(t, Compare("1.10", "1.9", false) > 0)
	assert.True(t, Compare("1.0.1", "1.0", false) > 0)
	assert.True(t, Compare("1.0.0", "1.0", true) > 0)

	assert.Equal(t, 0, Compare("1.0.002", "1.0.2", false))
	assert.Equal(t, 0, Compare("1.0.0", "1..0", false))
	assert.Equal(t, 0, Compare("a.01.b", "a.1.b", false))
	assert.Equal(t, 0, Compare("1.0.1", "1.0.1.0", false))
	assert.Equal(t, 0, Compare("1.0.0", "1.0", false))
	assert.Equal(t, 0, Compare("1.0", "1", false))
	assert.Equal(t, 0, Compare("0", "00.00.00", false))
}

func TestNormalizeForCompare(t *testing.T) {
	assert.Equal(t, "asdf", normalizeForCompare("asdf.0.00.000"))
	assert.Equal(t, "asdf.100", normalizeForCompare("asdf.100"))
	assert.Equal(t, "asdf.100", normalizeForCompare("asdf.100."))
	assert.Equal(t, "1", normalizeForCompare("1.0.0"))
	assert.Equal(t, "", normalizeForCompare("0.0"))
	assert.Equal(t, "", normalizeForCompare(""))
}

func TestSort(t *testing.T) {
	actual := []string{"4.0", "4.0.1", "3.7", "4.0", "3.8", "2.0.1+asdf.qwerty"}
	expected := []string{"2.0.1+asdf.qwerty", "3.7", "3.8", "4.0", "4.0", "4.0.1"}
	sort.Sort(ByVersion(actual))
	assert.Equal(t, actual, expected)
}

// TestVersionStringCompare tests version string comparison
func TestVersionStringCompare(t *testing.T) {
	testCases := []struct {
		a      string
		b      string
		result int
	}{
		{"0", "1.0.152.0", -1},
		{"0.0.1.0", "1.0.152.0", -1},
		{"1.05.00.0156", "1.0.221.9289", 1},
		{"2.05.1", "1.3234.221.9289", 1},
		{"1", "1.0.1", -1},
		{"1.0.1", "1.0.2", -1},
		{"1.0.2", "1.0.3", -1},
		{"1.0.3", "1.1", -1},
		{"1.1", "1.1.1", -1},
		{"1.1.0", "1.0.152.0", 1},
		{"1.1.45", "1.0.152.0", 1},
		{"1.1.1", "1.1.2", -1},
		{"1.1.2", "1.2", -1},
		{"1.1.2", "1.1.2", 0},
		{"2.1.2", "2.1.2", 0},
		{"7.1", "7", 1},
	}

	for _, test := range testCases {
		compareResult, err := VersionCompare(test.a, test.b)
		assert.NoError(t, err)
		assert.Equal(t, compareResult, test.result)
	}
}

// TestVersionStringCompare tests version string comparison
func TestVersionStringCompareWithError(t *testing.T) {
	testCases := []struct {
		a string
		b string
	}{
		{"Invalid version", "1.0.152.0"},
		{"0.0.1.0", "Invalid version"},
	}

	for _, test := range testCases {
		_, err := VersionCompare(test.a, test.b)
		assert.Error(t, err)
	}
}

// TestVersionStringCompare tests version string comparison
func TestIsValidVersion(t *testing.T) {
	testCases := []struct {
		version string
		success bool
	}{
		{"somestring", false},
		{"1.3.3.7", true},
		{"1.0", true},
		{"1", true},
	}

	for _, test := range testCases {
		result := IsValidVersion(test.version)
		if test.success {
			assert.True(t, result, fmt.Sprintf("Version %s should have been valid", test.version))
		} else {
			assert.False(t, result, fmt.Sprintf("Version %s should not have been valid", test.version))
		}
	}
}
