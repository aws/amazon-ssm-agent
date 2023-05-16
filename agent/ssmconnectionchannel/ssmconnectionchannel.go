// Copyright 2016 Amazon.com, Inc. or its affiliates. All Rights Reserved.
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

// Package ssmconnectionchannel contains logic for tracking the Agent's primary upstream connection channel.
package ssmconnectionchannel

import (
	"sync"
	"sync/atomic"

	"github.com/aws/amazon-ssm-agent/agent/contracts"
)

var (
	connectionChannel      = contracts.ConnectionChannel{}
	connectionChannelMutex = sync.Mutex{}
)

func SetConnectionChannel(ableToOpenMGSConnection *uint32) {
	connectionChannelMutex.Lock()
	defer connectionChannelMutex.Unlock()
	if ableToOpenMGSConnection != nil && atomic.LoadUint32(ableToOpenMGSConnection) == 1 {
		connectionChannel.SSMConnectionChannel = contracts.MGS
	} else {
		connectionChannel.SSMConnectionChannel = contracts.MDS
	}
}

func GetConnectionChannel() contracts.SSMConnectionChannel {
	connectionChannelMutex.Lock()
	defer connectionChannelMutex.Unlock()
	return connectionChannel.SSMConnectionChannel
}
