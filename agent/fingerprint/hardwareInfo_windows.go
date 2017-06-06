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

	"github.com/aws/amazon-ssm-agent/agent/appconfig"
)

const (
	hardwareID = "uuid"
)

var wmicCommand = filepath.Join(appconfig.EnvWinDir, "System32", "wbem", "wmic.exe")

var currentHwHash = func() map[string]string {
	hardwareHash := make(map[string]string)
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
