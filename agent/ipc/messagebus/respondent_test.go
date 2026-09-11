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
	"fmt"
	"testing"
	"time"

	"github.com/aws/amazon-ssm-agent/agent/appconfig"
	"github.com/aws/amazon-ssm-agent/agent/jsonutil"
	"github.com/aws/amazon-ssm-agent/agent/log"
	contextmocks "github.com/aws/amazon-ssm-agent/agent/mocks/context"
	logmocks "github.com/aws/amazon-ssm-agent/agent/mocks/log"
	"github.com/aws/amazon-ssm-agent/common/channel"
	channelmocks "github.com/aws/amazon-ssm-agent/common/channel/mocks"
	"github.com/aws/amazon-ssm-agent/common/message"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
)

type MessageBusTestSuite struct {
	suite.Suite
	mockLog                log.T
	mockHealthChannel      *channelmocks.IChannel
	mockTerminateChannel   *channelmocks.IChannel
	mockPreShutdownChannel *channelmocks.IChannel
	mockContext            *contextmocks.Mock
	messageBus             *MessageBus
	appConfig              appconfig.SsmagentConfig
}

func (suite *MessageBusTestSuite) SetupTest() {
	mockLog := logmocks.NewMockLog()
	suite.mockLog = mockLog
	suite.appConfig = appconfig.DefaultConfig()
	suite.mockContext = contextmocks.NewMockDefault()

	suite.mockContext.On("AppConfig").Return(&suite.appConfig)
	suite.mockContext.On("Log").Return(mockLog)

	suite.mockHealthChannel = &channelmocks.IChannel{}
	suite.mockTerminateChannel = &channelmocks.IChannel{}
	suite.mockPreShutdownChannel = &channelmocks.IChannel{}
	channels := make(map[message.TopicType]channel.IChannel)
	channels[message.GetWorkerHealthRequest] = suite.mockHealthChannel
	channels[message.TerminateWorkerRequest] = suite.mockTerminateChannel

	suite.messageBus = &MessageBus{
		context:                     suite.mockContext,
		healthChannel:               suite.mockHealthChannel,
		terminationChannel:          suite.mockTerminateChannel,
		preShutdownChannel:          suite.mockPreShutdownChannel,
		terminationRequestChannel:   make(chan bool, 1),
		terminationChannelConnected: make(chan bool, 1),
		sleepFunc:                   func(time.Duration) {},
	}
}

func (suite *MessageBusTestSuite) TestProcessHealthRequest_Successful() {
	// Arrange
	suite.mockHealthChannel.On("IsChannelInitialized").Return(true).Once()
	suite.mockHealthChannel.On("IsDialSuccessful").Return(true).Once()
	suite.mockHealthChannel.On("Close").Return(nil).Once()
	request := message.CreateHealthRequest()
	requestString, _ := jsonutil.Marshal(request)
	suite.mockHealthChannel.On("Recv").Return([]byte(requestString), nil).Once()
	suite.mockHealthChannel.On("Send", mock.Anything).Return(nil)
	// Kills the infinite loop
	suite.mockHealthChannel.On("Recv").Return(nil, fmt.Errorf("failed to receive message on channel")).Times(maxRecvErrCount)

	// Act
	suite.messageBus.ProcessHealthRequest()

	// Assert
	suite.mockHealthChannel.AssertExpectations(suite.T())
}

func (suite *MessageBusTestSuite) TestProcessHealthRequest_RecvError() {
	// Arrange
	suite.mockHealthChannel.On("IsChannelInitialized").Return(true).Once()
	suite.mockHealthChannel.On("IsDialSuccessful").Return(true).Once()
	suite.mockHealthChannel.On("Close").Return(nil).Once()
	suite.mockHealthChannel.On("Recv").Return(nil, fmt.Errorf("failed to receive message on channel")).Times(maxRecvErrCount)

	// Act
	suite.messageBus.ProcessHealthRequest()

	// Assert
	suite.mockHealthChannel.AssertExpectations(suite.T())
}

func (suite *MessageBusTestSuite) TestProcessHealthRequest_RecvErrorCount_Resets() {
	// Arrange
	suite.mockHealthChannel.On("IsChannelInitialized").Return(true).Once()
	suite.mockHealthChannel.On("IsDialSuccessful").Return(true).Once()
	suite.mockHealthChannel.On("Close").Return(nil).Once()
	suite.mockHealthChannel.On("Recv").Return(nil, fmt.Errorf("failed to receive message on channel")).Times(maxRecvErrCount - 1)
	request := message.CreateHealthRequest()
	requestString, _ := jsonutil.Marshal(request)
	suite.mockHealthChannel.On("Recv").Return([]byte(requestString), nil).Once()
	suite.mockHealthChannel.On("Send", mock.Anything).Return(nil)
	// Kills the infinite loop
	suite.mockHealthChannel.On("Recv").Return(nil, fmt.Errorf("failed to receive message on channel")).Times(maxRecvErrCount)

	// Act
	suite.messageBus.ProcessHealthRequest()

	// Assert
	suite.mockHealthChannel.AssertExpectations(suite.T())
}

func (suite *MessageBusTestSuite) TestProcessTerminationRequest_Error() {
	suite.mockTerminateChannel.On("IsDialSuccessful").Return(true).Once()
	suite.mockTerminateChannel.On("IsChannelInitialized").Return(true).Once()
	suite.mockTerminateChannel.On("Close").Return(nil).Once()
	suite.mockTerminateChannel.On("Recv").Return(nil, fmt.Errorf("failed to receive message on channel")).Times(maxRecvErrCount)

	suite.messageBus.ProcessTerminationRequest()

	suite.mockTerminateChannel.AssertExpectations(suite.T())

	// Assert termination channel connected and that a termination message is sent
	suite.Assertions.Equal(true, <-suite.messageBus.GetTerminationChannelConnectedChan())
	suite.Assertions.Equal(true, <-suite.messageBus.GetTerminationRequestChan())
}

func (suite *MessageBusTestSuite) TestProcessTerminationRequest_RecvRetried() {
	suite.mockTerminateChannel.On("IsDialSuccessful").Return(true).Once()
	suite.mockTerminateChannel.On("IsChannelInitialized").Return(true).Once()
	suite.mockTerminateChannel.On("Close").Return(nil).Once()
	suite.mockTerminateChannel.On("Recv").Return(nil, fmt.Errorf("failed to receive message on channel")).Times(maxRecvErrCount - 1)
	request := message.CreateTerminateWorkerRequest()
	requestString, _ := jsonutil.Marshal(request)
	suite.mockTerminateChannel.On("Recv").Return([]byte(requestString), nil)
	suite.mockTerminateChannel.On("Send", mock.Anything).Return(nil)
	suite.messageBus.ProcessTerminationRequest()
	suite.mockTerminateChannel.AssertExpectations(suite.T())

	// Assert termination channel connected and that a termination message is sent
	suite.Assertions.Equal(true, <-suite.messageBus.GetTerminationChannelConnectedChan())
	suite.Assertions.Equal(true, <-suite.messageBus.GetTerminationRequestChan())
}

func (suite *MessageBusTestSuite) TestProcessTerminationRequest_RecvRetriesReset() {
	suite.mockTerminateChannel.On("IsDialSuccessful").Return(true).Once()
	suite.mockTerminateChannel.On("IsChannelInitialized").Return(true).Once()
	suite.mockTerminateChannel.On("Close").Return(nil).Once()
	suite.mockTerminateChannel.On("Recv").Return(nil, fmt.Errorf("failed to receive message on channel")).Times(maxRecvErrCount - 1)
	suite.mockTerminateChannel.On("Recv").Return([]byte("not valid json message"), nil).Once()
	request := message.CreateTerminateWorkerRequest()
	requestString, _ := jsonutil.Marshal(request)
	suite.mockTerminateChannel.On("Recv").Return([]byte(requestString), nil).Once()
	suite.mockTerminateChannel.On("Send", mock.Anything).Return(nil)
	suite.messageBus.ProcessTerminationRequest()
	suite.mockTerminateChannel.AssertExpectations(suite.T())

	// Assert termination channel connected and that a termination message is sent
	suite.Assertions.Equal(true, <-suite.messageBus.GetTerminationChannelConnectedChan())
	suite.Assertions.Equal(true, <-suite.messageBus.GetTerminationRequestChan())
}

// Execute the test suite
func TestMessageBusTestSuite(t *testing.T) {
	suite.Run(t, new(MessageBusTestSuite))
}

func (suite *MessageBusTestSuite) TestProcessTerminationRequest_Successful() {
	suite.mockTerminateChannel.On("IsChannelInitialized").Return(true).Once()
	suite.mockTerminateChannel.On("IsDialSuccessful").Return(true).Once()
	suite.mockTerminateChannel.On("Close").Return(nil).Once()

	request := message.CreateTerminateWorkerRequest()
	requestString, _ := jsonutil.Marshal(request)
	suite.mockTerminateChannel.On("Recv").Return([]byte(requestString), nil)
	suite.mockTerminateChannel.On("Send", mock.Anything).Return(nil)

	suite.messageBus.ProcessTerminationRequest()

	suite.mockTerminateChannel.AssertExpectations(suite.T())

	// Assert termination channel connected and that a termination message is sent
	suite.Assertions.Equal(true, <-suite.messageBus.GetTerminationChannelConnectedChan())
	suite.Assertions.Equal(true, <-suite.messageBus.GetTerminationRequestChan())
}

func (suite *MessageBusTestSuite) TestProcessTerminationRequest_SuccessfulConnectionRetry() {
	// First try channel not connected but fails initialize
	suite.mockTerminateChannel.On("IsDialSuccessful").Return(false).Once()
	suite.mockTerminateChannel.On("Initialize", mock.Anything).Return(fmt.Errorf("SomeErr")).Once()
	suite.mockTerminateChannel.On("Close").Return(nil).Once()

	// Second try channel not connected but fails dial
	suite.mockTerminateChannel.On("IsDialSuccessful").Return(false).Once()
	suite.mockTerminateChannel.On("Initialize", mock.Anything).Return(nil)
	suite.mockTerminateChannel.On("Dial", mock.Anything).Return(fmt.Errorf("SomeDialError")).Once()
	suite.mockTerminateChannel.On("Close").Return(nil).Once()

	// Third try channel not connected but finally succeeds
	suite.mockTerminateChannel.On("IsDialSuccessful").Return(false).Once()
	suite.mockTerminateChannel.On("Initialize", mock.Anything).Return(nil)
	suite.mockTerminateChannel.On("Dial", mock.Anything).Return(nil).Once()
	suite.mockTerminateChannel.On("IsDialSuccessful").Return(true).Once()

	request := message.CreateTerminateWorkerRequest()
	requestString, _ := jsonutil.Marshal(request)
	suite.mockTerminateChannel.On("Recv").Return([]byte(requestString), nil)
	suite.mockTerminateChannel.On("Send", mock.Anything).Return(nil)

	// Fourth call to IsChannelInitialized succeeds, fourth call is for defer where it will call close
	suite.mockTerminateChannel.On("IsChannelInitialized").Return(true).Once()
	suite.mockTerminateChannel.On("Close").Return(nil).Once()

	suite.messageBus.ProcessTerminationRequest()

	// Assert termination channel connected and that a termination message is sent
	suite.Assertions.Equal(true, <-suite.messageBus.GetTerminationChannelConnectedChan())
	suite.Assertions.Equal(true, <-suite.messageBus.GetTerminationRequestChan())

	suite.mockTerminateChannel.AssertExpectations(suite.T())
}

// Prepare-shutdown is handled on its OWN channel and must NOT terminate the worker: it has to stay
// alive so it can make in-flight work reboot-safe, which is the entire point of core asking during
// the SCM preshutdown window.
func (suite *MessageBusTestSuite) TestProcessPrepareShutdownRequest_PreparesAndAcknowledges() {
	prepared := 0
	RegisterShutdownPreparer(func(log.T) {
		prepared++
	})
	defer RegisterShutdownPreparer(func(log.T) {})

	suite.mockPreShutdownChannel.On("IsDialSuccessful").Return(true).Once()
	suite.mockPreShutdownChannel.On("IsChannelInitialized").Return(true).Once()
	suite.mockPreShutdownChannel.On("Close").Return(nil).Once()

	request := message.CreatePrepareShutdownRequest()
	requestString, _ := jsonutil.Marshal(request)
	suite.mockPreShutdownChannel.On("Recv").Return([]byte(requestString), nil).Once()
	// Acknowledgement goes out on the prepare-shutdown channel, not the termination channel.
	suite.mockPreShutdownChannel.On("Send", mock.Anything).Return(nil).Once()
	// Then drive the loop to exit via receive errors -- the request itself must not end it.
	suite.mockPreShutdownChannel.On("Recv").Return(nil, fmt.Errorf("failed to receive message on channel")).Times(maxRecvErrCount)

	suite.messageBus.ProcessPrepareShutdownRequest()
	suite.mockPreShutdownChannel.AssertExpectations(suite.T())

	suite.Assertions.Equal(1, prepared, "prepare-shutdown must invoke the registered preparer")
	// Crucially: nothing was pushed to the termination request channel.
	suite.Assertions.Equal(0, len(suite.messageBus.GetTerminationRequestChan()),
		"prepare-shutdown must never terminate the worker")
}

// A worker with no preparer registered must still handle the request without panicking.
func (suite *MessageBusTestSuite) TestProcessPrepareShutdownRequest_NoPreparerRegistered() {
	suite.mockPreShutdownChannel.On("IsDialSuccessful").Return(true).Once()
	suite.mockPreShutdownChannel.On("IsChannelInitialized").Return(true).Once()
	suite.mockPreShutdownChannel.On("Close").Return(nil).Once()

	request := message.CreatePrepareShutdownRequest()
	requestString, _ := jsonutil.Marshal(request)
	suite.mockPreShutdownChannel.On("Recv").Return([]byte(requestString), nil).Once()
	suite.mockPreShutdownChannel.On("Send", mock.Anything).Return(nil).Once()
	suite.mockPreShutdownChannel.On("Recv").Return(nil, fmt.Errorf("failed to receive message on channel")).Times(maxRecvErrCount)

	suite.Assertions.NotPanics(func() { suite.messageBus.ProcessPrepareShutdownRequest() })
}

// A lost acknowledgement must not break anything: preparation already happened, so only core's
// telemetry is lost, never correctness.
func (suite *MessageBusTestSuite) TestProcessPrepareShutdownRequest_SendFailureStillPrepares() {
	prepared := 0
	RegisterShutdownPreparer(func(log.T) { prepared++ })
	defer RegisterShutdownPreparer(func(log.T) {})

	suite.mockPreShutdownChannel.On("IsDialSuccessful").Return(true).Once()
	suite.mockPreShutdownChannel.On("IsChannelInitialized").Return(true).Once()
	suite.mockPreShutdownChannel.On("Close").Return(nil).Once()

	request := message.CreatePrepareShutdownRequest()
	requestString, _ := jsonutil.Marshal(request)
	suite.mockPreShutdownChannel.On("Recv").Return([]byte(requestString), nil).Once()
	suite.mockPreShutdownChannel.On("Send", mock.Anything).Return(fmt.Errorf("send failed")).Once()
	suite.mockPreShutdownChannel.On("Recv").Return(nil, fmt.Errorf("failed to receive message on channel")).Times(maxRecvErrCount)

	suite.messageBus.ProcessPrepareShutdownRequest()

	suite.Assertions.Equal(1, prepared, "preparation must happen even when the ack cannot be sent")
}
