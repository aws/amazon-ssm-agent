// Copyright 2026 Amazon.com, Inc. or its affiliates. All Rights Reserved.
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

package messaging

import (
	"sync"
	"sync/atomic"
	"time"
)

// machineShutdownSignalTTL bounds how long a preshutdown notification keeps withholding Failed patch
// results. A shutdown can be aborted or vetoed after preshutdown is delivered -- an admin runs
// shutdown /a, another service blocks it -- and this worker is the long-running one that survives an
// agent restart, so a signal with no expiry would mask every genuine patch failure for the rest of
// the process's life.
//
// 240s is derived, not tuned: a result this gate legitimately covers arrives while the machine is
// being torn down, so the ceiling is the longest SCM preshutdown window (180s on Server 2016 and
// older; 10s on 2019+) plus WaitToKillServiceTimeout, and 240s clears that with a minute to spare.
// Since the TTL is also how long an aborted shutdown can mask a real failure, the slack stays small.
const machineShutdownSignalTTL = 240 * time.Second

// machineShutdownSignaledAt holds the UnixNano of the last preshutdown notification core relayed
// here while this worker was still alive, or 0 if none has arrived. A timestamp rather than a bool
// so the signal expires on its own; nothing tells a service that a shutdown was called off.
var machineShutdownSignaledAt atomic.Int64

// timeNow is injectable so the TTL is testable without sleeping.
var timeNow = time.Now

// SetMachineShuttingDown marks that a machine shutdown is underway.
func SetMachineShuttingDown() {
	machineShutdownSignaledAt.Store(timeNow().UnixNano())
}

// IsMachineShuttingDown reports whether a machine shutdown was signaled to this process recently
// enough to still explain a Failed patch result. Expires after machineShutdownSignalTTL so an
// aborted shutdown cannot permanently mask real failures.
func IsMachineShuttingDown() bool {
	signaledAt := machineShutdownSignaledAt.Load()
	if signaledAt == 0 {
		return false
	}
	return timeNow().Sub(time.Unix(0, signaledAt)) < machineShutdownSignalTTL
}

// inFlightPatchDocs tracks in-flight patch documents by ID so a shutdown can make each reboot-safe
// without waiting for a result the killed worker may never deliver.
var (
	inFlightPatchDocsMu sync.Mutex
	inFlightPatchDocs   = make(map[string]*ExecuterBackend)
)

// registerInFlightPatchDoc records a patch document's backend as in flight.
func registerInFlightPatchDoc(documentID string, backend *ExecuterBackend) {
	if documentID == "" {
		return
	}
	inFlightPatchDocsMu.Lock()
	defer inFlightPatchDocsMu.Unlock()
	inFlightPatchDocs[documentID] = backend
}

// unregisterInFlightPatchDoc drops a patch document once it is no longer in flight.
func unregisterInFlightPatchDoc(documentID string) {
	if documentID == "" {
		return
	}
	inFlightPatchDocsMu.Lock()
	defer inFlightPatchDocsMu.Unlock()
	delete(inFlightPatchDocs, documentID)
}

// snapshotInFlightPatchDocs returns the in-flight patch document backends. The registry lock is
// released before callers act on the result; each backend guards its own state with docStateMu,
// since preparation and the messaging worker write the same document concurrently.
func snapshotInFlightPatchDocs() []*ExecuterBackend {
	inFlightPatchDocsMu.Lock()
	defer inFlightPatchDocsMu.Unlock()
	backends := make([]*ExecuterBackend, 0, len(inFlightPatchDocs))
	for _, backend := range inFlightPatchDocs {
		backends = append(backends, backend)
	}
	return backends
}
