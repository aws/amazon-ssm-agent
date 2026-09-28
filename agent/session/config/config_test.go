package config

import (
	"testing"

	"github.com/aws/amazon-ssm-agent/agent/mocks/context"
	"github.com/stretchr/testify/assert"
)

func TestGetMgsEndpointForUnknownRegion(t *testing.T) {
	region := "unknown-region-1"
	// GetMgsEndpoint returns a scheme-less hostname; callers prepend
	// HttpsPrefix or WebSocketPrefix themselves (mgsinteractor, datachannel,
	// websocketchannel). See config.go — the endpoint helper's scheme is
	// stripped to keep this contract stable across appconfig-override and
	// helper-fallback branches.
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
