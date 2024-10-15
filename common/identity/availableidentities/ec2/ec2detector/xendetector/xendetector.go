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

// Package xendetector implements logic to determine if we are running on an amazon Xen hypervisor
package xendetector

import (
	"strings"

	"github.com/aws/amazon-ssm-agent/common/identity/availableidentities/ec2/ec2detector/helper"
)

const (
	expectedVersionSuffix = ".amazon"
	Name                  = "Xen"
)

type xenDetector struct {
	helper  helper.DetectorHelper
	version string
	uuid    string
}

func (d *xenDetector) getUuid() string {
	if d.uuid != "" {
		return d.uuid
	}

	d.uuid = d.helper.GetSystemInfo(xenUuidSystemInfoParam)
	return d.uuid
}

func (d *xenDetector) getVersion() string {
	if d.version != "" {
		return d.version
	}

	d.version = d.helper.GetSystemInfo(xenVersionSystemInfoParam)
	return d.version
}

func (d *xenDetector) IsEc2() bool {
	if !strings.HasSuffix(strings.ToLower(d.getVersion()), expectedVersionSuffix) {
		return false
	}

	return d.helper.MatchUuid(d.getUuid())
}

func (d *xenDetector) GetName() string {
	return Name
}

func New(helper helper.DetectorHelper) *xenDetector {
	return &xenDetector{helper: helper}
}
