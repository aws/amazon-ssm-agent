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

// +build darwin

// Package platform contains platform specific utilities.
package platform

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/aws/amazon-ssm-agent/agent/log"
)

const (
	platformDetailsCommand = "sw_vers"
)

var platformInfoMap = map[string]string{}
var platformQueryMutex = sync.Mutex{}

func getPlatformName(log log.T) (value string, err error) {
	value, err = getPlatformDetail(log, "ProductName")
	log.Debugf("platform name: %v", value)
	return
}

func getPlatformType(log log.T) (value string, err error) {
	return "macos", nil
}

func getPlatformVersion(log log.T) (value string, err error) {
	value, err = getPlatformDetail(log, "ProductVersion")
	log.Debugf("platform version: %v", value)
	return
}

func getPlatformSku(log log.T) (value string, err error) {
	return
}

var execWithTimeout = func(cmd string, param ...string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	return exec.CommandContext(ctx, cmd, param...).Output()
}

func getPlatformDetail(log log.T, param string) (value string, err error) {
	var contentsBytes []byte
	platformQueryMutex.Lock()
	defer platformQueryMutex.Unlock()

	if mapVal, ok := platformInfoMap[param]; ok {
		return mapVal, nil
	}

	if contentsBytes, err = execWithTimeout(platformDetailsCommand); err != nil {
		log.Errorf("Failed to query for platform info: %v", err)
		return notAvailableMessage, err
	}

	platformString := strings.TrimSpace(string(contentsBytes))
	if len(platformString) == 0 {
		return notAvailableMessage, fmt.Errorf("received empty string when querying for platform info")
	}

	log.Debugf("queried for platform info: %s", platformString)
	platformInfoMap = map[string]string{}
	for _, platformLine := range strings.Split(platformString, "\n") {
		if len(platformLine) == 0 {
			continue
		}

		platformLineSplit := strings.Split(platformLine, ":")

		if len(platformLineSplit) < 2 {
			log.Warnf("Unexpected line when parsing darwin platform: %s", platformLine)
			continue
		}

		platformInfoKey := strings.TrimSpace(platformLineSplit[0])
		platformInfoVal := strings.TrimSpace(platformLineSplit[1])
		platformInfoMap[platformInfoKey] = platformInfoVal
	}

	if mapVal, ok := platformInfoMap[param]; ok {
		return mapVal, err
	}

	log.Warnf("Failed to parse platform info for %s in string\n%s", param, platformString)
	return notAvailableMessage, fmt.Errorf("failed to find platform key")
}

var hostNameCommand = filepath.Join("/bin", "hostname")

// fullyQualifiedDomainName returns the Fully Qualified Domain Name of the instance, otherwise the hostname
func fullyQualifiedDomainName(log log.T) string {
	var hostName, fqdn string
	var err error

	if hostName, err = os.Hostname(); err != nil {
		return ""
	}

	var contentBytes []byte
	if contentBytes, err = execWithTimeout(hostNameCommand, "-f"); err == nil {
		fqdn = string(contentBytes)
		//trim whitespaces - since by default above command appends '\n' at the end.
		//e.g: 'ip-172-31-7-113.ec2.internal\n'
		fqdn = strings.TrimSpace(fqdn)
	}

	if fqdn != "" {
		return fqdn
	}

	return strings.TrimSpace(hostName)
}

func isPlatformNanoServer(log log.T) (bool, error) {
	return false, nil
}
