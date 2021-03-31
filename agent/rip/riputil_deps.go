package rip

import (
	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/amazon-ssm-agent/common/identity/endpoint"
)

var ruEndpoint IRipUtilEndpoint = &ruIdentityEndpointImpl{}

type IRipUtilEndpoint interface {
	GetDefaultEndpoint(log.T, string, string, string) string
}

type ruIdentityEndpointImpl struct{}

func (ruIdentityEndpointImpl) GetDefaultEndpoint(log log.T, service, region, serviceDomain string) string {
	return endpoint.GetDefaultEndpoint(log, service, region, serviceDomain)
}
