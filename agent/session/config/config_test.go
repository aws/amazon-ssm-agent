package config

import (
	"testing"

	"github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/stretchr/testify/assert"
)

func TestGetMgsEndpointForUnknownRegion(t *testing.T) {
	region := "unknown-region-1"
	expected := ServiceName + "." + region + ".amazonaws.com"

	contextMock := context.NewMockDefault()

	endpoint := GetMgsEndpoint(contextMock, region)

	assert.Equal(t, expected, endpoint)
}

func TestGetMgsEndpointForUnknownCnRegion(t *testing.T) {
	region := "cn-unknown-1"
	expected := ServiceName + "." + region + ".amazonaws.com.cn"

	contextMock := context.NewMockDefault()

	endpoint := GetMgsEndpoint(contextMock, region)

	assert.Equal(t, expected, endpoint)
}

func TestGetMgsEndpointForKnownAwsRegion(t *testing.T) {
	region := "us-east-1"
	expected := ServiceName + "." + region + ".amazonaws.com"

	contextMock := context.NewMockDefault()

	endpoint := GetMgsEndpoint(contextMock, region)

	assert.Equal(t, expected, endpoint)
}

func TestGetMgsEndpointForKnownAwsCnRegion(t *testing.T) {
	region := "cn-northwest-1"
	expected := ServiceName + ".cn-northwest-1.amazonaws.com.cn"

	contextMock := context.NewMockDefault()

	endpoint := GetMgsEndpoint(contextMock, region)

	assert.Equal(t, expected, endpoint)
}
