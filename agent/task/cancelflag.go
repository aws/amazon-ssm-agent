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

// Package task contains a default implementation of the interfaces in the task package.
package task

import (
	"sync"
)

// State represents the state of a job.
type State int

const (
	// Canceled indicates a job for which cancellation has been requested.
	Canceled State = 1

	// Completed indicates a completed job.
	Completed State = 2

	// ShutDown indicates a job for which ShutDown has been requested.
	ShutDown State = 3
)

// CancelFlag is an object that is passed to any job submitted to a task in order to
// communicated job cancellation. Job cancellation has to be cooperative.
type CancelFlag interface {
	// Canceled returns true if a cancel or Shutdown has been requested, false otherwise.
	// This method should be called periodically in the job.
	Canceled() bool

	// Set sets the state of this flag and wakes up waiting callers.
	Set(state State)

	// ShutDown returns true if a ShutDown has been requested, false otherwise.
	// This method should be called periodically in the job.
	ShutDown() bool

	// State returns the current flag state
	State() State

	// Wait blocks the caller until either a cancel has been requested or the
	// task has completed normally. Returns Canceled if cancel has been requested,
	// or Completed if the task completed normally.
	// This is intended to be used to wake up a job that may be waiting on some resources, as follows:
	// The main job starts a go routine that calls Wait. The main job then does its processing.
	// During processing the job may be waiting on certain events/conditions.
	// In the go routine, once Wait returns, if the return value indicates that a cancel
	// request has been received, the go routine wakes up the running job.
	Wait() (state State)
}

// ChanneledCancelFlag is a default implementation of the task.CancelFlag interface.
type ChanneledCancelFlag struct {
	state  State
	ch     chan struct{}
	closed bool
	m      sync.RWMutex
}

// NewChanneledCancelFlag creates a new instance of ChanneledCancelFlag.
func NewChanneledCancelFlag() *ChanneledCancelFlag {
	flag := &ChanneledCancelFlag{ch: make(chan struct{})}
	return flag
}

// Canceled returns true if this flag has been set to Cancel state, false otherwise.
func (t *ChanneledCancelFlag) Canceled() bool {
	t.m.RLock()
	defer t.m.RUnlock()
	return t.state == Canceled
}

// ShutDown returns true if this flag has been set to ShutDown state, false otherwise.
func (t *ChanneledCancelFlag) ShutDown() bool {
	t.m.RLock()
	defer t.m.RUnlock()
	return t.state == ShutDown
}

// State returns the current flag state.
func (t *ChanneledCancelFlag) State() State {
	t.m.RLock()
	defer t.m.RUnlock()
	return t.state
}

// Wait blocks until the flag is set to either Cancel or Completed state. Returns the state.
func (t *ChanneledCancelFlag) Wait() (state State) {
	<-t.ch
	return t.State()
}

// Set sets the state of this flag and wakes up waiting callers.
func (t *ChanneledCancelFlag) Set(state State) {
	t.m.Lock()
	defer t.m.Unlock()
	t.state = state

	// close channel to wake up routines that are waiting
	if !t.closed {
		// avoid double closing, which would panic
		close(t.ch)
		t.closed = true
	}
}
