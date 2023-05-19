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
	"fmt"
	"time"

	"github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/aws/amazon-ssm-agent/agent/contracts"
	"github.com/aws/amazon-ssm-agent/agent/jsonutil"
	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/amazon-ssm-agent/agent/messageservice/interactor/mgsinteractor/utils"
	mgsContracts "github.com/aws/amazon-ssm-agent/agent/session/contracts"
	"github.com/twinj/uuid"
)

// SessionCompleteReplyType defines methods and properties to handle the AgentComplete reply
type SessionCompleteReplyType struct {
	context               context.T
	agentResult           contracts.DocumentResult
	backOffSecond         int
	noOfContinuousRetries int
	shouldPersist         bool
	replyId               uuid.UUID
	retryNumber           int
}

// NewSessionCompleteType returns new session complete reply type
func NewSessionCompleteType(ctx context.T, res contracts.DocumentResult, replyId uuid.UUID, retryNumber int) IReplyType {
	sessionReplyTypeObj := &SessionCompleteReplyType{
		context:               ctx,
		agentResult:           res,
		shouldPersist:         false,
		noOfContinuousRetries: 3,
		backOffSecond:         1,
		replyId:               replyId,
		retryNumber:           retryNumber,
	}
	return sessionReplyTypeObj
}

// GetName return name of the reply type
func (srt *SessionCompleteReplyType) GetName() contracts.ResultType {
	return contracts.SessionResult
}

// ConvertToAgentMessage converts result to agent message
func (srt *SessionCompleteReplyType) ConvertToAgentMessage() (*mgsContracts.AgentMessage, error) {
	return srt.agentTaskComplete(&srt.agentResult)
}

// GetMessageUUID returns message UUID
// used for logging and persistence in the intercators
func (srt *SessionCompleteReplyType) GetMessageUUID() uuid.UUID {
	return srt.replyId
}

// GetRetryNumber denotes how many times the message was retried sending
// this includes only continuous retries
func (srt *SessionCompleteReplyType) GetRetryNumber() int {
	return srt.retryNumber
}

// ShouldPersistData denotes whether the reply should be persisted
func (srt *SessionCompleteReplyType) ShouldPersistData() bool {
	return srt.shouldPersist
}

// GetBackOffSecond returns the backoff time to wait till the agent
func (srt *SessionCompleteReplyType) GetBackOffSecond() int {
	return srt.backOffSecond
}

// GetNumberOfContinuousRetries represents the number of continuous retries needed during send reply failure
// After continuous retries, the result will be saved in the local disk if persist enabled.
func (srt *SessionCompleteReplyType) GetNumberOfContinuousRetries() int {
	return srt.noOfContinuousRetries
}

// IncrementRetries increment retry number
func (srt *SessionCompleteReplyType) IncrementRetries() int {
	srt.retryNumber++
	return srt.retryNumber
}

// GetResult get agent result
func (srt *SessionCompleteReplyType) GetResult() contracts.DocumentResult {
	return srt.agentResult
}

// agentTaskComplete constructs agent message from agent task complete reply
func (srt *SessionCompleteReplyType) agentTaskComplete(result *contracts.DocumentResult) (message *mgsContracts.AgentMessage, err error) {
	log := srt.context.Log()
	commandTopic := utils.GetTopicFromDocResult(result.ResultType, result.RelatedDocumentType)
	payload := srt.formatAgentTaskCompletePayload(log, result.LastPlugin, result.PluginResults, result.MessageID, commandTopic)
	var agentTaskCompletePayload mgsContracts.AgentTaskCompletePayload
	if err := jsonutil.Remarshal(payload, &agentTaskCompletePayload); err != nil {
		// should never happen
		log.Errorf("unable to parse AgentTaskCompletePayload: %v, err: %v", agentTaskCompletePayload, err)
		return nil, err
	}
	replyBytes, err := json.Marshal(agentTaskCompletePayload)
	if err != nil {
		// should not happen
		log.Errorf("Cannot build AgentTaskComplete message %s", err)
		return nil, err
	}
	log.Debug("Sending reply ", jsonutil.Indent(string(replyBytes)))
	agentMessage := &mgsContracts.AgentMessage{
		MessageType:    mgsContracts.TaskCompleteMessage,
		SchemaVersion:  1,
		CreatedDate:    uint64(time.Now().UnixNano() / 1000000),
		SequenceNumber: 0,
		Flags:          0,
		MessageId:      srt.GetMessageUUID(),
		Payload:        replyBytes,
	}
	return agentMessage, nil
}

// formatAgentTaskCompletePayload builds AgentTaskComplete message Payload from the total task result.
func (srt *SessionCompleteReplyType) formatAgentTaskCompletePayload(log log.T,
	pluginId string,
	pluginResults map[string]*contracts.PluginResult,
	sessionId string,
	topic utils.CommandTopic) mgsContracts.AgentTaskCompletePayload {

	if len(pluginResults) < 1 {
		log.Error("Error in FormatAgentTaskCompletePayload, the outputs map is empty!")
		return mgsContracts.AgentTaskCompletePayload{}
	}

	// get plugin result
	if pluginId == "" {
		// for instance reboot scenarios, it only contains document level result which does not contain pluginId.
		for key := range pluginResults {
			pluginId = key
			break
		}
	}
	instanceId, err := srt.context.Identity().InstanceID()
	if err != nil {
		log.Error("Unable to retrieve instance id", err)
	}
	pluginResult := pluginResults[pluginId]
	if pluginResult == nil {
		log.Error("Error in FormatAgentTaskCompletePayload, the pluginOutput is nil!")
		return mgsContracts.AgentTaskCompletePayload{}
	}

	sessionPluginResultOutput := mgsContracts.SessionPluginResultOutput{}
	if pluginResult.Error != "" {
		sessionPluginResultOutput.Output = pluginResult.Error
	} else if pluginResult.Output != nil {
		if err := jsonutil.Remarshal(pluginResult.Output, &sessionPluginResultOutput); err != nil {
			sessionPluginResultOutput.Output = fmt.Sprintf("%v", pluginResult.Output)
		}
	}
	payload := mgsContracts.AgentTaskCompletePayload{
		SchemaVersion:    1,
		TaskId:           sessionId,
		Topic:            string(topic),
		FinalTaskStatus:  string(pluginResult.Status),
		IsRoutingFailure: false,
		AwsAccountId:     "",
		InstanceId:       instanceId,
		Output:           sessionPluginResultOutput.Output,
		S3Bucket:         sessionPluginResultOutput.S3Bucket,
		S3UrlSuffix:      sessionPluginResultOutput.S3UrlSuffix,
		CwlGroup:         sessionPluginResultOutput.CwlGroup,
		CwlStream:        sessionPluginResultOutput.CwlStream,
		RetryNumber:      srt.retryNumber,
	}
	return payload
}
