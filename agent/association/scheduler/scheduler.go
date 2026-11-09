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

// Package scheduler provides ability to create scheduled job
package scheduler

import (
	"fmt"
	"math/rand"
	"sync"
	"time"

	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/amazon-ssm-agent/agent/times"
)

const initialJitter = 30 * time.Second

// Job serializes polls and supports stopping or requesting an earlier poll.
type Job struct {
	stop     chan struct{}
	wake     chan struct{}
	done     chan struct{}
	stopOnce sync.Once
}

// CreateScheduler preserves prompt initial discovery, then spreads recurring
// polls across the full configured interval. The phase is chosen once per job,
// so healthy recurring polls remain one interval apart.
func CreateScheduler(log log.T, task func(), frequencyInMinutes int) (*Job, error) {
	return createScheduler(log, task, frequencyInMinutes, rand.Int63n, times.DefaultClock)
}

func createScheduler(log log.T, task func(), frequencyInMinutes int, random func(int64) int64, clock times.Clock) (*Job, error) {
	if frequencyInMinutes <= 0 || int64(frequencyInMinutes) > (1<<63-1)/int64(time.Minute) {
		return nil, fmt.Errorf("association polling frequency must be a positive duration in minutes")
	}
	if task == nil {
		return nil, fmt.Errorf("association polling task must not be nil")
	}
	interval := time.Duration(frequencyInMinutes) * time.Minute
	initialDelay := time.Duration(random(int64(initialJitter)))
	// Use (0, interval] so a zero random draw does not produce a second
	// immediate poll following initial discovery.
	phaseDelay := time.Duration(random(int64(interval)))
	if phaseDelay == 0 {
		phaseDelay = interval
	}
	job := &Job{
		stop: make(chan struct{}),
		wake: make(chan struct{}, 1),
		done: make(chan struct{}),
	}
	log.Infof("Association polling interval %v, initial delay %v, recurring phase delay %v", interval, initialDelay, phaseDelay)
	go job.run(task, interval, initialDelay, phaseDelay, clock)
	return job, nil
}

func (job *Job) run(task func(), interval, initialDelay, phaseDelay time.Duration, clock times.Clock) {
	defer close(job.done)
	next := clock.Now().Add(initialDelay)
	initial := true
	for {
		select {
		case <-job.stop:
			return
		case <-job.wake:
		case <-clock.After(next.Sub(clock.Now())):
		}
		// Give stop priority if it arrived along with a timer or wake request.
		select {
		case <-job.stop:
			return
		default:
		}

		task()
		now := clock.Now()
		if initial {
			initial = false
			next = now.Add(phaseDelay)
		} else if !next.After(now) {
			// Keep the selected phase even after slow polls or explicit wakes.
			// Skip missed slots instead of overlapping polls or catching up in a burst.
			next = next.Add((now.Sub(next)/interval + 1) * interval)
		}
	}
}

// Stop cancels all future polls. An already running poll is allowed to finish.
func Stop(job *Job) {
	if job != nil {
		job.stopOnce.Do(func() { close(job.stop) })
	}
}

// ScheduleNextRun requests a poll without shifting the recurring phase.
// Requests arriving while a poll is running coalesce into one pending request.
func ScheduleNextRun(job *Job) {
	if job == nil {
		return
	}
	select {
	case <-job.stop:
		return
	default:
	}
	select {
	case job.wake <- struct{}{}:
	default:
	}
}
