// Copyright 2025 Amazon.com, Inc. or its affiliates. All Rights Reserved.
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

func TestProviderConstants(t *testing.T) {
	// Test Provider type constants
	assert.Equal(t, Provider("EC2"), EC2)
	assert.Equal(t, Provider("Azure"), Azure)
	assert.Equal(t, Provider("GCP"), GCP)
}

func TestProvidersMap(t *testing.T) {
	// Test that Providers map contains expected entries
	assert.Equal(t, EC2, Providers["EC2"])
	assert.Equal(t, Azure, Providers["Azure"])
	assert.Equal(t, GCP, Providers["GCP"])

	// Test map length
	assert.Equal(t, 3, len(Providers))
}

func TestProviderMapLookup(t *testing.T) {
	// Test valid provider lookups
	provider, exists := Providers["EC2"]
	assert.True(t, exists)
	assert.Equal(t, EC2, provider)

	provider, exists = Providers["Azure"]
	assert.True(t, exists)
	assert.Equal(t, Azure, provider)

	provider, exists = Providers["GCP"]
	assert.True(t, exists)
	assert.Equal(t, GCP, provider)

	// Test invalid provider lookup
	_, exists = Providers["InvalidProvider"]
	assert.False(t, exists)
}
