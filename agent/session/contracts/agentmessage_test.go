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
	"crypto/sha256"
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/aws/amazon-ssm-agent/agent/contracts"
	"github.com/aws/amazon-ssm-agent/agent/jsonutil"
	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/stretchr/testify/assert"
	"github.com/twinj/uuid"
)

var (
	taskId         = "aws.ssm.2b196342-d7d4-436e-8f09-3883a1116ac3.i-57c0a7be"
	messageType    = InteractiveShellMessage
	schemaVersion  = uint32(1)
	messageId      = "dd01e56b-ff48-483e-a508-b5f073f31b16"
	createdDate    = uint64(1503434274948)
	payload        = []byte("payload")
	topic          = "test"
	destination    = "i-01234567"
	sessionId      = "2b196342-d7d4-436e-8f09-3883a1116ac3"
	docSchema      = "1.2"
	documentName   = "runShellScript"
	sequenceNumber = int64(2)
	instanceId     = "i-12345678"
)

func TestGetInteger(t *testing.T) {
	input := []byte{0x00, 0x00, 0x00, 0x00, 0xFF, 0x00}
	result, err := getInteger(log.NewMockLog(), input, 1)
	assert.Equal(t, int32(255), result)
	assert.Nil(t, err)

	input = []byte{0x00, 0x00, 0x00, 0x01, 0x00, 0x00}
	result, err = getInteger(log.NewMockLog(), input, 1)
	assert.Equal(t, int32(256), result)
	assert.Nil(t, err)

	input = []byte{0x00, 0x00, 0x00, 0x00, 0xFF, 0x00}
	result, err = getInteger(log.NewMockLog(), input, 2)
	assert.Equal(t, int32(0), result)
	assert.NotNil(t, err)
}

func TestGetBytesFromInteger(t *testing.T) {
	input := int32(256)
	result, err := integerToBytes(log.NewMockLog(), input)
	assert.Nil(t, err)
	assert.Equal(t, byte(0x00), result[0])
	assert.Equal(t, byte(0x00), result[1])
	assert.Equal(t, byte(0x01), result[2])
	assert.Equal(t, byte(0x00), result[3])
}

func TestPutInteger(t *testing.T) {
	input := []byte{0x00, 0x00, 0x00, 0x00, 0xFF, 0x00}
	err := putInteger(log.NewMockLog(), input, 1, 256)
	assert.Nil(t, err)
	assert.Equal(t, byte(0x00), input[1])
	assert.Equal(t, byte(0x00), input[2])
	assert.Equal(t, byte(0x01), input[3])
	assert.Equal(t, byte(0x00), input[4])

	result, err2 := getInteger(log.NewMockLog(), input, 1)
	assert.Nil(t, err2)
	assert.Equal(t, int32(256), result)
}

func TestGetLong(t *testing.T) {
	input := []byte{0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x01, 0x00, 0x01, 0x00, 0x00, 0x00}
	result, err := getLong(log.NewMockLog(), input, 1)
	assert.Equal(t, int64(65537), result)
	assert.Nil(t, err)
}

func TestPutLong(t *testing.T) {
	input := []byte{0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x01, 0x00, 0x01, 0x00, 0x00, 0x00}
	err := putLong(log.NewMockLog(), input, 1, 4294967296) // 2 to the 32 + 1
	assert.Nil(t, err)
	assert.Equal(t, byte(0x00), input[1])
	assert.Equal(t, byte(0x00), input[2])
	assert.Equal(t, byte(0x00), input[3])
	assert.Equal(t, byte(0x01), input[4])
	assert.Equal(t, byte(0x00), input[5])
	assert.Equal(t, byte(0x00), input[6])
	assert.Equal(t, byte(0x00), input[7])
	assert.Equal(t, byte(0x00), input[8])

	testLong, err2 := getLong(log.NewMockLog(), input, 1)
	assert.Nil(t, err2)
	assert.Equal(t, int64(4294967296), testLong)
}

func TestPutGetString(t *testing.T) {
	input := []byte{0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x01, 0x01, 0x00, 0x01}
	err1 := putString(log.NewMockLog(), input, 1, 8, "hello")
	assert.Nil(t, err1)

	result, err := getString(log.NewMockLog(), input, 1, 8)
	assert.Nil(t, err)
	assert.Equal(t, "hello", result)

}

func TestSerializeAndDeserializeAgentMessage(t *testing.T) {

	u, _ := uuid.Parse(messageId)

	agentMessage := &AgentMessage{
		MessageType:    messageType,
		SchemaVersion:  schemaVersion,
		CreatedDate:    createdDate,
		SequenceNumber: 1,
		Flags:          2,
		MessageId:      u,
		Payload:        payload,
	}

	// Test SerializeAgentMessage

	serializedBytes, err := agentMessage.Serialize(log.NewMockLog())
	assert.Nil(t, err, "Error serializing message")

	seralizedMessageType := strings.TrimRight(string(serializedBytes[AgentMessage_MessageTypeOffset:AgentMessage_MessageTypeOffset+AgentMessage_MessageTypeLength-1]), " ")
	assert.Equal(t, seralizedMessageType, messageType)

	serializedVersion, err := getUInteger(log.NewMockLog(), serializedBytes, AgentMessage_SchemaVersionOffset)
	assert.Nil(t, err)
	assert.Equal(t, serializedVersion, schemaVersion)

	serializedCD, err := getULong(log.NewMockLog(), serializedBytes, AgentMessage_CreatedDateOffset)
	assert.Nil(t, err)
	assert.Equal(t, serializedCD, createdDate)

	serializedSequence, err := getLong(log.NewMockLog(), serializedBytes, AgentMessage_SequenceNumberOffset)
	assert.Nil(t, err)
	assert.Equal(t, serializedSequence, int64(1))

	serializedFlags, err := getULong(log.NewMockLog(), serializedBytes, AgentMessage_FlagsOffset)
	assert.Nil(t, err)
	assert.Equal(t, serializedFlags, uint64(2))

	seralizedMessageId, err := getUuid(log.NewMockLog(), serializedBytes, AgentMessage_MessageIdOffset)
	assert.Nil(t, err)
	assert.Equal(t, seralizedMessageId.String(), messageId)

	serializedDigest, err := getBytes(log.NewMockLog(), serializedBytes, AgentMessage_PayloadDigestOffset, AgentMessage_PayloadDigestLength)
	assert.Nil(t, err)
	hasher := sha256.New()
	hasher.Write(agentMessage.Payload)
	expectedHash := hasher.Sum(nil)
	assert.True(t, reflect.DeepEqual(serializedDigest, expectedHash))

	// Test DeserializeAgentMessage
	deserializedAgentMessage := &AgentMessage{}
	err = deserializedAgentMessage.Deserialize(log.NewMockLog(), serializedBytes)
	assert.Nil(t, err)
	assert.Equal(t, messageType, deserializedAgentMessage.MessageType)
	assert.Equal(t, schemaVersion, deserializedAgentMessage.SchemaVersion)
	assert.Equal(t, messageId, deserializedAgentMessage.MessageId.String())
	assert.Equal(t, createdDate, deserializedAgentMessage.CreatedDate)
	assert.Equal(t, uint64(2), deserializedAgentMessage.Flags)
	assert.Equal(t, int64(1), deserializedAgentMessage.SequenceNumber)
	assert.True(t, reflect.DeepEqual(payload, deserializedAgentMessage.Payload))
}

func TestParseAgentMessage(t *testing.T) {
	u, _ := uuid.Parse(messageId)

	agentJson := "{\"DataChannelId\":\"44da928d-1200-4501-a38a-f10d72e38cc4\",\"documentContent\":{\"schemaVersion\":\"1.0\"," +
		"\"inputs\":{\"cloudWatchLogGroup\":\"\",\"s3BucketName\":\"\",\"s3KeyPrefix\":\"\"},\"description\":\"Document to hold " +
		"regional settings for Session Manager\",\"sessionType\":\"Standard_Stream\",\"parameters\":{}," +
		"\"properties\":{\"windows\":{\"commands\":\"date\",\"runAsElevated\":true},\"linux\":{\"commands\":\"ls\",\"runAsElevated\":true}}}," +
		"\"sessionId\":\"44da928d-1200-4501-a38a-f10d72e38cc4\"," +
		"\"runAsUser\":\"test-user\"," +
		"\"DataChannelToken\":\"AAEAAdDZESkS1C2/AWLlDccG608LYJUJZJLkxcjxl0x1T70kAAAAAFrozgJYbJT2fY6yQPDqQZhygozZ83LhsoYdP7VWmuo\"}"
	mgsPayload := MGSPayload{
		Payload:       string(agentJson),
		TaskId:        taskId,
		Topic:         topic,
		SchemaVersion: 1,
	}
	mgsPayloadJson, err := json.Marshal(mgsPayload)
	agentMessage := &AgentMessage{
		HeaderLength:   20,
		MessageType:    messageType,
		SchemaVersion:  schemaVersion,
		CreatedDate:    createdDate,
		SequenceNumber: 1,
		Flags:          2,
		MessageId:      u,
		Payload:        mgsPayloadJson,
	}

	shellPropsObj := ShellProperties{
		Windows: ShellConfig{
			Commands:      "date",
			RunAsElevated: true,
		},
		Linux: ShellConfig{
			Commands:      "ls",
			RunAsElevated: true,
		},
	}

	var shellProps interface{}
	jsonutil.Remarshal(shellPropsObj, &shellProps)

	assert.Nil(t, agentMessage.Validate())

	docState, err := agentMessage.ParseAgentMessage(context.NewMockDefault(), "", "i-123", "client-id")
	pluginInfo := docState.InstancePluginsInformation
	assert.Nil(t, err)
	assert.NotNil(t, docState)
	assert.Equal(t, "1.0", docState.SchemaVersion)
	assert.Equal(t, 1, len(pluginInfo))
	assert.Equal(t, "44da928d-1200-4501-a38a-f10d72e38cc4", pluginInfo[0].Configuration.MessageId)
	assert.Equal(t, contracts.StartSession, docState.DocumentType)
	assert.Equal(t, "44da928d-1200-4501-a38a-f10d72e38cc4", pluginInfo[0].Configuration.SessionId)
	assert.Equal(t, shellProps, pluginInfo[0].Configuration.Properties)
	assert.Equal(t, "test-user", pluginInfo[0].Configuration.RunAsUser)
}

func TestValidateReturnsErrorWithEmptyAgentMessage(t *testing.T) {
	agentMessage := &AgentMessage{}
	err := agentMessage.Validate()
	assert.NotNil(t, err)
}
