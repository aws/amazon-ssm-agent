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

package polling

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

func testJob(t *testing.T, clock *manualClock, task func(), minutes int, initial, firstRecurring time.Duration, jitters ...time.Duration) *Job {
	t.Helper()
	return testJobWithInitialJitter(t, clock, task, minutes, 30*time.Second, initial, firstRecurring, jitters...)
}

func testJobWithInitialJitter(t *testing.T, clock *manualClock, task func(), minutes int, initialJitter, initial, firstRecurring time.Duration, jitters ...time.Duration) *Job {
	t.Helper()
	draws := 0
	if initialJitter == 0 {
		// An immediate initial report must not consume an initial jitter draw.
		draws = 1
	}
	random := func(bound int64) int64 {
		draws++
		switch draws {
		case 1:
			require.Equal(t, int64(30*time.Second), bound)
			return int64(initial)
		case 2:
			require.Equal(t, int64(time.Duration(minutes)*time.Minute), bound)
			return int64(firstRecurring)
		default:
			if bound != int64(30*time.Second)+1 {
				t.Errorf("recurring jitter bound = %v, want %v", bound, int64(30*time.Second)+1)
			}
			if index := draws - 3; index < len(jitters) {
				return int64(jitters[index])
			}
			return 0
		}
	}
	job, err := start(logmocks.NewMockLog(), task, minutes, initialJitter, random, clock)
	require.NoError(t, err)
	t.Cleanup(func() {
		job.Stop()
		waitStopped(t, job)
	})
	return job
}

func TestImmediateReportThenFreshJitterAtConfiguredHeartbeatIntervals(t *testing.T) {
	for _, minutes := range []int{5, 12, 60} {
		t.Run((time.Duration(minutes) * time.Minute).String(), func(t *testing.T) {
			clock := newManualClock()
			calls := make(chan time.Time, 8)
			interval := time.Duration(minutes) * time.Minute
			testJobWithInitialJitter(t, clock, func() { calls <- clock.Now() }, minutes,
				0, 0, interval-time.Nanosecond, 0, 30*time.Second, 15*time.Second)
			for _, expectedDelay := range []time.Duration{
				0, interval - time.Nanosecond,
				interval + 15*time.Second, interval - 15*time.Second, interval,
			} {
				request := clock.nextWait(t)
				require.Equal(t, expectedDelay, request.delay)
				clock.fire(request)
				require.Equal(t, clock.Now(), nextCall(t, calls))
			}
		})
	}
}

func TestImmediateReportsSeparateAgainAfterSharedPause(t *testing.T) {
	clocks := []*manualClock{newManualClock(), newManualClock()}
	calls := []chan time.Time{make(chan time.Time, 8), make(chan time.Time, 8)}
	for i, clock := range clocks {
		testJobWithInitialJitter(t, clock, func() { calls[i] <- clock.Now() }, 12,
			0, 0, 3*time.Minute, 15*time.Second, time.Duration(i)*30*time.Second)
		for poll := 0; poll < 2; poll++ {
			clock.fire(clock.nextWait(t))
			nextCall(t, calls[i])
		}
		request := clock.nextWait(t)
		clock.advance(time.Unix(1800, 0).Sub(clock.Now()))
		request.ready <- clock.Now()
		require.Equal(t, time.Unix(1800, 0), nextCall(t, calls[i]))
	}
	starts := make([]time.Time, 2)
	for i, clock := range clocks {
		clock.fire(clock.nextWait(t))
		starts[i] = nextCall(t, calls[i])
	}
	require.Equal(t, 30*time.Second, starts[0].Sub(starts[1]),
		"fresh recurring jitter separates reports following a shared pause")
}

func TestStopDuringImmediateReportPreventsRecurringJob(t *testing.T) {
	clock := newManualClock()
	calls := make(chan time.Time, 2)
	release := make(chan struct{})
	job := testJobWithInitialJitter(t, clock, func() {
		calls <- clock.Now()
		<-release
	}, 12, 0, 0, time.Minute)
	defer close(release)
	request := clock.nextWait(t)
	require.Zero(t, request.delay)
	clock.fire(request)
	nextCall(t, calls)
	job.Stop()
	clock.advance(30 * time.Minute)
	select {
	case <-clock.requests:
		t.Fatal("stopping during initial reporting started a recurring timer")
	default:
	}
	select {
	case <-job.Done():
		t.Fatal("stop did not allow the initial report to finish")
	default:
	}
}

func TestEveryRecurringWaitDrawsFreshJitter(t *testing.T) {
	clock := newManualClock()
	calls := make(chan time.Time, 10)
	testJob(t, clock, func() { calls <- clock.Now() }, 10, 12*time.Second, 287*time.Second,
		0, 30*time.Second, 15*time.Second, 4*time.Second)
	for _, expectedDelay := range []time.Duration{
		12 * time.Second, 287 * time.Second,
		10*time.Minute + 15*time.Second, 9*time.Minute + 45*time.Second,
		10 * time.Minute, 10*time.Minute + 11*time.Second,
	} {
		request := clock.nextWait(t)
		require.Equal(t, expectedDelay, request.delay)
		clock.fire(request)
		require.Equal(t, clock.Now(), nextCall(t, calls))
	}
}

func TestFirstRecurringDelayCanReachEndOfConfiguredInterval(t *testing.T) {
	for _, minutes := range []int{5, 10, 12, 60} {
		t.Run((time.Duration(minutes) * time.Minute).String(), func(t *testing.T) {
			clock := newManualClock()
			calls := make(chan time.Time, 2)
			firstRecurring := time.Duration(minutes)*time.Minute - time.Nanosecond
			testJob(t, clock, func() { calls <- clock.Now() }, minutes, 0, firstRecurring)
			clock.fire(clock.nextWait(t))
			nextCall(t, calls)
			require.Equal(t, firstRecurring, clock.nextWait(t).delay)
		})
	}
}

func TestZeroFirstRecurringDrawWaitsAnInterval(t *testing.T) {
	clock := newManualClock()
	calls := make(chan time.Time, 2)
	testJob(t, clock, func() { calls <- clock.Now() }, 10, 0, 0)
	clock.fire(clock.nextWait(t))
	nextCall(t, calls)
	require.Equal(t, 10*time.Minute, clock.nextWait(t).delay)
}

func TestRecurringJitterUsesConfiguredInterval(t *testing.T) {
	for _, minutes := range []int{5, 10, 12, 60} {
		t.Run((time.Duration(minutes) * time.Minute).String(), func(t *testing.T) {
			clock := newManualClock()
			calls := make(chan time.Time, 4)
			testJob(t, clock, func() { calls <- clock.Now() }, minutes, 0, time.Minute,
				30*time.Second, 0)
			for i := 0; i < 2; i++ {
				clock.fire(clock.nextWait(t))
				nextCall(t, calls)
			}
			request := clock.nextWait(t)
			require.Equal(t, time.Duration(minutes)*time.Minute-15*time.Second, request.delay)
			clock.fire(request)
			nextCall(t, calls)
			require.Equal(t, time.Duration(minutes)*time.Minute+15*time.Second, clock.nextWait(t).delay)
		})
	}
}

func TestSlowPollWaitsAfterCompletionWithoutCatchUp(t *testing.T) {
	clock := newManualClock()
	calls := make(chan time.Time, 4)
	count := 0
	testJob(t, clock, func() {
		count++
		calls <- clock.Now()
		if count == 2 {
			clock.advance(25 * time.Minute)
		}
	}, 10, 10*time.Second, 2*time.Minute, 30*time.Second, 15*time.Second)
	clock.fire(clock.nextWait(t))
	nextCall(t, calls)
	clock.fire(clock.nextWait(t))
	require.Equal(t, time.Unix(130, 0), nextCall(t, calls))
	request := clock.nextWait(t)
	require.Equal(t, 9*time.Minute+45*time.Second, request.delay)
	clock.fire(request)
	require.Equal(t, time.Unix(2215, 0), nextCall(t, calls))
	require.Equal(t, 10*time.Minute, clock.nextWait(t).delay)
}

func TestSynchronizedPollsSpreadOutOverTime(t *testing.T) {
	clocks := []*manualClock{newManualClock(), newManualClock()}
	calls := []chan time.Time{make(chan time.Time, 8), make(chan time.Time, 8)}
	testJob(t, clocks[0], func() { calls[0] <- clocks[0].Now() }, 10, 0, 2*time.Minute,
		0, 0, 0, 0)
	testJob(t, clocks[1], func() { calls[1] <- clocks[1].Now() }, 10, 0, 2*time.Minute,
		30*time.Second, 30*time.Second, 30*time.Second, 30*time.Second)
	for poll := 0; poll < 6; poll++ {
		starts := make([]time.Time, 2)
		for i, clock := range clocks {
			clock.fire(clock.nextWait(t))
			starts[i] = nextCall(t, calls[i])
		}
		if poll < 2 {
			require.Equal(t, starts[0], starts[1], "initial polling phases match")
		} else {
			require.Equal(t, time.Duration(poll-1)*30*time.Second, starts[0].Sub(starts[1]),
				"fresh jitter accumulates instead of returning to the initial phase")
		}
	}
}

func TestDelayedTimersSpreadOutAgain(t *testing.T) {
	clocks := []*manualClock{newManualClock(), newManualClock()}
	calls := []chan time.Time{make(chan time.Time, 8), make(chan time.Time, 8)}
	testJob(t, clocks[0], func() { calls[0] <- clocks[0].Now() }, 10, 0, 2*time.Minute,
		10*time.Second, 0, 15*time.Second)
	testJob(t, clocks[1], func() { calls[1] <- clocks[1].Now() }, 10, 0, 7*time.Minute,
		20*time.Second, 30*time.Second, 5*time.Second)
	for i, clock := range clocks {
		// Complete initial discovery and the first recurring poll.
		for poll := 0; poll < 2; poll++ {
			clock.fire(clock.nextWait(t))
			nextCall(t, calls[i])
		}
		request := clock.nextWait(t)

		// A shared pause makes both steady-state timers wake at minute 25.
		clock.advance(time.Unix(1500, 0).Sub(clock.Now()))
		request.ready <- clock.Now()
		require.Equal(t, time.Unix(1500, 0), nextCall(t, calls[i]))
	}

	// Fresh jitter separates the next polls despite the common recovery time.
	for i, clock := range clocks {
		request := clock.nextWait(t)
		require.Equal(t, 10*time.Minute+15*time.Second-time.Duration(i)*30*time.Second, request.delay)
		clock.fire(request)
		require.Equal(t, time.Unix(2115-int64(i)*30, 0), nextCall(t, calls[i]))
	}
	require.Equal(t, 10*time.Minute, clocks[0].nextWait(t).delay)
	require.Equal(t, 10*time.Minute+10*time.Second, clocks[1].nextWait(t).delay)
}

func TestStopCancelsInitialAndRecurringDelays(t *testing.T) {
	for pollsBeforeStop, name := range []string{"initial", "first recurring", "steady state"} {
		t.Run(name, func(t *testing.T) {
			clock := newManualClock()
			var count int32
			job := testJob(t, clock, func() { atomic.AddInt32(&count, 1) }, 10, 20*time.Second, 9*time.Minute)
			request := clock.nextWait(t)
			for i := 0; i < pollsBeforeStop; i++ {
				clock.fire(request)
				request = clock.nextWait(t)
			}
			job.Stop()
			job.Stop()
			waitStopped(t, job)
			before := atomic.LoadInt32(&count)
			clock.fire(request)
			require.Equal(t, before, atomic.LoadInt32(&count))
		})
	}
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
	clock.advance(30 * time.Minute)
	select {
	case <-clock.requests:
		t.Fatal("scheduler started another timer while a poll was active")
	default:
	}
	require.Empty(t, calls, "a second poll overlapped the active poll")
	job.Stop()
	select {
	case <-job.done:
		t.Fatal("active poll was not allowed to finish")
	default:
	}
}

func TestInvalidSchedulerInputs(t *testing.T) {
	for _, minutes := range []int{0, -1} {
		job, err := Start(logmocks.NewMockLog(), func() {}, minutes, 30*time.Second)
		require.Error(t, err)
		require.Nil(t, job)
	}
	job, err := Start(logmocks.NewMockLog(), nil, 10, 30*time.Second)
	require.Error(t, err)
	require.Nil(t, job)
}

func TestRecurringJitterPreservesConfiguredMean(t *testing.T) {
	for _, minutes := range []int{5, 10, 12, 60} {
		t.Run((time.Duration(minutes) * time.Minute).String(), func(t *testing.T) {
			clock := newManualClock()
			calls := make(chan time.Time, 16)
			// Balanced draws span both endpoints and intermediate values. Their
			// average wait must equal the configured interval, not shorten it.
			draws := []time.Duration{0, 30 * time.Second, 5 * time.Second, 25 * time.Second,
				10 * time.Second, 20 * time.Second, 15 * time.Second, 15 * time.Second}
			testJob(t, clock, func() { calls <- clock.Now() }, minutes, 0, time.Minute, draws...)
			for poll := 0; poll < 2; poll++ {
				clock.fire(clock.nextWait(t))
				nextCall(t, calls)
			}
			started := clock.Now()
			for range draws {
				clock.fire(clock.nextWait(t))
				nextCall(t, calls)
			}
			require.Equal(t, time.Duration(len(draws)*minutes)*time.Minute, clock.Now().Sub(started))
		})
	}
}
