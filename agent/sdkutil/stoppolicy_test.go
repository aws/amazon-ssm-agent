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

// Package sdkutil provides utilies used to call awssdk.
package sdkutil

import (
	"fmt"
	"sync"
	"testing"

	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/stretchr/testify/assert"
)

type StopPolicyTest struct {
	input           StopPolicy
	expectedHealthy bool
}

var (
	stopPolicyTests = []StopPolicyTest{
		{
			// if threshold is 0 => allow infinitely
			StopPolicy{
				"ErrorCount1",
				10,
				0,
				new(sync.Mutex)},
			true,
		}, // 0
		{
			// if threshold is 0 => allow infinitely
			StopPolicy{
				"ErrorCount2",
				10,
				0,
				new(sync.Mutex)},
			true,
		}, // 1
		{
			// if threshold is 1 => allow if error < threshold
			StopPolicy{
				"ErrorCount3",
				0,
				1,
				new(sync.Mutex)},
			true,
		}, // 2
		{
			// if threshold is 1 => disallow after error is also 1
			StopPolicy{
				"ErrorCount4",
				1,
				1,
				new(sync.Mutex)},
			false,
		}, // 3
	}
)

var logger = log.NewMockLog()

func runStopPolicyTests(t *testing.T, tests []StopPolicyTest) {
	for i, test := range tests {
		output := test.input.IsHealthy()
		assert.Equal(t, test.expectedHealthy, output, fmt.Sprintf("testcase %d failed, input %v", i, test.input))
	}
}

func TestStopPolicys(t *testing.T) {
	logger.Info("Starting stoppolicy tests")
	runStopPolicyTests(t, stopPolicyTests)
}

func TestStopPolicyUsage(t *testing.T) {
	logger.Info("test stop policy")
	s := NewStopPolicy("StopPolicyName1", 10)
	assert.Equal(t, true, s.IsHealthy(), "agent must be healty with 0 error count and threshold 10")

	s.AddErrorCount(10)
	assert.Equal(t, false, s.IsHealthy())

	s.AddErrorCount(-1)
	assert.Equal(t, true, s.IsHealthy())
}
