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

// Polls a given function
package poll

import (
	"math/rand"
	"time"

	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/carlescere/scheduler"
)

// Poller has a loop function to call when polling and a StartPolling to start polling.
type poller interface {
	loop(poll func(), log log.T, pollJob **scheduler.Job)
	StartPolling(poll func(), pollFrequencyInMinutes int, log log.T) (err error)
}

// Poll service is the service that does the polling
type PollService struct{}

var scheduleNextRun = func(j *scheduler.Job) {
	j.SkipWait <- true
}

// StartPolling runs a given poll job every pollFrequencyMinutes
func (p *PollService) StartPolling(poll func(), pollFrequencyInMinutes int, log log.T) (err error) {
	var pollJob *scheduler.Job = nil
	if pollJob, err = scheduler.Every(pollFrequencyInMinutes).Minutes().Run(func() {
		p.loop(poll, log, &pollJob)
	}); err != nil {
		log.Errorf("unable to poll. %v", err)
	}
	return
}

func (PollService) loop(poll func(), log log.T, pollJob **scheduler.Job) {

	pollStartTime := time.Now()
	poll()
	// Slow down a bit in case poll returns
	// without blocking, which may cause us to
	// flood the service with requests.
	sleepMilli(pollStartTime, 2000+rand.Intn(500))

	scheduleNextRun(*pollJob)
}

var sleepMilli = func(pollStartTime time.Time, sleepDurationInMilliseconds int) {
	if time.Since(pollStartTime) < 1*time.Second {
		time.Sleep(time.Duration(sleepDurationInMilliseconds) * time.Millisecond)
	}
}
