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

	testCommon "github.com/aws/amazon-ssm-agent/agent/update/tester/common"
	"github.com/aws/amazon-ssm-agent/common/identity"
	"github.com/digitalocean/go-smbios/smbios"
)

var getTimeNow = time.Now

// ShouldRunTest determines if test should run
func (l *Ec2DetectorTestCase) ShouldRunTest() bool {
	return identity.IsEC2Instance(l.context.Identity())
}

// ExecuteTestCase executes the ec2 detector test case, test only runs when instance id starts with i-
func (l *Ec2DetectorTestCase) ExecuteTestCase() (output testCommon.TestOutput) {
	l.startTime = time.Now()
	defer func() {
		if err := recover(); err != nil {
			l.context.Log().Warnf("test panic: %v", err)
			l.context.Log().Warnf("Stacktrace:\n%s", debug.Stack())

			output = l.generateTestOutput(false, unknown)
			output.AdditionalInfo += "_panic"
		}
	}()

	biosInfoList, err := l.getHostInfo()
	if err != nil {
		output = l.generateTestOutput(false, unknown)
		l.context.Log().Warnf("Failed to get host info after %v with status %s", time.Now().Sub(l.startTime), output.Result)
		return output
	}

	hostInfo := l.extractHostInfo(biosInfoList)
	l.uuidDP = l.getUuidDP(hostInfo)
	l.vendorDP = l.getVendorDP(hostInfo)
	l.versionDP = l.getVersionDP(hostInfo)

	output = l.generateTestOutput(l.isEc2Instance(), l.getEc2HypervisorVendor())
	l.context.Log().Infof("Successfully finished ec2detector test after %v with status %s", time.Now().Sub(l.startTime), output.Result)

	return output
}

// cleanBiosString casts string to lower case and trims spaces from string
func (l *Ec2DetectorTestCase) cleanBiosString(val string) string {
	return strings.TrimSpace(strings.ToLower(val))
}

// extractHostInfo parses the list of smbios.Structure to a HostInfo based on SMBIOS spec
func (l *Ec2DetectorTestCase) extractHostInfo(biosInfoList []*smbios.Structure) HostInfo {
	var hostInfo HostInfo
	l.startedParsing = true
	// Parser created from SMBIOS spec: https://www.dmtf.org/sites/default/files/standards/documents/DSP0134_3.1.1.pdf
	for _, biosItem := range biosInfoList {
		// Only parse System Information with type 1
		if biosItem.Header.Type != 1 {
			continue
		}
		if len(biosItem.Formatted) >= 4 {
			manufacturerIndex := int(biosItem.Formatted[0])
			if manufacturerIndex > 0 && len(biosItem.Strings) >= manufacturerIndex {
				l.hasManufacturer = true
				hostInfo.Manufacturer = l.cleanBiosString(biosItem.Strings[manufacturerIndex-1])
			}

			versionIndex := int(biosItem.Formatted[2])
			if versionIndex > 0 && len(biosItem.Strings) >= versionIndex {
				l.hasVersion = true
				hostInfo.Version = l.cleanBiosString(biosItem.Strings[versionIndex-1])
			}

			serialNumberIndex := int(biosItem.Formatted[3])
			if serialNumberIndex > 0 && len(biosItem.Strings) >= serialNumberIndex {
				l.hasSerialNumber = true
				hostInfo.SerialNumber = l.cleanBiosString(biosItem.Strings[serialNumberIndex-1])
			}
		}
	}

	return hostInfo
}

// streamAndDecode queries streamAndDecode with retries and sleep
func (l *Ec2DetectorTestCase) streamAndDecode() ([]*smbios.Structure, error) {
	rc, _, err := smbios.Stream()
	if err != nil {
		l.streamFailures += 1
		return []*smbios.Structure{}, fmt.Errorf("failed to open smbios stream: %v", err)
	}
	defer rc.Close()

	d := smbios.NewDecoder(rc)
	biosInfoList, err := d.Decode()
	if err != nil {
		l.decodeFailures += 1
		return []*smbios.Structure{}, fmt.Errorf("failed to decode smbios structures: %v", err)
	}

	return biosInfoList, nil
}

// getHostInfo queries streamAndDecode with retries and sleep
func (l *Ec2DetectorTestCase) getHostInfo() ([]*smbios.Structure, error) {
	var biosInfoList []*smbios.Structure
	var err error

	for i := 0; i < maxRetry; i++ {
		if i != 0 {
			time.Sleep(sleepBetweenRetry)
		}
		biosInfoList, err = l.streamAndDecode()

		if err == nil {
			return biosInfoList, err
		}

		l.context.Log().Warnf("Failed stream and decode try %d/%d with error: %v", i+1, maxRetry, err)
	}

	return biosInfoList, err
}

// btoi casts boolean to a datapoint
func (l *Ec2DetectorTestCase) btoi(b bool) dp {
	if b {
		return 1
	}
	return 0
}

// generateTestOutput constructs the TestOutput based on the state of Ec2DetectorTestCase attributes
func (l *Ec2DetectorTestCase) generateTestOutput(isSuccess bool, hypervisor hypervisor) testCommon.TestOutput {
	var testOutput testCommon.TestOutput

	if isSuccess {
		testOutput.Result = testCommon.TestCasePass
	} else {
		testOutput.Result = testCommon.TestCaseFail
	}

	timeDP := getTimeNow().Sub(l.startTime).Milliseconds() / timeDPMSIncrement
	if timeDP > maxTimeDP {
		timeDP = maxTimeDP
	}

	testOutput.AdditionalInfo = fmt.Sprintf("_%s_sf%d_df%d_sp%d_uuid%d_vendor%d_version%d_t%d",
		hypervisor,
		l.streamFailures,
		l.decodeFailures,
		l.btoi(l.startedParsing),
		l.uuidDP,
		l.vendorDP,
		l.versionDP,
		timeDP)

	return testOutput
}

// getUuidDP returns the uuid datapoint for based on smbios serial number attribute
func (l *Ec2DetectorTestCase) getUuidDP(info HostInfo) dp {
	if !l.hasSerialNumber {
		return uuidNotInSMBios
	} else if info.SerialNumber == "" {
		return uuidEmpty
	} else if !uuidRegex.MatchString(info.SerialNumber) {
		return uuidInvalidFormat
	} else if bigEndianEc2UuidRegex.MatchString(info.SerialNumber) {
		return uuidMatchBigEndian
	} else if littleEndianEc2UuidRegex.MatchString(info.SerialNumber) {
		return uuidMatchLittleEndian
	}

	// uuid has valid format but does not have ec2 prefix
	return uuidNoMatch
}

// getVendorDP returns the vendor datapoint for based on smbios manufacturer attribute
func (l *Ec2DetectorTestCase) getVendorDP(info HostInfo) dp {
	if !l.hasManufacturer {
		return vendorNotInSMBios
	} else if info.Manufacturer == "" {
		return vendorEmpty
	} else if info.Manufacturer == XenVendorValue {
		return vendorGenericXen
	} else if info.Manufacturer == nitroVendorValue {
		return vendorNitro
	}

	return vendorUnknown
}

// getVersionDP returns the version datapoint for based on smbios version attribute
func (l *Ec2DetectorTestCase) getVersionDP(info HostInfo) dp {
	if !l.hasVersion {
		return versionNotInSMBios
	} else if info.Version == "" {
		return versionEmpty
	} else if strings.HasSuffix(info.Version, xenVersionSuffix) {
		return versionEndsWithDotAmazon
	} else if strings.Contains(info.Version, amazonVersionString) {
		return versionContainsAmazon
	}

	return versionUnknown
}

// isEc2Instance returns true of uuid starts with ec2 and if either vendor indicates nitro or if version ends with .amazon
func (l *Ec2DetectorTestCase) isEc2Instance() bool {
	return (l.uuidDP == uuidMatchBigEndian || l.uuidDP == uuidMatchLittleEndian) &&
		(l.vendorDP == vendorNitro || l.versionDP == versionEndsWithDotAmazon)
}

// getEc2HypervisorVendor determines amazon hypervisor vendor based on vendor and version
func (l *Ec2DetectorTestCase) getEc2HypervisorVendor() hypervisor {
	if l.vendorDP == vendorNitro {
		return nitro
	} else if l.versionDP == versionEndsWithDotAmazon {
		return amazonXen
	}

	return unknown
}
