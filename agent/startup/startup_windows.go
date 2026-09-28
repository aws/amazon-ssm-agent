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
//go:build windows
// +build windows

// Package startup implements startup plugin processor
package startup

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"runtime/debug"
	"strconv"
	"strings"

	"github.com/aws/amazon-ssm-agent/agent/appconfig"
	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/amazon-ssm-agent/agent/platform"
	"github.com/aws/amazon-ssm-agent/agent/startup/model"
	"github.com/aws/amazon-ssm-agent/agent/startup/serialport"
	"github.com/aws/amazon-ssm-agent/agent/version"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/aws/retry"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/feature/ec2/imds"
)

const (
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

	// NitroEnclavesEntity Properties
	NitroEnclavesName            = "Name"
	NitroEnclavesVersionProperty = "Version"

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

	// PS command to get AWS PV package entry from registry HKLM:\SOFTWARE\Amazon\PVDriver
	getPvPackageVersionCmd = "Get-ItemProperty -Path 'HKLM:\\SOFTWARE\\Amazon\\PVDriver'"

	// PS command to get AWS Nitro Enclaves package entry from registry HKLM:\SOFTWARE\Amazon\AwsNitroEnclaves
	getNitroEnclavesPackageVersionCmd = "Get-ItemProperty -Path 'HKLM:\\SOFTWARE\\Amazon\\AwsNitroEnclaves'"

	// PS command to get all AWS drivers from Windows driver list.
	getWindowsDriversCmd = "Get-WindowsDriver -Online | Where-Object { " +
		"$_.OriginalFileName -like '*xenvbd*' -or " +
		"$_.ClassName -eq 'Net' -and ( " +
		"$_.ProviderName -like 'Intel*' -or " +
		"$_.ProviderName -eq 'Citrix Systems, Inc.' -or " +
		"$_.ProviderName -eq 'Amazon Inc.' -or " +
		"$_.ProviderName -eq 'Amazon Web Services, Inc.' ) " +
		"}"

	// PS command to get all event logs for System
	getEventLogsCmd = "Get-WinEvent -FilterHashtable @( " +
		"@{ " + logNameProperty + "='System'; " + providerNameProperty + "='Microsoft-Windows-Kernel-General'; " +
		idProperty + "=12; " + levelProperty + "=4 }, " +
		"@{ " + logNameProperty + "='System'; " + providerNameProperty + "='Microsoft-Windows-WER-SystemErrorReporting'; " +
		idProperty + "=1001; " + levelProperty + "=2 } " +
		") | Sort-Object " + timeCreatedProperty + " -Descending"

	// WMI filter to get all AWS driver entries shown in Device Manager
	getAllPnpEntitiesWhereClause = "Where Service='xenvbd' Or Manufacturer Like 'Intel%' Or Manufacturer='Citrix Systems, Inc.' " +
		"Or Manufacturer='Amazon Inc.' Or Manufacturer='Amazon Web Services, Inc.'"

	// WMI filter to get all AWS signed drivers
	getPnpSignedDriversWhereClause = "Where DeviceID='%v' Or DeviceClass='Net' And (Manufacturer Like 'Intel%%' " +
		"Or Manufacturer='Citrix Systems, Inc.' Or Manufacturer='Amazon Inc.' Or Manufacturer='Amazon Web Services, Inc.')"
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
	// IMDS ignores RetryMaxAttempts and only honors a Retryer func, so bound
	// retries via WithRetryer instead of WithRetryMaxAttempts.
	cfg, err := config.LoadDefaultConfig(context.TODO(), config.WithRetryer(func() aws.Retryer {
		return retry.NewStandard(func(o *retry.StandardOptions) {
			o.MaxAttempts = 10
		})
	}))
	if err != nil {
		// LoadDefaultConfig can fail for reasons unrelated to EC2 detection
		// (e.g. unreadable shared config/credentials files). Rather than
		// treating this as "not in EC2", log the error and fall back to a
		// minimal config so we can still probe the metadata service.
		log.Warnf("failed to load default AWS config, falling back to minimal config: %v", err)
		cfg = aws.Config{
			RetryMaxAttempts: 10,
		}
	}

	client := imds.NewFromConfig(cfg)
	out, err := client.GetMetadata(context.TODO(), &imds.GetMetadataInput{Path: ""})
	if err != nil {
		// This is as designed to check if instance is in EC2, so it is not an error
		return false
	}
	// The GetMetadata output Content is an io.ReadCloser that must be closed to
	// avoid leaking the underlying HTTP response body.
	defer out.Content.Close()

	return true
}

// ExecuteTasks opens serial port, write agent verion, AWS driver info and bugchecks in console log.
func (p *Processor) ExecuteTasks() (err error) {
	defer func() {
		if msg := recover(); msg != nil {
			p.context.Log().Errorf("Failed to run through windows startup with err: %v", msg)
			p.context.Log().Errorf("Stacktrace:\n%s", debug.Stack())
		}
	}()
	var driverInfo []model.DriverInfo
	var bugChecks []string

	log := p.context.Log()

	log.Info("Executing startup processor tasks")

	windowsInfo, windowsInfoError := getWindowsInfo(log)

	sp, err := serialport.NewSerialPortWithRetry(log)
	if err != nil {
		log.Errorf("Error occurred while opening serial port: %v", err.Error())
		return
	}

	// defer is set to close the serial port during unexpected.
	defer func() {
		// serial port MUST be closed.
		sp.ClosePort()
	}()

	// write the agent version to serial port.
	sp.WritePort(fmt.Sprintf("Amazon SSM Agent v%v is running", version.Version))

	if windowsInfoError == nil {
		sp.WritePort(fmt.Sprintf("OsProductName: %v", windowsInfo.ProductName))
		sp.WritePort(fmt.Sprintf("OsInstallOption: %v", getInstallationOptionBySKU(log)))
		sp.WritePort(fmt.Sprintf("OsVersion: %v", getOsVersion(log, windowsInfo.CurrentMajorVersionNumber, windowsInfo.CurrentMinorVersionNumber)))
		sp.WritePort(fmt.Sprintf("OsBuildLabEx: %v", windowsInfo.BuildLabEx))
	}

	pvPackageInfo, PvError := getAWSPvPackageInfo(log)
	// write AWS PV Driver Package version to serial port if exists
	if PvError == nil {
		sp.WritePort(fmt.Sprintf("Driver: AWS PV Driver Package v%v", pvPackageInfo.Version))
	}

	nitroEnclavesPackageInfo, NitroEnclavesError := getAWSNitroEnclavesPackageInfo(log)
	// write AWS Nitro Enclaves Package version to serial port if exists
	if NitroEnclavesError == nil {
		sp.WritePort(fmt.Sprintf("Driver: AWS Nitro Enclaves Package v%v", nitroEnclavesPackageInfo.Version))
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

// getWindowsInfo queries Windows information from registry key
func getWindowsInfo(log log.T) (windowsInfo model.WindowsInfo, err error) {
	properties := []string{productNameProperty, buildLabExProperty, currentMajorVersionNumber, currentMinorVersionNumber}
	if err = runPowershell(&windowsInfo, getWindowsInfoCmd, properties, false); err != nil {
		log.Infof("Error occurred while querying Windows info: %v", err.Error())
	}
	return
}

func getOsVersion(log log.T, majorVersionNumber int, minorVersionNumber int) string {
	// ec2 console output must show only major and minor versions.
	if majorVersionNumber == 0 {
		if platformVersion, err := platform.PlatformVersion(log); err == nil {
			versionSplit := strings.Split(platformVersion, ".")
			if len(versionSplit) > 1 {
				return fmt.Sprintf("%v.%v", versionSplit[0], versionSplit[1])
			} else if len(versionSplit) == 1 {
				return fmt.Sprintf("%v.0", versionSplit[0])
			}
		}
	} else {
		return fmt.Sprintf("%v.%v", majorVersionNumber, minorVersionNumber)
	}

	return ""
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

// getAWSNitroEnclavesPackage queries AwsNitroEnclaves information from registry key.
func getAWSNitroEnclavesPackageInfo(log log.T) (NitroEnclavesPackageInfo model.NitroEnclavesPackageInfo, err error) {
	// this queries the registry for AWS Nitro Enclaves Package version
	properties := []string{NitroEnclavesName, NitroEnclavesVersionProperty}
	if err = runPowershell(&NitroEnclavesPackageInfo, getNitroEnclavesPackageVersionCmd, properties, false); err != nil {
		log.Debugf("Error occurred while querying Version for AWSNitroEnclavesPackage: %v", err.Error())
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

// getAWSDriverInfoForFull collects and returns driver information using Win32_PnPEntity and Win32_PnPSignedDriver.
func getAWSDriverInfoForFull(log log.T) (driverInfo []model.DriverInfo, err error) {
	// query xenvbd (AWS PV Storage Host Adapter) to get its DeviceId.
	var pnpEntities []platform.Win32_PnPEntity
	if pnpEntities, err = platform.GetWMIData[platform.Win32_PnPEntity]("Where Service='xenvbd'"); err != nil {
		log.Infof("Error occurred while querying DeviceID for AWS PV Storage Host Adapter: %v", err.Error())
		return
	}

	var deviceID string
	if len(pnpEntities) > 0 {
		deviceID = pnpEntities[0].DeviceID
	} else {
		log.Infof("No data found for DeviceID for AWS PV Storage Host Adapter")
		return
	}

	// query signed AWS drivers to get proper Name and Version.
	var pnpSignedDrivers []platform.Win32_PnPSignedDriver
	if pnpSignedDrivers, err = platform.GetWMIData[platform.Win32_PnPSignedDriver](fmt.Sprintf(getPnpSignedDriversWhereClause, deviceID)); err != nil {
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

// getAWSDriverInfoForNano collects and returns the driver information using Win32_PnPEntity and Get-WindowsDriver command.
func getAWSDriverInfoForNano(log log.T) (driverInfo []model.DriverInfo, err error) {
	var windowsDrivers []model.WindowsDriver

	// this queries AWS drivers in current Windows image to get Version.
	properties := []string{originalFileNameProperty, versionProperty}
	if err = runPowershell(&windowsDrivers, getWindowsDriversCmd, properties, true); err != nil {
		log.Infof("Error occurred while query Windows drivers: %v", err.Error())
		return
	}

	// query AWS drivers to get proper Name.
	var pnpEntities []platform.Win32_PnPEntity
	if pnpEntities, err = platform.GetWMIData[platform.Win32_PnPEntity](getAllPnpEntitiesWhereClause); err != nil {
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
	cmdOut, err = cmdExec.ExecuteCommand(appconfig.PowerShellPluginCommandName, args...)
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
func getInstallationOptionBySKU(log log.T) string {
	// the server options only include nano, core or undefined
	serverOptions := map[string]string{
		"0":   "Undefined",
		"12":  serverCore,
		"13":  serverCore,
		"14":  serverCore,
		"29":  serverCore,
		"39":  serverCore,
		"40":  serverCore,
		"41":  serverCore,
		"43":  serverCore,
		"44":  serverCore,
		"45":  serverCore,
		"46":  serverCore,
		"63":  serverCore,
		"143": nanoServer,
		"144": nanoServer,
		"147": serverCore,
		"148": serverCore,
	}

	if sku, err := platform.PlatformSku(log); err == nil {
		if val, ok := serverOptions[sku]; ok {
			return val
		} else {
			// return full server if it's neither nano, core or undefined.
			return fullServer
		}
	} else {
		return "Undefined"
	}
}
