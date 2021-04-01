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
//
// +build windows

// Package startup implements startup plugin processor
package startup

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"runtime/debug"
	"strconv"
	"strings"
	"time"

	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/amazon-ssm-agent/agent/platform"
	"github.com/aws/amazon-ssm-agent/agent/startup/model"
	"github.com/aws/amazon-ssm-agent/agent/startup/serialport"
	"github.com/aws/amazon-ssm-agent/agent/version"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/ec2metadata"
	"github.com/aws/aws-sdk-go/aws/session"
)

const (
	// Retry max count for opening serial port
	serialPortRetryMaxCount = 2

	// Wait time before retrying to open serial port
	serialPortRetryWaitTime = 1

	// OS installation options
	fullServer = "Full"
	nanoServer = "Nano"
	serverCore = "Server Core"

	// Windows and OS Info Properties
	productNameProperty        = "ProductName"
	buildLabExProperty         = "BuildLabEx"
	osVersionProperty          = "Version"
	operatingSystemSkuProperty = "OperatingSystemSKU"
	currentMajorVersionNumber  = "CurrentMajorVersionNumber"
	currentMinorVersionNumber  = "CurrentMinorVersionNumber"

	// PvEntity Properties
	PvName            = "Name"
	PvVersionProperty = "Version"

	// PnpEntity Properties
	deviceIDProperty = "DeviceID"
	serviceProperty  = "Service"
	nameProperty     = "Name"

	// PnpSignedDriver Properties
	descriptionProperty   = "Description"
	driverVersionProperty = "DriverVersion"

	// WindowsDriver Properties
	originalFileNameProperty = "OriginalFileName"
	versionProperty          = "Version"

	// EventLog Properties
	idProperty           = "Id"
	logNameProperty      = "LogName"
	levelProperty        = "Level"
	providerNameProperty = "ProviderName"
	messageProperty      = "Message"
	timeCreatedProperty  = "TimeCreated"
	propertiesProperty   = "Properties"

	// PS command to look up Windows information
	getWindowsInfoCmd = "Get-ItemProperty -Path 'HKLM:\\SOFTWARE\\Microsoft\\Windows NT\\CurrentVersion'"

	// PS command to get OS information
	getOSInfoCmd = "Get-CimInstance Win32_OperatingSystem"

	// PS command to get AWS PV package entry from registry HKLM:\SOFTWARE\Amazon\PVDriver
	getPvPackageVersionCmd = "Get-ItemProperty -Path 'HKLM:\\SOFTWARE\\Amazon\\PVDriver'"

	// PS command to get AWS PV Storage Host Adapter entry shown in Device Manager
	getPvDriverPnpEntityCmd = "Get-CimInstance Win32_PnPEntity | Where-Object { $_.Service -eq 'xenvbd' }"

	// PS command to get all AWS signed drivers
	getPnpSignedDriversCmd = "Get-CimInstance Win32_PnPSignedDriver | Where-Object { " +
		"$_.DeviceID -eq '%v' -or " +
		"$_.DeviceClass -eq 'Net' -and ( " +
		"$_.Manufacturer -like 'Intel*' -or " +
		"$_.Manufacturer -eq 'Citrix Systems, Inc.' -or " +
		"$_.Manufacturer -eq 'Amazon Inc.' -or " +
		"$_.Manufacturer -eq 'Amazon Web Services, Inc.' )" +
		"}"

	// PS command to get all AWS drivers from Windows driver list.
	getWindowsDriversCmd = "Get-WindowsDriver -Online | Where-Object { " +
		"$_.OriginalFileName -like '*xenvbd*' -or " +
		"$_.ClassName -eq 'Net' -and ( " +
		"$_.ProviderName -like 'Intel*' -or " +
		"$_.ProviderName -eq 'Citrix Systems, Inc.' -or " +
		"$_.ProviderName -eq 'Amazon Inc.' -or " +
		"$_.ProviderName -eq 'Amazon Web Services, Inc.' ) " +
		"}"

	// PS command to get all AWS driver entries shown in Device Manager
	getAllPnpEntitiesCmd = "Get-CimInstance Win32_PnPEntity | Where-Object { " +
		"$_.Service -eq 'xenvbd' -or " +
		"$_.Manufacturer -like 'Intel*' -or " +
		"$_.Manufacturer -eq 'Citrix Systems, Inc.' -or " +
		"$_.Manufacturer -eq 'Amazon Inc.' -or " +
		"$_.Manufacturer -eq 'Amazon Web Services, Inc.' " +
		"}"

	// PS command to get all event logs for System
	getEventLogsCmd = "Get-WinEvent -FilterHashtable @( " +
		"@{ " + logNameProperty + "='System'; " + providerNameProperty + "='Microsoft-Windows-Kernel-General'; " +
		idProperty + "=12; " + levelProperty + "=4 }, " +
		"@{ " + logNameProperty + "='System'; " + providerNameProperty + "='Microsoft-Windows-WER-SystemErrorReporting'; " +
		idProperty + "=1001; " + levelProperty + "=2 } " +
		") | Sort-Object " + timeCreatedProperty + " -Descending"

	defaultComPort = "\\\\.\\COM1"
)

// IsAllowed returns true if the current platform/instance allows startup processor.
// To allow startup processor in windows,
// 1. the windows major version must be 10 or above.
// 2. the instance must be running in EC2 environment.
// To check instance is in EC2 environment, it checks if metadata service is reachable.
// It attempts to get metadata with retry upto 10 time to ignore arbitrary failures/errors.
func (p *Processor) IsAllowed() bool {
	log := p.context.Log()

	// get the current OS version
	osVersion, err := platform.PlatformVersion(log)
	if err != nil {
		log.Errorf("Error occurred while getting OS version: %v", err.Error())
		return false
	} else if osVersion == "" {
		log.Errorf("Error occurred while getting OS version: OS version was empty")
	}

	// check if split worked
	osVersionSplit := strings.Split(osVersion, ".")
	if osVersionSplit == nil || len(osVersionSplit) == 0 {
		log.Error("Error occurred while parsing OS version")
		return false
	}

	// check if the OS version is 10 or above
	osMajorVersion, err := strconv.Atoi(osVersionSplit[0])
	if err != nil || osMajorVersion < 10 {
		// This is as designed to check OS version, so it is not an error
		return false
	}

	// check if metadata is rechable which indicates the instance is in EC2.
	// maximum retry is 10 to ensure the failure/error is not caused by arbitrary reason.
	ec2MetadataService := ec2metadata.New(session.New(aws.NewConfig().WithMaxRetries(10)))
	if metadata, err := ec2MetadataService.GetMetadata(""); err != nil || metadata == "" {
		// This is as designed to check if instance is in EC2, so it is not an error
		return false
	}

	return true
}

func discoverPort(log log.T, windowsInfo model.WindowsInfo) (port string, err error) {
	// TODO: Discover correct port to use.
	return defaultComPort, nil
}

// ExecuteTasks opens serial port, write agent verion, AWS driver info and bugchecks in console log.
func (p *Processor) ExecuteTasks() (err error) {
	defer func() {
		if msg := recover(); msg != nil {
			p.context.Log().Errorf("Failed to run through windows startup with err: %v", msg)
			p.context.Log().Errorf("Stacktrace:\n%s", debug.Stack())
		}
	}()
	var sp *serialport.SerialPort

	var driverInfo []model.DriverInfo
	var bugChecks []string

	log := p.context.Log()

	log.Info("Executing startup processor tasks")

	windowsInfo, osInfo, windowsInfoError := getSystemInfo(log)

	port := defaultComPort
	if windowsInfoError == nil {
		if port, err = discoverPort(log, windowsInfo); err != nil || port == "" {
			log.Infof("Could not discover port, %v. Setting to default port: %s", err, defaultComPort)
			port = defaultComPort
		}
	}
	log.Infof("Opening serial port: %s", port)

	// attempt to initialize and open the serial port.
	// since only three minute is allowed to write logs to console during boot,
	// it attempts to open serial port for approximately three minutes.
	retryCount := 0
	for retryCount < serialPortRetryMaxCount {
		sp = serialport.NewSerialPort(log, port)
		if err = sp.OpenPort(); err != nil {
			log.Errorf("%v. Retrying in %v seconds...", err.Error(), serialPortRetryWaitTime)
			time.Sleep(serialPortRetryWaitTime * time.Second)
			retryCount++
		} else {
			break
		}

		// if the retry count hits the maximum count, log the error and return.
		if retryCount == serialPortRetryMaxCount {
			err = errors.New("Timeout: Serial port is in use or not available")
			log.Errorf("Error occurred while opening serial port: %v", err.Error())
			return
		}
	}

	// defer is set to close the serial port during unexpected.
	defer func() {
		//serial port MUST be closed.
		sp.ClosePort()
	}()

	// write the agent version to serial port.
	sp.WritePort(fmt.Sprintf("Amazon SSM Agent v%v is running", version.Version))

	if windowsInfoError == nil {
		sp.WritePort(fmt.Sprintf("OsProductName: %v", windowsInfo.ProductName))
		sp.WritePort(fmt.Sprintf("OsInstallOption: %v", getInstallationOptionBySKU(osInfo.OperatingSystemSKU)))
		sp.WritePort(fmt.Sprintf("OsVersion: %v", osInfo.Version))
		sp.WritePort(fmt.Sprintf("OsBuildLabEx: %v", windowsInfo.BuildLabEx))
	}

	pvPackageInfo, PvError := getAWSPvPackageInfo(log)
	// write AWS PV Driver Package version to serial port if exists
	if PvError == nil {
		sp.WritePort(fmt.Sprintf("Driver: AWS PV Driver Package v%v", pvPackageInfo.Version))
	}

	// write all running AWS drivers to serial port.
	if driverInfo, err = getAWSDriverInfo(log); err == nil {
		for _, di := range driverInfo {
			sp.WritePort(fmt.Sprintf("Driver: %v v%v", di.Name, di.Version))
		}
	}

	// write all bugchecks occurred since the last boot time.
	if bugChecks, err = getBugChecks(log); err == nil {
		for _, bugCheck := range bugChecks {
			sp.WritePort(fmt.Sprintf("BCC: %v", bugCheck))
		}
	}

	return
}

// getSystemInfo queries Windows information from registry key and OS information from Win32_OperatingSystem.
func getSystemInfo(log log.T) (windowsInfo model.WindowsInfo, osInfo model.OperatingSystemInfo, err error) {
	// this queries Windows info.
	properties := []string{productNameProperty, buildLabExProperty, currentMajorVersionNumber, currentMinorVersionNumber}
	if err = runPowershell(&windowsInfo, getWindowsInfoCmd, properties, false); err != nil {
		log.Infof("Error occurred while querying Windows info: %v", err.Error())
	}

	// this queries OS info.
	properties = []string{osVersionProperty, operatingSystemSkuProperty}
	if err = runPowershell(&osInfo, getOSInfoCmd, properties, false); err != nil {
		log.Infof("Error occurred while querying OS info: %v", err.Error())
	}

	// ec2 console output must show only major and minor versions.
	if windowsInfo.CurrentMajorVersionNumber == 0 {
		versionSplit := strings.Split(osInfo.Version, ".")
		if len(versionSplit) > 1 {
			osInfo.Version = fmt.Sprintf("%v.%v", versionSplit[0], versionSplit[1])
		} else if len(versionSplit) == 1 {
			osInfo.Version = fmt.Sprintf("%v.0", versionSplit[0])
		}
	} else {
		osInfo.Version = fmt.Sprintf("%v.%v", windowsInfo.CurrentMajorVersionNumber, windowsInfo.CurrentMinorVersionNumber)
	}

	return
}

// getAWSPvPackage queries PvDriver information from registry key.
func getAWSPvPackageInfo(log log.T) (pvPackageInfo model.PvPackageInfo, err error) {
	var isNano bool

	// Nano Server does not contain AWS PV DriverPackage in registry, need to query for all drivers
	if isNano, err = platform.IsPlatformNanoServer(log); err != nil || !isNano {

		// this queries the registry for AWS PV Package version
		// PVDrivers after 8.2.1 store version information in the registry.
		// Attempt to pull from new registry entry, ignore and fallback to PvEntity logic if not found
		properties := []string{PvName, PvVersionProperty}
		if err = runPowershell(&pvPackageInfo, getPvPackageVersionCmd, properties, false); err != nil {
			log.Infof("Error occurred while querying Version for AWSPVPackage: %v", err.Error())
			return
		}
	} else if isNano {

		// Create a new error to detect nano servers
		err = errors.New("is a nano server")
	}

	return
}

// getAWSDriverInfo queries driver information from instance using powershell.
// because Nano server doesn't support Win32_PnpSignedDriver.
func getAWSDriverInfo(log log.T) (driverInfo []model.DriverInfo, err error) {
	var isNano bool
	if isNano, err = platform.IsPlatformNanoServer(log); err != nil || !isNano {
		driverInfo, err = getAWSDriverInfoForFull(log)
	} else {
		driverInfo, err = getAWSDriverInfoForNano(log)
	}

	return
}

// getAWSDriverInfoForFull runs powershell using Win32_PnPEntity and Win32_PnPSignedDriver
// and collects and returns driver information.
func getAWSDriverInfoForFull(log log.T) (driverInfo []model.DriverInfo, err error) {
	var pnpSignedDrivers []model.PnpSignedDriver
	var pnpEntities []model.PnpEntity
	var deviceID string

	// this queries xenvbd (AWS PV Storage Host Adapter) to get its DeviceId.
	properties := []string{deviceIDProperty}
	if err = runPowershell(&pnpEntities, getPvDriverPnpEntityCmd, properties, true); err != nil {
		log.Infof("Error occurred while querying DeviceID for AWS PV Storage Host Adapter: %v", err.Error())
		return
	}

	// get the DeviceID if the previous query had a result.
	if len(pnpEntities) != 0 {
		deviceID = pnpEntities[0].DeviceID
	}

	// this queries signed AWS drivers to get proper Name and Version.
	command := fmt.Sprintf(getPnpSignedDriversCmd, deviceID)
	properties = []string{descriptionProperty, driverVersionProperty}
	if err = runPowershell(&pnpSignedDrivers, command, properties, true); err != nil {
		log.Infof("Error occurred while querying signed AWS drivers: %v", err.Error())
		return
	}

	// build driver info based on the query result.
	for _, pnpSignedDriver := range pnpSignedDrivers {
		driverInfo = append(driverInfo, model.DriverInfo{
			Name:    pnpSignedDriver.Description,
			Version: pnpSignedDriver.DriverVersion,
		})
	}

	return
}

// getAWSDriverInfoForNano runs powershell using Win32_PnPEntity and Get-WindowsDriver command
// and collects and returns the driver information.
func getAWSDriverInfoForNano(log log.T) (driverInfo []model.DriverInfo, err error) {
	var windowsDrivers []model.WindowsDriver
	var pnpEntities []model.PnpEntity

	// this queries AWS drivers in current Windows image to get Version.
	properties := []string{originalFileNameProperty, versionProperty}
	if err = runPowershell(&windowsDrivers, getWindowsDriversCmd, properties, true); err != nil {
		log.Infof("Error occurred while query Windows drivers: %v", err.Error())
		return
	}

	// this queries AWS drivers to get proper Name.
	properties = []string{serviceProperty, nameProperty}
	if err = runPowershell(&pnpEntities, getAllPnpEntitiesCmd, properties, true); err != nil {
		log.Infof("Error occurred while querying AWS drivers: %v", err.Error())
		return
	}

	// build driver info based on the query result.
	// use Service property from PVDriver result and OriginalFileName property from WindowsDriver result to match entries.
	// Example:
	//   OriginalFileName - "C:\\Windows\\System32\\DriverStore\\FileRepository\\xenvbd.inf_amd64_xxxxx\\xenvbd.inf"
	//   Service - xenvbd
	for _, windowsDriver := range windowsDrivers {
		for _, pnpEntity := range pnpEntities {
			if pnpEntity.Service != "" && strings.HasSuffix(windowsDriver.OriginalFileName, pnpEntity.Service+".inf") {
				driverInfo = append(driverInfo, model.DriverInfo{
					Name:    pnpEntity.Name,
					Version: windowsDriver.Version,
				})
			}
		}
	}

	return
}

// getBugChecks finds and returns bugchecks occurred since the last boot time
func getBugChecks(log log.T) (bugChecks []string, err error) {
	var eventLogs []model.EventLog

	// this quries windows eventlogs for System log.
	properties := []string{idProperty, levelProperty, providerNameProperty, timeCreatedProperty, propertiesProperty}
	if err = runPowershell(&eventLogs, getEventLogsCmd, properties, true); err != nil {
		log.Infof("Error occurred while querying eventlogs: %v", err.Error())
		return
	}

	// iterate result eventlogs and find bugchecks occurred since the last boot time.
	for _, eventLog := range eventLogs {
		// iterate until [Microsoft-Windows-Kernel-General 12 Information] is found.
		if eventLog.ProviderName == "Microsoft-Windows-Kernel-General" && eventLog.ID == 12 && eventLog.Level == 4 {
			break
		}
		// if it finds [Microsoft-Windows-WER-SystemErrorReporting 1001 Error], it is likely to be caused by bugcheck.
		if eventLog.ProviderName == "Microsoft-Windows-WER-SystemErrorReporting" && eventLog.ID == 1001 && eventLog.Level == 2 {
			properties := eventLog.Properties
			if len(properties) > 0 {
				if value, found := properties[0].Value.(string); found {
					bugChecks = append(bugChecks, value)
					continue
				}
				bugChecks = append(bugChecks, "N/A")
			}
		}
	}

	return
}

// runPowershell runs powershell with given arguments and properties and convert that into json object.
func runPowershell(jsonObj interface{}, command string, properties []string, expectArray bool) (err error) {
	var args []string
	var cmdOut []byte

	// add commas between properties.
	var selectProperties bytes.Buffer
	propertiesSize := len(properties)
	for i := 0; i < propertiesSize; i++ {
		selectProperties.WriteString(properties[i])
		if i != propertiesSize-1 {
			selectProperties.WriteString(", ")
		}
	}

	// build the powershell command.
	args = append(args, command)
	args = append(args, "| Select-Object")
	args = append(args, selectProperties.String())
	args = append(args, "| ConvertTo-Json -Depth 3")

	// execute powershell with arguments in cmd.
	cmdOut, err = cmdExec.ExecuteCommand("powershell", args...)
	if err != nil {
		err = errors.New(fmt.Sprintf("Error while running powershell %v: %v", args, err.Error()))
		return
	}

	if len(cmdOut) == 0 {
		err = errors.New(fmt.Sprintf("Error while running powershell %v: No output", args))
		return
	}

	// surround the output with bracket if json array was expected, but output string doesn't represent json array.
	if expectArray {
		cmdOutStr := string(cmdOut)
		if !strings.HasPrefix(cmdOutStr, "[") && !strings.HasSuffix(cmdOutStr, "]") {
			cmdOutStr = "[" + cmdOutStr + "]"
			cmdOut = []byte(cmdOutStr)
		}
	}

	// unmarshal the result into given json object.
	err = json.Unmarshal(cmdOut, &jsonObj)

	return
}

// getInstallationOptionBySKU returns installation option of current windows.
func getInstallationOptionBySKU(sku int) string {
	// the server options only include nano, core or undefined
	serverOptions := map[int]string{
		0:   "Undefined",
		12:  serverCore,
		13:  serverCore,
		14:  serverCore,
		29:  serverCore,
		39:  serverCore,
		40:  serverCore,
		41:  serverCore,
		43:  serverCore,
		44:  serverCore,
		45:  serverCore,
		46:  serverCore,
		63:  serverCore,
		143: nanoServer,
		144: nanoServer,
		147: serverCore,
		148: serverCore,
	}

	if val, ok := serverOptions[sku]; ok {
		return val
	} else {
		// return full server if it's neither nano, core or undefined.
		return fullServer
	}
}
