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
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/aws/amazon-ssm-agent/agent/appconfig"
	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/amazon-ssm-agent/agent/log/ssmlog"

	"golang.org/x/sys/windows/svc"
	"golang.org/x/sys/windows/svc/mgr"
)

type WMIInterface string

const (
	hardwareID     = "uuid"
	wmiServiceName = "Winmgmt"

	serviceRetryInterval = 15 // Seconds
	serviceRetry         = 5

	winVersionCommand = "Get-CimInstance -ClassName Win32_OperatingSystem | Format-List -Property Version"

	win2025MajorVersion = 10
	win2025MinorVersion = 0
	win2025BuildNumber  = 26040

	wmic        WMIInterface = "WMIC"
	cimInstance WMIInterface = "CIM Instance"
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

	wmiInterface, _ := getWMIInterface(log)
	hardwareHash[hardwareID], _ = csproductUuid(log, wmiInterface)
	hardwareHash["processor-hash"], _ = processorInfoHash(log, wmiInterface)
	hardwareHash["memory-hash"], _ = memoryInfoHash(log, wmiInterface)
	hardwareHash["bios-hash"], _ = biosInfoHash(log, wmiInterface)
	hardwareHash["system-hash"], _ = systemInfoHash(log, wmiInterface)
	hardwareHash["hostname-info"], _ = hostnameInfo()
	hardwareHash[ipAddressID], _ = primaryIpInfo()
	hardwareHash["macaddr-info"], _ = macAddrInfo()
	hardwareHash["disk-info"], _ = diskInfoHash(log, wmiInterface)

	return hardwareHash, nil
}

// get WMI interface which should be uses to retrieve the hardware info, if any error occur use WMIC by default
func getWMIInterface(logger log.T) (wmiInterface WMIInterface, err error) {
	// get Windows version number via CIM Instance, as it works on all Windows versions
	if outputBytes, err := exec.Command(appconfig.PowerShellPluginCommandName, winVersionCommand).Output(); err == nil {
		outputStr := strings.Replace(strings.Replace(string(outputBytes), "\n", "", -1), "\r", "", -1) //removes any new lines from the output
		logger.Debugf("Windows version property command output: %s", outputStr)                        //expected output format should be: Version : 10.0.26100
		outputStrElements := strings.Split(outputStr, ":")
		if len(outputStrElements) != 2 {
			logger.Warnf("Unexpected command output format for Windows version property: %s, setting WMIC as WMI interface", outputStr)
			return wmic, fmt.Errorf("Error while parsing command output for windows version property. Got %v elements and was expecting 2.",
				len(outputStrElements))
		}

		winVersion := strings.Split(strings.TrimSpace(outputStrElements[1]), ".") //secend element in the output is the actual Windows version value
		if len(winVersion) != 3 {
			logger.Warnf("Unexpected format for the Windows version property value: %s, setting WMIC as WMI interface", outputStr)
			return wmic, fmt.Errorf("Error while parsing the Windows version property value. Got %v elements and was expecting 3.", len(winVersion))
		}

		winMajorVersion, err := strconv.Atoi(winVersion[0])
		if err != nil {
			logger.Warnf("Bad format for the Windows major version value: %s, setting WMIC as WMI interface", winVersion[0])
			return wmic, fmt.Errorf("Error while parsing Windows major version: %v", err)
		}
		winMinorVersion, err := strconv.Atoi(winVersion[1])
		if err != nil {
			logger.Warnf("Bad format for the Windows minor version value: %s, setting WMIC as WMI interface", winVersion[1])
			return wmic, fmt.Errorf("Error while parsing Windows minor version: %v", err)
		}
		winBuildNumber, err := strconv.Atoi(winVersion[2])
		if err != nil {
			logger.Warnf("Bad format for the Windows build number value: %s, setting WMIC as WMI interface", winVersion[2])
			return wmic, fmt.Errorf("Error while parsing Windows build number: %v", err)
		}

		// if it is Windows 2025 or later use CIM Instance, otherwise use WMIC
		if winMajorVersion >= win2025MajorVersion && winMinorVersion >= win2025MinorVersion && winBuildNumber >= win2025BuildNumber {
			logger.Debugf("Windows version is %d.%d.%d, setting CIM Instance as WMI interface", winMajorVersion, winMinorVersion, winBuildNumber)
			return cimInstance, nil
		} else {
			logger.Debugf("Windows version is %d.%d.%d, setting wmic as WMI interface", winMajorVersion, winMinorVersion, winBuildNumber)
			return wmic, nil
		}
	}

	logger.Warnf("There was an issue while retrieving Windows version data: %s, setting WMIC as WMI interface", err)
	return wmic, err
}

func csproductUuid(logger log.T, wmiInterface WMIInterface) (encodedValue string, err error) {
	var uuid string
	switch wmiInterface {
	case wmic:
		encodedValue, uuid, err = commandOutputHash(wmicCommand, "csproduct", "get", "UUID")
	case cimInstance:
		encodedValue, uuid, err = commandOutputHash(appconfig.PowerShellPluginCommandName, getCimInstanceCmdArgs(logger, "Win32_ComputerSystemProduct", "UUID"))
	default:
		logger.Warnf("Unknown WMI interface: %v", wmiInterface)
	}

	logger.Tracef("Current UUID value: /%v/", uuid)
	return
}

func processorInfoHash(logger log.T, wmiInterface WMIInterface) (value string, err error) {
	switch wmiInterface {
	case wmic:
		value, _, err = commandOutputHash(wmicCommand, "cpu", "list", "brief")
	case cimInstance:
		propertiesToRetrieve := []string{"Caption", "DeviceID", "Manufacturer", "MaxClockSpeed", "Name", "SocketDesignation"}
		value, _, err = commandOutputHash(appconfig.PowerShellPluginCommandName, getCimInstanceCmdArgs(logger, "WIN32_PROCESSOR", propertiesToRetrieve...))
	default:
		logger.Warnf("Unknown WMI interface: %v", wmiInterface)
	}

	return
}

func memoryInfoHash(logger log.T, wmiInterface WMIInterface) (value string, err error) {
	switch wmiInterface {
	case wmic:
		value, _, err = commandOutputHash(wmicCommand, "memorychip", "list", "brief")
	case cimInstance:
		propertiesToRetrieve := []string{"Capacity", "DeviceLocator", "MemoryType", "Name", "Tag", "TotalWidth"}
		value, _, err = commandOutputHash(appconfig.PowerShellPluginCommandName, getCimInstanceCmdArgs(logger, "Win32_PhysicalMemory", propertiesToRetrieve...))
	default:
		logger.Warnf("Unknown WMI interface: %v", wmiInterface)
	}

	return
}

func biosInfoHash(logger log.T, wmiInterface WMIInterface) (value string, err error) {
	switch wmiInterface {
	case wmic:
		value, _, err = commandOutputHash(wmicCommand, "bios", "list", "brief")
	case cimInstance:
		propertiesToRetrieve := []string{"Manufacturer", "Name", "SerialNumber", "SMBIOSBIOSVersion", "Version"}
		value, _, err = commandOutputHash(appconfig.PowerShellPluginCommandName, getCimInstanceCmdArgs(logger, "Win32_BIOS", propertiesToRetrieve...))
	default:
		logger.Warnf("Unknown WMI interface: %v", wmiInterface)
	}

	return
}

func systemInfoHash(logger log.T, wmiInterface WMIInterface) (value string, err error) {
	switch wmiInterface {
	case wmic:
		value, _, err = commandOutputHash(wmicCommand, "computersystem", "list", "brief")
	case cimInstance:
		propertiesToRetrieve := []string{"Domain", "Manufacturer", "Model", "Name", "PrimaryOwnerName", "TotalPhysicalMemory"}
		value, _, err = commandOutputHash(appconfig.PowerShellPluginCommandName, getCimInstanceCmdArgs(logger, "Win32_ComputerSystem", propertiesToRetrieve...))
	default:
		logger.Warnf("Unknown WMI interface: %v", wmiInterface)
	}

	return
}

func diskInfoHash(logger log.T, wmiInterface WMIInterface) (value string, err error) {
	switch wmiInterface {
	case wmic:
		value, _, err = commandOutputHash(wmicCommand, "diskdrive", "list", "brief")
	case cimInstance:
		propertiesToRetrieve := []string{"Caption", "DeviceID", "Model", "Partitions", "Size"}
		value, _, err = commandOutputHash(appconfig.PowerShellPluginCommandName, getCimInstanceCmdArgs(logger, "Win32_DiskDrive", propertiesToRetrieve...))
	default:
		logger.Warnf("Unknown WMI interface: %v", wmiInterface)
	}

	return
}

func getCimInstanceCmdArgs(logger log.T, className string, properties ...string) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Get-CimInstance -ClassName %s", className))
	if len(properties) > 0 {
		sb.WriteString(" | Select-Object ")
		for _, property := range properties {
			sb.WriteString(fmt.Sprintf("%s, ", property))
		}
	}

	cmd := fmt.Sprintf("%s | ConvertTo-Json", strings.TrimRight(sb.String(), ", "))
	logger.Debugf("getCimInstanceCmdArgs cmd=%s", cmd)
	return cmd
}
