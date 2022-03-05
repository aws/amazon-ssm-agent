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

//Package messagebus logic to send message and get reply over IPC
package messagebus

import (
	"testing"

	"github.com/aws/amazon-ssm-agent/agent/appconfig"
	contextmocks "github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/aws/amazon-ssm-agent/agent/jsonutil"
	"github.com/aws/amazon-ssm-agent/agent/log"
	channel "github.com/aws/amazon-ssm-agent/common/channel"
	channelmocks "github.com/aws/amazon-ssm-agent/common/channel/mocks"
	"github.com/aws/amazon-ssm-agent/common/message"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
)

type MessageBusTestSuite struct {
	suite.Suite
	mockLog              log.T
	mockHealthChannel    *channelmocks.IChannel
	mockTerminateChannel *channelmocks.IChannel
	mockContext          *contextmocks.Mock
	messageBus           *MessageBus
	appConfig            appconfig.SsmagentConfig
}

func (suite *MessageBusTestSuite) SetupTest() {
	mockLog := log.NewMockLog()
	suite.mockLog = mockLog
	suite.appConfig = appconfig.DefaultConfig()
	suite.mockContext = contextmocks.NewMockDefault()

	suite.mockContext.On("AppConfig").Return(&suite.appConfig)
	suite.mockContext.On("Log").Return(mockLog)

	suite.mockHealthChannel = &channelmocks.IChannel{}
	suite.mockTerminateChannel = &channelmocks.IChannel{}
	channels := make(map[message.TopicType]channel.IChannel)
	channels[message.GetWorkerHealthRequest] = suite.mockHealthChannel
	channels[message.TerminateWorkerRequest] = suite.mockTerminateChannel

	suite.messageBus = &MessageBus{
		context:            suite.mockContext,
		healthChannel:      suite.mockHealthChannel,
		terminationChannel: suite.mockTerminateChannel,
		rebootRequest:      make(chan bool, 1),
	}
}

//Execute the test suite
func TestMessageBusTestSuite(t *testing.T) {
	suite.Run(t, new(MessageBusTestSuite))
}

func (suite *MessageBusTestSuite) TestProcessTerminationRequest_Successful() {
	suite.mockTerminateChannel.On("IsConnect").Return(false).Once()
	suite.mockTerminateChannel.On("IsConnect").Return(true).Once()
	suite.mockTerminateChannel.On("Initialize", mock.Anything).Return(nil)
	suite.mockTerminateChannel.On("Dial", mock.Anything).Return(nil)
	suite.mockTerminateChannel.On("Close").Return(nil)

	request := message.CreateTerminateWorkerRequest()
	requestString, _ := jsonutil.Marshal(request)
	suite.mockTerminateChannel.On("Recv").Return([]byte(requestString), nil)
	suite.mockTerminateChannel.On("Send", mock.Anything).Return(nil)

	suite.messageBus.ProcessTerminationRequest()

	suite.mockTerminateChannel.AssertExpectations(suite.T())
}
