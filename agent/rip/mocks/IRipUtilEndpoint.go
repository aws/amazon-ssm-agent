package mocks

import (
	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/stretchr/testify/mock"
)

type IRipUtilEndpoint struct {
	mock.Mock
}

func (_m *IRipUtilEndpoint) GetDefaultEndpoint(logger log.T, service, region, serviceDomain string) string {
	ret := _m.Called(logger, service, region, serviceDomain)
	var defaultEndpoint string

	if rf, ok := ret.Get(0).(func(log.T, string, string, string) string); ok {
		defaultEndpoint = rf(logger, service, region, serviceDomain)
	} else {
		defaultEndpoint = ret.Get(0).(string)
	}

	return defaultEndpoint
}
