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

// package onprem contains functions that help procure the managed instance auth credentials
// dependencies
package onprem

import (
	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/amazon-ssm-agent/agent/managedInstances/registration"
	"github.com/cenkalti/backoff"
)

// dependency for managed instance registration
var managedInstance instanceRegistration = instanceInfo{}

var backoffRetry = backoff.Retry

type instanceRegistration interface {
	InstanceID(log.T) string
	Region(log.T) string
	PrivateKey(log.T) string
	PrivateKeyType(log log.T) string
	Fingerprint(log.T) (string, error)
	GenerateKeyPair() (string, string, string, error)
	UpdatePrivateKey(log.T, string, string) error
	HasManagedInstancesCredentials(log.T) bool
	GeneratePublicKey(string) (string, error)
	ShouldRotatePrivateKey(log.T, int, bool) (bool, error)
}

type instanceInfo struct{}

// ServerID returns the managed instance ID
func (instanceInfo) InstanceID(log log.T) string { return registration.InstanceID(log) }

// Region returns the managed instance region
func (instanceInfo) Region(log log.T) string { return registration.Region(log) }

// PrivateKey returns the managed instance PrivateKey
func (instanceInfo) PrivateKey(log log.T) string { return registration.PrivateKey(log) }

// PrivateKey returns the managed instance PrivateKey
func (instanceInfo) PrivateKeyType(log log.T) string { return registration.PrivateKeyType(log) }

// Fingerprint returns the managed instance fingerprint
func (instanceInfo) Fingerprint(log log.T) (string, error) { return registration.Fingerprint(log) }

// GenerateKeyPair generate a new keypair
func (instanceInfo) GenerateKeyPair() (publicKey, privateKey, keyType string, err error) {
	return registration.GenerateKeyPair()
}

// UpdatePrivateKey saves the private key into the registration persistence store
func (instanceInfo) UpdatePrivateKey(log log.T, privateKey, privateKeyType string) (err error) {
	return registration.UpdatePrivateKey(log, privateKey, privateKeyType)
}

// HasManagedInstanceCredentials returns if the instance has registration
func (instanceInfo) HasManagedInstancesCredentials(log log.T) bool {
	return registration.HasManagedInstancesCredentials(log)
}

// ShouldRotatePrivateKey returns true of the age of the private key is greater or equal than argument.
func (instanceInfo) ShouldRotatePrivateKey(log log.T, privateKeyMaxDaysAge int, serviceSaysRotate bool) (bool, error) {
	return registration.ShouldRotatePrivateKey(log, privateKeyMaxDaysAge, serviceSaysRotate)
}

// GeneratePublicKey generate the public key of a provided private key
func (instanceInfo) GeneratePublicKey(privateKey string) (string, error) {
	return registration.GeneratePublicKey(privateKey)
}
