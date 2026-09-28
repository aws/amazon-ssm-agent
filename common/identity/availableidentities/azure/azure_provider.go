// Copyright 2026 Amazon.com, Inc. or its affiliates. All Rights Reserved.
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
package azure

import (
	"context"
	"fmt"
	"sync"

	"github.com/aws/aws-sdk-go-v2/aws"

	"github.com/aws/amazon-ssm-agent/agent/appconfig"
	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/amazon-ssm-agent/agent/managedInstances/registration"
	"github.com/aws/amazon-ssm-agent/agent/managedInstances/sharedCredentials"
	"github.com/aws/amazon-ssm-agent/common/identity/credentialproviders"
	"github.com/aws/amazon-ssm-agent/common/identity/credentialproviders/onpremprovider"
	"github.com/aws/amazon-ssm-agent/common/identity/credentialproviders/sharedprovider"
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
	return fetchAvailabilityZone()
}

// AvailabilityZoneId returns empty if the managed instance is not EC2
func (*Identity) AvailabilityZoneId() (string, error) {
	zone, err := fetchAvailabilityZone()
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("Zone%s", zone), nil
}

// InstanceType returns the managed instance instanceType
func (*Identity) InstanceType() (string, error) {
	return IdentityType, nil
}

// initShareCreds initializes credentials using shared credentials provider that reads credentials from shared location
func (i *Identity) initShareCreds() {
	shareCredsProvider := sharedprovider.NewCredentialsProvider(i.Log)
	i.credentials, _ = shareCredsProvider.Retrieve(context.TODO())
}

// initNonShareCreds initializes credentials provider and credentials that do not share credentials via aws credentials file
func (i *Identity) initNonShareCreds() {
	i.credentialsProvider = onpremprovider.NewCredentialsProvider(i.Log, i.Config, i.registrationInfo, true)
	i.credentials, _ = i.credentialsProvider.Retrieve(context.TODO())
}

// CredentialsProvider returns the initialized credentials provider
func (i *Identity) CredentialsProvider() aws.CredentialsProvider {
	i.credsInitMutex.Lock()
	defer i.credsInitMutex.Unlock()

	if !i.credentials.HasKeys() {
		if i.shouldShareCredentials {
			i.initShareCreds()
		} else {
			i.initNonShareCreds()
		}
	}

	if i.credentialsProvider == nil {
		i.credentialsProvider = onpremprovider.NewCredentialsProvider(i.Log, i.Config, i.registrationInfo, i.shouldShareCredentials)
	}

	return i.credentialsProvider
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

// IsIdentityEnvironment returns if instance is running on Azure and has managed instance registration
func (i *Identity) IsIdentityEnvironment() bool {
	// First check if we're on Azure
	_, err := fetchAzureMetadata()
	if err != nil {
		i.Log.Infof("Agent not taking Azure identity: %v", err)
		return false
	}

	// Then check if we have managed instance credentials
	return i.registrationInfo.HasManagedInstancesCredentials(i.Log, "", registration.RegVaultKey)
}

// VpcPrimaryCIDRBlock returns ipv4, ipv6 VPC CIDR block addresses if exists
func (i *Identity) VpcPrimaryCIDRBlock() (map[string][]string, error) {
	return fetchCIDRBlock()
}

// IdentityType returns the identity type of the managed instance
func (i *Identity) IdentityType() string { return IdentityType }

func (i *Identity) SourceId() (string, error) { return fetchVMID() }

func (i *Identity) SourceType() string { return SourceType }

func (i *Identity) SourceLocation() (string, error) { return fetchRegion() }

func (i *Identity) ComputerName() (string, error) { return fetchComputerName() }

// NewAzureIdentity initializes the Azure identity and credentials providers and determines if credentials should be shared or not
func NewAzureIdentity(log log.T, config *appconfig.SsmagentConfig) *Identity {
	var err error
	var shareFile string

	log = log.WithContext("[AzureIdentityProvider]")
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
		shareFile:              shareFile,
		shouldShareCredentials: shouldShareCredentials,
		credsInitMutex:         sync.Mutex{},
	}
}
