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
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/amazon-ssm-agent/agent/managedInstances/auth"
	"github.com/aws/amazon-ssm-agent/agent/managedInstances/fingerprint"
)

type instanceInfo struct {
	InstanceID            string `json:"instanceID"`
	Region                string `json:"region"`
	InstanceType          string `json:"instanceType"`
	AvailabilityZone      string `json:"availabilityZone"`
	PrivateKey            string `json:"privateKey"`
	PrivateKeyType        string `json:"privateKeyType"`
	PrivateKeyCreatedDate string `json:"privateKeyCreatedDate"`
}

var (
	lock             sync.RWMutex
	loadedServerInfo instanceInfo
)

const (
	RegVaultKey = "RegistrationKey"

	// If not date is stored with private key, the default age is 10 years
	defaultPrivateKeyAgeInDays = 3650
	defaultDateStringFormat    = "2006-01-02 15:04:05.999999999 -0700 MST"
)

// InstanceID of the managed instance.
func InstanceID(log log.T) string {
	instance := getInstanceInfo(log)
	return instance.InstanceID
}

// Region of the managed instance.
func Region(log log.T) string {
	instance := getInstanceInfo(log)
	return instance.Region
}

// PrivateKey of the managed instance.
func PrivateKey(log log.T) string {
	instance := getInstanceInfo(log)
	return instance.PrivateKey
}

// PrivateKeyType of the managed instance.
func PrivateKeyType(log log.T) string {
	instance := getInstanceInfo(log)
	return instance.PrivateKeyType
}

// Fingerprint of the managed instance.
func Fingerprint(log log.T) (string, error) {
	return fingerprint.InstanceFingerprint(log)
}

// HasManagedInstancesCredentials returns true when the valid registration information is present
func HasManagedInstancesCredentials(log log.T) bool {
	info := getInstanceInfo(log)

	// check if we need to activate instance
	return info.PrivateKey != "" && info.Region != "" && info.InstanceID != ""
}

// UpdatePrivateKey saves the private key into the registration persistence store
func UpdatePrivateKey(log log.T, privateKey, privateKeyType string) (err error) {
	info := getInstanceInfo(log)
	info.PrivateKey = privateKey
	info.PrivateKeyType = privateKeyType
	info.PrivateKeyCreatedDate = time.Now().Format(defaultDateStringFormat)
	return updateServerInfo(info)
}

func ShouldRotatePrivateKey(log log.T, privateKeyMaxDaysAge int, serviceSaysRotate bool) (bool, error) {
	// only ssm-agent-worker should rotate private key to reduce chances of race condition
	if !strings.HasPrefix(filepath.Base(os.Args[0]), "ssm-agent-worker") {
		return false, nil
	}

	// check if service tells agent to rotate
	if serviceSaysRotate {
		return true, nil
	}

	// If max age is less or equal to 0, rotation is off
	if privateKeyMaxDaysAge <= 0 {
		return false, nil
	}
	info := getInstanceInfo(log)

	keyAgeInDays := defaultPrivateKeyAgeInDays
	if info.PrivateKeyCreatedDate != "" {
		// Parse stored time using default time format
		date, err := time.Parse(defaultDateStringFormat, info.PrivateKeyCreatedDate)

		if err != nil {
			return false, err
		}
		keyAgeInDays = int(time.Since(date).Hours() / 24)
	}

	return keyAgeInDays >= privateKeyMaxDaysAge, nil
}

func GeneratePublicKey(privateKey string) (publicKey string, err error) {
	var rsaKey auth.RsaKey
	rsaKey, err = auth.DecodePrivateKey(privateKey)
	if err != nil {
		return
	}

	return rsaKey.EncodePublicKey()
}

// UpdateServerInfo saves the instance info into the registration persistence store
func UpdateServerInfo(instanceID, region, privateKey, privateKeyType string) (err error) {
	info := instanceInfo{
		InstanceID:            instanceID,
		Region:                region,
		PrivateKey:            privateKey,
		PrivateKeyType:        privateKeyType,
		PrivateKeyCreatedDate: time.Now().Format(defaultDateStringFormat),
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

func loadServerInfo() (loadErr error) {
	lock.Lock()
	defer lock.Unlock()

	var info instanceInfo = instanceInfo{}

	if !vault.IsManifestExists() {
		loadedServerInfo = info
		return nil
	}
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

func getInstanceInfo(log log.T) instanceInfo {
	if loadedServerInfo.InstanceID == "" {
		if err := loadServerInfo(); err != nil {
			log.Warnf("error while loading server info", err)
		}
	}
	lock.RLock()
	defer lock.RUnlock()
	return loadedServerInfo
}
