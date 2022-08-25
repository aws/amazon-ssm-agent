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
	"github.com/aws/amazon-ssm-agent/agent/jsonutil"
	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/amazon-ssm-agent/agent/messageservice/interactor/mgsinteractor/utils"
	mgsContracts "github.com/aws/amazon-ssm-agent/agent/session/contracts"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
	"github.com/twinj/uuid"
)

type SessionCompleteReplyTestSuite struct {
	suite.Suite
}

var (
	instanceId  = "i-123123123"
	messageId   = "dd01e56b-ff48-483e-a508-b5f073f31b16"
	errorMsg    = "plugin failed"
	s3Bucket    = "s3Bucket"
	s3UrlSuffix = "s3UrlSuffix"
	cwlGroup    = "cwlGroup"
	cwlStream   = "cwlStream"
	status      = contracts.ResultStatusInProgress
)

// Execute the test suite
func TestSessionCompleteReplyTestSuite(t *testing.T) {
	suite.Run(t, new(SessionCompleteReplyTestSuite))
}

// Testing the name
func (suite *SessionCompleteReplyTestSuite) TestName() {
	ctx := context.NewMockDefault()
	docResult := contracts.DocumentResult{ResultType: contracts.SessionResult}
	uuidVal := uuid.NewV4()
	sessionComplete := NewSessionCompleteType(ctx, docResult, uuidVal, 0)
	assert.Equal(suite.T(), sessionComplete.GetName(), contracts.SessionResult)
}

func (suite *SessionCompleteReplyTestSuite) TestSessionCompleteReply_BasicInitializationCheck() {
	ctx := context.NewMockDefault()
	docResult := contracts.DocumentResult{ResultType: contracts.SessionResult}
	uuidVal := uuid.NewV4()
	sessionComplete := NewSessionCompleteType(ctx, docResult, uuidVal, 0)
	assert.Equal(suite.T(), uuidVal.String(), sessionComplete.GetMessageUUID().String())
	assert.Equal(suite.T(), 1, sessionComplete.GetBackOffSecond())
	assert.Equal(suite.T(), 3, sessionComplete.GetNumberOfContinuousRetries())
	assert.Equal(suite.T(), false, sessionComplete.ShouldPersistData())
	assert.Equal(suite.T(), 0, sessionComplete.GetRetryNumber())
}

func (suite *SessionCompleteReplyTestSuite) TestSessionCompleteReply_InitializeWithRetryNumberCheck() {
	ctx := context.NewMockDefault()
	docResult := contracts.DocumentResult{ResultType: contracts.SessionResult}
	uuidVal := uuid.NewV4()
	sessionComplete := NewSessionCompleteType(ctx, docResult, uuidVal, 1)
	assert.Equal(suite.T(), uuidVal.String(), sessionComplete.GetMessageUUID().String())
	assert.Equal(suite.T(), 1, sessionComplete.GetBackOffSecond())
	assert.Equal(suite.T(), 3, sessionComplete.GetNumberOfContinuousRetries())
	assert.Equal(suite.T(), false, sessionComplete.ShouldPersistData())
	assert.Equal(suite.T(), 1, sessionComplete.GetRetryNumber())
}

func (suite *SessionCompleteReplyTestSuite) TestSessionCompleteReply_AgentMessageGenerationCheck() {
	ctx := context.NewMockDefault()
	plugInResults := make(map[string]*contracts.PluginResult)
	plugInResults["testPlugin"] = &contracts.PluginResult{Status: contracts.ResultStatusSuccess}
	docResult := contracts.DocumentResult{ResultType: contracts.SessionResult, LastPlugin: "testPlugin", PluginResults: plugInResults}
	uuidVal := uuid.NewV4()
	agentComplete := NewSessionCompleteType(ctx, docResult, uuidVal, 0)

	agentMessage, err := agentComplete.ConvertToAgentMessage()
	assert.Nil(suite.T(), err)
	assert.Equal(suite.T(), uuidVal.String(), agentMessage.MessageId.String())
	replyContent := mgsContracts.AgentTaskCompletePayload{}
	err = json.Unmarshal(agentMessage.Payload, &replyContent)
	assert.Nil(suite.T(), err)
	assert.Equal(suite.T(), mgsContracts.TaskCompleteMessage, replyContent.Topic)
	assert.Equal(suite.T(), string(contracts.ResultStatusSuccess), replyContent.FinalTaskStatus)
}

// test case for document result when instance reboot happens.
func (suite *SessionCompleteReplyTestSuite) TestSessionCompleteReply_BuildAgentTaskCompleteWhenPluginIdIsEmptyAndStatusIsFailed() {
	log := log.NewMockLog()
	sessionPluginResultOutput := mgsContracts.SessionPluginResultOutput{
		Output:      errorMsg,
		S3Bucket:    s3Bucket,
		S3UrlSuffix: s3UrlSuffix,
		CwlGroup:    cwlGroup,
		CwlStream:   cwlStream,
	}
	pluginResults := make(map[string]*contracts.PluginResult)
	pluginResult := contracts.PluginResult{
		PluginName: "Standard_Stream",
		Status:     contracts.ResultStatusFailed,
		Output:     sessionPluginResultOutput,
	}
	pluginResults["Standard_Stream"] = &pluginResult

	result := contracts.DocumentResult{
		Status:          contracts.ResultStatusFailed,
		PluginResults:   pluginResults,
		LastPlugin:      "",
		MessageID:       messageId,
		AssociationID:   "",
		NPlugins:        1,
		DocumentName:    "documentName",
		DocumentVersion: "1",
	}
	ctx := context.NewMockDefault()
	uuidVal := uuid.NewV4()
	agentComplete := NewSessionCompleteType(ctx, result, uuidVal, 0).(*SessionCompleteReplyType)

	payloadInterface := agentComplete.formatAgentTaskCompletePayload(log, result.LastPlugin, pluginResults, result.MessageID, utils.SessionCompleteTopic)

	var payload mgsContracts.AgentTaskCompletePayload
	jsonutil.Remarshal(payloadInterface, &payload)

	assert.Equal(suite.T(), instanceId, payload.InstanceId)
	assert.Equal(suite.T(), string(contracts.ResultStatusFailed), payload.FinalTaskStatus)
	assert.Equal(suite.T(), messageId, payload.TaskId)
	assert.Equal(suite.T(), errorMsg, payload.Output)
	assert.Equal(suite.T(), 0, payload.RetryNumber) // starts with zero
}

// Testing buildAgentTaskComplete.
func (suite *SessionCompleteReplyTestSuite) TestBuildAgentTaskCompleteWhenPluginResultOutputHasS3AndCWInfo() {
	log := log.NewMockLog()
	sessionPluginResultOutput := mgsContracts.SessionPluginResultOutput{
		Output:      errorMsg,
		S3Bucket:    s3Bucket,
		S3UrlSuffix: s3UrlSuffix,
		CwlGroup:    cwlGroup,
		CwlStream:   cwlStream,
	}
	pluginResults := make(map[string]*contracts.PluginResult)
	pluginResult := contracts.PluginResult{
		PluginName: "Standard_Stream",
		Status:     contracts.ResultStatusSuccess,
		Output:     sessionPluginResultOutput,
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

	ctx := context.NewMockDefault()
	uuidVal := uuid.NewV4()
	agentComplete := NewSessionCompleteType(ctx, result, uuidVal, 0).(*SessionCompleteReplyType)

	payloadInterface := agentComplete.formatAgentTaskCompletePayload(log, result.LastPlugin, pluginResults, result.MessageID, utils.SessionCompleteTopic)

	var payload mgsContracts.AgentTaskCompletePayload
	jsonutil.Remarshal(payloadInterface, &payload)

	assert.Equal(suite.T(), instanceId, payload.InstanceId)
	assert.Equal(suite.T(), string(contracts.ResultStatusSuccess), payload.FinalTaskStatus)
	assert.Equal(suite.T(), messageId, payload.TaskId)
	assert.Equal(suite.T(), errorMsg, payload.Output)
	assert.Equal(suite.T(), s3Bucket, payload.S3Bucket)
	assert.Equal(suite.T(), s3UrlSuffix, payload.S3UrlSuffix)
	assert.Equal(suite.T(), cwlGroup, payload.CwlGroup)
	assert.Equal(suite.T(), cwlStream, payload.CwlStream)
	assert.Equal(suite.T(), 0, payload.RetryNumber) // starts with zero
}

// Testing buildAgentTaskComplete.
func (suite *SessionCompleteReplyTestSuite) TestBuildAgentTaskCompleteWhenPluginResultOutputHasError() {
	log := log.NewMockLog()
	pluginResults := make(map[string]*contracts.PluginResult)
	pluginResult := contracts.PluginResult{
		PluginName: "Standard_Stream",
		Status:     contracts.ResultStatusFailed,
		Output:     errorMsg,
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
	ctx := context.NewMockDefault()
	uuidVal := uuid.NewV4()
	agentComplete := NewSessionCompleteType(ctx, result, uuidVal, 0).(*SessionCompleteReplyType)

	payloadInterface := agentComplete.formatAgentTaskCompletePayload(log, result.LastPlugin, pluginResults, result.MessageID, utils.SessionCompleteTopic)

	var payload mgsContracts.AgentTaskCompletePayload
	jsonutil.Remarshal(payloadInterface, &payload)

	assert.Equal(suite.T(), instanceId, payload.InstanceId)
	assert.Equal(suite.T(), string(contracts.ResultStatusFailed), payload.FinalTaskStatus)
	assert.Equal(suite.T(), messageId, payload.TaskId)
	assert.Equal(suite.T(), errorMsg, payload.Output)
	assert.Equal(suite.T(), "", payload.S3Bucket)
	assert.Equal(suite.T(), "", payload.S3UrlSuffix)
	assert.Equal(suite.T(), "", payload.CwlGroup)
	assert.Equal(suite.T(), "", payload.CwlStream)
	assert.Equal(suite.T(), 0, payload.RetryNumber) // starts with zero
}

// Testing buildAgentTaskComplete.
func (suite *SessionCompleteReplyTestSuite) TestBuildAgentTaskComplete() {
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
	ctx := context.NewMockDefault()
	uuidVal := uuid.NewV4()
	agentComplete := NewSessionCompleteType(ctx, result, uuidVal, 0).(*SessionCompleteReplyType)
	payloadInterface := agentComplete.formatAgentTaskCompletePayload(log, result.LastPlugin, pluginResults, result.MessageID, utils.SessionCompleteTopic)

	var payload mgsContracts.AgentTaskCompletePayload
	jsonutil.Remarshal(payloadInterface, &payload)

	assert.Equal(suite.T(), instanceId, payload.InstanceId)
	assert.Equal(suite.T(), string(status), payload.FinalTaskStatus)
	assert.Equal(suite.T(), messageId, payload.TaskId)
	assert.Equal(suite.T(), "", payload.Output)
	assert.Equal(suite.T(), "", payload.S3Bucket)
	assert.Equal(suite.T(), "", payload.S3UrlSuffix)
	assert.Equal(suite.T(), "", payload.CwlGroup)
	assert.Equal(suite.T(), "", payload.CwlStream)
	assert.Equal(suite.T(), 0, payload.RetryNumber) // starts with zero
}
