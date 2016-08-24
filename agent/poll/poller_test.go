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

package poll

import (
	"testing"
	"time"

	"github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/stretchr/testify/assert"
)

const (
	testPollFrequencInyMinutes = 1
)

var pollCalls int

// Integration test to test polling
func TestPoll(t *testing.T) {
	context := context.NewMockDefault()
	pollService := PollService{}
	sleepMilli = func(a time.Time, b int) {}
	pollService.StartPolling(incrementPollCalls, testPollFrequencyInMinutes, context.Log())
	time.Sleep(time.Second)
	assert.Equal(t, 1, pollCalls)
}

func preparePoll() {
	pollCalls = 0
}

func incrementPollCalls() {
	pollCalls = 1
}
