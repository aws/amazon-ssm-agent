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

// package registration provides managed instance information
package registration

import (
	"encoding/json"
	"fmt"
	"log"
	"sync"

	"github.com/aws/amazon-ssm-agent/agent/fingerprint"
	"github.com/aws/amazon-ssm-agent/agent/managedInstances/auth"
)

type instanceInfo struct {
	InstanceID       string `json:"instanceID"`
	Region           string `json:"region"`
	InstanceType     string `json:"instanceType"`
	AvailabilityZone string `json:"availabilityZone"`
	PrivateKey       string `json:"privateKey"`
	PrivateKeyType   string `json:"privateKeyType"`
}

var (
	lock             sync.RWMutex
	loadedServerInfo instanceInfo
)

const (
	RegVaultKey = "RegistrationKey"
)

func init() {
	if err := loadServerInfo(); err != nil {
		log.Println(err)
	}
}

// InstanceID of the managed instance.
func InstanceID() string {
	instance := getInstanceInfo()
	return instance.InstanceID
}

// Region of the managed instance.
func Region() string {
	instance := getInstanceInfo()
	return instance.Region
}

// InstanceType of the managed instance.
func InstanceType() string {
	instance := getInstanceInfo()
	if instance.InstanceID != "" {
		return "on-premises"
	}

	return ""
}

// AvailabilityZone of the managed instance.
func AvailabilityZone() string {
	instance := getInstanceInfo()
	if instance.InstanceID != "" {
		return "on-premises"
	}

	return ""
}

// PrivateKey of the managed instance.
func PrivateKey() string {
	instance := getInstanceInfo()
	return instance.PrivateKey
}

// Fingerprint of the managed instance.
func Fingerprint() (string, error) {
	return fingerprint.InstanceFingerprint()
}

// HasManagedInstancesCredentials returns true when the valid registration information is present
func HasManagedInstancesCredentials() (bool, error) {
	info := getInstanceInfo()

	// check if we need to activate instance
	return info.PrivateKey != "" && info.Region != "" && info.InstanceID != "", nil
}

// UpdatePrivateKey saves the private key into the registration persistence store
func UpdatePrivateKey(privateKey, privateKeyType string) (err error) {
	info := getInstanceInfo()
	info.PrivateKey = privateKey
	info.PrivateKeyType = privateKeyType
	return updateServerInfo(info)
}

// UpdateServerInfo saves the instance info into the registration persistence store
func UpdateServerInfo(instanceID, region, privateKey, privateKeyType string) (err error) {
	info := instanceInfo{
		InstanceID:     instanceID,
		Region:         region,
		PrivateKey:     privateKey,
		PrivateKeyType: privateKeyType,
	}
	return updateServerInfo(info)
}

// GenerateKeyPair generate a new keypair
func GenerateKeyPair() (publicKey, privateKey, keyType string, err error) {
	var keyPair auth.RsaKey

	keyPair, err = auth.CreateKeypair()
	if err != nil {
		return
	}

	privateKey, err = keyPair.EncodePrivateKey()
	if err != nil {
		return
	}

	publicKey, err = keyPair.EncodePublicKey()
	if err != nil {
		return
	}

	keyType = auth.KeyType
	return
}

func updateServerInfo(info instanceInfo) (err error) {
	lock.Lock()
	defer lock.Unlock()

	var data []byte
	if data, err = json.Marshal(info); err != nil {
		return fmt.Errorf("Failed to marshal instance info. %v", err)
	} else {
		//call vault apis here and update the refId
		if err = vault.Store(RegVaultKey, data); err != nil {
			return fmt.Errorf("Failed to store instance info in vault. %v", err)
		}
	}

	loadedServerInfo = info
	return
}

func loadServerInfo() error {
	lock.Lock()
	defer lock.Unlock()

	var info instanceInfo = instanceInfo{}
	if d, err := vault.Retrieve(RegVaultKey); err != nil {
		return fmt.Errorf("Failed to load instance info from vault. %v", err)
	} else {
		if err = json.Unmarshal(d, &info); err != nil {
			return fmt.Errorf("Failed to unmarshal instance info. %v", err)
		}
	}

	loadedServerInfo = info
	return nil
}

func getInstanceInfo() instanceInfo {
	lock.RLock()
	defer lock.RUnlock()

	return loadedServerInfo
}
