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

package appconfig

import (
	"github.com/stretchr/testify/mock"
)

// Note: This code is used in the test files. However, this code is not in a _test.go file
// because then we would have to copy it in every test package that needs the mock.

// Mock mocks a AppConfig.
type Mock struct {
	mock.Mock
}

// NewMockAppConfig returns an instance of Mock for AppConfig.
func NewMockAppConfig() *Mock {
	return new(Mock)
}

// GetConfig is a mocked method that just returns what mock tells it to.
func (m *Mock) GetConfig(reload bool) (appconfig SsmagentConfig, errs []error) {
	args := m.Called(reload)
	return args.Get(0).(SsmagentConfig), args.Get(1).([]error)
}
