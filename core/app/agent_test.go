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

// Package app represents the core SSM agent object
package app

import (
	"fmt"
	"testing"
	"time"

	"github.com/aws/amazon-ssm-agent/agent/log"
	contextmocks "github.com/aws/amazon-ssm-agent/core/app/context/mocks"
	selfupdatemocks "github.com/aws/amazon-ssm-agent/core/app/selfupdate/mocks"
	containermocks "github.com/aws/amazon-ssm-agent/core/workerprovider/longrunningprovider/mocks"
	"github.com/stretchr/testify/suite"
)

// AgentTestSuite define agent test suite, and absorb the built-in basic suite
// functionality from testify - including a T() method which
// returns the current testing context
type AgentTestSuite struct {
	suite.Suite
	coreAgent      CoreAgent
	context        *contextmocks.ICoreAgentContext
	mockconatiner  *containermocks.IContainer
	mockselfupdate *selfupdatemocks.ISelfUpdate
}

// SetupTest makes sure that all the components referenced in the test case are initialized
// before each test
func (suite *AgentTestSuite) SetupTest() {
	suite.mockconatiner = &containermocks.IContainer{}
	suite.context = &contextmocks.ICoreAgentContext{}
	suite.mockselfupdate = &selfupdatemocks.ISelfUpdate{}
	suite.coreAgent = &SSMCoreAgent{
		context:    suite.context,
		container:  suite.mockconatiner,
		selfupdate: suite.mockselfupdate,
	}

	mockLog := log.NewMockLog()
	suite.context.On("Log").Return(mockLog)
}

//Execute the test suite
func TestAgentTestSuite(t *testing.T) {
	suite.Run(t, new(AgentTestSuite))
}

// TestAgentStart tests that agent starts the core manager when it starts
func (suite *AgentTestSuite) TestAgentStart() {
	suite.mockconatiner.On("Monitor").Return()
	suite.mockconatiner.On("Start").Return([]error{})
	suite.mockselfupdate.On("Start").Return()

	suite.coreAgent.Start()
	time.Sleep(10 * time.Millisecond)

	suite.mockconatiner.AssertExpectations(suite.T())
}

func (suite *AgentTestSuite) TestAgentStart_WithStartWorkerError() {
	suite.mockconatiner.On("Monitor").Return()
	suite.mockconatiner.On("Start").Return(
		[]error{fmt.Errorf("test1"), fmt.Errorf("test2")})
	suite.mockselfupdate.On("Start").Return()

	suite.coreAgent.Start()
	time.Sleep(10 * time.Millisecond)

	suite.mockconatiner.AssertExpectations(suite.T())
}
