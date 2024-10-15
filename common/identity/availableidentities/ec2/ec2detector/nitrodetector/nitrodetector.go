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

//go:build !darwin
// +build !darwin

// Package nitrodetector implements logic to determine if we are running on an nitro hypervisor
package nitrodetector

import (
	"strings"

	"github.com/aws/amazon-ssm-agent/common/identity/availableidentities/ec2/ec2detector/helper"
)

const (
	expectedNitroVendor = "amazon ec2"
	Name                = "Nitro"
)

type nitroDetector struct {
	helper helper.DetectorHelper
	vendor string
	uuid   string
}

func (d *nitroDetector) getUuid() string {
	if d.uuid == "" {
		d.uuid = d.helper.GetSystemInfo(nitroUuidSystemInfoParam)
	}

	return d.uuid
}

func (d *nitroDetector) getVendor() string {
	if d.vendor == "" {
		d.vendor = d.helper.GetSystemInfo(nitroVendorSystemInfoParam)
	}

	return d.vendor
}

func (d *nitroDetector) IsEc2() bool {
	if strings.ToLower(d.getVendor()) != expectedNitroVendor {
		return false
	}

	return d.helper.MatchUuid(d.getUuid())
}

func (d *nitroDetector) GetName() string {
	return Name
}

func New(helper helper.DetectorHelper) *nitroDetector {
	return &nitroDetector{helper: helper}
}
