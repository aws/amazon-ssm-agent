// Copyright 2016 Amazon.com, Inc. or its affiliates. All Rights Reserved.
//
// Licensed under the Amazon Software License (the "License"). You may not
// use this file except in compliance with the License. A copy of the
// License is located at
//
// http://aws.amazon.com/asl/
//
// or in the "license" file accompanying this file. This file is distributed
// on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND,
// express or implied. See the License for the specific language governing
// permissions and limitations under the License.

package context

import (
	"github.com/aws/amazon-ssm-agent/agent/appconfig"
	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/stretchr/testify/mock"
)

// Note: This code is used in the test files. However, this code is not in a _test.go file
// because then we would have to copy it in every test package that needs the mock.

// Mock stands for a mocked context.
type Mock struct {
	mock.Mock
}

// NewMockDefault returns an instance of Mock with default expectations set.
func NewMockDefault() *Mock {
	ctx := new(Mock)
	log := log.NewMockLog()
	config := appconfig.NewMockAppConfig()
	ctx.On("Log").Return(log)
	ctx.On("AppConfig").Return(config)
	ctx.On("With", mock.AnythingOfType("string")).Return(ctx)
	return ctx
}

// AppConfig mocks the Config function.
func (m *Mock) AppConfig() appconfig.T {
	args := m.Called()
	return args.Get(0).(appconfig.T)
}

// Log mocks the Log function.
func (m *Mock) Log() log.T {
	args := m.Called()
	return args.Get(0).(log.T)
}

// With mocks the With function.
func (m *Mock) With(ctx string) T {
	args := m.Called(ctx)
	return args.Get(0).(T)
}
