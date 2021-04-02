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

// package rolecreds contains functions that help procure the managed instance auth credentials
// tests
package onprem

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/aws/amazon-ssm-agent/agent/appconfig"
	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/amazon-ssm-agent/common/identity/availableidentities/onprem/rsaauth"
	"github.com/cenkalti/backoff"

	"github.com/aws/aws-sdk-go/service/ssm"
	"github.com/stretchr/testify/assert"
)

var (
	accessKeyID     = "accessKeyID"
	secretAccessKey = "secretAccessKey"
	sessionToken    = "sessionToken"
)

func init() {
	backoffRetry = func(fun backoff.Operation, _ backoff.BackOff) error {
		return fun()
	}

	createNewClient = func(log log.T, config *appconfig.SsmagentConfig, privateKey string, oldClient rsaauth.RsaSignedService) rsaauth.RsaSignedService {
		return oldClient
	}
}

func TestRetrieve_ShouldReturnValidToken(t *testing.T) {
	updateKeyPair := false
	tokenExpirationDate := time.Now().Add(1 * time.Hour)
	managedInstance = &registrationStub{
		shouldRotate: false,
	}
	testProvider := managedInstancesRoleProvider{
		client: &RsaSignedServiceStub{
			roleResponse: ssm.RequestManagedInstanceRoleTokenOutput{
				AccessKeyId:         &accessKeyID,
				SecretAccessKey:     &secretAccessKey,
				SessionToken:        &sessionToken,
				UpdateKeyPair:       &updateKeyPair,
				TokenExpirationDate: &tokenExpirationDate,
			},
		},
		config: &appconfig.SsmagentConfig{},
		log:    log.NewMockLog(),
	}
	cred, err := testProvider.Retrieve()
	assert.NoError(t, err)
	assert.Equal(t, accessKeyID, cred.AccessKeyID)
	assert.Equal(t, secretAccessKey, cred.SecretAccessKey)
	assert.Equal(t, sessionToken, cred.SessionToken)
}

func TestRetrieve_ShouldUpdateKeyPair_Error(t *testing.T) {
	updateKeyPair := true
	tokenExpirationDate := time.Now().Add(1 * time.Hour)
	managedInstance = &registrationStub{
		publicKey:    "publicKey",
		privateKey:   "privateKey",
		keyType:      "Rsa",
		shouldRotate: false,
		errList:      []error{nil, fmt.Errorf("SomeError")},
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
		client: client,
		config: &appconfig.SsmagentConfig{},
		log:    log.NewMockLog(),
	}
	_, err := testProvider.Retrieve()
	assert.NoError(t, err)
	assert.Equal(t, 0, client.updateCalled)
}

func TestRetrieve_ShouldFailOnError(t *testing.T) {
	// Fail on machine fingerprint error
	machineFingerprintError := fmt.Errorf("machineFingerprintError")
	managedInstance = &registrationStub{
		errList: []error{machineFingerprintError},
	}
	testProvider := managedInstancesRoleProvider{
		client: &RsaSignedServiceStub{},
		config: &appconfig.SsmagentConfig{},
		log:    log.NewMockLog(),
	}
	_, err := testProvider.Retrieve()
	assert.True(t, strings.Contains(err.Error(), machineFingerprintError.Error()))

	// Fail on requestManagedInstanceRoleTokenError
	requestManagedInstanceRoleTokenError := fmt.Errorf("requestManagedInstanceRoleToken")
	managedInstance = &registrationStub{}
	testProvider = managedInstancesRoleProvider{
		client: &RsaSignedServiceStub{
			errList: []error{requestManagedInstanceRoleTokenError},
		},
		config: &appconfig.SsmagentConfig{},
		log:    log.NewMockLog(),
	}
	_, err = testProvider.Retrieve()
	assert.True(t, strings.Contains(err.Error(), requestManagedInstanceRoleTokenError.Error()))
}

func TestRotatePrivateKey_FailGenerateOldPublicKey(t *testing.T) {
	managedInstance = &registrationStub{
		errList: []error{fmt.Errorf("FailGenerateOldPublicKey")},
	}

	testProvider := managedInstancesRoleProvider{
		client: &RsaSignedServiceStub{},
		config: &appconfig.SsmagentConfig{},
		log:    log.NewMockLog(),
	}

	err := testProvider.rotatePrivateKey("test123", nil)
	assert.NotNil(t, err)
}

func TestRotatePrivateKey_FailGenerateKeyPair(t *testing.T) {
	managedInstance = &registrationStub{
		errList: []error{nil, fmt.Errorf("FailGenerateKeyPair")},
	}

	testProvider := managedInstancesRoleProvider{
		client: &RsaSignedServiceStub{},
		config: &appconfig.SsmagentConfig{},
		log:    log.NewMockLog(),
	}

	err := testProvider.rotatePrivateKey("test123", nil)
	assert.NotNil(t, err)
}

func TestRotatePrivateKey_FailUpdateKey_SuccessVerifyOldKey(t *testing.T) {
	managedInstance = &registrationStub{}
	rsaClient := &RsaSignedServiceStub{
		keyResponse:  ssm.UpdateManagedInstancePublicKeyOutput{},
		roleResponse: ssm.RequestManagedInstanceRoleTokenOutput{},
		errList:      []error{fmt.Errorf("SomeError")},
	}

	testProvider := managedInstancesRoleProvider{
		client: rsaClient,
		config: &appconfig.SsmagentConfig{},
		log:    log.NewMockLog(),
	}

	err := testProvider.rotatePrivateKey("test123", nil)
	assert.NotNil(t, err)
	assert.Equal(t, 1, rsaClient.updateCalled)
	assert.Equal(t, 1, rsaClient.roleCalled)
}

func TestRotatePrivateKey_FailUpdateKey_NewKeyWorks_SuccessSaveNewKey(t *testing.T) {
	managedInstance = &registrationStub{}
	rsaClient := &RsaSignedServiceStub{
		keyResponse:  ssm.UpdateManagedInstancePublicKeyOutput{},
		roleResponse: ssm.RequestManagedInstanceRoleTokenOutput{},
		errList:      []error{fmt.Errorf("UpdateError"), fmt.Errorf("OldKeyTestError")},
	}

	testProvider := managedInstancesRoleProvider{
		client: rsaClient,
		config: &appconfig.SsmagentConfig{},
		log:    log.NewMockLog(),
	}

	err := testProvider.rotatePrivateKey("test123", nil)
	assert.Nil(t, err)
	assert.Equal(t, 1, rsaClient.updateCalled)
	assert.Equal(t, 2, rsaClient.roleCalled)
}

func TestRotatePrivateKey_SuccessUpdateKey_FailSaveNewKey_FailUpdateToOldKey(t *testing.T) {
	managedInstance = &registrationStub{
		errList: []error{nil, nil, fmt.Errorf("FailSaveKey")},
	}
	rsaClient := &RsaSignedServiceStub{
		keyResponse:  ssm.UpdateManagedInstancePublicKeyOutput{},
		roleResponse: ssm.RequestManagedInstanceRoleTokenOutput{},
		errList:      []error{nil, fmt.Errorf("FailUpdateToOldKey")},
	}

	testProvider := managedInstancesRoleProvider{
		client: rsaClient,
		config: &appconfig.SsmagentConfig{},
		log:    log.NewMockLog(),
	}

	err := testProvider.rotatePrivateKey("test123", nil)
	assert.NotNil(t, err)
	assert.Equal(t, 2, rsaClient.updateCalled)
	assert.Equal(t, 0, rsaClient.roleCalled)
}

func TestRotatePrivateKey_SuccessUpdateKey_FailSaveNewKey_SuccessUpdateToOldKey(t *testing.T) {
	managedInstance = &registrationStub{
		errList: []error{nil, nil, fmt.Errorf("FailSaveKey")},
	}
	rsaClient := &RsaSignedServiceStub{
		keyResponse:  ssm.UpdateManagedInstancePublicKeyOutput{},
		roleResponse: ssm.RequestManagedInstanceRoleTokenOutput{},
		errList:      []error{nil, nil},
	}

	testProvider := managedInstancesRoleProvider{
		client: rsaClient,
		config: &appconfig.SsmagentConfig{},
		log:    log.NewMockLog(),
	}

	err := testProvider.rotatePrivateKey("test123", nil)
	assert.NotNil(t, err)
	assert.Equal(t, 2, rsaClient.updateCalled)
	assert.Equal(t, 0, rsaClient.roleCalled)
}

// RsaSignedService client stub
type RsaSignedServiceStub struct {
	errList      []error
	roleResponse ssm.RequestManagedInstanceRoleTokenOutput
	keyResponse  ssm.UpdateManagedInstancePublicKeyOutput
	updateCalled int
	roleCalled   int
}

func (r *RsaSignedServiceStub) getErr() error {
	if len(r.errList) == 0 {
		return nil
	}
	err := r.errList[0]
	r.errList = r.errList[1:]
	return err
}

func (r *RsaSignedServiceStub) RequestManagedInstanceRoleToken(fingerprint string) (response *ssm.RequestManagedInstanceRoleTokenOutput, err error) {
	r.roleCalled += 1
	return &r.roleResponse, r.getErr()
}

func (r *RsaSignedServiceStub) UpdateManagedInstancePublicKey(publicKey, publicKeyType string) (response *ssm.UpdateManagedInstancePublicKeyOutput, err error) {
	r.updateCalled += 1
	return &r.keyResponse, r.getErr()
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
	hasCreds         bool
	shouldRotate     bool
	errList          []error
}

func (r *registrationStub) getErr() error {
	if len(r.errList) == 0 {
		return nil
	}
	err := r.errList[0]
	r.errList = r.errList[1:]
	return err
}

func (r *registrationStub) InstanceID(log log.T) string { return r.instanceID }

func (r *registrationStub) Region(log log.T) string { return r.region }

func (r *registrationStub) InstanceType(log log.T) string { return r.instanceType }

func (r *registrationStub) AvailabilityZone(log log.T) string { return r.availabilityZone }

func (r *registrationStub) Fingerprint(log log.T) (string, error) {
	return r.fingerprint, r.getErr()
}

func (r *registrationStub) PrivateKey(log log.T) string { return r.privateKey }

func (r *registrationStub) PrivateKeyType(log log.T) string { return r.keyType }

func (r *registrationStub) GenerateKeyPair() (publicKey, privateKey, keyType string, err error) {
	return r.publicKey, r.privateKey, r.keyType, r.getErr()
}

func (r *registrationStub) UpdatePrivateKey(log log.T, privateKey, privateKeyType string) (err error) {
	return r.getErr()
}

func (r *registrationStub) ShouldRotatePrivateKey(log.T, int, bool) (bool, error) {
	return r.shouldRotate, r.getErr()
}

func (r *registrationStub) GeneratePublicKey(string) (string, error) {
	return r.publicKey, r.getErr()
}

func (r *registrationStub) HasManagedInstancesCredentials(log log.T) bool {
	return r.hasCreds
}
