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
	Vendor  string
	Version string
	Uuid    string
}

// Ec2DetectorTestCase represents the test case testing the ec2 detection module in ec2 environments
type Ec2DetectorTestCase struct {
	context context.T

	smbiosHostInfo HostInfo
	smbiosErr      error

	systemHostInfo HostInfo
	systemErr      error
}

const (
	maxRetry          = 5
	sleepBetweenRetry = 200 * time.Millisecond

	amazonVersionString = "amazon"
	xenVersionSuffix    = "." + amazonVersionString

	nitroVendorValue = "amazon ec2"
)

// detected amazon hypervisors
type hypervisor string

const (
	nitro     hypervisor = "n"
	amazonXen hypervisor = "x"
	unknown   hypervisor = "u"
)

// keys indicating primary or secondary approach
type approachType string

const (
	primary   approachType = "p"
	secondary approachType = "s"
)

type dp uint8

const (
	// error datapoint
	errNotSet                    dp = 0
	errUnknown                   dp = 1
	errFailedOpenStream          dp = 2
	errFailedDecodeStream        dp = 3
	errFailedQuerySystemHostInfo    = 4
	errFailedGetVendorAndVersion dp = 5
	errFailedGetUuid             dp = 6
)

var (
	bigEndianEc2UuidRegex    = regexp.MustCompile("^ec2[0-9a-f]{5}(-[0-9a-f]{4}){3}-[0-9a-f]{12}$")
	littleEndianEc2UuidRegex = regexp.MustCompile("^[0-9a-f]{4}2[0-9a-f]ec(-[0-9a-f]{4}){3}-[0-9a-f]{12}$")
)
