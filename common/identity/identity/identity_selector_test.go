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
	logmocks "github.com/aws/amazon-ssm-agent/agent/mocks/log"
	"github.com/aws/amazon-ssm-agent/common/identity"
	"github.com/aws/amazon-ssm-agent/common/identity/availableidentities/ec2/mocks"
	identitymocks "github.com/aws/amazon-ssm-agent/common/identity/mocks"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestNewAgentIdentity_ContainerMode_MissingIdentityFunc(t *testing.T) {
	var config appconfig.SsmagentConfig
	config.Agent.ContainerMode = true

	selector := &identitymocks.IAgentIdentitySelectorMock{}
	identityGenerators := make(map[string]CreateIdentityFunc)

	ident, err := newAgentIdentityInner(logmocks.NewMockLog(), &config, selector, appconfig.DefaultIdentityConsumptionOrder, identityGenerators)
	assert.Nil(t, ident)
	assert.NotNil(t, err)
}

func TestNewAgentIdentity_ContainerMode_NoIdentitySelected(t *testing.T) {
	var config appconfig.SsmagentConfig
	config.Agent.ContainerMode = true

	selector := &identitymocks.IAgentIdentitySelectorMock{}

	selector.On("SelectAgentIdentity", mock.Anything, mock.Anything).Return(nil, fmt.Errorf("some error"))
	identityGenerators := make(map[string]CreateIdentityFunc)
	identityGenerators["ECS"] = func(log.T, *appconfig.SsmagentConfig) []identity.IAgentIdentityInner {
		return []identity.IAgentIdentityInner{}
	}

	ident, err := newAgentIdentityInner(logmocks.NewMockLog(), &config, selector, appconfig.DefaultIdentityConsumptionOrder, identityGenerators)
	assert.Nil(t, ident)
	assert.NotNil(t, err)
}

func TestNewAgentIdentity_ContainerMode_BackwardsCompatibilityOverride(t *testing.T) {
	var config appconfig.SsmagentConfig
	config.Agent.ContainerMode = true

	selector := &identitymocks.IAgentIdentitySelectorMock{}

	agentIdentity := &mocks.IEC2Identity{}
	selector.On("SelectAgentIdentity", mock.Anything, mock.Anything).Return(agentIdentity, nil)
	identityGenerators := make(map[string]CreateIdentityFunc)
	onPremCalled := false
	identityGenerators["OnPrem"] = func(log.T, *appconfig.SsmagentConfig) []identity.IAgentIdentityInner {
		onPremCalled = true
		return []identity.IAgentIdentityInner{}
	}

	ident, err := newAgentIdentityInner(logmocks.NewMockLog(), &config, selector, []string{"OnPrem"}, identityGenerators)
	assert.NotNil(t, agentIdentity, ident)
	assert.Nil(t, err)
	assert.True(t, onPremCalled)
}

func TestNewAgentIdentity_ContainerMode_IdentitySelected(t *testing.T) {
	var config appconfig.SsmagentConfig
	config.Agent.ContainerMode = true

	agentIdentity := &mocks.IEC2Identity{}
	selector := &identitymocks.IAgentIdentitySelectorMock{}
	selector.On("SelectAgentIdentity", mock.Anything, mock.Anything).Return(agentIdentity, nil)
	identityGenerators := make(map[string]CreateIdentityFunc)
	identityGenerators["ECS"] = func(log.T, *appconfig.SsmagentConfig) []identity.IAgentIdentityInner {
		return []identity.IAgentIdentityInner{}
	}

	ident, err := newAgentIdentityInner(logmocks.NewMockLog(), &config, selector, []string{"ECS"}, identityGenerators)
	assert.NotNil(t, ident)
	assert.Nil(t, err)
}

func TestNewAgentIdentity_MissingIdentityFunc(t *testing.T) {
	var config appconfig.SsmagentConfig

	selector := &identitymocks.IAgentIdentitySelectorMock{}
	identityGenerators := make(map[string]CreateIdentityFunc)

	ident, err := newAgentIdentityInner(logmocks.NewMockLog(), &config, selector, []string{"SomeRandomIdentity"}, identityGenerators)
	assert.Nil(t, ident)
	assert.NotNil(t, err)
}

func TestNewAgentIdentity_NoIdentitySelected(t *testing.T) {
	var config appconfig.SsmagentConfig

	selector := &identitymocks.IAgentIdentitySelectorMock{}

	selector.On("SelectAgentIdentity", mock.Anything, mock.Anything).Return(nil, fmt.Errorf("some error"))
	identityGenerators := make(map[string]CreateIdentityFunc)
	identityGenerators["SomeRandomIdentity"] = func(log.T, *appconfig.SsmagentConfig) []identity.IAgentIdentityInner {
		return []identity.IAgentIdentityInner{}
	}

	ident, err := newAgentIdentityInner(logmocks.NewMockLog(), &config, selector, []string{"SomeRandomIdentity"}, identityGenerators)
	assert.Nil(t, ident)
	assert.NotNil(t, err)
}

func TestNewAgentIdentity_IdentitySelected(t *testing.T) {
	var config appconfig.SsmagentConfig

	agentIdentity := &mocks.IEC2Identity{}
	selector := &identitymocks.IAgentIdentitySelectorMock{}
	selector.On("SelectAgentIdentity", mock.Anything, mock.Anything).Return(agentIdentity, nil)
	identityGenerators := make(map[string]CreateIdentityFunc)
	identityGenerators["SomeRandomIdentity"] = func(log.T, *appconfig.SsmagentConfig) []identity.IAgentIdentityInner {
		return []identity.IAgentIdentityInner{}
	}

	ident, err := newAgentIdentityInner(logmocks.NewMockLog(), &config, selector, []string{"SomeRandomIdentity"}, identityGenerators)
	assert.NotNil(t, ident)
	assert.Nil(t, err)
}

func TestDefaultAgentIdentitySelector_NotIsEnvironment(t *testing.T) {
	selector := &defaultAgentIdentitySelector{
		log: logmocks.NewMockLog(),
	}

	agentIdentity := &mocks.IEC2Identity{}
	agentIdentity.On("IsIdentityEnvironment").Return(false)

	ident := selector.SelectAgentIdentity([]identity.IAgentIdentityInner{agentIdentity}, "SomeIdentityKey")

	assert.Nil(t, ident)
}

func TestDefaultAgentIdentitySelector_NoInstanceIDNoRegion(t *testing.T) {
	selector := &defaultAgentIdentitySelector{
		log: logmocks.NewMockLog(),
	}

	agentIdentity := &mocks.IEC2Identity{}
	agentIdentity.On("IsIdentityEnvironment").Return(true)

	ident := selector.SelectAgentIdentity([]identity.IAgentIdentityInner{agentIdentity}, "SomeIdentityKey")

	assert.NotNil(t, ident)
}

func TestInstanceIDRegionAgentIdentitySelector_NotIsEnvironment(t *testing.T) {
	selector := &instanceIDRegionAgentIdentitySelector{
		log:        logmocks.NewMockLog(),
		region:     "",
		instanceID: "",
	}

	agentIdentity := &mocks.IEC2Identity{}
	agentIdentity.On("IsIdentityEnvironment").Return(false)

	ident := selector.SelectAgentIdentity([]identity.IAgentIdentityInner{agentIdentity}, "SomeIdentityKey")

	assert.Nil(t, ident)
}

func TestInstanceIDRegionAgentIdentitySelector_NoInstanceIDNoRegion(t *testing.T) {
	selector := &instanceIDRegionAgentIdentitySelector{
		log:        logmocks.NewMockLog(),
		region:     "",
		instanceID: "",
	}

	agentIdentity := &mocks.IEC2Identity{}
	agentIdentity.On("IsIdentityEnvironment").Return(true)

	ident := selector.SelectAgentIdentity([]identity.IAgentIdentityInner{agentIdentity}, "SomeIdentityKey")

	assert.NotNil(t, ident)
}

func TestInstanceIDRegionAgentIdentitySelector_ErrorWhenInstanceId(t *testing.T) {
	selector := &instanceIDRegionAgentIdentitySelector{
		log:        logmocks.NewMockLog(),
		region:     "",
		instanceID: "SomeInstanceId",
	}

	agentIdentity := &mocks.IEC2Identity{}
	agentIdentity.On("IsIdentityEnvironment").Return(true)
	agentIdentity.On("InstanceID").Return("", fmt.Errorf("SomeError"))

	ident := selector.SelectAgentIdentity([]identity.IAgentIdentityInner{agentIdentity}, "SomeIdentityKey")

	assert.Nil(t, ident)
}

func TestInstanceIDRegionAgentIdentitySelector_IncorrectInstanceId(t *testing.T) {
	selector := &instanceIDRegionAgentIdentitySelector{
		log:        logmocks.NewMockLog(),
		region:     "",
		instanceID: "SomeInstanceId",
	}

	agentIdentity := &mocks.IEC2Identity{}
	agentIdentity.On("IsIdentityEnvironment").Return(true)
	agentIdentity.On("InstanceID").Return("SomeOtherInstanceId", nil)

	ident := selector.SelectAgentIdentity([]identity.IAgentIdentityInner{agentIdentity}, "SomeIdentityKey")

	assert.Nil(t, ident)
}

func TestInstanceIDRegionAgentIdentitySelector_CorrectInstanceIdNoRegion(t *testing.T) {
	selector := &instanceIDRegionAgentIdentitySelector{
		log:        logmocks.NewMockLog(),
		region:     "",
		instanceID: "SomeInstanceId",
	}

	agentIdentity := &mocks.IEC2Identity{}
	agentIdentity.On("IsIdentityEnvironment").Return(true)
	agentIdentity.On("InstanceID").Return("SomeInstanceId", nil)
	ident := selector.SelectAgentIdentity([]identity.IAgentIdentityInner{agentIdentity}, "SomeIdentityKey")

	assert.NotNil(t, ident)
}

func TestInstanceIDRegionAgentIdentitySelector_ErrorWhenRegion(t *testing.T) {
	selector := &instanceIDRegionAgentIdentitySelector{
		log:        logmocks.NewMockLog(),
		region:     "SomeRegion",
		instanceID: "",
	}

	agentIdentity := &mocks.IEC2Identity{}
	agentIdentity.On("IsIdentityEnvironment").Return(true)
	agentIdentity.On("Region").Return("", fmt.Errorf("SomeError"))

	ident := selector.SelectAgentIdentity([]identity.IAgentIdentityInner{agentIdentity}, "SomeIdentityKey")

	assert.Nil(t, ident)
}

func TestInstanceIDRegionAgentIdentitySelector_IncorrectRegion(t *testing.T) {
	selector := &instanceIDRegionAgentIdentitySelector{
		log:        logmocks.NewMockLog(),
		region:     "SomeRegion",
		instanceID: "",
	}

	agentIdentity := &mocks.IEC2Identity{}
	agentIdentity.On("IsIdentityEnvironment").Return(true)
	agentIdentity.On("Region").Return("SomeOtherRegion", nil)

	ident := selector.SelectAgentIdentity([]identity.IAgentIdentityInner{agentIdentity}, "SomeIdentityKey")

	assert.Nil(t, ident)
}

func TestInstanceIDRegionAgentIdentitySelector_CorrectRegion(t *testing.T) {
	selector := &instanceIDRegionAgentIdentitySelector{
		log:        logmocks.NewMockLog(),
		region:     "SomeRegion",
		instanceID: "",
	}

	agentIdentity := &mocks.IEC2Identity{}
	agentIdentity.On("IsIdentityEnvironment").Return(true)
	agentIdentity.On("Region").Return("SomeRegion", nil)

	ident := selector.SelectAgentIdentity([]identity.IAgentIdentityInner{agentIdentity}, "SomeIdentityKey")

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
