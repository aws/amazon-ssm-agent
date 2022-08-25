// Copyright 2021 Amazon.com, Inc. or its affiliates. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License"). You may not
// use this file except in compliance with the License. A copy of the
// License is located at
//
// http://aws.amazon.com/apache2.0/
//
// or in the "license" file accompanying this file. This file is distributed
// on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND,
// either express or implied. See the License for the specific language governing`
// permissions and limitations under the License.

// Package messageservice will be responsible for initializing MDS and MGS interactors and then
// launch message handlers to handle the commands received from interactors.
// This package is the starting point for the message service module.
package messageservice

import (
	"fmt"
	"testing"

	"github.com/aws/amazon-ssm-agent/agent/appconfig"
	"github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/amazon-ssm-agent/agent/messageservice/interactor"
	interactorMock "github.com/aws/amazon-ssm-agent/agent/messageservice/interactor/mocks"
	"github.com/aws/amazon-ssm-agent/agent/messageservice/messagehandler"
	"github.com/aws/amazon-ssm-agent/agent/messageservice/messagehandler/mocks"
	"github.com/aws/amazon-ssm-agent/agent/messageservice/messagehandler/processorwrappers"
	processorWrapperMock "github.com/aws/amazon-ssm-agent/agent/messageservice/messagehandler/processorwrappers/mocks"
	"github.com/aws/amazon-ssm-agent/agent/messageservice/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
)

type MessageServiceTestSuite struct {
	suite.Suite
}

// Execute the test suite
func TestMessageServiceTestSuite(t *testing.T) {
	suite.Run(t, new(MessageServiceTestSuite))
}

// TestMessageService_Initialize tests initialize scenarios
func (suite *MessageServiceTestSuite) TestMessageService_Initialize() {
	// Nano platform check
	ctx := context.NewMockDefault()
	isPlatformNanoServer = func(log log.T) (bool, error) {
		return true, nil
	}
	msgService := NewService(ctx).(*MessageService)
	assert.Equal(suite.T(), 1, len(msgService.interactors), "length not expected for nano")

	// container mode check
	isPlatformNanoServer = func(log log.T) (bool, error) {
		return false, nil
	}
	ssmAgentConfig := appconfig.SsmagentConfig{
		Agent: appconfig.AgentInfo{
			ContainerMode: true,
		},
	}
	ctx = context.NewMockDefaultWithConfig(ssmAgentConfig)
	msgService = NewService(ctx).(*MessageService)
	assert.Equal(suite.T(), 1, len(msgService.interactors), "length not expected for container")

	// for other platforms
	isPlatformNanoServer = func(log log.T) (bool, error) {
		return false, nil
	}
	ssmAgentConfig = appconfig.SsmagentConfig{
		Agent: appconfig.AgentInfo{
			ContainerMode: false,
		},
	}
	ctx = context.NewMockDefaultWithConfig(ssmAgentConfig)
	msgService = NewService(ctx).(*MessageService)
	assert.Equal(suite.T(), 2, len(msgService.interactors), "length not expected for other platforms")

	// other remaining case
	isPlatformNanoServer = func(log log.T) (bool, error) {
		return true, nil
	}
	ssmAgentConfig = appconfig.SsmagentConfig{
		Agent: appconfig.AgentInfo{
			ContainerMode: true,
		},
	}
	ctx = context.NewMockDefaultWithConfig(ssmAgentConfig)
	msgService = NewService(ctx).(*MessageService)
	assert.Equal(suite.T(), 0, len(msgService.interactors), "length not expected for container")
}

// TestMessageService_ModuleName tests module name
func (suite *MessageServiceTestSuite) TestMessageService_ModuleName() {
	isPlatformNanoServer = func(log log.T) (bool, error) {
		return false, nil
	}
	ssmAgentConfig := appconfig.SsmagentConfig{
		Agent: appconfig.AgentInfo{
			ContainerMode: false,
		},
	}
	ctx := context.NewMockDefaultWithConfig(ssmAgentConfig)
	msgService := NewService(ctx)
	assert.Equal(suite.T(), msgService.ModuleName(), ServiceName, "invalid module name")
}

// TestMessageService_ModuleExecute tests module execute
func (suite *MessageServiceTestSuite) TestMessageService_ModuleExecute() {
	isPlatformNanoServer = func(log log.T) (bool, error) {
		return false, nil
	}
	ssmAgentConfig := appconfig.SsmagentConfig{
		Agent: appconfig.AgentInfo{
			ContainerMode: false,
		},
	}
	ctx := context.NewMockDefaultWithConfig(ssmAgentConfig)
	msgService := NewService(ctx).(*MessageService)
	assert.Equal(suite.T(), 2, len(msgService.interactors), "length not expected for other platforms")

	// clearing interactor array
	msgService.interactors = make([]interactor.IInteractor, 0)
	interactorMockObj := &interactorMock.IInteractor{}
	interactorMockObj.On("Initialize").Return(nil)
	interactorMockObj.On("PostProcessorInitialization", mock.Anything).Return(nil)
	interactorMockObj.On("GetName").Return(mock.Anything)
	interactorMockObj.On("GetSupportedWorkers").Return([]utils.WorkerName{utils.DocumentWorkerName, utils.SessionWorkerName})
	msgService.interactors = append(msgService.interactors, interactorMockObj, interactorMockObj)
	messageHandlerMock := &mocks.IMessageHandler{}
	msgService.messageHandler = messageHandlerMock
	messageHandlerMock.On("Initialize").Return(nil)
	messageHandlerMock.On("Submit", mock.Anything).Return(messagehandler.ErrorCode(""))
	messageHandlerMock.On("RegisterProcessor", mock.Anything).Return(nil)

	processorWrapperMock1 := &processorWrapperMock.IProcessorWrapper{}
	processorWrapperMock1.On("GetName").Return(mock.Anything)
	processorWrapperMock1.On("Initialize", mock.Anything).Return(nil)

	processorWrapperMock2 := &processorWrapperMock.IProcessorWrapper{}
	processorWrapperMock2.On("GetName").Return(mock.Anything)
	processorWrapperMock2.On("Initialize", mock.Anything).Return(nil)

	// Missing Processor Wrapper case
	getProcessorWrapperDelegateMap = func() map[utils.WorkerName]func(context.T, *utils.ProcessorWorkerConfig) processorwrappers.IProcessorWrapper {
		return map[utils.WorkerName]func(context.T, *utils.ProcessorWorkerConfig) processorwrappers.IProcessorWrapper{}
	}
	msgService.ModuleExecute()
	messageHandlerMock.AssertNumberOfCalls(suite.T(), "Initialize", 1)
	interactorMockObj.AssertNumberOfCalls(suite.T(), "Initialize", 2)

	// Processor Wrapper available case
	getProcessorWrapperDelegateMap = func() map[utils.WorkerName]func(context.T, *utils.ProcessorWorkerConfig) processorwrappers.IProcessorWrapper {
		delegateMap := make(map[utils.WorkerName]func(context.T, *utils.ProcessorWorkerConfig) processorwrappers.IProcessorWrapper)
		delegateMap[utils.DocumentWorkerName] = func(context.T, *utils.ProcessorWorkerConfig) processorwrappers.IProcessorWrapper {
			return processorWrapperMock1
		}
		delegateMap[utils.SessionWorkerName] = func(context.T, *utils.ProcessorWorkerConfig) processorwrappers.IProcessorWrapper {
			return processorWrapperMock2
		}
		return delegateMap
	}
	msgService.ModuleExecute()
	messageHandlerMock.AssertNumberOfCalls(suite.T(), "Initialize", 2) // +1 from previous execution
	interactorMockObj.AssertNumberOfCalls(suite.T(), "Initialize", 4)
}

// TestMessageService_ModuleExecute tests module execute
func (suite *MessageServiceTestSuite) TestMessageService_ModuleStop() {
	isPlatformNanoServer = func(log log.T) (bool, error) {
		return false, nil
	}
	ssmAgentConfig := appconfig.SsmagentConfig{
		Agent: appconfig.AgentInfo{
			ContainerMode: false,
		},
	}
	ctx := context.NewMockDefaultWithConfig(ssmAgentConfig)
	msgService := NewService(ctx).(*MessageService)
	assert.Equal(suite.T(), 2, len(msgService.interactors), "length not expected for other platforms")

	// clearing interactor array
	msgService.interactors = make([]interactor.IInteractor, 0)
	interactorMockObj := &interactorMock.IInteractor{}
	interactorMockObj.On("PreProcessorClose")
	interactorMockObj.On("Close").Return(nil)
	interactorMockObj.On("GetName").Return("")
	msgService.interactors = append(msgService.interactors, interactorMockObj, interactorMockObj)
	messageHandlerMock := &mocks.IMessageHandler{}
	messageHandlerMock.On("Stop", mock.Anything).Return(nil)
	msgService.messageHandler = messageHandlerMock
	err := msgService.ModuleStop()
	assert.Nil(suite.T(), err, "stop should not return error")

	// Close error - expected stop error
	msgService.interactors = make([]interactor.IInteractor, 0)
	interactorMockObj = &interactorMock.IInteractor{}
	interactorMockObj.On("PreProcessorClose").Return(nil)
	interactorMockObj.On("Close").Return(fmt.Errorf("random error"))
	interactorMockObj.On("GetName").Return(mock.Anything)
	msgService.interactors = append(msgService.interactors, interactorMockObj, interactorMockObj)
	messageHandlerMock = &mocks.IMessageHandler{}
	messageHandlerMock.On("Stop", mock.Anything).Return(nil)
	msgService.messageHandler = messageHandlerMock
	err = msgService.ModuleStop()
	interactorMockObj.AssertNumberOfCalls(suite.T(), "Close", 2)
}
