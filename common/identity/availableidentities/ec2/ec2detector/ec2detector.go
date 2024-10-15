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

// Package ec2detector implements the detection of EC2 using specific sub-detectors
package ec2detector

import (
	"github.com/aws/amazon-ssm-agent/agent/appconfig"
	"github.com/aws/amazon-ssm-agent/common/identity/availableidentities/ec2/ec2detector/helper"
)

type Ec2Detector interface {
	// IsEC2Instance returns true if any sub detector detects it is running on EC2
	IsEC2Instance() bool
}

type ec2Detector struct {
	detectors []helper.Detector
	config    appconfig.SsmagentConfig
}

func (e *ec2Detector) IsEC2Instance() bool {
	if e.config.Identity.Ec2SystemInfoDetectionResponse != "" {
		switch e.config.Identity.Ec2SystemInfoDetectionResponse {
		case "true":
			return true
		case "false":
			return false
		}
	}

	for _, detector := range e.detectors {
		if detector.IsEc2() {
			return true
		}
	}

	return false
}

// New returns a struct implementing the EC2Detector interface to detect if we are running on ec2
func New(config appconfig.SsmagentConfig) *ec2Detector {
	return &ec2Detector{
		helper.GetAllDetectors(),
		config,
	}
}
