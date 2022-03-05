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
//
// Package utils implements some common functionalities for channel
package utils

import (
	"os"
	"path"

	"github.com/aws/amazon-ssm-agent/agent/appconfig"
	"github.com/aws/amazon-ssm-agent/agent/platform"
	"github.com/aws/amazon-ssm-agent/common/filewatcherbasedipc"
	"github.com/aws/amazon-ssm-agent/common/message"
)

type SocketType string

const (
	Surveyor   SocketType = "surveyor"
	Respondent SocketType = "respondent"
)

const (
	TestAddress string = message.DefaultIPCPrefix + message.DefaultCoreAgentChannel + "testPipe"

	ErrorListenDial = "invoke listen or dial before this call"
)

// ICommProtocol interface is for implementing communication protocols
type IFileChannelCommProtocol interface {
	Initialize()
	Send(message *message.Message) error
	Close() error
	Recv() ([]byte, error)
	SetOption(name string, value interface{}) error
	Listen(addr string) error
	Dial(addr string) error
	GetCommProtocolInfo() SocketType
}

// getDefaultChannelPath returns channel path
func getDefaultChannelPath(fileAddress string) (string, error) {
	instanceID, err := platform.InstanceID()
	if err != nil {
		return "", err
	}
	return path.Join(appconfig.DefaultDataStorePath, instanceID, filewatcherbasedipc.DefaultFileChannelPath, path.Base(fileAddress)), nil
}

// IsDefaultChannelPresent verifies whether the channel directory is present or not
func IsDefaultChannelPresent() bool {
	ipcPath, fileErr := getDefaultChannelPath(message.GetWorkerHealthChannel)
	if fileErr != nil {
		return false
	}
	if _, err := os.Stat(ipcPath); os.IsNotExist(err) {
		return false
	}
	return true
}
