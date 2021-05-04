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

// Package fingerprint contains functions that helps identify an instance
// this is done to protect customers from launching two instances with the same instance identifier
// and thus running commands intended for one on the other
package fingerprint

import (
	"crypto/md5"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/twinj/uuid"
)

type hwInfo struct {
	Fingerprint         string            `json:"fingerprint"`
	HardwareHash        map[string]string `json:"hardwareHash"`
	SimilarityThreshold int               `json:"similarityThreshold"`
}

const (
	defaultMatchPercent = 40
	vaultKey            = "InstanceFingerprint"
	ipAddressID         = "ipaddress-info"
)

var (
	fingerprint string = ""
	logger      log.T
	logLock     sync.RWMutex
)

func InstanceFingerprint(log log.T) (string, error) {
	if isLoaded() {
		return fingerprint, nil
	}

	lock.Lock()
	defer lock.Unlock()

	var err error
	fingerprint, err = generateFingerprint(log)
	if err != nil {
		return "", err
	}

	loaded = true
	return fingerprint, nil
}

func SetSimilarityThreshold(log log.T, value int) (err error) {
	if value != -1 && (value < 1 || 100 < value) { // zero not allowed
		return fmt.Errorf("Invalid Similarity Threshold value of %v. Value must be between 1 and 100 or -1 (check disabled)", value)
	}

	savedHwInfo := hwInfo{}
	if savedHwInfo, err = fetch(log); err == nil {
		savedHwInfo.SimilarityThreshold = value
		err = save(savedHwInfo)
	}

	if err != nil {
		return fmt.Errorf("Unable to set similarity threshold due to, %v", err)
	}
	return nil
}

// generateFingerprint generates new fingerprint and saves it in the vault
func generateFingerprint(log log.T) (string, error) {
	var hardwareHash map[string]string
	var savedHwInfo hwInfo
	var err error
	var hwHashErr error

	// retry getting the new hash and compare with the saved hash for 3 times
	for attempt := 1; attempt <= 3; attempt++ {
		// fetch current hardware hash values
		hardwareHash, hwHashErr = currentHwHash()

		if hwHashErr != nil || !isValidHardwareHash(hardwareHash) {
			// sleep 5 seconds until the next retry
			time.Sleep(5 * time.Second)
			continue
		}

		// try get previously saved fingerprint data from vault
		savedHwInfo, err = fetch(log)
		if err != nil {
			continue
		}

		// first time generation, breakout retry
		if !hasFingerprint(savedHwInfo) {
			log.Debugf("No initial fingerprint detected, skipping retry...")
			// Set the default similarity threshold during first time generation
			savedHwInfo.SimilarityThreshold = defaultMatchPercent
			break
		}

		// stop retry if the hardware hashes are the same
		if isSimilarHardwareHash(log, savedHwInfo.HardwareHash, hardwareHash, savedHwInfo.SimilarityThreshold) {
			log.Debugf("Calculated hardware hash is same as saved one, returning fingerprint")
			return savedHwInfo.Fingerprint, nil
		}

		log.Debugf("Calculated hardware hash is different with saved one, retry to ensure the difference is not cause by the dependency has not been ready")
		// sleep 5 seconds until the next retry
		time.Sleep(5 * time.Second)
	}

	if hwHashErr != nil {
		log.Errorf("Error while fetching hardware hashes from instance: %s", hwHashErr)
		return "", hwHashErr
	} else if !isValidHardwareHash(hardwareHash) {
		return "", fmt.Errorf("Hardware hash generated contains invalid characters. %s", hardwareHash)
	}

	if err != nil {
		log.Warnf("Error while fetching fingerprint data from vault: %s", err)
	}

	uuid.SwitchFormat(uuid.CleanHyphen)
	// check if this is the first time we are generating the fingerprint
	// or if there is no match
	new_fingerprint := ""
	if !hasFingerprint(savedHwInfo) {
		// generate new fingerprint
		log.Info("No initial fingerprint detected, generating fingerprint file...")
		new_fingerprint = uuid.NewV4().String()
	} else if !isSimilarHardwareHash(log, savedHwInfo.HardwareHash, hardwareHash, savedHwInfo.SimilarityThreshold) {
		log.Info("Calculated hardware difference, regenerating fingerprint...")
		new_fingerprint = uuid.NewV4().String()
	} else {
		return savedHwInfo.Fingerprint, nil
	}

	// generate updated info to save to vault
	updatedHwInfo := hwInfo{
		Fingerprint:         new_fingerprint,
		HardwareHash:        hardwareHash,
		SimilarityThreshold: savedHwInfo.SimilarityThreshold,
	}

	// save content in vault
	if err = save(updatedHwInfo); err != nil {
		log.Errorf("Error while saving fingerprint data from vault: %s", err)
	}
	return new_fingerprint, err
}

func fetch(log log.T) (hwInfo, error) {
	savedHwInfo := hwInfo{}

	// try get previously saved fingerprint data from vault
	d, err := vault.Retrieve(vaultKey)
	if err != nil {
		_ = log.Warnf("Could not read InstanceFingerprint file: %v", err)
		return hwInfo{}, nil
	} else if d == nil {
		return hwInfo{}, nil
	}

	// unmarshal the retrieved data
	if err := json.Unmarshal([]byte(d), &savedHwInfo); err != nil {
		return hwInfo{}, err
	}
	return savedHwInfo, nil
}

func save(info hwInfo) error {
	// check fingerprint
	if info.Fingerprint == "" {
		return errors.New("save called with empty fingerprint key")
	}

	// marshal the updated info
	data, err := json.Marshal(info)
	if err != nil {
		return err
	}

	// save content in vault
	if err = vault.Store(vaultKey, data); err != nil {
		return err
	}

	return nil
}

func hasFingerprint(info hwInfo) bool {
	return info.Fingerprint != ""
}

// isSimilarHardwareHash returns true if the VM or container running this instance is the same one that was registered
// with Systems Manager; otherwise, false.
//
// If the current machine ID (SID) and IP Address are the same as when the agent was registered, then this is definitely
// the same machine.  If the SID is different, then this is definitely *not* the same instance.  If the IP Address has
// changed, then this *might* be the same machine.
//
// IP Address can change if the VM moves to a new host or just randomly if a DHCP server assigns a new address.
// If the IP address is changed we look at other machine configuration values to decide whether the instance is
// *probably* the same.  How many configuration values can be different and still be the "same machine" is controlled
// by the threshold value.
//
// logger is the application log writer
// savedHwHash is a map of machine property names to their values when the agent was registered
// currentHwHash is a map of machine property names to their current values
// threshold is the percentage of machine properties (other than SID and IP Address) that have to match for the instance
// to be considered the same
func isSimilarHardwareHash(log log.T, savedHwHash map[string]string, currentHwHash map[string]string, threshold int) bool {

	var totalCount, successCount int
	isSimilar := true

	// similarity check is disabled when threshold is set to -1
	if threshold == -1 {
		log.Debugf("Similarity check is disabled, skipping hardware comparison")
		return true
	}

	// check input
	if len(savedHwHash) == 0 || len(currentHwHash) == 0 {

		_ = log.Errorf(
			"Cannot connect to AWS Systems Manager.  " +
				"The saved machine configuration could not be loaded or the current machine configuration could not be determined.")

		return false
	}

	// check whether hardwareId (uuid/machineid) has changed
	// this usually happens during provisioning
	if currentHwHash[hardwareID] != savedHwHash[hardwareID] {

		_ = log.Errorf(
			"Cannot connect to AWS Systems Manager.  The hardware ID (%v) has changed from the registered value (%v).",
			currentHwHash[hardwareID],
			savedHwHash[hardwareID])

		isSimilar = false

	} else {

		mismatchedKeyMessages := make([]string, 0, len(currentHwHash))
		const unmatchedValueFormat = "The '%s' value (%s) has changed from the registered machine configuration value (%s)."
		const matchedValueFormat = "The '%s' value matches the registered machine configuration value."

		// check whether ipaddress is the same - if the machine key and the IP address have not changed, it's the same instance.
		if currentHwHash[ipAddressID] == savedHwHash[ipAddressID] {

			log.Debugf(matchedValueFormat, "IP Address")

		} else {

			message := fmt.Sprintf(unmatchedValueFormat, "IP Address", currentHwHash[ipAddressID], savedHwHash[ipAddressID])
			mismatchedKeyMessages = append(mismatchedKeyMessages, message)
			log.Debug(message)

			// identify number of successful matches
			for key, currValue := range currentHwHash {

				if prevValue, ok := savedHwHash[key]; ok && currValue == prevValue {

					log.Debugf(matchedValueFormat, key)
					successCount++

				} else {

					message := fmt.Sprintf(unmatchedValueFormat, key, currValue, prevValue)
					mismatchedKeyMessages = append(mismatchedKeyMessages, message)
					log.Debug(message)
				}
			}

			// check if the changed match exceeds the minimum match percent
			totalCount = len(currentHwHash)
			if float32(successCount)/float32(totalCount)*100 < float32(threshold) {

				_ = log.Error("Cannot connect to AWS Systems Manager.  Machine configuration has changed more than the allowed threshold.")
				for _, message := range mismatchedKeyMessages {

					_ = log.Warn(message)
				}

				isSimilar = false
			}
		}
	}

	return isSimilar
}

func hostnameInfo() (value string, err error) {
	return os.Hostname()
}

func primaryIpInfo() (value string, err error) {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return "", err
	}
	for _, address := range addrs {
		// check the address type and if it is not a loopback then return it
		if ipnet, ok := address.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
			if ipnet.IP.To4() != nil {
				return ipnet.IP.String(), nil
			}
		}
	}
	return "", err
}

func macAddrInfo() (value string, err error) {
	ifaces, err := net.Interfaces()
	if err != nil {
		return "", err
	}

	for _, i := range ifaces {
		if i.HardwareAddr.String() != "" {
			return i.HardwareAddr.String(), nil
		}
	}

	return "", nil
}

func commandOutputHash(command string, params ...string) (encodedValue string, value string, err error) {
	var contentBytes []byte
	if contentBytes, err = exec.Command(command, params...).Output(); err == nil {
		value = string(contentBytes) // without encoding
		sum := md5.Sum(contentBytes)
		encodedValue = base64.StdEncoding.EncodeToString(sum[:])
	}
	return
}

func isValidHardwareHash(hardwareHash map[string]string) bool {
	for _, value := range hardwareHash {
		if !utf8.ValidString(value) {
			return false
		}
	}

	return true
}

func ClearStoredHardwareInfo(log log.T) {
	// create empty hardware info
	data, err := json.Marshal(hwInfo{})
	if err != nil {
		log.Errorf("Failed to create empty hardware info: %v", err)
		return
	}

	// save content in vault
	if err = vault.Store(vaultKey, data); err != nil {
		log.Errorf("Failed to store empty hardware info: %v", err)
	}
}
