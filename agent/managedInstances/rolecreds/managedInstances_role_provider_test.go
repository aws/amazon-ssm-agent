// Copyright 2016 Amazon.com, Inc. or its affiliates. All Rights Reserved.
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

// package rolecreds contains functions that help procure the managed instance auth credentials
// tests
package rolecreds

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go/service/ssm"
	"github.com/stretchr/testify/assert"
)

var (
	accessKeyID     = "accessKeyID"
	secretAccessKey = "secretAccessKey"
	sessionToken    = "sessionToken"
)

func TestRetrieve_ShouldReturnValidToken(t *testing.T) {
	updateKeyPair := false
	tokenExpirationDate := time.Now().Add(1 * time.Hour)
	managedInstance = registrationStub{}
	testProvider := managedInstancesRoleProvider{
		Client: &RsaSignedServiceStub{
			roleResponse: ssm.RequestManagedInstanceRoleTokenOutput{
				AccessKeyId:         &accessKeyID,
				SecretAccessKey:     &secretAccessKey,
				SessionToken:        &sessionToken,
				UpdateKeyPair:       &updateKeyPair,
				TokenExpirationDate: &tokenExpirationDate,
			},
		},
	}
	cred, err := testProvider.Retrieve()
	assert.NoError(t, err)
	assert.Equal(t, accessKeyID, cred.AccessKeyID)
	assert.Equal(t, secretAccessKey, cred.SecretAccessKey)
	assert.Equal(t, sessionToken, cred.SessionToken)
}

func TestRetrieve_ShouldUpdateKeyPair(t *testing.T) {
	updateKeyPair := true
	tokenExpirationDate := time.Now().Add(1 * time.Hour)
	managedInstance = registrationStub{
		publicKey:  "publicKey",
		privateKey: "privateKey",
		keyType:    "Rsa",
	}
	client := &RsaSignedServiceStub{
		roleResponse: ssm.RequestManagedInstanceRoleTokenOutput{
			AccessKeyId:         &accessKeyID,
			SecretAccessKey:     &secretAccessKey,
			SessionToken:        &sessionToken,
			UpdateKeyPair:       &updateKeyPair,
			TokenExpirationDate: &tokenExpirationDate,
		},
	}
	testProvider := managedInstancesRoleProvider{
		Client: client,
	}
	_, err := testProvider.Retrieve()
	assert.NoError(t, err)
	assert.True(t, client.updateCalled)
}

func TestRetrieve_ShouldFailOnError(t *testing.T) {
	// Fail on machine fingerprint error
	machineFingerprintError := fmt.Errorf("machineFingerprintError")
	managedInstance = registrationStub{
		err: machineFingerprintError,
	}
	testProvider := managedInstancesRoleProvider{
		Client: &RsaSignedServiceStub{},
	}
	_, err := testProvider.Retrieve()
	assert.True(t, strings.Contains(err.Error(), machineFingerprintError.Error()))

	// Fail on requestManagedInstanceRoleTokenError
	requestManagedInstanceRoleTokenError := fmt.Errorf("requestManagedInstanceRoleToken")
	managedInstance = registrationStub{}
	testProvider = managedInstancesRoleProvider{
		Client: &RsaSignedServiceStub{
			err: requestManagedInstanceRoleTokenError,
		},
	}
	_, err = testProvider.Retrieve()
	assert.True(t, strings.Contains(err.Error(), requestManagedInstanceRoleTokenError.Error()))
}

// RsaSignedService client stub
type RsaSignedServiceStub struct {
	err          error
	roleResponse ssm.RequestManagedInstanceRoleTokenOutput
	keyResponse  ssm.UpdateManagedInstancePublicKeyOutput
	updateCalled bool
}

func (r *RsaSignedServiceStub) RequestManagedInstanceRoleToken(fingerprint string) (response *ssm.RequestManagedInstanceRoleTokenOutput, err error) {
	return &r.roleResponse, r.err
}

func (r *RsaSignedServiceStub) UpdateManagedInstancePublicKey(publicKey, publicKeyType string) (response *ssm.UpdateManagedInstancePublicKeyOutput, err error) {
	r.updateCalled = true
	return &r.keyResponse, err
}

// registration stub
type registrationStub struct {
	instanceID       string
	region           string
	instanceType     string
	availabilityZone string
	fingerprint      string
	publicKey        string
	privateKey       string
	keyType          string
	err              error
}

func (r registrationStub) InstanceID() string { return r.instanceID }

func (r registrationStub) Region() string { return r.region }

func (r registrationStub) InstanceType() string { return r.instanceType }

func (r registrationStub) AvailabilityZone() string { return r.availabilityZone }

func (r registrationStub) Fingerprint() (string, error) { return r.fingerprint, r.err }

func (r registrationStub) PrivateKey() string { return r.privateKey }

func (r registrationStub) GenerateKeyPair() (publicKey, privateKey, keyType string, err error) {
	return r.publicKey, r.privateKey, r.keyType, r.err
}

func (r registrationStub) UpdatePrivateKey(privateKey, privateKeyType string) (err error) {
	return r.err
}
