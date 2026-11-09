// Copyright 2026 Amazon.com, Inc. or its affiliates. All Rights Reserved.
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

package scheduler

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"

	logmocks "github.com/aws/amazon-ssm-agent/agent/mocks/log"
	"github.com/stretchr/testify/require"
)

type timerRequest struct {
	delay time.Duration
	ready chan time.Time
}

type manualClock struct {
	mu       sync.Mutex
	now      time.Time
	requests chan timerRequest
}

func newManualClock() *manualClock {
	return &manualClock{now: time.Unix(0, 0), requests: make(chan timerRequest, 32)}
}

func (c *manualClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

func (c *manualClock) After(delay time.Duration) <-chan time.Time {
	request := timerRequest{delay: delay, ready: make(chan time.Time, 1)}
	c.requests <- request
	return request.ready
}

func (c *manualClock) advance(delay time.Duration) {
	c.mu.Lock()
	c.now = c.now.Add(delay)
	c.mu.Unlock()
}

func (c *manualClock) fire(request timerRequest) {
	c.advance(request.delay)
	request.ready <- c.Now()
}

func (c *manualClock) nextWait(t *testing.T) timerRequest {
	t.Helper()
	select {
	case request := <-c.requests:
		return request
	case <-time.After(time.Second):
		t.Fatal("scheduler did not wait for a timer")
		return timerRequest{}
	}
}

func nextCall(t *testing.T, calls <-chan time.Time) time.Time {
	t.Helper()
	select {
	case now := <-calls:
		return now
	case <-time.After(time.Second):
		t.Fatal("poll did not run")
		return time.Time{}
	}
}

func waitStopped(t *testing.T, job *Job) {
	t.Helper()
	select {
	case <-job.done:
	case <-time.After(time.Second):
		t.Fatal("scheduler did not stop")
	}
}

func testJob(t *testing.T, clock *manualClock, task func(), minutes int, initial, phase time.Duration) *Job {
	t.Helper()
	draws := 0
	random := func(bound int64) int64 {
		draws++
		switch draws {
		case 1:
			require.Equal(t, int64(30*time.Second), bound)
			return int64(initial)
		case 2:
			require.Equal(t, int64(time.Duration(minutes)*time.Minute), bound)
			return int64(phase)
		default:
			t.Errorf("recurring phase was redrawn")
			return 0
		}
	}
	job, err := createScheduler(logmocks.NewMockLog(), task, minutes, random, clock)
	require.NoError(t, err)
	t.Cleanup(func() {
		Stop(job)
		waitStopped(t, job)
	})
	return job
}

func TestFullPeriodPhaseKeepsTenMinuteCadence(t *testing.T) {
	clock := newManualClock()
	calls := make(chan time.Time, 10)
	testJob(t, clock, func() { calls <- clock.Now() }, 10, 12*time.Second, 287*time.Second)
	for _, expectedDelay := range []time.Duration{12 * time.Second, 287 * time.Second, 10 * time.Minute, 10 * time.Minute, 10 * time.Minute} {
		request := clock.nextWait(t)
		require.Equal(t, expectedDelay, request.delay)
		clock.fire(request)
		require.Equal(t, clock.Now(), nextCall(t, calls))
	}
}

func TestPhaseCanReachEndOfConfiguredInterval(t *testing.T) {
	for _, minutes := range []int{5, 10, 60} {
		t.Run((time.Duration(minutes) * time.Minute).String(), func(t *testing.T) {
			clock := newManualClock()
			calls := make(chan time.Time, 2)
			phase := time.Duration(minutes)*time.Minute - time.Nanosecond
			testJob(t, clock, func() { calls <- clock.Now() }, minutes, 0, phase)
			clock.fire(clock.nextWait(t))
			nextCall(t, calls)
			require.Equal(t, phase, clock.nextWait(t).delay)
		})
	}
}

func TestZeroPhaseDrawWaitsAnInterval(t *testing.T) {
	clock := newManualClock()
	calls := make(chan time.Time, 2)
	testJob(t, clock, func() { calls <- clock.Now() }, 10, 0, 0)
	clock.fire(clock.nextWait(t))
	nextCall(t, calls)
	require.Equal(t, 10*time.Minute, clock.nextWait(t).delay)
}

func TestExplicitWakePreservesRecurringPhase(t *testing.T) {
	clock := newManualClock()
	calls := make(chan time.Time, 4)
	job := testJob(t, clock, func() { calls <- clock.Now() }, 10, 12*time.Second, 287*time.Second)
	clock.fire(clock.nextWait(t))
	nextCall(t, calls)
	require.Equal(t, 287*time.Second, clock.nextWait(t).delay)
	clock.advance(100 * time.Second)
	ScheduleNextRun(job)
	require.Equal(t, time.Unix(112, 0), nextCall(t, calls))
	request := clock.nextWait(t)
	require.Equal(t, 187*time.Second, request.delay)
	clock.fire(request)
	require.Equal(t, time.Unix(299, 0), nextCall(t, calls))
	require.Equal(t, 10*time.Minute, clock.nextWait(t).delay)
}

func TestSlowPollSkipsMissedSlotsWithoutCatchUp(t *testing.T) {
	clock := newManualClock()
	calls := make(chan time.Time, 4)
	count := 0
	testJob(t, clock, func() {
		count++
		calls <- clock.Now()
		if count == 2 {
			clock.advance(25 * time.Minute)
		}
	}, 10, 10*time.Second, 2*time.Minute)
	clock.fire(clock.nextWait(t))
	nextCall(t, calls)
	clock.fire(clock.nextWait(t))
	require.Equal(t, time.Unix(130, 0), nextCall(t, calls))
	request := clock.nextWait(t)
	require.Equal(t, 5*time.Minute, request.delay)
	clock.fire(request)
	require.Equal(t, time.Unix(1930, 0), nextCall(t, calls))
	require.Equal(t, 10*time.Minute, clock.nextWait(t).delay)
}

func TestStopCancelsInitialAndRecurringDelays(t *testing.T) {
	for _, afterInitial := range []bool{false, true} {
		name := "initial"
		if afterInitial {
			name = "recurring"
		}
		t.Run(name, func(t *testing.T) {
			clock := newManualClock()
			var count int32
			job := testJob(t, clock, func() { atomic.AddInt32(&count, 1) }, 10, 20*time.Second, 9*time.Minute)
			request := clock.nextWait(t)
			if afterInitial {
				clock.fire(request)
				request = clock.nextWait(t)
			}
			Stop(job)
			Stop(job)
			waitStopped(t, job)
			before := atomic.LoadInt32(&count)
			clock.fire(request)
			ScheduleNextRun(job)
			require.Equal(t, before, atomic.LoadInt32(&count))
		})
	}
}

func TestPollsDoNotOverlapAndWakeRequestsCoalesce(t *testing.T) {
	clock := newManualClock()
	calls := make(chan time.Time, 4)
	release := make(chan struct{})
	releaseOnce := sync.Once{}
	unblock := func() { releaseOnce.Do(func() { close(release) }) }
	defer unblock()
	job := testJob(t, clock, func() {
		calls <- clock.Now()
		<-release
	}, 10, 0, 5*time.Minute)
	clock.fire(clock.nextWait(t))
	nextCall(t, calls)
	queued := make(chan struct{})
	go func() {
		for i := 0; i < 100; i++ {
			ScheduleNextRun(job)
		}
		close(queued)
	}()
	select {
	case <-queued:
	case <-time.After(time.Second):
		t.Fatal("wake requests blocked behind an active poll")
	}
	select {
	case <-calls:
		t.Fatal("a second poll overlapped the active poll")
	default:
	}
	unblock()
	// One wait follows initial discovery; the queued wake bypasses that wait.
	clock.nextWait(t)
	nextCall(t, calls)
	clock.nextWait(t)
	Stop(job)
	waitStopped(t, job)
	require.Empty(t, calls, "coalesced wake requests produced extra polls")
}

func TestStopAllowsActivePollToFinish(t *testing.T) {
	clock := newManualClock()
	calls := make(chan time.Time, 2)
	release := make(chan struct{})
	job := testJob(t, clock, func() {
		calls <- clock.Now()
		<-release
	}, 10, 0, time.Minute)
	defer close(release)
	clock.fire(clock.nextWait(t))
	nextCall(t, calls)
	Stop(job)
	select {
	case <-job.done:
		t.Fatal("active poll was not allowed to finish")
	default:
	}
}

func TestInvalidSchedulerInputs(t *testing.T) {
	for _, minutes := range []int{0, -1} {
		job, err := CreateScheduler(logmocks.NewMockLog(), func() {}, minutes)
		require.Error(t, err)
		require.Nil(t, job)
	}
	job, err := CreateScheduler(logmocks.NewMockLog(), nil, 10)
	require.Error(t, err)
	require.Nil(t, job)
}
