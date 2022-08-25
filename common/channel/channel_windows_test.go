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
//go:build windows
// +build windows

// Package channel captures IPC implementation.
package channel

import (
	"testing"

	"github.com/aws/amazon-ssm-agent/agent/appconfig"
	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/amazon-ssm-agent/common/channel/mocks"
	"github.com/aws/amazon-ssm-agent/common/identity"
	identityMocks "github.com/aws/amazon-ssm-agent/common/identity/mocks"
	"github.com/stretchr/testify/assert"
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

func (suite *IChannelTestSuite) TestIPCSelection() {
	newNamedPipeChannelRef = func(log log.T, identity identity.IAgentIdentity) IChannel {
		return &mocks.IChannel{}
	}
	output := canUseNamedPipe(suite.log, suite.appconfig, suite.identity)
	assert.False(suite.T(), output)
}
