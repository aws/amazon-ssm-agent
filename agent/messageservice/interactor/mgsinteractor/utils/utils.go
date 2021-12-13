package utils

import (
	"encoding/json"
	"github.com/aws/amazon-ssm-agent/agent/contracts"
	"github.com/aws/amazon-ssm-agent/agent/jsonutil"
	"time"

	"github.com/aws/amazon-ssm-agent/agent/log"
	messageContracts "github.com/aws/amazon-ssm-agent/agent/runcommand/contracts"
	mgsContracts "github.com/aws/amazon-ssm-agent/agent/session/contracts"
	"github.com/twinj/uuid"
)

type CommandTopic string

const (
	// SendCommandTopic represents the topic added in the agent message payload for the document replies
	// for documents executed with topic aws.ssm.sendCommand
	SendCommandTopic CommandTopic = "aws.ssm.sendCommand"

	// CancelCommandTopic represents the topic added in the agent message payload for the document replies
	// for documents executed with topic aws.ssm.cancelCommand
	CancelCommandTopic CommandTopic = "aws.ssm.cancelCommand"

	// SessionCompleteTopic represents session agent complete topic
	SessionCompleteTopic = CommandTopic(mgsContracts.TaskCompleteMessage)
)

// GetTopicFromDocResult returns topic based on doc result
func GetTopicFromDocResult(resultType contracts.ResultType, documentType contracts.DocumentType) CommandTopic {
	var commandTopic CommandTopic
	if resultType == contracts.RunCommandResult {
		if documentType == contracts.CancelCommand {
			commandTopic = CancelCommandTopic // we do not send replies using this topic mostly
		} else {
			commandTopic = SendCommandTopic // use send command as default
		}
	} else if resultType == contracts.SessionResult {
		return SessionCompleteTopic
	}
	return commandTopic
}

// GenerateAgentJobReplyPayload generates AgentJobReply agent message
func GenerateAgentJobReplyPayload(log log.T, agentMessageUUID uuid.UUID, messageID string, replyPayload messageContracts.SendReplyPayload, topic CommandTopic) (*mgsContracts.AgentMessage, error) {
	payloadB, err := json.Marshal(replyPayload)
	if err != nil {
		log.Error("could not marshal reply payload!", err)
		return nil, err
	}
	payload := string(payloadB)
	log.Info("Sending reply ", jsonutil.Indent(payload))
	finalReplyContent := mgsContracts.AgentJobReplyContent{
		SchemaVersion: 1,
		JobId:         messageID,
		Content:       payload,
		Topic:         string(topic),
	}

	finalReplyContentBytes, err := json.Marshal(finalReplyContent)
	if err != nil {
		log.Errorf("Cannot build reply message %v", err)
		return nil, err
	}

	repMsg := &mgsContracts.AgentMessage{
		MessageType:    mgsContracts.AgentJobReply,
		SchemaVersion:  1,
		CreatedDate:    uint64(time.Now().UnixNano() / 1000000),
		SequenceNumber: 0,
		Flags:          0,
		MessageId:      agentMessageUUID,
		Payload:        finalReplyContentBytes,
	}
	return repMsg, nil
}
