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

package helper

import (
	"regexp"
	"strings"
	"sync"
)

const (
	bigEndianEc2UuidRegex    = "^ec2[0-9a-f]{5}(-[0-9a-f]{4}){3}-[0-9a-f]{12}$"
	littleEndianEc2UuidRegex = "^[0-9a-f]{4}2[0-9a-f]ec(-[0-9a-f]{4}){3}-[0-9a-f]{12}$"
)

var detectors []Detector

func RegisterDetector(detector Detector) {
	detectors = append(detectors, detector)
}

func GetAllDetectors() []Detector {
	return detectors
}

type Detector interface {
	// IsEc2 returns true if detector detects attributes indicating it is a ec2 instance
	IsEc2() bool
	// GetName returns the name of the detector
	GetName() string
}

type DetectorHelper interface {
	// MatchUuid ensured string matches an uuid format and starts with ec2
	MatchUuid(string) bool
	// GetSystemInfo retrieves the system information based on platform, linux reads files while on Windows queries wmic
	GetSystemInfo(string) string
}

type detectorHelper struct {
	lock  sync.Mutex
	cache map[string]string
}

func (*detectorHelper) MatchUuid(uuid string) bool {
	uuid = strings.ToLower(uuid)
	isBigEndianEc2Uuid := regexp.MustCompile(bigEndianEc2UuidRegex).MatchString(uuid)
	isLittleEndianEc2Uuid := regexp.MustCompile(littleEndianEc2UuidRegex).MatchString(uuid)

	return isBigEndianEc2Uuid || isLittleEndianEc2Uuid
}

func GetDetectorHelper() *detectorHelper {
	return &detectorHelper{}
}
