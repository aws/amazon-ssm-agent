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
	"testing"

	"github.com/aws/amazon-ssm-agent/agent/appconfig"
	"github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/aws/amazon-ssm-agent/agent/contracts"
	iohandlermocks "github.com/aws/amazon-ssm-agent/agent/framework/processor/executer/iohandler/mock"
	"github.com/aws/amazon-ssm-agent/agent/log"
	mgsContracts "github.com/aws/amazon-ssm-agent/agent/session/contracts"
	dataChannelMock "github.com/aws/amazon-ssm-agent/agent/session/datachannel/mocks"
	portSessionMock "github.com/aws/amazon-ssm-agent/agent/session/plugins/port/mocks"
	"github.com/aws/amazon-ssm-agent/agent/task"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
	"github.com/twinj/uuid"
)

var (
	mockLog         = log.NewMockLog()
	configuration   = contracts.Configuration{Properties: map[string]interface{}{"portNumber": "22"}, SessionId: "sessionId"}
	configurationPF = contracts.Configuration{Properties: map[string]interface{}{"portNumber": "22", "type": "LocalPortForwarding"}, SessionId: "sessionId"}
	payload         = []byte("testPayload")
	messageId       = "dd01e56b-ff48-483e-a508-b5f073f31b16"
	schemaVersion   = uint32(1)
	createdDate     = uint64(1503434274948)
	clientVersion   = "1.2.0"
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
		dataChannel: mockDataChannel,
		cancelled:   make(chan struct{}),
	}

	portPlugin.initializeParameters(mockLog, configuration)
	assert.IsType(t, &BasicPortSession{}, portPlugin.session)
	mockDataChannel.AssertExpectations(t)
}

func TestInitializeParametersWhenPortTypeIsLocalPortForwarding(t *testing.T) {
	mockDataChannel := &dataChannelMock.IDataChannel{}
	mockDataChannel.On("GetClientVersion").Return(clientVersion)

	portPlugin := &PortPlugin{
		dataChannel: mockDataChannel,
		cancelled:   make(chan struct{}),
	}

	portPlugin.initializeParameters(mockLog, configurationPF)
	assert.IsType(t, &MuxPortSession{}, portPlugin.session)
	mockDataChannel.AssertExpectations(t)
}

func TestInitializeParametersWhenPortTypeIsLocalPortForwardingAndOldClient(t *testing.T) {
	mockDataChannel := &dataChannelMock.IDataChannel{}
	mockDataChannel.On("GetClientVersion").Return("1.0.0")

	portPlugin := &PortPlugin{
		dataChannel: mockDataChannel,
		cancelled:   make(chan struct{}),
	}

	portPlugin.initializeParameters(mockLog, configurationPF)
	assert.IsType(t, &BasicPortSession{}, portPlugin.session)
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
		dataChannel: mockDataChannel,
		cancelled:   make(chan struct{}),
	}
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

	suite.plugin.Execute(suite.mockContext,
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

	suite.plugin.Execute(suite.mockContext,
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

	suite.plugin.Execute(suite.mockContext,
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

	GetSession = func(parameters PortParameters, cancelled chan struct{}, clientVersion string, sessionId string) (IPortSession, error) {
		return nil, errors.New("failed to initialize session")
	}

	suite.plugin.Execute(suite.mockContext,
		configuration,
		suite.mockCancelFlag,
		suite.mockIohandler,
		suite.mockDataChannel)

	suite.mockCancelFlag.AssertExpectations(suite.T())
	suite.mockIohandler.AssertExpectations(suite.T())
}

// todo: this unit test fails intermittently and need to be fixed
/*
func (suite *PortTestSuite) TestExecute() {
	suite.mockCancelFlag.On("Canceled").Return(false)
	suite.mockCancelFlag.On("ShutDown").Return(false)
	suite.mockCancelFlag.On("Wait").Return(task.Completed)
	suite.mockIohandler.On("SetExitCode", 0).Return(nil)
	suite.mockIohandler.On("SetStatus", contracts.ResultStatusSuccess).Return()
	suite.mockDataChannel.On("GetClientVersion").Return(clientVersion)
	suite.mockPortSession.On("InitializeSession", mock.Anything).Return(nil)
	suite.mockPortSession.On("WritePump", mock.Anything, suite.mockDataChannel).Return(0)
	suite.mockPortSession.On("Stop").Return()

	GetSession = func(parameters PortParameters, cancelled chan struct{}, clientVersion string, sessionId string) (IPortSession, error) {
		return suite.mockPortSession, nil
	}

	out, in := net.Pipe()
	defer in.Close()
	defer out.Close()
	DialCall = func(network string, address string) (net.Conn, error) {
		return out, nil
	}

	suite.plugin.Execute(suite.mockContext,
		configuration,
		suite.mockCancelFlag,
		suite.mockIohandler,
		suite.mockDataChannel)

	suite.mockCancelFlag.AssertExpectations(suite.T())
	suite.mockIohandler.AssertExpectations(suite.T())
	suite.mockDataChannel.AssertExpectations(suite.T())
	suite.mockPortSession.AssertExpectations(suite.T())
}
*/

// Testing InputStreamHandler
func (suite *PortTestSuite) TestInputStreamHandler() {
	suite.plugin.session = suite.mockPortSession
	suite.mockPortSession.On("HandleStreamMessage", mock.Anything, getAgentMessage(uint32(mgsContracts.Output), payload)).Return(nil)
	suite.plugin.InputStreamMessageHandler(suite.mockLog, getAgentMessage(uint32(mgsContracts.Output), payload))
	suite.mockPortSession.AssertExpectations(suite.T())
}

func (suite *PortTestSuite) TestInputStreamHandlerSessionNotReady() {
	suite.plugin.InputStreamMessageHandler(suite.mockLog, getAgentMessage(uint32(mgsContracts.Output), payload))
	suite.mockPortSession.AssertExpectations(suite.T())
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
