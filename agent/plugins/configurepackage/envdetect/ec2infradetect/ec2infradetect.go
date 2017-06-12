package ec2infradetect

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
func CollectEc2Infrastructure() (*Ec2Infrastructure, error) {
	e := &Ec2Infrastructure{}
	return e, nil
}
