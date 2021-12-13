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
// either express or implied. See the License for the specific language governing`
// permissions and limitations under the License.

// Package replytypes will be responsible for handling session complete replies received from the processor
package replytypes

import (
	"encoding/json"
	"testing"

	"github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/aws/amazon-ssm-agent/agent/contracts"
	mgsUtils "github.com/aws/amazon-ssm-agent/agent/messageservice/interactor/mgsinteractor/utils"
	mgsContracts "github.com/aws/amazon-ssm-agent/agent/session/contracts"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
	"github.com/twinj/uuid"
)

type AgentCompleteReplyTestSuite struct {
	suite.Suite
}

//Execute the test suite
func TestAgentCompleteReplyTestSuite(t *testing.T) {
	suite.Run(t, new(AgentCompleteReplyTestSuite))
}

func (suite *AgentCompleteReplyTestSuite) TestAgentCompleteReply_Initialize() {
	ctx := context.NewMockDefault()
	docResult := contracts.DocumentResult{ResultType: contracts.RunCommandResult}
	uuid := uuid.NewV4()
	agentComplete := NewAgentRunCommandReplyType(ctx, docResult, uuid, 0)
	assert.Equal(suite.T(), uuid.String(), agentComplete.GetMessageUUID().String())
	assert.Equal(suite.T(), 1, agentComplete.GetBackOffSecond())
	assert.Equal(suite.T(), 1, agentComplete.GetNumberOfContinuousRetries())
	assert.Equal(suite.T(), true, agentComplete.ShouldPersistData())
	assert.Equal(suite.T(), 0, agentComplete.GetRetryNumber())
}

func (suite *AgentCompleteReplyTestSuite) TestAgentCompleteReply_InitializeRetryNumber() {
	ctx := context.NewMockDefault()
	docResult := contracts.DocumentResult{ResultType: contracts.RunCommandResult}
	uuid := uuid.NewV4()
	agentComplete := NewAgentRunCommandReplyType(ctx, docResult, uuid, 2)
	assert.Equal(suite.T(), uuid.String(), agentComplete.GetMessageUUID().String())
	assert.Equal(suite.T(), 1, agentComplete.GetBackOffSecond())
	assert.Equal(suite.T(), 1, agentComplete.GetNumberOfContinuousRetries())
	assert.Equal(suite.T(), true, agentComplete.ShouldPersistData())
	assert.Equal(suite.T(), 2, agentComplete.GetRetryNumber())
}

func (suite *AgentCompleteReplyTestSuite) TestAgentCompleteReply_ConvertMessage() {
	ctx := context.NewMockDefault()
	outputMsgId := "messageId"
	docResult := contracts.DocumentResult{MessageID: "messageId", ResultType: contracts.RunCommandResult}
	uuid := uuid.NewV4()
	agentComplete := NewAgentRunCommandReplyType(ctx, docResult, uuid, 0)
	agentMessage, err := agentComplete.ConvertToAgentMessage()
	assert.Nil(suite.T(), err)
	assert.Equal(suite.T(), uuid.String(), agentMessage.MessageId.String())
	replyContent := mgsContracts.AgentJobReplyContent{}
	err = json.Unmarshal(agentMessage.Payload, &replyContent)
	assert.Nil(suite.T(), err)
	assert.Equal(suite.T(), string(mgsUtils.SendCommandTopic), replyContent.Topic)
	assert.Equal(suite.T(), outputMsgId, replyContent.JobId)
}
