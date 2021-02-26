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

package onprem

import (
	"fmt"

	"github.com/aws/aws-sdk-go/aws/credentials"
)

// InstanceID returns the managed instance ID
func (i *Identity) InstanceID() (string, error) { return managedInstance.InstanceID(i.Log), nil }

// Region returns the region of the managed instance
func (i *Identity) Region() (string, error) { return managedInstance.Region(i.Log), nil }

// AvailabilityZone returns the managed instance availabilityZone
func (*Identity) AvailabilityZone() (string, error) {
	return IdentityType, nil
}

// InstanceType returns the managed instance instanceType
func (*Identity) InstanceType() (string, error) {
	return IdentityType, nil
}

// ServiceDomain returns the service domain of a OnPrem instance
func (*Identity) ServiceDomain() (string, error) {
	return "", fmt.Errorf("No service domain available in OnPrem")
}

// Credentials returns the managed instance credentials
func (i *Identity) Credentials() *credentials.Credentials {
	shareLock.Lock()
	defer shareLock.Unlock()

	shareCreds = i.Config.Profile.ShareCreds
	shareProfile = i.Config.Profile.ShareProfile
	if i.credentialsSingleton == nil {
		p := &managedInstancesRoleProvider{
			log:          i.Log.WithContext("[OnPremCreds]"),
			config:       i.Config,
			ExpiryWindow: EarlyExpiryTimeWindow,
		}

		p.InitializeClient(managedInstance.PrivateKey(i.Log))

		i.credentialsSingleton = credentials.NewCredentials(p)
	}
	return i.credentialsSingleton
}

// IsIdentityEnvironment returns if instance has managed instance registration
func (i *Identity) IsIdentityEnvironment() bool {
	return managedInstance.HasManagedInstancesCredentials(i.Log)
}

// IdentityType returns the identity type of the managed instance
func (*Identity) IdentityType() string { return IdentityType }
