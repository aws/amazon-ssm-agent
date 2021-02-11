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
	"time"

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

// ServiceDomain returns the service domain of a ec2 instance
func (i *Identity) ServiceDomain() (string, error) {
	return i.Client.GetMetadata(ec2ServiceDomainResource)
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
