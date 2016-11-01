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

// Package gatherers contains routines for different types of inventory gatherers
package gatherers

import (
	"github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/aws/amazon-ssm-agent/agent/contracts"
	"github.com/aws/amazon-ssm-agent/agent/plugins/inventory/model"
	"github.com/stretchr/testify/mock"
)

// Note: This code is used in the test files. However, this code is not in a _test.go file
// because then we would have to copy it in every test package that needs the mock.

// Mock represents a mocked gatherer.
type Mock struct {
	mock.Mock
}

// NewMockDefault returns an instance of Mock with default expectations set.
func NewMockDefault() *Mock {
	return new(Mock)
}

// Name mock implementation of namesake
func (m *Mock) Name() string {
	args := m.Called()
	return args.String(0)
}

// Run mock implementation of namesake
func (m *Mock) Run(context context.T, configuration model.Config) ([]model.Item, error) {
	args := m.Called(context, configuration)
	return args.Get(0).([]model.Item), args.Error(1)
}

// RequestStop mock implementation of namesake
func (m *Mock) RequestStop(stopType contracts.StopType) error {
	args := m.Called(stopType)
	return args.Error(0)
}
