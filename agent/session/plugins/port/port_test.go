// Copyright 2019 Amazon.com, Inc. or its affiliates. All Rights Reserved.
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

// Package port implements session port plugin.
package port

import (
	"errors"
	"net"
	"testing"
	"time"

	"github.com/aws/amazon-ssm-agent/agent/appconfig"
	"github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/aws/amazon-ssm-agent/agent/contracts"
	iohandlermocks "github.com/aws/amazon-ssm-agent/agent/framework/processor/executer/iohandler/mock"
	"github.com/aws/amazon-ssm-agent/agent/log"
	mgsContracts "github.com/aws/amazon-ssm-agent/agent/session/contracts"
	dataChannelMock "github.com/aws/amazon-ssm-agent/agent/session/datachannel/mocks"
	portSessionMock "github.com/aws/amazon-ssm-agent/agent/session/plugins/port/mocks"
	"github.com/aws/amazon-ssm-agent/agent/task"
	"github.com/aws/amazon-ssm-agent/common/identity"
	identityMock "github.com/aws/amazon-ssm-agent/common/identity/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
	"github.com/twinj/uuid"
)

var (
	mockLog                     = log.NewMockLog()
	configuration               = contracts.Configuration{Properties: map[string]interface{}{"portNumber": port}, SessionId: sessionId}
	configurationPF             = contracts.Configuration{Properties: map[string]interface{}{"portNumber": port, "type": "LocalPortForwarding"}, SessionId: sessionId}
	configurationWithRemoteHost = contracts.Configuration{Properties: map[string]interface{}{"portNumber": port, "host": remoteHost, "type": "LocalPortForwarding"}, SessionId: sessionId}
	payload                     = []byte("testPayload")
	messageId                   = "dd01e56b-ff48-483e-a508-b5f073f31b16"
	schemaVersion               = uint32(1)
	createdDate                 = uint64(1503434274948)
	clientVersion               = "1.2.0"
	sessionId                   = "sessionId"
	port                        = "8080"
	localhost                   = "localhost"
	remoteHost                  = "https://remote.server.com"
)

type PortTestSuite struct {
	suite.Suite
	mockContext     *context.Mock
	mockLog         log.T
	mockCancelFlag  *task.MockCancelFlag
	mockDataChannel *dataChannelMock.IDataChannel
	mockIohandler   *iohandlermocks.MockIOHandler
	mockPortSession *portSessionMock.IPortSession
	plugin          *PortPlugin
}

// Testing initializeParameters
func TestInitializeParametersWhenPortTypeIsNil(t *testing.T) {
	mockDataChannel := &dataChannelMock.IDataChannel{}
	mockDataChannel.On("GetClientVersion").Return(clientVersion)

	portPlugin := &PortPlugin{
		context:     context.NewMockDefault(),
		dataChannel: mockDataChannel,
		cancelled:   make(chan struct{}),
	}

	portPlugin.initializeParameters(configuration)
	assert.IsType(t, &BasicPortSession{}, portPlugin.session)
	basicPortSession := portPlugin.session.(*BasicPortSession)
	assert.Equal(t, net.JoinHostPort(localhost, port), basicPortSession.destinationAddress)
	mockDataChannel.AssertExpectations(t)
}

func TestInitializeParametersWhenPortTypeIsLocalPortForwarding(t *testing.T) {
	mockDataChannel := &dataChannelMock.IDataChannel{}
	mockDataChannel.On("GetClientVersion").Return(clientVersion)

	portPlugin := &PortPlugin{
		context:     context.NewMockDefault(),
		dataChannel: mockDataChannel,
		cancelled:   make(chan struct{}),
	}

	portPlugin.initializeParameters(configurationPF)
	assert.IsType(t, &MuxPortSession{}, portPlugin.session)
	muxPortSession := portPlugin.session.(*MuxPortSession)
	assert.Equal(t, net.JoinHostPort(localhost, port), muxPortSession.destinationAddress)
	mockDataChannel.AssertExpectations(t)
}

func TestInitializeParametersWhenPortTypeIsLocalPortForwardingAndOldClient(t *testing.T) {
	mockDataChannel := &dataChannelMock.IDataChannel{}
	mockDataChannel.On("GetClientVersion").Return("1.0.0")

	portPlugin := &PortPlugin{
		context:     context.NewMockDefault(),
		dataChannel: mockDataChannel,
		cancelled:   make(chan struct{}),
	}

	portPlugin.initializeParameters(configurationPF)
	assert.IsType(t, &BasicPortSession{}, portPlugin.session)
	basicPortSession := portPlugin.session.(*BasicPortSession)
	assert.Equal(t, net.JoinHostPort(localhost, port), basicPortSession.destinationAddress)
	mockDataChannel.AssertExpectations(t)
}

func TestInitializeParametersWhenHostIsProvided(t *testing.T) {
	mockDataChannel := &dataChannelMock.IDataChannel{}
	mockDataChannel.On("GetClientVersion").Return(clientVersion)

	portPlugin := &PortPlugin{
		context:     context.NewMockDefault(),
		dataChannel: mockDataChannel,
		cancelled:   make(chan struct{}),
	}
	mockIdentity := &identityMock.IAgentIdentityInner{}
	newEC2Identity = func(log log.T, _ *appconfig.SsmagentConfig) identity.IAgentIdentityInner {
		return mockIdentity
	}
	newECSIdentity = newEC2Identity
	mockIdentity.On("IsIdentityEnvironment").Return(true)
	mockMetadata := &identityMock.IMetadataIdentity{}
	getMetadataIdentity = func(agentIdentity identity.IAgentIdentityInner) (identity.IMetadataIdentity, bool) {
		return mockMetadata, true
	}
	mockMetadata.On("VpcPrimaryCIDRBlock").Return(map[string][]string{"ipv4": {"172.31.0.0/16"}, "ipv6": {"2600:1f18:64ad::/56"}}, nil)
	lookupHost = func(host string) ([]string, error) {
		if host == remoteHost {
			return []string{"127.0.0.1"}, nil
		}
		return []string{host}, nil
	}
	portPlugin.initializeParameters(configurationWithRemoteHost)
	assert.IsType(t, &MuxPortSession{}, portPlugin.session)
	muxPortSession := portPlugin.session.(*MuxPortSession)
	assert.Equal(t, net.JoinHostPort(remoteHost, port), muxPortSession.destinationAddress)
	mockDataChannel.AssertExpectations(t)
}

func (suite *PortTestSuite) SetupTest() {
	mockContext := context.NewMockDefault()
	mockCancelFlag := &task.MockCancelFlag{}
	mockDataChannel := &dataChannelMock.IDataChannel{}
	mockIohandler := new(iohandlermocks.MockIOHandler)
	mockPortSession := &portSessionMock.IPortSession{}

	suite.mockContext = mockContext
	suite.mockCancelFlag = mockCancelFlag
	suite.mockLog = mockLog
	suite.mockDataChannel = mockDataChannel
	suite.mockIohandler = mockIohandler
	suite.mockPortSession = mockPortSession
	suite.plugin = &PortPlugin{
		context:     mockContext,
		dataChannel: mockDataChannel,
		cancelled:   make(chan struct{}),
	}

	mockIdentity := &identityMock.IAgentIdentityInner{}
	newEC2Identity = func(log log.T, _ *appconfig.SsmagentConfig) identity.IAgentIdentityInner {
		return mockIdentity
	}
	newECSIdentity = newEC2Identity
	mockIdentity.On("IsIdentityEnvironment").Return(true)
	mockMetadata := &identityMock.IMetadataIdentity{}
	getMetadataIdentity = func(agentIdentity identity.IAgentIdentityInner) (identity.IMetadataIdentity, bool) {
		return mockMetadata, true
	}
	mockMetadata.On("VpcPrimaryCIDRBlock").Return(map[string][]string{"ipv4": {"172.31.0.0/16"}, "ipv6": {"2600:1f18:64ad::/56"}}, nil)
}

// Testing Name
func (suite *PortTestSuite) TestName() {
	rst := suite.plugin.name()
	assert.Equal(suite.T(), rst, appconfig.PluginNamePort)
}

// Testing GetPluginParameters
func (suite *PortTestSuite) TestGetPluginParameters() {
	config := map[string]interface{}{"portNumber": "22", "type": "LocalPortForwarding"}
	assert.Equal(suite.T(), suite.plugin.GetPluginParameters(config), config)
}

// Testing Execute
func (suite *PortTestSuite) TestExecuteWhenCancelFlagIsShutDown() {
	suite.mockCancelFlag.On("ShutDown").Return(true)
	suite.mockIohandler.On("MarkAsShutdown").Return(nil)

	suite.plugin.Execute(
		configuration,
		suite.mockCancelFlag,
		suite.mockIohandler,
		suite.mockDataChannel)

	suite.mockCancelFlag.AssertExpectations(suite.T())
	suite.mockIohandler.AssertExpectations(suite.T())
}

func (suite *PortTestSuite) TestExecuteWhenCancelFlagIsCancelled() {
	suite.mockCancelFlag.On("Canceled").Return(true)
	suite.mockCancelFlag.On("ShutDown").Return(false)
	suite.mockIohandler.On("MarkAsCancelled").Return(nil)

	suite.plugin.Execute(
		configuration,
		suite.mockCancelFlag,
		suite.mockIohandler,
		suite.mockDataChannel)

	suite.mockCancelFlag.AssertExpectations(suite.T())
	suite.mockIohandler.AssertExpectations(suite.T())
}

func (suite *PortTestSuite) TestExecuteWithInvalidPortNumber() {
	suite.mockCancelFlag.On("Canceled").Return(false)
	suite.mockCancelFlag.On("ShutDown").Return(false)
	suite.mockIohandler.On("SetStatus", contracts.ResultStatusFailed).Return(nil)
	suite.mockIohandler.On("SetExitCode", 1).Return(nil)
	suite.mockIohandler.On("SetOutput", mock.Anything).Return()

	suite.plugin.Execute(
		contracts.Configuration{Properties: map[string]interface{}{"portNumber": ""}, SessionId: "sessionId"},
		suite.mockCancelFlag,
		suite.mockIohandler,
		suite.mockDataChannel)

	suite.mockCancelFlag.AssertExpectations(suite.T())
	suite.mockIohandler.AssertExpectations(suite.T())
}

func (suite *PortTestSuite) TestExecuteWhenInitializeSessionReturnsError() {
	suite.mockCancelFlag.On("Canceled").Return(false)
	suite.mockCancelFlag.On("ShutDown").Return(false)
	suite.mockIohandler.On("SetStatus", contracts.ResultStatusFailed).Return(nil)
	suite.mockIohandler.On("SetExitCode", 1).Return(nil)
	suite.mockIohandler.On("SetOutput", mock.Anything).Return()
	suite.mockDataChannel.On("GetClientVersion").Return(clientVersion)

	GetSession = func(context context.T, parameters PortParameters, cancelled chan struct{}, clientVersion string, sessionId string) (IPortSession, error) {
		return nil, errors.New("failed to initialize session")
	}

	suite.plugin.Execute(
		configuration,
		suite.mockCancelFlag,
		suite.mockIohandler,
		suite.mockDataChannel)

	suite.mockCancelFlag.AssertExpectations(suite.T())
	suite.mockIohandler.AssertExpectations(suite.T())
}

func (suite *PortTestSuite) TestExecute() {
	suite.mockCancelFlag.On("Canceled").Return(false)
	suite.mockCancelFlag.On("ShutDown").Return(false)
	suite.mockCancelFlag.On("Wait").Return(task.Completed)
	suite.mockIohandler.On("SetExitCode", 0).Return(nil)
	suite.mockIohandler.On("SetStatus", contracts.ResultStatusSuccess).Return()
	suite.mockDataChannel.On("GetClientVersion").Return(clientVersion)
	suite.mockPortSession.On("InitializeSession", mock.Anything).Return(nil)
	suite.mockPortSession.On("WritePump", suite.mockDataChannel).WaitUntil(time.After(time.Second)).Return(0)
	suite.mockPortSession.On("Stop").Return()

	GetSession = func(context context.T, parameters PortParameters, cancelled chan struct{}, clientVersion string, sessionId string) (IPortSession, error) {
		return suite.mockPortSession, nil
	}

	suite.plugin.Execute(
		configuration,
		suite.mockCancelFlag,
		suite.mockIohandler,
		suite.mockDataChannel)

	suite.mockCancelFlag.AssertExpectations(suite.T())
	suite.mockIohandler.AssertExpectations(suite.T())
	suite.mockDataChannel.AssertExpectations(suite.T())
	suite.mockPortSession.AssertExpectations(suite.T())
}

// Testing InputStreamHandler
func (suite *PortTestSuite) TestInputStreamHandler() {
	suite.plugin.session = suite.mockPortSession
	suite.mockPortSession.On("HandleStreamMessage", getAgentMessage(uint32(mgsContracts.Output), payload)).Return(nil)
	suite.mockPortSession.On("IsConnectionAvailable").Return(true)
	suite.plugin.InputStreamMessageHandler(suite.mockLog, getAgentMessage(uint32(mgsContracts.Output), payload))
	suite.mockPortSession.AssertExpectations(suite.T())
}

func (suite *PortTestSuite) TestInputStreamHandlerSessionNotReady() {
	suite.plugin.InputStreamMessageHandler(suite.mockLog, getAgentMessage(uint32(mgsContracts.Output), payload))
	suite.mockPortSession.AssertExpectations(suite.T())
}

func (suite *PortTestSuite) TestInputStreamHandlerConnectionNotReady() {
	suite.plugin.session = suite.mockPortSession
	suite.mockPortSession.On("IsConnectionAvailable").Return(false)
	suite.plugin.InputStreamMessageHandler(suite.mockLog, getAgentMessage(uint32(mgsContracts.Output), payload))
	suite.mockPortSession.AssertExpectations(suite.T())
}

func (suite *PortTestSuite) TestValidateParametersWhenInvalidPort() {
	err := suite.plugin.validateParameters(PortParameters{PortNumber: ""}, configuration)
	assert.Contains(suite.T(), err.Error(), "Port number is empty in session properties.")
}

func (suite *PortTestSuite) TestValidateParametersWhenVPCHostNotAllowed() {
	mockContext := &context.Mock{}
	suite.plugin.context = mockContext

	mockContext.On("AppConfig").Return(appconfig.SsmagentConfig{Mgs: appconfig.MgsConfig{DeniedPortForwardingRemoteIPs: []string{"169.254.169.254", "fd00:ec2::254", "169.254.169.253", "fd00:ec2::253", "169.254.169.123", "169.254.169.250"}}})
	mockContext.On("Log").Return(mockLog)

	err := suite.plugin.validateParameters(PortParameters{PortNumber: "80", Host: "172.31.0.2"}, configuration)
	assert.Contains(suite.T(), err.Error(), "Forwarding to IP address 172.31.0.2 is forbidden.")
	err = suite.plugin.validateParameters(PortParameters{PortNumber: "80", Host: "2600:1f18:64ad::2"}, configuration)
	assert.Contains(suite.T(), err.Error(), "Forwarding to IP address 2600:1f18:64ad::2 is forbidden.")

	mockContext.AssertExpectations(suite.T())
}

func (suite *PortTestSuite) TestValidateParametersWhenDefaultDenylistHostNotAllowed() {
	mockContext := &context.Mock{}
	suite.plugin.context = mockContext

	mockContext.On("AppConfig").Return(appconfig.SsmagentConfig{Mgs: appconfig.MgsConfig{DeniedPortForwardingRemoteIPs: []string{"169.254.169.254", "fd00:ec2::254", "169.254.169.253", "fd00:ec2::253", "169.254.169.123", "169.254.169.250"}}})
	mockContext.On("Log").Return(mockLog)

	err := suite.plugin.validateParameters(PortParameters{PortNumber: "80", Host: "169.254.169.253"}, configuration)
	assert.Contains(suite.T(), err.Error(), "Forwarding to IP address 169.254.169.253 is forbidden.")
	err = suite.plugin.validateParameters(PortParameters{PortNumber: "80", Host: "fd00:ec2::253"}, configuration)
	assert.Contains(suite.T(), err.Error(), "Forwarding to IP address fd00:ec2::253 is forbidden.")
	err = suite.plugin.validateParameters(PortParameters{PortNumber: "80", Host: "169.254.169.254"}, configuration)
	assert.Contains(suite.T(), err.Error(), "Forwarding to IP address 169.254.169.254 is forbidden.")
	err = suite.plugin.validateParameters(PortParameters{PortNumber: "80", Host: "fd00:ec2::253"}, configuration)
	assert.Contains(suite.T(), err.Error(), "Forwarding to IP address fd00:ec2::253 is forbidden.")
	err = suite.plugin.validateParameters(PortParameters{PortNumber: "80", Host: "169.254.169.250"}, configuration)
	assert.Contains(suite.T(), err.Error(), "Forwarding to IP address 169.254.169.250 is forbidden.")
	err = suite.plugin.validateParameters(PortParameters{PortNumber: "80", Host: "169.254.169.123"}, configuration)
	assert.Contains(suite.T(), err.Error(), "Forwarding to IP address 169.254.169.123 is forbidden.")

	mockContext.AssertExpectations(suite.T())
}

func (suite *PortTestSuite) TestValidateParametersWhenValidHostAndPort() {
	mockContext := &context.Mock{}
	suite.plugin.context = mockContext

	mockContext.On("AppConfig").Return(appconfig.SsmagentConfig{Mgs: appconfig.MgsConfig{DeniedPortForwardingRemoteIPs: []string{"169.254.169.254", "fd00:ec2::254", "169.254.169.253", "fd00:ec2::253"}}})
	mockContext.On("Log").Return(mockLog)

	err := suite.plugin.validateParameters(PortParameters{PortNumber: "80", Host: "127.0.0.1"}, configuration)
	assert.Nil(suite.T(), err)

	mockContext.AssertExpectations(suite.T())
}

func (suite *PortTestSuite) TestCalculateAddressMethod() {
	expected := []string{"172.31.0.0", "172.31.0.1", "172.31.0.2", "172.31.0.3", "172.31.255.255", "2600:1f18:64ad::", "2600:1f18:64ad::1", "2600:1f18:64ad::2", "2600:1f18:64ad::3", "2600:1f18:64ad:ff:ffff:ffff:ffff:ffff"}
	ipaddresses := map[string][]string{"ipv4": {"172.31.0.0/16"}, "ipv6": {"2600:1f18:64ad::/56"}}
	actual := calculateAddress(ipaddresses)
	assert.Equal(suite.T(), expected, actual)
}

// Execute the test suite
func TestPortTestSuite(t *testing.T) {
	suite.Run(t, new(PortTestSuite))
}

// getAgentMessage constructs and returns AgentMessage with given sequenceNumber, messageType & payload
func getAgentMessage(payloadType uint32, payload []byte) mgsContracts.AgentMessage {
	messageUUID, _ := uuid.Parse(messageId)
	agentMessage := mgsContracts.AgentMessage{
		MessageType:    mgsContracts.InputStreamDataMessage,
		SchemaVersion:  schemaVersion,
		CreatedDate:    createdDate,
		SequenceNumber: 1,
		Flags:          2,
		MessageId:      messageUUID,
		PayloadType:    payloadType,
		Payload:        payload,
	}
	return agentMessage
}
