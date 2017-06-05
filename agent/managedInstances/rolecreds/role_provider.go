// Copyright 2016 Amazon.com, Inc. or its affiliates. All Rights Reserved.
//
// Licensed under the AWS Customer Agreement (the "License"). You may
// not use this file except in compliance with the License. A copy of the
// License is located at
//
// http://aws.amazon.com/agreement/
//
// package rolecreds contains functions that help procure the managed instance auth credentials
package rolecreds

import (
	"fmt"
	"sync"
	"time"

	"github.com/aws/amazon-ssm-agent/agent/appconfig"
	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/amazon-ssm-agent/agent/managedInstances/sharedCredentials"
	"github.com/aws/amazon-ssm-agent/agent/ssm/rsaauth"
	"github.com/aws/aws-sdk-go/aws/credentials"
)

const (
	// ProviderName provides a name of managed instance Role provider
	ProviderName = "managedInstancesRoleProvider"

	// EarlyExpiryTimeWindow set a short amount of time that will mark the credentials as expired, this can avoid
	// calls being made with expired credentials. This value should not be too big that's greater than the default token
	// expiry time. For example, the token expires after 30 min and we set it to 40 min which expires the token
	// immediately. The value should also not be too small that it should trigger credential rotation before it expires.
	EarlyExpiryTimeWindow = 1 * time.Minute
)

// managedInstancesRoleProvider implements the AWS SDK credential provider, and is used to the create AWS client.
// It retrieves credentials from the SSM Auth service, and keeps track if those credentials are expired.
type managedInstancesRoleProvider struct {
	credentials.Expiry

	// Client is the required SSM managed instance service client to use when connecting to SSM Auth service.
	Client rsaauth.RsaSignedService

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
}

var (
	emptyCredential      = credentials.Value{ProviderName: ProviderName}
	credentialsSingleton *credentials.Credentials
	lock                 sync.RWMutex
	logger               log.T
	shareCreds           bool
	shareProfile         string
)

// ManagedInstanceCredentialsInstance returns a singleton instance of
// Crednetials which provides credentials of a managed instance.
func ManagedInstanceCredentialsInstance() *credentials.Credentials {
	lock.Lock()
	defer lock.Unlock()
	logger = log.Logger()
	shareCreds = true
	if config, err := appconfig.Config(false); err == nil {
		shareCreds = config.Profile.ShareCreds
		shareProfile = config.Profile.ShareProfile
	}

	if credentialsSingleton == nil {
		credentialsSingleton = newManagedInstanceCredentials()
	}
	return credentialsSingleton
}

// newManagedInstanceCredentials returns a pointer to a new Credentials object wrapping
// the managedInstancesRoleProvider.
func newManagedInstanceCredentials() *credentials.Credentials {
	instanceID := managedInstance.InstanceID()
	region := managedInstance.Region()
	privateKey := managedInstance.PrivateKey()
	p := &managedInstancesRoleProvider{
		Client:       rsaauth.NewRsaService(instanceID, region, privateKey),
		ExpiryWindow: EarlyExpiryTimeWindow,
	}

	return credentials.NewCredentials(p)
}

// Retrieve retrieves credentials from the SSM Auth service.
// Error will be returned if the request fails, or unable to extract
// the desired credentials.
func (m *managedInstancesRoleProvider) Retrieve() (credentials.Value, error) {
	fingerprint, err := managedInstance.Fingerprint()
	if err != nil {
		return emptyCredential, fmt.Errorf("error reading machine fingerprint: %v", err)
	}

	roleCreds, err := m.Client.RequestManagedInstanceRoleToken(fingerprint)
	if err != nil {
		return emptyCredential, fmt.Errorf("error occurred in RequestManagedInstanceRoleToken: %v", err)
	}

	// check if SSM has requested the agent to update the instance keypair
	if *roleCreds.UpdateKeyPair {
		publicKey, privateKey, keyType, err := managedInstance.GenerateKeyPair()
		if err != nil {
			return emptyCredential, fmt.Errorf("error generating keys: %v", err)
		}

		// call ssm UpdateManagedInstancePublicKey
		_, err = m.Client.UpdateManagedInstancePublicKey(publicKey, keyType)
		if err != nil {
			// TODO: Perform smart retry
			// In case of client error, try some Onprem API call with new private key
			// if call succeeds, then update the Private key, else retry UpdateManagedInstancePublicKey
			return emptyCredential, fmt.Errorf("error updating public key: %v", err)
		}

		// persist the new key
		err = managedInstance.UpdatePrivateKey(privateKey, keyType)
		if err != nil {
			return emptyCredential, fmt.Errorf("error persisting private key: %v", err)
		}
	}

	m.SetExpiration(*roleCreds.TokenExpirationDate, m.ExpiryWindow)

	// check to see if the agent should publish the credentials to the account aws credentials
	if shareCreds {
		err = sharedCredentials.Store(*roleCreds.AccessKeyId, *roleCreds.SecretAccessKey, *roleCreds.SessionToken, shareProfile)
		if err != nil {
			logger.Error(ProviderName, "Error occurred sharing credentials. ", err) // error does not stop execution
		}
	}

	return credentials.Value{
		AccessKeyID:     *roleCreds.AccessKeyId,
		SecretAccessKey: *roleCreds.SecretAccessKey,
		SessionToken:    *roleCreds.SessionToken,
		ProviderName:    ProviderName,
	}, nil
}
