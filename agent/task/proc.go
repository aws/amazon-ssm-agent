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

package task

import (
	"time"

	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/amazon-ssm-agent/agent/times"
)

// process launches one job in a separate go routine and waits
// for either the job to finish or for a cancel to be requested.
// If cancel is requested, this function waits for some time to allow the
// job to complete. If the job does not complete by the timeout, the go routine
// of the job is abandoned, and this function returns.
func process(log log.T, job Job, cancelFlag *ChanneledCancelFlag, cancelWait time.Duration, clock times.Clock) {
	// Make a buffered channel to avoid blocking on send. This helps
	// in case the job fails to cancel on time and we give up on it.
	// If the job finally ends, it will succeed to send a signal
	// on the channel and then it will terminate. This will allow
	// the garbage collector to collect the go routine's resources
	// and the channel.
	doneChan := make(chan struct{}, 1)

	go runJob(log, func() { job(cancelFlag) }, doneChan)

	done := waitEither(doneChan, cancelFlag.ch)
	if done {
		// task done, set the flag to wake up waiting routines
		cancelFlag.Set(Completed)
		return
	}

	log.Debugf("Execution has been canceled, waiting up to %v to finish", cancelWait)
	done = waitEither2(doneChan, clock.After(cancelWait))
	if done {
		// job completed within cancel waiting window
		cancelFlag.Set(Completed)
		return
	}

	log.Debugf("Job failed to terminate within %v!", cancelWait)
}

// waitEither waits until one of the two channels receives something.
// Returns true if the first channel received, false if the second one received.
func waitEither(chan1 chan struct{}, chan2 chan struct{}) (chan1Done bool) {
	select {
	case <-chan1:
		return true

	case <-chan2:
		return false
	}
}

func waitEither2(chan1 chan struct{}, chan2 <-chan time.Time) (chan1Done bool) {
	select {
	case <-chan1:
		return true

	case <-chan2:
		return false
	}
}

// runJob executes a job and then sends a signal on the given channel
func runJob(log log.T, job func(), doneChannel chan struct{}) {
	defer func() {
		// recover in case the job panics
		if msg := recover(); msg != nil {
			log.Errorf("Job failed with message %v", msg)
		}
		doneChannel <- struct{}{}
	}()
	job()
}
