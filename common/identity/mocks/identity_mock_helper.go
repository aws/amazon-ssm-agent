package mocks

import (
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/stretchr/testify/mock"
)

// Test fixtures deliberately do NOT touch the real credential chain at init
// time. The prior revision constructed MockCredentials/MockCredentialsProvider
// via credentialproviders.GetRemoteCreds() and GetRemoteCredsProvider(), which
// eagerly hit EC2 IMDS and executed a full config.LoadDefaultConfig chain the
// moment this package was imported by ANY test binary — burning a few seconds
// of wall time per test package on machines with reachable IMDS and, worse,
// producing real network traffic from unit tests. Use inert static
// credentials instead; production identity paths never touch this file.
var (
	MockAvailabilityZone      = "us-east-1a"
	MockAvailabilityZoneId    = "use1-az2"
	MockCredentials           = aws.Credentials{AccessKeyID: "mock-ak", SecretAccessKey: "mock-sk", Source: "MockIdentity"}
	MockCredentialsProvider   = credentials.StaticCredentialsProvider{Value: MockCredentials}
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
	agentIdentity.On("AvailabilityZoneId").Return(MockAvailabilityZoneId, nil)
	agentIdentity.On("CredentialsProvider").Return(MockCredentialsProvider)
	agentIdentity.On("IdentityType").Return(MockIdentityType, nil)
	agentIdentity.On("InstanceID").Return(MockInstanceID, nil)
	agentIdentity.On("InstanceType").Return(MockInstanceType, nil)
	agentIdentity.On("IsIdentityEnvironment").Return(MockIsIdentityEnvironment)
	agentIdentity.On("Region").Return(MockRegion, nil)
	agentIdentity.On("ServiceDomain").Return(MockServiceDomain, nil)
	agentIdentity.On("ShortInstanceID").Return(MockShortInstanceID, nil)
	agentIdentity.On("GetServiceEndpoint", mock.AnythingOfType("string")).Return(func(service string) string {
		return service + "." + MockRegion + "." + MockServiceDomain
	})
	return &agentIdentity

}

func NewMockAgentIdentity(instanceID, region, availabilityZone, instanceType, identityType string) *IAgentIdentity {
	agentIdentity := IAgentIdentity{}

	agentIdentity.On("AvailabilityZone").Return(availabilityZone, nil)
	agentIdentity.On("IdentityType").Return(identityType, nil)
	agentIdentity.On("CredentialsProvider").Return(MockCredentialsProvider)
	agentIdentity.On("InstanceID").Return(instanceID, nil)
	agentIdentity.On("InstanceType").Return(instanceType, nil)
	agentIdentity.On("IsIdentityEnvironment").Return(true)
	agentIdentity.On("Region").Return(region, nil)
	agentIdentity.On("ServiceDomain").Return("amazonaws.com", nil)
	agentIdentity.On("ShortInstanceID").Return(instanceID, nil)

	return &agentIdentity

}
