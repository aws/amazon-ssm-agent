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

	pool := NewPool(logger, nWorkers, 0, waitTimeout, clock)
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

// TestAcquireToken_ExpiredToken test expired tokens
func TestAcquireToken_ExpiredToken(t *testing.T) {
	// basic pool
	poolRef := NewPool(logger, 1, 2, 100*time.Millisecond, times.NewMockedClock())
	poolObj := poolRef.(*pool)
	expirationTime := time.Now().Add(time.Duration(-unusedTokenValidityInMinutes-2) * time.Minute) // added extra expiry
	// if buffer not full, should not delete
	poolObj.tokenHoldingJobIds["job 2"] = &expirationTime
	errorCode := poolObj.AcquireBufferToken("job 3")
	assert.Equal(t, 2, len(poolObj.tokenHoldingJobIds))
	assert.Equal(t, PoolErrorCode(""), errorCode)

	// if buffer full, should delete
	poolObj.tokenHoldingJobIds["job 4"] = &expirationTime
	errorCode = poolObj.AcquireBufferToken("job 5")
	assert.Equal(t, 2, len(poolObj.tokenHoldingJobIds))
	assert.Equal(t, PoolErrorCode(""), errorCode)

	errorCode = poolObj.AcquireBufferToken("job 6")
	assert.Equal(t, JobQueueFull, errorCode)
}

// TestAcquireToken_JobAlreadyPresentInJobStore_ShouldNotDeleteToken
// tests expired token deletion when the job already present in Job Store
func TestAcquireToken_JobAlreadyPresentInJobStore_ShouldNotDeleteToken(t *testing.T) {
	// basic pool
	poolRef := NewPool(logger, 1, 2, 100*time.Millisecond, times.NewMockedClock())
	poolObj := poolRef.(*pool)
	expirationTime := time.Now().Add(time.Duration(-unusedTokenValidityInMinutes-2) * time.Minute) // added extra expiry
	// If buffer full, delete tokens only not in job store
	// In this case, no deletion happens
	poolObj.tokenHoldingJobIds["job 1"] = &expirationTime
	poolObj.jobStore.AddJob("job 1", &JobToken{})
	poolObj.tokenHoldingJobIds["job 2"] = &expirationTime
	poolObj.jobStore.AddJob("job 2", &JobToken{})

	errorCode := poolObj.AcquireBufferToken("job 3")
	assert.Equal(t, JobQueueFull, errorCode)
}

// TestReleaseAndToken_BasicTest test basic release and acquire tokens operations
func TestReleaseAndToken_BasicTest(t *testing.T) {
	workerLimit := 5
	bufferLimit := 5
	newPool := NewPool(logger, workerLimit, bufferLimit, 100*time.Millisecond, times.NewMockedClock())
	for i := 0; i < workerLimit; i++ {
		errorCode := newPool.AcquireBufferToken(fmt.Sprintf("job %v", i))
		assert.Equal(t, PoolErrorCode(""), errorCode) //success
	}
	errorCode := newPool.AcquireBufferToken(fmt.Sprintf("job %v", workerLimit))
	assert.Equal(t, JobQueueFull, errorCode) //success
	for i := 0; i < workerLimit; i++ {
		errorCode = newPool.ReleaseBufferToken(fmt.Sprintf("job %v", i))
		assert.Equal(t, PoolErrorCode(""), errorCode) //success
	}
	errorCode = newPool.ReleaseBufferToken(fmt.Sprintf("job %v", workerLimit))
	assert.Equal(t, newPool.BufferTokensIssued(), 0) //success
}

// TestReleaseAndToken_ZeroBuffer test release and acquire tokens when the buffer is empty
func TestReleaseAndToken_ZeroBuffer(t *testing.T) {
	workerLimit := 5
	bufferLimit := 0
	newPool := NewPool(logger, workerLimit, bufferLimit, 100*time.Millisecond, times.NewMockedClock())
	for i := 0; i < workerLimit; i++ {
		errorCode := newPool.AcquireBufferToken(fmt.Sprintf("job %v", i))
		assert.Equal(t, UninitializedBuffer, errorCode)
	}
	errorCode := newPool.AcquireBufferToken(fmt.Sprintf("job %v", workerLimit))
	assert.Equal(t, UninitializedBuffer, errorCode)
	for i := 0; i < workerLimit; i++ {
		errorCode = newPool.ReleaseBufferToken(fmt.Sprintf("job %v", i))
		assert.Equal(t, UninitializedBuffer, errorCode)
	}
	errorCode = newPool.ReleaseBufferToken(fmt.Sprintf("job %v", workerLimit))
	assert.Equal(t, newPool.BufferTokensIssued(), 0)
}

// TestReleaseAndToken_InvalidJobId test release and acquire tokens when invalid job id passed
func TestReleaseAndToken_InvalidJobId(t *testing.T) {
	workerLimit := 5
	bufferLimit := 1
	newPool := NewPool(logger, workerLimit, bufferLimit, 100*time.Millisecond, times.NewMockedClock())
	errorCode := newPool.AcquireBufferToken("")
	assert.Equal(t, InvalidJobId, errorCode)

	errorCode = newPool.ReleaseBufferToken("")
	assert.Equal(t, InvalidJobId, errorCode)
	assert.Equal(t, newPool.BufferTokensIssued(), 0)
}

// TestReleaseAndToken_DuplicateCommand test release and acquire tokens when duplicate job id sent
func TestReleaseAndToken_DuplicateCommand(t *testing.T) {
	workerLimit := 5
	bufferLimit := 1
	newPool := NewPool(logger, workerLimit, bufferLimit, 100*time.Millisecond, times.NewMockedClock())
	errorCode := newPool.AcquireBufferToken("job id 1")
	assert.Equal(t, PoolErrorCode(""), errorCode) //success
	assert.Equal(t, newPool.BufferTokensIssued(), 1)
	errorCode = newPool.AcquireBufferToken("job id 1")
	assert.Equal(t, DuplicateCommand, errorCode) //success

	errorCode = newPool.ReleaseBufferToken("job id 2") // random job id
	assert.Equal(t, newPool.BufferTokensIssued(), 1)   //success

	errorCode = newPool.ReleaseBufferToken("job id 1") // random job id
	assert.Equal(t, newPool.BufferTokensIssued(), 0)   //success
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
