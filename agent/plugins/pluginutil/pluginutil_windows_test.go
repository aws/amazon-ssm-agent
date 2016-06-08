// Copyright 2016 Amazon.com, Inc. or its affiliates. All Rights Reserved.
//
// Licensed under the AWS Customer Agreement (the "License"). You may
// not use this file except in compliance with the License. A copy of the
// License is located at
//
// http://aws.amazon.com/agreement/
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
	TestCase{
		Input:  0,
		Output: contracts.ResultStatusSuccess,
	},
	TestCase{
		Input:  appconfig.RebootExitCode,
		Output: contracts.ResultStatusSuccessAndReboot,
	},
	TestCase{
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
