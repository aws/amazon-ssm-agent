// Copyright 2019 Amazon.com, Inc. or its affiliates. All Rights Reserved.
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

package mocks

import (
	"github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/stretchr/testify/mock"
)

// MockCoreAgent encapsulates the core functionality of the agent
type MockCoreAgent struct {
	mock.Mock
}

func (_m *MockCoreAgent) Start() {
	_m.Called()
}

func (_m *MockCoreAgent) Stop() {
	_m.Called()
}

func (_m *MockCoreAgent) SetContext(c context.T) {
	_m.Called(c)
}
