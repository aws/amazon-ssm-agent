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

// Package signal manages signal channel required by sending/receiving request for executing scheduled association
package signal

import (
	"runtime/debug"
	"sync"
	"time"

	"github.com/aws/amazon-ssm-agent/agent/log"
)

const (
	defaultScheduledJobQueueSize              = 100
	defaultScheduleHealthTimerDurationSeconds = 300
	scheduleForNextAssociationMessage         = "Next association is scheduled at %v, association will wait for %v"
)

// AssociationExecutionSignal uses to manage the channel required by sending/receiving signals for executing scheduled association
type AssociationExecutionSignal struct {
	executeSignal chan struct{}
	stopSignal    chan bool
}

var instance = AssociationExecutionSignal{
	executeSignal: make(chan struct{}, defaultScheduledJobQueueSize),
	stopSignal:    make(chan bool, 1),
}

var schedulerHealthTimer = time.NewTicker(defaultScheduleHealthTimerDurationSeconds * time.Second)
var waitTimerForNextScheduledAssociation *time.Timer
var nextScheduledDate time.Time

var lock sync.RWMutex

// InitializeAssociationSignalService creates goroutine to handle signals
func InitializeAssociationSignalService(log log.T, task func(log log.T)) {
	lock.Lock()
	defer lock.Unlock()

	go func() {
		defer func() {
			if r := recover(); r != nil {
				log.Errorf("Association signal service panic: %v", r)
				log.Errorf("Stacktrace:\n%s", debug.Stack())
			}
		}()
		for {
			_, more := <-instance.executeSignal
			if more {
				log.Debug("Receieved signal for executing scheduled association")
				if task != nil {
					task(log)
				}
			} else {
				log.Debug("Stopping association scheduler")
				if !instance.isStopped() {
					instance.stopSignal <- true
				}
				return
			}
		}
	}()

	go func() {
		for {
			select {
			case <-schedulerHealthTimer.C:
				ExecuteAssociation(log)
			}
		}
	}()
}

// ResetWaitTimerForNextScheduledAssociation stops old wait timer and creates new one with updated target date
// It will wait until the target date then sends the signal for executing next scheduled association
func ResetWaitTimerForNextScheduledAssociation(log log.T, targetDate time.Time) {
	lock.Lock()
	defer lock.Unlock()

	if targetDate.IsZero() {
		return
	}
	// duration can be negative, which will trigger the signal immediately
	duration := targetDate.UTC().Sub(time.Now().UTC())

	if nextScheduledDate.Equal(targetDate) {
		log.Infof(scheduleForNextAssociationMessage, nextScheduledDate, duration)
		return
	}

	// Stop previous timer as the next scheduled date is changed
	if waitTimerForNextScheduledAssociation != nil {
		waitTimerForNextScheduledAssociation.Stop()
	}

	log.Infof(scheduleForNextAssociationMessage, targetDate, duration)
	waitTimerForNextScheduledAssociation = time.AfterFunc(duration, func() { ExecuteAssociation(log) })
	nextScheduledDate = targetDate
}

// StopWaitTimerForNextScheduledAssociation stops the timer so it will not get triggered and send signal for the next scheduled association
func StopWaitTimerForNextScheduledAssociation() {
	if waitTimerForNextScheduledAssociation != nil {
		waitTimerForNextScheduledAssociation.Stop()
	}
}

// ExecuteAssociation sends out signal to the worker to process next scheduled association
func ExecuteAssociation(log log.T) {
	log.Debug("Sending signal to execute scheduled association")

	if !instance.isStopped() {
		instance.executeSignal <- struct{}{}
	}
}

// Stop close the channel and stop the timers
func Stop() {
	lock.Lock()
	defer lock.Unlock()

	if !instance.isStopped() {
		close(instance.executeSignal)
		close(instance.stopSignal)
	}
	if waitTimerForNextScheduledAssociation != nil {
		waitTimerForNextScheduledAssociation.Stop()
	}
	schedulerHealthTimer.Stop()
}

// StopExecutionSignal stops the signal channel, which stops the association execution
func StopExecutionSignal() {
	lock.Lock()
	defer lock.Unlock()
	if !instance.isStopped() {
		close(instance.executeSignal)
		close(instance.stopSignal)
	}
}

func (s *AssociationExecutionSignal) isStopped() bool {
	select {
	case <-s.stopSignal:
		// received signal or channel already closed
		return true
	default:
		return false
	}
}
