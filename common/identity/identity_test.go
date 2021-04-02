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

package identity

import (
	"testing"

	"github.com/aws/amazon-ssm-agent/agent/log"
	identityMocks "github.com/aws/amazon-ssm-agent/common/identity/mocks"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/stretchr/testify/assert"
)

func TestAgentIdentityCacher_InstanceID(t *testing.T) {
	var resStr string
	var resErr error

	val := "us-east-1a"
	agentIdentityInner := identityMocks.IAgentIdentityInner{}
	agentIdentityInner.On("InstanceID").Return(val, nil).Once()

	cacher := agentIdentityCacher{log: log.NewMockLog(), client: &agentIdentityInner}

	resStr, resErr = cacher.InstanceID()
	assert.Equal(t, val, resStr)
	assert.NoError(t, resErr)
	resStr, resErr = cacher.InstanceID()
	assert.Equal(t, val, resStr)
	assert.NoError(t, resErr)
}

func TestAgentIdentityCacher_AvailabilityZone(t *testing.T) {
	var resStr string
	var resErr error

	val := "us-east-1a"
	agentIdentityInner := &identityMocks.IAgentIdentityInner{}
	agentIdentityInner.On("AvailabilityZone").Return(val, nil).Once()

	cacher := agentIdentityCacher{log: log.NewMockLog(), client: agentIdentityInner}

	resStr, resErr = cacher.AvailabilityZone()
	assert.Equal(t, val, resStr)
	assert.NoError(t, resErr)
	resStr, resErr = cacher.AvailabilityZone()
	assert.Equal(t, val, resStr)
	assert.NoError(t, resErr)
}

func TestAgentIdentityCacher_InstanceType(t *testing.T) {
	var resStr string
	var resErr error

	val := "SomeInstanceType"
	agentIdentityInner := identityMocks.IAgentIdentityInner{}
	agentIdentityInner.On("InstanceType").Return(val, nil).Once()

	cacher := agentIdentityCacher{log: log.NewMockLog(), client: &agentIdentityInner}

	resStr, resErr = cacher.InstanceType()
	assert.Equal(t, val, resStr)
	assert.NoError(t, resErr)
	resStr, resErr = cacher.InstanceType()
	assert.Equal(t, val, resStr)
	assert.NoError(t, resErr)
}

func TestAgentIdentityCacher_Credentials(t *testing.T) {
	val := &credentials.Credentials{}
	agentIdentityInner := identityMocks.IAgentIdentityInner{}
	agentIdentityInner.On("Credentials").Return(val).Once()

	cacher := agentIdentityCacher{log: log.NewMockLog(), client: &agentIdentityInner}

	assert.Equal(t, val, cacher.Credentials())
	assert.Equal(t, val, cacher.Credentials())
}

func TestAgentIdentityCacher_IdentityType(t *testing.T) {
	var resStr string

	val := "SomeIdentityType"
	agentIdentityInner := identityMocks.IAgentIdentityInner{}
	agentIdentityInner.On("IdentityType").Return(val, nil).Once()

	cacher := agentIdentityCacher{log: log.NewMockLog(), client: &agentIdentityInner}

	resStr = cacher.IdentityType()
	assert.Equal(t, val, resStr)
	resStr = cacher.IdentityType()
	assert.Equal(t, val, resStr)
}
