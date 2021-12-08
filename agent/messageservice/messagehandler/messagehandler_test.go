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
// either express or implied. See the License for the specific language governing
// permissions and limitations under the License.

// Package messageservice implements the core module to start MDS and MGS connections
package messagehandler

import (
	"github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/aws/amazon-ssm-agent/agent/contracts"
	processorMock "github.com/aws/amazon-ssm-agent/agent/framework/processor/mock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
)

var (
	instanceId  = "i-1234"
	messageId   = "2b196342-d7d4-436e-8f09-3883a1116ac3"
	status      = contracts.ResultStatusInProgress
	errorMsg    = "plugin failed"
	s3Bucket    = "s3Bucket"
	s3UrlSuffix = "s3UrlSuffix"
	cwlGroup    = "cwlGroup"
	cwlStream   = "cwlStream"
)

type MessageHandlerTestSuite struct {
	suite.Suite
	mockContext             *context.Mock
	mockCommandProcessor    *processorMock.MockedProcessor
	mockSessionProcessor    *processorMock.MockedProcessor
	messagehandler          *MessageHandler
	mockIncomingMessageChan chan contracts.DocumentState
}

func (suite *MessageHandlerTestSuite) SetupTest() {
	mockContext := context.NewMockDefault()
	mockCommandProcessor := new(processorMock.MockedProcessor)
	mockSessionProcessor := new(processorMock.MockedProcessor)

	agentConfig := contracts.AgentConfiguration{
		InstanceID: instanceId,
	}
	mockIncomingMessageChan := make(chan contracts.DocumentState)

	suite.mockCommandProcessor = mockCommandProcessor
	suite.mockSessionProcessor = mockSessionProcessor
	suite.mockContext = mockContext
	suite.mockIncomingMessageChan = mockIncomingMessageChan
	suite.messagehandler = &MessageHandler{
		name:        Name,
		context:     mockContext,
		agentConfig: agentConfig,
	}
}

// Testing the module name
func (suite *MessageHandlerTestSuite) TestModuleName() {
	rst := suite.messagehandler.GetName()
	assert.Equal(suite.T(), rst, Name)
}

/*
// Testing the module execute
func (suite *MessageHandlerTestSuite) TestModuleExecute() {
	commandResultChan := make(chan contracts.DocumentResult)
	suite.mockCommandProcessor.On("InitialProcessing", false).Return(nil)
	suite.mockCommandProcessor.On("Start").Return(commandResultChan, nil)

	sessionResultChan := make(chan contracts.DocumentResult)
	suite.mockSessionProcessor.On("InitialProcessing", false).Return(nil)
	suite.mockSessionProcessor.On("Start").Return(sessionResultChan, nil)

	suite.messagehandler.ModuleExecute()

	pluginResults := make(map[string]*contracts.PluginResult)
	pluginResult := contracts.PluginResult{
		PluginName: "Standard_Stream",
		Status:     contracts.ResultStatusInProgress,
	}
	pluginResults["Standard_Stream"] = &pluginResult

	result := contracts.DocumentResult{
		Status:          status,
		PluginResults:   pluginResults,
		LastPlugin:      "Standard_Stream",
		MessageID:       messageId,
		AssociationID:   "",
		NPlugins:        1,
		DocumentName:    "documentName",
		DocumentVersion: "1",
	}
	commandResultChan <- result
	sessionResultChan <- result
	time.Sleep(60 * time.Millisecond)

	suite.mockCommandProcessor.AssertExpectations(suite.T())
	suite.mockSessionProcessor.AssertExpectations(suite.T())

	close(commandResultChan)
	close(sessionResultChan)
}

/*
func (suite *MessageHandlerTestSuite) TestModuleRequestStop() {
	suite.mockCommandProcessor.On("Stop", mock.Anything).Return(nil)
	suite.mockSessionProcessor.On("Stop", mock.Anything).Return(nil)

	suite.mgsInteractorService.On("Close").Return(nil)
	suite.mdsInteractorService.On("Close").Return(nil)

	suite.messageservice.ModuleRequestStop(contracts.StopTypeSoftStop)

	suite.mockCommandProcessor.AssertExpectations(suite.T())
	suite.mockSessionProcessor.AssertExpectations(suite.T())
}
*/
