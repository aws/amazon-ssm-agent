package ec2infradetect

import (
	"github.com/aws/amazon-ssm-agent/common/identity"
)

// Ec2Infrastructure contains information about instance, region and account
// queried from Ec2 metadata service
type Ec2Infrastructure struct {
	InstanceID       string
	Region           string
	AccountID        string
	AvailabilityZone string
	InstanceType     string
}

// CollectEc2Infrastructure queries Ec2 metadata service for infrastructure
// information
var CollectEc2Infrastructure = func(identity identity.IAgentIdentity) (*Ec2Infrastructure, error) {

	instanceID, _ := identity.InstanceID()
	instanceType, _ := identity.InstanceType()
	region, _ := identity.Region()
	availabilityZone, _ := identity.AvailabilityZone()

	e := &Ec2Infrastructure{
		InstanceID:       instanceID,
		Region:           region,
		AvailabilityZone: availabilityZone,
		InstanceType:     instanceType,
	}
	return e, nil
}
