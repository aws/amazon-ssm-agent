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
	"net"
	"sync"
	"testing"
	"time"

	"github.com/aws/amazon-ssm-agent/agent/appconfig"
	"github.com/aws/amazon-ssm-agent/agent/context"
	iohandlermocks "github.com/aws/amazon-ssm-agent/agent/framework/processor/executer/iohandler/mock"
	mgsContracts "github.com/aws/amazon-ssm-agent/agent/session/contracts"
	dataChannelMock "github.com/aws/amazon-ssm-agent/agent/session/datachannel/mocks"
	portSessionMock "github.com/aws/amazon-ssm-agent/agent/session/plugins/port/mocks"
	"github.com/aws/amazon-ssm-agent/agent/task"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
	"github.com/xtaci/smux"
)

type MuxPortTestSuite struct {
	suite.Suite
	mockContext     *context.Mock
	mockCancelFlag  *task.MockCancelFlag
	mockDataChannel *dataChannelMock.IDataChannel
	mockIohandler   *iohandlermocks.MockIOHandler
	mockPortSession *portSessionMock.IPortSession
	session         *MuxPortSession
}

func (suite *MuxPortTestSuite) SetupTest() {
	suite.mockContext = context.NewMockDefault()
	suite.mockDataChannel = &dataChannelMock.IDataChannel{}

	suite.session = &MuxPortSession{
		context:       suite.mockContext,
		clientVersion: muxKeepAliveDisabledAfterThisClientVersion,
		cancelled:     make(chan struct{})}
}

// Test HandleStreamMessage
func (suite *MuxPortTestSuite) TestHandleStreamMessage() {
	out, in := net.Pipe()
	suite.session.mgsConn = &MgsConn{nil, in}
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

func (suite *MuxPortTestSuite) TestHandleStreamMessageWriteFailed() {
	out, in := net.Pipe()
	suite.session.mgsConn = &MgsConn{nil, in}
	defer out.Close()
	// Close the write pipe
	in.Close()
	assert.Error(suite.T(), suite.session.HandleStreamMessage(getAgentMessage(uint32(mgsContracts.Output), payload)))
}

func (suite *MuxPortTestSuite) TestHandleStreamMessageWhenTerminateSessionFlagIsReceived() {
	var wg sync.WaitGroup
	out, in := net.Pipe()
	suite.session.mgsConn = &MgsConn{nil, in}
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

// Test WritePump
func (suite *MuxPortTestSuite) TestWritePumpFailsToRead() {
	suite.mockDataChannel.On("IsActive").Return(true)

	out, in := net.Pipe()
	smuxConfig := smux.DefaultConfig()
	session, _ := smux.Server(in, smuxConfig)
	defer session.Close()
	defer in.Close()
	out.Close()

	suite.session.mgsConn = &MgsConn{nil, out}
	suite.session.muxServer = &MuxServer{in, session}
	errCode := suite.session.WritePump(suite.mockDataChannel)

	assert.Equal(suite.T(), appconfig.ErrorExitCode, errCode)
}

func (suite *MuxPortTestSuite) TestWritePumpWhenDatachannelIsNotActive() {
	suite.mockDataChannel.On("IsActive").Return(false)

	out, in := net.Pipe()
	smuxConfig := smux.DefaultConfig()
	session, _ := smux.Server(in, smuxConfig)
	defer session.Close()
	defer out.Close()

	go func() {
		in.Write(payload)
		in.Close()
	}()

	suite.session.mgsConn = &MgsConn{nil, out}
	suite.session.muxServer = &MuxServer{in, session}
	go func() {
		suite.session.WritePump(suite.mockDataChannel)
	}()

	time.Sleep(10 * time.Millisecond)

	// Assert if SendStreamDataMessage function was not called
	suite.mockDataChannel.AssertExpectations(suite.T())
	suite.mockDataChannel.AssertNotCalled(suite.T(), "SendStreamDataMessage", suite.mockContext.Log(), mgsContracts.Output, payload)
}

func (suite *MuxPortTestSuite) TestWritePump() {
	suite.mockDataChannel.On("IsActive").Return(true)
	suite.mockDataChannel.On("SendStreamDataMessage", suite.mockContext.Log(), mgsContracts.Output, payload).Return(nil)

	out, in := net.Pipe()
	smuxConfig := smux.DefaultConfig()
	session, _ := smux.Server(in, smuxConfig)
	defer session.Close()
	defer out.Close()

	go func() {
		in.Write(payload)
		in.Close()
	}()

	suite.session.mgsConn = &MgsConn{nil, out}
	suite.session.muxServer = &MuxServer{in, session}
	suite.session.WritePump(suite.mockDataChannel)

	// Assert if SendStreamDataMessage function was called with same data from stdout
	suite.mockDataChannel.AssertExpectations(suite.T())
}

func (suite *MuxPortTestSuite) TestWritePumpWithSmuxKeepDisabledOnClientSide() {
	suite.mockDataChannel.On("IsActive").Return(true)
	suite.mockDataChannel.On("SendStreamDataMessage", suite.mockContext.Log(), mgsContracts.Output, payload).Return(nil)

	suite.session.clientVersion = "1.2.332.0"

	out, in := net.Pipe()
	smuxConfig := smux.DefaultConfig()
	smuxConfig.KeepAliveDisabled = true
	session, _ := smux.Server(in, smuxConfig)
	defer session.Close()
	defer out.Close()

	go func() {
		in.Write(payload)
		in.Close()
	}()

	suite.session.mgsConn = &MgsConn{nil, out}
	suite.session.muxServer = &MuxServer{in, session}
	suite.session.WritePump(suite.mockDataChannel)

	// Assert if SendStreamDataMessage function was called with same data from stdout
	suite.mockDataChannel.AssertExpectations(suite.T())
}

// Test handleDataTransfer
func (suite *MuxPortTestSuite) TestHandleDataTransferSrcToDst() {
	msg := make([]byte, 1024)
	out, in := net.Pipe()
	out1, in1 := net.Pipe()

	defer out1.Close()
	go func() {
		in.Write(payload)
		in.Close()
	}()
	go func() {
		n, _ := out1.Read(msg)
		msg = msg[:n]
	}()

	handleDataTransfer(in1, out)
	time.Sleep(time.Millisecond)
	assert.EqualValues(suite.T(), payload, msg)
}

func (suite *MuxPortTestSuite) TestHandleDataTransferDstToSrc() {
	msg := make([]byte, 1024)
	out, in := net.Pipe()
	out1, in1 := net.Pipe()

	defer out.Close()
	go func() {
		in1.Write(payload)
		in1.Close()
	}()
	go func() {
		n, _ := out.Read(msg)
		msg = msg[:n]
	}()

	handleDataTransfer(in, out1)
	time.Sleep(time.Millisecond)
	assert.EqualValues(suite.T(), payload, msg)
}

// Execute the test suite
func TestMuxPortSessionSuite(t *testing.T) {
	suite.Run(t, new(MuxPortTestSuite))
}
