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

// Package protocol implements some common communication protocols using file watcher.
package protocol

import (
	"errors"
	"fmt"
	"path/filepath"

	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/amazon-ssm-agent/common/identity"

	"github.com/aws/amazon-ssm-agent/agent/jsonutil"
	"github.com/aws/amazon-ssm-agent/common/channel/utils"
	"github.com/aws/amazon-ssm-agent/common/filewatcherbasedipc"
	"github.com/aws/amazon-ssm-agent/common/message"
)

// GetRespondentInstance returns the respondent instance
func GetRespondentInstance(log log.T, identity identity.IAgentIdentity) *respondent {
	return &respondent{
		log:      log,
		identity: identity,
	}
}

// respondent implements respondent actions from surveyor-respondent pattern
type respondent struct {
	log         log.T
	identity    identity.IAgentIdentity
	fileChannel filewatcherbasedipc.IPCChannel
	socketType  utils.SocketType
}

// Initialize initializes respondent properties
func (res *respondent) Initialize() {
	res.socketType = utils.Respondent
}

// Send sends message through file channel created
func (res *respondent) Send(message *message.Message) error {
	if res.fileChannel == nil {
		return errors.New(utils.ErrorListenDial)
	}
	msg, err := jsonutil.Marshal(message)
	if err != nil {
		return err
	}
	return res.fileChannel.Send(msg)
}

// Close closes and removes the file channel created which includes directory removal
// and file watcher close
func (res *respondent) Close() error {
	if res.fileChannel == nil {
		return errors.New(utils.ErrorListenDial)
	}
	res.fileChannel.Destroy()
	return nil
}

// GetCommProtocolInfo returns communication protocol info
func (res *respondent) GetCommProtocolInfo() utils.SocketType {
	return res.socketType
}

// Recv returns the message from the IPC file channel created
// message is returned whenever the master creates a new file
func (res *respondent) Recv() ([]byte, error) {
	if res.fileChannel == nil {
		return nil, errors.New(utils.ErrorListenDial)
	}
	select {
	case message, isOpen := <-res.fileChannel.GetMessage():
		if !isOpen {
			return nil, errors.New("respondent: file channel closed")
		}
		return []byte(message), nil
	}
}

// SetOption is used to specify additional options
func (res *respondent) SetOption(name string, value interface{}) (err error) {
	return nil
}

// Listen creates a new channel in the main worker side
func (res *respondent) Listen(path string) error {
	return nil
}

// Dial creates a new channel in the worker side
func (res *respondent) Dial(path string) error {
	channelName := filepath.Base(path)
	// added to make sure that respondent should not create file channel
	if isPresent, err := filewatcherbasedipc.IsFileWatcherChannelPresent(res.identity, channelName); !isPresent {
		return fmt.Errorf("file channel not present to dial : %v", err)
	}
	channel, err, _ := filewatcherbasedipc.CreateFileWatcherChannel(res.log, res.identity, filewatcherbasedipc.ModeRespondent, channelName, true)
	res.fileChannel = channel
	return err
}
