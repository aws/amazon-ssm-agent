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

// Package model contains contracts for inventory
package model

import (
	"sort"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCompareName(t *testing.T) {
	assert.Equal(t, -1, compareName("ABCD", "XYZ"))
	assert.Equal(t, -1, compareName("ABCD", "ABCDA"))

	assert.Equal(t, 1, compareName("ABCDA", "ABCD"))
	assert.Equal(t, 1, compareName("XYZ", "ABCD"))

	assert.Equal(t, 0, compareName("ABCD", "abcd"))
	assert.Equal(t, 0, compareName("abcd", "ABCD"))
}

func TestComparePublisher(t *testing.T) {
	assert.Equal(t, -1, comparePublisher("ABCD", "XYZ", false))
	assert.Equal(t, -1, comparePublisher("ABCD", "ABCDA", false))

	assert.Equal(t, 1, comparePublisher("ABCDA", "ABCD", false))
	assert.Equal(t, 1, comparePublisher("XYZ", "ABCD", false))

	assert.Equal(t, 0, comparePublisher("ABCD", "abcd", false))
	assert.Equal(t, 0, comparePublisher("abcd", "ABCD", false))
	assert.Equal(t, 0, comparePublisher("ABCD", "abcd", true))
	assert.Equal(t, 0, comparePublisher("abcd", "ABCD", true))

	assert.Equal(t, 0, comparePublisher("", "abcd", false))
	assert.Equal(t, 0, comparePublisher("abcd", "", false))

	assert.Equal(t, -1, comparePublisher("", "abcd", true))
	assert.Equal(t, 1, comparePublisher("abcd", "", true))
}

func unsortedA() []ApplicationData {
	return []ApplicationData{
		{Name: "zyx", Version: "1.0.0", ApplicationType: "1", Architecture: "A"},
		{Name: "zyx", Version: "2.0.0", ApplicationType: "2", Architecture: "B"},
		{Name: "yxw", Version: "2.0.0", ApplicationType: "3", Architecture: "C"},
		{Name: "yxw", Version: "1.0.0", ApplicationType: "4", Architecture: "D"},
		{Name: "xwv", Version: "1.0.0", ApplicationType: "5", Architecture: "E"},
		{Name: "abc", Version: "2.0.0", ApplicationType: "6", Architecture: "F"},
		{Name: "def", Version: "2.0.0", ApplicationType: "7", Architecture: "G"},
	}
}

func unsortedB() []ApplicationData {
	return []ApplicationData{
		{Name: "abc", Version: "2.0.0", ApplicationType: "A", URL: "1"},
		{Name: "zyx", Version: "1.0", ApplicationType: "B", URL: "2"},
		{Name: "yxw", Version: "1.0.1", ApplicationType: "C", URL: "3"},
		{Name: "xwv", Version: "1.0", ApplicationType: "D", URL: "4"},
		{Name: "zyxwv", Version: "2.0.0", ApplicationType: "E", URL: "5"},
		{Name: "def", Version: "2.0.0", ApplicationType: "F", URL: "6"},
		{Name: "def", Version: "2.0", ApplicationType: "G", URL: "7"},
	}
}

func unsortedC() []ApplicationData {
	return []ApplicationData{
		{Name: "4", Version: "1.0.0"},
		{Name: "3", Version: "1.0.0"},
	}
}

func unsortedD() []ApplicationData {
	return []ApplicationData{
		{Name: "2", Version: "1.0.0"},
		{Name: "1", Version: "1.0.0"},
	}
}

func mergedCD() []ApplicationData {
	return []ApplicationData{
		{Name: "1", Version: "1.0.0"},
		{Name: "2", Version: "1.0.0"},
		{Name: "3", Version: "1.0.0"},
		{Name: "4", Version: "1.0.0"},
	}
}

func unsortedE() []ApplicationData {
	return []ApplicationData{
		{Name: "1", Publisher: "A", Version: "1.0.0", URL: "a"},
		{Name: "1", Publisher: "B", Version: "1.0.0", URL: "b"},
	}
}

func unsortedF() []ApplicationData {
	return []ApplicationData{
		{Name: "1", Publisher: "b", Version: "1.0.0", URL: "c"},
		{Name: "1", Version: "1.0.0", URL: "d"},
	}
}

func mergedEF() []ApplicationData {
	return []ApplicationData{
		{Name: "1", Publisher: "A", Version: "1.0.0", URL: "a"},
		{Name: "1", Publisher: "B", Version: "1.0.0", URL: "b"},
	}
}

func mergedFE() []ApplicationData {
	return []ApplicationData{
		{Name: "1", Publisher: "A", Version: "1.0.0", URL: "d"},
		{Name: "1", Publisher: "b", Version: "1.0.0", URL: "c"},
	}
}

func sortedA() []ApplicationData {
	return []ApplicationData{
		{Name: "abc", Version: "2.0.0", ApplicationType: "6", Architecture: "F"},
		{Name: "def", Version: "2.0.0", ApplicationType: "7", Architecture: "G"},
		{Name: "xwv", Version: "1.0.0", ApplicationType: "5", Architecture: "E"},
		{Name: "yxw", Version: "1.0.0", ApplicationType: "4", Architecture: "D"},
		{Name: "yxw", Version: "2.0.0", ApplicationType: "3", Architecture: "C"},
		{Name: "zyx", Version: "1.0.0", ApplicationType: "1", Architecture: "A"},
		{Name: "zyx", Version: "2.0.0", ApplicationType: "2", Architecture: "B"},
	}
}

func sortedB() []ApplicationData {
	return []ApplicationData{
		{Name: "abc", Version: "2.0.0", ApplicationType: "A", URL: "1"},
		{Name: "def", Version: "2.0", ApplicationType: "G", URL: "7"},
		{Name: "def", Version: "2.0.0", ApplicationType: "F", URL: "6"},
		{Name: "xwv", Version: "1.0", ApplicationType: "D", URL: "4"},
		{Name: "yxw", Version: "1.0.1", ApplicationType: "C", URL: "3"},
		{Name: "zyx", Version: "1.0", ApplicationType: "B", URL: "2"},
		{Name: "zyxwv", Version: "2.0.0", ApplicationType: "E", URL: "5"},
	}
}

func mergedAB() []ApplicationData {
	return []ApplicationData{
		{Name: "abc", Version: "2.0.0", ApplicationType: "6", Architecture: "F", URL: "1"},
		{Name: "def", Version: "2.0.0", ApplicationType: "7", Architecture: "G", URL: "7"},
		{Name: "def", Version: "2.0.0", ApplicationType: "F", URL: "6"},
		{Name: "xwv", Version: "1.0.0", ApplicationType: "5", Architecture: "E", URL: "4"},
		{Name: "yxw", Version: "1.0.0", ApplicationType: "4", Architecture: "D"},
		{Name: "yxw", Version: "1.0.1", ApplicationType: "C", URL: "3"},
		{Name: "yxw", Version: "2.0.0", ApplicationType: "3", Architecture: "C"},
		{Name: "zyx", Version: "1.0.0", ApplicationType: "1", Architecture: "A", URL: "2"},
		{Name: "zyx", Version: "2.0.0", ApplicationType: "2", Architecture: "B"},
		{Name: "zyxwv", Version: "2.0.0", ApplicationType: "E", URL: "5"},
	}
}

func mergedBA() []ApplicationData {
	return []ApplicationData{
		{Name: "abc", Version: "2.0.0", ApplicationType: "A", Architecture: "F", URL: "1"},
		{Name: "def", Version: "2.0", ApplicationType: "G", Architecture: "G", URL: "7"},
		{Name: "def", Version: "2.0.0", ApplicationType: "F", URL: "6"},
		{Name: "xwv", Version: "1.0", ApplicationType: "D", Architecture: "E", URL: "4"},
		{Name: "yxw", Version: "1.0.0", ApplicationType: "4", Architecture: "D"},
		{Name: "yxw", Version: "1.0.1", ApplicationType: "C", URL: "3"},
		{Name: "yxw", Version: "2.0.0", ApplicationType: "3", Architecture: "C"},
		{Name: "zyx", Version: "1.0", ApplicationType: "B", Architecture: "A", URL: "2"},
		{Name: "zyx", Version: "2.0.0", ApplicationType: "2", Architecture: "B"},
		{Name: "zyxwv", Version: "2.0.0", ApplicationType: "E", URL: "5"},
	}
}

func TestSortA(t *testing.T) {
	postsortA := unsortedA()
	sort.Sort(ByNamePublisherVersion(postsortA))
	assert.Equal(t, sortedA(), postsortA)
}

func TestSortB(t *testing.T) {
	postsortB := unsortedB()
	sort.Sort(ByNamePublisherVersion(postsortB))
	assert.Equal(t, sortedB(), postsortB)
}

func TestMergeAB(t *testing.T) {
	assert.Equal(t, mergedAB(), MergeLists(unsortedA(), unsortedB()))
}

func TestMergeBA(t *testing.T) {
	assert.Equal(t, mergedBA(), MergeLists(unsortedB(), unsortedA()))
}

func TestMergeCD(t *testing.T) {
	assert.Equal(t, mergedCD(), MergeLists(unsortedC(), unsortedD()))
}

func TestMergeDC(t *testing.T) {
	assert.Equal(t, mergedCD(), MergeLists(unsortedD(), unsortedC()))
}

func TestMergeEF(t *testing.T) {
	assert.Equal(t, mergedEF(), MergeLists(unsortedE(), unsortedF()))
}

func TestMergeFE(t *testing.T) {
	assert.Equal(t, mergedFE(), MergeLists(unsortedF(), unsortedE()))
}

func TestMergeANil(t *testing.T) {
	assert.Equal(t, sortedA(), MergeLists(unsortedA(), nil))
}

func TestMergeNilA(t *testing.T) {
	assert.Equal(t, sortedA(), MergeLists(nil, unsortedA()))
}

func TestMergeAEmpty(t *testing.T) {
	assert.Equal(t, sortedA(), MergeLists(unsortedA(), []ApplicationData{}))
}

func TestMergeEmptyA(t *testing.T) {
	assert.Equal(t, sortedA(), MergeLists([]ApplicationData{}, unsortedA()))
}
