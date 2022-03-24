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

package nitrodetector

import (
	"strings"
	"testing"

	"github.com/aws/amazon-ssm-agent/common/identity/availableidentities/ec2/ec2detector/helper/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestIsEc2(t *testing.T) {
	helper := &mocks.DetectorHelper{}
	detector := &nitroDetector{helper: helper}

	detector.uuid = "someuuid"
	detector.vendor = "someothervendor"
	assert.False(t, detector.IsEc2())

	detector.uuid = "someuuid"
	detector.vendor = expectedNitroVendor + "-somepostfix"
	assert.False(t, detector.IsEc2())

	detector.uuid = "someuuid"
	detector.vendor = "someprefix-" + expectedNitroVendor
	assert.False(t, detector.IsEc2())

	helper.On("MatchUuid", mock.Anything).Return(false).Once()
	detector.uuid = "someuuid"
	detector.vendor = expectedNitroVendor
	assert.False(t, detector.IsEc2())

	helper.On("MatchUuid", mock.Anything).Return(true).Once()
	detector.uuid = "someuuid"
	detector.vendor = expectedNitroVendor
	assert.True(t, detector.IsEc2())

	helper.On("MatchUuid", mock.Anything).Return(true).Once()
	detector.uuid = "someuuid"
	detector.vendor = strings.ToUpper(expectedNitroVendor)
	assert.True(t, detector.IsEc2())

	helper.AssertExpectations(t)
}

func TestGetUuid(t *testing.T) {
	helper := &mocks.DetectorHelper{}
	detector := &nitroDetector{helper: helper}

	helper.On("GetSystemInfo", nitroUuidSystemInfoParam).Return("").Once()
	assert.Equal(t, "", detector.getUuid())
	assert.Equal(t, "", detector.uuid)

	helper.On("GetSystemInfo", nitroUuidSystemInfoParam).Return("something").Once()
	assert.Equal(t, "something", detector.getUuid())
	assert.Equal(t, "something", detector.uuid)
	assert.Equal(t, "something", detector.getUuid())

	helper.AssertExpectations(t)
}

func TestGetVendor(t *testing.T) {
	helper := &mocks.DetectorHelper{}
	detector := &nitroDetector{helper: helper}

	helper.On("GetSystemInfo", nitroVendorSystemInfoParam).Return("").Once()
	assert.Equal(t, "", detector.getVendor())
	assert.Equal(t, "", detector.vendor)

	helper.On("GetSystemInfo", nitroVendorSystemInfoParam).Return("something").Once()
	assert.Equal(t, "something", detector.getVendor())
	assert.Equal(t, "something", detector.vendor)
	assert.Equal(t, "something", detector.getVendor())

	helper.AssertExpectations(t)
}
