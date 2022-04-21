// Copyright 2020 Amazon.com, Inc. or its affiliates. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License"). You may not
// use this file except in compliance with the License. A copy of the
// License is located at
//
// http://aws.amazon.com/apache2.0/
//
// or in the "license" file accompanying this file. This file is distributed
// on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND,
// either express or implied. See the License for the specific language governing
// permissions and limitations under the License.

package ec2

import (
	"strings"
	"time"

	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/credentials/ec2rolecreds"
	"github.com/aws/aws-sdk-go/aws/ec2metadata"
	"github.com/aws/aws-sdk-go/aws/session"
)

// InstanceID returns the managed instance id
func (i *Identity) InstanceID() (string, error) {
	return i.Client.GetMetadata(ec2InstanceIDResource)
}

// Region returns the region of the ec2 instance
func (i *Identity) Region() (region string, err error) {
	if region, err = i.Client.Region(); err == nil {
		return
	}
	var document ec2metadata.EC2InstanceIdentityDocument
	if document, err = i.Client.GetInstanceIdentityDocument(); err == nil {
		region = document.Region
	}

	return
}

// AvailabilityZone returns the availabilityZone ec2 instance
func (i *Identity) AvailabilityZone() (string, error) {
	return i.Client.GetMetadata(ec2AvailabilityZoneResource)
}

// InstanceType returns the instance type of the ec2 instance
func (i *Identity) InstanceType() (string, error) {
	return i.Client.GetMetadata(ec2InstanceTypeResource)
}

// Credentials returns the managed instance credentials.
// Since credentials expire in about 6 hours, setting the ExpiryWindow to 5 hours
// will trigger a refresh 5 hours before they actually expire. So the TTL of credentials
// is reduced to about 1 hour to match EC2 assume role frequency.
func (i *Identity) Credentials() *credentials.Credentials {
	provider := &ec2rolecreds.EC2RoleProvider{
		Client:       ec2metadata.New(session.New()),
		ExpiryWindow: time.Hour * 5,
	}

	return credentials.NewCredentials(provider)
}

// IsIdentityEnvironment returns if instance is a ec2 instance
func (i *Identity) IsIdentityEnvironment() bool {
	_, err := i.InstanceID()
	return err == nil
}

// IdentityType returns the identity type of the ec2 instance
func (i *Identity) IdentityType() string { return IdentityType }

// VpcPrimaryCIDRBlock returns ipv4, ipv6 VPC CIDR block addresses if exists
func (i *Identity) VpcPrimaryCIDRBlock() (ip map[string][]string, err error) {
	macs, err := i.Client.GetMetadata(ec2MacsResource)
	if err != nil {
		return map[string][]string{}, err
	}

	addresses := strings.Split(macs, "\n")
	ipv4 := make([]string, len(addresses))
	ipv6 := make([]string, len(addresses))

	for index, address := range addresses {
		ipv4[index], _ = i.Client.GetMetadata(ec2MacsResource + "/" + address + "/" + ec2VpcCidrBlockV4Resource)
		ipv6[index], _ = i.Client.GetMetadata(ec2MacsResource + "/" + address + "/" + ec2VpcCidrBlockV6Resource)
	}

	return map[string][]string{"ipv4": ipv4, "ipv6": ipv6}, nil
}

// NewEC2Identity initializes the ec2 identity
func NewEC2Identity(log log.T) *Identity {
	awsConfig := &aws.Config{}
	awsConfig = awsConfig.WithMaxRetries(3)
	sess, _ := session.NewSession(awsConfig)

	log = log.WithContext("[EC2Identity]")
	return &Identity{
		Log:    log,
		Client: ec2metadata.New(sess),
	}
}
