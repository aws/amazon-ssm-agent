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

// Package protocol implements some common communication protocols using file watcher.
package protocol

import (
	"encoding/json"
	"testing"

	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/amazon-ssm-agent/common/channel/utils"
	"github.com/aws/amazon-ssm-agent/common/filewatcherbasedipc"
	channelmock "github.com/aws/amazon-ssm-agent/common/filewatcherbasedipc/mocks"
	"github.com/aws/amazon-ssm-agent/common/identity"
	identityMocks "github.com/aws/amazon-ssm-agent/common/identity/mocks"
	"github.com/aws/amazon-ssm-agent/common/message"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
)

// IRespondentSuite tests respondent
type IRespondentSuite struct {
	suite.Suite
	log                log.T
	identity           identity.IAgentIdentity
	respondentInstance *respondent
}

// TestRespondentSuite executes test suite
func TestRespondentSuite(t *testing.T) {
	suite.Run(t, new(IRespondentSuite))
}

// SetupTest initializes Setup
func (suite *IRespondentSuite) SetupTest() {
	suite.log = log.NewMockLog()
	suite.identity = identityMocks.NewDefaultMockAgentIdentity()
	suite.respondentInstance = GetRespondentInstance(suite.log, suite.identity)
}

// TestBasicTest tests basic functionality
func (suite *IRespondentSuite) TestBasic() {
	suite.respondentInstance.Initialize()
	assert.Equal(suite.T(), suite.respondentInstance.GetCommProtocolInfo(), utils.Respondent)
	suite.respondentInstance.fileChannel = channelmock.NewFakeChannel(suite.log, filewatcherbasedipc.ModeSurveyor, "sample")
	dummyMsg := message.Message{
		SchemaVersion: 1,
		Topic:         "TestBasic",
		Payload:       []byte("reply"),
	}
	dummyMsgOutput := dummyMsg
	suite.respondentInstance.Send(&dummyMsg)
	suite.respondentInstance.fileChannel = channelmock.NewFakeChannel(suite.log, filewatcherbasedipc.ModeRespondent, "sample")
	output, err := suite.respondentInstance.Recv()
	_ = json.Unmarshal(output, &dummyMsg)
	assert.Equal(suite.T(), dummyMsgOutput.Topic, dummyMsg.Topic)
	assert.Nil(suite.T(), err)
}

// TestPreListenDial tests pre listen and pre dial scenarios
func (suite *IRespondentSuite) TestPreListenDial() {
	dummyMsg := message.Message{
		SchemaVersion: 1,
		Topic:         "TestPreListenDial",
		Payload:       []byte("reply"),
	}
	suite.respondentInstance.fileChannel = nil
	err := suite.respondentInstance.Send(&dummyMsg)
	assert.NotNil(suite.T(), err)
	_, err = suite.respondentInstance.Recv()
	assert.NotNil(suite.T(), err)
}
