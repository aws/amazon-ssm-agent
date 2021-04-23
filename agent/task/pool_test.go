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

package task

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/amazon-ssm-agent/agent/times"
	"github.com/stretchr/testify/assert"
)

var logger = log.NewMockLog()

func TestPool(t *testing.T) {
	testCases := []struct {
		Workers int
		Jobs    int
	}{
		{Workers: 1, Jobs: 1},
		{Workers: 1, Jobs: 2},
		{Workers: 1, Jobs: 10},
		{Workers: 2, Jobs: 1},
		{Workers: 2, Jobs: 2},
		{Workers: 2, Jobs: 10},
		{Workers: 10, Jobs: 10},
		{Workers: 10, Jobs: 1},
		{Workers: 10, Jobs: 100},
	}

	for _, testCase := range testCases {
		for _, shouldCancel := range []bool{false, true} {
			testPool(t, testCase.Workers, testCase.Jobs, shouldCancel)
		}
	}
}

func testPool(t *testing.T, nWorkers int, nJobs int, shouldCancel bool) {
	clock := times.NewMockedClock()
	waitTimeout := 100 * time.Millisecond

	// the "After" method may be called even if shouldCancel is false
	// because we call pool shutdown at the end of the test.
	clock.On("After", waitTimeout).Return(clock.AfterChannel)

	shutdownTimeout := 10000 * time.Millisecond
	clock.On("After", shutdownTimeout).Return(clock.AfterChannel)
	clock.On("After", shutdownTimeout+waitTimeout).Return(clock.AfterChannel)

	pool := NewPool(logger, nWorkers, waitTimeout, clock)
	var wg sync.WaitGroup
	for i := 0; i < nJobs; i++ {
		jobID := fmt.Sprintf("job-%d", i)
		_ = jobID
		wg.Add(1)
		go func() {
			defer wg.Done()
			exercisePool(t, pool, jobID, shouldCancel)
		}()
	}
	wg.Wait()

	// give time for (some of) the jobs to complete normally
	time.Sleep(10 * time.Millisecond)

	// send cancel signal to all running jobs and wait to finish
	assert.True(t, pool.ShutdownAndWait(shutdownTimeout))

	// Not verifying clock.After(waitTimeout) here. Refer to proc.go. We can't guarantee that 'doneChan' is not set the
	// first time waitEither(doneChan, clock.After) is checked.
	clock.AssertCalled(t, "After", shutdownTimeout)
	clock.AssertCalled(t, "After", shutdownTimeout+waitTimeout)
}

func exercisePool(t *testing.T, pool Pool, jobID string, shouldCancel bool) {
	// submit job
	jobState := make(chan bool)
	var flag CancelFlag
	err := pool.Submit(logger, jobID, func(cancelFlag CancelFlag) {
		flag = cancelFlag
		jobState <- true
		if shouldCancel {
			assert.True(t, Canceled == cancelFlag.Wait())
		}
		jobState <- true
	})
	assert.Nil(t, err)

	// check that job cannot be submitted again
	err = pool.Submit(logger, jobID, func(CancelFlag) {})
	assert.NotNil(t, err)

	// see that job starts
	assert.True(t, <-jobState)

	if shouldCancel {
		assert.False(t, flag.Canceled())
		// cancel job
		assert.True(t, pool.Cancel(jobID))
		assert.True(t, flag.Canceled())

		// check that thejob was immediately removed
		assert.False(t, pool.Cancel(jobID))
	}

	// see that job completes
	assert.True(t, <-jobState)
}
