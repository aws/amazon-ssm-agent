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

package bootstrap

import (
	"testing"

	"github.com/aws/amazon-ssm-agent/agent/appconfig"
	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/amazon-ssm-agent/common/identity"
	identityMock "github.com/aws/amazon-ssm-agent/common/identity/mocks"
	fileSystemMock "github.com/aws/amazon-ssm-agent/core/workerprovider/longrunningprovider/datastore/filesystem/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestBootstrap(t *testing.T) {
	logger := log.NewMockLog()
	fileSystem := &fileSystemMock.FileSystem{}

	agentIdentity := &identityMock.IAgentIdentity{}
	agentIdentity.On("InstanceID").Return("SomeInstanceId", nil)

	newAgentIdentitySelector = func(log log.T) identity.IAgentIdentitySelector {
		// Selector object does not matter since it is exclusively passed to newAgentIdentity function
		return nil
	}

	newAgentIdentity = func(log log.T, config *appconfig.SsmagentConfig, selector identity.IAgentIdentitySelector) (identity.IAgentIdentity, error) {
		return agentIdentity, nil
	}

	fileSystem.On("Stat", mock.Anything).Return(nil, nil)
	fileSystem.On("IsNotExist", mock.Anything).Return(true)
	fileSystem.On("MkdirAll", mock.Anything, mock.Anything).Return(nil)

	bs := NewBootstrap(logger, fileSystem)

	context, err := bs.Init()
	assert.Nil(t, err)
	assert.NotNil(t, context)

	assert.Equal(t, context.Log(), logger)
	assert.Equal(t, context.Identity(), agentIdentity)
}
