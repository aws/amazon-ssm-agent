// Copyright 2016 Amazon.com, Inc. or its affiliates. All Rights Reserved.
//
// Licensed under the Amazon Software License (the "License"). You may not
// use this file except in compliance with the License. A copy of the
// License is located at
//
// http://aws.amazon.com/asl/
//
// or in the "license" file accompanying this file. This file is distributed
// on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND,
// express or implied. See the License for the specific language governing
// permissions and limitations under the License.

// Package platform contains platform specific utilities.
package platform

import (
	"github.com/aws/amazon-ssm-agent/agent/log"
)

const (
	gettingPlatformDetailsMessage = "getting platform details"
	notAvailableMessage           = "NotAvailable"
	commandOutputMessage          = "Command output %v"
)

var cachePlatformName string
var cachePlatformVersion string

// GetPlatformName gets the OS specific platform name
func GetPlatformName(log log.T) (name string, err error) {
	return getPlatformName(log)
}

// GetPlatformVersion gets the OS specific platform version
func GetPlatformVersion(log log.T) (version string, err error) {
	return getPlatformVersion(log)
}
