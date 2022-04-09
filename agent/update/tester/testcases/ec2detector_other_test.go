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
	"testing"
	"time"

	"github.com/aws/amazon-ssm-agent/agent/context"
	testCommon "github.com/aws/amazon-ssm-agent/agent/update/tester/common"
	"github.com/aws/amazon-ssm-agent/common/identity/availableidentities/ec2"
	"github.com/aws/amazon-ssm-agent/common/identity/availableidentities/onprem"
	identityMock "github.com/aws/amazon-ssm-agent/common/identity/mocks"
	"github.com/digitalocean/go-smbios/smbios"
	"github.com/stretchr/testify/assert"
)

func TestEc2DetectorTestCase_ShouldRunTest_OnPrem(t *testing.T) {
	identityMock := &identityMock.IAgentIdentity{}
	identityMock.On("IdentityType").Return(onprem.IdentityType)

	contextMock := &context.Mock{}
	contextMock.On("Identity").Return(identityMock)
	tc := Ec2DetectorTestCase{
		context: contextMock,
	}

	assert.False(t, tc.ShouldRunTest())
	contextMock.AssertExpectations(t)
	identityMock.AssertExpectations(t)
}

func TestEc2DetectorTestCase_ShouldRunTest_EC2(t *testing.T) {
	identityMock := &identityMock.IAgentIdentity{}
	identityMock.On("IdentityType").Return(ec2.IdentityType)

	contextMock := &context.Mock{}
	contextMock.On("Identity").Return(identityMock)
	tc := Ec2DetectorTestCase{
		context: contextMock,
	}

	assert.True(t, tc.ShouldRunTest())
	contextMock.AssertExpectations(t)
	identityMock.AssertExpectations(t)
}

func TestEc2DetectorTestCase_cleanBiosString(t *testing.T) {
	tc := Ec2DetectorTestCase{}
	assert.Equal(t, "examplestring", tc.cleanBiosString("\n\r\tExAmPlEsTrInG\r\n\t"))
}

func getValidSystemInfoStruct(structType uint8) *smbios.Structure {
	var result smbios.Structure

	result.Header.Type = structType

	result.Formatted = []byte{uint8(1), uint8(2), uint8(3), uint8(4)}
	result.Strings = []string{
		"Index0",
		"Index1",
		"Index2",
		"Index3",
	}

	return &result
}

func TestEc2DetectorTestCase_extractHostInfo_NoSystemInformation(t *testing.T) {
	var biosInfoList []*smbios.Structure
	tc := Ec2DetectorTestCase{}

	var i uint8
	for i = 0; i < 20; i++ {
		if i == 1 {
			continue
		}

		biosInfoList = append(biosInfoList, getValidSystemInfoStruct(i))
	}

	assert.False(t, tc.startedParsing)
	info := tc.extractHostInfo(biosInfoList)
	assert.True(t, tc.startedParsing)

	assert.Equal(t, "", info.Manufacturer)
	assert.Equal(t, "", info.Version)
	assert.Equal(t, "", info.SerialNumber)

	assert.False(t, tc.hasManufacturer)
	assert.False(t, tc.hasVersion)
	assert.False(t, tc.hasSerialNumber)
}

func TestEc2DetectorTestCase_extractHostInfo_TooFewFormatted(t *testing.T) {
	tc := Ec2DetectorTestCase{}

	biosInfo := getValidSystemInfoStruct(1)
	biosInfo.Formatted = []byte{uint8(1), uint8(2), uint8(3)}

	assert.False(t, tc.startedParsing)
	info := tc.extractHostInfo([]*smbios.Structure{biosInfo})
	assert.True(t, tc.startedParsing)

	assert.Equal(t, "", info.Manufacturer)
	assert.Equal(t, "", info.Version)
	assert.Equal(t, "", info.SerialNumber)

	assert.False(t, tc.hasManufacturer)
	assert.False(t, tc.hasVersion)
	assert.False(t, tc.hasSerialNumber)
}

func TestEc2DetectorTestCase_extractHostInfo_InvalidFormattedReferences(t *testing.T) {
	tc := Ec2DetectorTestCase{}

	biosInfo := getValidSystemInfoStruct(1)
	biosInfo.Strings = []string{}
	biosInfo.Formatted = []byte{uint8(1), uint8(1), uint8(1), uint8(1)}

	assert.False(t, tc.startedParsing)
	info := tc.extractHostInfo([]*smbios.Structure{biosInfo})
	assert.True(t, tc.startedParsing)

	assert.Equal(t, "", info.Manufacturer)
	assert.Equal(t, "", info.Version)
	assert.Equal(t, "", info.SerialNumber)

	assert.False(t, tc.hasManufacturer)
	assert.False(t, tc.hasVersion)
	assert.False(t, tc.hasSerialNumber)
}

func TestEc2DetectorTestCase_extractHostInfo_OnlyVersionSet(t *testing.T) {
	tc := Ec2DetectorTestCase{}

	biosInfo := getValidSystemInfoStruct(1)
	biosInfo.Formatted = []byte{uint8(0), uint8(0), uint8(2), uint8(0)}

	assert.False(t, tc.startedParsing)
	info := tc.extractHostInfo([]*smbios.Structure{biosInfo})
	assert.True(t, tc.startedParsing)

	assert.Equal(t, "", info.Manufacturer)
	assert.Equal(t, "index1", info.Version)
	assert.Equal(t, "", info.SerialNumber)

	assert.False(t, tc.hasManufacturer)
	assert.True(t, tc.hasVersion)
	assert.False(t, tc.hasSerialNumber)
}

func TestEc2DetectorTestCase_extractHostInfo_AllValues(t *testing.T) {
	tc := Ec2DetectorTestCase{}

	biosInfo := getValidSystemInfoStruct(1)

	assert.False(t, tc.startedParsing)
	info := tc.extractHostInfo([]*smbios.Structure{biosInfo})
	assert.True(t, tc.startedParsing)

	assert.Equal(t, "index0", info.Manufacturer)
	assert.Equal(t, "index2", info.Version)
	assert.Equal(t, "index3", info.SerialNumber)

	assert.True(t, tc.hasManufacturer)
	assert.True(t, tc.hasVersion)
	assert.True(t, tc.hasSerialNumber)
}

func TestEc2DetectorTestCase_extractHostInfo_AnotherAllValues(t *testing.T) {
	tc := Ec2DetectorTestCase{}

	biosInfo := getValidSystemInfoStruct(1)
	biosInfo.Formatted = []byte{uint8(4), uint8(3), uint8(2), uint8(1)}

	assert.False(t, tc.startedParsing)
	info := tc.extractHostInfo([]*smbios.Structure{biosInfo})
	assert.True(t, tc.startedParsing)

	assert.Equal(t, "index3", info.Manufacturer)
	assert.Equal(t, "index1", info.Version)
	assert.Equal(t, "index0", info.SerialNumber)

	assert.True(t, tc.hasManufacturer)
	assert.True(t, tc.hasVersion)
	assert.True(t, tc.hasSerialNumber)
}

func TestEc2DetectorTestCase_generateTestOutput(t *testing.T) {
	defer func() { getTimeNow = time.Now }()
	tc := Ec2DetectorTestCase{}

	var cnt uint8
	for _, isSuccess := range []bool{true, false} {
		for _, amazonHypervisor := range []hypervisor{nitro, amazonXen, unknown} {
			tc.startTime = time.Now()

			tc.streamFailures = cnt
			tc.decodeFailures = cnt + 1
			tc.startedParsing = cnt%2 == 0
			tc.uuidDP = dp(cnt + 2)
			tc.vendorDP = dp(cnt + 3)
			tc.versionDP = dp(cnt + 4)

			timeTaken := cnt + 5
			getTimeNow = func() time.Time {
				return tc.startTime.Add(time.Millisecond * timeDPMSIncrement * time.Duration(timeTaken))
			}

			output := tc.generateTestOutput(isSuccess, amazonHypervisor)

			if isSuccess {
				assert.Equal(t, testCommon.TestCasePass, output.Result)
			} else {
				assert.Equal(t, testCommon.TestCaseFail, output.Result)
			}

			expectedTimeDP := timeTaken
			if timeTaken > maxTimeDP {
				expectedTimeDP = maxTimeDP
			}

			expected := fmt.Sprintf("_%s_sf%d_df%d_sp%d_uuid%d_vendor%d_version%d_t%d",
				amazonHypervisor,
				tc.streamFailures,
				tc.decodeFailures,
				tc.btoi(tc.startedParsing),
				tc.uuidDP,
				tc.vendorDP,
				tc.versionDP,
				expectedTimeDP)

			assert.Equal(t, expected, output.AdditionalInfo)
			cnt++
		}
	}
}

func TestEc2DetectorTestCase_getUuidDP(t *testing.T) {
	var info HostInfo
	tc := Ec2DetectorTestCase{}

	assert.Equal(t, uuidNotSet, tc.uuidDP)
	assert.Equal(t, uuidNotInSMBios, tc.getUuidDP(info))

	tc.hasSerialNumber = true
	assert.Equal(t, uuidEmpty, tc.getUuidDP(info))

	info.SerialNumber = "some-invalid-format"
	assert.Equal(t, uuidInvalidFormat, tc.getUuidDP(info))

	info.SerialNumber = "ec2fffff-ffff-ffff-ffff-ffffffffffff"
	assert.Equal(t, uuidMatchBigEndian, tc.getUuidDP(info))

	info.SerialNumber = "ffff2fec-ffff-ffff-ffff-ffffffffffff"
	assert.Equal(t, uuidMatchLittleEndian, tc.getUuidDP(info))

	info.SerialNumber = "ffffffff-ffff-ffff-ffff-ffffffffffff"
	assert.Equal(t, uuidNoMatch, tc.getUuidDP(info))

	info.SerialNumber = "00000000-0000-0000-0000-000000000000"
	assert.Equal(t, uuidNoMatch, tc.getUuidDP(info))
}

func TestEc2DetectorTestCase_getVendorDP(t *testing.T) {
	var info HostInfo
	tc := Ec2DetectorTestCase{}

	assert.Equal(t, vendorNotSet, tc.vendorDP)
	assert.Equal(t, vendorNotInSMBios, tc.getVendorDP(info))

	tc.hasManufacturer = true
	assert.Equal(t, vendorEmpty, tc.getVendorDP(info))

	info.Manufacturer = XenVendorValue
	assert.Equal(t, vendorGenericXen, tc.getVendorDP(info))

	info.Manufacturer = nitroVendorValue
	assert.Equal(t, vendorNitro, tc.getVendorDP(info))

	info.Manufacturer = "somethingElse"
	assert.Equal(t, vendorUnknown, tc.getVendorDP(info))
}

func TestEc2DetectorTestCase_getVersionDP(t *testing.T) {
	var info HostInfo
	tc := Ec2DetectorTestCase{}

	assert.Equal(t, versionNotSet, tc.versionDP)
	assert.Equal(t, versionNotInSMBios, tc.getVersionDP(info))

	tc.hasVersion = true
	assert.Equal(t, versionEmpty, tc.getVersionDP(info))

	info.Version = "4.2.amazon"
	assert.Equal(t, versionEndsWithDotAmazon, tc.getVersionDP(info))

	info.Version = "something.amazon.else"
	assert.Equal(t, versionContainsAmazon, tc.getVersionDP(info))

	info.Version = "somethingElse"
	assert.Equal(t, versionUnknown, tc.getVersionDP(info))
}

func TestEc2DetectorTestCase_isEc2Instance_NegativeTests(t *testing.T) {
	tc := Ec2DetectorTestCase{}

	tc.uuidDP = uuidNoMatch
	tc.versionDP = versionEndsWithDotAmazon
	tc.vendorDP = vendorNotSet
	assert.False(t, tc.isEc2Instance())

	tc.uuidDP = uuidNoMatch
	tc.versionDP = versionNotSet
	tc.vendorDP = vendorNitro
	assert.False(t, tc.isEc2Instance())

	tc.uuidDP = uuidMatchLittleEndian
	tc.versionDP = versionNotSet
	tc.vendorDP = vendorNotSet
	assert.False(t, tc.isEc2Instance())

	tc.uuidDP = uuidMatchBigEndian
	tc.versionDP = versionNotSet
	tc.vendorDP = vendorNotSet
	assert.False(t, tc.isEc2Instance())
}

func TestEc2DetectorTestCase_isEc2Instance_PositiveTests(t *testing.T) {
	tc := Ec2DetectorTestCase{}

	tc.uuidDP = uuidMatchBigEndian
	tc.versionDP = versionEndsWithDotAmazon
	tc.vendorDP = vendorNotSet
	assert.True(t, tc.isEc2Instance())

	tc.uuidDP = uuidMatchBigEndian
	tc.versionDP = versionNotSet
	tc.vendorDP = vendorNitro
	assert.True(t, tc.isEc2Instance())

	tc.uuidDP = uuidMatchLittleEndian
	tc.versionDP = versionEndsWithDotAmazon
	tc.vendorDP = vendorNotSet
	assert.True(t, tc.isEc2Instance())

	tc.uuidDP = uuidMatchLittleEndian
	tc.versionDP = versionNotSet
	tc.vendorDP = vendorNitro
	assert.True(t, tc.isEc2Instance())
}

func TestEc2DetectorTestCase_getEc2HypervisorVendor(t *testing.T) {
	tc := Ec2DetectorTestCase{}

	tc.versionDP = versionEndsWithDotAmazon
	tc.vendorDP = vendorNotSet
	assert.Equal(t, amazonXen, tc.getEc2HypervisorVendor())

	tc.versionDP = versionContainsAmazon
	tc.vendorDP = vendorNotSet
	assert.Equal(t, unknown, tc.getEc2HypervisorVendor())

	tc.versionDP = versionNotSet
	tc.vendorDP = vendorNitro
	assert.Equal(t, nitro, tc.getEc2HypervisorVendor())

	tc.versionDP = versionEndsWithDotAmazon
	tc.vendorDP = vendorNitro
	assert.Equal(t, nitro, tc.getEc2HypervisorVendor())

	tc.versionDP = versionNotSet
	tc.vendorDP = vendorNotInSMBios
	assert.Equal(t, unknown, tc.getEc2HypervisorVendor())

	tc.versionDP = versionNotInSMBios
	tc.vendorDP = vendorNotInSMBios
	assert.Equal(t, unknown, tc.getEc2HypervisorVendor())
}
