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

// package channel captures IPC implementation.
package channel

import (
	"runtime"

	"github.com/aws/amazon-ssm-agent/agent/appconfig"
	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/amazon-ssm-agent/common/channel/utils"
	"github.com/aws/amazon-ssm-agent/common/identity"
	"github.com/aws/amazon-ssm-agent/common/message"
)

// IChannel interface for implementing various channels
type IChannel interface {
	Initialize(socketType utils.SocketType) error
	Send(message *message.Message) error
	Close() error
	Recv() ([]byte, error)
	SetOption(name string, value interface{}) error
	Listen(addr string) error
	Dial(addr string) error
	IsConnect() bool
}

// GetChannelCreator returns function reference for channel creation based
// on whether named pipe be created or not
func GetChannelCreator(log log.T, appConfig appconfig.SsmagentConfig, identity identity.IAgentIdentity) (channelCreateFn func(log.T, identity.IAgentIdentity) IChannel) {
	if canUseNamedPipe(log, appConfig, identity) {
		return NewNamedPipeChannel
	}
	return NewFileChannel
}

// canUseNamedPipe checks whether named pipe can be used for IPC or not
func canUseNamedPipe(log log.T, appConfig appconfig.SsmagentConfig, identity identity.IAgentIdentity) (useNamedPipe bool) {
	// named pipes '.Listen' halts randomly on windows 2012, disabling named pipes on windows and using file channel instead
	if runtime.GOOS == "windows" {
		log.Info("Not using named pipe on windows")
		return false
	}

	if appConfig.Agent.ForceFileIPC {
		return false
	}
	namedPipeChannel := NewNamedPipeChannel(log, identity)
	defer func() {
		if msg := recover(); msg != nil {
			log.Error("named pipe check panicked")
		}
	}()
	defer func() {
		err := namedPipeChannel.Close()
		if err != nil {
			log.Errorf("error while closing named pipe channel %v", err)
		}
	}()
	namedPipeChannel.Initialize(utils.Surveyor)
	if err := namedPipeChannel.Listen(utils.TestAddress); err == nil && !utils.IsDefaultChannelPresent(identity) {
		return true
	}
	return false
}
