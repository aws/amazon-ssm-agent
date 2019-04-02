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
	"github.com/aws/amazon-ssm-agent/agent/task"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
	"github.com/twinj/uuid"
)

var (
	mockLog       = log.NewMockLog()
	configuration = contracts.Configuration{Properties: map[string]interface{}{"portNumber": "22"}}
	payload       = []byte("testPayload")
	messageId     = "dd01e56b-ff48-483e-a508-b5f073f31b16"
	schemaVersion = uint32(1)
	createdDate   = uint64(1503434274948)
)

type PortTestSuite struct {
	suite.Suite
	mockContext     *context.Mock
	mockLog         log.T
	mockCancelFlag  *task.MockCancelFlag
	mockDataChannel *dataChannelMock.IDataChannel
	mockIohandler   *iohandlermocks.MockIOHandler
	plugin          *PortPlugin
}

func (suite *PortTestSuite) SetupTest() {
	mockContext := context.NewMockDefault()
	mockCancelFlag := &task.MockCancelFlag{}
	mockDataChannel := &dataChannelMock.IDataChannel{}
	mockIohandler := new(iohandlermocks.MockIOHandler)

	suite.mockContext = mockContext
	suite.mockCancelFlag = mockCancelFlag
	suite.mockLog = mockLog
	suite.mockDataChannel = mockDataChannel
	suite.mockIohandler = mockIohandler
	suite.plugin = &PortPlugin{
		dataChannel: mockDataChannel,
	}
}

func (suite *PortTestSuite) TearDownTest() {
	if suite.plugin.tcpConn != nil {
		suite.plugin.tcpConn.Close()
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

// Testing Execute
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
		contracts.Configuration{Properties: map[string]interface{}{"portNumber": ""}},
		suite.mockCancelFlag,
		suite.mockIohandler,
		suite.mockDataChannel)

	suite.mockCancelFlag.AssertExpectations(suite.T())
	suite.mockIohandler.AssertExpectations(suite.T())
}

func (suite *PortTestSuite) TestExecuteUnableToStartTCP() {
	suite.mockCancelFlag.On("Canceled").Return(false)
	suite.mockCancelFlag.On("ShutDown").Return(false)
	suite.mockIohandler.On("SetStatus", contracts.ResultStatusFailed).Return(nil)
	suite.mockIohandler.On("SetExitCode", 1).Return(nil)
	suite.mockIohandler.On("SetOutput", mock.Anything).Return()

	DialCall = func(network string, address string) (net.Conn, error) {
		return nil, errors.New("unable to connect")
	}

	suite.plugin.Execute(suite.mockContext,
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
	suite.mockDataChannel.On("SendStreamDataMessage", mock.Anything, mgsContracts.Output, payload).Return(nil)

	out, in := net.Pipe()
	DialCall = func(network string, address string) (net.Conn, error) {
		return out, nil
	}

	// Write and close the pipe
	go func() {
		in.Write(payload)
		in.Close()
	}()

	suite.plugin.Execute(suite.mockContext,
		configuration,
		suite.mockCancelFlag,
		suite.mockIohandler,
		suite.mockDataChannel)

	suite.mockCancelFlag.AssertExpectations(suite.T())
	suite.mockIohandler.AssertExpectations(suite.T())
	suite.mockDataChannel.AssertExpectations(suite.T())
}

// Testing writepump separately
func (suite *PortTestSuite) TestWritePump() {
	suite.mockDataChannel.On("SendStreamDataMessage", suite.mockLog, mgsContracts.Output, payload).Return(nil)

	out, in := net.Pipe()
	defer out.Close()

	go func() {
		in.Write(payload)
		in.Close()
	}()

	suite.plugin.tcpConn = out
	suite.plugin.writePump(suite.mockLog)

	// Assert if SendStreamDataMessage function was called with same data from stdout
	suite.mockDataChannel.AssertExpectations(suite.T())
}

// Testing InputStreamHandler
func (suite *PortTestSuite) TestInputStreamHandler() {
	out, in := net.Pipe()
	suite.plugin.tcpConn = in
	defer in.Close()
	defer out.Close()

	output := make([]byte, 100)
	go func() {
		time.Sleep(10 * time.Millisecond)
		n, _ := out.Read(output)
		assert.Equal(suite.T(), payload, output[:n])
	}()

	suite.plugin.InputStreamMessageHandler(suite.mockLog, getAgentMessage(uint32(mgsContracts.Output), payload))
}

func (suite *PortTestSuite) TestInputStreamHandlerWriteFailed() {
	out, in := net.Pipe()
	suite.plugin.tcpConn = in
	defer out.Close()
	// Close the write pipe
	in.Close()
	assert.Error(suite.T(),
		suite.plugin.InputStreamMessageHandler(suite.mockLog, getAgentMessage(uint32(mgsContracts.Output), payload)))
}

func (suite *PortTestSuite) TestInputStreamHandlerWithNilTCPConn() {
	assert.NoError(suite.T(),
		suite.plugin.InputStreamMessageHandler(suite.mockLog, getAgentMessage(uint32(mgsContracts.Output), payload)))
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
