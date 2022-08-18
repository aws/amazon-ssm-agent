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

package health

import (
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/aws/amazon-ssm-agent/agent/appconfig"
	"github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/amazon-ssm-agent/agent/sdkutil"
	ssmMock "github.com/aws/amazon-ssm-agent/agent/ssm/mocks"
	"github.com/aws/amazon-ssm-agent/agent/version"
	"github.com/aws/amazon-ssm-agent/common/identity"
	identityMock "github.com/aws/amazon-ssm-agent/common/identity/mocks"

	"github.com/carlescere/scheduler"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
)

// HealthCheck Test suite. Define the testsuite object.
// Add logMock, contextMock, serviceMock, healthJobMock struct into test suite.
// Suite is the testify framework struct
type HealthCheckTestSuite struct {
	suite.Suite
	logMock     *log.Mock
	contextMock *context.Mock
	serviceMock *ssmMock.Service
	healthJob   *scheduler.Job
	stopPolicy  *sdkutil.StopPolicy
	healthCheck IHealthCheck
}

// Setting up the HealthCheckTestSuite variable, initialize logMock and conntextMock struct
func (suite *HealthCheckTestSuite) SetupTest() {
	logMock := log.NewMockLog()
	contextMock := context.NewMockDefault()

	serviceMock := new(ssmMock.Service)
	healthJob := &scheduler.Job{
		Quit: make(chan bool),
	}

	stopPolicy := sdkutil.NewStopPolicy("hibernation", 10)

	suite.logMock = logMock
	suite.contextMock = contextMock
	suite.serviceMock = serviceMock
	suite.healthJob = healthJob
	suite.stopPolicy = stopPolicy
	suite.healthCheck = &HealthCheck{
		healthCheckStopPolicy: suite.stopPolicy,
		context:               suite.contextMock,
		service:               suite.serviceMock,
	}
}

// Testing the module name
func (suite *HealthCheckTestSuite) TestModuleName() {
	rst := suite.healthCheck.ModuleName()
	assert.Equal(suite.T(), rst, name)
}

// Testing the ModuleExecute method
func (suite *HealthCheckTestSuite) TestModuleExecute() {
	// Initialize the appconfigMock with HealthFrequencyMinutes as every five minute
	appconfigMock := &appconfig.SsmagentConfig{
		Ssm: appconfig.SsmCfg{
			HealthFrequencyMinutes: appconfig.DefaultSsmHealthFrequencyMinutes,
		},
	}

	mockIdentity := &identityMock.IAgentIdentityInner{}
	newEC2Identity = func(log log.T) identity.IAgentIdentityInner {
		return mockIdentity
	}

	availabilityZone := "us-east-1a"
	availabilityZoneId := "use1-az2"
	mockIdentity.On("IsIdentityEnvironment").Return(true)
	mockIdentity.On("AvailabilityZone").Return(availabilityZone, nil)
	mockIdentity.On("AvailabilityZoneId").Return(availabilityZoneId, nil)

	// Turn on the mock method
	suite.contextMock.On("AppConfig").Return(*appconfigMock)
	suite.serviceMock.On("UpdateInstanceInformation", mock.Anything, version.Version, "Active", AgentName, availabilityZone, availabilityZoneId).Return(nil, nil)
	suite.healthCheck.ModuleExecute()
	// Because ModuleExecute will launch two new go routine, wait five second to make sure the updateHealth() has launched
	time.Sleep(100 * time.Millisecond)
	// Assert the UpdateInstanceInformation get called in updateHealth() function, and the agent status is same as input.
	suite.serviceMock.AssertCalled(suite.T(), "UpdateInstanceInformation", mock.Anything, version.Version, "Active", AgentName, availabilityZone, availabilityZoneId)
}

// Testing the ModuleStop method with healthjob define
func (suite *HealthCheckTestSuite) TestModuleStopWithHealthJob() {
	suite.healthCheck = &HealthCheck{
		context:               suite.contextMock,
		healthJob:             suite.healthJob,
		service:               suite.serviceMock,
		healthCheckStopPolicy: suite.stopPolicy,
	}
	// Start a new wg to avoid go panic.
	wg := new(sync.WaitGroup)
	go func(wgc *sync.WaitGroup) {
		wgc.Add(1)
		defer wgc.Done()
		suite.healthCheck.ModuleStop()
	}(wg)
	// Check the value is sent to healthJobMock channel. And the value is true.
	val := <-suite.healthJob.Quit
	close(suite.healthJob.Quit)
	wg.Wait()
	assert.Equal(suite.T(), val, true, "ModuleStop should return true")
}

// Testing the ModuleStop method which doesn't have healthjob defination
func (suite *HealthCheckTestSuite) TestModuleStopWithoutHealthJob() {
	// Start a new wg to avoid go panic
	wg := new(sync.WaitGroup)
	go func(wgc *sync.WaitGroup) {
		wgc.Add(1)
		defer wgc.Done()
		rst := suite.healthCheck.ModuleStop()
		assert.Nil(suite.T(), rst, "result from ModuleStop should be nil")
	}(wg)
}

// Testing the GetAgentState method which should return Active status
func (suite *HealthCheckTestSuite) TestGetAgentStateActive() {
	// UpdateEmptyInstanceInformation will return active in the h.ping() function.
	suite.serviceMock.On("UpdateEmptyInstanceInformation", mock.Anything, version.Version, AgentName).Return(nil, nil)
	agentState, err := suite.healthCheck.GetAgentState()
	// Assert the status is Active and the error is nil.
	assert.Equal(suite.T(), agentState, Active, "agent state should be active")
	assert.Nil(suite.T(), err, "GatAgentState function should always return nil as error")
}

// Testing the GetAgentState method which should return Passive status
func (suite *HealthCheckTestSuite) TestGetAgentStatePassive() {
	// Turn on mock method in UpdateEmptyInstanceInformation, return an error if this function get called.
	suite.serviceMock.On("UpdateEmptyInstanceInformation", mock.Anything, version.Version, AgentName).Return(nil, errors.New("UpdatesWithError"))
	agentState, err := suite.healthCheck.GetAgentState()
	// Assert the status is Passive and h.ping() function return an error.
	assert.Equal(suite.T(), agentState, Passive, "agent state should be Passive")
	assert.NotNil(suite.T(), err, "GetAgentStatePassive should return error message UpdatesWithError")
}

// Execute the test suite
func TestHealthCheckTestSuite(t *testing.T) {
	suite.Run(t, new(HealthCheckTestSuite))
}
