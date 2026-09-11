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

// Package messaging implements the duplex transmission between the Executer and its document worker.
package messaging

import (
	"sync/atomic"
)

// machineShutdownSignaled records that core relayed a machine-shutdown notification to this process.
// A latch with no expiry, because by the time the notification exists the shutdown is committed: the
// SCM sends PRESHUTDOWN/SHUTDOWN only once the system shutdown sequence has begun, and both of the
// ways a shutdown can still be called off -- an admin's shutdown /a during the countdown, an app
// vetoing via WM_QUERYENDSESSION while interactive sessions end -- happen before services are
// stopped. So there is no aborted-shutdown case left to expire, and a wall-clock expiry would
// instead re-report the very failure this gate exists to suppress whenever the teardown outlasts it
// (WaitToKillServiceTimeout raised by policy, slow storage, a large install still flushing).
//
// Process memory only, so it cannot outlive the shutdown it describes: the flag dies with the worker
// and a booted agent starts clear.
var machineShutdownSignaled atomic.Bool

// SetMachineShuttingDown marks that a machine shutdown is underway.
func SetMachineShuttingDown() {
	machineShutdownSignaled.Store(true)
}

// IsMachineShuttingDown reports whether a machine shutdown was signaled to this process, which is
// what makes a Failed patch result attributable to the reboot rather than to the patching itself.
func IsMachineShuttingDown() bool {
	return machineShutdownSignaled.Load()
}
