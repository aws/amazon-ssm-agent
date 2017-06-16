package ec2infradetect

import (
	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/amazon-ssm-agent/agent/platform"
)

// dependency on platform detection
type platformProviderDep interface {
	InstanceID(log log.T) (string, error)
	InstanceType(log log.T) (string, error)
	AvailabilityZone(log log.T) (string, error)
	Region(log log.T) (string, error)
}

var platformProviderdep platformProviderDep = &platformProviderDepImp{}

type platformProviderDepImp struct{}

func (*platformProviderDepImp) InstanceID(log log.T) (string, error) {
	return platform.InstanceID()
}

func (*platformProviderDepImp) InstanceType(log log.T) (string, error) {
	return platform.InstanceType()
}

func (*platformProviderDepImp) AvailabilityZone(log log.T) (string, error) {
	return platform.AvailabilityZone()
}

func (*platformProviderDepImp) Region(log log.T) (string, error) {
	return platform.Region()
}
