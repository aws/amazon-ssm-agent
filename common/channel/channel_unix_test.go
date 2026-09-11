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
//go:build freebsd || linux || netbsd || openbsd
// +build freebsd linux netbsd openbsd

// Package channel captures IPC implementation.
package channel

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/aws/amazon-ssm-agent/agent/appconfig"
	"github.com/aws/amazon-ssm-agent/agent/log"
	logmocks "github.com/aws/amazon-ssm-agent/agent/mocks/log"
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
	suite.log = logmocks.NewMockLog()
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

func (suite *IChannelTestSuite) TestIPCSelection_OSCheck() {
	// Use a mutex to protect access to getOS
	var mu sync.Mutex
	originalGetOS := getOS
	defer func() {
		mu.Lock()
		getOS = originalGetOS
		mu.Unlock()
	}()

	mockLog := suite.log.(*logmocks.Mock)
	mockLog.On("Infof", mock.Anything, mock.Anything).Return()

	testCases := []struct {
		goos string
	}{
		{"windows"},
		{"darwin"},
	}

	for _, tc := range testCases {
		tc := tc // capture range variable
		suite.Run(fmt.Sprintf("OS_%s", tc.goos), func() {
			mu.Lock()
			getOS = func() string {
				return tc.goos
			}
			mu.Unlock()

			output := canUseNamedPipe(suite.log, suite.appconfig, suite.identity)
			assert.False(suite.T(), output)
			mockLog := suite.log.(*logmocks.Mock)
			mockLog.AssertCalled(suite.T(), "Infof", mock.Anything, mock.Anything)
		})
	}
}

func (suite *IChannelTestSuite) TestIPCSelection_CloseError() {
	channelMock := &mocks.IChannel{}
	expectedError := fmt.Errorf("failed to close pipe")

	channelMock.On("Initialize", mock.Anything).Return(nil)
	channelMock.On("Listen", mock.Anything).Return(fmt.Errorf("listen error"))
	channelMock.On("Close").Return(expectedError)

	newNamedPipeChannelRef = func(_ log.T, _ identity.IAgentIdentity) IChannel {
		return channelMock
	}
	isDefaultChannelPresentRef = func(_ identity.IAgentIdentity) bool {
		return false
	}

	mockLog := suite.log.(*logmocks.Mock)
	// canUseNamedPipe returns as soon as its goroutine reports the fallback, which happens before the
	// deferred Close and therefore before the Errorf under test. Waiting on Close is not enough
	// either -- the Errorf runs after Close returns -- so wait for the log call itself.
	//
	// The signal has to hang off the expectation NewMockLog already registered: testify matches the
	// first expectation whose arguments fit, and that catch-all Errorf always fits, so a more
	// specific one added here would never be selected.
	closeErrorLogged := make(chan struct{})
	var once sync.Once
	for _, expectedCall := range mockLog.ExpectedCalls {
		if expectedCall.Method != "Errorf" {
			continue
		}
		expectedCall.Run(func(args mock.Arguments) {
			if format, ok := args.Get(0).(string); ok && format == closeErrorLogFormat {
				once.Do(func() { close(closeErrorLogged) })
			}
		})
	}

	suite.appconfig.Agent.ForceFileIPC = false
	output := canUseNamedPipe(suite.log, suite.appconfig, suite.identity)

	select {
	case <-closeErrorLogged:
	case <-time.After(5 * time.Second):
		suite.FailNow("timed out waiting for the named pipe close failure to be logged")
	}

	assert.False(suite.T(), output)
	mockLog.AssertCalled(suite.T(), "Errorf", closeErrorLogFormat, []interface{}{expectedError})
	mockLog.AssertCalled(suite.T(), "Info", []interface{}{"falling back to file based IPC as named pipe creation failed"})
}
