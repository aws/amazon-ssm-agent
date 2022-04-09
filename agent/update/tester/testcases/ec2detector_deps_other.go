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
	"regexp"
	"time"

	"github.com/aws/amazon-ssm-agent/agent/context"
)

type HostInfo struct {
	Manufacturer string
	Version      string
	SerialNumber string
}

// Ec2DetectorTestCase represents the test case testing the ec2 detection module in ec2 environments
type Ec2DetectorTestCase struct {
	context context.T

	startTime time.Time

	streamFailures uint8
	decodeFailures uint8

	startedParsing  bool
	hasManufacturer bool
	hasVersion      bool
	hasSerialNumber bool

	uuidDP    dp
	vendorDP  dp
	versionDP dp
}

const (
	maxRetry          = 5
	sleepBetweenRetry = 200 * time.Millisecond

	amazonVersionString = "amazon"
	xenVersionSuffix    = "." + amazonVersionString

	nitroVendorValue = "amazon ec2"
	XenVendorValue   = "xen"
)

// detected amazon hypervisors
type hypervisor string

const (
	nitro     hypervisor = "n"
	amazonXen hypervisor = "x"
	unknown   hypervisor = "u"
)

type dp uint8

const (
	// uuid states
	uuidNotSet            dp = 0
	uuidNotInSMBios       dp = 1
	uuidEmpty             dp = 2
	uuidInvalidFormat     dp = 3
	uuidMatchBigEndian    dp = 4
	uuidMatchLittleEndian dp = 5
	uuidNoMatch           dp = 6

	// vendor states
	vendorNotSet      dp = 0
	vendorNotInSMBios dp = 1
	vendorEmpty       dp = 2
	vendorGenericXen  dp = 3
	vendorNitro       dp = 4
	vendorUnknown     dp = 5

	// version states
	versionNotSet            dp = 0
	versionNotInSMBios       dp = 1
	versionEmpty             dp = 2
	versionEndsWithDotAmazon dp = 3
	versionContainsAmazon    dp = 4
	versionUnknown           dp = 5

	timeDPMSIncrement = 500
	maxTimeDP         = 9
)

var (
	bigEndianEc2UuidRegex    = regexp.MustCompile("^ec2[0-9a-f]{5}(-[0-9a-f]{4}){3}-[0-9a-f]{12}$")
	littleEndianEc2UuidRegex = regexp.MustCompile("^[0-9a-f]{4}2[0-9a-f]ec(-[0-9a-f]{4}){3}-[0-9a-f]{12}$")
	uuidRegex                = regexp.MustCompile("^[0-9a-f]{8}(-[0-9a-f]{4}){3}-[0-9a-f]{12}$")
)
