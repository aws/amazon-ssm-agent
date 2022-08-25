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

// Package longrunningprovider provides an interface to start/stop a worker process for long-running tasks.
package longrunningprovider

import (
	"fmt"
	"testing"
	"time"

	reboot "github.com/aws/amazon-ssm-agent/core/app/reboot/model"

	"github.com/aws/amazon-ssm-agent/agent/appconfig"
	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/amazon-ssm-agent/common/message"
	contextmocks "github.com/aws/amazon-ssm-agent/core/app/context/mocks"
	messagebusmocks "github.com/aws/amazon-ssm-agent/core/ipc/messagebus/mocks"
	discovermocks "github.com/aws/amazon-ssm-agent/core/workerprovider/longrunningprovider/discover/mocks"
	"github.com/aws/amazon-ssm-agent/core/workerprovider/longrunningprovider/model"
	providermocks "github.com/aws/amazon-ssm-agent/core/workerprovider/longrunningprovider/provider/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
)

type LongRunningProviderTestSuite struct {
	suite.Suite
	configs        map[string]*model.WorkerConfig
	pingResults    []*message.Message
	mockLog        log.T
	container      *WorkerContainer
	workerProvider *providermocks.IProvider
	workerDiscover *discovermocks.IDiscover
	messageBus     *messagebusmocks.IMessageBus
	context        *contextmocks.ICoreAgentContext
	appConfig      appconfig.SsmagentConfig
}

func (suite *LongRunningProviderTestSuite) SetupTest() {
	mockLog := log.NewMockLog()
	suite.mockLog = mockLog
	suite.workerProvider = &providermocks.IProvider{}
	suite.workerDiscover = &discovermocks.IDiscover{}
	suite.messageBus = &messagebusmocks.IMessageBus{}
	suite.context = &contextmocks.ICoreAgentContext{}
	suite.appConfig = appconfig.DefaultConfig()
	suite.appConfig.Agent.LongRunningWorkerMonitorIntervalSeconds = 1

	suite.container = &WorkerContainer{
		stopWorkerMonitor: make(chan bool, 1),
		workerProvider:    suite.workerProvider,
		workerDiscover:    suite.workerDiscover,
		messageBus:        suite.messageBus,
		context:           suite.context,
	}

	suite.context.On("With", mock.Anything).Return(suite.context)
	suite.context.On("Log").Return(mockLog)
	suite.context.On("AppConfig").Return(&suite.appConfig)

	suite.configs = createStandardSSMAgentWorkers()

	result, _ := message.CreateHealthResult(model.SSMAgentWorkerName, message.LongRunning, 1)
	suite.pingResults = []*message.Message{result}

	sleep = func(duration time.Duration) {}
}

// Execute the test suite
func TestLongRunningProviderTestSuite(t *testing.T) {
	suite.Run(t, new(LongRunningProviderTestSuite))
}

func (suite *LongRunningProviderTestSuite) TestStartWorkers_Successful() {
	suite.workerDiscover.On("FindWorkerConfigs").Return(suite.configs)
	suite.messageBus.On("SendSurveyMessage", mock.Anything).Return(suite.pingResults, nil)
	suite.workerProvider.On("Start", suite.configs, suite.pingResults).Return(nil)

	suite.container.Start()

	suite.workerDiscover.AssertExpectations(suite.T())
	suite.messageBus.AssertExpectations(suite.T())
	suite.workerProvider.AssertExpectations(suite.T())
}

func (suite *LongRunningProviderTestSuite) TestWatchWorkers_RestartWorker() {

	suite.workerDiscover.On("FindWorkerConfigs").Return(suite.configs)
	suite.messageBus.On("SendSurveyMessage", mock.Anything).Return(suite.pingResults, nil)
	suite.workerProvider.On("Monitor", suite.configs, suite.pingResults).Return(nil)

	go suite.container.Monitor()

	time.Sleep(2 * time.Second)
	suite.workerDiscover.AssertExpectations(suite.T())
	suite.messageBus.AssertExpectations(suite.T())
	suite.workerProvider.AssertExpectations(suite.T())
}

func (suite *LongRunningProviderTestSuite) TestStartWorkerWithTimer_StopByTimer() {

	suite.container.stopWorkerMonitor <- true
	suite.container.Monitor()

	suite.workerProvider.AssertExpectations(suite.T())
}

func (suite *LongRunningProviderTestSuite) TestStopWorker_FailureSendSurvey() {
	getPpid = func() int {
		return 1
	}
	suite.messageBus.On("SendSurveyMessage", mock.Anything).Return(nil, fmt.Errorf("SomeError")).Once()
	suite.messageBus.On("Stop").Return().Once()

	suite.container.Stop(reboot.StopTypeHardStop)

	suite.messageBus.AssertExpectations(suite.T())
	assert.True(suite.T(), <-suite.container.stopWorkerMonitor)
}

func (suite *LongRunningProviderTestSuite) TestStopWorker_SuccessSendSurvey() {
	response := []*message.Message{
		{
			SchemaVersion: 1,
			Topic:         message.GetWorkerHealthResult,
			Payload:       []byte("SomePayload"),
		},
	}
	getPpid = func() int {
		return 1
	}
	suite.messageBus.On("SendSurveyMessage", mock.Anything).Return(response, nil).Once()
	suite.messageBus.On("Stop").Return().Once()
	suite.workerProvider.AssertNotCalled(suite.T(), "KillAllWorkerProcesses")

	suite.container.Stop(reboot.StopTypeHardStop)

	suite.messageBus.AssertExpectations(suite.T())
	suite.workerProvider.AssertExpectations(suite.T())
	assert.True(suite.T(), <-suite.container.stopWorkerMonitor)
}

func (suite *LongRunningProviderTestSuite) TestStopWorker_ParentProcessZero() {
	getPpid = func() int {
		return 0
	}

	suite.messageBus.On("SendSurveyMessage", mock.Anything).Return([]*message.Message{}, nil).Once()
	suite.messageBus.On("Stop").Return().Once()
	suite.workerProvider.On("KillAllWorkerProcesses").Return().Once()

	suite.container.Stop(reboot.StopTypeHardStop)

	suite.messageBus.AssertExpectations(suite.T())
	suite.workerProvider.AssertExpectations(suite.T())

	assert.True(suite.T(), <-suite.container.stopWorkerMonitor)

}

func createStandardSSMAgentWorkers() map[string]*model.WorkerConfig {
	worker := model.WorkerConfig{
		Name: model.SSMAgentWorkerName,
		Path: appconfig.DefaultSSMAgentWorker,
	}

	configs := make(map[string]*model.WorkerConfig)
	configs[worker.Name] = &worker

	return configs
}
