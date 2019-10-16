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

// +build windows

// package fingerprint contains functions that helps identify an instance
// hardwareInfo contains platform specific way of fetching the hardware hash
package fingerprint

import (
	"path/filepath"
	"time"

	"github.com/aws/amazon-ssm-agent/agent/appconfig"
	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/amazon-ssm-agent/agent/log/ssmlog"

	"golang.org/x/sys/windows/svc"
	"golang.org/x/sys/windows/svc/mgr"
)

const (
	hardwareID     = "uuid"
	wmiServiceName = "Winmgmt"

	serviceRetryInterval = 15 // Seconds
	serviceRetry         = 10
)

func isServiceAvailable(log log.T, service *mgr.Service) bool {
	for attempt := 1; attempt <= serviceRetry; attempt++ {
		status, err := service.Query()
		if err == nil && status.State == svc.Running {
			return true
		}

		if err != nil {
			log.Debugf("Attempt %d: Failed to get WMI service status: %v", attempt, err)
		} else {
			log.Debugf("Attempt %d: WMI not running - Current status: %v", attempt, status.State)
		}
		time.Sleep(serviceRetryInterval * time.Second)
	}

	return false
}

var wmicCommand = filepath.Join(appconfig.EnvWinDir, "System32", "wbem", "wmic.exe")

var currentHwHash = func() map[string]string {
	log := ssmlog.SSMLogger(true)
	hardwareHash := make(map[string]string)

	winManager, err := mgr.Connect()
	if err != nil {
		log.Errorf("Something went wrong while trying to connect to Service Manager - %v", err)
		return hardwareHash
	}

	var wmiService *mgr.Service
	wmiService, err = winManager.OpenService(wmiServiceName)

	if err != nil {
		log.Errorf("Opening WMIC Service failed with error %v", err)
		return hardwareHash
	}

	if !isServiceAvailable(log, wmiService) {
		return hardwareHash
	}

	hardwareHash[hardwareID], _ = csproductUuid()
	hardwareHash["processor-hash"], _ = processorInfoHash()
	hardwareHash["memory-hash"], _ = memoryInfoHash()
	hardwareHash["bios-hash"], _ = biosInfoHash()
	hardwareHash["system-hash"], _ = systemInfoHash()
	hardwareHash["hostname-info"], _ = hostnameInfo()
	hardwareHash[ipAddressID], _ = primaryIpInfo()
	hardwareHash["macaddr-info"], _ = macAddrInfo()
	hardwareHash["disk-info"], _ = diskInfoHash()

	return hardwareHash
}

func csproductUuid() (string, error) {
	return commandOutputHash(wmicCommand, "csproduct", "get", "UUID")
}

func processorInfoHash() (value string, err error) {
	return commandOutputHash(wmicCommand, "cpu", "list", "brief")
}

func memoryInfoHash() (value string, err error) {
	return commandOutputHash(wmicCommand, "memorychip", "list", "brief")
}

func biosInfoHash() (value string, err error) {
	return commandOutputHash(wmicCommand, "bios", "list", "brief")
}

func systemInfoHash() (value string, err error) {
	return commandOutputHash(wmicCommand, "computersystem", "list", "brief")
}

func diskInfoHash() (value string, err error) {
	return commandOutputHash(wmicCommand, "diskdrive", "list", "brief")
}
