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

// Package runcommand implements runcommand core processing module
package runcommand

import (
	"math/rand"
	"sync"
	"time"

	"github.com/aws/amazon-ssm-agent/agent/sdkutil"
	"github.com/carlescere/scheduler"
)

var lastPollTimeMap map[string]time.Time = make(map[string]time.Time)
var lock sync.RWMutex

var processMessage = (*RunCommandService).processMessage

func updateLastPollTime(processorType string, currentTime time.Time) {
	lock.Lock()
	defer lock.Unlock()
	lastPollTimeMap[processorType] = currentTime
}

func getLastPollTime(processorType string) time.Time {
	lock.RLock()
	defer lock.RUnlock()
	return lastPollTimeMap[processorType]
}

// loop reads messages from MDS then processes them.
func (s *RunCommandService) loop() {
	// time lock to only have one loop active anytime.
	// this is extra insurance to prevent any race condition
	pollStartTime := time.Now()
	updateLastPollTime(s.name, pollStartTime)

	log := s.context.Log()
	if !s.isDone() {
		if s.processorStopPolicy != nil {
			if s.name == mdsName {
				log.Debugf("%v's stoppolicy before polling is %v", s.name, s.processorStopPolicy)
			}
			if s.processorStopPolicy.IsHealthy() == false {
				log.Errorf("%v stopped temporarily due to internal failure. We will retry automatically after %v minutes", s.name, pollMessageFrequencyMinutes)
				s.reset()
				return
			}
		} else {
			log.Debugf("creating new stop-policy.")
			s.processorStopPolicy = newStopPolicy(s.name)
		}

		s.pollOnce()
		if s.name == mdsName {
			log.Debugf("%v's stoppolicy after polling is %v", s.name, s.processorStopPolicy)
		}

		// Slow down a bit in case GetMessages returns
		// without blocking, which may cause us to
		// flood the service with requests.
		if time.Since(pollStartTime) < 1*time.Second {
			time.Sleep(time.Duration(2000+rand.Intn(500)) * time.Millisecond)
		}

		// check if any other poll loop has started in the meantime
		// to prevent any possible race condition due to the scheduler
		if getLastPollTime(s.name) == pollStartTime {
			// skip waiting for the next scheduler polling event and start polling immediately
			scheduleNextRun(s.messagePollJob)
		}
	}
}

var scheduleNextRun = func(j *scheduler.Job) {
	j.SkipWait <- true
}

func (s *RunCommandService) reset() {
	log := s.context.Log()
	log.Debugf("Resetting processor:%v", s.name)
	// reset stop policy and let the scheduler start the polling after pollMessageFrequencyMinutes timeout
	s.processorStopPolicy.ResetErrorCount()

	// creating a new mds service object for the retry
	// this is extra insurance to avoid service object getting corrupted - adding resiliency
	config := s.context.AppConfig()
	if s.name == mdsName {
		s.service = newMdsService(config)
	}
}

// Stop stops the message poller.
func (s *RunCommandService) stop() {
	log := s.context.Log()
	log.Debugf("Stopping processor:%v", s.name)
	s.service.Stop()

	// close channel; subsequent calls to isDone will return true
	if !s.isDone() {
		close(s.stopSignal)
	}

	if s.messagePollJob != nil {
		s.messagePollJob.Quit <- true
	}

	if s.assocProcessor != nil {
		s.assocProcessor.Stop()
	}
}

// isDone returns true if a stop has been requested, false otherwise.
func (s *RunCommandService) isDone() bool {
	select {
	case <-s.stopSignal:
		// received signal or channel already closed
		return true
	default:
		return false
	}
}

// pollOnce calls GetMessages once and processes the result.
func (s *RunCommandService) pollOnce() {
	log := s.context.Log()
	if s.name == mdsName {
		log.Debugf("Polling for messages")
	}
	messages, err := s.service.GetMessages(log, s.config.InstanceID)
	if err != nil {
		sdkutil.HandleAwsError(log, err, s.processorStopPolicy)
		return
	}
	if len(messages.Messages) > 0 {
		log.Debugf("Got %v messages", len(messages.Messages))
	}

	for _, msg := range messages.Messages {
		processMessage(s, msg)
	}
	if s.name == mdsName {
		log.Debugf("Done poll once")
	}
}
