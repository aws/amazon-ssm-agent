package ec2infradetect

import (
	"github.com/aws/amazon-ssm-agent/agent/log"
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
func CollectEc2Infrastructure(log log.T) (*Ec2Infrastructure, error) {

	instanceID, _ := platformProviderdep.InstanceID(log)
	instanceType, _ := platformProviderdep.InstanceType(log)
	region, _ := platformProviderdep.Region(log)
	availabilityZone, _ := platformProviderdep.AvailabilityZone(log)

	e := &Ec2Infrastructure{
		InstanceID:       instanceID,
		Region:           region,
		AvailabilityZone: availabilityZone,
		InstanceType:     instanceType,
	}
	return e, nil
}
