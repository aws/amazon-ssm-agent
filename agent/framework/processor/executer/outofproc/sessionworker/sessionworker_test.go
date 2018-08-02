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

// Package main implements a separate worker which is used to execute requests from session manager.
package main

import (
	"testing"

	"github.com/aws/amazon-ssm-agent/agent/appconfig"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
)

var (
	testProgramPath      = appconfig.DefaultSessionWorker
	testDataChannelId    = "test-dataChannelId"
	testDataChannelToken = "test-dataChannelToken"
	testPluginName       = "test-pluginName"
	testChannelName      = "test-channelName"
	testClientId         = "test-clientId"
)

type SessionWorkerTestSuite struct {
	suite.Suite
}

// Testing worker invoked by master.
func (suite *SessionWorkerTestSuite) TestSessionWorkerInitialize() {
	ctxLight, channelName, err := initialize([]string{testProgramPath, testChannelName})
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), testChannelName, channelName)
	assert.Equal(suite.T(),
		ctxLight.CurrentContext(),
		[]string{defaultSessionWorkerContextName, "[" + channelName + "]"})
}

//Execute the test suite
func TestSessionTestSuite(t *testing.T) {
	suite.Run(t, new(SessionWorkerTestSuite))
}
