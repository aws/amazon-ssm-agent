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
//
//go:build darwin || freebsd || linux || netbsd || openbsd
// +build darwin freebsd linux netbsd openbsd

// Package channel captures IPC implementation.
package channel

import (
	"fmt"
	"testing"
	"time"

	"github.com/aws/amazon-ssm-agent/agent/appconfig"
	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/amazon-ssm-agent/common/channel/mocks"
	"github.com/aws/amazon-ssm-agent/common/identity"
	identityMocks "github.com/aws/amazon-ssm-agent/common/identity/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
)

type IChannelTestSuite struct {
	suite.Suite
	log       log.T
	identity  identity.IAgentIdentity
	appconfig appconfig.SsmagentConfig
}

// TestChannelSuite executes test suite
func TestChannelSuite(t *testing.T) {
	suite.Run(t, new(IChannelTestSuite))
}

// SetupTest initializes Setup
func (suite *IChannelTestSuite) SetupTest() {
	suite.log = log.NewMockLog()
	suite.identity = identityMocks.NewDefaultMockAgentIdentity()
	suite.appconfig = appconfig.DefaultConfig()
}

func (suite *IChannelTestSuite) TestIPCSelection_ForceFileIPC() {
	newNamedPipeChannelRef = func(log log.T, identity identity.IAgentIdentity) IChannel {
		return &mocks.IChannel{}
	}
	suite.appconfig.Agent.ForceFileIPC = true
	output := canUseNamedPipe(suite.log, suite.appconfig, suite.identity)
	assert.False(suite.T(), output)
}

func (suite *IChannelTestSuite) TestIPCSelection_NamedPipe_Success() {
	channelMock := &mocks.IChannel{}
	channelMock.On("Initialize", mock.Anything).Return(nil)
	channelMock.On("Listen", mock.Anything).Return(nil)
	channelMock.On("Close").Return(nil)

	newNamedPipeChannelRef = func(log log.T, identity identity.IAgentIdentity) IChannel {
		return channelMock
	}
	isDefaultChannelPresentRef = func(identity identity.IAgentIdentity) bool {
		return false
	}
	suite.appconfig.Agent.ForceFileIPC = false
	output := canUseNamedPipe(suite.log, suite.appconfig, suite.identity)
	assert.True(suite.T(), output)
}

func (suite *IChannelTestSuite) TestIPCSelection_NamedPipe_Fail() {
	channelMock := &mocks.IChannel{}
	channelMock.On("Initialize", mock.Anything).Return(nil)
	channelMock.On("Listen", mock.Anything).Return(fmt.Errorf("error"))
	channelMock.On("Close").Return(nil)

	newNamedPipeChannelRef = func(log log.T, identity identity.IAgentIdentity) IChannel {
		return channelMock
	}
	isDefaultChannelPresentRef = func(identity identity.IAgentIdentity) bool {
		return false
	}
	suite.appconfig.Agent.ForceFileIPC = false
	output := canUseNamedPipe(suite.log, suite.appconfig, suite.identity)
	assert.False(suite.T(), output)
}

func (suite *IChannelTestSuite) TestIPCSelection_NamedPipe_Panic() {
	newNamedPipeChannelRef = func(log log.T, identity identity.IAgentIdentity) IChannel {
		return &mocks.IChannel{}
	}
	isDefaultChannelPresentRef = func(identity identity.IAgentIdentity) bool {
		return false
	}
	suite.appconfig.Agent.ForceFileIPC = false
	startTime := time.Now()
	output := canUseNamedPipe(suite.log, suite.appconfig, suite.identity)
	endTime := time.Now()
	assert.False(suite.T(), output)
	assert.Condition(suite.T(), func() (success bool) {
		return endTime.Sub(startTime.Add(10*time.Second)) >= 0
	})
}
