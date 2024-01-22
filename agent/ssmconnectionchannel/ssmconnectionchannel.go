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

// Package ssmconnectionchannel contains logic for tracking the Agent's primary upstream connection channel and its various states.
package ssmconnectionchannel

import (
	"sync"

	"github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/aws/amazon-ssm-agent/agent/contracts"
)

// MGSState shows the current MGS connection state
type MGSState string

const (
	// MGSSuccess denotes that the current MGS connection state is successful
	MGSSuccess MGSState = "MGSSuccess"

	// MGSFailed denotes that the current MGS connection state has failed
	MGSFailed MGSState = "MGSFailed"

	// MGSFailedDueToAccessDenied denotes that the current MGS connection state threw access denied
	MGSFailedDueToAccessDenied MGSState = "MGSFailedDueToAccessDenied"
)

var (
	connectionChannel      = contracts.ConnectionChannel{}
	connectionChannelMutex = sync.RWMutex{}

	// mdsSwitchChannel is used by MDSInteractor to switch ON/OFF MDS
	mdsSwitchChannel = make(chan bool)
)

// SetConnectionChannel sets the Upstream SSM connection channel(MDS or MGS)
func SetConnectionChannel(context context.T, state MGSState) {
	connectionChannelMutex.Lock()
	defer func() {
		log := context.Log()
		log.Infof("SSM Connection channel status is set to %v", connectionChannel.SSMConnectionChannel)
		connectionChannelMutex.Unlock()
	}()

	// MDS is never turned off for container mode.
	// Hence, the SSMConnectionChannel status is always set to MGS
	if context.AppConfig().Agent.ContainerMode {
		connectionChannel.SSMConnectionChannel = contracts.MGS
		return
	}

	// case for MGS is successfully established
	if state == MGSSuccess {
		// If SSMConnectionChannel status is MGS, it means that the MDS shutdown was retried before.
		// Hence, when we received MGS Success again, return from this function without trying to shut down MDS
		if connectionChannel.SSMConnectionChannel == contracts.MGS {
			return
		}

		// Shutdown MDS when MGS connection is successful
		connectionChannel.SSMConnectionChannel = contracts.MGS
		mdsSwitchChannel <- false
		return
	}

	// case for MGS failed due ot AccessDenied
	if state == MGSFailedDueToAccessDenied {
		// If SSMConnectionChannel status is MDS, it means that the MDS is still running.
		// Hence, when we receive MGS Access Denied error, we return from this function without trying to launch MDS as it is already running
		if connectionChannel.SSMConnectionChannel == contracts.MDS || connectionChannel.SSMConnectionChannel == "" {
			// MDS will be set when the initial value is blank
			connectionChannel.SSMConnectionChannel = contracts.MDS
			return
		}
		// Turn ON MDS when MGS connection fails with AccessDenied
		connectionChannel.SSMConnectionChannel = contracts.MDS
		mdsSwitchChannel <- true
		return
	}

	// reset to MDS when ssm connection channel is blank
	// In this case, MDS is already started but MGS is failing
	if connectionChannel.SSMConnectionChannel == "" {
		connectionChannel.SSMConnectionChannel = contracts.MDS
	}

	// No operation for all other MGS states
}

// GetConnectionChannel returns the SSM Connection channel(MDS or MGS)
func GetConnectionChannel() contracts.SSMConnectionChannel {
	connectionChannelMutex.RLock()
	defer connectionChannelMutex.RUnlock()
	return connectionChannel.SSMConnectionChannel
}

// GetMDSSwitchChannel returns the golang channel containing values to switch ON or OFF MDS
func GetMDSSwitchChannel() chan bool {
	return mdsSwitchChannel
}

// CloseMDSSwitchChannel close MDS switch golang channel
func CloseMDSSwitchChannel() {
	close(mdsSwitchChannel)
}
