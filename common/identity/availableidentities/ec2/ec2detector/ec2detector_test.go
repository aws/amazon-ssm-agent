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

package ec2detector

import (
	"testing"

	"github.com/aws/amazon-ssm-agent/agent/appconfig"
	"github.com/aws/amazon-ssm-agent/common/identity/availableidentities/ec2/ec2detector/helper"
	"github.com/aws/amazon-ssm-agent/common/identity/availableidentities/ec2/ec2detector/helper/mocks"
	"github.com/stretchr/testify/assert"
)

func TestIsEC2Instance(t *testing.T) {
	detector := ec2Detector{}
	trueSubDetector := &mocks.Detector{}
	falseSubDetector := &mocks.Detector{}

	assert.False(t, detector.IsEC2Instance())

	trueSubDetector.On("IsEc2").Return(true).Once()
	detector.detectors = []helper.Detector{trueSubDetector}
	assert.True(t, detector.IsEC2Instance())

	trueSubDetector.On("IsEc2").Return(true).Once()
	detector.detectors = []helper.Detector{trueSubDetector, falseSubDetector}
	assert.True(t, detector.IsEC2Instance())

	falseSubDetector.On("IsEc2").Return(false).Once()
	trueSubDetector.On("IsEc2").Return(true).Once()
	detector.detectors = []helper.Detector{falseSubDetector, trueSubDetector}
	assert.True(t, detector.IsEC2Instance())

	falseSubDetector.On("IsEc2").Return(false).Once()
	detector.detectors = []helper.Detector{falseSubDetector}
	assert.False(t, detector.IsEC2Instance())

	trueSubDetector.AssertExpectations(t)
	falseSubDetector.AssertExpectations(t)
}

func TestIsEC2Instance_ConfiguredReturnValue(t *testing.T) {
	detector := ec2Detector{}
	subDetector := &mocks.Detector{}

	detector.detectors = []helper.Detector{subDetector}
	detector.config = appconfig.SsmagentConfig{Identity: appconfig.IdentityCfg{Ec2SystemInfoDetectionResponse: "true"}}
	assert.True(t, detector.IsEC2Instance())

	detector.detectors = []helper.Detector{subDetector}
	detector.config = appconfig.SsmagentConfig{Identity: appconfig.IdentityCfg{Ec2SystemInfoDetectionResponse: "false"}}
	assert.False(t, detector.IsEC2Instance())

	subDetector.On("IsEc2").Return(true).Once()
	detector.detectors = []helper.Detector{subDetector}
	detector.config = appconfig.SsmagentConfig{Identity: appconfig.IdentityCfg{Ec2SystemInfoDetectionResponse: "unsupportedValue"}}
	assert.True(t, detector.IsEC2Instance())
}
