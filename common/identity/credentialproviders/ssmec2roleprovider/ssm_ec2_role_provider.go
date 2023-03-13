// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License").
// You may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package ssmec2roleprovider

import (
	"fmt"
	"sync"
	"time"

	"github.com/aws/amazon-ssm-agent/agent/appconfig"
	"github.com/aws/amazon-ssm-agent/agent/backoffconfig"
	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/amazon-ssm-agent/agent/managedInstances/registration"
	"github.com/aws/amazon-ssm-agent/agent/ssm/authregister"
	"github.com/aws/amazon-ssm-agent/agent/ssm/authtokenrequest"
	"github.com/aws/amazon-ssm-agent/agent/ssm/rsaauth"
	"github.com/aws/amazon-ssm-agent/common/identity/credentialproviders/iirprovider"

	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/service/ssm"
)

const RegistrationType = "EC2"

var (
	newIirRsaAuth           = rsaauth.NewIirRsaClient
	getStoredInstanceId     = registration.InstanceID
	getStoredPrivateKey     = registration.PrivateKey
	getStoredPublicKey      = registration.PublicKey
	getStoredPrivateKeyType = registration.PrivateKeyType
	loadRegistrationLock    = &sync.Mutex{}
)

// SSMEC2RoleProvider sends requests for credentials to systems manager signed with AWS SigV4
type SSMEC2RoleProvider struct {
	credentials.Expiry
	// ExpiryWindow will allow the credentials to trigger refreshing prior to
	// the credentials actually expiring. This is beneficial so race conditions
	// with expiring credentials do not cause request to fail unexpectedly
	// due to ExpiredTokenException exceptions.
	//
	// So a ExpiryWindow of 10s would cause calls to IsExpired() to return true
	// 10 seconds before the credentials are actually expired.
	//
	// If ExpiryWindow is 0 or less it will be ignored.
	ExpiryWindow time.Duration
	Config       *appconfig.SsmagentConfig
	Log          log.T

	IMDSClient         iirprovider.IEC2MdsSdkClient
	tokenRequestClient authtokenrequest.IClient
	InstanceInfo       *InstanceInfo

	registrationInfo *authregister.RegistrationInfo
}

func (p *SSMEC2RoleProvider) isEC2InstanceRegistered() bool {
	if p.registrationInfo == nil {
		registrationInfo := p.loadRegistrationInfo(p.InstanceInfo.InstanceId)
		if registrationInfo == nil || registrationInfo.InstanceId == "" {
			p.Log.Debug("EC2 instance is not yet registered with Systems Manager")
			return false
		}

		loadRegistrationLock.Lock()
		defer loadRegistrationLock.Unlock()
		p.registrationInfo = registrationInfo
	}

	return p.registrationInfo.PrivateKey != "" && p.registrationInfo.KeyType != ""
}

// Retrieve retrieves EC2 credentials from Systems Manager
func (p *SSMEC2RoleProvider) Retrieve() (credentials.Value, error) {
	var err error
	var roleCreds *ssm.RequestManagedInstanceRoleTokenOutput

	if !p.isEC2InstanceRegistered() {
		p.SetExpiration(time.Time{}, 0)
		return EmptyCredentials(), fmt.Errorf("ec2 instance not yet registered with Systems Manager")
	}

	if p.tokenRequestClient == nil {
		p.tokenRequestClient = newIirRsaAuth(p.Log.WithContext("[TokenRequestService]"),
			p.Config,
			p.IMDSClient,
			p.InstanceInfo.Region,
			p.registrationInfo.PrivateKey)
	}

	exponentialBackoff, err := backoffconfig.GetDefaultExponentialBackoff()
	if err != nil {
		p.Log.Errorf("failed to create backoff Config. Error: %v", exponentialBackoff)
		p.SetExpiration(time.Time{}, 0)
		return EmptyCredentials(), err
	}

	// Get role token
	roleCreds, err = p.tokenRequestClient.RequestManagedInstanceRoleToken(p.InstanceInfo.InstanceId)
	if err != nil {
		p.SetExpiration(time.Time{}, 0)
		return EmptyCredentials(), fmt.Errorf("error calling RequestManagedInstanceRoleToken: %w", err)
	}

	// Set the expiration of our credentials
	p.SetExpiration(*roleCreds.TokenExpirationDate, p.ExpiryWindow)

	return credentials.Value{
		AccessKeyID:     *roleCreds.AccessKeyId,
		SecretAccessKey: *roleCreds.SecretAccessKey,
		SessionToken:    *roleCreds.SessionToken,
		ProviderName:    ProviderName,
	}, nil
}

// EmptyCredentials returns empty SSMEC2RoleProvider credentials
func EmptyCredentials() credentials.Value {
	return credentials.Value{ProviderName: ProviderName}
}

func (p *SSMEC2RoleProvider) loadRegistrationInfo(instanceId string) *authregister.RegistrationInfo {
	registrationInfo := &authregister.RegistrationInfo{
		InstanceId: getStoredInstanceId(p.Log, RegistrationType, registration.EC2RegistrationVaultKey),
		PrivateKey: getStoredPrivateKey(p.Log, RegistrationType, registration.EC2RegistrationVaultKey),
		KeyType:    getStoredPrivateKeyType(p.Log, RegistrationType, registration.EC2RegistrationVaultKey),
		PublicKey:  getStoredPublicKey(p.Log, RegistrationType, registration.EC2RegistrationVaultKey),
	}

	if registrationInfo.InstanceId == "" || registrationInfo.PrivateKey == "" ||
		registrationInfo.KeyType == "" || registrationInfo.InstanceId != instanceId {
		registrationInfo.InstanceId = "" // setting it as blank to try registration
	}

	return registrationInfo
}
