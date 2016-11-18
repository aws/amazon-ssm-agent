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

// Package processor implements MDS plugin processor
// processor_scheduler contains the GetMessages Scheduler implementation
package processor

import (
	"math/rand"
	"sync"
	"time"

	"github.com/aws/amazon-ssm-agent/agent/sdkutil"
	"github.com/carlescere/scheduler"
)

var lastPollTimeMap map[string]time.Time = make(map[string]time.Time)
var lock sync.RWMutex

var processMessage = (*Processor).processMessage

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
func (p *Processor) loop() {
	// time lock to only have one loop active anytime.
	// this is extra insurance to prevent any race condition
	pollStartTime := time.Now()
	updateLastPollTime(p.name, pollStartTime)

	log := p.context.Log()
	if !p.isDone() {
		if p.processorStopPolicy != nil {
			if p.name == mdsName {
				log.Debugf("%v's stoppolicy before polling is %v", p.name, p.processorStopPolicy)
			}
			if p.processorStopPolicy.IsHealthy() == false {
				log.Errorf("%v stopped temporarily due to internal failure. We will retry automatically after %v minutes", p.name, pollMessageFrequencyMinutes)
				p.reset()
				return
			}
		} else {
			log.Debugf("creating new stop-policy.")
			p.processorStopPolicy = newStopPolicy(p.name)
		}

		p.pollOnce()
		if p.name == mdsName {
			log.Debugf("%v's stoppolicy after polling is %v", p.name, p.processorStopPolicy)
		}

		// Slow down a bit in case GetMessages returns
		// without blocking, which may cause us to
		// flood the service with requests.
		if time.Since(pollStartTime) < 1*time.Second {
			time.Sleep(time.Duration(2000+rand.Intn(500)) * time.Millisecond)
		}

		// check if any other poll loop has started in the meantime
		// to prevent any possible race condition due to the scheduler
		if getLastPollTime(p.name) == pollStartTime {
			// skip waiting for the next scheduler polling event and start polling immediately
			scheduleNextRun(p.messagePollJob)
		}
	}
}

var scheduleNextRun = func(j *scheduler.Job) {
	j.SkipWait <- true
}

func (p *Processor) reset() {
	log := p.context.Log()
	log.Debugf("Resetting processor:%v", p.name)
	// reset stop policy and let the scheduler start the polling after pollMessageFrequencyMinutes timeout
	p.processorStopPolicy.ResetErrorCount()

	// creating a new mds service object for the retry
	// this is extra insurance to avoid service object getting corrupted - adding resiliency
	config := p.context.AppConfig()
	if p.name == mdsName {
		p.service = newMdsService(config)
	}
}

// Stop stops the MDSProcessor.
func (p *Processor) stop() {
	log := p.context.Log()
	log.Debugf("Stopping processor:%v", p.name)
	p.service.Stop()

	// close channel; subsequent calls to isDone will return true
	if !p.isDone() {
		close(p.stopSignal)
	}

	if p.messagePollJob != nil {
		p.messagePollJob.Quit <- true
	}

	if p.assocProcessor != nil {
		p.assocProcessor.Stop()
	}
}

// isDone returns true if a stop has been requested, false otherwise.
func (p *Processor) isDone() bool {
	select {
	case <-p.stopSignal:
		// received signal or channel already closed
		return true
	default:
		return false
	}
}

// pollOnce calls GetMessages once and processes the result.
func (p *Processor) pollOnce() {
	log := p.context.Log()
	if p.name == mdsName {
		log.Debugf("Polling for messages")
	}
	messages, err := p.service.GetMessages(log, p.config.InstanceID)
	if err != nil {
		sdkutil.HandleAwsError(log, err, p.processorStopPolicy)
		return
	}
	if len(messages.Messages) > 0 {
		log.Debugf("Got %v messages", len(messages.Messages))
	}

	for _, msg := range messages.Messages {
		processMessage(p, msg)
	}
	if p.name == mdsName {
		log.Debugf("Done poll once")
	}
}
