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
	"github.com/aws/amazon-ssm-agent/agent/log/ssmlog"
	"github.com/twinj/uuid"
)

type hwInfo struct {
	Fingerprint         string            `json:"fingerprint"`
	HardwareHash        map[string]string `json:"hardwareHash"`
	SimilarityThreshold int               `json:"similarityThreshold"`
}

const (
	minimumMatchPercent = 40
	vaultKey            = "InstanceFingerprint"
	ipAddressID         = "ipaddress-info"
)

var (
	fingerprint string = ""
	logger      log.T
	logLock     sync.RWMutex
)

func InstanceFingerprint() (string, error) {
	if isLoaded() {
		return fingerprint, nil
	}

	lock.Lock()
	defer lock.Unlock()

	var err error
	fingerprint, err = generateFingerprint()
	if err != nil {
		return "", err
	}

	loaded = true
	return fingerprint, nil
}

func SetSimilarityThreshold(value int) (err error) {
	if value < 1 || 100 < value { // zero not allowed
		return fmt.Errorf("Invalid Similarity Threshold value of %v. Value must be between 1 and 100.", value)
	}

	savedHwInfo := hwInfo{}
	if savedHwInfo, err = fetch(); err == nil {
		savedHwInfo.SimilarityThreshold = value
		err = save(savedHwInfo)
	}

	if err != nil {
		return fmt.Errorf("Unable to set similarity threshold due to, %v", err)
	}
	return nil
}

// generateFingerprint generates new fingerprint and saves it in the vault
func generateFingerprint() (string, error) {
	var hardwareHash map[string]string
	var savedHwInfo hwInfo
	var err error
	var hwHashErr error

	logger := getLogger()

	uuid.SwitchFormat(uuid.CleanHyphen)
	result := ""
	threshold := minimumMatchPercent

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
		savedHwInfo, err = fetch()
		if err != nil {
			continue
		}

		if savedHwInfo.SimilarityThreshold >= 0 {
			threshold = savedHwInfo.SimilarityThreshold
		}

		// first time generation, breakout retry
		if !hasFingerprint(savedHwInfo) {
			logger.Debugf("No initial fingerprint detected, skipping retry...")
			break
		}

		// stop retry if the hardware hashes are the same
		if isSimilarHardwareHash(logger, savedHwInfo.HardwareHash, hardwareHash, threshold) {
			logger.Debugf("Calculated hardware hash is same as saved one, skipping retry...")
			break
		}

		logger.Debugf("Calculated hardware hash is different with saved one, retry to ensure the difference is not cause by the dependency has not been ready")
		// sleep 5 seconds until the next retry
		time.Sleep(5 * time.Second)
	}

	if hwHashErr != nil {
		logger.Errorf("Error while fetching hardware hashes from instance: %s", hwHashErr)
		return result, hwHashErr
	} else if !isValidHardwareHash(hardwareHash) {
		return result, fmt.Errorf("Hardware hash generated contains invalid characters. %s", hardwareHash)
	}

	if err != nil {
		logger.Warnf("Error while fetching fingerprint data from vault: %s", err)
	}

	// check if this is the first time we are generating the fingerprint
	// or if there is no match
	if !hasFingerprint(savedHwInfo) {
		// generate new fingerprint
		logger.Info("No initial fingerprint detected, generating fingerprint file...")
		result = uuid.NewV4().String()
	} else if !isSimilarHardwareHash(logger, savedHwInfo.HardwareHash, hardwareHash, threshold) {
		logger.Info("Calculated hardware difference, regenerating fingerprint...")
		result = uuid.NewV4().String()
	} else {
		result = savedHwInfo.Fingerprint
		return result, nil
	}

	// generate updated info to save to vault
	updatedHwInfo := hwInfo{
		Fingerprint:         result,
		HardwareHash:        hardwareHash,
		SimilarityThreshold: threshold,
	}

	// save content in vault
	if err = save(updatedHwInfo); err != nil {
		logger.Errorf("Error while saving fingerprint data from vault: %s", err)
	}
	return result, err
}

func fetch() (hwInfo, error) {
	logger = getLogger()
	savedHwInfo := hwInfo{}

	// try get previously saved fingerprint data from vault
	d, err := vault.Retrieve(vaultKey)
	if err != nil {
		_ = logger.Warnf("Could not read InstanceFingerprint file: %v", err)
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
func isSimilarHardwareHash(logger log.T, savedHwHash map[string]string, currentHwHash map[string]string, threshold int) bool {

	var totalCount, successCount int
	isSimilar := true

	// check input
	if len(savedHwHash) == 0 || len(currentHwHash) == 0 {

		_ = logger.Errorf(
			"Cannot connect to AWS Systems Manager.  " +
				"The saved machine configuration could not be loaded or the current machine configuration could not be determined.")

		return false
	}

	// check whether hardwareId (uuid/machineid) has changed
	// this usually happens during provisioning
	if currentHwHash[hardwareID] != savedHwHash[hardwareID] {

		_ = logger.Errorf(
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

			logger.Debugf(matchedValueFormat, "IP Address")

		} else {

			message := fmt.Sprintf(unmatchedValueFormat, "IP Address", currentHwHash[ipAddressID], savedHwHash[ipAddressID])
			mismatchedKeyMessages = append(mismatchedKeyMessages, message)
			logger.Debug(message)

			// identify number of successful matches
			for key, currValue := range currentHwHash {

				if prevValue, ok := savedHwHash[key]; ok && currValue == prevValue {

					logger.Debugf(matchedValueFormat, key)
					successCount++

				} else {

					message := fmt.Sprintf(unmatchedValueFormat, key, currValue, prevValue)
					mismatchedKeyMessages = append(mismatchedKeyMessages, message)
					logger.Debug(message)
				}
			}

			// check if the changed match exceeds the minimum match percent
			totalCount = len(currentHwHash)
			if float32(successCount)/float32(totalCount)*100 < float32(threshold) {

				_ = logger.Error("Cannot connect to AWS Systems Manager.  Machine configuration has changed more than the allowed threshold.")
				for _, message := range mismatchedKeyMessages {

					_ = logger.Warn(message)
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

// getLogger returns the logger for the component.
// If the logger instance has not been created, a default SSMLogger is created.
func getLogger() log.T {
	if logger == nil {

		logLock.RLock()
		defer logLock.RUnlock()

		// in a race, another thread may have already set the logger instance before this thread acquired the lock
		if logger == nil {
			logger = ssmlog.SSMLogger(true)
		}
	}

	return logger
}

// setLogger sets the logger for the component to a non-default logger.
// This method is used by tests to inject a fake or mock logger that doesn't write to the file system.
func setLogger(newLogger log.T) {
	logLock.Lock()
	defer logLock.Unlock()
	logger = newLogger
}
