package utils

import (
	"encoding/json"
	"testing"

	logger "github.com/aws/amazon-ssm-agent/agent/log"
	model "github.com/aws/amazon-ssm-agent/agent/runcommand/contracts"
	"github.com/aws/amazon-ssm-agent/agent/session/contracts"
	"github.com/stretchr/testify/assert"
	"github.com/twinj/uuid"
)

func TestGenerateAgentJobReplyPayload(t *testing.T) {
	log := logger.NewMockLog()
	uuid := uuid.NewV4()
	messageID := "messageid"
	sendReplyPayload := model.SendReplyPayload{}
	agentMsg, err := GenerateAgentJobReplyPayload(log, uuid, messageID, sendReplyPayload, SendCommandTopic)
	expectedMessage := &contracts.AgentMessage{
		MessageType:    contracts.AgentJobReply,
		SchemaVersion:  1,
		CreatedDate:    agentMsg.CreatedDate, // using agentMsg.CreatedDate to prevent flaky test
		SequenceNumber: 0,
		Flags:          0,
		MessageId:      uuid,
		Payload:        getPayload(sendReplyPayload, messageID),
	}
	assert.Nil(t, err)
	assert.Equal(t, expectedMessage, agentMsg)
}

func getPayload(replyPayload model.SendReplyPayload, messageID string) []byte {
	payloadB, _ := json.Marshal(replyPayload)
	payload := string(payloadB)
	finalReplyContent := contracts.AgentJobReplyContent{
		SchemaVersion: 1,
		JobId:         messageID,
		Content:       payload,
		Topic:         string(SendCommandTopic),
	}
	finalReplyContentBytes, _ := json.Marshal(finalReplyContent)
	return finalReplyContentBytes
}
