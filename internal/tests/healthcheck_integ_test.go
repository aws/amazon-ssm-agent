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
// Package tests represents stress and integration tests of the agent
package tests

import (
	"errors"
	"runtime/debug"
	"testing"
	"time"

	"github.com/aws/amazon-ssm-agent/agent/agent"
	"github.com/aws/amazon-ssm-agent/agent/appconfig"
	"github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/aws/amazon-ssm-agent/agent/contracts"
	"github.com/aws/amazon-ssm-agent/agent/framework/coremanager"
	"github.com/aws/amazon-ssm-agent/agent/health"
	"github.com/aws/amazon-ssm-agent/agent/hibernation"
	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/amazon-ssm-agent/agent/log/ssmlog"
	"github.com/aws/amazon-ssm-agent/internal/tests/testutils"
	"github.com/aws/aws-sdk-go/service/ssm"
	ssmsdkmock "github.com/aws/aws-sdk-go/service/ssm/ssmiface/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
)

type AgentHealthIntegrationTestSuite struct {
	suite.Suite
	context        context.T
	ssmAgent       agent.ISSMAgent
	ssmSdkMock     *ssmsdkmock.SSMAPI
	config         appconfig.SsmagentConfig
	ssmHealthCheck *health.HealthCheck
	log            log.T
}

func (suite *AgentHealthIntegrationTestSuite) SetupTest() {
	log := ssmlog.SSMLogger(true)
	suite.log = log

	config, configerr := appconfig.Config(true)
	if configerr != nil {
		log.Debugf("appconfig could not be loaded - %v", configerr)
		return
	}
	config.Ssm.HealthFrequencyMinutes = 1
	suite.config = config
	// Add config into context
	context := context.Default(log, config)
	// Changing the minimum heathcheck frequency as 1 minute for testing
	appConst := context.AppConstants()
	appConst.MinHealthFrequencyMinutes = 1
	suite.context = context

	// Use healthcheck service with mock ssm service injected.
	healthcheck, ssmsdkMock := testutils.NewHealthCheck(context)
	suite.ssmHealthCheck = healthcheck
	suite.ssmSdkMock = ssmsdkMock

	hs := hibernation.NewHibernateMode(healthcheck, context)
	assert.NotNil(suite.T(), hs)

	var modules []contracts.ICoreModule
	modules = append(modules, healthcheck)

	// Only inject the healthcheck module into coremanager
	var cpm *coremanager.CoreManager
	var err error
	if cpm, err = testutils.NewCoreManager(context, &modules, log); err != nil {
		log.Errorf("error occurred when starting core manager: %v", err)
		return
	}
	log.Info("Finish Setting up Coremanager")
	// Create core ssm agent and set the coremanager & context into it.
	suite.ssmAgent = agent.NewSSMAgent(context, healthcheck, hs)
	suite.ssmAgent.SetContext(context)
	suite.ssmAgent.SetCoreManager(cpm)
}

func (suite *AgentHealthIntegrationTestSuite) TestHealthCheck() {
	suite.log.Info("Starting Test Health Check, it will take around 5-8 minutes.")
	// Setting the number of excepted scheduled health ping as 5
	designHealthPings := 5

	defer func() {
		// Logging the errors and handle agent panics
		if msg := recover(); msg != nil {
			suite.log.Errorf("Agent crashed with message %v!", msg)
			suite.log.Errorf("%s: %s", msg, debug.Stack())
		}
		suite.log.Flush()
		suite.log.Close()
	}()

	start := time.Now().UTC()
	// create a new channel for blocking the test.
	c := make(chan string)
	actualHealthPings := 0
	suite.ssmSdkMock.On("UpdateInstanceInformation", mock.AnythingOfType("*ssm.UpdateInstanceInformationInput")).Return(func(*ssm.UpdateInstanceInformationInput) *ssm.UpdateInstanceInformationOutput {
		// The time elapse since test start.
		elapsed := time.Since(start).Minutes()
		actualHealthPings++
		suite.log.Infof("Waiting for next health ping, elapsed time is %v minute \n", elapsed)
		if elapsed >= time.Duration(time.Duration( designHealthPings - 1 )*time.Minute).Minutes() {
			if actualHealthPings >= designHealthPings {
				c <- "Succeed"
				suite.T().Log("HealthCheck test succeed.")
			} else {
				// If the number of method calls didn't match expected value, assert the test failed
				c <- "Failed"
				suite.T().Error("HealthCheck test failed. Didn't get enough health pings in testing time interval")
			}
		}
		return &ssm.UpdateInstanceInformationOutput{}
	}, nil).Times(designHealthPings + 1)

	suite.ssmAgent.Start()
	<-c
	//Stop agent execution
	suite.ssmAgent.Stop()
}

func (suite *AgentHealthIntegrationTestSuite) TestHibernationCheck() {
	//Turn on the mock of GetAgentState, return Passive status and error.
	suite.T().Log("Starting Test Hibernation, it will take around 5 minutes.")
	exitHibernate := false
	defer func() {
		// Logging the errors and handle agent panics
		if msg := recover(); msg != nil {
			suite.log.Errorf("Agent crashed with message %v!", msg)
			suite.log.Errorf("%s: %s", msg, debug.Stack())
		}
		suite.log.Flush()
	}()

	suite.T().Log("Turn on Test Hibernations mocks")
	// Open the sdkmock for UpdateInstanceInformation once, agent state is Passive
	suite.ssmSdkMock.On("UpdateInstanceInformation", mock.AnythingOfType("*ssm.UpdateInstanceInformationInput")).Return(&ssm.UpdateInstanceInformationOutput{}, func(*ssm.UpdateInstanceInformationInput) error {
		// Turn on mock , Agent State will first return Passive for first time
		assert.Equal(suite.T(), exitHibernate, false)
		suite.log.Info("Waiting for next ping for hibernation, time interval is 5 Minutes \n")
		return errors.New("integration test is blocked in hibernation mode")
	}).Times(5)

	// Change sdkmock behavior, agent state back to Active
	suite.ssmSdkMock.On("UpdateInstanceInformation", mock.AnythingOfType("*ssm.UpdateInstanceInformationInput")).Return(&ssm.UpdateInstanceInformationOutput{}, func(*ssm.UpdateInstanceInformationInput) error {
		suite.T().Log("Hibernation module try to get agent state second time, return nil as error")
		return nil
	})

	suite.ssmAgent.Hibernate()
	exitHibernate = true
	suite.T().Log("Hibernation Test Succeed")
}

func TestAgentHealthTestSuite(t *testing.T) {
	suite.Run(t, new(AgentHealthIntegrationTestSuite))
}
