// Copyright 2016 Amazon.com, Inc. or its affiliates. All Rights Reserved.
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
//
// Package pluginutil implements some common functions shared by multiple plugins.
//
// +build windows

package pluginutil

import (
	"testing"

	"github.com/aws/amazon-ssm-agent/agent/appconfig"
	"github.com/aws/amazon-ssm-agent/agent/contracts"
	"github.com/aws/amazon-ssm-agent/agent/task"
	"github.com/stretchr/testify/assert"
)

type TestCase struct {
	Input  int
	Output contracts.ResultStatus
}

var TestCases = []TestCase{
	{
		Input:  0,
		Output: contracts.ResultStatusSuccess,
	},
	{
		Input:  appconfig.RebootExitCode,
		Output: contracts.ResultStatusSuccessAndReboot,
	},
	{
		Input:  commandStoppedPreemptivelyExitCode,
		Output: contracts.ResultStatusTimedOut,
	},
}

// testGetStatus tests that exitCodes are mapped correctly to their respective ResultStatus
func TestGetStatus(t *testing.T) {
	var mockCancelFlag *task.MockCancelFlag
	setCancelFlagExpectations(mockCancelFlag)

	for _, testCase := range TestCases {
		status := GetStatus(testCase.Input, mockCancelFlag)
		assert.Equal(t, testCase.Output, status)
	}
}

func setCancelFlagExpectations(mockCancelFlag *task.MockCancelFlag) {
	mockCancelFlag.On("Canceled").Return(false)
	mockCancelFlag.On("ShutDown").Return(false)
}
