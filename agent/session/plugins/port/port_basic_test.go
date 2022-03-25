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

// Package port implements session manager's port plugin.
package port

import (
	"bytes"
	"encoding/binary"
	"errors"
	"io"
	"net"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/aws/amazon-ssm-agent/agent/appconfig"
	"github.com/aws/amazon-ssm-agent/agent/context"
	iohandlermocks "github.com/aws/amazon-ssm-agent/agent/framework/processor/executer/iohandler/mock"
	mgsConfig "github.com/aws/amazon-ssm-agent/agent/session/config"
	mgsContracts "github.com/aws/amazon-ssm-agent/agent/session/contracts"
	dataChannelMock "github.com/aws/amazon-ssm-agent/agent/session/datachannel/mocks"
	portSessionMock "github.com/aws/amazon-ssm-agent/agent/session/plugins/port/mocks"
	"github.com/aws/amazon-ssm-agent/agent/task"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
)

type BasicPortTestSuite struct {
	suite.Suite
	mockContext     *context.Mock
	mockCancelFlag  *task.MockCancelFlag
	mockDataChannel *dataChannelMock.IDataChannel
	mockIohandler   *iohandlermocks.MockIOHandler
	mockPortSession *portSessionMock.IPortSession
	session         *BasicPortSession
}

func (suite *BasicPortTestSuite) SetupTest() {
	suite.mockContext = context.NewMockDefault()
	suite.mockDataChannel = &dataChannelMock.IDataChannel{}
	suite.session = &BasicPortSession{
		context:            suite.mockContext,
		reconnectToPortErr: make(chan error),
		cancelled:          make(chan struct{}),
	}
}

// Test HandleStreamMessage
func (suite *BasicPortTestSuite) TestHandleStreamMessage() {
	out, in := net.Pipe()
	suite.session.conn = in
	defer in.Close()
	defer out.Close()

	output := make([]byte, 100)
	go func() {
		time.Sleep(10 * time.Millisecond)
		n, _ := out.Read(output)
		assert.Equal(suite.T(), payload, output[:n])
	}()

	suite.session.HandleStreamMessage(getAgentMessage(uint32(mgsContracts.Output), payload))
}

func (suite *BasicPortTestSuite) TestHandleStreamMessageWriteFailed() {
	out, in := net.Pipe()
	suite.session.conn = in
	defer out.Close()
	// Close the write pipe
	in.Close()
	assert.Error(suite.T(), suite.session.HandleStreamMessage(getAgentMessage(uint32(mgsContracts.Output), payload)))
}

func (suite *BasicPortTestSuite) TestHandleStreamMessageWhenTerminateSessionFlagIsReceived() {
	var wg sync.WaitGroup
	out, in := net.Pipe()
	suite.session.conn = in
	in.Close()
	out.Close()
	flagBuf := new(bytes.Buffer)
	binary.Write(flagBuf, binary.BigEndian, mgsContracts.TerminateSession)

	wg.Add(1)
	go func() {
		defer wg.Done()
		cancelled := <-suite.session.cancelled
		assert.Equal(suite.T(), struct{}{}, cancelled)
	}()

	suite.session.HandleStreamMessage(getAgentMessage(uint32(mgsContracts.Flag), flagBuf.Bytes()))
	wg.Wait()
}

func (suite *BasicPortTestSuite) TestHandleStreamMessageWithReconnectToPortSetToTrue() {
	prevConnOut, prevConnIn := net.Pipe()
	suite.session.conn = prevConnIn
	prevConnIn.Close()
	prevConnOut.Close()

	out, in := net.Pipe()
	defer in.Close()
	defer out.Close()
	DialCall = func(network string, address string) (net.Conn, error) {
		return out, nil
	}

	suite.session.reconnectToPort = false

	output := make([]byte, 100)
	go func() {
		<-suite.session.reconnectToPortErr

		time.Sleep(10 * time.Millisecond)
		n, _ := out.Read(output)
		assert.Equal(suite.T(), payload, output[:n])
	}()

	suite.session.HandleStreamMessage(getAgentMessage(uint32(mgsContracts.Output), payload))
	assert.Equal(suite.T(), false, suite.session.reconnectToPort)
}

// Testing handleTCPReadError
func (suite *BasicPortTestSuite) TestHandleTCPReadNonEOFError() {
	returnCode := suite.session.handleTCPReadError(errors.New("some error!!!"))
	assert.Equal(suite.T(), appconfig.ErrorExitCode, returnCode)
}

func (suite *BasicPortTestSuite) TestHandleTCPReadErrorWhenEOFError() {
	returnCode := suite.session.handleTCPReadError(io.EOF)
	assert.Equal(suite.T(), appconfig.SuccessExitCode, returnCode)
}

func (suite *BasicPortTestSuite) TestHandleTCPReadErrorWhenReconnectionToPortIsSuccessForLocalPortForwarding() {
	out, in := net.Pipe()
	defer in.Close()
	defer out.Close()

	suite.session.portType = mgsConfig.LocalPortForwarding
	suite.session.conn = out
	suite.session.reconnectToPort = false

	go func() {
		time.Sleep(10 * time.Millisecond)
		suite.session.reconnectToPortErr <- nil
	}()

	returnCode := suite.session.handleTCPReadError(errors.New("some error!!"))
	assert.Equal(suite.T(), true, suite.session.reconnectToPort)
	assert.Equal(suite.T(), mgsConfig.ResumeReadExitCode, returnCode)
}

func (suite *BasicPortTestSuite) TestHandleTCPReadErrorWhenReconnectionToPortFailedForLocalPortForwarding() {
	out, in := net.Pipe()
	defer in.Close()
	defer out.Close()

	suite.session.portType = mgsConfig.LocalPortForwarding
	suite.session.conn = out
	suite.session.reconnectToPort = false

	go func() {
		time.Sleep(10 * time.Millisecond)
		suite.session.reconnectToPortErr <- errors.New("failed to start tcp connection!!")
	}()

	returnCode := suite.session.handleTCPReadError(errors.New("some error!!"))
	assert.Equal(suite.T(), true, suite.session.reconnectToPort)
	assert.Equal(suite.T(), appconfig.ErrorExitCode, returnCode)
}

// Testing writepump
func (suite *BasicPortTestSuite) TestWritePump() {
	suite.mockDataChannel.On("IsActive").Return(true)
	suite.mockDataChannel.On("SendStreamDataMessage", suite.mockContext.Log(), mgsContracts.Output, payload).Return(nil)

	out, in := net.Pipe()
	defer out.Close()

	go func() {
		in.Write(payload)
		in.Close()
	}()

	suite.session.conn = out
	suite.session.WritePump(suite.mockDataChannel)

	// Assert if SendStreamDataMessage function was called with same data from stdout
	suite.mockDataChannel.AssertExpectations(suite.T())
}

func (suite *BasicPortTestSuite) TestWritePumpWhenDatachannelIsNotActive() {
	suite.mockDataChannel.On("IsActive").Return(false)

	out, in := net.Pipe()
	defer out.Close()

	go func() {
		in.Write(payload)
		in.Close()
	}()

	suite.session.conn = out
	go func() {
		suite.session.WritePump(suite.mockDataChannel)
	}()

	time.Sleep(10 * time.Millisecond)

	// Assert if SendStreamDataMessage function was not called
	suite.mockDataChannel.AssertExpectations(suite.T())
	suite.mockDataChannel.AssertNotCalled(suite.T(), "SendStreamDataMessage", suite.mockContext.Log(), mgsContracts.Output, payload)
}

func (suite *BasicPortTestSuite) TestInitializeWithReachableEndpoint() {
	addr, _ := suite.SpawnMockServer()
	suite.session.destinationAddress = net.JoinHostPort(addr.IP.String(), strconv.Itoa(addr.Port))

	DialCall = func(network string, address string) (net.Conn, error) {
		return net.Dial(network, address)
	}

	assert.Nil(suite.T(), suite.session.InitializeSession())
}

func (suite *BasicPortTestSuite) TestInitializeWithUnreachableEndpoint() {
	addr, listener := suite.SpawnMockServer()
	listener.Close()

	suite.session.destinationAddress = net.JoinHostPort(addr.IP.String(), strconv.Itoa(addr.Port))

	DialCall = func(network string, address string) (net.Conn, error) {
		return net.Dial(network, address)
	}

	assert.Error(suite.T(), suite.session.InitializeSession())
}

func (suite *BasicPortTestSuite) SpawnMockServer() (addr *net.TCPAddr, listener net.Listener) {
	listener, _ = net.Listen("tcp", "127.0.0.1:0")
	addr = listener.Addr().(*net.TCPAddr)
	go func() {
		if conn, _ := listener.Accept(); conn != nil {
			conn.Write(payload)
			conn.Close()
		}
	}()
	time.Sleep(200 * time.Millisecond)
	return
}

// Execute the test suite
func TestBasicPortTestSuite(t *testing.T) {
	suite.Run(t, new(BasicPortTestSuite))
}
