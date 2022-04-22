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
// permissions and limitations under the License.

//go:build freebsd || linux || netbsd || openbsd || windows
// +build freebsd linux netbsd openbsd windows

package testcases

import (
	"fmt"
	"runtime/debug"
	"strings"
	"time"

	"github.com/aws/amazon-ssm-agent/agent/log"
	testCommon "github.com/aws/amazon-ssm-agent/agent/update/tester/common"
	"github.com/aws/amazon-ssm-agent/common/identity"
	"github.com/digitalocean/go-smbios/smbios"
)

const (
	failedToOpenStream          = "failed to open smbios stream"
	failedToDecodeStream        = "failed to decode smbios"
	failedToGetVendorAndVersion = "failed to get vendor and version"
	failedToGetUuid             = "failed to get uuid"
	failedQuerySystemHostInfo   = "failed to query system host info"
)

// ShouldRunTest determines if test should run
func (l *Ec2DetectorTestCase) ShouldRunTest() bool {
	return identity.IsEC2Instance(l.context.Identity())
}

func btoi(b bool) int {
	if b {
		return 1
	}
	return 0
}

// cleanBiosString casts string to lower case and trims spaces from string
func cleanBiosString(val string) string {
	return strings.TrimSpace(strings.ToLower(val))
}

func matchUuid(uuid string) bool {
	return bigEndianEc2UuidRegex.MatchString(uuid) || littleEndianEc2UuidRegex.MatchString(uuid)
}

func matchNitroEc2(info HostInfo) bool {
	return matchUuid(info.Uuid) && info.Vendor == nitroVendorValue
}

func matchXenEc2(info HostInfo) bool {
	return matchUuid(info.Uuid) && strings.HasSuffix(info.Version, xenVersionSuffix)
}

func isEc2Instance(info HostInfo) bool {
	return matchXenEc2(info) || matchNitroEc2(info)
}

// extractSmbiosHostInfo parses the list of smbios.Structure to a HostInfo based on SMBIOS spec
func extractSmbiosHostInfo(biosInfoList []*smbios.Structure) (HostInfo, error) {
	var hostInfo HostInfo
	// Parser created from SMBIOS spec: https://www.dmtf.org/sites/default/files/standards/documents/DSP0134_3.1.1.pdf
	for _, biosItem := range biosInfoList {
		// Only parse System Information with type 1
		if biosItem.Header.Type != 1 {
			continue
		}
		if len(biosItem.Formatted) >= 4 {
			manufacturerIndex := int(biosItem.Formatted[0])
			if manufacturerIndex > 0 && len(biosItem.Strings) >= manufacturerIndex {
				hostInfo.Vendor = cleanBiosString(biosItem.Strings[manufacturerIndex-1])
			}

			versionIndex := int(biosItem.Formatted[2])
			if versionIndex > 0 && len(biosItem.Strings) >= versionIndex {
				hostInfo.Version = cleanBiosString(biosItem.Strings[versionIndex-1])
			}

			serialNumberIndex := int(biosItem.Formatted[3])
			if serialNumberIndex > 0 && len(biosItem.Strings) >= serialNumberIndex {
				hostInfo.Uuid = cleanBiosString(biosItem.Strings[serialNumberIndex-1])
			}
		}
	}

	if hostInfo.Version == "" && hostInfo.Vendor == "" {
		return hostInfo, fmt.Errorf(failedToGetVendorAndVersion)
	}

	if hostInfo.Uuid == "" {
		return hostInfo, fmt.Errorf(failedToGetUuid)
	}

	return hostInfo, nil
}

// streamAndDecode queries streamAndDecode with retries and sleep
func streamAndDecodeSmbios() ([]*smbios.Structure, error) {
	rc, _, err := smbios.Stream()
	if err != nil {
		return []*smbios.Structure{}, fmt.Errorf("%s: %v", failedToOpenStream, err)
	}
	defer rc.Close()

	d := smbios.NewDecoder(rc)
	biosInfoList, err := d.Decode()
	if err != nil {
		return []*smbios.Structure{}, fmt.Errorf("%s: %v", failedToDecodeStream, err)
	}

	return biosInfoList, nil
}

// getSmbiosHostInfo queries streamAndDecode with retries and sleep
func getSmbiosHostInfo(log log.T) (HostInfo, error) {
	var biosInfoList []*smbios.Structure
	var err error

	for i := 0; i < maxRetry; i++ {
		if i != 0 {
			time.Sleep(sleepBetweenRetry)
		}
		biosInfoList, err = streamAndDecodeSmbios()

		if err == nil {
			return extractSmbiosHostInfo(biosInfoList)
		}

		log.Warnf("Failed stream and decode try %d/%d with error: %v", i+1, maxRetry, err)
	}

	return HostInfo{}, err
}

func (l *Ec2DetectorTestCase) generateHostInfoResult(info HostInfo, queryErr error, approach approachType) (bool, string) {
	testPass := false
	detectedHypervisor := unknown
	errDP := errNotSet

	if queryErr == nil {
		testPass = isEc2Instance(info)
		if testPass {
			if matchNitroEc2(info) {
				detectedHypervisor = nitro
			} else {
				detectedHypervisor = amazonXen
			}
		}
	} else {
		if strings.Contains(queryErr.Error(), failedToOpenStream) {
			errDP = errFailedOpenStream
		} else if strings.Contains(queryErr.Error(), failedToDecodeStream) {
			errDP = errFailedDecodeStream
		} else if strings.Contains(queryErr.Error(), failedQuerySystemHostInfo) {
			errDP = errFailedQuerySystemHostInfo
		} else if strings.Contains(queryErr.Error(), failedToGetVendorAndVersion) {
			errDP = errFailedGetVendorAndVersion
		} else if strings.Contains(queryErr.Error(), failedToGetUuid) {
			errDP = errFailedGetUuid
		} else {
			errDP = errUnknown
		}
	}

	return testPass, fmt.Sprintf("%sh%s_%se%d", approach, detectedHypervisor, approach, errDP)
}

func (l *Ec2DetectorTestCase) generateTestResult(primaryInfo HostInfo, primaryErr error, secondaryInfo HostInfo, secondaryErr error) (testCommon.TestResult, string) {
	isPrimarySuccess, primaryAdditionalInfo := l.generateHostInfoResult(primaryInfo, primaryErr, primary)
	isSecondarySuccess, secondaryAdditionalInfo := l.generateHostInfoResult(secondaryInfo, secondaryErr, secondary)

	result := testCommon.TestCaseFail
	if isPrimarySuccess || isSecondarySuccess {
		result = testCommon.TestCasePass
	}

	return result, fmt.Sprintf("_p%d_s%d_%s_%s", btoi(isPrimarySuccess), btoi(isSecondarySuccess), primaryAdditionalInfo, secondaryAdditionalInfo)
}

// generateTestOutput constructs the TestOutput based on the state of Ec2DetectorTestCase attributes
func (l *Ec2DetectorTestCase) generateTestOutput() testCommon.TestOutput {
	var testOutput testCommon.TestOutput
	testOutput.Result, testOutput.AdditionalInfo = l.generatePlatformTestResult()
	return testOutput
}

// ExecuteTestCase executes the ec2 detector test case, test only runs when instance id starts with i-
func (l *Ec2DetectorTestCase) ExecuteTestCase() (output testCommon.TestOutput) {
	defer func() {
		if err := recover(); err != nil {
			l.context.Log().Warnf("test panic: %v", err)
			l.context.Log().Warnf("Stacktrace:\n%s", debug.Stack())

			output = l.generateTestOutput()
			output.Result = testCommon.TestCaseFail
			output.AdditionalInfo += "_panic"
		}
	}()

	l.queryHostInfo()
	return l.generateTestOutput()
}
