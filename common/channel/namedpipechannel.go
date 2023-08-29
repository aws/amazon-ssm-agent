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

// package channel captures mango socket implementation.
package channel

import (
	"encoding/json"
	"fmt"

	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/amazon-ssm-agent/common/channel/utils"
	"github.com/aws/amazon-ssm-agent/common/identity"
	"github.com/aws/amazon-ssm-agent/common/message"
	"go.nanomsg.org/mangos/v3"
	"go.nanomsg.org/mangos/v3/protocol/respondent"
	"go.nanomsg.org/mangos/v3/protocol/surveyor"
)

type namedPipeChannel struct {
	socket             mangos.Socket
	log                log.T
	isDialSuccessful   bool
	isListenSuccessful bool
}

var (
	getSurveyorSocket   = surveyor.NewSocket
	getRespondentSocket = respondent.NewSocket
)

// NewNamedPipeChannel creates a new instance of named pipe channel
// This channel does not have multi-threading support. Currently, the invocations happen only in one go-routine
func NewNamedPipeChannel(log log.T, identity identity.IAgentIdentity) IChannel {
	return &namedPipeChannel{
		log: log,
	}
}

// Initialize creates underlying socket
func (channel *namedPipeChannel) Initialize(socketType utils.SocketType) error {
	var err error
	var socket mangos.Socket
	channel.log.Info("using named pipe channel for IPC")
	if socketType == utils.Surveyor {
		if socket, err = getSurveyorSocket(); err != nil {
			return err
		}
		channel.socket = socket
		return nil

	} else if socketType == utils.Respondent {
		if socket, err = getRespondentSocket(); err != nil {
			return err
		}
		channel.socket = socket
		return nil

	} else {
		return fmt.Errorf("unsupported socket type")
	}
}

// Send puts the message on the outbound send queue.
func (channel *namedPipeChannel) Send(message *message.Message) error {
	msg, err := json.Marshal(message)
	if err != nil {
		return err
	}
	if !channel.IsChannelInitialized() {
		return ErrIPCChannelClosed
	}
	if !(channel.IsListenSuccessful() || channel.IsDialSuccessful()) {
		return ErrDialListenUnSuccessful
	}
	return channel.socket.Send(msg)
}

func (channel *namedPipeChannel) Close() error {
	defer func() {
		channel.socket = nil
	}()
	if !channel.IsChannelInitialized() {
		return ErrIPCChannelClosed
	}
	return channel.socket.Close()
}

// Recv receives a complete message.
func (channel *namedPipeChannel) Recv() ([]byte, error) {
	if !channel.IsChannelInitialized() {
		return nil, ErrIPCChannelClosed
	}
	if !(channel.IsListenSuccessful() || channel.IsDialSuccessful()) {
		return nil, ErrDialListenUnSuccessful
	}
	return channel.socket.Recv()
}

// SetOption is used to specify additional options
func (channel *namedPipeChannel) SetOption(name string, value interface{}) error {
	if !channel.IsChannelInitialized() {
		return ErrIPCChannelClosed
	}
	return channel.socket.SetOption(name, value)
}

// Listen connects a local endpoint to the Socket.
func (channel *namedPipeChannel) Listen(addr string) error {
	if !channel.IsChannelInitialized() {
		return ErrIPCChannelClosed
	}
	err := channel.socket.Listen(addr)
	if err != nil {
		return err
	}

	channel.isListenSuccessful = true
	return nil
}

// Dial connects a remote endpoint to the Socket.
func (channel *namedPipeChannel) Dial(addr string) error {
	if !channel.IsChannelInitialized() {
		return ErrIPCChannelClosed
	}

	err := channel.socket.Dial(addr)
	if err != nil {
		return err
	}
	channel.isDialSuccessful = true
	return nil
}

// IsChannelInitialized returns true if channel initialization is successful.
func (channel *namedPipeChannel) IsChannelInitialized() bool {
	return channel.socket != nil
}

// IsDialSuccessful returns true if Dial() successfully connects to IPC channels.
// In Dial(), we connect to the IPC with address being the parameter
func (channel *namedPipeChannel) IsDialSuccessful() bool {
	return channel.isDialSuccessful
}

// IsListenSuccessful returns true if Listen() successfully creates IPC channels.
// In Listen(), we will create named pipes on Windows and sockets on Linux/Darwin for IPC.
func (channel *namedPipeChannel) IsListenSuccessful() bool {
	return channel.isListenSuccessful
}
