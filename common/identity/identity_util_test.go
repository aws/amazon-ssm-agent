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

	"github.com/aws/amazon-ssm-agent/agent/appconfig"
	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/stretchr/testify/assert"
)

func TestIsOnPremInstance(t *testing.T) {

	logger := log.NewMockLog()
	appConfig := &appconfig.SsmagentConfig{}

	assert.False(t, IsOnPremInstance(nil))

	agentIdentity := &agentIdentityCacher{
		client: newEC2Identity(logger, appConfig)[0],
	}
	assert.False(t, IsOnPremInstance(agentIdentity))

	agentIdentity = &agentIdentityCacher{
		client: newECSIdentity(logger, appConfig)[0],
	}
	assert.False(t, IsOnPremInstance(agentIdentity))

	agentIdentity = &agentIdentityCacher{
		client: newOnPremIdentity(logger, appConfig)[0],
	}
	assert.True(t, IsOnPremInstance(agentIdentity))
}
