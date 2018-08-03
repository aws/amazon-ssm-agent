// Copyright 2018 Amazon.com, Inc. or its affiliates. All Rights Reserved.
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

// Package session implements the core module to start web-socket connection with message gateway service.
package session

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/aws/amazon-ssm-agent/agent/contracts"
	processorMock "github.com/aws/amazon-ssm-agent/agent/framework/processor/mock"
	"github.com/aws/amazon-ssm-agent/agent/log"
	mgsConfig "github.com/aws/amazon-ssm-agent/agent/session/config"
	mgsContracts "github.com/aws/amazon-ssm-agent/agent/session/contracts"
	controlChannelMock "github.com/aws/amazon-ssm-agent/agent/session/controlchannel/mocks"
	serviceMock "github.com/aws/amazon-ssm-agent/agent/session/service/mocks"
	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
)

var (
	instanceId = "i-1234"
	messageId  = "2b196342-d7d4-436e-8f09-3883a1116ac3"
	status     = contracts.ResultStatusInProgress
)

type SessionTestSuite struct {
	suite.Suite
	mockContext        *context.Mock
	mockProcessor      *processorMock.MockedProcessor
	session            contracts.ICoreModule
	mockService        *serviceMock.Service
	mockControlChannel *controlChannelMock.IControlChannel
}

func (suite *SessionTestSuite) SetupTest() {
	mockContext := context.NewMockDefault()
	mockService := &serviceMock.Service{}
	mockProcessor := new(processorMock.MockedProcessor)
	agentConfig := contracts.AgentConfiguration{
		InstanceID: instanceId,
	}
	mockControlChannel := &controlChannelMock.IControlChannel{}

	suite.mockControlChannel = mockControlChannel
	suite.mockProcessor = mockProcessor
	suite.mockService = mockService
	suite.mockContext = mockContext
	suite.session = &Session{
		name:           mgsConfig.SessionServiceName,
		context:        mockContext,
		agentConfig:    agentConfig,
		service:        mockService,
		processor:      mockProcessor,
		controlChannel: mockControlChannel}
}

// Testing the module name
func (suite *SessionTestSuite) TestModuleName() {
	rst := suite.session.ModuleName()
	assert.Equal(suite.T(), rst, mgsConfig.SessionServiceName)
}

// Testing the module execute
func (suite *SessionTestSuite) TestModuleExecute() {
	resChan := make(chan contracts.DocumentResult)
	suite.mockProcessor.On("InitialProcessing").Return(nil)
	suite.mockProcessor.On("Start").Return(resChan, nil)
	suite.mockControlChannel.On("Initialize", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil)
	suite.mockControlChannel.On("SetWebSocket", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil)
	suite.mockControlChannel.On("Open", mock.Anything).Return(nil)
	suite.mockControlChannel.On("SendMessage", mock.Anything, mock.Anything, websocket.BinaryMessage).Return(nil)

	suite.session.ModuleExecute(suite.mockContext)

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
	resChan <- result
	time.Sleep(60 * time.Millisecond)

	suite.mockProcessor.AssertExpectations(suite.T())
	suite.mockService.AssertExpectations(suite.T())
	suite.mockControlChannel.AssertExpectations(suite.T())

	close(resChan)
}

// Testing the module request stop
func (suite *SessionTestSuite) TestModuleRequestStop() {
	suite.mockControlChannel.On("Close", mock.Anything).Return(nil)
	suite.mockProcessor.On("Stop", mock.Anything).Return(nil)

	suite.session.ModuleRequestStop(contracts.StopTypeSoftStop)

	suite.mockControlChannel.AssertExpectations(suite.T())
	suite.mockProcessor.AssertExpectations(suite.T())
}

// Testing buildAgentTaskComplete.
func (suite *SessionTestSuite) TestBuildAgentTaskComplete() {
	log := log.NewMockLog()
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
	msg, err := buildAgentTaskComplete(log, result, instanceId)
	assert.Nil(suite.T(), err)

	agentMessage := &mgsContracts.AgentMessage{}
	agentMessage.Deserialize(log, msg)
	assert.Equal(suite.T(), mgsContracts.TaskCompleteMessage, agentMessage.MessageType)

	payload := &mgsContracts.AgentTaskCompletePayload{}
	json.Unmarshal(agentMessage.Payload, payload)
	assert.Equal(suite.T(), instanceId, payload.InstanceId)
	assert.Equal(suite.T(), string(status), payload.FinalTaskStatus)
	assert.Equal(suite.T(), messageId, payload.TaskId)
}

func (suite *SessionTestSuite) TestBuildAgentTaskCompleteWhenPluginIdIsEmpty() {
	log := log.NewMockLog()
	pluginResults := make(map[string]*contracts.PluginResult)
	pluginResult := contracts.PluginResult{
		PluginName: "Standard_Stream",
		Status:     contracts.ResultStatusInProgress,
	}
	pluginResults["Standard_Stream"] = &pluginResult

	result := contracts.DocumentResult{
		Status:          status,
		PluginResults:   pluginResults,
		LastPlugin:      "",
		MessageID:       messageId,
		AssociationID:   "",
		NPlugins:        1,
		DocumentName:    "documentName",
		DocumentVersion: "1",
	}
	msg, err := buildAgentTaskComplete(log, result, instanceId)
	assert.Nil(suite.T(), err)
	assert.Nil(suite.T(), msg)
}

//Execute the test suite
func TestSessionTestSuite(t *testing.T) {
	suite.Run(t, new(SessionTestSuite))
}
