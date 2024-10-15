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
	"fmt"
	"math/rand"
	"runtime/debug"
	"sync"
	"time"

	"github.com/aws/amazon-ssm-agent/agent/log"
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

// loop sends replies to MDS
func (s *RunCommandService) sendReplyLoop() {
	log := s.context.Log()
	if err := s.checkStopPolicy(log); err != nil {
		return
	}

	s.sendFailedReplies()

	if s.name == mdsName {
		log.Debugf("%v's stoppolicy after polling is %v", s.name, s.processorStopPolicy)
	}
}

// loop reads messages from MDS then processes them.
func (s *RunCommandService) messagePollLoop() {
	log := s.context.Log()
	defer func() {
		if msg := recover(); msg != nil {
			log.Errorf("message poll loop panic: %v", msg)
			log.Errorf("Stacktrace:\n%s", debug.Stack())
		}
	}()
	s.messagePollWaitGroup.Add(1)
	defer s.messagePollWaitGroup.Done()
	// time lock to only have one loop active anytime.
	// this is extra insurance to prevent any race condition
	pollStartTime := time.Now()
	if s.name == mdsName {
		log.Debug("Starting message poll")
	}
	updateLastPollTime(s.name, pollStartTime)

	if err := s.checkStopPolicy(log); err != nil {
		return
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
	if pollStartTime.Equal(getLastPollTime(s.name)) {
		// skip waiting for the next scheduler polling event and start polling immediately
		scheduleNextRun(s.messagePollJob)
	} else {
		if s.name == mdsName {
			log.Debugf("Other message poll already started at %v, scheduler wait will not be skipped", getLastPollTime(s.name))
		}
	}
}

func (s *RunCommandService) checkStopPolicy(log log.T) error {
	if s.processorStopPolicy != nil {
		if s.name == mdsName {
			log.Debugf("%v's stoppolicy before polling is %v", s.name, s.processorStopPolicy)
		}
		if s.processorStopPolicy.IsHealthy() == false {
			err := fmt.Errorf("%v stopped temporarily due to internal failure. We will retry automatically after %v minutes", s.name, pollMessageFrequencyMinutes)
			log.Errorf("%v", err)
			s.reset()
			return err
		}
	} else {
		log.Debugf("creating new stop-policy.")
		s.processorStopPolicy = newStopPolicy(s.name)
	}
	return nil
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
	if s.name == mdsName {
		s.service = newMdsService(s.context)
	}
}

// Stop stops the message poller.
func (s *RunCommandService) stop() {
	log := s.context.Log()
	log.Debugf("Stopping module:%v", s.name)

	// Ask scheduler not to schedule more jobs
	if s.messagePollJob != nil {
		s.messagePollJob.Quit <- true
	}
	if s.sendReplyJob != nil {
		s.sendReplyJob.Quit <- true
	}

	// Stop any ongoing calls
	s.service.Stop()

	// Wait for ongoing messagePoll loops to terminate
	log.Debugf("Waiting for polling function to return")
	s.messagePollWaitGroup.Wait()
	log.Debugf("ended message poll wait")
}

// pollOnce calls GetMessages once and processes the result.
func (s *RunCommandService) pollOnce() {
	log := s.context.Log()
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
		log.Debugf("Finished message poll")
	}
}
