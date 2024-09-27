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

// Package platform contains platform specific utilities.
package platform

import (
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"

	"github.com/aws/amazon-ssm-agent/agent/appconfig"
	"github.com/aws/amazon-ssm-agent/agent/log"
)

const (
	caption = "Caption"
	version = "Version"
	sku     = "OperatingSystemSKU"
)

// Win32_OperatingSystems https://msdn.microsoft.com/en-us/library/aa394239%28v=vs.85%29.aspx
const (
	// PRODUCT_DATA_CENTER_NANO_SERVER = 143
	ProductDataCenterNanoServer = "143"

	// PRODUCT_STANDARD_NANO_SERVER = 144
	ProductStandardNanoServer = "144"

	// WindowsServer2016Version represents Win32_OperatingSystemVersion https://learn.microsoft.com/en-us/windows/win32/sysinfo/operating-system-version
	WindowsServer2016Version = 10
)

var (
	getPlatformVersionRef = getPlatformVersion
)

// isPlatformWindowsServer2012OrEarlier returns true if platform is Windows Server 2012 or earlier
func isPlatformWindowsServer2012OrEarlier(log log.T) (bool, error) {
	var platformVersion string
	var platformVersionInt int
	var err error

	if platformVersion, err = getPlatformVersionRef(log); err != nil {
		return false, err
	}
	versionParts := strings.Split(platformVersion, ".")
	if len(versionParts) == 0 {
		return false, fmt.Errorf("could not get the version from versionstring: %v", versionParts)
	}

	if platformVersionInt, err = strconv.Atoi(versionParts[0]); err != nil {
		return false, err
	}
	return platformVersionInt < WindowsServer2016Version, nil
}

// IsPlatformNanoServer returns true if SKU is 143 or 144
func isPlatformNanoServer(log log.T) (bool, error) {
	var sku string
	var err error

	// Get platform sku information
	if sku, err = getPlatformSku(log); err != nil {
		log.Infof("Failed to fetch sku - %v", err)
		return false, err
	}

	// If sku represents nano server, return true
	if sku == ProductDataCenterNanoServer || sku == ProductStandardNanoServer {
		return true, nil
	}

	return false, nil
}

func getPlatformName(log log.T) (value string, err error) {
	return getPlatformDetails(caption, log)
}

func getPlatformType(log log.T) (value string, err error) {
	return "windows", nil
}

func getPlatformVersion(log log.T) (value string, err error) {
	return getPlatformDetails(version, log)
}

func getPlatformSku(log log.T) (value string, err error) {
	return getPlatformDetails(sku, log)
}

func getPlatformDetails(property string, log log.T) (value string, err error) {
	log.Debugf(gettingPlatformDetailsMessage)
	value = notAvailableMessage

	cmdName := appconfig.PowerShellPluginCommandName
	cmdArgs := []string{fmt.Sprintf("Get-CimInstance -ClassName Win32_OperatingSystem | Format-List -Property %s", property)}
	var cmdOut []byte
	if cmdOut, err = exec.Command(cmdName, cmdArgs...).Output(); err != nil {
		log.Debugf("There was an error running %v %v, err:%v", cmdName, cmdArgs, err)
		return
	}

	// Stringnize cmd output and trim spaces
	value = strings.TrimSpace(string(cmdOut))

	// Match whitespaces between property and = sign and remove whitespaces
	rp := regexp.MustCompile(fmt.Sprintf("%v(\\s*)%v", property, ":"))
	value = rp.ReplaceAllString(value, "")

	// Trim spaces again
	value = strings.TrimSpace(value)

	log.Debugf(commandOutputMessage, value)
	return
}

// fullyQualifiedDomainName returns the Fully Qualified Domain Name of the instance, otherwise the hostname
func fullyQualifiedDomainName(log log.T) string {
	hostName, _ := os.Hostname()

	dnsHostName := getWMIComputerSystemValue("DNSHostName")
	domainName := getWMIComputerSystemValue("Domain")

	if dnsHostName == "" || domainName == "" {
		return hostName
	}

	return dnsHostName + "." + domainName
}

// getWMIComputerSystemValue return the specified attribute from WMI via powershell
func getWMIComputerSystemValue(attribute string) string {
	cmdArgs := []string{fmt.Sprintf("Get-CimInstance -Class Win32_ComputerSystem | Format-List -Property %s", attribute)}
	if contentBytes, err := exec.Command(appconfig.PowerShellPluginCommandName, cmdArgs...).Output(); err == nil {
		contents := string(contentBytes)
		data := strings.Split(contents, ":")
		if len(data) > 1 {
			return strings.TrimSpace(data[1])
		}
	}
	return ""
}
