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

package taskimpl

import (
	"testing"

	"github.com/aws/amazon-ssm-agent/agent/task"
	"github.com/stretchr/testify/assert"
)

// TestCanceled tests that the Cancel method returns the correct state
func TestCanceled(t *testing.T) {
	cancelFlag := NewCancelFlag()
	assert.Equal(t, cancelFlag.Canceled(), false)

	cancelFlag.Set(task.Canceled)
	assert.Equal(t, cancelFlag.Canceled(), true)

	cancelFlag.Set(task.Completed)
	assert.Equal(t, cancelFlag.Canceled(), false)
}

// TestWait tests that the Wait method blocks the caller and returns the
// correct state once unblocked
func TestWait(t *testing.T) {
	states := []task.State{task.Canceled, task.Completed}
	for _, state := range states {
		testWait(t, state)
	}
}

func testWait(t *testing.T, state task.State) {
	flag := NewCancelFlag()

	// wait on the flag and return its state once unblocked
	ch := make(chan task.State)
	go func() {
		ch <- task.State(0)
		ch <- flag.Wait()
	}()

	// wait for go routine to start and test initial state
	assert.Equal(t, task.State(0), <-ch)
	assert.Equal(t, flag.Canceled(), false)

	flag.Set(state)

	// check that routine "wakes up" with the correct state
	assert.Equal(t, state, <-ch)
	assert.Equal(t, flag.Canceled(), state == task.Canceled)
}
