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
package ec2

import (
	"fmt"
	"testing"

	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/aws-sdk-go/aws/ec2metadata"
	"github.com/stretchr/testify/assert"
)

func TestEC2IdentityType_InstanceId(t *testing.T) {
	client := &iEC2MdsSdkClientMock{}

	identity := Identity{
		Log:    log.NewMockLog(),
		Client: client,
	}
	val := "SomeId"
	client.On("GetMetadata", ec2InstanceIDResource).Return(val, nil).Once()

	res, err := identity.InstanceID()
	assert.Equal(t, res, val)
	assert.NoError(t, err)
}

func TestEC2IdentityType_RegionFirstSuccess(t *testing.T) {
	client := &iEC2MdsSdkClientMock{}

	identity := Identity{
		Log:    log.NewMockLog(),
		Client: client,
	}
	val := "SomeRegion"
	client.On("Region").Return(val, nil).Once()

	res, err := identity.Region()
	assert.Equal(t, res, val)
	assert.NoError(t, err)
}

func TestEC2IdentityType_RegionFailDocumentSuccess(t *testing.T) {
	client := &iEC2MdsSdkClientMock{}

	identity := Identity{
		Log:    log.NewMockLog(),
		Client: client,
	}
	val := "SomeOtherRegion"
	document := ec2metadata.EC2InstanceIdentityDocument{Region: val}

	client.On("Region").Return("", fmt.Errorf("SomeError")).Once()
	client.On("GetInstanceIdentityDocument").Return(document, nil).Once()

	res, err := identity.Region()
	assert.Equal(t, res, val)
	assert.NoError(t, err)
}

func TestEC2IdentityType_AvailabilityZone(t *testing.T) {
	client := &iEC2MdsSdkClientMock{}

	identity := Identity{
		Log:    log.NewMockLog(),
		Client: client,
	}
	val := "SomeAZ"
	client.On("GetMetadata", ec2AvailabilityZoneResource).Return(val, nil).Once()

	res, err := identity.AvailabilityZone()
	assert.Equal(t, res, val)
	assert.NoError(t, err)
}

func TestEC2IdentityType_InstanceType(t *testing.T) {
	client := &iEC2MdsSdkClientMock{}

	identity := Identity{
		Log:    log.NewMockLog(),
		Client: client,
	}
	val := "SomeInstanceType"
	client.On("GetMetadata", ec2InstanceTypeResource).Return(val, nil).Once()

	res, err := identity.InstanceType()
	assert.Equal(t, res, val)
	assert.NoError(t, err)
}

func TestEC2IdentityType_ServiceDomain(t *testing.T) {
	client := &iEC2MdsSdkClientMock{}

	identity := Identity{
		Log:    log.NewMockLog(),
		Client: client,
	}
	val := "SomeServiceDomain"
	client.On("GetMetadata", ec2ServiceDomainResource).Return(val, nil).Once()

	res, err := identity.ServiceDomain()
	assert.Equal(t, res, val)
	assert.NoError(t, err)
}

func TestEC2IdentityType_Credentials(t *testing.T) {
	client := &iEC2MdsSdkClientMock{}

	identity := Identity{
		Log:    log.NewMockLog(),
		Client: client,
	}

	assert.NotNil(t, identity.Credentials())
}

func TestEC2IdentityType_IsIdentityEnvironment(t *testing.T) {
	client := &iEC2MdsSdkClientMock{}

	identity := Identity{
		Log:    log.NewMockLog(),
		Client: client,
	}

	// Success
	client.On("GetMetadata", ec2InstanceIDResource).Return("", fmt.Errorf("SomeError")).Once()
	assert.False(t, identity.IsIdentityEnvironment())

	client.On("GetMetadata", ec2InstanceIDResource).Return("SomeInstanceId", nil).Once()
	assert.True(t, identity.IsIdentityEnvironment())

}

func TestEC2IdentityType_IdentityType(t *testing.T) {
	identity := Identity{
		Log: log.NewMockLog(),
	}

	res := identity.IdentityType()
	assert.Equal(t, res, IdentityType)
}
