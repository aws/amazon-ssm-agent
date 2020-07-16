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
	"time"

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
	fingerprint string
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
	if value < 1 || value > 100 { // zero not allowed
		return fmt.Errorf("Invalid Similarity Threshold value of %v. Value must be between 0 and 100.", value)
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

	log := ssmlog.SSMLogger(true)
	uuid.SwitchFormat(uuid.CleanHyphen)
	result := ""
	threshold := minimumMatchPercent

	// retry getting the new hash and compare with the saved hash for 3 times
	for attempt := 1; attempt <= 3; attempt++ {
		// fetch current hardware hash values
		hardwareHash = currentHwHash()

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
			log.Debugf("No initial fingerprint detected, skipping retry...")
			break
		}

		// stop retry if the hardware hashes are the same
		if isSimilarHardwareHash(log, savedHwInfo.HardwareHash, hardwareHash, threshold) {
			log.Debugf("Calculated hardware hash is same as saved one, skipping retry...")
			break
		}

		log.Debugf("Calculated hardware hash is different with saved one, retry to ensure the difference is not cause by the dependency has not been ready")
		// sleep 5 seconds until the next retry
		time.Sleep(5 * time.Second)
	}
	if err != nil {
		log.Warnf("Error while fetching fingerprint data from vault: %s", err)
	}
	// check if this is the first time we are generating the fingerprint
	// or if there is no match
	if !hasFingerprint(savedHwInfo) {
		// generate new fingerprint
		log.Info("No initial fingerprint detected, generating fingerprint file...")
		result = uuid.NewV4().String()
	} else if !isSimilarHardwareHash(log, savedHwInfo.HardwareHash, hardwareHash, threshold) {
		log.Info("Calculated hardware difference, regenerating fingerprint...")
		result = uuid.NewV4().String()
	} else {
		result = savedHwInfo.Fingerprint
	}

	// generate updated info to save to vault
	updatedHwInfo := hwInfo{
		Fingerprint:         result,
		HardwareHash:        hardwareHash,
		SimilarityThreshold: threshold,
	}

	// save content in vault
	if err = save(updatedHwInfo); err != nil {
		log.Errorf("Error while saving fingerprint data from vault: %s", err)
	}
	return result, err
}

func fetch() (hwInfo, error) {
	log := ssmlog.SSMLogger(true)
	savedHwInfo := hwInfo{}

	// try get previously saved fingerprint data from vault
	d, err := vault.Retrieve(vaultKey)
	if err != nil {
		log.Infof("[Warning] Could not read InstanceFingerprint file: %v", err)
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

// isSimilarHardwareHash compares two maps of hashes, and returns true if the
// percentage of match is greater than or equals to the threshold provided.
// It returns false if any of the map is empty or the percentage of match is
// less than threshold.
func isSimilarHardwareHash(logger log.T, savedHwHash map[string]string, currentHwHash map[string]string, threshold int) bool {
	var totalCount, successCount int
	// check input
	if len(savedHwHash) == 0 || len(currentHwHash) == 0 {
		logger.Debugf("saved hash or current hash is empty")
		return false
	}

	// check whether hardwareId (uuid/machineid) has changed
	// this usually happens during provisioning
	if currentHwHash[hardwareID] != savedHwHash[hardwareID] {
		logger.Debugf("saved hardware hash is not equal to current")
		logger.Tracef("saved hardware hash, current hardware hash: /%v/, /%v/", savedHwHash[hardwareID], currentHwHash[hardwareID])
		return false
	}

	// check whether ipaddress has remained the same
	// this happens when the instance type is changed for the provisioned instance
	if currentHwHash[ipAddressID] == savedHwHash[ipAddressID] {
		logger.Debugf("saved ip address is equal to current")
		return true
	}

	// identify number of successful match
	for key, currValue := range currentHwHash {
		if prevValue, ok := savedHwHash[key]; ok && currValue == prevValue {
			successCount++
		} else {
			logger.Debugf("saved %v value changed/not present in the hardware hash", key)
		}
	}

	// check if the match exceeds the minimum match percent
	totalCount = len(currentHwHash)
	if float32(successCount)/float32(totalCount)*100 < float32(threshold) {
		logger.Debugf("match exceeds the minimum match percent")
		return false
	}

	return true
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
