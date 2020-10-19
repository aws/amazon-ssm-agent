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
	"github.com/aws/amazon-ssm-agent/common/message"
	"go.nanomsg.org/mangos/v3"
	"go.nanomsg.org/mangos/v3/protocol/respondent"
	"go.nanomsg.org/mangos/v3/protocol/surveyor"
)

type SocketType string

const (
	Surveyor   SocketType = "surveyor"
	Respondent SocketType = "respondent"
)

type IChannel interface {
	Initialize(socketType SocketType) error
	Send(message *message.Message) error
	Close() error
	Recv() ([]byte, error)
	SetOption(name string, value interface{}) error
	Listen(addr string) error
	Dial(addr string) error
	IsConnect() bool
}

type Channel struct {
	socket mangos.Socket
	log    log.T
}

// NewChannel creates an new instance of Channel
func NewChannel(log log.T) IChannel {
	return &Channel{
		log: log,
	}
}

// Initialize creates underlying socket
func (channel *Channel) Initialize(socketType SocketType) error {
	var err error
	var socket mangos.Socket

	if socketType == Surveyor {
		if socket, err = surveyor.NewSocket(); err != nil {
			return err
		}

		channel.socket = socket
		return nil

	} else if socketType == Respondent {
		if socket, err = respondent.NewSocket(); err != nil {
			return err
		}

		channel.socket = socket
		return nil

	} else {
		return fmt.Errorf("unsupported socket type")
	}
}

// Send puts the message on the outbound send queue.
func (channel *Channel) Send(message *message.Message) error {
	msg, err := json.Marshal(message)
	if err != nil {
		return err
	}
	return channel.socket.Send(msg)
}

func (channel *Channel) Close() error {
	return channel.socket.Close()
}

// Receive receives a complete message.
func (channel *Channel) Recv() ([]byte, error) {
	return channel.socket.Recv()
}

// SetOption is used to set an option for a socket.
func (channel *Channel) SetOption(name string, value interface{}) error {
	return channel.socket.SetOption(name, value)
}

// Listen connects a local endpoint to the Socket.
func (channel *Channel) Listen(addr string) error {
	return channel.socket.Listen(addr)
}

// Dial connects a remote endpoint to the Socket.
func (channel *Channel) Dial(addr string) error {
	return channel.socket.Dial(addr)
}

// IsConnect returns true if channel is ready to use
func (channel *Channel) IsConnect() bool {
	return channel.socket != nil
}
