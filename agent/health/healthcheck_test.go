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
	"sync/atomic"
	"testing"
	"time"

	"github.com/aws/amazon-ssm-agent/agent/appconfig"
	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/amazon-ssm-agent/agent/mocks/context"
	logmocks "github.com/aws/amazon-ssm-agent/agent/mocks/log"
	"github.com/aws/amazon-ssm-agent/agent/polling"
	"github.com/aws/amazon-ssm-agent/agent/sdkutil"
	ssmMock "github.com/aws/amazon-ssm-agent/agent/ssm/mocks"
	"github.com/aws/amazon-ssm-agent/agent/ssmconnectionchannel"
	"github.com/aws/amazon-ssm-agent/agent/version"
	"github.com/aws/amazon-ssm-agent/common/identity"
	identityMock "github.com/aws/amazon-ssm-agent/common/identity/mocks"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

// HealthCheck Test suite. Define the testsuite object.
// Add logMock, contextMock, and serviceMock to the test suite.
// Suite is the testify framework struct
type HealthCheckTestSuite struct {
	suite.Suite
	logMock     *logmocks.Mock
	contextMock *context.Mock
	serviceMock *ssmMock.Service
	stopPolicy  *sdkutil.StopPolicy
	healthCheck IHealthCheck
}

// Setting up the HealthCheckTestSuite variable, initialize logMock and conntextMock struct
func (suite *HealthCheckTestSuite) SetupTest() {
	logMock := logmocks.NewMockLog()
	contextMock := context.NewMockDefault()

	serviceMock := new(ssmMock.Service)

	stopPolicy := sdkutil.NewStopPolicy("hibernation", 10)

	suite.logMock = logMock
	suite.contextMock = contextMock
	suite.serviceMock = serviceMock
	suite.stopPolicy = stopPolicy
	suite.healthCheck = &HealthCheck{
		healthCheckStopPolicy: suite.stopPolicy,
		context:               suite.contextMock,
		service:               suite.serviceMock,
	}
}

// Stop pending timers and let an active report finish before replacing mocks.
func (suite *HealthCheckTestSuite) TearDownTest() {
	h := suite.healthCheck.(*HealthCheck)
	suite.NoError(h.ModuleStop())
	if h.healthJob != nil {
		select {
		case <-h.healthJob.Done():
		case <-time.After(time.Second):
			suite.T().Fatal("health polling did not stop")
		}
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

	mockEC2Identity := &identityMock.IAgentIdentityInner{}
	newEC2Identity = func(log log.T) identity.IAgentIdentityInner {
		return mockEC2Identity
	}
	availabilityZone := "us-east-1a"
	availabilityZoneId := "use1-az2"
	mockEC2Identity.On("IsIdentityEnvironment").Return(true)
	mockEC2Identity.On("AvailabilityZone").Return(availabilityZone, nil)
	mockEC2Identity.On("AvailabilityZoneId").Return(availabilityZoneId, nil)

	mockECSIdentity := &identityMock.IAgentIdentityInner{}
	newECSIdentity = func(log log.T) identity.IAgentIdentityInner {
		return mockECSIdentity
	}
	mockECSIdentity.On("IsIdentityEnvironment").Return(false)

	mockOnPremIdentity := &identityMock.IAgentIdentityInner{}
	newOnPremIdentity = func(log log.T, config *appconfig.SsmagentConfig) identity.IAgentIdentityInner {
		return mockOnPremIdentity
	}
	mockOnPremIdentity.On("IsIdentityEnvironment").Return(false)

	ssmConnectionChannel := "ssmmessages"
	sourceId := ""
	sourceLocation := ""
	sourceType := ""
	computerName := ""
	var ableToOpenMGSConnection uint32

	// reset
	suite.resetConnectionChannel()

	atomic.StoreUint32(&ableToOpenMGSConnection, 1)
	setConnectionGoRoutine := make(chan bool, 1)
	go func() {
		ssmconnectionchannel.SetConnectionChannel(suite.contextMock, ssmconnectionchannel.MGSSuccess)
		setConnectionGoRoutine <- false
	}()

	mdsSwitchFlag := <-ssmconnectionchannel.GetMDSSwitchChannel()
	suite.False(mdsSwitchFlag, "MDS flag is invalid")

	// Turn on the mock method
	suite.contextMock.On("AppConfig").Return(*appconfigMock)
	suite.serviceMock.On("UpdateInstanceInformation", mock.Anything, version.Version, "Active", AgentName, availabilityZone, availabilityZoneId, ssmConnectionChannel, sourceId, sourceType, sourceLocation, computerName).Return(nil, nil)
	suite.NoError(suite.healthCheck.ModuleExecute())
	job := suite.healthCheck.(*HealthCheck).healthJob
	// Repeated starts must not introduce another immediate report or poll loop.
	suite.NoError(suite.healthCheck.ModuleExecute())
	suite.Same(job, suite.healthCheck.(*HealthCheck).healthJob)
	// Allow the asynchronous initial health report to complete.
	time.Sleep(100 * time.Millisecond)
	// Assert the UpdateInstanceInformation get called in updateHealth() function, and the agent status is same as input.
	suite.serviceMock.AssertCalled(suite.T(), "UpdateInstanceInformation", mock.Anything, version.Version, "Active", AgentName, availabilityZone, availabilityZoneId, ssmConnectionChannel, sourceId, sourceType, sourceLocation, computerName)

	select {
	case <-setConnectionGoRoutine:
		break
	case <-time.After(2 * time.Second):
		assert.Fail(suite.T(), "setConnection Go Routine not killed")
	}
}

// Testing the ModuleExecute method
func (suite *HealthCheckTestSuite) TestModuleExecuteWithOnPremIdentity() {
	// Initialize the appconfigMock with HealthFrequencyMinutes as every five minute
	appconfigMock := &appconfig.SsmagentConfig{
		Ssm: appconfig.SsmCfg{
			HealthFrequencyMinutes: appconfig.DefaultSsmHealthFrequencyMinutes,
		},
	}

	mockEC2Identity := &identityMock.IAgentIdentityInner{}
	newEC2Identity = func(log log.T) identity.IAgentIdentityInner {
		return mockEC2Identity
	}

	availabilityZone := "us-east-1a"
	availabilityZoneId := "use1-az2"
	mockEC2Identity.On("IsIdentityEnvironment").Return(true)
	mockEC2Identity.On("AvailabilityZone").Return(availabilityZone, nil)
	mockEC2Identity.On("AvailabilityZoneId").Return(availabilityZoneId, nil)

	mockOnPremIdentity := &identityMock.IAgentIdentityInner{}
	newOnPremIdentity = func(log log.T, config *appconfig.SsmagentConfig) identity.IAgentIdentityInner {
		return mockOnPremIdentity
	}
	mockOnPremIdentity.On("IsIdentityEnvironment").Return(true)

	mockECSIdentity := &identityMock.IAgentIdentityInner{}
	newECSIdentity = func(log log.T) identity.IAgentIdentityInner {
		return mockECSIdentity
	}
	mockECSIdentity.On("IsIdentityEnvironment").Return(false)

	// reset
	suite.resetConnectionChannel()

	ssmConnectionChannel := "ssmmessages"
	sourceId := ""
	sourceLocation := ""
	sourceType := ""
	computerName := ""

	var ableToOpenMGSConnection uint32
	atomic.StoreUint32(&ableToOpenMGSConnection, 1)
	setConnectionGoRoutine := make(chan bool, 1)
	go func() {
		ssmconnectionchannel.SetConnectionChannel(suite.contextMock, ssmconnectionchannel.MGSSuccess)
		setConnectionGoRoutine <- false
	}()

	mdsSwitchFlag := <-ssmconnectionchannel.GetMDSSwitchChannel()
	suite.False(mdsSwitchFlag, "MDS flag is invalid")

	// Turn on the mock method
	suite.contextMock.On("AppConfig").Return(*appconfigMock)
	suite.serviceMock.On("UpdateInstanceInformation", mock.Anything, version.Version, "Active", AgentName, "", "", ssmConnectionChannel, sourceId, sourceType, sourceLocation, computerName).Return(nil, nil)
	suite.healthCheck.ModuleExecute()
	// Allow the asynchronous initial health report to complete.
	time.Sleep(100 * time.Millisecond)
	// Assert the UpdateInstanceInformation get called in updateHealth() function, and the agent status is same as input.
	suite.serviceMock.AssertCalled(suite.T(), "UpdateInstanceInformation", mock.Anything, version.Version, "Active", AgentName, "", "", ssmConnectionChannel, sourceId, sourceType, sourceLocation, computerName)
	suite.serviceMock.AssertNotCalled(suite.T(), "IsIdentityEnvironment", true)

	select {
	case <-setConnectionGoRoutine:
		break
	case <-time.After(2 * time.Second):
		assert.Fail(suite.T(), "set connection go routine not killed")
	}
}

// Testing the ModuleExecute method
func (suite *HealthCheckTestSuite) TestModuleExecuteWithNilOnPremIdentity() {
	// Initialize the appconfigMock with HealthFrequencyMinutes as every five minute
	appconfigMock := &appconfig.SsmagentConfig{
		Ssm: appconfig.SsmCfg{
			HealthFrequencyMinutes: appconfig.DefaultSsmHealthFrequencyMinutes,
		},
	}

	mockEC2Identity := &identityMock.IAgentIdentityInner{}
	newEC2Identity = func(log log.T) identity.IAgentIdentityInner {
		return mockEC2Identity
	}

	availabilityZone := "us-east-1a"
	availabilityZoneId := "use1-az2"
	mockEC2Identity.On("IsIdentityEnvironment").Return(true)
	mockEC2Identity.On("AvailabilityZone").Return(availabilityZone, nil)
	mockEC2Identity.On("AvailabilityZoneId").Return(availabilityZoneId, nil)

	newOnPremIdentity = func(log log.T, config *appconfig.SsmagentConfig) identity.IAgentIdentityInner {
		return nil
	}

	mockECSIdentity := &identityMock.IAgentIdentityInner{}
	newECSIdentity = func(log log.T) identity.IAgentIdentityInner {
		return mockECSIdentity
	}
	mockECSIdentity.On("IsIdentityEnvironment").Return(false)

	// reset
	suite.resetConnectionChannel()

	ssmConnectionChannel := "ssmmessages"
	sourceId := ""
	sourceLocation := ""
	sourceType := ""
	computerName := ""
	var ableToOpenMGSConnection uint32
	atomic.StoreUint32(&ableToOpenMGSConnection, 1)
	setConnectionGoRoutine := make(chan bool, 1)
	go func() {
		ssmconnectionchannel.SetConnectionChannel(suite.contextMock, ssmconnectionchannel.MGSSuccess)
		setConnectionGoRoutine <- false

	}()

	mdsSwitchFlag := <-ssmconnectionchannel.GetMDSSwitchChannel()
	suite.False(mdsSwitchFlag, "MDS flag is invalid")

	// Turn on the mock method
	suite.contextMock.On("AppConfig").Return(*appconfigMock)
	suite.serviceMock.On("UpdateInstanceInformation", mock.Anything, version.Version, "Active", AgentName, availabilityZone, availabilityZoneId, ssmConnectionChannel, sourceId, sourceType, sourceLocation, computerName).Return(nil, nil)
	suite.healthCheck.ModuleExecute()
	// Allow the asynchronous initial health report to complete.
	time.Sleep(100 * time.Millisecond)
	// Assert the UpdateInstanceInformation get called in updateHealth() function, and the agent status is same as input.
	suite.serviceMock.AssertCalled(suite.T(), "UpdateInstanceInformation", mock.Anything, version.Version, "Active", AgentName, availabilityZone, availabilityZoneId, ssmConnectionChannel, sourceId, sourceType, sourceLocation, computerName)
	select {
	case <-setConnectionGoRoutine:
		break
	case <-time.After(2 * time.Second):
		assert.Fail(suite.T(), "set connection go routine not killed")
	}
}

// Testing the ModuleExecute method with MDS connection
func (suite *HealthCheckTestSuite) TestModuleExecuteWithMDSConnection() {
	// Initialize the appconfigMock with HealthFrequencyMinutes as every five minute
	appconfigMock := &appconfig.SsmagentConfig{
		Ssm: appconfig.SsmCfg{
			HealthFrequencyMinutes: appconfig.DefaultSsmHealthFrequencyMinutes,
		},
	}

	mockEC2Identity := &identityMock.IAgentIdentityInner{}
	newEC2Identity = func(log log.T) identity.IAgentIdentityInner {
		return mockEC2Identity
	}
	availabilityZone := "us-east-1a"
	availabilityZoneId := "use1-az2"
	mockEC2Identity.On("IsIdentityEnvironment").Return(true)
	mockEC2Identity.On("AvailabilityZone").Return(availabilityZone, nil)
	mockEC2Identity.On("AvailabilityZoneId").Return(availabilityZoneId, nil)

	mockECSIdentity := &identityMock.IAgentIdentityInner{}
	newECSIdentity = func(log log.T) identity.IAgentIdentityInner {
		return mockECSIdentity
	}
	mockECSIdentity.On("IsIdentityEnvironment").Return(false)

	mockOnPremIdentity := &identityMock.IAgentIdentityInner{}
	newOnPremIdentity = func(log log.T, config *appconfig.SsmagentConfig) identity.IAgentIdentityInner {
		return mockOnPremIdentity
	}
	mockOnPremIdentity.On("IsIdentityEnvironment").Return(false)

	// reset
	suite.resetConnectionChannel()

	ssmConnectionChannel := "ec2messages"
	sourceId := ""
	sourceLocation := ""
	sourceType := ""
	computerName := ""
	var ableToOpenMGSConnection uint32
	atomic.StoreUint32(&ableToOpenMGSConnection, 0)
	setConnectionGoRoutine := make(chan bool, 1)
	go func() {
		ssmconnectionchannel.SetConnectionChannel(suite.contextMock, ssmconnectionchannel.MGSFailedDueToAccessDenied)
		setConnectionGoRoutine <- false

	}()
	time.Sleep(1 * time.Second)

	// Turn on the mock method
	suite.contextMock.On("AppConfig").Return(*appconfigMock)
	suite.serviceMock.On("UpdateInstanceInformation", mock.Anything, version.Version, "Active", AgentName, availabilityZone, availabilityZoneId, ssmConnectionChannel, sourceId, sourceType, sourceLocation, computerName).Return(nil, nil)
	suite.healthCheck.ModuleExecute()
	// Allow the asynchronous initial health report to complete.
	time.Sleep(100 * time.Millisecond)
	// Assert the UpdateInstanceInformation get called in updateHealth() function, and the agent status is same as input.
	suite.serviceMock.AssertCalled(suite.T(), "UpdateInstanceInformation", mock.Anything, version.Version, "Active", AgentName, availabilityZone, availabilityZoneId, ssmConnectionChannel, sourceId, sourceType, sourceLocation, computerName)
	select {
	case <-setConnectionGoRoutine:
		break
	case <-time.After(2 * time.Second):
		assert.Fail(suite.T(), "set connection go routine not killed")
	}
}

func (suite *HealthCheckTestSuite) resetConnectionChannel() {
	go func() {
		ssmconnectionchannel.SetConnectionChannel(suite.contextMock, ssmconnectionchannel.MGSFailedDueToAccessDenied)
	}()
	done := make(chan struct{})
	go func() {
		select {
		case <-time.After(500 * time.Millisecond):
		case <-ssmconnectionchannel.GetMDSSwitchChannel():
		}
		close(done)
	}()
	<-done
}

func (suite *HealthCheckTestSuite) TestModuleStopWithHealthJob() {
	job, err := polling.Start(suite.logMock, func() {}, 5, 30*time.Second, polling.Jitter{})
	suite.Require().NoError(err)
	suite.healthCheck.(*HealthCheck).healthJob = job
	suite.NoError(suite.healthCheck.ModuleStop())
	suite.NoError(suite.healthCheck.ModuleStop())
	select {
	case <-job.Done():
	case <-time.After(time.Second):
		suite.T().Fatal("ModuleStop did not cancel the pending health job")
	}
}

func (suite *HealthCheckTestSuite) TestModuleStopWithoutHealthJob() {
	suite.NoError(suite.healthCheck.ModuleStop())
}

func TestHealthPollingUsesConfiguredFrequency(t *testing.T) {
	for _, test := range []struct{ configured, expected int }{
		{5, 5}, {12, 12}, {60, 60}, {0, 5}, {4, 5}, {61, 5},
	} {
		config := appconfig.DefaultConfig()
		config.Ssm.HealthFrequencyMinutes = test.configured
		ctx := context.NewMockDefaultWithConfig(config)
		h := &HealthCheck{context: ctx}
		require.Equal(t, test.expected, h.scheduleInMinutes())
	}
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

type MockProvider struct {
	mock.Mock
}

func (m *MockProvider) AvailabilityZone() (string, error) {
	args := m.Called()
	return args.String(0), args.Error(1)
}

func (m *MockProvider) AvailabilityZoneId() (string, error) {
	args := m.Called()
	return args.String(0), args.Error(1)
}

func (m *MockProvider) SourceId() (string, error) {
	args := m.Called()
	return args.String(0), args.Error(1)
}

func (m *MockProvider) SourceType() string {
	args := m.Called()
	return args.String(0)
}

func (m *MockProvider) SourceLocation() (string, error) {
	args := m.Called()
	return args.String(0), args.Error(1)
}

func (m *MockProvider) ComputerName() (string, error) {
	args := m.Called()
	return args.String(0), args.Error(1)
}

func resetConnectionChannelForTest(ctx *context.Mock) {
	go func() {
		ssmconnectionchannel.SetConnectionChannel(ctx, ssmconnectionchannel.MGSFailedDueToAccessDenied)
	}()
	go func() {
		select {
		case <-time.After(500 * time.Millisecond):
			break
		case <-ssmconnectionchannel.GetMDSSwitchChannel():
			break
		}
	}()
	time.Sleep(500 * time.Millisecond)
}

func setupProviderTest(t *testing.T, mockProvider identity.IProvider) (*HealthCheck, *ssmMock.Service) {
	t.Helper()

	healthModule = nil

	ctxMock := context.NewMockDefault()
	serviceMock := new(ssmMock.Service)
	stopPolicy := sdkutil.NewStopPolicy("healthTest", 10)

	h := &HealthCheck{
		healthCheckStopPolicy: stopPolicy,
		context:               ctxMock,
		service:               serviceMock,
	}

	originalGetIdentityProvider := getIdentityProvider
	getIdentityProvider = func(log log.T, provider string, appConfig *appconfig.SsmagentConfig) identity.IProvider {
		return mockProvider
	}
	t.Cleanup(func() { getIdentityProvider = originalGetIdentityProvider })

	resetConnectionChannelForTest(ctxMock)

	go func() {
		ssmconnectionchannel.SetConnectionChannel(ctxMock, ssmconnectionchannel.MGSSuccess)
	}()
	<-ssmconnectionchannel.GetMDSSwitchChannel()

	return h, serviceMock
}

func setupNilProviderTest(t *testing.T) (*HealthCheck, *ssmMock.Service) {
	t.Helper()

	healthModule = nil

	ctxMock := context.NewMockDefault()
	serviceMock := new(ssmMock.Service)
	stopPolicy := sdkutil.NewStopPolicy("healthTest", 10)

	h := &HealthCheck{
		healthCheckStopPolicy: stopPolicy,
		context:               ctxMock,
		service:               serviceMock,
	}

	originalGetIdentityProvider := getIdentityProvider
	getIdentityProvider = func(log log.T, provider string, appConfig *appconfig.SsmagentConfig) identity.IProvider {
		return nil
	}
	t.Cleanup(func() { getIdentityProvider = originalGetIdentityProvider })

	originalNewEC2Identity := newEC2Identity
	originalNewECSIdentity := newECSIdentity
	originalNewOnPremIdentity := newOnPremIdentity
	t.Cleanup(func() {
		newEC2Identity = originalNewEC2Identity
		newECSIdentity = originalNewECSIdentity
		newOnPremIdentity = originalNewOnPremIdentity
	})

	resetConnectionChannelForTest(ctxMock)

	go func() {
		ssmconnectionchannel.SetConnectionChannel(ctxMock, ssmconnectionchannel.MGSSuccess)
	}()
	<-ssmconnectionchannel.GetMDSSwitchChannel()

	return h, serviceMock
}

func TestBugCondition_SingleMethodFailure_AvailabilityZoneError(t *testing.T) {
	mockProvider := new(MockProvider)

	mockProvider.On("AvailabilityZone").Return("", errors.New("metadata service unavailable"))
	mockProvider.On("AvailabilityZoneId").Return("use1-az2", nil)
	mockProvider.On("SourceId").Return("i-1234567890abcdef0", nil)
	mockProvider.On("SourceType").Return("EC2")
	mockProvider.On("SourceLocation").Return("us-east-1", nil)
	mockProvider.On("ComputerName").Return("ip-10-0-0-1", nil)

	h, serviceMock := setupProviderTest(t, mockProvider)

	serviceMock.On("UpdateInstanceInformation",
		mock.Anything, mock.Anything, mock.Anything, mock.Anything,
		mock.Anything, mock.Anything, mock.Anything, mock.Anything,
		mock.Anything, mock.Anything, mock.Anything,
	).Return(nil, nil)

	h.updateHealth()

	serviceMock.AssertNotCalled(t, "UpdateInstanceInformation",
		mock.Anything, mock.Anything, mock.Anything, mock.Anything,
		mock.Anything, mock.Anything, mock.Anything, mock.Anything,
		mock.Anything, mock.Anything, mock.Anything,
	)
}

func TestBugCondition_AllMethodsFailure(t *testing.T) {
	mockProvider := new(MockProvider)

	mockProvider.On("AvailabilityZone").Return("", errors.New("AZ unavailable"))
	mockProvider.On("AvailabilityZoneId").Return("", errors.New("AZ ID unavailable"))
	mockProvider.On("SourceId").Return("", errors.New("source ID unavailable"))
	mockProvider.On("SourceType").Return("EC2")
	mockProvider.On("SourceLocation").Return("", errors.New("source location unavailable"))
	mockProvider.On("ComputerName").Return("", errors.New("computer name unavailable"))

	h, serviceMock := setupProviderTest(t, mockProvider)

	serviceMock.On("UpdateInstanceInformation",
		mock.Anything, mock.Anything, mock.Anything, mock.Anything,
		mock.Anything, mock.Anything, mock.Anything, mock.Anything,
		mock.Anything, mock.Anything, mock.Anything,
	).Return(nil, nil)

	h.updateHealth()

	serviceMock.AssertNotCalled(t, "UpdateInstanceInformation",
		mock.Anything, mock.Anything, mock.Anything, mock.Anything,
		mock.Anything, mock.Anything, mock.Anything, mock.Anything,
		mock.Anything, mock.Anything, mock.Anything,
	)
}

func TestBugCondition_ComputerNameOnlyFailure(t *testing.T) {
	mockProvider := new(MockProvider)

	mockProvider.On("AvailabilityZone").Return("us-east-1a", nil)
	mockProvider.On("AvailabilityZoneId").Return("use1-az2", nil)
	mockProvider.On("SourceId").Return("i-1234567890abcdef0", nil)
	mockProvider.On("SourceType").Return("EC2")
	mockProvider.On("SourceLocation").Return("us-east-1", nil)
	mockProvider.On("ComputerName").Return("", errors.New("hostname lookup failed"))

	h, serviceMock := setupProviderTest(t, mockProvider)

	serviceMock.On("UpdateInstanceInformation",
		mock.Anything, mock.Anything, mock.Anything, mock.Anything,
		mock.Anything, mock.Anything, mock.Anything, mock.Anything,
		mock.Anything, mock.Anything, mock.Anything,
	).Return(nil, nil)

	h.updateHealth()

	serviceMock.AssertNotCalled(t, "UpdateInstanceInformation",
		mock.Anything, mock.Anything, mock.Anything, mock.Anything,
		mock.Anything, mock.Anything, mock.Anything, mock.Anything,
		mock.Anything, mock.Anything, mock.Anything,
	)
}

func TestPreservation_SuccessfulProvider(t *testing.T) {
	mockProvider := new(MockProvider)

	mockProvider.On("AvailabilityZone").Return("us-east-1a", nil)
	mockProvider.On("AvailabilityZoneId").Return("use1-az2", nil)
	mockProvider.On("SourceId").Return("i-1234567890abcdef0", nil)
	mockProvider.On("SourceType").Return("EC2")
	mockProvider.On("SourceLocation").Return("us-east-1", nil)
	mockProvider.On("ComputerName").Return("ip-10-0-0-1", nil)

	h, serviceMock := setupProviderTest(t, mockProvider)

	serviceMock.On("UpdateInstanceInformation",
		mock.Anything, version.Version, "Active", AgentName,
		"us-east-1a", "use1-az2", "ssmmessages",
		"i-1234567890abcdef0", "EC2", "us-east-1", "ip-10-0-0-1",
	).Return(nil, nil)

	h.updateHealth()

	serviceMock.AssertCalled(t, "UpdateInstanceInformation",
		mock.Anything, version.Version, "Active", AgentName,
		"us-east-1a", "use1-az2", "ssmmessages",
		"i-1234567890abcdef0", "EC2", "us-east-1", "ip-10-0-0-1",
	)
}

func TestPreservation_NilProvider_EC2Fallback(t *testing.T) {
	h, serviceMock := setupNilProviderTest(t)

	mockOnPrem := &identityMock.IAgentIdentityInner{}
	mockOnPrem.On("IsIdentityEnvironment").Return(false)
	newOnPremIdentity = func(log log.T, config *appconfig.SsmagentConfig) identity.IAgentIdentityInner {
		return mockOnPrem
	}

	mockEC2 := &identityMock.IAgentIdentityInner{}
	mockEC2.On("IsIdentityEnvironment").Return(true)
	mockEC2.On("AvailabilityZone").Return("us-west-2b", nil)
	mockEC2.On("AvailabilityZoneId").Return("usw2-az1", nil)
	newEC2Identity = func(log log.T) identity.IAgentIdentityInner {
		return mockEC2
	}

	mockECS := &identityMock.IAgentIdentityInner{}
	mockECS.On("IsIdentityEnvironment").Return(false)
	newECSIdentity = func(log log.T) identity.IAgentIdentityInner {
		return mockECS
	}

	serviceMock.On("UpdateInstanceInformation",
		mock.Anything, version.Version, "Active", AgentName,
		"us-west-2b", "usw2-az1", "ssmmessages",
		"", "", "", "",
	).Return(nil, nil)

	h.updateHealth()

	serviceMock.AssertCalled(t, "UpdateInstanceInformation",
		mock.Anything, version.Version, "Active", AgentName,
		"us-west-2b", "usw2-az1", "ssmmessages",
		"", "", "", "",
	)
}

func TestPreservation_NilProvider_OnPremIdentity(t *testing.T) {
	h, serviceMock := setupNilProviderTest(t)

	mockOnPrem := &identityMock.IAgentIdentityInner{}
	mockOnPrem.On("IsIdentityEnvironment").Return(true)
	newOnPremIdentity = func(log log.T, config *appconfig.SsmagentConfig) identity.IAgentIdentityInner {
		return mockOnPrem
	}

	serviceMock.On("UpdateInstanceInformation",
		mock.Anything, version.Version, "Active", AgentName,
		"", "", "ssmmessages",
		"", "", "", "",
	).Return(nil, nil)

	h.updateHealth()

	serviceMock.AssertCalled(t, "UpdateInstanceInformation",
		mock.Anything, version.Version, "Active", AgentName,
		"", "", "ssmmessages",
		"", "", "", "",
	)
}

func TestPreservation_NilProvider_NoIdentity(t *testing.T) {
	h, serviceMock := setupNilProviderTest(t)

	mockOnPrem := &identityMock.IAgentIdentityInner{}
	mockOnPrem.On("IsIdentityEnvironment").Return(false)
	newOnPremIdentity = func(log log.T, config *appconfig.SsmagentConfig) identity.IAgentIdentityInner {
		return mockOnPrem
	}

	mockEC2 := &identityMock.IAgentIdentityInner{}
	mockEC2.On("IsIdentityEnvironment").Return(false)
	newEC2Identity = func(log log.T) identity.IAgentIdentityInner {
		return mockEC2
	}

	mockECS := &identityMock.IAgentIdentityInner{}
	mockECS.On("IsIdentityEnvironment").Return(false)
	newECSIdentity = func(log log.T) identity.IAgentIdentityInner {
		return mockECS
	}

	serviceMock.On("UpdateInstanceInformation",
		mock.Anything, version.Version, "Active", AgentName,
		"", "", "ssmmessages",
		"", "", "", "",
	).Return(nil, nil)

	h.updateHealth()

	serviceMock.AssertCalled(t, "UpdateInstanceInformation",
		mock.Anything, version.Version, "Active", AgentName,
		"", "", "ssmmessages",
		"", "", "", "",
	)
}
