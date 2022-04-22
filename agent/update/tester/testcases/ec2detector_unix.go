// Copyright 2022 Amazon.com, Inc. or its affiliates. All Rights Reserved.
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
// permissions and limitations under the License

//go:build freebsd || linux || netbsd || openbsd
// +build freebsd linux netbsd openbsd

package testcases

import (
	"fmt"
	"io/ioutil"

	testCommon "github.com/aws/amazon-ssm-agent/agent/update/tester/common"
)

const (
	ec2DetectorTestCaseName = "UnixEc2Detector"

	nitroVendorSystemInfoParam = "/sys/class/dmi/id/sys_vendor"
	nitroUuidSystemInfoParam   = "/sys/class/dmi/id/product_uuid"

	xenVersionSystemInfoParam = "/sys/hypervisor/version/extra"
	xenUuidSystemInfoParam    = "/sys/hypervisor/uuid"
)

var readFile = func(filePath string) string {
	bytes, err := ioutil.ReadFile(filePath)
	if err != nil {
		return ""
	}

	return cleanBiosString(string(bytes))
}

func getSystemHostInfo() (HostInfo, error) {
	var hostInfo HostInfo

	hostInfo.Vendor = readFile(nitroVendorSystemInfoParam)
	hostInfo.Version = readFile(xenVersionSystemInfoParam)

	if hostInfo.Version == "" && hostInfo.Vendor == "" {
		return hostInfo, fmt.Errorf(failedToGetVendorAndVersion)
	}

	if hostInfo.Uuid = readFile(xenUuidSystemInfoParam); hostInfo.Uuid == "" {
		if hostInfo.Uuid = readFile(nitroUuidSystemInfoParam); hostInfo.Uuid == "" {
			return hostInfo, fmt.Errorf(failedToGetUuid)
		}
	}

	return hostInfo, nil
}

func (l *Ec2DetectorTestCase) queryHostInfo() {
	l.systemHostInfo, l.systemErr = getSystemHostInfo()
	l.smbiosHostInfo, l.smbiosErr = getSmbiosHostInfo(l.context.Log())
}

func (l *Ec2DetectorTestCase) generatePlatformTestResult() (testCommon.TestResult, string) {
	return l.generateTestResult(l.systemHostInfo, l.systemErr, l.smbiosHostInfo, l.smbiosErr)
}
