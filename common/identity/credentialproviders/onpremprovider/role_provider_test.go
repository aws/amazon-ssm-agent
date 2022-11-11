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
package onpremprovider

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/aws/amazon-ssm-agent/agent/appconfig"
	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/amazon-ssm-agent/agent/ssm/authtokenrequest"

	"github.com/aws/aws-sdk-go/service/ssm"

	"github.com/cenkalti/backoff/v4"
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

	createNewClient = func(m *onpremCredentialsProvider, privateKey string) authtokenrequest.IClient {
		return m.client
	}
}

func TestRetrieve_ShouldReturnValidToken(t *testing.T) {
	updateKeyPair := false
	tokenExpirationDate := time.Now().Add(1 * time.Hour)
	testProvider := onpremCredentialsProvider{
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
		registrationInfo: &registrationStub{
			shouldRotate: false,
		},
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
	client := &RsaSignedServiceStub{
		roleResponse: ssm.RequestManagedInstanceRoleTokenOutput{
			AccessKeyId:         &accessKeyID,
			SecretAccessKey:     &secretAccessKey,
			SessionToken:        &sessionToken,
			UpdateKeyPair:       &updateKeyPair,
			TokenExpirationDate: &tokenExpirationDate,
		},
	}
	testProvider := onpremCredentialsProvider{
		client: client,
		config: &appconfig.SsmagentConfig{},
		log:    log.NewMockLog(),
		registrationInfo: &registrationStub{
			publicKey:    "publicKey",
			privateKey:   "privateKey",
			keyType:      "Rsa",
			shouldRotate: false,
			errList:      []error{nil, fmt.Errorf("SomeError")},
		},
	}
	_, err := testProvider.Retrieve()
	assert.NoError(t, err)
	assert.Equal(t, 0, client.updateCalled)
}

func TestRetrieve_ShouldFailOnError(t *testing.T) {
	// Fail on machine fingerprint error
	machineFingerprintError := fmt.Errorf("machineFingerprintError")
	testProvider := onpremCredentialsProvider{
		client: &RsaSignedServiceStub{},
		config: &appconfig.SsmagentConfig{},
		log:    log.NewMockLog(),
		registrationInfo: &registrationStub{
			errList: []error{machineFingerprintError},
		},
	}
	_, err := testProvider.Retrieve()
	assert.True(t, strings.Contains(err.Error(), machineFingerprintError.Error()))

	// Fail on requestManagedInstanceRoleTokenError
	requestManagedInstanceRoleTokenError := fmt.Errorf("requestManagedInstanceRoleToken")
	testProvider = onpremCredentialsProvider{
		client: &RsaSignedServiceStub{
			errList: []error{requestManagedInstanceRoleTokenError},
		},
		config:           &appconfig.SsmagentConfig{},
		log:              log.NewMockLog(),
		registrationInfo: &registrationStub{},
	}
	_, err = testProvider.Retrieve()
	assert.True(t, strings.Contains(err.Error(), requestManagedInstanceRoleTokenError.Error()))
}

func TestRotatePrivateKey_FailGenerateOldPublicKey(t *testing.T) {
	testProvider := onpremCredentialsProvider{
		client: &RsaSignedServiceStub{},
		config: &appconfig.SsmagentConfig{},
		log:    log.NewMockLog(),
		registrationInfo: &registrationStub{
			errList: []error{fmt.Errorf("FailGenerateOldPublicKey")},
		},
	}

	err := testProvider.rotatePrivateKey("test123", nil)
	assert.NotNil(t, err)
}

func TestRotatePrivateKey_FailGenerateKeyPair(t *testing.T) {
	testProvider := onpremCredentialsProvider{
		client: &RsaSignedServiceStub{},
		config: &appconfig.SsmagentConfig{},
		log:    log.NewMockLog(),
		registrationInfo: &registrationStub{
			errList: []error{nil, fmt.Errorf("FailGenerateKeyPair")},
		},
	}

	err := testProvider.rotatePrivateKey("test123", nil)
	assert.NotNil(t, err)
}

func TestRotatePrivateKey_FailUpdateKey_SuccessVerifyOldKey(t *testing.T) {
	rsaClient := &RsaSignedServiceStub{
		keyResponse:  ssm.UpdateManagedInstancePublicKeyOutput{},
		roleResponse: ssm.RequestManagedInstanceRoleTokenOutput{},
		errList:      []error{fmt.Errorf("SomeError")},
	}

	testProvider := onpremCredentialsProvider{
		client:           rsaClient,
		config:           &appconfig.SsmagentConfig{},
		log:              log.NewMockLog(),
		registrationInfo: &registrationStub{},
	}

	err := testProvider.rotatePrivateKey("test123", nil)
	assert.NotNil(t, err)
	assert.Equal(t, 1, rsaClient.updateCalled)
	assert.Equal(t, 1, rsaClient.roleCalled)
}

func TestRotatePrivateKey_FailUpdateKey_NewKeyWorks_SuccessSaveNewKey(t *testing.T) {
	rsaClient := &RsaSignedServiceStub{
		keyResponse:  ssm.UpdateManagedInstancePublicKeyOutput{},
		roleResponse: ssm.RequestManagedInstanceRoleTokenOutput{},
		errList:      []error{fmt.Errorf("UpdateError"), fmt.Errorf("OldKeyTestError")},
	}

	testProvider := onpremCredentialsProvider{
		client:           rsaClient,
		config:           &appconfig.SsmagentConfig{},
		log:              log.NewMockLog(),
		registrationInfo: &registrationStub{},
	}

	err := testProvider.rotatePrivateKey("test123", nil)
	assert.Nil(t, err)
	assert.Equal(t, 1, rsaClient.updateCalled)
	assert.Equal(t, 2, rsaClient.roleCalled)
}

func TestRotatePrivateKey_SuccessUpdateKey_FailSaveNewKey_FailUpdateToOldKey(t *testing.T) {
	rsaClient := &RsaSignedServiceStub{
		keyResponse:  ssm.UpdateManagedInstancePublicKeyOutput{},
		roleResponse: ssm.RequestManagedInstanceRoleTokenOutput{},
		errList:      []error{nil, fmt.Errorf("FailUpdateToOldKey")},
	}

	testProvider := onpremCredentialsProvider{
		client: rsaClient,
		config: &appconfig.SsmagentConfig{},
		log:    log.NewMockLog(),
		registrationInfo: &registrationStub{
			errList: []error{nil, nil, fmt.Errorf("FailSaveKey")},
		},
	}

	err := testProvider.rotatePrivateKey("test123", nil)
	assert.NotNil(t, err)
	assert.Equal(t, 2, rsaClient.updateCalled)
	assert.Equal(t, 0, rsaClient.roleCalled)
}

func TestRotatePrivateKey_SuccessUpdateKey_FailSaveNewKey_SuccessUpdateToOldKey(t *testing.T) {

	rsaClient := &RsaSignedServiceStub{
		keyResponse:  ssm.UpdateManagedInstancePublicKeyOutput{},
		roleResponse: ssm.RequestManagedInstanceRoleTokenOutput{},
		errList:      []error{nil, nil},
	}

	testProvider := onpremCredentialsProvider{
		client: rsaClient,
		config: &appconfig.SsmagentConfig{},
		log:    log.NewMockLog(),
		registrationInfo: &registrationStub{
			errList: []error{nil, nil, fmt.Errorf("FailSaveKey")},
		},
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

func (r *registrationStub) InstanceID(log log.T, manifestFileNamePrefix, vaultKey string) string {
	return r.instanceID
}

func (r *registrationStub) Region(log log.T, manifestFileNamePrefix, vaultKey string) string {
	return r.region
}

func (r *registrationStub) InstanceType(log log.T) string { return r.instanceType }

func (r *registrationStub) AvailabilityZone(log log.T) string { return r.availabilityZone }

func (r *registrationStub) Fingerprint(log log.T) (string, error) {
	return r.fingerprint, r.getErr()
}

func (r *registrationStub) PrivateKey(log log.T, manifestFileNamePrefix, vaultKey string) string {
	return r.privateKey
}

func (r *registrationStub) PrivateKeyType(log log.T, manifestFileNamePrefix, vaultKey string) string {
	return r.keyType
}

func (r *registrationStub) GenerateKeyPair() (publicKey, privateKey, keyType string, err error) {
	return r.publicKey, r.privateKey, r.keyType, r.getErr()
}

func (r *registrationStub) UpdatePrivateKey(log log.T, privateKey, privateKeyType, manifestFileNamePrefix, vaultKey string) (err error) {
	return r.getErr()
}

func (r *registrationStub) ShouldRotatePrivateKey(log.T, string, int, bool, string, string) (bool, error) {
	return r.shouldRotate, r.getErr()
}

func (r *registrationStub) GeneratePublicKey(string) (string, error) {
	return r.publicKey, r.getErr()
}

func (r *registrationStub) HasManagedInstancesCredentials(log log.T, manifestFileNamePrefix, vaultKey string) bool {
	return r.hasCreds
}

func (r *registrationStub) ReloadInstanceInfo(log log.T, manifestFileNamePrefix, vaultKey string) {}
