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
	"testing"
	"time"

	"github.com/aws/amazon-ssm-agent/agent/contracts"
	"github.com/aws/amazon-ssm-agent/agent/mocks/log"
	taskmocks "github.com/aws/amazon-ssm-agent/agent/mocks/task"
	"github.com/aws/amazon-ssm-agent/agent/task"
	"github.com/stretchr/testify/assert"
)

// resetShutdownState clears the process-global shutdown flag and registry, so tests here must not
// run in parallel.
func resetShutdownState() {
	machineShutdownSignaledAt.Store(0)
	timeNow = time.Now
	inFlightPatchDocsMu.Lock()
	inFlightPatchDocs = make(map[string]*ExecuterBackend)
	inFlightPatchDocsMu.Unlock()
}

// newPatchBackendForShutdown builds a backend for a patch document in the state a worker killed
// mid-run leaves behind.
func newPatchBackendForShutdown(docID string, pluginStatus contracts.ResultStatus) (*ExecuterBackend, *contracts.DocumentState) {
	docState := &contracts.DocumentState{
		DocumentInformation: contracts.DocumentInfo{
			DocumentID:     docID,
			DocumentName:   "AWS-RunPatchBaseline",
			DocumentStatus: contracts.ResultStatusFailed,
		},
		InstancePluginsInformation: []contracts.PluginState{
			{
				Id: "PatchWindows",
				Result: contracts.PluginResult{
					Status: pluginStatus,
					Code:   1,
					Error:  "failed to run commands: exit status 1",
				},
			},
		},
	}
	backend := &ExecuterBackend{
		docState: docState,
		log:      log.NewMockLog(),
	}
	return backend, docState
}

func TestIsMachineShuttingDown_DefaultsFalse(t *testing.T) {
	resetShutdownState()
	assert.False(t, IsMachineShuttingDown(), "no shutdown signaled yet")
}

func TestPrepareForShutdown_SetsFlagWithNoInFlightDocs(t *testing.T) {
	resetShutdownState()

	withheld := PrepareForShutdown(log.NewMockLog())

	assert.Equal(t, 0, withheld, "no in-flight patch documents to prepare")
	assert.True(t, IsMachineShuttingDown(), "flag must be set even with nothing in flight, so a "+
		"result consumed moments later is still recognized as reboot-interrupted")
}

// Must persist itself: the Executer saves only after messaging returns, which the reboot can
// prevent, and an in-memory-only change would be lost.
func TestPrepareForShutdown_PersistsInProgressState(t *testing.T) {
	resetShutdownState()

	backend, docState := newPatchBackendForShutdown("doc-persist", contracts.ResultStatusFailed)
	var saved []contracts.DocumentState
	backend.BindDocumentStore(func(state contracts.DocumentState) {
		saved = append(saved, state)
	})

	withheld := PrepareForShutdown(log.NewMockLog())

	assert.Equal(t, 1, withheld)
	assert.True(t, IsMachineShuttingDown())

	// The in-memory state is reboot-safe...
	assert.Equal(t, contracts.ResultStatusInProgress, docState.DocumentInformation.DocumentStatus)
	assert.Equal(t, contracts.ResultStatusInProgress, docState.InstancePluginsInformation[0].Result.Status)
	assert.Equal(t, 0, docState.InstancePluginsInformation[0].Result.Code)
	assert.Empty(t, docState.InstancePluginsInformation[0].Result.Error)

	// ...and, critically, it was written through to the store rather than left in memory.
	assert.Len(t, saved, 1, "shutdown path must persist synchronously")
	assert.Equal(t, contracts.ResultStatusInProgress, saved[0].DocumentInformation.DocumentStatus)
	assert.Equal(t, contracts.ResultStatusInProgress, saved[0].InstancePluginsInformation[0].Result.Status)
}

// The processor acts on SuccessAndReboot by requesting a reboot, and this path is a heuristic.
func TestPrepareForShutdown_NeverStampsSuccessAndReboot(t *testing.T) {
	resetShutdownState()

	backend, docState := newPatchBackendForShutdown("doc-noreboot", contracts.ResultStatusFailed)
	backend.BindDocumentStore(func(contracts.DocumentState) {})

	PrepareForShutdown(log.NewMockLog())

	assert.NotEqual(t, contracts.ResultStatusSuccessAndReboot, docState.DocumentInformation.DocumentStatus)
	assert.NotEqual(t, contracts.ResultStatusSuccessAndReboot, docState.InstancePluginsInformation[0].Result.Status)
}

// Only already-Failed results get rewritten.
func TestPrepareForShutdown_LeavesSuccessfulPluginResultsAlone(t *testing.T) {
	resetShutdownState()

	backend, docState := newPatchBackendForShutdown("doc-healthy", contracts.ResultStatusSuccess)
	backend.BindDocumentStore(func(contracts.DocumentState) {})

	PrepareForShutdown(log.NewMockLog())

	assert.Equal(t, contracts.ResultStatusSuccess, docState.InstancePluginsInformation[0].Result.Status,
		"a completed plugin must not be rewound by shutdown preparation")
}

// A completed document must not be forced back to InProgress by a later shutdown.
func TestPrepareForShutdown_SkipsUnregisteredDocs(t *testing.T) {
	resetShutdownState()

	backend, docState := newPatchBackendForShutdown("doc-done", contracts.ResultStatusFailed)
	backend.BindDocumentStore(func(contracts.DocumentState) {})
	unregisterInFlightPatchDoc("doc-done")

	withheld := PrepareForShutdown(log.NewMockLog())

	assert.Equal(t, 0, withheld)
	assert.Equal(t, contracts.ResultStatusFailed, docState.DocumentInformation.DocumentStatus,
		"a completed document must keep its terminal status")
}

func TestPrepareForShutdown_PreparesEveryInFlightDoc(t *testing.T) {
	resetShutdownState()

	backendA, docStateA := newPatchBackendForShutdown("doc-a", contracts.ResultStatusFailed)
	backendB, docStateB := newPatchBackendForShutdown("doc-b", contracts.ResultStatusFailed)
	backendA.BindDocumentStore(func(contracts.DocumentState) {})
	backendB.BindDocumentStore(func(contracts.DocumentState) {})

	withheld := PrepareForShutdown(log.NewMockLog())

	assert.Equal(t, 2, withheld)
	assert.Equal(t, contracts.ResultStatusInProgress, docStateA.DocumentInformation.DocumentStatus)
	assert.Equal(t, contracts.ResultStatusInProgress, docStateB.DocumentInformation.DocumentStatus)
}

// A document with no store bound is not registered, so a shutdown cannot mark it reboot-safe in
// memory only and silently fail to persist.
func TestPrepareForShutdown_UnboundStoreIsNotRegistered(t *testing.T) {
	resetShutdownState()

	backend, docState := newPatchBackendForShutdown("doc-nostore", contracts.ResultStatusFailed)
	_ = backend // deliberately not bound

	withheld := PrepareForShutdown(log.NewMockLog())

	assert.Equal(t, 0, withheld, "an unbound document must not be reported as prepared")
	assert.Equal(t, contracts.ResultStatusFailed, docState.DocumentInformation.DocumentStatus)
}

// Binding a nil store must not register the document either.
func TestBindDocumentStore_NilSaveIsNotRegistered(t *testing.T) {
	resetShutdownState()

	backend, _ := newPatchBackendForShutdown("doc-nilsave", contracts.ResultStatusFailed)
	backend.BindDocumentStore(nil)

	assert.Equal(t, 0, PrepareForShutdown(log.NewMockLog()))
}

// A non-patch document is never registered, since re-running an arbitrary document may not be safe.
func TestBindDocumentStore_NonPatchDocumentIsNotRegistered(t *testing.T) {
	resetShutdownState()

	backend, _ := newPatchBackendForShutdown("doc-shell", contracts.ResultStatusFailed)
	backend.docState.DocumentInformation.DocumentName = "AWS-RunShellScript"
	backend.BindDocumentStore(func(contracts.DocumentState) {})

	assert.Equal(t, 0, PrepareForShutdown(log.NewMockLog()))
}

// A Failed patch result arriving after the shutdown signal is withheld and kept InProgress.
func TestProcess_WithholdsFailedPatchResultAfterShutdownSignaled(t *testing.T) {
	resetShutdownState()
	defer resetShutdownState()

	outputChan := make(chan contracts.DocumentResult, 10)
	stopChan := make(chan int, 1)
	backend, docState := newPatchBackendForShutdown("doc-gate", contracts.ResultStatusFailed)
	backend.output = outputChan
	backend.stopChan = stopChan
	backend.BindDocumentStore(func(contracts.DocumentState) {})
	// The withhold spawns a reporter on the default 45s grace. Release makes it no-op when it wakes
	// instead of rewriting docState and pushing to outputChan after this test has returned. The
	// grace is deliberately left long: shortening it lets the reporter fire mid-test and undo the
	// very withholding the assertions below check.
	defer backend.Release()

	// Core signaled the shutdown while the document was still running.
	SetMachineShuttingDown()

	err := backend.Process(buildCompleteDatagram(t, contracts.ResultStatusFailed))
	assert.NoError(t, err)

	assert.Len(t, outputChan, 0, "terminal failure must be withheld, not reported")
	assert.Len(t, stopChan, 0, "messaging must stay up so the reboot can kill this process mid-wait")
	assert.Equal(t, contracts.ResultStatusInProgress, docState.DocumentInformation.DocumentStatus)
	assert.Equal(t, contracts.ResultStatusInProgress, docState.InstancePluginsInformation[0].Result.Status)
}

// With no shutdown signaled the gate fails closed and a genuine failure is reported.
func TestProcess_ReportsFailedPatchResultWithoutShutdown(t *testing.T) {
	resetShutdownState()

	outputChan := make(chan contracts.DocumentResult, 10)
	stopChan := make(chan int, 1)
	backend, _ := newPatchBackendForShutdown("doc-genuine", contracts.ResultStatusFailed)
	backend.output = outputChan
	backend.stopChan = stopChan
	backend.BindDocumentStore(func(contracts.DocumentState) {})

	err := backend.Process(buildCompleteDatagram(t, contracts.ResultStatusFailed))
	assert.NoError(t, err)

	res := <-outputChan
	assert.Equal(t, contracts.ResultStatusFailed, res.Status, "genuine failure must be reported")
}

// The withheld path returns before the terminal unregister, so Release is what keeps a document the
// shutdown already handled from being prepared a second time. Held open here with a long grace so
// the deferred reporter cannot unregister the document first -- that path has its own test.
func TestRelease_DropsDocumentAfterWithheldResult(t *testing.T) {
	resetShutdownState()
	defer resetShutdownState()

	backend, _ := newPatchBackendForShutdown("doc-release", contracts.ResultStatusFailed)
	backend.withheldFailureGrace = time.Hour
	backend.output = make(chan contracts.DocumentResult, 10)
	backend.stopChan = make(chan int, 1)
	backend.BindDocumentStore(func(contracts.DocumentState) {})

	SetMachineShuttingDown()
	assert.NoError(t, backend.Process(buildCompleteDatagram(t, contracts.ResultStatusFailed)))
	assert.Len(t, backend.stopChan, 0, "messaging must stay up while the failure is held")

	// Still registered: the withhold path returned early.
	assert.Equal(t, 1, PrepareForShutdown(log.NewMockLog()))

	backend.Release()

	assert.Equal(t, 0, PrepareForShutdown(log.NewMockLog()),
		"Release must drop the document so a later shutdown cannot re-persist it")
}

// Deferred unconditionally, so it runs on documents that were never registered -- non-patch
// documents, and patch documents that never had a store bound.
func TestRelease_IsIdempotentAndSafeWhenUnregistered(t *testing.T) {
	resetShutdownState()

	backend, _ := newPatchBackendForShutdown("doc-idempotent", contracts.ResultStatusFailed)

	// Never registered: no store was ever bound.
	assert.NotPanics(t, backend.Release)

	backend.BindDocumentStore(func(contracts.DocumentState) {})
	backend.Release()
	assert.NotPanics(t, backend.Release, "second Release must be a no-op")
	assert.Equal(t, 0, PrepareForShutdown(log.NewMockLog()))
}

// buildCompleteDatagram builds a document-complete datagram carrying the given status.
func buildCompleteDatagram(t *testing.T, status contracts.ResultStatus) string {
	result := contracts.DocumentResult{
		Status:        status,
		LastPlugin:    "",
		PluginResults: map[string]*contracts.PluginResult{},
	}
	datagram, err := CreateDatagram(MessageTypeComplete, result)
	assert.NoError(t, err)
	return datagram
}

// Preparation and Process write the same docState from different goroutines and overlap during the
// preshutdown window. Run under -race.
func TestPrepareForShutdown_ConcurrentWithProcessIsRaceFree(t *testing.T) {
	resetShutdownState()

	backend, _ := newPatchBackendForShutdown("doc-race", contracts.ResultStatusFailed)
	backend.output = make(chan contracts.DocumentResult, 256)
	backend.stopChan = make(chan int, 256)
	// The save must also be serialized; touching shared state here would trip -race otherwise.
	saves := 0
	backend.BindDocumentStore(func(contracts.DocumentState) { saves++ })

	var wg sync.WaitGroup
	wg.Add(2)

	// Writer 1: the shutdown-preparation path.
	go func() {
		defer wg.Done()
		for i := 0; i < 200; i++ {
			PrepareForShutdown(log.NewMockLog())
		}
	}()

	// Writer 2: the messaging worker delivering results.
	go func() {
		defer wg.Done()
		datagram := buildCompleteDatagram(t, contracts.ResultStatusInProgress)
		for i := 0; i < 200; i++ {
			_ = backend.Process(datagram)
		}
	}()

	wg.Wait()
	assert.True(t, IsMachineShuttingDown())
}

// The store must get a snapshot, not the live document.
func TestMarkRebootSafe_PersistsSnapshotNotLiveState(t *testing.T) {
	resetShutdownState()

	backend, docState := newPatchBackendForShutdown("doc-snapshot", contracts.ResultStatusFailed)
	var saved contracts.DocumentState
	backend.BindDocumentStore(func(state contracts.DocumentState) { saved = state })

	PrepareForShutdown(log.NewMockLog())
	assert.Equal(t, contracts.ResultStatusInProgress, saved.InstancePluginsInformation[0].Result.Status)

	// Mutate the live document the way a later Failed result would.
	docState.InstancePluginsInformation[0].Result.Status = contracts.ResultStatusFailed

	assert.Equal(t, contracts.ResultStatusInProgress, saved.InstancePluginsInformation[0].Result.Status,
		"persisted snapshot must not alias the live document's plugin slice")
}

// A backend built the real way must not be visible to the shutdown path before a store is bound:
// markRebootSafe would find no store and silently fail to persist, defeating the feature in exactly
// the scenario it exists for.
func TestNewExecuterBackend_NotRegisteredUntilStoreBound(t *testing.T) {
	resetShutdownState()

	docState := &contracts.DocumentState{
		DocumentInformation: contracts.DocumentInfo{
			DocumentID:   "doc-ctor",
			DocumentName: "AWS-RunPatchBaseline",
		},
		InstancePluginsInformation: []contracts.PluginState{
			{Id: "PatchWindows", Result: contracts.PluginResult{Status: contracts.ResultStatusFailed}},
		},
	}
	cancel := taskmocks.NewMockDefault()
	cancel.On("Wait").Return(task.Completed).Maybe()
	cancel.On("Canceled").Return(false).Maybe()
	cancel.On("ShutDown").Return(false).Maybe()

	backend := NewExecuterBackend(log.NewMockLog(), make(chan contracts.DocumentResult, 10), docState, cancel)

	assert.Equal(t, 0, PrepareForShutdown(log.NewMockLog()),
		"must not be registered before a store is bound")

	saved := 0
	backend.BindDocumentStore(func(contracts.DocumentState) { saved++ })

	assert.Equal(t, 1, PrepareForShutdown(log.NewMockLog()), "registered once a store is bound")
	assert.Equal(t, 1, saved, "state must actually reach the store")
}

// Regression: a genuinely successful document must not be rewound to InProgress by a shutdown that
// arrives just after it completed. Without the IsSuccess guard, markRebootSafe would resurrect the
// finished run and re-execute it after the reboot.
func TestMarkRebootSafe_DoesNotRewindSuccessfulDocument(t *testing.T) {
	resetShutdownState()

	docState := &contracts.DocumentState{
		DocumentInformation: contracts.DocumentInfo{
			DocumentID:     "doc-success",
			DocumentName:   "AWS-RunPatchBaseline",
			DocumentStatus: contracts.ResultStatusSuccess,
		},
		InstancePluginsInformation: []contracts.PluginState{
			{Id: "PatchWindows", Result: contracts.PluginResult{Status: contracts.ResultStatusSuccess}},
		},
	}
	backend := &ExecuterBackend{docState: docState, log: log.NewMockLog()}
	var saved []contracts.DocumentState
	backend.BindDocumentStore(func(state contracts.DocumentState) { saved = append(saved, state) })

	PrepareForShutdown(log.NewMockLog())

	assert.Equal(t, contracts.ResultStatusSuccess, docState.DocumentInformation.DocumentStatus,
		"a completed Success must not be rewound to InProgress by a shutdown")
	assert.Equal(t, contracts.ResultStatusSuccess, docState.InstancePluginsInformation[0].Result.Status)
	assert.Empty(t, saved, "a successful document must not be persisted as reboot-safe")
}

// Regression: the IsSuccess guard must NOT over-skip. A Failed patch document during the shutdown
// window is the reboot-interrupted false negative this path exists to rescue -- it must still be
// rewound to InProgress and persisted, exactly as before the guard was added.
func TestMarkRebootSafe_StillRescuesFailedDocumentDuringShutdown(t *testing.T) {
	resetShutdownState()

	backend, docState := newPatchBackendForShutdown("doc-failed-rescue", contracts.ResultStatusFailed)
	var saved []contracts.DocumentState
	backend.BindDocumentStore(func(state contracts.DocumentState) { saved = append(saved, state) })

	PrepareForShutdown(log.NewMockLog())

	assert.Equal(t, contracts.ResultStatusInProgress, docState.DocumentInformation.DocumentStatus,
		"a reboot-interrupted Failed must still be rewound to InProgress")
	assert.Equal(t, contracts.ResultStatusInProgress, docState.InstancePluginsInformation[0].Result.Status)
	assert.Len(t, saved, 1, "the rescued document must still be persisted InProgress")
}

// Regression: a completed document is dropped from the in-flight registry atomically with stamping
// its terminal status (under docStateMu, inside Process), so a shutdown that races in afterward
// cannot snapshot the backend and rewind the finished run.
func TestProcess_CompletedDocumentUnregisteredFromShutdownRegistry(t *testing.T) {
	resetShutdownState()

	docState := &contracts.DocumentState{
		DocumentInformation: contracts.DocumentInfo{
			DocumentID:     "doc-complete-unregister",
			DocumentName:   "AWS-RunPatchBaseline",
			DocumentStatus: contracts.ResultStatusInProgress,
		},
		InstancePluginsInformation: []contracts.PluginState{
			{Id: "PatchWindows", Result: contracts.PluginResult{Status: contracts.ResultStatusSuccess}},
		},
	}
	backend := &ExecuterBackend{
		docState: docState,
		log:      log.NewMockLog(),
		output:   make(chan contracts.DocumentResult, 10),
		stopChan: make(chan int, 10),
	}
	backend.BindDocumentStore(func(contracts.DocumentState) {})

	// A normal (non-withheld) Success completion, no shutdown signaled.
	assert.NoError(t, backend.Process(buildCompleteDatagram(t, contracts.ResultStatusSuccess)))
	<-backend.output
	assert.Equal(t, stopTypeTerminate, <-backend.stopChan)

	assert.Equal(t, 0, PrepareForShutdown(log.NewMockLog()),
		"a completed patch document must be dropped from the in-flight registry so a shutdown cannot rewind it")
}

// A shutdown can be aborted after preshutdown is delivered (shutdown /a, a vetoing service), and
// this worker outlives an agent restart. Once the signal ages out, a genuine patch failure must
// report as Failed instead of being withheld for the life of the process.
func TestIsMachineShuttingDown_ExpiresAfterTTL(t *testing.T) {
	resetShutdownState()
	defer resetShutdownState()

	base := time.Date(2026, 9, 3, 12, 0, 0, 0, time.UTC)
	now := base
	timeNow = func() time.Time { return now }

	SetMachineShuttingDown()
	assert.True(t, IsMachineShuttingDown(), "signal must hold immediately after preshutdown")

	// Still inside the window a real reboot would kill this worker in.
	now = base.Add(machineShutdownSignalTTL - time.Second)
	assert.True(t, IsMachineShuttingDown(), "signal must still hold inside the TTL")

	// The shutdown never happened; the signal must stop masking failures.
	now = base.Add(machineShutdownSignalTTL)
	assert.False(t, IsMachineShuttingDown(), "an aborted shutdown must not mask failures forever")
}

// The TTL must clear the longest window Windows allows between preshutdown and killing the service,
// or it could expire while a real reboot is still tearing the machine down.
func TestMachineShutdownSignalTTL_CoversPreshutdownWindow(t *testing.T) {
	assert.Greater(t, machineShutdownSignalTTL, 180*time.Second,
		"must exceed the 180s maximum SCM preshutdown window")
}

// A later notification must restart the clock rather than inherit an already-stale timestamp.
func TestSetMachineShuttingDown_RefreshesTheWindow(t *testing.T) {
	resetShutdownState()
	defer resetShutdownState()

	base := time.Date(2026, 9, 3, 12, 0, 0, 0, time.UTC)
	now := base
	timeNow = func() time.Time { return now }

	SetMachineShuttingDown()
	now = base.Add(machineShutdownSignalTTL + time.Minute)
	assert.False(t, IsMachineShuttingDown(), "first signal has aged out")

	SetMachineShuttingDown()
	assert.True(t, IsMachineShuttingDown(), "a fresh preshutdown must re-arm the gate")
}

// End-to-end version of the TTL: the reboot-interrupted gate reads this signal, so once it expires a
// genuine Failed patch result must pass through as Failed rather than being withheld.
func TestExecuterBackend_ExpiredShutdownSignalDoesNotWithholdFailure(t *testing.T) {
	resetShutdownState()
	defer resetShutdownState()

	base := time.Date(2026, 9, 3, 12, 0, 0, 0, time.UTC)
	now := base
	timeNow = func() time.Time { return now }
	SetMachineShuttingDown()

	backend, _ := newPatchBackendForShutdown("doc-expired", contracts.ResultStatusFailed)
	docResult := &contracts.DocumentResult{Status: contracts.ResultStatusFailed}

	// Inside the window the failure is treated as reboot-interrupted...
	assert.True(t, backend.isRebootInterruptedPatchFailureLocked(docResult))

	// ...but once the signal ages out the same failure must be reported as a real one.
	now = base.Add(machineShutdownSignalTTL + time.Second)
	assert.False(t, backend.isRebootInterruptedPatchFailureLocked(docResult),
		"a genuine patch failure must not be withheld after the shutdown signal expires")
}

// Withholding is only correct if a reboot follows. When the shutdown is aborted the process survives
// the grace period, and the failure that was withheld must be reported rather than left InProgress
// until some later restart.
func TestProcess_ReportsWithheldFailureWhenShutdownNeverProceeds(t *testing.T) {
	resetShutdownState()
	defer resetShutdownState()

	outputChan := make(chan contracts.DocumentResult, 10)
	stopChan := make(chan int, 1)
	backend, docState := newPatchBackendForShutdown("doc-aborted", contracts.ResultStatusFailed)
	backend.withheldFailureGrace = 10 * time.Millisecond
	backend.output = outputChan
	backend.stopChan = stopChan
	var saved []contracts.DocumentState
	backend.BindDocumentStore(func(state contracts.DocumentState) {
		saved = append(saved, state)
	})

	SetMachineShuttingDown()
	assert.NoError(t, backend.Process(buildCompleteDatagram(t, contracts.ResultStatusFailed)))

	// The shutdown never happens, so the withheld failure surfaces.
	select {
	case result := <-outputChan:
		assert.Equal(t, contracts.ResultStatusFailed, result.Status,
			"an aborted shutdown must report the genuine failure")
	case <-time.After(5 * time.Second):
		t.Fatal("withheld failure was never reported after the grace period")
	}
	assert.Equal(t, stopTypeTerminate, <-stopChan, "messaging must be torn down after reporting")

	// Critically, the persisted state must no longer say InProgress, or the resume path in
	// initialize() would re-run a patch whose failure has just been reported.
	assert.Equal(t, contracts.ResultStatusFailed, docState.DocumentInformation.DocumentStatus)
	assert.Equal(t, contracts.ResultStatusFailed, docState.InstancePluginsInformation[0].Result.Status)
	assert.NotEmpty(t, saved)
	assert.Equal(t, contracts.ResultStatusFailed, saved[len(saved)-1].DocumentInformation.DocumentStatus,
		"the reboot-safe InProgress state must be overwritten before reporting Failed")

	// And it must be out of the shutdown registry, so a later preshutdown cannot rewind it.
	assert.Equal(t, 0, PrepareForShutdown(log.NewMockLog()))
}

// A reboot that does proceed tears the backend down first. The reporter must then stay silent rather
// than write to a result channel that is closing.
func TestProcess_WithheldFailureNotReportedAfterRelease(t *testing.T) {
	resetShutdownState()
	defer resetShutdownState()

	outputChan := make(chan contracts.DocumentResult, 10)
	stopChan := make(chan int, 1)
	backend, docState := newPatchBackendForShutdown("doc-rebooted", contracts.ResultStatusFailed)
	backend.withheldFailureGrace = 50 * time.Millisecond
	backend.output = outputChan
	backend.stopChan = stopChan
	backend.BindDocumentStore(func(contracts.DocumentState) {})

	SetMachineShuttingDown()
	assert.NoError(t, backend.Process(buildCompleteDatagram(t, contracts.ResultStatusFailed)))

	// Stands in for the reboot killing the run: the owner tears down before the grace elapses.
	backend.Release()
	time.Sleep(backend.withheldFailureGrace * 4)

	assert.Len(t, outputChan, 0, "a released backend must not report the withheld failure")
	assert.Equal(t, contracts.ResultStatusInProgress, docState.DocumentInformation.DocumentStatus,
		"state must stay reboot-safe so the document resumes after the reboot")
}

// The grace must resolve before the Executer's zombie timer (60s) tears messaging down with a
// synthesized failure, otherwise the deferred report never wins the race.
func TestWithheldFailureGrace_BeatsZombieTimeout(t *testing.T) {
	assert.Less(t, defaultWithheldFailureGrace, 60*time.Second,
		"must fire before defaultZombieProcessTimeout")
}

// Regression: unregistering under docStateMu closes only the window where the snapshot
// is taken AFTER Process drops the document. snapshotInFlightPatchDocs takes just
// inFlightPatchDocsMu, so a preshutdown can capture this backend BEFORE that -- while Process is
// still inside its docStateMu section, having read the shutdown flag as unset, so it reports rather
// than withholds. No registry removal reaches an already-snapshotted pointer, and IsSuccess() does
// not protect a Failed, so only terminalReported stops the rewind. Calling markRebootSafe on a held
// pointer models that ordering.
func TestMarkRebootSafe_DoesNotRewindFailureAlreadyReported(t *testing.T) {
	resetShutdownState()
	defer resetShutdownState()

	docState := &contracts.DocumentState{
		DocumentInformation: contracts.DocumentInfo{
			DocumentID:     "doc-reported-failure",
			DocumentName:   "AWS-RunPatchBaseline",
			DocumentStatus: contracts.ResultStatusInProgress,
		},
		InstancePluginsInformation: []contracts.PluginState{
			{
				Id: "PatchWindows",
				Result: contracts.PluginResult{
					Status: contracts.ResultStatusFailed,
					Code:   1,
					Error:  "failed to run commands: exit status 1",
				},
			},
		},
	}
	backend := &ExecuterBackend{
		docState: docState,
		log:      log.NewMockLog(),
		output:   make(chan contracts.DocumentResult, 10),
		stopChan: make(chan int, 10),
	}
	var saved []contracts.DocumentState
	backend.BindDocumentStore(func(state contracts.DocumentState) { saved = append(saved, state) })

	// The preshutdown snapshot, taken before Process unregisters the document.
	snapshotted := backend

	// No shutdown signaled when the withhold gate reads the flag, so this Failed goes upstream.
	assert.NoError(t, backend.Process(buildCompleteDatagram(t, contracts.ResultStatusFailed)))
	assert.Equal(t, contracts.ResultStatusFailed, (<-backend.output).Status)
	assert.Equal(t, stopTypeTerminate, <-backend.stopChan)
	saved = nil

	// The shutdown now proceeds against the stale snapshot.
	SetMachineShuttingDown()
	snapshotted.markRebootSafe()

	assert.Equal(t, contracts.ResultStatusFailed, docState.DocumentInformation.DocumentStatus,
		"a reported Failed must not be rewound: the resumed run cannot report and instead re-runs "+
			"holding patch-baseline-concurrent.lock")
	assert.Equal(t, contracts.ResultStatusFailed, docState.InstancePluginsInformation[0].Result.Status,
		"the reported plugin failure must not be rewritten to InProgress")
	assert.Empty(t, saved, "an already-reported document must not be persisted as reboot-safe")
}

// The withheld-failure reporter is the other path that reports a terminal result, so a preshutdown
// holding a snapshot from before its unregister must not rewind what it just reported as Failed.
func TestReportWithheldFailure_MarksDocumentReportedSoShutdownCannotRewind(t *testing.T) {
	resetShutdownState()
	defer resetShutdownState()

	backend, docState := newPatchBackendForShutdown("doc-withheld-reported", contracts.ResultStatusFailed)
	backend.withheldFailureGrace = 10 * time.Millisecond
	backend.output = make(chan contracts.DocumentResult, 10)
	backend.stopChan = make(chan int, 10)
	backend.BindDocumentStore(func(contracts.DocumentState) {})

	// The preshutdown snapshot, taken before the reporter unregisters the document.
	snapshotted := backend

	SetMachineShuttingDown()
	assert.NoError(t, backend.Process(buildCompleteDatagram(t, contracts.ResultStatusFailed)))

	// The shutdown is aborted, so the withheld failure surfaces as the genuine failure it is.
	select {
	case result := <-backend.output:
		assert.Equal(t, contracts.ResultStatusFailed, result.Status)
	case <-time.After(5 * time.Second):
		t.Fatal("withheld failure was never reported after the grace period")
	}

	// A second preshutdown arrives and finds the stale snapshot.
	SetMachineShuttingDown()
	snapshotted.markRebootSafe()

	assert.Equal(t, contracts.ResultStatusFailed, docState.DocumentInformation.DocumentStatus,
		"a withheld failure that has since been reported must not be rewound to InProgress")
	assert.Equal(t, contracts.ResultStatusFailed, docState.InstancePluginsInformation[0].Result.Status)
}

// Regression: Release sets released under docStateMu but unregisters outside it, so a preshutdown can
// snapshot this backend in between. The exit that matters is a messaging error, where the Executer
// stamps Failed and sends a synthesized failure itself -- terminalReported is never set because the
// report bypassed Process, and IsSuccess() does not protect a Failed, so only released stops the
// rewind of an already-reported failure.
func TestMarkRebootSafe_DoesNotRewindReleasedDocument(t *testing.T) {
	resetShutdownState()
	defer resetShutdownState()

	backend, docState := newPatchBackendForShutdown("doc-released", contracts.ResultStatusFailed)
	var saved []contracts.DocumentState
	backend.BindDocumentStore(func(state contracts.DocumentState) { saved = append(saved, state) })

	// The preshutdown snapshot, taken before Release unregisters the document.
	snapshotted := backend

	// Stands in for the messaging-error exit: the Executer stamped Failed and reported it, then the
	// deferred Release ran. Nothing set terminalReported, because that report bypassed Process.
	backend.Release()
	assert.False(t, backend.terminalReported, "the messaging-error exit reports without Process")
	saved = nil

	SetMachineShuttingDown()
	snapshotted.markRebootSafe()

	assert.Equal(t, contracts.ResultStatusFailed, docState.DocumentInformation.DocumentStatus,
		"a released document must not be rewound: nothing is executing it, and its failure was reported")
	assert.Equal(t, contracts.ResultStatusFailed, docState.InstancePluginsInformation[0].Result.Status)
	assert.Empty(t, saved, "a released document must not be persisted as reboot-safe")
}

// The released guard must not over-block: a document still in flight, with its owner intact, is the
// case this path exists to rescue.
func TestMarkRebootSafe_StillRescuesUnreleasedInFlightDocument(t *testing.T) {
	resetShutdownState()
	defer resetShutdownState()

	backend, docState := newPatchBackendForShutdown("doc-still-running", contracts.ResultStatusFailed)
	var saved []contracts.DocumentState
	backend.BindDocumentStore(func(state contracts.DocumentState) { saved = append(saved, state) })

	PrepareForShutdown(log.NewMockLog())

	assert.Equal(t, contracts.ResultStatusInProgress, docState.DocumentInformation.DocumentStatus)
	assert.Len(t, saved, 1)
}
