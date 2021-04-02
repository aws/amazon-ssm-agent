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
	"time"

	"github.com/aws/amazon-ssm-agent/agent/appconfig"
	"github.com/aws/amazon-ssm-agent/agent/backoffconfig"
	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/amazon-ssm-agent/agent/managedInstances/sharedCredentials"
	"github.com/aws/amazon-ssm-agent/common/identity/availableidentities/onprem/rsaauth"
	"github.com/aws/amazon-ssm-agent/common/identity/endpoint"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/service/ssm"
	"github.com/cenkalti/backoff"
)

// Retrieve retrieves credentials from the SSM Auth service.
// Error will be returned if the request fails, or unable to extract
// the desired credentials.
func (m *managedInstancesRoleProvider) Retrieve() (credentials.Value, error) {
	var err error
	var roleCreds *ssm.RequestManagedInstanceRoleTokenOutput

	fingerprint, err := managedInstance.Fingerprint(m.log)
	if err != nil {
		m.log.Warnf("Failed to get machine fingerprint: %v", err)
		return emptyCredential, err
	}

	exponentialBackoff, err := backoffconfig.GetDefaultExponentialBackoff()
	if err != nil {
		m.log.Warnf("Failed to create backoff config with error: %v", exponentialBackoff)
		return emptyCredential, err
	}

	// Get role token
	err = backoffRetry(func() error {
		roleCreds, err = m.client.RequestManagedInstanceRoleToken(fingerprint)
		return err
	}, exponentialBackoff)

	// Failed to get role token
	if err != nil {
		return emptyCredential, fmt.Errorf("error occurred in RequestManagedInstanceRoleToken: %v", err)
	}

	shouldRotate, err := managedInstance.ShouldRotatePrivateKey(m.log, m.config.Profile.KeyAutoRotateDays, *roleCreds.UpdateKeyPair)
	if err != nil {
		m.log.Warnf("Failed to check if private key should be rotated: %v", err)
	} else if shouldRotate {
		rotateKeyErr := m.rotatePrivateKey(fingerprint, exponentialBackoff)
		if rotateKeyErr != nil {
			m.log.Error("Failed to rotate private key with error: ", rotateKeyErr)
		}
	}

	// Set the expiration window to be half of the token's lifetime. This allows credential refreshes to survive transient
	// network issues more easily. Expiring at half the lifetime also follows the behavior of other protocols such as DHCP
	// https://tools.ietf.org/html/rfc2131#section-4.4.5. Note that not all of the behavior specified in that RFC is
	// implemented, just the suggestion to start renewals at 50% of token validity.
	m.ExpiryWindow = time.Until(*roleCreds.TokenExpirationDate) / 2

	// Set the expiration of our credentials
	m.SetExpiration(*roleCreds.TokenExpirationDate, m.ExpiryWindow)

	// check to see if the agent should publish the credentials to the account aws credentials
	shareLock.RLock()
	defer shareLock.RUnlock()
	if shareCreds {
		err = sharedCredentials.Store(m.log, *roleCreds.AccessKeyId, *roleCreds.SecretAccessKey,
			*roleCreds.SessionToken, shareProfile, m.config.Profile.ForceUpdateCreds)
		if err != nil {
			m.log.Error(ProviderName, "Error occurred sharing credentials. ", err) // error does not stop execution
		}
	}

	return credentials.Value{
		AccessKeyID:     *roleCreds.AccessKeyId,
		SecretAccessKey: *roleCreds.SecretAccessKey,
		SessionToken:    *roleCreds.SessionToken,
		ProviderName:    ProviderName,
	}, nil
}

// rotatePrivateKey attempts to rotate the instance private key
func (m *managedInstancesRoleProvider) rotatePrivateKey(fingerprint string, exponentialBackoff *backoff.ExponentialBackOff) error {
	m.log.Infof("Attempting to rotate private key")

	oldPrivateKey := managedInstance.PrivateKey(m.log)
	oldKeyType := managedInstance.PrivateKeyType(m.log)
	oldPublicKey, err := managedInstance.GeneratePublicKey(oldPrivateKey)
	if err != nil {
		m.log.Warnf("Failed to generate old public key: %v", err)
		return err
	}

	newPublicKey, newPrivateKey, newKeyType, err := managedInstance.GenerateKeyPair()

	if err != nil {
		m.log.Warnf("Failed to generate new key pair: %v", err)
		return err
	}

	// Update remote public key
	err = backoffRetry(func() error {
		_, innerErr := m.client.UpdateManagedInstancePublicKey(newPublicKey, newKeyType)
		return innerErr
	}, exponentialBackoff)

	if err != nil {
		m.log.Warnf("Failed to update public key, trying to recover: %s", err)

		// Updating public key failed, test if old public key is stored in SSM
		err = backoffRetry(func() error {
			_, innerErr := m.client.RequestManagedInstanceRoleToken(fingerprint)
			return innerErr
		}, exponentialBackoff)

		if err == nil {
			return fmt.Errorf("Failed to update remote public key, old key still works")
		}

		// Test if new key works
		m.InitializeClient(newPrivateKey)

		err = backoffRetry(func() error {
			_, innerErr := m.client.RequestManagedInstanceRoleToken(fingerprint)
			return innerErr
		}, exponentialBackoff)

		if err != nil {
			m.log.Warnf("Unable to verify neither new nor old key, rolling back private key change")
			m.InitializeClient(managedInstance.PrivateKey(m.log))
			return err
		}

		m.log.Infof("Successfully verified new key is upstream, updating local key")
	}

	// New key has been updated remotely, update client to use new private key
	m.InitializeClient(newPrivateKey)

	// New key was successfully updated in service, trying to save new key to disk
	err = backoffRetry(func() error {
		innerErr := managedInstance.UpdatePrivateKey(m.log, newPrivateKey, newKeyType)
		return innerErr
	}, exponentialBackoff)

	if err != nil {
		m.log.Warn("Failed to save private key locally, attempting to update remote key back to old public key: ", err)

		// Attempt to update the remote public key to the old public key
		err = backoffRetry(func() error {
			_, innerErr := m.client.UpdateManagedInstancePublicKey(oldPublicKey, oldKeyType)
			return innerErr
		}, exponentialBackoff)

		if err != nil {
			m.log.Error("Failed to roll-back remove public key change, instance most likely needs to be re-registed: %v", err)
			return fmt.Errorf("Failed to update remote public key after saving locally failed: %v", err)
		}

		m.log.Warn("Successfully rolled back remote key, and recovered registration")
		m.InitializeClient(oldPrivateKey)
		return fmt.Errorf("Failed to save new private key to disk")
	}

	m.log.Info("Successfully rotated private key")
	return nil
}

func (m *managedInstancesRoleProvider) InitializeClient(privateKey string) {
	m.client = createNewClient(m.log, m.config, privateKey, m.client)
}

var createNewClient = func(log log.T, config *appconfig.SsmagentConfig, privateKey string, _ rsaauth.RsaSignedService) rsaauth.RsaSignedService {
	instanceID := managedInstance.InstanceID(log)
	region := managedInstance.Region(log)

	// Get default SSM Endpoint
	defaultEndpoint := endpoint.GetDefaultEndpoint(log, "ssm", region, "")

	return rsaauth.NewRsaService(log, config, instanceID, region, defaultEndpoint, privateKey)
}
