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
	"fmt"
	"time"

	"github.com/aws/amazon-ssm-agent/agent/contracts"
	logger "github.com/aws/amazon-ssm-agent/agent/log"
)

const (
	// InteractiveShellMessage message type for interactive shell.
	InteractiveShellMessage string = "interactive_shell"
	// TaskReplyMessage represents message type for task reply
	TaskReplyMessage string = "agent_task_reply"
	// TaskCompleteMessage represents message type for task complete
	TaskCompleteMessage string = "agent_task_complete"
	// AcknowledgeMessage represents message type for acknowledge
	AcknowledgeMessage string = "acknowledge"
	// AgentSessionState represents status of session
	AgentSessionState string = "agent_session_state"
	// ChannelClosedMessage represents message type for ChannelClosed
	ChannelClosedMessage string = "channel_closed"
	// OutputStreamDataMessage represents message type for outgoing stream data
	OutputStreamDataMessage string = "output_stream_data"
	// InputStreamDataMessage represents message type for incoming stream data
	InputStreamDataMessage string = "input_stream_data"
	// PausePublicationMessage message type for pause sending data packages.
	PausePublicationMessage string = "pause_publication"
	// StartPublicationMessage message type for start sending data packages.
	StartPublicationMessage string = "start_publication"
)

type ShellProperties struct {
	Windows ShellConfig `json:"windows" yaml:"windows"`
	Linux   ShellConfig `json:"linux" yaml:"linux"`
}

type ShellConfig struct {
	Commands      string `json:"commands" yaml:"commands"`
	RunAsElevated bool   `json:"runAsElevated" yaml:"runAsElevated"`
}

type IMessage interface {
	Deserialize(log logger.T, agentMessage AgentMessage) (err error)
	Serialize(log logger.T) (result []byte, err error)
}

// PayloadMessageBase represent the base struct for all messages that include a payload.
// * HeaderLength is a 4 byte integer that represents the header length.
// * Payload digest is a 32 byte containing the SHA-256 hash of the payload.
// * Payload length is an 8 byte unsigned integer containing the byte length of data in the Payload field.
// * Payload is a variable length byte data.
type PayloadMessageBase struct {
	HeaderLength  uint32
	PayloadDigest []byte
	PayloadLength uint32
	Payload       []byte
}

// Messages for Control Channel

// TaskMessageBase represents basic structure for task messages.
// * MessageType is a 32 byte UTF-8 string containing the message type.
// * SchemaVersion is a 4 byte integer containing the message schema version number.
// * CreatedDate is an 8 byte integer containing the message create epoch millis in UTC.
// * MessageId is a 40 byte UTF-8 string containing a random UUID identifying this message.
type TaskMessageBase struct {
	MessageType   string
	SchemaVersion uint32
	CreatedDate   uint64
	MessageId     string
	TaskId        string
	Topic         string
}

// MGSPayload parallels the structure of a start-session MGS message payload.
type MGSPayload struct {
	Payload       string `json:"Content"`
	TaskId        string `json:"TaskId"`
	Topic         string `json:"Topic"`
	SchemaVersion int    `json:"SchemaVersion"`
}

// AgentTaskPayload parallels the structure of a send command MGS message payload.
type AgentTaskPayload struct {
	DocumentName    string                           `json:"DocumentName"`
	DocumentContent contracts.SessionDocumentContent `json:"DocumentContent"`
	SessionId       string                           `json:"SessionId"`
	Parameters      map[string]interface{}           `json:"Parameters"`
	RunAsUser       string                           `json:"RunAsUser"`
}

// AcknowledgeContent is used to inform the sender of an acknowledge message that the message has been received.
// * MessageType is a 32 byte UTF-8 string containing the message type.
// * MessageId is a 40 byte UTF-8 string containing the UUID identifying this message being acknowledged.
// * SequenceNumber is an 8 byte integer containing the message sequence number for serialized message.
// * IsSequentialMessage is a boolean field representing whether the acknowledged message is part of a sequence
type AcknowledgeContent struct {
	MessageType         string `json:"AcknowledgedMessageType"`
	MessageId           string `json:"AcknowledgedMessageId"`
	SequenceNumber      int64  `json:"AcknowledgedMessageSequenceNumber"`
	IsSequentialMessage bool   `json:"IsSequentialMessage"`
}

// AgentSessionState is used to inform the sender of agent's session state.
type AgentSessionStateContent struct {
	SchemaVersion int    `json:"SchemaVersion"`
	SessionState  string `json:"SessionState"`
	SessionId     string `json:"SessionId"`
}

// Deserialize parses AcknowledgeContent message from payload of AgentMessage.
func (dataStreamAcknowledge *AcknowledgeContent) Deserialize(log logger.T, agentMessage AgentMessage) (err error) {
	if agentMessage.MessageType != AcknowledgeMessage {
		err = fmt.Errorf("AgentMessage is not of type AcknowledgeMessage. Found message type: %s", agentMessage.MessageType)
		return
	}

	if err = json.Unmarshal(agentMessage.Payload, dataStreamAcknowledge); err != nil {
		log.Errorf("Could not deserialize rawMessage to AcknowledgeMessage: %s", err)
	}
	return
}

// Serialize marshals AcknowledgeContent as payloads into bytes.
func (dataStreamAcknowledge *AcknowledgeContent) Serialize(log logger.T) (result []byte, err error) {
	result, err = json.Marshal(dataStreamAcknowledge)
	if err != nil {
		log.Errorf("Could not serialize AcknowledgeContent message: %v, err: %s", dataStreamAcknowledge, err)
	}
	return
}

// ChannelClosed is used to inform the agent of a channel to be closed.
// * MessageType is a 32 byte UTF-8 string containing the message type.
// * MessageId is a 40 byte UTF-8 string containing the UUID identifying this message.
// * DestinationId is a string field containing the session target.
// * SessionId is a string field representing which session to close.
// * SchemaVersion is a 4 byte integer containing the message schema version number.
// * CreatedDate is a string field containing the message create epoch millis in UTC.
type ChannelClosed struct {
	MessageType   string `json:"MessageType"`
	MessageId     string `json:"MessageId"`
	DestinationId string `json:"DestinationId"`
	SessionId     string `json:"SessionId"`
	SchemaVersion int    `json:"SchemaVersion"`
	CreatedDate   string `json:"CreatedDate"`
}

// Deserialize parses channelClosed message from payload of AgentMessage.
func (channelClose *ChannelClosed) Deserialize(log logger.T, agentMessage AgentMessage) (err error) {
	if agentMessage.MessageType != ChannelClosedMessage {
		err = fmt.Errorf("AgentMessage is not of type ChannelClosed. Found message type: %s", agentMessage.MessageType)
		return
	}

	if err = json.Unmarshal(agentMessage.Payload, channelClose); err != nil {
		log.Errorf("Could not deserialize rawMessage to ChannelClosed: %s", err)
	}
	return
}

// Serialize marshals ChannelClosed as payloads into bytes.
func (channelClose *ChannelClosed) Serialize(log logger.T) (result []byte, err error) {
	result, err = json.Marshal(channelClose)
	if err != nil {
		log.Errorf("Could not serialize ChannelClosed message: %v, err: %s", channelClose, err)
	}
	return
}

// AgentTaskCompletePayload is sent by the agent to inform the task is complete and what the overall result was.
type AgentTaskCompletePayload struct {
	SchemaVersion    int    `json:"SchemaVersion"`
	TaskId           string `json:"TaskId"`
	Topic            string `json:"Topic"`
	FinalTaskStatus  string `json:"FinalTaskStatus"`
	IsRoutingFailure bool   `json:"IsRoutingFailure"`
	AwsAccountId     string `json:"AwsAccountId"`
	InstanceId       string `json:"InstanceId"`
	Output           string `json:"Output"`
	S3Bucket         string `json:"S3Bucket"`
	S3UrlSuffix      string `json:"S3UrlSuffix"`
	CwlGroup         string `json:"CwlGroup"`
	CwlStream        string `json:"CwlStream"`
}

// SessionPluginResultOutput represents PluginResult output sent to MGS as part of AgentTaskComplete message
type SessionPluginResultOutput struct {
	Output      string
	S3Bucket    string
	S3UrlSuffix string
	CwlGroup    string
	CwlStream   string
}

type PayloadType uint32

const (
	Output               PayloadType = 1
	Error                PayloadType = 2
	Size                 PayloadType = 3
	Parameter            PayloadType = 4
	HandshakeRequest     PayloadType = 5
	HandshakeResponse    PayloadType = 6
	HandshakeComplete    PayloadType = 7
	EncChallengeRequest  PayloadType = 8
	EncChallengeResponse PayloadType = 9
)

type SessionStatus string

const (
	Connected   SessionStatus = "Connected"
	Terminating SessionStatus = "Terminating"
)

type SizeData struct {
	Cols uint32 `json:"cols"`
	Rows uint32 `json:"rows"`
}

// ActionType used in Handshake to determine action requested by the agent
type ActionType string

const (
	// Used to perform KMSEncryption related actions.
	KMSEncryption ActionType = "KMSEncryption"
	// Can be used to perform session type specific actions.
	SessionType ActionType = "SessionType"
)

type ActionStatus int

const (
	Success     ActionStatus = 1
	Failed      ActionStatus = 2
	Unsupported ActionStatus = 3
)

// This is sent by the agent to initialize KMS encryption
type KMSEncryptionRequest struct {
	KMSKeyID string `json:"KMSKeyId"`
}

// This is received by the agent to set up KMS encryption
type KMSEncryptionResponse struct {
	KMSCipherTextKey  []byte `json:"KMSCipherTextKey"`
	KMSCipherTextHash []byte `json:"KMSCipherTextHash"`
}

type SessionTypeRequest struct {
	SessionType string      `json:"SessionType"`
	Properties  interface{} `json:"Properties"`
}

// Handshake payload sent by the agent to the session manager plugin
type HandshakeRequestPayload struct {
	AgentVersion           string                  `json:"AgentVersion"`
	RequestedClientActions []RequestedClientAction `json:"RequestedClientActions"`
}

// An action requested by the agent to the plugin
type RequestedClientAction struct {
	ActionType       ActionType  `json:"ActionType"`
	ActionParameters interface{} `json:"ActionParameters"`
}

// The result of processing the action by the plugin
type ProcessedClientAction struct {
	ActionType   ActionType      `json:"ActionType"`
	ActionStatus ActionStatus    `json:"ActionStatus"`
	ActionResult json.RawMessage `json:"ActionResult"`
	Error        string          `json:"Error"`
}

// Handshake Response sent by the plugin in response to the handshake request
type HandshakeResponsePayload struct {
	ClientVersion          string                  `json:"ClientVersion"`
	ProcessedClientActions []ProcessedClientAction `json:"ProcessedClientActions"`
	Errors                 []string                `json:"Errors"`
}

// This is sent by the agent as a challenge to the client. The challenge field
// is some data that was encrypted by the agent. The client must be able to decrypt
// this and in turn encrypt it with its own key.
type EncryptionChallengeRequest struct {
	Challenge []byte `json:"Challenge"`
}

// This is received by the agent from the client. The challenge field contains
// some data received, decrypted and then encrypted by the client. Agent must
// be able to decrypt this and verify it matches the original plaintext challenge.
type EncryptionChallengeResponse struct {
	Challenge []byte `json:"Challenge"`
}

// Handshake Complete indicates to client that handshake is complete.
// This signals the client to start the plugin and display a customer message where appropriate.
type HandshakeCompletePayload struct {
	HandshakeTimeToComplete time.Duration `json:"HandshakeTimeToComplete"`
	CustomerMessage         string        `json:"CustomerMessage"`
}
