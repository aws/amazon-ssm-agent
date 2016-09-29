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

// Package taskpool wraps execution task pool and cancel task pool
package taskpool

import (
	"sync"
	"time"

	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/amazon-ssm-agent/agent/task"
	"github.com/aws/amazon-ssm-agent/agent/times"
)

// Manager contains execution pool and cancel pool
type Manager struct {
	executionPool task.Pool
	cancelPool    task.Pool
}

// T represents the interface for taskpool
type T interface {
	Submit(log log.T, jobID string, job task.Job) error
	ShutdownAndWait(timeout time.Duration)
}

// NewTaskPool creates a new instance of Manager
func NewTaskPool(log log.T, documentWorkersLimit int, cancelWaitDurationMillisecond time.Duration) T {
	clock := times.DefaultClock
	cancelWaitDuration := cancelWaitDurationMillisecond * time.Millisecond

	mgr := Manager{
		executionPool: task.NewPool(log, documentWorkersLimit, cancelWaitDuration, clock),
		cancelPool:    task.NewPool(log, documentWorkersLimit, cancelWaitDuration, clock),
	}

	return mgr
}

// Submit submits the task to the execution pool
func (m Manager) Submit(log log.T, jobID string, job task.Job) error {
	return m.executionPool.Submit(log, jobID, job)
}

// ShutdownAndWait wait and shutdown the task pool
func (m Manager) ShutdownAndWait(timeout time.Duration) {
	var wg sync.WaitGroup

	wg.Add(1)
	go func() {
		defer wg.Done()
		m.executionPool.ShutdownAndWait(timeout)
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		m.cancelPool.ShutdownAndWait(timeout)
	}()

	// wait for everything to shutdown
	wg.Wait()
}
