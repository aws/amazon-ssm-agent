// Copyright 2020 Amazon.com, Inc. or its affiliates. All Rights Reserved.
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
package identity

import (
	"fmt"
	"testing"

	"github.com/aws/amazon-ssm-agent/agent/appconfig"
	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/amazon-ssm-agent/common/identity/availableidentities/ec2/mocks"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestNewAgentIdentity_ContainerMode_MissingIdentityFunc(t *testing.T) {
	var config appconfig.SsmagentConfig
	config.Agent.ContainerMode = true

	selector := &iAgentIdentitySelectorMock{}
	identityGenerators := make(map[string]createIdentityFunc)

	ident, err := newAgentIdentityInner(log.NewMockLog(), &config, selector, appconfig.DefaultIdentityConsumptionOrder, identityGenerators)
	assert.Nil(t, ident)
	assert.NotNil(t, err)
}

func TestNewAgentIdentity_ContainerMode_NoIdentitySelected(t *testing.T) {
	var config appconfig.SsmagentConfig
	config.Agent.ContainerMode = true

	selector := &iAgentIdentitySelectorMock{}

	selector.On("selectAgentIdentity", mock.Anything, mock.Anything).Return(nil, fmt.Errorf("some error"))
	identityGenerators := make(map[string]createIdentityFunc)
	identityGenerators["ECS"] = func(log.T, *appconfig.SsmagentConfig) []IAgentIdentityInner {
		return []IAgentIdentityInner{}
	}

	ident, err := newAgentIdentityInner(log.NewMockLog(), &config, selector, appconfig.DefaultIdentityConsumptionOrder, identityGenerators)
	assert.Nil(t, ident)
	assert.NotNil(t, err)
}

func TestNewAgentIdentity_ContainerMode_BackwardsCompatibilityOverride(t *testing.T) {
	var config appconfig.SsmagentConfig
	config.Agent.ContainerMode = true

	selector := &iAgentIdentitySelectorMock{}

	agentIdentity := &mocks.IEC2Identity{}
	selector.On("selectAgentIdentity", mock.Anything, mock.Anything).Return(agentIdentity, nil)
	identityGenerators := make(map[string]createIdentityFunc)
	onPremCalled := false
	identityGenerators["OnPrem"] = func(log.T, *appconfig.SsmagentConfig) []IAgentIdentityInner {
		onPremCalled = true
		return []IAgentIdentityInner{}
	}

	ident, err := newAgentIdentityInner(log.NewMockLog(), &config, selector, []string{"OnPrem"}, identityGenerators)
	assert.NotNil(t, agentIdentity, ident)
	assert.Nil(t, err)
	assert.True(t, onPremCalled)
}

func TestNewAgentIdentity_ContainerMode_IdentitySelected(t *testing.T) {
	var config appconfig.SsmagentConfig
	config.Agent.ContainerMode = true

	agentIdentity := &mocks.IEC2Identity{}
	selector := &iAgentIdentitySelectorMock{}
	selector.On("selectAgentIdentity", mock.Anything, mock.Anything).Return(agentIdentity, nil)
	identityGenerators := make(map[string]createIdentityFunc)
	identityGenerators["ECS"] = func(log.T, *appconfig.SsmagentConfig) []IAgentIdentityInner {
		return []IAgentIdentityInner{}
	}

	ident, err := newAgentIdentityInner(log.NewMockLog(), &config, selector, []string{"ECS"}, identityGenerators)
	assert.NotNil(t, ident)
	assert.Nil(t, err)
}

func TestNewAgentIdentity_MissingIdentityFunc(t *testing.T) {
	var config appconfig.SsmagentConfig

	selector := &iAgentIdentitySelectorMock{}
	identityGenerators := make(map[string]createIdentityFunc)

	ident, err := newAgentIdentityInner(log.NewMockLog(), &config, selector, []string{"SomeRandomIdentity"}, identityGenerators)
	assert.Nil(t, ident)
	assert.NotNil(t, err)
}

func TestNewAgentIdentity_NoIdentitySelected(t *testing.T) {
	var config appconfig.SsmagentConfig

	selector := &iAgentIdentitySelectorMock{}

	selector.On("selectAgentIdentity", mock.Anything, mock.Anything).Return(nil, fmt.Errorf("some error"))
	identityGenerators := make(map[string]createIdentityFunc)
	identityGenerators["SomeRandomIdentity"] = func(log.T, *appconfig.SsmagentConfig) []IAgentIdentityInner {
		return []IAgentIdentityInner{}
	}

	ident, err := newAgentIdentityInner(log.NewMockLog(), &config, selector, []string{"SomeRandomIdentity"}, identityGenerators)
	assert.Nil(t, ident)
	assert.NotNil(t, err)
}

func TestNewAgentIdentity_IdentitySelected(t *testing.T) {
	var config appconfig.SsmagentConfig

	agentIdentity := &mocks.IEC2Identity{}
	selector := &iAgentIdentitySelectorMock{}
	selector.On("selectAgentIdentity", mock.Anything, mock.Anything).Return(agentIdentity, nil)
	identityGenerators := make(map[string]createIdentityFunc)
	identityGenerators["SomeRandomIdentity"] = func(log.T, *appconfig.SsmagentConfig) []IAgentIdentityInner {
		return []IAgentIdentityInner{}
	}

	ident, err := newAgentIdentityInner(log.NewMockLog(), &config, selector, []string{"SomeRandomIdentity"}, identityGenerators)
	assert.NotNil(t, ident)
	assert.Nil(t, err)
}

func TestDefaultAgentIdentitySelector_NotIsEnvironment(t *testing.T) {
	selector := &defaultAgentIdentitySelector{
		log: log.NewMockLog(),
	}

	agentIdentity := &mocks.IEC2Identity{}
	agentIdentity.On("IsIdentityEnvironment").Return(false)

	ident := selector.selectAgentIdentity([]IAgentIdentityInner{agentIdentity}, "SomeIdentityKey")

	assert.Nil(t, ident)
}

func TestDefaultAgentIdentitySelector_NoInstanceIDNoRegion(t *testing.T) {
	selector := &defaultAgentIdentitySelector{
		log: log.NewMockLog(),
	}

	agentIdentity := &mocks.IEC2Identity{}
	agentIdentity.On("IsIdentityEnvironment").Return(true)

	ident := selector.selectAgentIdentity([]IAgentIdentityInner{agentIdentity}, "SomeIdentityKey")

	assert.NotNil(t, ident)
}

func TestInstanceIDRegionAgentIdentitySelector_NotIsEnvironment(t *testing.T) {
	selector := &instanceIDRegionAgentIdentitySelector{
		log:        log.NewMockLog(),
		region:     "",
		instanceID: "",
	}

	agentIdentity := &mocks.IEC2Identity{}
	agentIdentity.On("IsIdentityEnvironment").Return(false)

	ident := selector.selectAgentIdentity([]IAgentIdentityInner{agentIdentity}, "SomeIdentityKey")

	assert.Nil(t, ident)
}

func TestInstanceIDRegionAgentIdentitySelector_NoInstanceIDNoRegion(t *testing.T) {
	selector := &instanceIDRegionAgentIdentitySelector{
		log:        log.NewMockLog(),
		region:     "",
		instanceID: "",
	}

	agentIdentity := &mocks.IEC2Identity{}
	agentIdentity.On("IsIdentityEnvironment").Return(true)

	ident := selector.selectAgentIdentity([]IAgentIdentityInner{agentIdentity}, "SomeIdentityKey")

	assert.NotNil(t, ident)
}

func TestInstanceIDRegionAgentIdentitySelector_ErrorWhenInstanceId(t *testing.T) {
	selector := &instanceIDRegionAgentIdentitySelector{
		log:        log.NewMockLog(),
		region:     "",
		instanceID: "SomeInstanceId",
	}

	agentIdentity := &mocks.IEC2Identity{}
	agentIdentity.On("IsIdentityEnvironment").Return(true)
	agentIdentity.On("InstanceID").Return("", fmt.Errorf("SomeError"))

	ident := selector.selectAgentIdentity([]IAgentIdentityInner{agentIdentity}, "SomeIdentityKey")

	assert.Nil(t, ident)
}

func TestInstanceIDRegionAgentIdentitySelector_IncorrectInstanceId(t *testing.T) {
	selector := &instanceIDRegionAgentIdentitySelector{
		log:        log.NewMockLog(),
		region:     "",
		instanceID: "SomeInstanceId",
	}

	agentIdentity := &mocks.IEC2Identity{}
	agentIdentity.On("IsIdentityEnvironment").Return(true)
	agentIdentity.On("InstanceID").Return("SomeOtherInstanceId", nil)

	ident := selector.selectAgentIdentity([]IAgentIdentityInner{agentIdentity}, "SomeIdentityKey")

	assert.Nil(t, ident)
}

func TestInstanceIDRegionAgentIdentitySelector_CorrectInstanceIdNoRegion(t *testing.T) {
	selector := &instanceIDRegionAgentIdentitySelector{
		log:        log.NewMockLog(),
		region:     "",
		instanceID: "SomeInstanceId",
	}

	agentIdentity := &mocks.IEC2Identity{}
	agentIdentity.On("IsIdentityEnvironment").Return(true)
	agentIdentity.On("InstanceID").Return("SomeInstanceId", nil)
	ident := selector.selectAgentIdentity([]IAgentIdentityInner{agentIdentity}, "SomeIdentityKey")

	assert.NotNil(t, ident)
}

func TestInstanceIDRegionAgentIdentitySelector_ErrorWhenRegion(t *testing.T) {
	selector := &instanceIDRegionAgentIdentitySelector{
		log:        log.NewMockLog(),
		region:     "SomeRegion",
		instanceID: "",
	}

	agentIdentity := &mocks.IEC2Identity{}
	agentIdentity.On("IsIdentityEnvironment").Return(true)
	agentIdentity.On("Region").Return("", fmt.Errorf("SomeError"))

	ident := selector.selectAgentIdentity([]IAgentIdentityInner{agentIdentity}, "SomeIdentityKey")

	assert.Nil(t, ident)
}

func TestInstanceIDRegionAgentIdentitySelector_IncorrectRegion(t *testing.T) {
	selector := &instanceIDRegionAgentIdentitySelector{
		log:        log.NewMockLog(),
		region:     "SomeRegion",
		instanceID: "",
	}

	agentIdentity := &mocks.IEC2Identity{}
	agentIdentity.On("IsIdentityEnvironment").Return(true)
	agentIdentity.On("Region").Return("SomeOtherRegion", nil)

	ident := selector.selectAgentIdentity([]IAgentIdentityInner{agentIdentity}, "SomeIdentityKey")

	assert.Nil(t, ident)
}

func TestInstanceIDRegionAgentIdentitySelector_CorrectRegion(t *testing.T) {
	selector := &instanceIDRegionAgentIdentitySelector{
		log:        log.NewMockLog(),
		region:     "SomeRegion",
		instanceID: "",
	}

	agentIdentity := &mocks.IEC2Identity{}
	agentIdentity.On("IsIdentityEnvironment").Return(true)
	agentIdentity.On("Region").Return("SomeRegion", nil)

	ident := selector.selectAgentIdentity([]IAgentIdentityInner{agentIdentity}, "SomeIdentityKey")

	assert.NotNil(t, ident)
}

func TestIsDefaultIdentityConsumptionOrder(t *testing.T) {
	testOrder := []string{"SomeIdentity", "AnotherIdentity"}
	defaultOrder := []string{"SomeIdentity"}
	assert.False(t, isDefaultIdentityConsumptionOrder(testOrder, defaultOrder))

	testOrder = []string{"SomeIdentity", "AnotherIdentity"}
	defaultOrder = []string{"AnotherIdentity", "SomeIdentity"}
	assert.False(t, isDefaultIdentityConsumptionOrder(testOrder, defaultOrder))

	testOrder = []string{"SomeIdentity", "AnotherIdentity"}
	defaultOrder = []string{"SomeIdentity", "AnotherIdentity"}
	assert.True(t, isDefaultIdentityConsumptionOrder(testOrder, defaultOrder))
}
