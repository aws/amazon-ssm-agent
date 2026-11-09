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

// Package polling schedules serial periodic tasks with fresh jitter on every wait.
package polling

import (
	"fmt"
	"math/rand"
	"sync"
	"time"

	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/amazon-ssm-agent/agent/times"
)

const (
	recurringJitter = 15 * time.Second
)

// Job runs polls serially until stopped.
type Job struct {
	stop     chan struct{}
	done     chan struct{}
	stopOnce sync.Once
}

// Start schedules task serially with an initial delay in [0, initialJitter).
// A zero initialJitter runs the initial task promptly. After initial completion,
// the first recurring wait spans (0, interval]. Every later wait adds fresh
// jitter in [-15s, +15s], allowing phases to drift without shortening the
// configured mean interval.
// Waits begin after task completion, preventing overlapping or catch-up calls.
func Start(log log.T, task func(), frequencyInMinutes int, initialJitter time.Duration) (*Job, error) {
	return start(log, task, frequencyInMinutes, initialJitter, rand.Int63n, times.DefaultClock)
}

func start(log log.T, task func(), frequencyInMinutes int, initialJitter time.Duration, random func(int64) int64, clock times.Clock) (*Job, error) {
	if frequencyInMinutes <= 0 || int64(frequencyInMinutes) > ((1<<63-1)-int64(recurringJitter))/int64(time.Minute) {
		return nil, fmt.Errorf("polling frequency must be a positive duration in minutes")
	}
	if task == nil {
		return nil, fmt.Errorf("polling task must not be nil")
	}
	interval := time.Duration(frequencyInMinutes) * time.Minute
	if initialJitter < 0 {
		return nil, fmt.Errorf("initial polling jitter must not be negative")
	}
	var initialDelay time.Duration
	if initialJitter > 0 {
		initialDelay = time.Duration(random(int64(initialJitter)))
	}
	// Use (0, interval] so a zero random draw does not produce a second
	// immediate poll following initial discovery.
	firstRecurringDelay := time.Duration(random(int64(interval)))
	if firstRecurringDelay == 0 {
		firstRecurringDelay = interval
	}
	job := &Job{
		stop: make(chan struct{}),
		done: make(chan struct{}),
	}
	log.Infof("Polling interval %v, initial delay %v, first recurring delay %v, recurring jitter +/- %v", interval, initialDelay, firstRecurringDelay, recurringJitter)
	go job.run(task, interval, initialDelay, firstRecurringDelay, random, clock)
	return job, nil
}

func (job *Job) run(task func(), interval, initialDelay, firstRecurringDelay time.Duration, random func(int64) int64, clock times.Clock) {
	defer close(job.done)
	delay := initialDelay
	initial := true
	for {
		select {
		case <-job.stop:
			return
		case <-clock.After(delay):
		}
		// Give stop priority if it arrived along with the timer.
		select {
		case <-job.stop:
			return
		default:
		}

		task()
		select {
		case <-job.stop:
			return
		default:
		}
		if initial {
			initial = false
			delay = firstRecurringDelay
		} else {
			// Draw again after every poll, including recovery from a delayed
			// timer. Wait from completion so slow polls never catch up in a burst.
			// Center jitter on zero to retain the configured mean interval.
			delay = interval + recurringJitter - time.Duration(random(int64(2*recurringJitter)+1))
		}
	}
}

// Stop cancels all future polls. An already running poll is allowed to finish.
func (job *Job) Stop() {
	if job != nil {
		job.stopOnce.Do(func() { close(job.stop) })
	}
}

// Done is closed when the job has stopped and its active task has finished.
func (job *Job) Done() <-chan struct{} {
	return job.done
}
