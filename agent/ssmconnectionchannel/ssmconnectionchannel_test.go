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
	"github.com/stretchr/testify/assert"
	"sync/atomic"
	"testing"
)

func TestSetConnectionChannelSetsAndGetsConnectionToMGSIfAbleToOpenMGSConnectionEqualsOne(t *testing.T) {
	var ableToOpenMGSConnection uint32
	atomic.StoreUint32(&ableToOpenMGSConnection, 1)
	SetConnectionChannel(&ableToOpenMGSConnection)
	messagingService := GetConnectionChannel()
	assert.Equal(t, string(messagingService), "ssmmessages")
}

func TestSetConnectionChannelSetsAndGetsConnectionToMDSIfAbleToOpenMGSConnectionEqualsOne(t *testing.T) {
	var ableToOpenMGSConnection uint32
	atomic.StoreUint32(&ableToOpenMGSConnection, 0)
	SetConnectionChannel(&ableToOpenMGSConnection)
	messagingService := GetConnectionChannel()
	assert.Equal(t, string(messagingService), "ec2messages")
}

func TestSetConnectionChannelSetsConnectionToMDSIfAbleToOpenMGSConnectionNil(t *testing.T) {
	var ableToOpenMGSConnection *uint32 = nil
	SetConnectionChannel(ableToOpenMGSConnection)
	messagingService := GetConnectionChannel()
	assert.Equal(t, string(messagingService), "ec2messages")
}

func TestGetConnectionChannelReturnsEmptyStringIfConnectionHasNotBeenSet(t *testing.T) {
	connectionChannel.SSMConnectionChannel = ""
	messagingService := GetConnectionChannel()
	assert.Equal(t, string(messagingService), "")
}
