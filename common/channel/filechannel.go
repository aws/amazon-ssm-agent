// Copyright 2020 Amazon.com, Inc. or its affiliates. All Rights Reserved.
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

// package channel captures file IPC implementation.
package channel

import (
	"fmt"

	"github.com/aws/amazon-ssm-agent/agent/log"
	commProtocol "github.com/aws/amazon-ssm-agent/common/channel/protocol"
	"github.com/aws/amazon-ssm-agent/common/channel/utils"
	"github.com/aws/amazon-ssm-agent/common/identity"
	"github.com/aws/amazon-ssm-agent/common/message"
)

var (
	getSurveyInstance     = commProtocol.GetSurveyInstance
	getRespondentInstance = commProtocol.GetRespondentInstance
)

// NewFileChannel creates an new instance of FileChannel which internally uses file watcher based ipc channel
// This channel does not have multi-threading support. Currently, the invocations happen only in one go-routine
func NewFileChannel(log log.T, identity identity.IAgentIdentity) IChannel {
	return &fileChannel{
		log:      log,
		identity: identity,
	}
}

// FileChannel is responsible for using file watcher logic
// to implement various communication protocol
type fileChannel struct {
	log                      log.T
	identity                 identity.IAgentIdentity
	isFileChannelInitialized bool
	isDialSuccessful         bool
	isListenSuccessful       bool
	msgProtocol              utils.IFileChannelCommProtocol
}

// Initialize initializes survey properties
func (fc *fileChannel) Initialize(socketType utils.SocketType) error {
	fc.log.Info("using file channel for IPC")
	if socketType == utils.Surveyor {
		fc.msgProtocol = getSurveyInstance(fc.log, fc.identity)
	} else if socketType == utils.Respondent {
		fc.msgProtocol = getRespondentInstance(fc.log, fc.identity)
	} else {
		return fmt.Errorf("unsupported socket type")
	}
	fc.isFileChannelInitialized = true
	fc.isDialSuccessful = false
	fc.isListenSuccessful = false
	return nil
}

// Send puts the message on the outbound send queue.
func (fc *fileChannel) Send(message *message.Message) error {
	if !fc.IsChannelInitialized() {
		return ErrIPCChannelClosed
	}
	if !(fc.IsListenSuccessful() || fc.IsDialSuccessful()) {
		return ErrDialListenUnSuccessful
	}
	return fc.msgProtocol.Send(message)
}

// Close closes and removes the file channel created which includes directory removal
// and file watcher close
func (fc *fileChannel) Close() error {
	return fc.msgProtocol.Close()
}

// Recv gets the message from the IPC file channel created
// message is returned whenever the worker creates a new file
func (fc *fileChannel) Recv() ([]byte, error) {
	if !fc.IsChannelInitialized() {
		return nil, ErrIPCChannelClosed
	}
	if !(fc.IsListenSuccessful() || fc.IsDialSuccessful()) {
		return nil, ErrDialListenUnSuccessful
	}
	return fc.msgProtocol.Recv()
}

// SetOption is used to set an option.
func (fc *fileChannel) SetOption(name string, value interface{}) (err error) {
	return fc.msgProtocol.SetOption(name, value)
}

// Listen creates a new channel in the main worker side
func (fc *fileChannel) Listen(addr string) error {
	err := fc.msgProtocol.Listen(addr)
	if err != nil {
		return err
	}
	fc.isListenSuccessful = true
	return nil
}

// Dial creates a new channel in the worker side
func (fc *fileChannel) Dial(addr string) error {
	err := fc.msgProtocol.Dial(addr)
	if err != nil {
		return err
	}
	fc.isDialSuccessful = true
	return nil
}

// IsChannelInitialized returns true if channel initialization is successful.
func (fc *fileChannel) IsChannelInitialized() bool {
	return fc.isFileChannelInitialized
}

// IsDialSuccessful returns true if Dial() successfully connects to IPC channels.
// In Dial(), we connect to the IPC with address being the parameter.
func (fc *fileChannel) IsDialSuccessful() bool {
	return fc.isDialSuccessful
}

// IsListenSuccessful returns true if Listen() successfully creates IPC channels.
// In Listen(), we will create named pipes on Windows and sockets on Linux/Darwin for IPC.
func (fc *fileChannel) IsListenSuccessful() bool {
	return fc.isListenSuccessful
}
