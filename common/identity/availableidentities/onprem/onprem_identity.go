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
	"sync"

	"github.com/aws/amazon-ssm-agent/common/identity/credentialproviders"

	"github.com/aws/amazon-ssm-agent/agent/appconfig"
	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/amazon-ssm-agent/agent/managedInstances/registration"
	"github.com/aws/amazon-ssm-agent/agent/managedInstances/sharedCredentials"
	"github.com/aws/amazon-ssm-agent/common/identity/credentialproviders/onpremprovider"
	"github.com/aws/amazon-ssm-agent/common/identity/credentialproviders/sharedprovider"
	"github.com/aws/aws-sdk-go/aws/credentials"
)

// InstanceID returns the managed instance ID
func (i *Identity) InstanceID() (string, error) {
	return i.registrationInfo.InstanceID(i.Log, "", registration.RegVaultKey), nil
}

// Region returns the region of the managed instance
func (i *Identity) Region() (string, error) {
	return i.registrationInfo.Region(i.Log, "", registration.RegVaultKey), nil
}

// AvailabilityZone returns the managed instance availabilityZone
func (*Identity) AvailabilityZone() (string, error) {
	return IdentityType, nil
}

// AvailabilityZoneId returns empty if the managed instance is not EC2
func (*Identity) AvailabilityZoneId() (string, error) {
	return "", nil
}

// InstanceType returns the managed instance instanceType
func (*Identity) InstanceType() (string, error) {
	return IdentityType, nil
}

// ServiceDomain returns the service domain of a OnPrem instance
func (*Identity) ServiceDomain() (string, error) {
	return "", nil
}

// initShareCreds initializes credentials using shared credentials provider that reads credentials from shared location, falls back to non shared credentials provider for any failure
func (i *Identity) initShareCreds() {
	shareCredsProvider, err := sharedprovider.NewCredentialsProvider(i.Log)
	if err != nil {
		i.Log.Errorf("Failed to initialize shared credentials provider, falling back to remote credentials provider: %v", err)
		i.initNonShareCreds()
		return
	}
	i.credentials = credentials.NewCredentials(shareCredsProvider)
}

// initNonShareCreds initializes credentials provider and credentials that do not share credentials via aws credentials file
func (i *Identity) initNonShareCreds() {
	i.credentialsProvider = onpremprovider.NewCredentialsProvider(i.Log, i.Config, i.registrationInfo, true)
	i.credentials = credentials.NewCredentials(i.credentialsProvider)
}

// Credentials returns the managed instance credentials
func (i *Identity) Credentials() *credentials.Credentials {
	i.credsInitMutex.Lock()
	defer i.credsInitMutex.Unlock()

	if i.credentials == nil {
		if i.shouldShareCredentials {
			i.initShareCreds()
		} else {
			i.initNonShareCreds()
		}
	}

	return i.credentials
}

// CredentialProvider returns the initialized credentials provider
func (i *Identity) CredentialProvider() credentialproviders.IRemoteProvider {
	i.credsInitMutex.Lock()
	defer i.credsInitMutex.Unlock()

	if i.credentialsProvider == nil {
		i.credentialsProvider = onpremprovider.NewCredentialsProvider(i.Log, i.Config, i.registrationInfo, i.shouldShareCredentials)
	}

	return i.credentialsProvider
}

// IsIdentityEnvironment returns if instance has managed instance registration
func (i *Identity) IsIdentityEnvironment() bool {
	return i.registrationInfo.HasManagedInstancesCredentials(i.Log, "", registration.RegVaultKey)
}

// IdentityType returns the identity type of the managed instance
func (*Identity) IdentityType() string { return IdentityType }

// NewOnPremIdentity initializes the onprem identity and credentials providers and determines if credentials should be shared or not
func NewOnPremIdentity(log log.T, config *appconfig.SsmagentConfig) *Identity {
	var err error
	var shareFile string

	log = log.WithContext("[OnPremIdentity]")
	shouldShareCredentials := config.Profile.ShareCreds

	registrationInfo := registration.NewOnpremRegistrationInfo()

	// Check if share creds path can be set, if it should, determine the path
	if shouldShareCredentials {
		shareFile, err = sharedCredentials.GetSharedCredsFilePath("")
		if err != nil {
			log.Errorf("Failed to get path to shared credentials file, not sharing credentials: %v", err)
			shouldShareCredentials = false
		}
	}

	return &Identity{
		Log:                    log,
		Config:                 config,
		registrationInfo:       registrationInfo,
		credentialsProvider:    nil,
		credentials:            nil,
		shareFile:              shareFile,
		shouldShareCredentials: shouldShareCredentials,
		credsInitMutex:         sync.Mutex{},
	}
}
