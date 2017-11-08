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

// Package datauploader contains routines upload inventory data to SSM - Inventory service
package datauploader

import (
	"github.com/stretchr/testify/mock"
)

// Mock stands for a mocked plugin.
type MockOptimizer struct {
	mock.Mock
}

func NewMockDefault() *MockOptimizer {
	opt := new(MockOptimizer)
	return opt
}

func (m *MockOptimizer) UpdateContentHash(inventoryItemName, hash string) (err error) {
	args := m.Called(inventoryItemName, hash)
	return args.Error(0)
}

func (m *MockOptimizer) GetContentHash(inventoryItemName string) (hash string) {
	args := m.Called(inventoryItemName)
	return args.String(0)
}
