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
// permissions and limitations under the License
//
// package testcases contains test cases from all testStages
package testcases

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/aws/amazon-ssm-agent/agent/log"
	channel "github.com/aws/amazon-ssm-agent/common/channel"
	channelmocks "github.com/aws/amazon-ssm-agent/common/channel/mocks"
	"github.com/aws/amazon-ssm-agent/common/identity"
	identityMocks "github.com/aws/amazon-ssm-agent/common/identity/mocks"
	"github.com/aws/amazon-ssm-agent/common/message"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
)

type NamedPipeTestCaseTestSuite struct {
	suite.Suite
	namedPipeTestCaseObj *NamedPipeTestCase
	mockContext          *context.Mock
}

//Execute the test suite
func TestNamedPipeTestCaseTestSuite(t *testing.T) {
	suite.Run(t, new(NamedPipeTestCaseTestSuite))
}

// SetupTest initializes Setup
func (suite *NamedPipeTestCaseTestSuite) SetupTest() {
	channelCreator = func() error {
		return nil
	}

	suite.namedPipeTestCaseObj = &NamedPipeTestCase{}
	suite.mockContext = &context.Mock{}
	suite.mockContext.On("Log").Return(log.NewMockLog())
	suite.mockContext.On("With", "[TestNamedPipe]").Return(suite.mockContext)
	suite.mockContext.On("Identity").Return(identityMocks.NewDefaultMockAgentIdentity())

	suite.namedPipeTestCaseObj.Initialize(suite.mockContext)
	createChannel = func(log.T, identity.IAgentIdentity) channel.IChannel {
		return getChannelMock(nil, nil, nil, nil, nil)
	}
}

// TestListenFail tests the listenPipe failure scenario
func (suite *NamedPipeTestCaseTestSuite) TestListenFail() {
	createChannel = func(log.T, identity.IAgentIdentity) channel.IChannel {
		listenErr := errors.New("error")
		return getChannelMock(nil, nil, listenErr, nil, nil)
	}
	suite.namedPipeTestCaseObj.Initialize(suite.mockContext)
	output := suite.namedPipeTestCaseObj.ExecuteTestCase()
	assert.Contains(suite.T(), output.Err.Error(), "listening to pipe failed")
}

// TestDialFail tests the Dial failure scenario
func (suite *NamedPipeTestCaseTestSuite) TestDialFail() {
	createChannel = func(log.T, identity.IAgentIdentity) channel.IChannel {
		dialErr := errors.New("error")
		return getChannelMock(nil, dialErr, nil, nil, nil)
	}
	suite.namedPipeTestCaseObj.Initialize(suite.mockContext)
	output := suite.namedPipeTestCaseObj.ExecuteTestCase()
	assert.Contains(suite.T(), output.Err.Error(), "dialing was unsuccessful")
}

// TestInitializeFail tests the initialization fail scenario
func (suite *NamedPipeTestCaseTestSuite) TestInitializeFail() {
	createChannel = func(log.T, identity.IAgentIdentity) channel.IChannel {
		initErr := errors.New("error")
		return getChannelMock(initErr, nil, nil, nil, nil)
	}
	suite.namedPipeTestCaseObj.Initialize(suite.mockContext)
	output := suite.namedPipeTestCaseObj.ExecuteTestCase()
	assert.NotNil(suite.T(), output.Err)
}

// TestInitializeFail tests the initialization fail scenario
func (suite *NamedPipeTestCaseTestSuite) TestSendFail() {
	createChannel = func(log.T, identity.IAgentIdentity) channel.IChannel {
		sendErr := errors.New("error")
		return getChannelMock(nil, nil, nil, nil, sendErr)
	}
	suite.namedPipeTestCaseObj.Initialize(suite.mockContext)
	output := suite.namedPipeTestCaseObj.ExecuteTestCase()
	assert.NotNil(suite.T(), output.Err)
}

// TestCleanEndTestCase tests the clean up scenario
func (suite *NamedPipeTestCaseTestSuite) TestCleanEndTestCase() {
	suite.namedPipeTestCaseObj.CleanupTestCase()
	assert.NotNil(suite.T(), suite.namedPipeTestCaseObj.GetTestSetUpCleanupEventHandle())
}

// getChannelMock returns channel mock
func getChannelMock(initErr error, dialErr error, listenErr error, recvMsg []byte, sendErr error) *channelmocks.IChannel {
	dummyMsg := message.Message{
		SchemaVersion: 1,
		Topic:         "TestTopic",
		Payload:       []byte("reply"),
	}
	mockChannel := &channelmocks.IChannel{}
	mockChannel.On("Initialize", mock.Anything).Return(initErr)
	mockChannel.On("Dial", mock.Anything).Return(dialErr)
	mockChannel.On("Listen", mock.Anything).Return(listenErr)
	if recvMsg != nil {
		msg, _ := json.Marshal(dummyMsg)
		mockChannel.On("Recv").Return(msg, nil)
	} else {
		mockChannel.On("Recv").Return(recvMsg, nil)
	}
	mockChannel.On("Send", mock.Anything).Return(sendErr)
	mockChannel.On("Close").Return(nil)
	mockChannel.On("SetOption", mock.Anything, mock.Anything).Return(nil)
	return mockChannel
}
