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

// Package noninteractivecommands implements session shell sessionPlugin with non-interactive command execution.
package noninteractivecommands

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

type NonInteractiveCommandsTestSuite struct {
	suite.Suite
	mockContext     *context.Mock
	mockLog         log.T
	mockCancelFlag  *task.MockCancelFlag
	mockDataChannel *dataChannelMock.IDataChannel
	mockIohandler   *iohandlermocks.MockIOHandler
	plugin          *NonInteractiveCommandsPlugin
}

func (suite *NonInteractiveCommandsTestSuite) SetupTest() {
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
	suite.plugin = &NonInteractiveCommandsPlugin{}
}

//Execute the test suite
func TestInteractiveCommandsTestSuite(t *testing.T) {
	suite.Run(t, new(NonInteractiveCommandsTestSuite))
}

// Testing Name
func (suite *NonInteractiveCommandsTestSuite) TestName() {
	rst := suite.plugin.name()
	assert.Equal(suite.T(), rst, appconfig.PluginNameNonInteractiveCommands)
}

// Testing GetPluginParameters
func (suite *NonInteractiveCommandsTestSuite) TestGetPluginParameters() {
	mockSessionPlugin := new(sessionPluginMock.ISessionPlugin)
	mockSessionPlugin.On("GetPluginParameters", mock.Anything).Return(nil)
	suite.plugin.sessionPlugin = mockSessionPlugin

	assert.Equal(suite.T(), suite.plugin.GetPluginParameters(map[string]interface{}{"key": "value"}), nil)
}

// Testing RequireHandshake
func (suite *NonInteractiveCommandsTestSuite) TestRequireHandshake() {
	assert.Equal(suite.T(), suite.plugin.RequireHandshake(), true)
}

// Testing Execute
func (suite *NonInteractiveCommandsTestSuite) TestExecute() {
	mockSessionPlugin := new(sessionPluginMock.ISessionPlugin)
	mockSessionPlugin.On("Execute", mock.Anything, suite.mockCancelFlag, suite.mockIohandler, suite.mockDataChannel).Return()
	suite.plugin.sessionPlugin = mockSessionPlugin

	suite.plugin.Execute(contracts.Configuration{}, suite.mockCancelFlag, suite.mockIohandler, suite.mockDataChannel)

	mockSessionPlugin.AssertExpectations(suite.T())
}

// Testing InputStreamMessageHandler base case.
func (suite *NonInteractiveCommandsTestSuite) TestInputStreamMessageHandler() {
	mockSessionPlugin := new(sessionPluginMock.ISessionPlugin)
	mockSessionPlugin.On("InputStreamMessageHandler", suite.mockLog, mock.Anything).Return(nil)
	suite.plugin.sessionPlugin = mockSessionPlugin

	err := suite.plugin.InputStreamMessageHandler(suite.mockLog, mgsContracts.AgentMessage{})

	mockSessionPlugin.AssertExpectations(suite.T())
	assert.Nil(suite.T(), err)
}
