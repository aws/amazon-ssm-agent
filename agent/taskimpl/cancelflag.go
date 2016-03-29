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

// Package taskimpl contains a default implementation of the interfaces in the task package.
package taskimpl

import (
	"sync"

	"github.com/aws/amazon-ssm-agent/agent/task"
)

// CancelFlag is a default implementation of the task.CancelFlag interface.
type CancelFlag struct {
	state  task.State
	ch     chan struct{}
	closed bool
	m      sync.RWMutex
}

// NewCancelFlag creates a new instance of CancelFlag.
func NewCancelFlag() *CancelFlag {
	flag := &CancelFlag{ch: make(chan struct{})}
	return flag
}

// Canceled returns true if this flag has been set to Cancel state, false otherwise.
func (t *CancelFlag) Canceled() bool {
	t.m.RLock()
	defer t.m.RUnlock()
	return t.state == task.Canceled || t.state == task.ShutDown
}

// ShutDown returns true if this flag has been set to ShutDown state, false otherwise.
func (t *CancelFlag) ShutDown() bool {
	t.m.RLock()
	defer t.m.RUnlock()
	return t.state == task.ShutDown
}

// State returns the current flag state.
func (t *CancelFlag) State() task.State {
	t.m.RLock()
	defer t.m.RUnlock()
	return t.state
}

// Wait blocks until the flag is set to either Cancel or Completed state. Returns the state.
func (t *CancelFlag) Wait() (state task.State) {
	<-t.ch
	return t.State()
}

// Set sets the state of this flag and wakes up waiting callers.
func (t *CancelFlag) Set(state task.State) {
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
