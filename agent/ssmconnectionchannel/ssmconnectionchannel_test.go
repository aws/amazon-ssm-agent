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
	"testing"

	"github.com/aws/amazon-ssm-agent/agent/appconfig"
	logmocks "github.com/aws/amazon-ssm-agent/agent/mocks/log"

	"github.com/aws/amazon-ssm-agent/agent/contracts"
	contextmocks "github.com/aws/amazon-ssm-agent/agent/mocks/context"
	"github.com/stretchr/testify/assert"
)

func TestSetConnectionChannel_MGSSuccess_MDSSwitchOff(t *testing.T) {
	contextMock := contextmocks.NewMockDefault()
	resetConnectionChannel()

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		SetConnectionChannel(contextMock, MGSSuccess)
	}()

	mdsSwitchCh := <-GetMDSSwitchChannel()
	assert.Equal(t, mdsSwitchCh, false)

	wg.Wait()
	messagingService := GetConnectionChannel()
	assert.Equal(t, string(messagingService), string(contracts.MGS))
}

func TestSetConnectionChannel_MGSSuccess_MDSAlreadyStopped(t *testing.T) {
	contextMock := contextmocks.NewMockDefault()
	resetConnectionChannel()

	setConnectionChannelForTest(contracts.MGS)

	SetConnectionChannel(contextMock, MGSSuccess)

	assert.Equal(t, len(mdsSwitchChannel), 0)
	messagingService := GetConnectionChannel()
	assert.Equal(t, string(messagingService), string(contracts.MGS))
}

func TestSetConnectionChannel_MGSFailed_MDSAlreadyStarted(t *testing.T) {
	contextMock := contextmocks.NewMockDefault()
	resetConnectionChannel()

	SetConnectionChannel(contextMock, MGSFailed)

	assert.Equal(t, len(mdsSwitchChannel), 0)
	messagingService := GetConnectionChannel()
	assert.Equal(t, string(messagingService), string(contracts.MDS))
}

func TestSetConnectionChannel_MGSFailed_MDSNotRunning(t *testing.T) {
	contextMock := contextmocks.NewMockDefault()
	resetConnectionChannel()

	setConnectionChannelForTest(contracts.MGS)

	SetConnectionChannel(contextMock, MGSFailed)

	assert.Equal(t, len(mdsSwitchChannel), 0)
	messagingService := GetConnectionChannel()
	assert.Equal(t, string(messagingService), string(contracts.MGS))
}

func TestSetConnectionChannel_MGSAccessDenied_MDSAlreadySwitchON(t *testing.T) {
	contextMock := contextmocks.NewMockDefault()
	resetConnectionChannel()

	SetConnectionChannel(contextMock, MGSFailedDueToAccessDenied)

	assert.Equal(t, len(mdsSwitchChannel), 0)
	messagingService := GetConnectionChannel()
	assert.Equal(t, string(messagingService), string(contracts.MDS))
}

func TestSetConnectionChannel_MGSAccessDenied_MDSNotSwitchON(t *testing.T) {
	contextMock := contextmocks.NewMockDefault()
	resetConnectionChannel()

	// Pre-set to MGS so the AccessDenied path sends a switch-ON signal.
	setConnectionChannelForTest(contracts.MGS)

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		SetConnectionChannel(contextMock, MGSFailedDueToAccessDenied)
	}()

	// The channel receive synchronizes with the send inside SetConnectionChannel.
	mdsSwitchCh := <-GetMDSSwitchChannel()
	assert.Equal(t, mdsSwitchCh, true)

	// Wait for the goroutine to fully finish before asserting shared state.
	wg.Wait()
	messagingService := GetConnectionChannel()
	assert.Equal(t, string(messagingService), string(contracts.MDS))
}

func TestSetConnectionChannel_ContainerMode(t *testing.T) {
	appConfig := appconfig.DefaultConfig()
	appConfig.Agent.ContainerMode = true

	contextMock := new(contextmocks.Mock)
	contextMock.On("Log").Return(logmocks.NewMockLog())
	contextMock.On("AppConfig").Return(appConfig)

	resetConnectionChannel()

	// Pre-set to MGS; container mode always keeps MGS regardless of state.
	setConnectionChannelForTest(contracts.MGS)

	SetConnectionChannel(contextMock, MGSFailedDueToAccessDenied)

	assert.Equal(t, len(mdsSwitchChannel), 0)
	messagingService := GetConnectionChannel()
	assert.Equal(t, string(messagingService), string(contracts.MGS))
}

func TestGetConnectionChannelReturnsEmptyStringIfConnectionHasNotBeenSet(t *testing.T) {
	resetConnectionChannel()
	messagingService := GetConnectionChannel()
	assert.Equal(t, string(messagingService), "")
}

func resetConnectionChannel() {
	connectionChannel.SSMConnectionChannel = ""
	// Drain any pending signal from the channel
	select {
	case <-mdsSwitchChannel:
	default:
	}
}

// setConnectionChannelForTest directly sets the current connection channel,
// bypassing SetConnectionChannel's state machine so tests can establish a
// known starting state.
func setConnectionChannelForTest(channel contracts.SSMConnectionChannel) {
	connectionChannel.SSMConnectionChannel = channel
}
