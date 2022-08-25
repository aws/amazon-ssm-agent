// Copyright 2019 Amazon.com, Inc. or its affiliates. All Rights Reserved.
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

// Package interactivecommands implements session shell plugin with interactive commands.
package interactivecommands

import (
	"testing"

	"github.com/aws/amazon-ssm-agent/agent/appconfig"
	"github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/aws/amazon-ssm-agent/agent/contracts"
	iohandlermocks "github.com/aws/amazon-ssm-agent/agent/framework/processor/executer/iohandler/mock"
	"github.com/aws/amazon-ssm-agent/agent/log"
	mgsContracts "github.com/aws/amazon-ssm-agent/agent/session/contracts"
	dataChannelMock "github.com/aws/amazon-ssm-agent/agent/session/datachannel/mocks"
	sessionPluginMock "github.com/aws/amazon-ssm-agent/agent/session/plugins/sessionplugin/mocks"
	"github.com/aws/amazon-ssm-agent/agent/task"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
)

type InteractiveCommandsTestSuite struct {
	suite.Suite
	mockContext     *context.Mock
	mockLog         log.T
	mockCancelFlag  *task.MockCancelFlag
	mockDataChannel *dataChannelMock.IDataChannel
	mockIohandler   *iohandlermocks.MockIOHandler
	plugin          *InteractiveCommandsPlugin
}

func (suite *InteractiveCommandsTestSuite) SetupTest() {
	mockContext := context.NewMockDefault()
	mockCancelFlag := &task.MockCancelFlag{}
	mockDataChannel := &dataChannelMock.IDataChannel{}
	mockIohandler := new(iohandlermocks.MockIOHandler)
	mockLog := log.NewMockLog()

	suite.mockContext = mockContext
	suite.mockLog = mockLog
	suite.mockCancelFlag = mockCancelFlag
	suite.mockDataChannel = mockDataChannel
	suite.mockIohandler = mockIohandler
	suite.plugin = &InteractiveCommandsPlugin{}
}

// Execute the test suite
func TestInteractiveCommandsTestSuite(t *testing.T) {
	suite.Run(t, new(InteractiveCommandsTestSuite))
}

// Testing Name
func (suite *InteractiveCommandsTestSuite) TestName() {
	rst := suite.plugin.name()
	assert.Equal(suite.T(), rst, appconfig.PluginNameInteractiveCommands)
}

// Testing GetPluginParameters
func (suite *InteractiveCommandsTestSuite) TestGetPluginParameters() {
	mockSessionPlugin := new(sessionPluginMock.ISessionPlugin)
	mockSessionPlugin.On("GetPluginParameters", mock.Anything).Return(nil)
	suite.plugin.sessionPlugin = mockSessionPlugin

	assert.Equal(suite.T(), suite.plugin.GetPluginParameters(map[string]interface{}{"key": "value"}), nil)
}

// Testing RequireHandshake
func (suite *InteractiveCommandsTestSuite) TestRequireHandshake() {
	mockSessionPlugin := new(sessionPluginMock.ISessionPlugin)
	mockSessionPlugin.On("RequireHandshake").Return(false)
	suite.plugin.sessionPlugin = mockSessionPlugin

	assert.Equal(suite.T(), suite.plugin.RequireHandshake(), false)
}

// Testing Execute
func (suite *InteractiveCommandsTestSuite) TestExecute() {
	mockSessionPlugin := new(sessionPluginMock.ISessionPlugin)
	mockSessionPlugin.On("Execute", mock.Anything, suite.mockCancelFlag, suite.mockIohandler, suite.mockDataChannel).Return()
	suite.plugin.sessionPlugin = mockSessionPlugin

	suite.plugin.Execute(contracts.Configuration{}, suite.mockCancelFlag, suite.mockIohandler, suite.mockDataChannel)

	mockSessionPlugin.AssertExpectations(suite.T())
}

// Testing InputStreamMessageHandler base case.
func (suite *InteractiveCommandsTestSuite) TestInputStreamMessageHandler() {
	mockSessionPlugin := new(sessionPluginMock.ISessionPlugin)
	mockSessionPlugin.On("InputStreamMessageHandler", suite.mockLog, mock.Anything).Return(nil)
	suite.plugin.sessionPlugin = mockSessionPlugin

	err := suite.plugin.InputStreamMessageHandler(suite.mockLog, mgsContracts.AgentMessage{})

	mockSessionPlugin.AssertExpectations(suite.T())
	assert.Nil(suite.T(), err)
}
