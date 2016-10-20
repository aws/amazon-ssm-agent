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
	"math/rand"
	"time"

	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/carlescere/scheduler"
)

const (
	defaultSleepDurationInMilliSeconds int = 30000
)

// CreateScheduler runs a given poll job every pollFrequencyMinutes
func CreateScheduler(log log.T, task func(), frequencyInMinutes int) (job *scheduler.Job, err error) {
	if job, err = scheduler.Every(frequencyInMinutes).Minutes().Run(func() {
		loop(task, log, job)
	}); err != nil {
		return nil, log.Errorf("unable to create scheduler, %v", err)
	}
	return job, nil
}

// Stop stops the scheduler.
func Stop(job *scheduler.Job) {
	if job != nil {
		job.Quit <- true
	}
}

// loop executes the task provided when creates scheduler
func loop(task func(), log log.T, job *scheduler.Job) {
	// time lock to only have one loop active anytime.
	// this is extra insurance to prevent any race condition
	taskStartTime := time.Now()

	sleepMilli(taskStartTime, defaultSleepDurationInMilliSeconds)

	task()
}

// ScheduleNextRun skips waiting and schedule next run immediately
func ScheduleNextRun(j *scheduler.Job) {
	j.SkipWait <- true
}

var sleepMilli = func(pollStartTime time.Time, sleepDurationInMilliseconds int) {
	sleepDurationInMilliseconds = rand.Intn(sleepDurationInMilliseconds)
	if time.Since(pollStartTime) < 1*time.Second {
		time.Sleep(time.Duration(sleepDurationInMilliseconds) * time.Millisecond)
	}
}
