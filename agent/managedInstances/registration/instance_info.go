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
	lock                       = sync.RWMutex{}
	loadedServerInfo           instanceInfo
	loadedServerInfoKey        string
	loadedServerManifestPrefix string
)

const (
	RegVaultKey             = "RegistrationKey"
	EC2RegistrationVaultKey = "EC2RegistrationKey"

	// If not date is stored with private key, the default age is 10 years
	defaultPrivateKeyAgeInDays = 3650
	defaultDateStringFormat    = "2006-01-02 15:04:05.999999999 -0700 MST"
)

// InstanceID of the managed instance.
func InstanceID(log log.T, manifestFileNamePrefix, vaultKey string) string {
	instance := getInstanceInfo(log, manifestFileNamePrefix, vaultKey)
	return instance.InstanceID
}

// Region of the managed instance.
func Region(log log.T, manifestFileNamePrefix, vaultKey string) string {
	instance := getInstanceInfo(log, manifestFileNamePrefix, vaultKey)
	return instance.Region
}

// PrivateKey of the managed instance.
func PrivateKey(log log.T, manifestFileNamePrefix, vaultKey string) string {
	instance := getInstanceInfo(log, manifestFileNamePrefix, vaultKey)
	return instance.PrivateKey
}

// PrivateKeyType of the managed instance.
func PrivateKeyType(log log.T, manifestFileNamePrefix, vaultKey string) string {
	instance := getInstanceInfo(log, manifestFileNamePrefix, vaultKey)
	return instance.PrivateKeyType
}

// Fingerprint of the managed instance.
func Fingerprint(log log.T) (string, error) {
	return fingerprint.InstanceFingerprint(log)
}

// HasManagedInstancesCredentials returns true when the valid registration information is present
func HasManagedInstancesCredentials(log log.T, manifestFileNamePrefix, vaultKey string) bool {
	info := getInstanceInfo(log, manifestFileNamePrefix, vaultKey)

	// check if we need to activate instance
	return info.PrivateKey != "" && info.Region != "" && info.InstanceID != ""
}

// UpdatePrivateKey saves the private key into the registration persistence store
func UpdatePrivateKey(log log.T, privateKey, privateKeyType, manifestFileNamePrefix, vaultKey string) (err error) {
	info := getInstanceInfo(log, manifestFileNamePrefix, vaultKey)
	info.PrivateKey = privateKey
	info.PrivateKeyType = privateKeyType
	info.PrivateKeyCreatedDate = time.Now().Format(defaultDateStringFormat)
	return updateServerInfo(info, "", vaultKey)
}

// ShouldRotatePrivateKey returns true if serviceSaysRotate or private key has surpassed privateKeyMaxDaysAge
func ShouldRotatePrivateKey(log log.T, executableToRotateKey string, privateKeyMaxDaysAge int, serviceSaysRotate bool, manifestFileNamePrefix, vaultKey string) (bool, error) {
	// only one executable should rotate private key to reduce chances of race condition
	if !strings.HasPrefix(filepath.Base(os.Args[0]), executableToRotateKey) {
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
	info := getInstanceInfo(log, manifestFileNamePrefix, vaultKey)

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
func UpdateServerInfo(instanceID, region, privateKey, privateKeyType, manifestFileNamePrefix, vaultKey string) (err error) {
	info := instanceInfo{
		InstanceID:            instanceID,
		Region:                region,
		PrivateKey:            privateKey,
		PrivateKeyType:        privateKeyType,
		PrivateKeyCreatedDate: time.Now().Format(defaultDateStringFormat),
	}

	return updateServerInfo(info, manifestFileNamePrefix, vaultKey)
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

func updateServerInfo(info instanceInfo, manifestFileNamePrefix, vaultKey string) (err error) {
	lock.Lock()
	defer lock.Unlock()

	var data []byte
	if data, err = json.Marshal(info); err != nil {
		return fmt.Errorf("failed to marshal instance info. %v", err)
	}

	//call vault apis here and update the refId
	if err = vault.Store(manifestFileNamePrefix, vaultKey, data); err != nil {
		return fmt.Errorf("failed to store instance info in vault. %v", err)
	}

	loadedServerInfo = info
	loadedServerInfoKey = vaultKey
	loadedServerManifestPrefix = manifestFileNamePrefix
	return
}

func loadServerInfo(manifestFileNamePrefix, vaultKey string) (loadErr error) {
	lock.RLock()
	defer lock.RUnlock()

	var info instanceInfo = instanceInfo{}

	if !vault.IsManifestExists(manifestFileNamePrefix) {
		loadedServerInfo = info
		return nil
	}

	if d, err := vault.Retrieve(manifestFileNamePrefix, vaultKey); err != nil {
		return fmt.Errorf("Failed to load instance info from vault. %v", err)
	} else {
		if err = json.Unmarshal(d, &info); err != nil {
			return fmt.Errorf("Failed to unmarshal instance info. %v", err)
		}
	}

	loadedServerInfo = info
	loadedServerInfoKey = vaultKey
	loadedServerManifestPrefix = manifestFileNamePrefix
	return nil
}

func getInstanceInfo(log log.T, manifestFileNamePrefix, vaultKey string) instanceInfo {
	if loadedServerInfo.InstanceID == "" || loadedServerManifestPrefix != manifestFileNamePrefix || loadedServerInfoKey != vaultKey {
		if err := loadServerInfo(manifestFileNamePrefix, vaultKey); err != nil {
			log.Warnf("Error while loading server info %v", err)
		}
	}

	return loadedServerInfo
}

func ReloadInstanceInfo(log log.T, manifestFileNamePrefix, vaultKey string) {
	if err := loadServerInfo(manifestFileNamePrefix, vaultKey); err != nil {
		log.Warnf("Error while loading server info %v", err)
	}
}

// Temp moved here, should refactor entire file to be behind interface
func NewOnpremRegistrationInfo() IOnpremRegistrationInfo {
	return onpremRegistation{}
}

type IOnpremRegistrationInfo interface {
	InstanceID(log.T, string, string) string
	Region(log.T, string, string) string
	PrivateKey(log.T, string, string) string
	PrivateKeyType(log.T, string, string) string
	Fingerprint(log.T) (string, error)
	GenerateKeyPair() (string, string, string, error)
	UpdatePrivateKey(log.T, string, string, string, string) error
	HasManagedInstancesCredentials(log.T, string, string) bool
	GeneratePublicKey(string) (string, error)
	ShouldRotatePrivateKey(log.T, string, int, bool, string, string) (bool, error)
	ReloadInstanceInfo(log.T, string, string)
}

type onpremRegistation struct{}

// InstanceID returns the managed instance ID
func (onpremRegistation) InstanceID(log log.T, manifestFileNamePrefix, vaultKey string) string {
	return InstanceID(log, manifestFileNamePrefix, vaultKey)
}

// Region returns the managed instance region
func (onpremRegistation) Region(log log.T, manifestFileNamePrefix, vaultKey string) string {
	return Region(log, manifestFileNamePrefix, vaultKey)
}

// PrivateKey returns the managed instance PrivateKey
func (onpremRegistation) PrivateKey(log log.T, manifestFileNamePrefix, vaultKey string) string {
	return PrivateKey(log, manifestFileNamePrefix, vaultKey)
}

// PrivateKeyType returns the managed instance PrivateKey
func (onpremRegistation) PrivateKeyType(log log.T, manifestFileNamePrefix, vaultKey string) string {
	return PrivateKeyType(log, manifestFileNamePrefix, vaultKey)
}

// Fingerprint returns the managed instance fingerprint
func (onpremRegistation) Fingerprint(log log.T) (string, error) { return Fingerprint(log) }

// GenerateKeyPair generate a new keypair
func (onpremRegistation) GenerateKeyPair() (publicKey, privateKey, keyType string, err error) {
	return GenerateKeyPair()
}

// UpdatePrivateKey saves the private key into the registration persistence store
func (onpremRegistation) UpdatePrivateKey(log log.T, privateKey, privateKeyType, manifestFileNamePrefix, vaultKey string) (err error) {
	return UpdatePrivateKey(log, privateKey, privateKeyType, manifestFileNamePrefix, vaultKey)
}

// HasManagedInstancesCredentials returns if the instance has registration
func (onpremRegistation) HasManagedInstancesCredentials(log log.T, manifestFileNamePrefix, vaultKey string) bool {
	return HasManagedInstancesCredentials(log, manifestFileNamePrefix, vaultKey)
}

// ShouldRotatePrivateKey returns true of the age of the private key is greater or equal than argument.
func (onpremRegistation) ShouldRotatePrivateKey(log log.T, executableToRotateKey string, privateKeyMaxDaysAge int, serviceSaysRotate bool, manifestFileNamePrefix, vaultKey string) (bool, error) {
	return ShouldRotatePrivateKey(log, executableToRotateKey, privateKeyMaxDaysAge, serviceSaysRotate, manifestFileNamePrefix, vaultKey)
}

// GeneratePublicKey generate the public key of a provided private key
func (onpremRegistation) GeneratePublicKey(privateKey string) (string, error) {
	return GeneratePublicKey(privateKey)
}

// ReloadInstanceInfo reloads instance info from disk
func (onpremRegistation) ReloadInstanceInfo(log log.T, manifestFileNamePrefix string, vaultKey string) {
	ReloadInstanceInfo(log, manifestFileNamePrefix, vaultKey)
}
