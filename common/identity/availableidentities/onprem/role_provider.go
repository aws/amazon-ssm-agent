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
	"github.com/aws/amazon-ssm-agent/agent/managedInstances/sharedCredentials"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"time"
)

// Retrieve retrieves credentials from the SSM Auth service.
// Error will be returned if the request fails, or unable to extract
// the desired credentials.
func (m *managedInstancesRoleProvider) Retrieve() (credentials.Value, error) {
	fingerprint, err := managedInstance.Fingerprint(m.Log)
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
		err = managedInstance.UpdatePrivateKey(m.Log, privateKey, keyType)
		if err != nil {
			return emptyCredential, fmt.Errorf("error persisting private key: %v", err)
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
		err = sharedCredentials.Store(*roleCreds.AccessKeyId, *roleCreds.SecretAccessKey, *roleCreds.SessionToken, shareProfile)
		if err != nil {
			m.Log.Error(ProviderName, "Error occurred sharing credentials. ", err) // error does not stop execution
		}
	}

	return credentials.Value{
		AccessKeyID:     *roleCreds.AccessKeyId,
		SecretAccessKey: *roleCreds.SecretAccessKey,
		SessionToken:    *roleCreds.SessionToken,
		ProviderName:    ProviderName,
	}, nil
}
