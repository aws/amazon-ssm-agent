// Copyright 2026 Amazon.com, Inc. or its affiliates. All Rights Reserved.
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

package googlecloud

import (
	"testing"

	"github.com/aws/amazon-ssm-agent/agent/appconfig"
	"github.com/aws/amazon-ssm-agent/agent/managedInstances/registration"
	"github.com/aws/amazon-ssm-agent/agent/managedInstances/registration/mocks"
	logmocks "github.com/aws/amazon-ssm-agent/agent/mocks/log"
	"github.com/stretchr/testify/assert"
)

func TestIdentity_InstanceType(t *testing.T) {
	identity := &Identity{}
	instanceType, err := identity.InstanceType()

	assert.NoError(t, err)
	assert.Equal(t, IdentityType, instanceType)
}

func TestIdentity_IdentityType(t *testing.T) {
	identity := &Identity{}
	identityType := identity.IdentityType()

	assert.Equal(t, IdentityType, identityType)
}

func TestIdentity_SourceType(t *testing.T) {
	identity := &Identity{}
	sourceType := identity.SourceType()

	assert.Equal(t, SourceType, sourceType)
}

func TestIdentity_InstanceID(t *testing.T) {
	mockLog := logmocks.NewMockLog()
	mockRegistrationInfo := &mocks.IOnpremRegistrationInfo{}

	identity := &Identity{
		Log:              mockLog,
		registrationInfo: mockRegistrationInfo,
	}

	expectedInstanceID := "test-instance-id"
	mockRegistrationInfo.On("InstanceID", mockLog, "", registration.RegVaultKey).Return(expectedInstanceID)

	instanceID, err := identity.InstanceID()

	assert.NoError(t, err)
	assert.Equal(t, expectedInstanceID, instanceID)
	mockRegistrationInfo.AssertExpectations(t)
}

func TestIdentity_Region(t *testing.T) {
	mockLog := logmocks.NewMockLog()
	mockRegistrationInfo := &mocks.IOnpremRegistrationInfo{}

	identity := &Identity{
		Log:              mockLog,
		registrationInfo: mockRegistrationInfo,
	}

	expectedRegion := "us-east-1"
	mockRegistrationInfo.On("Region", mockLog, "", registration.RegVaultKey).Return(expectedRegion)

	region, err := identity.Region()

	assert.NoError(t, err)
	assert.Equal(t, expectedRegion, region)
	mockRegistrationInfo.AssertExpectations(t)
}

func TestNewGCPIdentity_WithSharedCredentials(t *testing.T) {
	mockLog := logmocks.NewMockLog()
	config := &appconfig.SsmagentConfig{
		Profile: appconfig.CredentialProfile{
			ShareCreds: true,
		},
	}

	identity := NewGCPIdentity(mockLog, config)

	assert.NotNil(t, identity)
	assert.Equal(t, config, identity.Config)
	assert.True(t, identity.shouldShareCredentials)
	assert.NotNil(t, identity.registrationInfo)
}

func TestNewGCPIdentity_WithoutSharedCredentials(t *testing.T) {
	mockLog := logmocks.NewMockLog()
	config := &appconfig.SsmagentConfig{
		Profile: appconfig.CredentialProfile{
			ShareCreds: false,
		},
	}

	identity := NewGCPIdentity(mockLog, config)

	assert.NotNil(t, identity)
	assert.Equal(t, config, identity.Config)
	assert.False(t, identity.shouldShareCredentials)
	assert.NotNil(t, identity.registrationInfo)
}
