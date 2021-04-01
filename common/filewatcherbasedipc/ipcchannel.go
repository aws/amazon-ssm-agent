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

// package filewatcherbasedipc is used to establish IPC between master and workers using files.
package filewatcherbasedipc

import (
	"os"
	"path"

	"github.com/aws/amazon-ssm-agent/agent/fileutil"
	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/amazon-ssm-agent/common/channel/utils"
	"github.com/aws/amazon-ssm-agent/common/identity"
)

const (
	ModeMaster     Mode = "master"
	ModeWorker     Mode = "worker"
	ModeSurveyor   Mode = "surveyor"
	ModeRespondent Mode = "respondent"
)

type Mode string

//Channel is defined as a persistent interface for raw json datagram transmission, it is designed to adopt both file ad named pipe
type IPCChannel interface {
	//send a raw json datagram to the channel, return when send is "complete" -- message is dropped to the persistent layer
	Send(string) error
	//receive a dategram, the go channel on the other end is closed when channel is closed
	GetMessage() <-chan string
	//safely release all in memory resources -- drain the sending/receiving/queue and GetMessage() go channel, channel is reusable after close
	Close()
	//destroy the persistent channel transport, channel is no longer reusable after destroy
	Destroy()
	// CleanupOwnModeFiles cleans up it own mode files
	CleanupOwnModeFiles()
	// GetPath returns IPC filepath
	GetPath() string
}

// IsFileWatcherChannelPresent checks whether the file watcher channel is present or not
func IsFileWatcherChannelPresent(identity identity.IAgentIdentity, channelName string) (bool, error) {
	channelPath, err := utils.GetDefaultChannelPath(identity, channelName)
	if err != nil {
		return false, err
	}
	if _, err = os.Stat(channelPath); os.IsNotExist(err) {
		return false, nil
	}
	return true, err
}

//find the folder named as "documentID" under the default root dir
//if not found, create a new filechannel under the default root dir
//return the channel and the found flag
// shouldReadRetry - is this flag is set to true, it will use fileReadWithRetry function to read
func CreateFileWatcherChannel(log log.T, identity identity.IAgentIdentity, mode Mode, filename string, shouldReadRetry bool) (IPCChannel, error, bool) {
	rootChannelDir, err := utils.GetDefaultChannelPath(identity, "")
	if err != nil {
		return nil, err, false
	}
	list, err := fileutil.ReadDir(rootChannelDir)
	if err != nil {
		log.Infof("failed to read the default channel root directory: %v, creating a new Channel", err)
		f, err := NewFileWatcherChannel(log, mode, path.Join(rootChannelDir, filename), shouldReadRetry)
		return f, err, false
	}
	for _, val := range list {
		if val.Name() == filename {
			log.Infof("channel: %v found", filename)
			f, err := NewFileWatcherChannel(log, mode, path.Join(rootChannelDir, filename), shouldReadRetry)
			return f, err, true
		}
	}
	log.Infof("channel: %v not found, creating a new file channel...", filename)
	f, err := NewFileWatcherChannel(log, mode, path.Join(rootChannelDir, filename), shouldReadRetry)
	return f, err, false
}

// RemoveFileWatcherChannel removes the channel folder specific to the command
func RemoveFileWatcherChannel(identity identity.IAgentIdentity, channelName string) error {
	channelPath, err := utils.GetDefaultChannelPath(identity, channelName)

	if err == nil {
		if _, fileStatErr := os.Stat(channelPath); os.IsNotExist(fileStatErr) {
			return nil
		}
		err = os.RemoveAll(channelPath)
	}
	return err
}
