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

package customidentity

import (
	"github.com/aws/amazon-ssm-agent/agent/appconfig"
	"github.com/aws/amazon-ssm-agent/common/identity/credentialproviders"
	"github.com/aws/aws-sdk-go/aws/credentials"
)

// InstanceID returns the managed instance id
func (i *Identity) InstanceID() (string, error) {
	return i.CustomIdentity.InstanceID, nil
}

// Region returns the region of the ec2 instance
func (i *Identity) Region() (region string, err error) {
	return i.CustomIdentity.Region, nil
}

// AvailabilityZone returns the availabilityZone of the ec2 instance
func (i *Identity) AvailabilityZone() (string, error) {
	return i.CustomIdentity.AvailabilityZone, nil
}

// AvailabilityZoneId returns the availabilityZoneId of the ec2 instance
func (i *Identity) AvailabilityZoneId() (string, error) {
	return i.CustomIdentity.AvailabilityZoneId, nil
}

// InstanceType returns the instance type of the ec2 instance
func (i *Identity) InstanceType() (string, error) {
	return i.CustomIdentity.InstanceType, nil
}

// Credentials returns the configured credentials
func (i *Identity) Credentials() *credentials.Credentials {
	switch i.CustomIdentity.CredentialsProvider {
	case appconfig.DefaultCustomIdentityCredentialsProvider:
		return credentialproviders.GetDefaultCreds()
	}

	i.Log.Warnf("CustomIdentity credentials provider '%s' not supported", i.CustomIdentity.CredentialsProvider)
	return credentialproviders.GetDefaultCreds()
}

// IsIdentityEnvironment always returns true for custom identities
func (i *Identity) IsIdentityEnvironment() bool {
	return true
}

// IdentityType returns the identity type of the CustomIdentity
func (i *Identity) IdentityType() string { return IdentityType }
