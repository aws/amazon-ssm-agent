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
	"time"

	"github.com/aws/amazon-ssm-agent/agent/jsonutil"
	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/amazon-ssm-agent/common/channel/utils"
	"github.com/aws/amazon-ssm-agent/common/filewatcherbasedipc"
	"github.com/aws/amazon-ssm-agent/common/identity"
	"github.com/aws/amazon-ssm-agent/common/message"
	"go.nanomsg.org/mangos/v3"
)

// GetRespondentInstance returns the surveyor instance
func GetSurveyInstance(log log.T, identity identity.IAgentIdentity) *survey {
	return &survey{
		log:      log,
		identity: identity,
	}
}

// survey implements surveyor actions from surveyor-respondent pattern
type survey struct {
	log                        log.T
	identity                   identity.IAgentIdentity
	fileChannel                filewatcherbasedipc.IPCChannel
	optionSurveyTimeoutSeconds time.Duration
	socketType                 utils.SocketType
	recvTimer                  *time.Timer
	address                    string
}

// Initialize initializes survey properties
func (sur *survey) Initialize() {
	sur.optionSurveyTimeoutSeconds = 2 * time.Second
	sur.socketType = utils.Surveyor
}

// GetCommProtocolInfo returns communication protocol info
func (sur *survey) GetCommProtocolInfo() utils.SocketType {
	return sur.socketType
}

// Send sends message through file channel created
func (sur *survey) Send(message *message.Message) error {
	if sur.fileChannel == nil {
		return errors.New(utils.ErrorListenDial)
	}
	msg, err := jsonutil.Marshal(message)
	if err != nil {
		return err
	}
	sur.recvTimer = time.NewTimer(sur.optionSurveyTimeoutSeconds)
	return sur.fileChannel.Send(msg)
}

// Close closes and removes the file channel created which includes directory removal
// and file watcher close
func (sur *survey) Close() error {
	if sur.fileChannel == nil {
		return errors.New(utils.ErrorListenDial)
	}
	sur.fileChannel.Destroy()
	return nil
}

// Recv gets the message from the IPC file channel created
// message is returned whenever the worker creates a new file
func (sur *survey) Recv() ([]byte, error) {
	if sur.fileChannel == nil {
		return nil, errors.New(utils.ErrorListenDial)
	}
	select {
	case <-sur.recvTimer.C:
		sur.recvTimer.Stop()
		sur.fileChannel.CleanupOwnModeFiles()
		time.Sleep(500 * time.Millisecond) // better to wait for the files to be cleaned up
		return nil, errors.New("survey timed out")
	case message, isOpen := <-sur.fileChannel.GetMessage():
		if !isOpen {
			sur.Close()
			return nil, errors.New("file channel closed")
		}
		return []byte(message), nil
	}

}

// SetOption is used to specify additional options
func (sur *survey) SetOption(name string, value interface{}) (err error) {
	switch name {
	case mangos.OptionSurveyTime:
		var ok bool
		sur.optionSurveyTimeoutSeconds, ok = value.(time.Duration)
		if !ok {
			return fmt.Errorf("invalid option value")
		}
	default:
		return fmt.Errorf("invalid option")
	}
	return nil
}

// Listen creates a new channel in the main worker side
func (sur *survey) Listen(address string) error {
	sur.address = address
	channel, err, _ := filewatcherbasedipc.CreateFileWatcherChannel(sur.log, sur.identity, filewatcherbasedipc.ModeSurveyor, filepath.Base(address), false)
	sur.fileChannel = channel
	return err
}

// Dial creates a new channel in the worker side
func (sur *survey) Dial(path string) error {
	return nil
}
