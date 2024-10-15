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
	"testing"
	"time"

	"github.com/aws/amazon-ssm-agent/agent/appconfig"
	logmocks "github.com/aws/amazon-ssm-agent/agent/mocks/log"

	"github.com/aws/amazon-ssm-agent/agent/contracts"
	contextmocks "github.com/aws/amazon-ssm-agent/agent/mocks/context"
	"github.com/stretchr/testify/assert"
)

func TestSetConnectionChannel_MGSSuccess_MDSSwitchOff(t *testing.T) {
	contextMock := contextmocks.NewMockDefault()
	resetConnectionChannel()
	go func() {
		SetConnectionChannel(contextMock, MGSSuccess)
	}()
	mdsSwitchCh := <-GetMDSSwitchChannel()
	assert.Equal(t, mdsSwitchCh, false)
	messagingService := GetConnectionChannel()
	assert.Equal(t, string(messagingService), string(contracts.MGS))
}

func TestSetConnectionChannel_MGSSuccess_MDSAlreadyStopped(t *testing.T) {
	contextMock := contextmocks.NewMockDefault()
	connectionChannel.SSMConnectionChannel = contracts.MGS
	go func() {
		SetConnectionChannel(contextMock, MGSSuccess)
	}()
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
	connectionChannel.SSMConnectionChannel = contracts.MGS
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
	connectionChannel.SSMConnectionChannel = contracts.MGS
	go func() {
		SetConnectionChannel(contextMock, MGSFailedDueToAccessDenied)
	}()
	mdsSwitchCh := <-GetMDSSwitchChannel()
	assert.Equal(t, mdsSwitchCh, true)
	messagingService := GetConnectionChannel()
	assert.Equal(t, string(messagingService), string(contracts.MDS))
}

func TestSetConnectionChannel_ContainerMode(t *testing.T) {
	appConfig := appconfig.DefaultConfig()
	appConfig.Agent.ContainerMode = true

	contextMock := new(contextmocks.Mock)
	contextMock.On("Log").Return(logmocks.NewMockLog())
	contextMock.On("AppConfig").Return(appConfig)

	connectionChannel.SSMConnectionChannel = contracts.MGS
	SetConnectionChannel(contextMock, MGSFailedDueToAccessDenied)
	assert.Equal(t, len(mdsSwitchChannel), 0)
	messagingService := GetConnectionChannel()
	assert.Equal(t, string(messagingService), string(contracts.MGS))
}

func TestGetConnectionChannelReturnsEmptyStringIfConnectionHasNotBeenSet(t *testing.T) {
	connectionChannel.SSMConnectionChannel = ""
	messagingService := GetConnectionChannel()
	assert.Equal(t, string(messagingService), "")
}

func resetConnectionChannel() {
	go func() {
		contextMock := contextmocks.NewMockDefault()
		SetConnectionChannel(contextMock, MGSFailedDueToAccessDenied)
	}()
	go func() {
		select {
		case <-time.After(500 * time.Millisecond):
			break
		case <-GetMDSSwitchChannel():
			break
		}
	}()
	time.Sleep(500 * time.Millisecond)
}
