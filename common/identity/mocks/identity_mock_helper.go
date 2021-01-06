package mocks

import "github.com/aws/aws-sdk-go/aws/credentials"

func NewDefaultMockAgentIdentity() *IAgentIdentity {
	agentIdentity := IAgentIdentity{}

	agentIdentity.On("AvailabilityZone").Return("us-east-1a", nil)
	agentIdentity.On("Credentials").Return(&credentials.Credentials{}, nil)
	agentIdentity.On("IdentityType").Return("EC2", nil)
	agentIdentity.On("InstanceID").Return("i-123123123", nil)
	agentIdentity.On("InstanceType").Return("someInstanceType", nil)
	agentIdentity.On("IsIdentityEnvironment").Return(true)
	agentIdentity.On("Region").Return("us-east-1", nil)
	agentIdentity.On("ServiceDomain").Return("amazonaws.com", nil)
	agentIdentity.On("ShortInstanceID").Return("i-123123123", nil)
	agentIdentity.On("GetDefaultEndpoint").Return("ssm.us-east-1.amazonaws.com")

	return &agentIdentity

}

func NewMockAgentIdentity(instanceID, region, availabilityZone, instanceType, identityType string) *IAgentIdentity {
	agentIdentity := IAgentIdentity{}

	agentIdentity.On("AvailabilityZone").Return(availabilityZone, nil)
	agentIdentity.On("Credentials").Return(&credentials.Credentials{}, nil)
	agentIdentity.On("IdentityType").Return(identityType, nil)
	agentIdentity.On("InstanceID").Return(instanceID, nil)
	agentIdentity.On("InstanceType").Return(instanceType, nil)
	agentIdentity.On("IsIdentityEnvironment").Return(true)
	agentIdentity.On("Region").Return(region, nil)
	agentIdentity.On("ServiceDomain").Return("amazonaws.com", nil)
	agentIdentity.On("ShortInstanceID").Return(instanceID, nil)

	return &agentIdentity

}
