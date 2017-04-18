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
package packageservice

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIsLatest(t *testing.T) {
	data := []struct {
		version  string
		expected bool
	}{
		{"latest", true},
		{"Latest", true},
		{"LATEST", true},
		{"LaTeSt", true},
		{"", true},
		{" ", false},
		{"asdf", false},
		{"666", false},
		{"€‹⁄Ô°·ÔÅˆÇ", false},
	}

	for _, testdata := range data {
		t.Run(testdata.version, func(t *testing.T) {
			result := IsLatest(testdata.version)
			assert.Equal(t, testdata.expected, result)
		})
	}
}
