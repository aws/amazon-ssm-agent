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
	"net"
	"os"
	"os/exec"

	"fmt"

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
	uuid.SwitchFormat(uuid.CleanHyphen)
	result := ""

	// fetch current hardware hash values
	hardwareHash := currentHwHash()

	// try get previously saved fingerprint data from vault
	savedHwInfo, err := fetch()
	if err != nil {
		return "", err
	}

	threshold := minimumMatchPercent
	if savedHwInfo.SimilarityThreshold >= 0 {
		threshold = savedHwInfo.SimilarityThreshold
	}

	// check if this is the first time we are generating the fingerprint
	// or if there is no match
	if savedHwInfo.Fingerprint == "" || !isSimilarHardwareHash(savedHwInfo.HardwareHash, hardwareHash, threshold) {
		// generate new fingerprint
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
	save(updatedHwInfo)
	return result, nil
}

func fetch() (hwInfo, error) {
	savedHwInfo := hwInfo{}

	// try get previously saved fingerprint data from vault
	d, err := vault.Retrieve(vaultKey)
	if err != nil || d == nil {
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

// isSimilarHardwareHash compares two maps of hashes, and returns true if the
// percentage of match is greater than or equals to the threshold provided.
// It returns false if any of the map is empty or the percentage of match is
// less than threshold.
func isSimilarHardwareHash(savedHwHash map[string]string, currentHwHash map[string]string, threshold int) bool {
	var totalCount, successCount int
	// check input
	if len(savedHwHash) == 0 || len(currentHwHash) == 0 {
		return false
	}

	// check whether hardwareId (uuid/machineid) has changed
	// this usually happens during provisioning
	if currentHwHash[hardwareID] != savedHwHash[hardwareID] {
		return false
	}

	// check whether ipaddress has remained the same
	// this happens when the instance type is changed for the provisioned instance
	if currentHwHash[ipAddressID] == savedHwHash[ipAddressID] {
		return true
	}

	// identify number of successful match
	for key, currValue := range currentHwHash {
		if prevValue, ok := savedHwHash[key]; ok && currValue == prevValue {
			successCount++
		}
	}

	// check if the match exceeds the minimum match percent
	totalCount = len(currentHwHash)
	if float32(successCount)/float32(totalCount)*100 < float32(threshold) {
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

func commandOutputHash(command string, params ...string) (value string, err error) {
	var contentBytes []byte
	if contentBytes, err = exec.Command(command, params...).Output(); err == nil {
		sum := md5.Sum(contentBytes)
		value = base64.StdEncoding.EncodeToString(sum[:])
	}
	return
}
