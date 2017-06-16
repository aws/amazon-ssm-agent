package ec2infradetect

import (
	"testing"

	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

var loggerMock = log.NewMockLog()

func TestCollectEc2Infrastructure(t *testing.T) {
	mockObj := new(platfromProviderMock)
	mockObj.On("InstanceID", mock.Anything).Return("abc", nil)
	mockObj.On("InstanceType", mock.Anything).Return("xyz", nil)
	mockObj.On("AvailabilityZone", mock.Anything).Return("az1", nil)
	mockObj.On("Region", mock.Anything).Return("reg1", nil)

	platformProviderdep = mockObj

	result, err := CollectEc2Infrastructure(loggerMock)

	assert.Equal(t, "abc", result.InstanceID)
	assert.Equal(t, "xyz", result.InstanceType)
	assert.Equal(t, "az1", result.AvailabilityZone)
	assert.Equal(t, "reg1", result.Region)
	assert.NoError(t, err)
}
