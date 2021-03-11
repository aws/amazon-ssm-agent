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

package context

import (
	"strings"

	"github.com/aws/amazon-ssm-agent/agent/appconfig"
	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/amazon-ssm-agent/common/identity"
	identityMocks "github.com/aws/amazon-ssm-agent/common/identity/mocks"
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
	config := appconfig.SsmagentConfig{}
	agentIdentity := identityMocks.NewDefaultMockAgentIdentity()
	appconst := appconfig.AppConstants{
		MinHealthFrequencyMinutes: appconfig.DefaultSsmHealthFrequencyMinutesMin,
		MaxHealthFrequencyMinutes: appconfig.DefaultSsmHealthFrequencyMinutesMax,
	}
	ctx.On("Log").Return(log)
	ctx.On("AppConfig").Return(config)
	ctx.On("With", mock.AnythingOfType("string")).Return(ctx)
	ctx.On("CurrentContext").Return([]string{})
	ctx.On("Identity").Return(agentIdentity)
	ctx.On("AppConstants").Return(&appconst)
	return ctx
}

func NewMockDefaultWithConfig(config appconfig.SsmagentConfig) *Mock {
	ctx := new(Mock)
	log := log.NewMockLog()
	agentIdentity := identityMocks.NewDefaultMockAgentIdentity()
	appconst := appconfig.AppConstants{
		MinHealthFrequencyMinutes: appconfig.DefaultSsmHealthFrequencyMinutesMin,
		MaxHealthFrequencyMinutes: appconfig.DefaultSsmHealthFrequencyMinutesMax,
	}
	ctx.On("Log").Return(log)
	ctx.On("AppConfig").Return(config)
	ctx.On("With", mock.AnythingOfType("string")).Return(ctx)
	ctx.On("CurrentContext").Return([]string{})
	ctx.On("Identity").Return(agentIdentity)
	ctx.On("AppConstants").Return(&appconst)
	return ctx
}

// NewMockDefaultWithContext returns an instance of Mock with specified context.
func NewMockDefaultWithContext(context []string) *Mock {
	ctx := new(Mock)
	log := log.NewMockLogWithContext(strings.Join(context, ""))
	config := appconfig.SsmagentConfig{}
	agentIdentity := identityMocks.NewDefaultMockAgentIdentity()
	appconst := appconfig.AppConstants{
		MinHealthFrequencyMinutes: appconfig.DefaultSsmHealthFrequencyMinutesMin,
		MaxHealthFrequencyMinutes: appconfig.DefaultSsmHealthFrequencyMinutesMax,
	}
	ctx.On("Log").Return(log)
	ctx.On("AppConfig").Return(config)
	ctx.On("With", mock.AnythingOfType("string")).Return(ctx)
	ctx.On("CurrentContext").Return(context)
	ctx.On("Identity").Return(agentIdentity)
	ctx.On("AppConstants").Return(&appconst)
	return ctx
}

// AppConfig mocks the Config function.
func (m *Mock) AppConfig() appconfig.SsmagentConfig {
	args := m.Called()
	return args.Get(0).(appconfig.SsmagentConfig)
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

// CurrentContext mocks the CurrentContext function.
func (m *Mock) CurrentContext() []string {
	args := m.Called()
	return args.Get(0).([]string)
}

// AppConstants mocks the AppConstatns function.
func (m *Mock) AppConstants() *appconfig.AppConstants {
	args := m.Called()
	return args.Get(0).(*appconfig.AppConstants)
}

// Identity mocks the Identity function.
func (m *Mock) Identity() identity.IAgentIdentity {
	args := m.Called()
	return args.Get(0).(identity.IAgentIdentity)
}
