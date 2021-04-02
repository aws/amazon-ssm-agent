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
	"time"

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

// ISurveySuite tests surveyor
type ISurveySuite struct {
	suite.Suite
	log            log.T
	identity       identity.IAgentIdentity
	surveyInstance *survey
}

// TestTesterSuite executes test suite
func TestSurveySuite(t *testing.T) {
	suite.Run(t, new(ISurveySuite))
}

// SetupTest initializes Setup
func (suite *ISurveySuite) SetupTest() {
	suite.log = log.NewMockLog()
	suite.identity = identityMocks.NewDefaultMockAgentIdentity()
	suite.surveyInstance = GetSurveyInstance(suite.log, suite.identity)
}

// TestBasicTest tests basic functionality
func (suite *ISurveySuite) TestBasic() {
	suite.surveyInstance.Initialize()

	assert.Equal(suite.T(), suite.surveyInstance.GetCommProtocolInfo(), utils.Surveyor)

	suite.surveyInstance.fileChannel = channelmock.NewFakeChannel(log.NewMockLog(), filewatcherbasedipc.ModeSurveyor, "sample")
	dummyMsg := message.Message{
		SchemaVersion: 1,
		Topic:         "TestBasic",
		Payload:       []byte("reply"),
	}
	dummyMsgOutput := dummyMsg
	suite.surveyInstance.Send(&dummyMsg)
	suite.surveyInstance.fileChannel = channelmock.NewFakeChannel(log.NewMockLog(), filewatcherbasedipc.ModeRespondent, "sample")
	output, err := suite.surveyInstance.Recv()
	_ = json.Unmarshal(output, &dummyMsg)
	assert.Equal(suite.T(), dummyMsg.Topic, dummyMsgOutput.Topic)
	assert.Nil(suite.T(), err)
}

// TestTimeout tests timeout functionality
func (suite *ISurveySuite) TestTimeout() {
	dummyMsg := message.Message{
		SchemaVersion: 1,
		Topic:         "TestTimeout",
		Payload:       []byte("reply"),
	}
	suite.surveyInstance.Send(&dummyMsg)
	time.Sleep(3 * time.Second)
	_, err := suite.surveyInstance.Recv()
	assert.NotNil(suite.T(), err)
}

// TestPreListenDial tests pre listen and pre dial scenarios
func (suite *ISurveySuite) TestPreListenDial() {
	dummyMsg := message.Message{
		SchemaVersion: 1,
		Topic:         "TestPreListenDial",
		Payload:       []byte("reply"),
	}
	suite.surveyInstance.fileChannel = nil
	err := suite.surveyInstance.Send(&dummyMsg)
	assert.NotNil(suite.T(), err)
	_, err = suite.surveyInstance.Recv()
	assert.NotNil(suite.T(), err)
}
