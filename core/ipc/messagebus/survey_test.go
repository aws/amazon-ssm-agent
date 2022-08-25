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

// Package messagebus logic to send message and get reply over IPC
package messagebus

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/amazon-ssm-agent/common/channel"
	channelmocks "github.com/aws/amazon-ssm-agent/common/channel/mocks"
	"github.com/aws/amazon-ssm-agent/common/message"
	contextmocks "github.com/aws/amazon-ssm-agent/core/app/context/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
)

var (
	pid        = 1000
	workerType = message.LongRunning
	workerName = "worker-name"
)

type MessageBusTestSuite struct {
	suite.Suite
	mockLog              log.T
	mockHealthChannel    *channelmocks.IChannel
	mockTerminateChannel *channelmocks.IChannel
	mockContext          *contextmocks.ICoreAgentContext
	messageBus           *MessageBus
}

func (suite *MessageBusTestSuite) SetupTest() {
	mockLog := log.NewMockLog()
	suite.mockLog = mockLog
	suite.mockContext = &contextmocks.ICoreAgentContext{}

	suite.mockContext.On("With", mock.Anything).Return(suite.mockContext)
	suite.mockContext.On("Log").Return(mockLog)

	suite.mockHealthChannel = &channelmocks.IChannel{}
	suite.mockTerminateChannel = &channelmocks.IChannel{}
	channels := make(map[message.TopicType]channel.IChannel)
	channels[message.GetWorkerHealthRequest] = suite.mockHealthChannel
	channels[message.TerminateWorkerRequest] = suite.mockTerminateChannel

	suite.messageBus = &MessageBus{
		context:        suite.mockContext,
		surveyChannels: channels,
	}
}

// Execute the test suite
func TestMessageBusTestSuite(t *testing.T) {
	suite.Run(t, new(MessageBusTestSuite))
}

func (suite *MessageBusTestSuite) TestStart_Successful() {
	suite.mockHealthChannel.On("Initialize", mock.Anything).Return(nil)
	suite.mockHealthChannel.On("Listen", mock.Anything).Return(nil)
	suite.mockHealthChannel.On("SetOption", mock.Anything, mock.Anything).Return(nil)
	suite.mockTerminateChannel.On("Initialize", mock.Anything).Return(nil)
	suite.mockTerminateChannel.On("Listen", mock.Anything).Return(nil)
	suite.mockTerminateChannel.On("SetOption", mock.Anything, mock.Anything).Return(nil)

	err := suite.messageBus.Start()

	assert.Nil(suite.T(), err)
	suite.mockHealthChannel.AssertExpectations(suite.T())
	suite.mockTerminateChannel.AssertExpectations(suite.T())
}

func (suite *MessageBusTestSuite) TestStart_Fail() {
	suite.mockHealthChannel.On("Initialize", mock.Anything).Return(errors.New("failed"))

	err := suite.messageBus.Start()

	assert.NotNil(suite.T(), err)
	suite.mockHealthChannel.AssertExpectations(suite.T())
}

func (suite *MessageBusTestSuite) TestStop_Successful() {
	suite.mockHealthChannel.On("Close").Return(nil)
	suite.mockTerminateChannel.On("Close").Return(nil)

	suite.messageBus.Stop()

	suite.mockHealthChannel.AssertExpectations(suite.T())
	suite.mockTerminateChannel.AssertExpectations(suite.T())
}

func (suite *MessageBusTestSuite) TestSendSurveyMessage_Successful() {
	healthResult, _ := message.CreateHealthResult(
		workerName,
		workerType,
		pid)

	resultString, _ := json.Marshal(healthResult)

	suite.mockHealthChannel.On("IsConnect").Return(true)
	suite.mockHealthChannel.On("Send", mock.Anything).Return(nil)
	suite.mockHealthChannel.On("Recv").Return(resultString, nil).Once()
	suite.mockHealthChannel.On("Recv").Return(nil, errors.New("stop")).Once()

	surveyMsg := &message.Message{
		SchemaVersion: 1,
		Topic:         message.GetWorkerHealthRequest,
	}

	results, err := suite.messageBus.SendSurveyMessage(surveyMsg)

	assert.Nil(suite.T(), err)
	assert.True(suite.T(), len(results) == 1)
	suite.mockHealthChannel.AssertExpectations(suite.T())
	for _, result := range results {
		var payload message.HealthResultPayload
		json.Unmarshal(result.Payload, &payload)
		assert.Equal(suite.T(), payload.SchemaVersion, 1)
		assert.Equal(suite.T(), payload.Name, workerName)
		assert.Equal(suite.T(), payload.WorkerType, workerType)
		assert.Equal(suite.T(), payload.Pid, pid)
	}
}

func (suite *MessageBusTestSuite) TestSendSurveyMessage_Fail() {
	resultString := "can not deserialize"

	suite.mockHealthChannel.On("IsConnect").Return(true)
	suite.mockHealthChannel.On("Send", mock.Anything).Return(nil)
	suite.mockHealthChannel.On("Recv").Return([]byte(resultString), nil).Once()
	suite.mockHealthChannel.On("Recv").Return(nil, errors.New("stop")).Once()

	surveyMsg := &message.Message{
		SchemaVersion: 1,
		Topic:         message.GetWorkerHealthRequest,
	}

	results, err := suite.messageBus.SendSurveyMessage(surveyMsg)

	assert.Nil(suite.T(), err)
	assert.True(suite.T(), len(results) == 0)
	suite.mockHealthChannel.AssertExpectations(suite.T())
}
