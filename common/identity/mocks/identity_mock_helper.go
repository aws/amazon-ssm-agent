package mocks

import (
	"github.com/aws/amazon-ssm-agent/common/identity/creds"
	"github.com/stretchr/testify/mock"
)

var (
	MockAvailabilityZone      = "us-east-1a"
	MockCredentials           = creds.GetRemoteCreds()
	MockIdentityType          = "EC2"
	MockInstanceID            = "i-123123123"
	MockInstanceType          = "someInstanceType"
	MockIsIdentityEnvironment = true
	MockRegion                = "us-east-1"
	MockServiceDomain         = "amazonaws.com"
	MockShortInstanceID       = "i-123123123"
)

func NewDefaultMockAgentIdentity() *IAgentIdentity {
	agentIdentity := IAgentIdentity{}

	agentIdentity.On("AvailabilityZone").Return(MockAvailabilityZone, nil)
	agentIdentity.On("Credentials").Return(MockCredentials, nil)
	agentIdentity.On("IdentityType").Return(MockIdentityType, nil)
	agentIdentity.On("InstanceID").Return(MockInstanceID, nil)
	agentIdentity.On("InstanceType").Return(MockInstanceType, nil)
	agentIdentity.On("IsIdentityEnvironment").Return(MockIsIdentityEnvironment)
	agentIdentity.On("Region").Return(MockRegion, nil)
	agentIdentity.On("ServiceDomain").Return(MockServiceDomain, nil)
	agentIdentity.On("ShortInstanceID").Return(MockShortInstanceID, nil)
	agentIdentity.On("GetDefaultEndpoint", mock.AnythingOfType("string")).Return(func(service string) string {
		return service + "." + MockRegion + "." + MockServiceDomain
	})
	return &agentIdentity

}

func NewMockAgentIdentity(instanceID, region, availabilityZone, instanceType, identityType string) *IAgentIdentity {
	agentIdentity := IAgentIdentity{}

	agentIdentity.On("AvailabilityZone").Return(availabilityZone, nil)
	agentIdentity.On("Credentials").Return(MockCredentials, nil)
	agentIdentity.On("IdentityType").Return(identityType, nil)
	agentIdentity.On("InstanceID").Return(instanceID, nil)
	agentIdentity.On("InstanceType").Return(instanceType, nil)
	agentIdentity.On("IsIdentityEnvironment").Return(true)
	agentIdentity.On("Region").Return(region, nil)
	agentIdentity.On("ServiceDomain").Return("amazonaws.com", nil)
	agentIdentity.On("ShortInstanceID").Return(instanceID, nil)

	return &agentIdentity

}
