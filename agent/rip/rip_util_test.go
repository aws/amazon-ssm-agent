// Copyright 2018 Amazon.com, Inc. or its affiliates. All Rights Reserved.
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

// Parts of this file are automatically updated and should not be edited.

// Package rip contains AWS services regional endpoints.
package rip

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetMgsEndpointForUnknownRegion(t *testing.T) {
	endpoint := GetDefaultServiceEndpoint("unknown-region", MgsServiceName)
	expected := MgsServiceName + ".unknown-region.amazonaws.com"

	assert.Equal(t, expected, endpoint)
}

func TestGetMgsEndpointForCnRegion(t *testing.T) {
	endpoint := GetDefaultServiceEndpoint("cn-north-1", MgsServiceName)
	expected := MgsServiceName + ".cn-north-1.amazonaws.com.cn"

	assert.Equal(t, expected, endpoint)
}
