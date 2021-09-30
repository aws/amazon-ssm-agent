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

//go:build windows
// +build windows

// package fingerprint contains functions that helps identify an instance
// hardwareInfo contains platform specific way of fetching the hardware hash
package fingerprint

import (
	"fmt"
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
	serviceRetry         = 5
)

func waitForService(log log.T, service *mgr.Service) error {
	var err error
	var status svc.Status

	for attempt := 1; attempt <= serviceRetry; attempt++ {
		status, err = service.Query()

		if err == nil && status.State == svc.Running {
			return nil
		}

		if err != nil {
			log.Debugf("Attempt %d: Failed to get WMI service status: %v", attempt, err)
		} else {
			log.Debugf("Attempt %d: WMI not running - Current status: %v", attempt, status.State)
		}
		time.Sleep(serviceRetryInterval * time.Second)
	}

	return fmt.Errorf("Failed to wait for WMI to get into Running status")
}

var wmicCommand = filepath.Join(appconfig.EnvWinDir, "System32", "wbem", "wmic.exe")

var currentHwHash = func() (map[string]string, error) {
	log := ssmlog.SSMLogger(true)
	hardwareHash := make(map[string]string)

	// Wait for WMI Service
	winManager, err := mgr.Connect()
	log.Debug("Waiting for WMI Service to be ready.....")
	if err != nil {
		log.Warnf("Failed to connect to WMI: '%v'", err)
		return hardwareHash, err
	}

	// Open WMI Service
	var wmiService *mgr.Service
	wmiService, err = winManager.OpenService(wmiServiceName)
	if err != nil {
		log.Warnf("Failed to open wmi service: '%v'", err)
		return hardwareHash, err
	}

	// Wait for WMI Service to start
	if err = waitForService(log, wmiService); err != nil {
		log.Warn("WMI Service cannot be query for hardware hash.")
		return hardwareHash, err
	}

	log.Debug("WMI Service is ready to be queried....")

	hardwareHash[hardwareID], _ = csproductUuid(log)
	hardwareHash["processor-hash"], _ = processorInfoHash()
	hardwareHash["memory-hash"], _ = memoryInfoHash()
	hardwareHash["bios-hash"], _ = biosInfoHash()
	hardwareHash["system-hash"], _ = systemInfoHash()
	hardwareHash["hostname-info"], _ = hostnameInfo()
	hardwareHash[ipAddressID], _ = primaryIpInfo()
	hardwareHash["macaddr-info"], _ = macAddrInfo()
	hardwareHash["disk-info"], _ = diskInfoHash()

	return hardwareHash, nil
}

func csproductUuid(logger log.T) (encodedValue string, err error) {
	encodedValue, uuid, err := commandOutputHash(wmicCommand, "csproduct", "get", "UUID")
	logger.Tracef("Current UUID value: /%v/", uuid)
	return
}

func processorInfoHash() (value string, err error) {
	value, _, err = commandOutputHash(wmicCommand, "cpu", "list", "brief")
	return
}

func memoryInfoHash() (value string, err error) {
	value, _, err = commandOutputHash(wmicCommand, "memorychip", "list", "brief")
	return
}

func biosInfoHash() (value string, err error) {
	value, _, err = commandOutputHash(wmicCommand, "bios", "list", "brief")
	return
}

func systemInfoHash() (value string, err error) {
	value, _, err = commandOutputHash(wmicCommand, "computersystem", "list", "brief")
	return
}

func diskInfoHash() (value string, err error) {
	value, _, err = commandOutputHash(wmicCommand, "diskdrive", "list", "brief")
	return
}
