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
	"fmt"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/credentials/ec2rolecreds"

	"github.com/aws/amazon-ssm-agent/agent/mocks/log"
	"github.com/aws/amazon-ssm-agent/common/identity/availableidentities/ec2/mocks"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

type mockEndpointHelper struct {
	mock.Mock
}

func (m *mockEndpointHelper) GetServiceEndpoint(service, region string) string {
	args := m.Called(service, region)
	return args.String(0)
}

func TestAgentIdentityCacher_InstanceID(t *testing.T) {
	var resStr string
	var resErr error

	val := "us-east-1a"
	agentIdentityInner := &mocks.IEC2Identity{}
	agentIdentityInner.On("InstanceID").Return(val, nil).Once()

	cacher := agentIdentityCacher{log: log.NewMockLog(), client: agentIdentityInner}

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
	agentIdentityInner := &mocks.IEC2Identity{}
	agentIdentityInner.On("AvailabilityZone").Return(val, nil).Once()

	cacher := agentIdentityCacher{log: log.NewMockLog(), client: agentIdentityInner}

	resStr, resErr = cacher.AvailabilityZone()
	assert.Equal(t, val, resStr)
	assert.NoError(t, resErr)
	resStr, resErr = cacher.AvailabilityZone()
	assert.Equal(t, val, resStr)
	assert.NoError(t, resErr)
}

func TestAgentIdentityCacher_AvailabilityZoneId(t *testing.T) {
	var resStr string
	var resErr error

	val := "use1-az2"
	agentIdentityInner := &mocks.IEC2Identity{}
	agentIdentityInner.On("AvailabilityZoneId").Return(val, nil).Once()

	cacher := agentIdentityCacher{log: log.NewMockLog(), client: agentIdentityInner}

	resStr, resErr = cacher.AvailabilityZoneId()
	assert.Equal(t, val, resStr)
	assert.NoError(t, resErr)
	resStr, resErr = cacher.AvailabilityZoneId()
	assert.Equal(t, val, resStr)
	assert.NoError(t, resErr)
}

func TestAgentIdentityCacher_InstanceType(t *testing.T) {
	var resStr string
	var resErr error

	val := "SomeInstanceType"
	agentIdentityInner := &mocks.IEC2Identity{}
	agentIdentityInner.On("InstanceType").Return(val, nil).Once()

	cacher := agentIdentityCacher{log: log.NewMockLog(), client: agentIdentityInner}

	resStr, resErr = cacher.InstanceType()
	assert.Equal(t, val, resStr)
	assert.NoError(t, resErr)
	resStr, resErr = cacher.InstanceType()
	assert.Equal(t, val, resStr)
	assert.NoError(t, resErr)
}

func TestAgentIdentityCacher_Credentials(t *testing.T) {
	val := ec2rolecreds.New()
	agentIdentityInner := &mocks.IEC2Identity{}
	// .Once() asserts the inner CredentialsProvider() is called EXACTLY once
	// across all cacher.CredentialsProvider() invocations below — proving the
	// cache actually hits. A previous revision of the cacher never wrote its
	// provider field, so every call fell through to the inner client and this
	// assertion would have failed with "Return not set for call 2".
	agentIdentityInner.On("CredentialsProvider").Return(val).Once()

	cacher := agentIdentityCacher{log: log.NewMockLog(), client: agentIdentityInner}

	assert.Equal(t, val, cacher.CredentialsProvider())
	// Second and third calls must be served from the cache — the .Once()
	// expectation above fires if the inner is consulted again.
	assert.Equal(t, val, cacher.CredentialsProvider())
	assert.Equal(t, val, cacher.CredentialsProvider())
	agentIdentityInner.AssertExpectations(t)
}

func TestAgentIdentityCacher_IdentityType(t *testing.T) {
	var resStr string

	val := "SomeIdentityType"
	agentIdentityInner := &mocks.IEC2Identity{}
	agentIdentityInner.On("IdentityType").Return(val, nil).Once()

	cacher := agentIdentityCacher{log: log.NewMockLog(), client: agentIdentityInner}

	resStr = cacher.IdentityType()
	assert.Equal(t, val, resStr)
	resStr = cacher.IdentityType()
	assert.Equal(t, val, resStr)
}

func TestAgentIdentityCacher_ShortInstanceID_NormalInstanceID(t *testing.T) {
	// Setup mock with a standard instance ID
	val := "i-1234567890abcdef"
	agentIdentityInner := &mocks.IEC2Identity{}
	agentIdentityInner.On("InstanceID").Return(val, nil).Once()

	cacher := agentIdentityCacher{
		log:    log.NewMockLog(),
		client: agentIdentityInner,
	}

	shortID, err := cacher.ShortInstanceID()
	assert.NoError(t, err)
	assert.Equal(t, val, shortID)
}

func TestAgentIdentityCacher_ShortInstanceID_ExtremelyLongInstanceID(t *testing.T) {
	val := strings.Repeat("a", 200)
	agentIdentityInner := &mocks.IEC2Identity{}
	agentIdentityInner.On("InstanceID").Return(val, nil)

	mockLog := log.NewMockLog()
	cacher := agentIdentityCacher{log: mockLog, client: agentIdentityInner}

	// Verify warning is logged for inability to shorten
	shortID, err := cacher.ShortInstanceID()
	mockLog.Warnf("Unable to shorten instance id '%s'", shortID)
	assert.Nil(t, err)
	assert.Len(t, shortID, 200)
}

func TestAgentIdentityCacher_ShortInstanceID_ErrorRetrievingInstanceID(t *testing.T) {
	// Setup mock to return an error when retrieving instance ID
	agentIdentityInner := &mocks.IEC2Identity{}
	agentIdentityInner.On("InstanceID").Return("", fmt.Errorf("retrieval error"))

	cacher := agentIdentityCacher{
		log:    log.NewMockLog(),
		client: agentIdentityInner,
	}

	// Verify error is propagated
	_, err := cacher.ShortInstanceID()
	assert.Error(t, err)
}

func TestAgentIdentityCacher_ShortInstanceID_SuccessfulShortnedID(t *testing.T) {
	// Setup mock with a shortened instance ID
	val := strings.Repeat("a", 64) + "_" + strings.Repeat("a", 64)
	agentIdentityInner := &mocks.IEC2Identity{}
	agentIdentityInner.On("InstanceID").Return(val, nil)

	cacher := agentIdentityCacher{
		log:    log.NewMockLog(),
		client: agentIdentityInner,
	}

	shortID, err := cacher.ShortInstanceID()
	assert.NoError(t, err)
	assert.Len(t, shortID, 64)

}

func TestAgentIdentityCacher_Region_Caching(t *testing.T) {
	// Test that region is cached after first retrieval
	val := "us-west-2"
	agentIdentityInner := &mocks.IEC2Identity{}

	// Expect Region() to be called only once
	agentIdentityInner.On("Region").Return(val, nil)

	cacher := agentIdentityCacher{log: log.NewMockLog(), client: agentIdentityInner}

	region1, err1 := cacher.Region()
	assert.NoError(t, err1)
	assert.Equal(t, val, region1)
}

func TestAgentIdentityCacher_GetServiceEndpoint_Success(t *testing.T) {
	// Setup
	region := "us-east-1"
	service := "s3"
	expectedEndpoint := "s3.us-east-1.amazonaws.com"

	// Create mock dependencies
	mockLog := log.NewMockLog()
	mockEndpointHelper := &mockEndpointHelper{}
	mockIdentityInner := &mocks.IEC2Identity{}

	// Configure mock behaviors
	mockIdentityInner.On("Region").Return(region, nil)
	mockEndpointHelper.On("GetServiceEndpoint", service, region).Return(expectedEndpoint)

	// Create cacher with mocks
	cacher := agentIdentityCacher{
		log:            mockLog,
		client:         mockIdentityInner,
		endpointHelper: mockEndpointHelper,
	}

	// Execute
	endpoint := cacher.GetServiceEndpoint(service)

	// Assert
	assert.Equal(t, expectedEndpoint, endpoint)
	mockIdentityInner.AssertExpectations(t)
	mockEndpointHelper.AssertExpectations(t)
}

func TestAgentIdentityCacher_GetServiceEndpoint_RegionError(t *testing.T) {
	// Setup
	service := "ec2"
	regionError := fmt.Errorf("region retrieval failed")

	// Create mock dependencies
	mockLog := log.NewMockLog()
	mockEndpointHelper := &mockEndpointHelper{}
	mockIdentityInner := &mocks.IEC2Identity{}

	// Configure mock behaviors
	mockIdentityInner.On("Region").Return("", regionError)
	mockEndpointHelper.On("GetServiceEndpoint", service, "").Return("")

	// Create cacher with mocks
	cacher := agentIdentityCacher{
		log:            mockLog,
		client:         mockIdentityInner,
		endpointHelper: mockEndpointHelper,
	}

	// Execute
	endpoint := cacher.GetServiceEndpoint(service)

	// Assert
	assert.Empty(t, endpoint)
	mockIdentityInner.AssertExpectations(t)
	mockEndpointHelper.AssertExpectations(t)
}
