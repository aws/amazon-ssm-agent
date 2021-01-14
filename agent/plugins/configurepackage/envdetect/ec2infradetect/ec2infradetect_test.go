package ec2infradetect

import (
	"testing"

	identityMocks "github.com/aws/amazon-ssm-agent/common/identity/mocks"
	"github.com/stretchr/testify/assert"
)

func TestCollectEc2Infrastructure(t *testing.T) {
	instanceID := "i-abc123"
	region := "us-west-1"
	instanceType := "SomeInstanceType"
	availabilityZone := "SomeAZ"

	identity := identityMocks.NewMockAgentIdentity(instanceID, region, availabilityZone, instanceType, "")

	result, err := CollectEc2Infrastructure(identity)

	assert.Equal(t, instanceID, result.InstanceID)
	assert.Equal(t, instanceType, result.InstanceType)
	assert.Equal(t, availabilityZone, result.AvailabilityZone)
	assert.Equal(t, region, result.Region)
	assert.NoError(t, err)
}
