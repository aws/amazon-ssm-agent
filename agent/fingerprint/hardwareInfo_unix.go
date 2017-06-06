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

// +build freebsd linux netbsd openbsd darwin

// Package fingerprint contains functions that helps identify an instance
// hardwareInfo contains platform specific way of fetching the hardware hash
package fingerprint

import (
	"fmt"

	"github.com/aws/amazon-ssm-agent/agent/fileutil"
)

const (
	systemDMachineIDPath = "/etc/machine-id"
	upstartMachineIDPath = "/var/lib/dbus/machine-id"
	dmidecodeCommand     = "/usr/sbin/dmidecode"
	hardwareID           = "machine-id"
)

var currentHwHash = func() map[string]string {
	hardwareHash := make(map[string]string)
	hardwareHash[hardwareID], _ = machineID()
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

func machineID() (string, error) {
	if fileutil.Exists(systemDMachineIDPath) {
		return fileutil.ReadAllText(systemDMachineIDPath)
	} else if fileutil.Exists(upstartMachineIDPath) {
		return fileutil.ReadAllText(upstartMachineIDPath)
	} else {
		return "", fmt.Errorf("unable to fetch machine-id")
	}

}

func processorInfoHash() (value string, err error) {
	return commandOutputHash(dmidecodeCommand, "-t", "processor")
}

func memoryInfoHash() (value string, err error) {
	return commandOutputHash(dmidecodeCommand, "-t", "memory")
}

func biosInfoHash() (value string, err error) {
	return commandOutputHash(dmidecodeCommand, "-t", "bios")
}

func systemInfoHash() (value string, err error) {
	return commandOutputHash(dmidecodeCommand, "-t", "system")
}

func diskInfoHash() (value string, err error) {
	return commandOutputHash("ls", "-l", "/dev/disk/by-uuid")
}
