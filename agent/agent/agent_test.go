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

// Package agent represents the core SSM agent object
package agent

import (
	"testing"

	"github.com/aws/amazon-ssm-agent/agent/context"
	coremanager "github.com/aws/amazon-ssm-agent/agent/framework/coremanager/mocks"
	health "github.com/aws/amazon-ssm-agent/agent/health"
	healthmock "github.com/aws/amazon-ssm-agent/agent/health/mocks"
	hibernation "github.com/aws/amazon-ssm-agent/agent/hibernation/mocks"
	"github.com/stretchr/testify/suite"
)

// AgentTestSuite define agent test suite, and absorb the built-in basic suite
// functionality from testify - including a T() method which
// returns the current testing context
type AgentTestSuite struct {
	suite.Suite
	mockSSMAgent     ISSMAgent
	mockCoreManager  *coremanager.ICoreManager
	mockHiberation   *hibernation.IHibernate
	mockHealthModule *healthmock.IHealthCheck
}

// SetupTest makes sure that all the components referenced in the test case are initialized
// before each test
func (suite *AgentTestSuite) SetupTest() {
	mockCoreManager := new(coremanager.ICoreManager)
	mockCoreManager.On("Start").Return()

	mockHiberation := new(hibernation.IHibernate)
	mockHiberation.On("ExecuteHibernation").Return(health.Active)

	mockHealthModule := new(healthmock.IHealthCheck)

	suite.mockCoreManager = mockCoreManager
	suite.mockHiberation = mockHiberation
	suite.mockHealthModule = mockHealthModule
	suite.mockSSMAgent = &SSMAgent{
		context:        context.NewMockDefault(),
		coreManager:    mockCoreManager,
		healthModule:   mockHealthModule,
		hibernateState: mockHiberation,
	}
}

// TestAgentActiveHibernation tests that agent executes hibernation if the agent state was passive
// All methods that begin with "Test" are run as tests within a suite.
func (suite *AgentTestSuite) TestAgentActiveHibernation() {
	suite.mockHealthModule.On("GetAgentState").Return(health.Passive, nil)
	suite.mockSSMAgent.Hibernate()
	suite.mockHiberation.AssertCalled(suite.T(), "ExecuteHibernation")
}

// TestAgentPassiveHibernation tests that agent doesnot executes hibernation if the agent state was active
func (suite *AgentTestSuite) TestAgentPassiveHibernation() {
	suite.mockHealthModule.On("GetAgentState").Return(health.Active, nil)
	suite.mockSSMAgent.Hibernate()
	suite.mockHiberation.AssertNotCalled(suite.T(), "ExecuteHibernation")
}

// TestAgentStart tests that agent starts the core manager when it starts
func (suite *AgentTestSuite) TestAgentStart() {
	suite.mockSSMAgent.Start()
	suite.mockCoreManager.AssertCalled(suite.T(), "Start")
}

// TestAgentTestSuite a normal test function that passes our suite to suite.Run
// in order for 'go test' to run this suite
func TestAgentTestSuite(t *testing.T) {
	suite.Run(t, new(AgentTestSuite))
}
