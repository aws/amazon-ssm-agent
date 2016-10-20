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

// +build integration

// Package scheduler provides ability to create scheduled job
package scheduler

import (
	"testing"
	"time"

	"github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/stretchr/testify/assert"
)

var pollCalls int = 0

func TestCreatingScheduler(t *testing.T) {
	context := context.NewMockDefault()
	//override sleepMilli so it will not sleep before the poll
	sleepMilli = func(pollStartTime time.Time, sleepDurationInMilliseconds int) {}
	var testPollFrequencyInMinutes = 10

	job, err := CreateScheduler(context.Log(), incrementPollCalls, testPollFrequencyInMinutes)
	if err != nil {
		time.Sleep(100 * time.Millisecond)
		assert.True(t, pollCalls == 1)

		ScheduleNextRun(job)
		time.Sleep(100 * time.Millisecond)
		assert.True(t, pollCalls == 2)
	}
}

func incrementPollCalls() {
	pollCalls++
}
