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
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestCanceled tests that the Cancel method returns the correct state
func TestCanceled(t *testing.T) {
	cancelFlag := NewChanneledCancelFlag()
	assert.Equal(t, cancelFlag.Canceled(), false)

	cancelFlag.Set(Canceled)
	assert.Equal(t, cancelFlag.Canceled(), true)
	assert.NotEqual(t, cancelFlag.ShutDown(), true)

	cancelFlag.Set(ShutDown)
	assert.Equal(t, cancelFlag.ShutDown(), true)
	assert.NotEqual(t, cancelFlag.Canceled(), true)

	cancelFlag.Set(Completed)
	assert.Equal(t, cancelFlag.Canceled(), false)
	assert.Equal(t, cancelFlag.ShutDown(), false)

}

// TestWait tests that the Wait method blocks the caller and returns the
// correct state once unblocked
func TestWait(t *testing.T) {
	states := []State{Canceled, Completed}
	for _, state := range states {
		testWait(t, state)
	}
}

func testWait(t *testing.T, state State) {
	flag := NewChanneledCancelFlag()

	// wait on the flag and return its state once unblocked
	ch := make(chan State)
	go func() {
		ch <- State(0)
		ch <- flag.Wait()
	}()

	// wait for go routine to start and test initial state
	assert.Equal(t, State(0), <-ch)
	assert.Equal(t, flag.Canceled(), false)

	flag.Set(state)

	// check that routine "wakes up" with the correct state
	assert.Equal(t, state, <-ch)
	assert.Equal(t, flag.Canceled(), state == Canceled)
}
