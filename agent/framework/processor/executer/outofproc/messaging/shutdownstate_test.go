// Copyright 2026 Amazon.com, Inc. or its affiliates. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License"). You may not
// use this file except in compliance with the License. A copy of the
// License is located at
//
// http://aws.amazon.com/apache2.0/
//
// or in the "license" file accompanying this file. This file is distributed
// on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either
// express or implied. See the License for the specific language governing
// permissions and limitations under the License.

package messaging

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// resetShutdownState clears the process-global signal, before and after, so a test that trips the
// latch cannot leak it into the rest of the package.
func resetShutdownState(t *testing.T) {
	t.Helper()
	machineShutdownSignaled.Store(false)
	t.Cleanup(func() { machineShutdownSignaled.Store(false) })
}

func TestIsMachineShuttingDown_NoSignal(t *testing.T) {
	resetShutdownState(t)

	assert.False(t, IsMachineShuttingDown(), "no preshutdown notification means no shutdown")
}

func TestIsMachineShuttingDown_AfterSignal(t *testing.T) {
	resetShutdownState(t)

	SetMachineShuttingDown()

	assert.True(t, IsMachineShuttingDown())
}

// The signal must not expire, because nothing can un-commit the shutdown that produced it. A latch
// that cleared itself would re-report exactly the failure this gate suppresses on any teardown that
// outlasted the expiry -- so the read stays true however long the shutdown takes.
func TestIsMachineShuttingDown_DoesNotExpire(t *testing.T) {
	resetShutdownState(t)

	SetMachineShuttingDown()

	for i := 0; i < 3; i++ {
		assert.True(t, IsMachineShuttingDown(), "the signal must hold for as long as this process lives")
	}
}

// The service control handler prepares from both svc.PreShutdown and svc.Shutdown, so a repeated
// notification has to be harmless rather than toggling the gate.
func TestSetMachineShuttingDown_IsIdempotent(t *testing.T) {
	resetShutdownState(t)

	SetMachineShuttingDown()
	SetMachineShuttingDown()

	assert.True(t, IsMachineShuttingDown())
}

// PrepareForShutdown is the whole shutdown hook: it sets the flag the document paths read, and does
// nothing else. Asserting the observable effect rather than a call count is what keeps this honest
// if the hook ever grows a body again.
func TestPrepareForShutdown_SignalsShutdown(t *testing.T) {
	resetShutdownState(t)
	assert.False(t, IsMachineShuttingDown())

	PrepareForShutdown(logger)

	assert.True(t, IsMachineShuttingDown())
}
