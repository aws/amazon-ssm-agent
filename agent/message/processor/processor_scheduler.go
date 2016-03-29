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

var lastPollTime time.Time
var lock sync.RWMutex

func updateLastPollTime(currentTime time.Time) {
	lock.Lock()
	defer lock.Unlock()
	lastPollTime = currentTime
}

func getLastPollTime() time.Time {
	lock.RLock()
	defer lock.RUnlock()
	return lastPollTime
}

// loop reads messages from MDS then processes them.
func (p *Processor) loop() {
	// time lock to only have one loop active anytime.
	// this is extra insurance to prevent any race condition
	pollStartTime := time.Now()
	updateLastPollTime(pollStartTime)

	log := p.context.Log()
	if !p.isDone() {
		if p.processorStopPolicy != nil {
			log.Debugf("mdsprocessor's stoppolicy before polling is %v", p.processorStopPolicy)
			if p.processorStopPolicy.IsHealthy() == false {
				log.Errorf("mdsprocessor stopped temporarily due to internal failure. We will retry automatically after %v minutes", pollMessageFrequencyMinutes)
				p.reset()
				return
			}
		} else {
			log.Debugf("creating new stop-policy.")
			p.processorStopPolicy = newStopPolicy()
		}

		p.pollOnce()
		log.Debugf("mdsprocessor's stoppolicy after polling is %v", p.processorStopPolicy)

		// Slow down a bit in case GetMessages returns
		// without blocking, which may cause us to
		// flood the service with requests.
		if time.Since(pollStartTime) < 1*time.Second {
			time.Sleep(time.Duration(2000+rand.Intn(500)) * time.Millisecond)
		}

		// check if any other poll loop has started in the meantime
		// to prevent any possible race condition due to the scheduler
		if getLastPollTime() == pollStartTime {
			// skip waiting for the next scheduler polling event and start polling immediately
			scheduleNextRun(p.messagePollJob)
		}
	}
}

var scheduleNextRun = func(j *scheduler.Job) {
	j.SkipWait <- true
}

func (p *Processor) reset() {
	// reset stop policy and let the scheduler start the polling after pollMessageFrequencyMinutes timeout
	p.processorStopPolicy.ResetErrorCount()

	// creating a new mds service object for the retry
	// this is extra insurance to avoid service object getting corrupted - adding resiliency
	config := p.context.AppConfig()
	p.service = newMdsService(config)
}

// Stop stops the MDSProcessor.
func (p *Processor) stop() {
	p.service.Stop()

	// close channel; subsequent calls to isDone will return true
	if !p.isDone() {
		close(p.stopSignal)
	}

	if p.messagePollJob != nil {
		p.messagePollJob.Quit <- true
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
	log.Debugf("Polling for messages")
	messages, err := p.service.GetMessages(log, p.config.InstanceID)
	if err != nil {
		sdkutil.HandleAwsError(log, err, p.processorStopPolicy)
		return
	}
	log.Debugf("Got %v messages", len(messages.Messages))

	for _, msg := range messages.Messages {
		p.processMessage(msg)
	}
	log.Debugf("Done poll once")
}
