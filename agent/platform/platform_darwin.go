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

//go:build darwin
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
	"time"

	"github.com/aws/amazon-ssm-agent/agent/log"
)

const (
	platformDetailsCommand = "sw_vers"
)

func isPlatformWindowsServer2012OrEarlier(_ log.T) (bool, error) {
	return false, nil
}

func isPlatformWindowsServer2025OrLater(_ log.T) (bool, error) {
	return false, nil
}

func isWindowsServer2025OrLater(_ string, _ log.T) (bool, error) {
	return false, nil
}

func getPlatformData(log log.T) (PlatformData, error) {
	platformName, platformVersion, err := getPlatformDetails(log)
	return PlatformData{
		Name:    platformName,
		Version: platformVersion,
		Type:    "macos",
	}, err
}

var execWithTimeout = func(cmd string, param ...string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	return exec.CommandContext(ctx, cmd, param...).Output()
}

func getPlatformDetails(log log.T) (name string, version string, err error) {
	var contentsBytes []byte
	name, version = notAvailableMessage, notAvailableMessage

	if contentsBytes, err = execWithTimeout(platformDetailsCommand); err != nil {
		log.Errorf("Failed to query for platform info: %v", err)
		return name, version, err
	}

	platformString := strings.TrimSpace(string(contentsBytes))
	if len(platformString) == 0 {
		return name, version, fmt.Errorf("received empty string when querying for platform info")
	}

	log.Debugf("queried for platform info: %s", platformString)
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

		if platformInfoKey == "ProductName" {
			name = platformInfoVal
		} else if platformInfoKey == "ProductVersion" {
			version = platformInfoVal
		}
	}

	return name, version, err
}

func initSystemInfoCache(_ log.T, _ string) (string, error) { return "", nil }

var hostNameCommand = filepath.Join("/bin", "hostname")

// fullyQualifiedDomainName returns the Fully Qualified Domain Name of the instance, otherwise the hostname
func fullyQualifiedDomainName(_ log.T) string {
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

func isPlatformNanoServer(_ log.T) (bool, error) {
	return false, nil
}

func SetOOMScoreAdjust(_ log.T) {
	// No-op on macOS
}
