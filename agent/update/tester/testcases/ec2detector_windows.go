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

//go:build windows
// +build windows

package testcases

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/aws/amazon-ssm-agent/agent/appconfig"
	testCommon "github.com/aws/amazon-ssm-agent/agent/update/tester/common"
)

const (
	ec2DetectorTestCaseName = "WinEc2Detector"

	commandExecTimeout = 3 * time.Second
	commandMaxRetry    = 3
	biosInfoCmd        = "Get-CimInstance -ClassName Win32_BIOS"

	uuidKey    = "SerialNumber"
	vendorKey  = "Manufacturer"
	versionKey = "SMBIOSBIOSVersion"
)

var execCommand = func(cmd string, params ...string) (string, error) {
	var err error
	var byteOutput []byte

	ctx, cancel := context.WithTimeout(context.Background(), commandExecTimeout)
	defer cancel()
	for i := 0; i < commandMaxRetry; i++ {
		byteOutput, err = exec.CommandContext(ctx, cmd, params...).Output()
		if err == nil {
			return strings.TrimSpace(string(byteOutput)), nil
		}
	}

	return "", err
}

func getSystemHostInfo() (HostInfo, error) {
	var info HostInfo
	args := append(strings.Split(appconfig.PowerShellCommandArgs, " "), biosInfoCmd)
	output, err := execCommand(appconfig.PowerShellPluginCommandName, args...)
	if err != nil {
		return info, fmt.Errorf("%s: %v", failedQuerySystemHostInfo, err)
	}

	for _, biosLine := range strings.Split(output, "\r\n") {
		splitLine := strings.SplitN(biosLine, ":", 2)
		if len(splitLine) != 2 {
			continue
		}

		value := cleanBiosString(splitLine[1])
		switch strings.TrimSpace(splitLine[0]) {
		case uuidKey:
			info.Uuid = value
		case vendorKey:
			info.Vendor = value
		case versionKey:
			info.Version = value
		}
	}

	if info.Version == "" && info.Vendor == "" {
		return info, fmt.Errorf(failedToGetVendorAndVersion)
	}

	if info.Uuid == "" {
		return info, fmt.Errorf(failedToGetUuid)
	}

	return info, nil
}

func (l *Ec2DetectorTestCase) queryHostInfo() {
	l.smbiosHostInfo, l.smbiosErr = getSmbiosHostInfo(l.context.Log())
	l.systemHostInfo, l.systemErr = getSystemHostInfo()
}

func (l *Ec2DetectorTestCase) generatePlatformTestResult() (testCommon.TestResult, string) {
	return l.generateTestResult(l.smbiosHostInfo, l.smbiosErr, l.systemHostInfo, l.systemErr)
}
