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
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/aws/amazon-ssm-agent/agent/contracts"
	"github.com/aws/amazon-ssm-agent/agent/docparser"
	logger "github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/amazon-ssm-agent/agent/times"
	"github.com/twinj/uuid"
)

type IAgentMessage interface {
	Deserialize(log logger.T, input []byte) (err error)
	Serialize(log logger.T) (result []byte, err error)
	Validate() error
	ParseAgentMessage(context context.T, messagesOrchestrationRootDir string, instanceId string, clientId string) (*contracts.DocumentState, error)
}

// AgentMessage represents a message for agent to send/receive. AgentMessage Message in MGS is equivalent to MDS' InstanceMessage.
// All agent messages are sent in this form to the MGS service.
type AgentMessage struct {
	HeaderLength   uint32
	MessageType    string
	SchemaVersion  uint32
	CreatedDate    uint64
	SequenceNumber int64
	Flags          uint64
	MessageId      uuid.UUID
	PayloadDigest  []byte
	PayloadType    uint32
	PayloadLength  uint32
	Payload        []byte
}

// HL - HeaderLength is a 4 byte integer that represents the header length.
// MessageType is a 32 byte UTF-8 string containing the message type.
// SchemaVersion is a 4 byte integer containing the message schema version number.
// CreatedDate is an 8 byte integer containing the message create epoch millis in UTC.
// SequenceNumber is an 8 byte integer containing the message sequence number for serialized message streams.
// Flags is an 8 byte unsigned integer containing a packed array of control flags:
//   Bit 0 is SYN - SYN is set (1) when the recipient should consider Seq to be the first message number in the stream
//   Bit 1 is FIN - FIN is set (1) when this message is the final message in the sequence.
// MessageId is a 40 byte UTF-8 string containing a random UUID identifying this message.
// Payload digest is a 32 byte containing the SHA-256 hash of the payload.
// Payload Type is a 4 byte integer containing the payload type.
// Payload length is an 4 byte unsigned integer containing the byte length of data in the Payload field.
// Payload is a variable length byte data.
//
// | HL|         MessageType           |Ver|  CD   |  Seq  | Flags |
// |         MessageId                     |           Digest              |PayType| PayLen|
// |         Payload      			|

const (
	AgentMessage_HLLength             = 4
	AgentMessage_MessageTypeLength    = 32
	AgentMessage_SchemaVersionLength  = 4
	AgentMessage_CreatedDateLength    = 8
	AgentMessage_SequenceNumberLength = 8
	AgentMessage_FlagsLength          = 8
	AgentMessage_MessageIdLength      = 16
	AgentMessage_PayloadDigestLength  = 32
	AgentMessage_PayloadTypeLength    = 4
	AgentMessage_PayloadLengthLength  = 4
)

const (
	AgentMessage_HLOffset             = 0
	AgentMessage_MessageTypeOffset    = AgentMessage_HLOffset + AgentMessage_HLLength
	AgentMessage_SchemaVersionOffset  = AgentMessage_MessageTypeOffset + AgentMessage_MessageTypeLength
	AgentMessage_CreatedDateOffset    = AgentMessage_SchemaVersionOffset + AgentMessage_SchemaVersionLength
	AgentMessage_SequenceNumberOffset = AgentMessage_CreatedDateOffset + AgentMessage_CreatedDateLength
	AgentMessage_FlagsOffset          = AgentMessage_SequenceNumberOffset + AgentMessage_SequenceNumberLength
	AgentMessage_MessageIdOffset      = AgentMessage_FlagsOffset + AgentMessage_FlagsLength
	AgentMessage_PayloadDigestOffset  = AgentMessage_MessageIdOffset + AgentMessage_MessageIdLength
	AgentMessage_PayloadTypeOffset    = AgentMessage_PayloadDigestOffset + AgentMessage_PayloadDigestLength
	AgentMessage_PayloadLengthOffset  = AgentMessage_PayloadTypeOffset + AgentMessage_PayloadTypeLength
	AgentMessage_PayloadOffset        = AgentMessage_PayloadLengthOffset + AgentMessage_PayloadLengthLength
)

// Deserialize deserializes the byte array into an AgentMessage message.
// * Payload is a variable length byte data.
// * | HL|         MessageType           |Ver|  CD   |  Seq  | Flags |
// * |         MessageId                     |           Digest              |PayType| PayLen|
// * |         Payload      			|
func (agentMessage *AgentMessage) Deserialize(log logger.T, input []byte) (err error) {
	agentMessage.MessageType, err = getString(log, input, AgentMessage_MessageTypeOffset, AgentMessage_MessageTypeLength)
	if err != nil {
		log.Errorf("Could not deserialize field MessageType with error: %v", err)
		return err
	}
	agentMessage.SchemaVersion, err = getUInteger(log, input, AgentMessage_SchemaVersionOffset)
	if err != nil {
		log.Errorf("Could not deserialize field SchemaVersion with error: %v", err)
		return err
	}
	agentMessage.CreatedDate, err = getULong(log, input, AgentMessage_CreatedDateOffset)
	if err != nil {
		log.Errorf("Could not deserialize field CreatedDate with error: %v", err)
		return err
	}
	agentMessage.SequenceNumber, err = getLong(log, input, AgentMessage_SequenceNumberOffset)
	if err != nil {
		log.Errorf("Could not deserialize field SequenceNumber with error: %v", err)
		return err
	}
	agentMessage.Flags, err = getULong(log, input, AgentMessage_FlagsOffset)
	if err != nil {
		log.Errorf("Could not deserialize field Flags with error: %v", err)
		return err
	}
	agentMessage.MessageId, err = getUuid(log, input, AgentMessage_MessageIdOffset)
	if err != nil {
		log.Errorf("Could not deserialize field MessageId with error: %v", err)
		return err
	}

	agentMessage.PayloadDigest, err = getBytes(log, input, AgentMessage_PayloadDigestOffset, AgentMessage_PayloadDigestLength)
	if err != nil {
		log.Errorf("Could not deserialize field PayloadDigest with error: %v", err)
		return err
	}
	agentMessage.PayloadType, err = getUInteger(log, input, AgentMessage_PayloadTypeOffset)
	if err != nil {
		log.Errorf("Could not deserialize field PayloadType with error: %v", err)
		return err
	}

	agentMessage.PayloadLength, err = getUInteger(log, input, AgentMessage_PayloadLengthOffset)

	headerLength, herr := getUInteger(log, input, AgentMessage_HLOffset)
	if herr != nil {
		log.Errorf("Could not deserialize field HeaderLength with error: %v", err)
		return err
	}

	agentMessage.HeaderLength = headerLength
	agentMessage.Payload = input[headerLength+AgentMessage_PayloadLengthLength:]

	return nil
}

// Serialize serializes AgentMessage message into a byte array.
// * Payload is a variable length byte data.
// * | HL|         MessageType           |Ver|  CD   |  Seq  | Flags |
// * |         MessageId                     |           Digest              |PayType| PayLen|
// * |         Payload      			|
func (agentMessage *AgentMessage) Serialize(log logger.T) (result []byte, err error) {
	payloadLength := uint32(len(agentMessage.Payload))
	headerLength := uint32(AgentMessage_PayloadLengthOffset)
	// If the payloadinfo length is incorrect, fix it.
	if payloadLength != agentMessage.PayloadLength {
		log.Debugf("Payload length will be adjusted: ", agentMessage.PayloadLength)
		agentMessage.PayloadLength = payloadLength
	}

	totalMessageLength := headerLength + AgentMessage_PayloadLengthLength + payloadLength
	result = make([]byte, totalMessageLength)

	if err = putUInteger(log, result, AgentMessage_HLOffset, headerLength); err != nil {
		log.Errorf("Could not serialize HeaderLength with error: %v", err)
		return make([]byte, 1), err
	}

	startPosition := AgentMessage_MessageTypeOffset
	endPosition := AgentMessage_MessageTypeOffset + AgentMessage_MessageTypeLength - 1
	if err = putString(log, result, startPosition, endPosition, agentMessage.MessageType); err != nil {
		log.Errorf("Could not serialize MessageType with error: %v", err)
		return make([]byte, 1), err
	}

	if err = putUInteger(log, result, AgentMessage_SchemaVersionOffset, agentMessage.SchemaVersion); err != nil {
		log.Errorf("Could not serialize SchemaVersion with error: %v", err)
		return make([]byte, 1), err
	}

	if err = putULong(log, result, AgentMessage_CreatedDateOffset, agentMessage.CreatedDate); err != nil {
		log.Errorf("Could not serialize CreatedDate with error: %v", err)
		return make([]byte, 1), err
	}

	if err = putLong(log, result, AgentMessage_SequenceNumberOffset, agentMessage.SequenceNumber); err != nil {
		log.Errorf("Could not serialize SequenceNumber with error: %v", err)
		return make([]byte, 1), err
	}

	if err = putULong(log, result, AgentMessage_FlagsOffset, agentMessage.Flags); err != nil {
		log.Errorf("Could not serialize Flags with error: %v", err)
		return make([]byte, 1), err
	}

	if err = putUuid(log, result, AgentMessage_MessageIdOffset, agentMessage.MessageId); err != nil {
		log.Errorf("Could not serialize MessageId with error: %v", err)
		return make([]byte, 1), err
	}

	hasher := sha256.New()
	hasher.Write(agentMessage.Payload)

	startPosition = AgentMessage_PayloadDigestOffset
	endPosition = AgentMessage_PayloadDigestOffset + AgentMessage_PayloadDigestLength - 1
	if err = putBytes(log, result, startPosition, endPosition, hasher.Sum(nil)); err != nil {
		log.Errorf("Could not serialize PayloadDigest with error: %v", err)
		return make([]byte, 1), err
	}

	if err = putUInteger(log, result, AgentMessage_PayloadTypeOffset, agentMessage.PayloadType); err != nil {
		log.Errorf("Could not serialize PayloadType with error: %v", err)
		return make([]byte, 1), err
	}

	if err = putUInteger(log, result, AgentMessage_PayloadLengthOffset, agentMessage.PayloadLength); err != nil {
		log.Errorf("Could not serialize PayloadLength with error: %v", err)
		return make([]byte, 1), err
	}

	startPosition = AgentMessage_PayloadOffset
	endPosition = AgentMessage_PayloadOffset + int(payloadLength) - 1
	if err = putBytes(log, result, startPosition, endPosition, agentMessage.Payload); err != nil {
		log.Errorf("Could not serialize Payload with error: %v", err)
		return make([]byte, 1), err
	}

	return result, nil
}

// Validate returns error if the message is invalid
func (agentMessage *AgentMessage) Validate() error {
	if agentMessage.HeaderLength == 0 {
		return errors.New("HeaderLength cannot be zero")
	}
	if agentMessage.MessageType == "" {
		return errors.New("MessageType is missing")
	}
	if agentMessage.CreatedDate == 0 {
		return errors.New("CreatedDate is missing")
	}
	return nil
}

// ParseAgentMessage parses session message to documentState object for processor.
func (agentMessage *AgentMessage) ParseAgentMessage(context context.T,
	messagesOrchestrationRootDir string,
	instanceId string,
	clientId string) (*contracts.DocumentState, error) {

	log := context.Log()

	// parse message to retrieve parameters
	parsedMessagePayload, err := deserializeAgentTaskPayload(log, *agentMessage)
	if err != nil {
		errorMsg := "Encountered error while parsing input - internal error"
		return nil, fmt.Errorf("%v", errorMsg)
	}

	log.Debugf("Receiving session id %s, clientId: %s", parsedMessagePayload.SessionId, clientId)
	log.Tracef("Processing start-session message %s", agentMessage.Payload)

	// adapt plugin configuration format from MGS to plugin expected format
	documentInfo := buildDocumentInfo(*agentMessage, parsedMessagePayload.SessionId, parsedMessagePayload, instanceId)
	messageOrchestrationDirectory := filepath.Join(messagesOrchestrationRootDir, parsedMessagePayload.SessionId)

	parserInfo := docparser.DocumentParserInfo{
		OrchestrationDir: messageOrchestrationDirectory,
		MessageId:        documentInfo.MessageID,
		DocumentId:       documentInfo.DocumentID,
	}
	docContent := &docparser.SessionDocContent{
		SchemaVersion: parsedMessagePayload.DocumentContent.SchemaVersion,
		Description:   parsedMessagePayload.DocumentContent.Description,
		SessionType:   parsedMessagePayload.DocumentContent.SessionType,
		Inputs:        parsedMessagePayload.DocumentContent.Inputs,
		Parameters:    parsedMessagePayload.DocumentContent.Parameters,
		Properties:    parsedMessagePayload.DocumentContent.Properties,
	}

	docState, err := docparser.InitializeDocState(
		log,
		contracts.StartSession,
		docContent,
		documentInfo,
		parserInfo,
		parsedMessagePayload.Parameters)
	if err != nil {
		return nil, fmt.Errorf("error initialing document state: %s", err)
	}

	log.Debugf("Docstate document ID after Initializing: %s", docState.DocumentInformation.DocumentID)
	return &docState, nil
}

// deserializeAgentTaskPayload parses agent task message payloads received.
func deserializeAgentTaskPayload(log logger.T, agentMessage AgentMessage) (agentTaskPayload AgentTaskPayload, err error) {
	if agentMessage.MessageType != InteractiveShellMessage {
		err = errors.New("AgentMessage is not of type interactive_shell")
		return
	}
	whatstring := string(agentMessage.Payload)
	log.Debug(whatstring)

	var mgsPayload MGSPayload
	if err = json.Unmarshal(agentMessage.Payload, &mgsPayload); err != nil {
		log.Errorf("Could not deserialize rawMessage: %s", string(agentMessage.Payload))
	}

	// workaround to unmarshal the real payload (should be fixed from the service side)
	if err = json.Unmarshal([]byte(mgsPayload.Payload), &agentTaskPayload); err != nil {
		log.Errorf("Could not deserialize AgentTask payload rawMessage: %s", string(mgsPayload.Payload))
	}
	return
}

// buildDocumentInfo builds new DocumentInfo object
func buildDocumentInfo(
	agentMessage AgentMessage,
	sessionId string,
	parsedMessagePayload AgentTaskPayload,
	instanceId string) contracts.DocumentInfo {

	// DocumentID is a unique name for file system
	// For Session, DocumentID = SessionID
	return contracts.DocumentInfo{
		CommandID:      sessionId,
		CreatedDate:    time.Unix(int64(agentMessage.CreatedDate), 0).String(),
		DocumentID:     sessionId,
		InstanceID:     instanceId,
		MessageID:      sessionId,
		RunID:          times.ToIsoDashUTC(times.DefaultClock.Now()),
		DocumentName:   parsedMessagePayload.DocumentName,
		DocumentStatus: contracts.ResultStatusInProgress,
		RunAsUser:      parsedMessagePayload.RunAsUser,
	}
}

// getUuid gets the 128bit uuid from an array of bytes starting from the offset.
func getUuid(log logger.T, byteArray []byte, offset int) (result uuid.UUID, err error) {
	byteArrayLength := len(byteArray)
	if offset > byteArrayLength-1 || offset+16-1 > byteArrayLength-1 || offset < 0 {
		log.Error("getUuid failed: Offset is invalid.")
		return nil, errors.New("Offset is outside the byte array.")
	}

	leastSignificantLong, err := getLong(log, byteArray, offset)
	if err != nil {
		log.Error("getUuid failed: failed to get uuid LSBs Long value.")
		return nil, errors.New("Failed to get uuid LSBs long value.")
	}

	leastSignificantBytes, err := longToBytes(log, leastSignificantLong)
	if err != nil {
		log.Error("getUuid failed: failed to get uuid LSBs bytes value.")
		return nil, errors.New("Failed to get uuid LSBs bytes value.")
	}

	mostSignificantLong, err := getLong(log, byteArray, offset+8)
	if err != nil {
		log.Error("getUuid failed: failed to get uuid MSBs Long value.")
		return nil, errors.New("Failed to get uuid MSBs long value.")
	}

	mostSignificantBytes, err := longToBytes(log, mostSignificantLong)
	if err != nil {
		log.Error("getUuid failed: failed to get uuid MSBs bytes value.")
		return nil, errors.New("Failed to get uuid MSBs bytes value.")
	}

	uuidBytes := append(mostSignificantBytes, leastSignificantBytes...)

	return uuid.New(uuidBytes), nil
}

// putUuid puts the 128 bit uuid to an array of bytes starting from the offset.
func putUuid(log logger.T, byteArray []byte, offset int, input uuid.UUID) (err error) {
	if input == nil {
		log.Error("putUuid failed: input is null.")
		return errors.New("putUuid failed: input is null.")
	}

	byteArrayLength := len(byteArray)
	if offset > byteArrayLength-1 || offset+16-1 > byteArrayLength-1 || offset < 0 {
		log.Error("putUuid failed: Offset is invalid.")
		return errors.New("Offset is outside the byte array.")
	}

	leastSignificantLong, err := bytesToLong(log, input.Bytes()[8:16])
	if err != nil {
		log.Error("putUuid failed: Failed to get leastSignificant Long value.")
		return errors.New("Failed to get leastSignificant Long value.")
	}

	mostSignificantLong, err := bytesToLong(log, input.Bytes()[0:8])
	if err != nil {
		log.Error("putUuid failed: Failed to get mostSignificantLong Long value.")
		return errors.New("Failed to get mostSignificantLong Long value.")
	}

	if err = putLong(log, byteArray, offset, leastSignificantLong); err != nil {
		log.Error("putUuid failed: Failed to put leastSignificantLong Long value.")
		return errors.New("Failed to put leastSignificantLong Long value.")
	}

	if err = putLong(log, byteArray, offset+8, mostSignificantLong); err != nil {
		log.Error("putUuid failed: Failed to put mostSignificantLong Long value.")
		return errors.New("Failed to put mostSignificantLong Long value.")
	}

	return nil
}

// getBytes gets an array of bytes starting from the offset.
func getBytes(log logger.T, byteArray []byte, offset int, byteLength int) (result []byte, err error) {
	byteArrayLength := len(byteArray)
	if offset > byteArrayLength-1 || offset+byteLength-1 > byteArrayLength-1 || offset < 0 {
		log.Error("getBytes failed: Offset is invalid.")
		return make([]byte, byteLength), errors.New("Offset is outside the byte array.")
	}
	return byteArray[offset : offset+byteLength], nil
}

// putBytes puts bytes into the array at the correct offset.
func putBytes(log logger.T, byteArray []byte, offsetStart int, offsetEnd int, inputBytes []byte) (err error) {
	byteArrayLength := len(byteArray)
	if offsetStart > byteArrayLength-1 || offsetEnd > byteArrayLength-1 || offsetStart > offsetEnd || offsetStart < 0 {
		log.Error("putBytes failed: Offset is invalid.")
		return errors.New("Offset is outside the byte array.")
	}

	if offsetEnd-offsetStart+1 != len(inputBytes) {
		log.Error("putBytes failed: Not enough space to save the bytes.")
		return errors.New("Not enough space to save the bytes.")
	}

	copy(byteArray[offsetStart:offsetEnd+1], inputBytes)
	return nil
}

// getString get a string value from the byte array starting from the specified offset to the defined length.
func getString(log logger.T, byteArray []byte, offset int, stringLength int) (result string, err error) {
	byteArrayLength := len(byteArray)
	if offset > byteArrayLength-1 || offset+stringLength-1 > byteArrayLength-1 || offset < 0 {
		log.Error("getString failed: Offset is invalid.")
		return "", errors.New("Offset is outside the byte array.")
	}

	//remove nulls from the bytes array
	b := bytes.Trim(byteArray[offset:offset+stringLength], "\x00")

	return strings.TrimSpace(string(b)), nil
}

// putString puts a string value to a byte array starting from the specified offset.
func putString(log logger.T, byteArray []byte, offsetStart int, offsetEnd int, inputString string) (err error) {
	byteArrayLength := len(byteArray)
	if offsetStart > byteArrayLength-1 || offsetEnd > byteArrayLength-1 || offsetStart > offsetEnd || offsetStart < 0 {
		log.Error("putString failed: Offset is invalid.")
		return errors.New("Offset is outside the byte array.")
	}

	if offsetEnd-offsetStart+1 < len(inputString) {
		log.Error("putString failed: Not enough space to save the string.")
		return errors.New("Not enough space to save the string.")
	}

	// wipe out the array location first and then insert the new value.
	for i := offsetStart; i <= offsetEnd; i++ {
		byteArray[i] = ' '
	}

	copy(byteArray[offsetStart:offsetEnd+1], inputString)
	return nil
}

// getUInteger gets an unsigned integer
func getUInteger(log logger.T, byteArray []byte, offset int) (result uint32, err error) {
	var temp int32
	temp, err = getInteger(log, byteArray, offset)
	return uint32(temp), err
}

// putUInteger puts an unsigned integer
func putUInteger(log logger.T, byteArray []byte, offset int, value uint32) (err error) {
	return putInteger(log, byteArray, offset, int32(value))
}

// getULong gets an unsigned long integer
func getULong(log logger.T, byteArray []byte, offset int) (result uint64, err error) {
	var temp int64
	temp, err = getLong(log, byteArray, offset)
	return uint64(temp), err
}

// putULong puts an unsigned long integer.
func putULong(log logger.T, byteArray []byte, offset int, value uint64) (err error) {
	return putLong(log, byteArray, offset, int64(value))
}

// getLong gets a long integer value from a byte array starting from the specified offset. 64 bit.
func getLong(log logger.T, byteArray []byte, offset int) (result int64, err error) {
	byteArrayLength := len(byteArray)
	if offset > byteArrayLength-1 || offset+8 > byteArrayLength-1 || offset < 0 {
		log.Error("getLong failed: Offset is invalid.")
		return 0, errors.New("Offset is outside the byte array.")
	}
	return bytesToLong(log, byteArray[offset:offset+8])
}

// putLong puts a long integer value to a byte array starting from the specified offset.
func putLong(log logger.T, byteArray []byte, offset int, value int64) (err error) {
	byteArrayLength := len(byteArray)
	if offset > byteArrayLength-1 || offset+8 > byteArrayLength-1 || offset < 0 {
		log.Error("putLong failed: Offset is invalid.")
		return errors.New("Offset is outside the byte array.")
	}

	mbytes, err := longToBytes(log, value)
	if err != nil {
		log.Error("putLong failed: getBytesFromInteger Failed.")
		return err
	}

	copy(byteArray[offset:offset+8], mbytes)
	return nil
}

// getInteger gets an integer value from a byte array starting from the specified offset.
func getInteger(log logger.T, byteArray []byte, offset int) (result int32, err error) {
	byteArrayLength := len(byteArray)
	if offset > byteArrayLength-1 || offset+4 > byteArrayLength-1 || offset < 0 {
		log.Error("getInteger failed: Offset is invalid.")
		return 0, errors.New("Offset is bigger than the byte array.")
	}
	return bytesToInteger(log, byteArray[offset:offset+4])
}

// putInteger puts an integer value to a byte array starting from the specified offset.
func putInteger(log logger.T, byteArray []byte, offset int, value int32) (err error) {
	byteArrayLength := len(byteArray)
	if offset > byteArrayLength-1 || offset+4 > byteArrayLength-1 || offset < 0 {
		log.Error("putInteger failed: Offset is invalid.")
		return errors.New("Offset is outside the byte array.")
	}

	bytes, err := integerToBytes(log, value)
	if err != nil {
		log.Error("putInteger failed: getBytesFromInteger Failed.")
		return err
	}

	copy(byteArray[offset:offset+4], bytes)
	return nil
}

// bytesToLong gets a Long integer from a byte array.
func bytesToLong(log logger.T, input []byte) (result int64, err error) {
	var res int64
	inputLength := len(input)
	if inputLength != 8 {
		log.Error("bytesToLong failed: input array size is not equal to 8.")
		return 0, errors.New("Input array size is not equal to 8.")
	}
	buf := bytes.NewBuffer(input)
	binary.Read(buf, binary.BigEndian, &res)
	return res, nil
}

// longToBytes gets bytes array from a long integer.
func longToBytes(log logger.T, input int64) (result []byte, err error) {
	buf := new(bytes.Buffer)
	binary.Write(buf, binary.BigEndian, input)
	if buf.Len() != 8 {
		log.Error("longToBytes failed: buffer output length is not equal to 8.")
		return make([]byte, 8), errors.New("Input array size is not equal to 8.")
	}

	return buf.Bytes(), nil
}

// integerToBytes gets bytes array from an integer.
func integerToBytes(log logger.T, input int32) (result []byte, err error) {
	buf := new(bytes.Buffer)
	binary.Write(buf, binary.BigEndian, input)
	if buf.Len() != 4 {
		log.Error("integerToBytes failed: buffer output length is not equal to 4.")
		return make([]byte, 4), errors.New("Input array size is not equal to 4.")
	}

	return buf.Bytes(), nil
}

// bytesToInteger gets an integer from a byte array.
func bytesToInteger(log logger.T, input []byte) (result int32, err error) {
	var res int32
	inputLength := len(input)
	if inputLength != 4 {
		log.Error("bytesToInteger failed: input array size is not equal to 4.")
		return 0, errors.New("Input array size is not equal to 4.")
	}
	buf := bytes.NewBuffer(input)
	binary.Read(buf, binary.BigEndian, &res)
	return res, nil
}
