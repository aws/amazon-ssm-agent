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

// Package replytypes will be responsible for handling agent run command reply type from the processor
package replytypes

import (
	"github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/aws/amazon-ssm-agent/agent/contracts"
	"github.com/aws/amazon-ssm-agent/agent/messageservice/interactor/mgsinteractor/utils"
	"github.com/aws/amazon-ssm-agent/agent/runcommand"
	mgsContracts "github.com/aws/amazon-ssm-agent/agent/session/contracts"
	"github.com/twinj/uuid"
)

// NewAgentRunCommandReplyType returns new Agent Run Command reply type
func NewAgentRunCommandReplyType(ctx context.T, res contracts.DocumentResult, replyId uuid.UUID, retryNumber int) IReplyType {
	return &AgentRunCommandReplyType{
		context:               ctx,
		agentResult:           res,
		noOfContinuousRetries: 1,
		backOffSecond:         1,
		shouldPersist:         true,
		replyId:               replyId,
		retryNumber:           retryNumber,
	}
}

// AgentRunCommandReplyType defines methods and properties to handle RunCommandResult
type AgentRunCommandReplyType struct {
	context               context.T
	agentResult           contracts.DocumentResult
	backOffSecond         int
	noOfContinuousRetries int
	shouldPersist         bool
	replyId               uuid.UUID
	retryNumber           int
}

// GetName return name of the reply type
func (ad *AgentRunCommandReplyType) GetName() contracts.ResultType {
	return contracts.RunCommandResult
}

// ConvertToAgentMessage converts result to agent message
func (ad *AgentRunCommandReplyType) ConvertToAgentMessage() (*mgsContracts.AgentMessage, error) {
	return ad.constructMessage(&ad.agentResult)
}

// GetMessageUUID returns message UUID
// used for logging and persistence in the interactors
func (ad *AgentRunCommandReplyType) GetMessageUUID() uuid.UUID {
	return ad.replyId
}

// GetResult get agent result
func (ad *AgentRunCommandReplyType) GetResult() contracts.DocumentResult {
	return ad.agentResult
}

// GetRetryNumber denotes how many times the message was retried sending
// this includes only continuous retries
func (ad *AgentRunCommandReplyType) GetRetryNumber() int {
	return ad.retryNumber
}

// ShouldPersistData denotes whether the reply should be persisted
func (ad *AgentRunCommandReplyType) ShouldPersistData() bool {
	return ad.shouldPersist
}

// GetBackOffSecond returns the backoff time to wait till the agent
func (ad *AgentRunCommandReplyType) GetBackOffSecond() int {
	return ad.backOffSecond
}

// GetNumberOfContinuousRetries represents the number of continuous retries needed during send reply failure
// After continuous retries, the result will be saved in the local disk if persist enabled.
func (ad *AgentRunCommandReplyType) GetNumberOfContinuousRetries() int {
	return ad.noOfContinuousRetries
}

// IncrementRetries increment retry number
func (ad *AgentRunCommandReplyType) IncrementRetries() int {
	ad.retryNumber++
	return ad.retryNumber
}

// constructMessage constructs agent message with the reply as payload
func (ad *AgentRunCommandReplyType) constructMessage(result *contracts.DocumentResult) (*mgsContracts.AgentMessage, error) {
	log := ad.context.Log()
	appConfig := ad.context.AppConfig()
	agentInfo := contracts.AgentInfo{
		Lang:      appConfig.Os.Lang,
		Name:      appConfig.Agent.Name,
		Version:   appConfig.Agent.Version,
		Os:        appConfig.Os.Name,
		OsVersion: appConfig.Os.Version,
	}
	replyPayload := runcommand.FormatPayload(log, result.LastPlugin, agentInfo, result.PluginResults)
	commandTopic := utils.GetTopicFromDocResult(result.ResultType, result.RelatedDocumentType)
	return utils.GenerateAgentJobReplyPayload(log, ad.replyId, result.MessageID, replyPayload, commandTopic)
}
