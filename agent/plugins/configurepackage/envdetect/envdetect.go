package envdetect

import (
	"github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/aws/amazon-ssm-agent/agent/plugins/configurepackage/envdetect/ec2infradetect"
	"github.com/aws/amazon-ssm-agent/agent/plugins/configurepackage/envdetect/osdetect"
)

// Environment contains data for:
// * Operating system
// * Ec2 infrastructure
type Environment struct {
	OperatingSystem   *osdetect.OperatingSystem
	Ec2Infrastructure *ec2infradetect.Ec2Infrastructure
}

type Collector interface {
	CollectData(context context.T) (*Environment, error)
}

type CollectorImp struct {
}

// CollectData queries operating system and infrastructure data
func (cd *CollectorImp) CollectData(context context.T) (*Environment, error) {
	os, err := osdetect.CollectOSData(context.Log())
	if err != nil {
		return nil, err
	}

	ec2inf, err := ec2infradetect.CollectEc2Infrastructure(context.Identity())
	if err != nil {
		return nil, err
	}

	e := &Environment{
		OperatingSystem:   os,
		Ec2Infrastructure: ec2inf,
	}
	return e, nil
}
