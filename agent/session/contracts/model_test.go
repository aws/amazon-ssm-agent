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

// contracts package defines all channel messages structure.
package contracts

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/stretchr/testify/assert"
	"github.com/twinj/uuid"
)

func TestSerializeAndDeserializeAgentMessageWithAcknowledgeContent(t *testing.T) {
	uuid.SwitchFormat(uuid.CleanHyphen)
	uid := uuid.NewV4()

	acknowledgeContent := AcknowledgeContent{
		MessageType:         messageType,
		MessageId:           uid.String(),
		SequenceNumber:      sequenceNumber,
		IsSequentialMessage: true,
	}

	acknowledgeContentBytes, err := acknowledgeContent.Serialize(log.NewMockLog())

	agentMessage := &AgentMessage{
		MessageType:    AcknowledgeMessage,
		SchemaVersion:  1,
		CreatedDate:    uint64(time.Now().UnixNano() / 1000000),
		SequenceNumber: 0,
		Flags:          3,
		MessageId:      uid,
		Payload:        acknowledgeContentBytes,
	}

	serializedAgentMsg, err := agentMessage.Serialize(log.NewMockLog())
	deserializedAgentMsg := &AgentMessage{}
	deserializedAgentMsg.Deserialize(log.NewMockLog(), serializedAgentMsg)
	deserializedAcknowledgeContent := &AcknowledgeContent{}
	err = deserializedAcknowledgeContent.Deserialize(log.NewMockLog(), *deserializedAgentMsg)

	assert.Nil(t, err)
	assert.Equal(t, messageType, deserializedAcknowledgeContent.MessageType)
	assert.Equal(t, uid.String(), deserializedAcknowledgeContent.MessageId)
	assert.Equal(t, sequenceNumber, deserializedAcknowledgeContent.SequenceNumber)
	assert.True(t, deserializedAcknowledgeContent.IsSequentialMessage)
}

func TestDeserializeAgentMessageWithChannelClosed(t *testing.T) {
	channelClosed := ChannelClosed{
		MessageType:   ChannelClosedMessage,
		MessageId:     messageId,
		DestinationId: "destination-id",
		SessionId:     sessionId,
		SchemaVersion: 1,
		CreatedDate:   "2018-01-01",
	}

	u, _ := uuid.Parse(messageId)
	channelClosedJson, err := json.Marshal(channelClosed)
	agentMessage := AgentMessage{
		MessageType:    ChannelClosedMessage,
		SchemaVersion:  schemaVersion,
		CreatedDate:    createdDate,
		SequenceNumber: 1,
		Flags:          2,
		MessageId:      u,
		Payload:        channelClosedJson,
	}

	deserializedChannelClosed := &ChannelClosed{}
	deserializedChannelClosed.Deserialize(log.NewMockLog(), agentMessage)

	assert.Nil(t, err)
	assert.Equal(t, ChannelClosedMessage, deserializedChannelClosed.MessageType)
	assert.Equal(t, messageId, deserializedChannelClosed.MessageId)
	assert.Equal(t, sessionId, deserializedChannelClosed.SessionId)
	assert.Equal(t, "destination-id", deserializedChannelClosed.DestinationId)
}
