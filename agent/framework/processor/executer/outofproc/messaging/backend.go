package messaging

import (
	"errors"
	"runtime/debug"
	"sync"
	"sync/atomic"
	"time"

	"github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/aws/amazon-ssm-agent/agent/contracts"
	"github.com/aws/amazon-ssm-agent/agent/jsonutil"
	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/amazon-ssm-agent/agent/task"
)

const (
	//make sure the channel operation in backend is not blocked
	defaultBackendChannelSize = 10
	// Backend states with potential future addition to further manage backend
	// Currently with Init and Processing statuses to avoid idle process leak.
	BackendStateInit int32 = 0
	BackendStateProc int32 = 1
)

type PluginRunner func(
	context context.T,
	docState contracts.DocumentState,
	resChan chan contracts.PluginResult,
	cancelFlag task.CancelFlag,
)

// servicingRebootChecker reports whether a machine shutdown has been signaled, gating whether a
// Failed patch result was reboot-caused. Backed by the flag core sets over IPC. Injectable.
var servicingRebootChecker = func(log.T) bool { return IsMachineShuttingDown() }

// worker backend receives request messages from master, controls a pluginRunner based off the request and send reponses to Executer
type WorkerBackend struct {
	ctx        context.T
	input      chan string
	once       sync.Once
	cancelFlag task.CancelFlag
	runner     PluginRunner
	stopChan   chan int
	state      atomic.Int32
}

// Executer backend formulate the run request to the worker, and collect back the responses from worker
type ExecuterBackend struct {
	//the shared state object that Executer hand off to data backend
	docState   *contracts.DocumentState
	input      chan string
	cancelFlag task.CancelFlag
	output     chan contracts.DocumentResult
	stopChan   chan int
	log        log.T
	// saveDocState persists docState; injected by the Executer, which owns the store.
	saveDocState func(contracts.DocumentState)
	// docStateMu serializes docState and the store save between the messaging worker and the
	// shutdown-preparation handler, which run on separate goroutines and overlap.
	docStateMu sync.Mutex
	// released records that the backend's owner has torn down, so the deferred reporter knows the
	// result channel is about to close and must not be written to. Guarded by docStateMu.
	released bool
	// terminalReported records that a terminal result went upstream, so the shutdown path must not
	// rewind the document to InProgress. Guarded by docStateMu, and set under the same lock
	// acquisition that stamps the terminal status so a waiting markRebootSafe observes it.
	terminalReported bool
	// withheldFailureGrace overrides defaultWithheldFailureGrace. Per-backend rather than a package
	// var so tests can shorten it without writing to shared state the reporter goroutine reads.
	withheldFailureGrace time.Duration
}

// defaultWithheldFailureGrace is how long a withheld patch failure is held before it is reported as
// the genuine failure it may be. Withholding is only correct if a reboot actually follows: the
// process dies during this wait and the document resumes from its persisted InProgress state.
// Surviving the wait means the shutdown was aborted or vetoed, and the truthful answer is the Failed
// we withheld.
//
// Bounded above by defaultZombieProcessTimeout (60s): once the document worker exits, the Executer's
// zombie timer tears messaging down at 60s with a synthesized failure, so this report must win that
// race. Bounded below by how long the agent worker hosting this goroutine outlives preshutdown --
// core does not hold the SCM window open, so it is core's PrepareShutdown (bounded to 5s) plus its
// stop sequence, not the SCM's version-dependent allowance.
const defaultWithheldFailureGrace = 45 * time.Second

func NewExecuterBackend(log log.T, output chan contracts.DocumentResult, docState *contracts.DocumentState, cancelFlag task.CancelFlag) *ExecuterBackend {
	stopChan := make(chan int, defaultBackendChannelSize)
	inputChan := make(chan string, defaultBackendChannelSize)
	p := ExecuterBackend{
		output:     output,
		docState:   docState,
		input:      inputChan,
		cancelFlag: cancelFlag,
		stopChan:   stopChan,
		log:        log,

		withheldFailureGrace: defaultWithheldFailureGrace,
	}
	// start marshals this on its own goroutine; pass a deep copy so the plugin slice does not
	// alias the live document.
	go p.start(log, p.copyDocState())
	return &p
}

// PrepareForShutdown makes every in-flight patch document reboot-safe ahead of a machine shutdown
// and records that a shutdown is underway. Runs while core blocks the OS in the SCM preshutdown
// window, so it is ordered ahead of the document worker being killed. Returns the number prepared.
//
// The flag covers a result that still arrives after this point; the persisted InProgress state
// covers the case where none ever arrives because the worker was killed first.
func PrepareForShutdown(logger log.T) int {
	SetMachineShuttingDown()

	backends := snapshotInFlightPatchDocs()
	for _, backend := range backends {
		backend.markRebootSafe()
	}
	if len(backends) > 0 {
		logger.Infof("machine shutdown signaled: marked %v in-flight patch document(s) reboot-safe "+
			"so an interrupted run resumes instead of reporting a false-negative failure", len(backends))
	}
	return len(backends)
}

// markRebootSafe forces an in-flight patch document to InProgress so the resume re-runs it instead
// of reporting a reboot-interrupted Failed as terminal. Only already-Failed plugin results are
// rewritten. Persists synchronously because the Executer's end-of-run save may never happen.
//
// A document whose result is already decided is left untouched -- see the guard below. A Failed or
// still-running document during the shutdown window is presumed reboot-interrupted and is still
// rescued; that false-negative Failed is exactly what this path exists to prevent.
func (p *ExecuterBackend) markRebootSafe() {
	p.docStateMu.Lock()
	// Skip a document that is no longer this backend's to rewind. All three guards are needed because
	// snapshotInFlightPatchDocs takes only inFlightPatchDocsMu, so a snapshot taken before a registry
	// removal still holds this backend:
	//
	//   terminalReported -- the result went upstream via Process or the withheld reporter.
	//   released         -- the owner tore down, covering the exits that report without Process.
	//   IsSuccess        -- a Success arrived by reply or from disk.
	//
	// Rewinding any of those re-runs a document whose result is already decided, and the re-run holds
	// patch-baseline-concurrent.lock, failing unrelated commands. A still-running document, or an
	// unreported Failed, is the false negative this path rescues, so it MUST still be rewound.
	if p.terminalReported || p.released || p.docState.DocumentInformation.DocumentStatus.IsSuccess() {
		documentID := p.docState.DocumentInformation.DocumentID
		documentStatus := p.docState.DocumentInformation.DocumentStatus
		terminalReported := p.terminalReported
		released := p.released
		p.docStateMu.Unlock()
		p.log.Debugf("patch document %v is no longer rewindable (status=%v, reported upstream=%v, "+
			"released=%v); not rewinding to InProgress for reboot",
			documentID, documentStatus, terminalReported, released)
		return
	}
	p.resetRebootInterruptedPluginResults()
	p.docState.DocumentInformation.DocumentStatus = contracts.ResultStatusInProgress
	stateCopy := p.copyDocStateLocked()
	documentID := p.docState.DocumentInformation.DocumentID
	documentName := p.docState.DocumentInformation.DocumentName
	save := p.saveDocState
	p.docStateMu.Unlock()

	if save != nil {
		save(stateCopy)
	} else {
		p.log.Warnf("no document store bound for %v; reboot-safe state kept in memory only", documentID)
	}
	p.log.TelemetryWarnf("patch document %v (name=%v) marked reboot-safe ahead of machine shutdown; "+
		"status kept InProgress to resume after the reboot", documentID, documentName)
}

// copyDocState copies docState, taking the lock.
func (p *ExecuterBackend) copyDocState() contracts.DocumentState {
	p.docStateMu.Lock()
	defer p.docStateMu.Unlock()
	return p.copyDocStateLocked()
}

// copyDocStateLocked copies docState for the store. Caller must hold docStateMu. The plugin slice
// is deep-copied so the copy does not alias the live document.
func (p *ExecuterBackend) copyDocStateLocked() contracts.DocumentState {
	stateCopy := *p.docState
	if p.docState.InstancePluginsInformation != nil {
		plugins := make([]contracts.PluginState, len(p.docState.InstancePluginsInformation))
		copy(plugins, p.docState.InstancePluginsInformation)
		stateCopy.InstancePluginsInformation = plugins
	}
	return stateCopy
}

// snapshotPluginResultsLocked copies each plugin's result by plugin ID, so a later rewrite of the
// live document can be undone. Caller must hold docStateMu.
func (p *ExecuterBackend) snapshotPluginResultsLocked() map[string]contracts.PluginResult {
	results := make(map[string]contracts.PluginResult, len(p.docState.InstancePluginsInformation))
	for _, plugin := range p.docState.InstancePluginsInformation {
		results[plugin.Id] = plugin.Result
	}
	return results
}

// Release drops the document from the in-flight registry. Idempotent, and safe on a document that
// was never registered. Deferred by the owner of the backend's lifetime so that every exit -- the
// withheld-result early return, a messaging error, or a panic -- stops the shutdown path from
// mutating a document that is no longer running.
func (p *ExecuterBackend) Release() {
	p.docStateMu.Lock()
	documentID := p.docState.DocumentInformation.DocumentID
	// Runs before the owner closes the result channel, so this is what tells a pending withheld-
	// failure reporter that it has nowhere left to write.
	p.released = true
	p.docStateMu.Unlock()

	unregisterInFlightPatchDoc(documentID)
}

// BindDocumentStore supplies the save callback used by the shutdown path, so the backend does not
// depend on the store abstraction. Registering the document here rather than at construction keeps
// it invisible to the shutdown path until a store is bound, which would otherwise leave
// markRebootSafe unable to persist.
func (p *ExecuterBackend) BindDocumentStore(save func(contracts.DocumentState)) {
	p.docStateMu.Lock()
	p.saveDocState = save
	documentID := p.docState.DocumentInformation.DocumentID
	documentName := p.docState.DocumentInformation.DocumentName
	p.docStateMu.Unlock()

	if save != nil && contracts.IsPatchDocument(documentName) {
		registerInFlightPatchDoc(documentID, p)
	}
}

func (p *ExecuterBackend) start(log log.T, docState contracts.DocumentState) {
	defer func() {
		if r := recover(); r != nil {
			log.Errorf("Executer backend start panic: \n%v", r)
			log.Errorf("Stacktrace:\n%s", debug.Stack())
		}
	}()
	startDatagram, _ := CreateDatagram(MessageTypePluginConfig, docState)
	p.input <- startDatagram
	p.cancelFlag.Wait()
	if p.cancelFlag.Canceled() {
		cancelDatagram, _ := CreateDatagram(MessageTypeCancel, "cancel")
		p.input <- cancelDatagram
	} else if p.cancelFlag.ShutDown() {
		p.stopChan <- stopTypeShutdown
	}
	//cancel state is complete, safe return
	close(p.input)
}

func (p *ExecuterBackend) Accept() <-chan string {
	return p.input
}

func (p *ExecuterBackend) Stop() <-chan int {
	return p.stopChan
}

func (p *ExecuterBackend) Close() {
	p.input = nil
}

func (p *ExecuterBackend) CloseStop() {
	return
}

func (p *ExecuterBackend) GetBackendState() int32 {
	// For Executor Backend, default to processing as initiator of communications
	return BackendStateProc
}

func (p *ExecuterBackend) ForceQuit() {
	// Force Quit is currently targeted for WorkerBackend, purposefully do
	// nothing for Executor Backend until we find a valid reason to do so.
	return
}

// TODO handle error and logging, when err, ask messaging to stop
// TODO version handling?
func (p *ExecuterBackend) Process(datagram string) error {
	t, content := ParseDatagram(datagram)
	switch t {
	case MessageTypeReply, MessageTypeComplete:
		var docResult contracts.DocumentResult
		jsonutil.Unmarshal(content, &docResult)
		// A Failed patch document cut short by a servicing reboot is a false negative: withhold
		// the terminal result and keep it InProgress so it resumes. Bounded by CommandRetryLimit
		// and the 48h document expiry.
		// The shutdown-preparation handler writes docState concurrently; hold the lock across the
		// gate check and the mutations.
		p.docStateMu.Lock()
		if t == MessageTypeComplete && p.isRebootInterruptedPatchFailureLocked(&docResult) {
			// Snapshot the real plugin results before the rewrite discards them. Restoring from
			// docResult.PluginResults instead would depend on the worker having populated that map.
			withheldPlugins := p.snapshotPluginResultsLocked()
			p.resetRebootInterruptedPluginResults()
			p.docState.DocumentInformation.DocumentStatus = contracts.ResultStatusInProgress
			stateCopy := p.copyDocStateLocked()
			save := p.saveDocState
			p.docStateMu.Unlock()

			// Persist the reboot-safe state now: if the machine does go down, this is what the
			// resume reads, and the Executer's end-of-run save may never happen.
			if save != nil {
				save(stateCopy)
			}
			// Hold the failure rather than discard it, and leave messaging running. A real reboot
			// kills this process during the wait, which is what makes the withholding correct. If we
			// are still alive afterwards the shutdown never happened, and the withheld failure is
			// reported as Failed instead of leaving the document InProgress until some later restart.
			go p.reportWithheldFailureUnlessShutdownProceeds(docResult, withheldPlugins)
			return nil
		}
		p.formatDocResult(&docResult)
		if t == MessageTypeComplete {
			// Record the decision and unregister while still holding docStateMu (the lock
			// markRebootSafe also takes), so a concurrent PrepareForShutdown cannot rewind a
			// finished document to InProgress and resume an already-reported run.
			//
			// Both guards are needed, for opposite orderings: removing from the registry here hides
			// the backend from a snapshot taken after this point, and the flag stops one taken
			// before it, which already holds the pointer. Lifetime cleanup is still the deferred
			// Release in the messaging loop.
			p.terminalReported = true
			unregisterInFlightPatchDoc(p.docState.DocumentInformation.DocumentID)
		}
		p.docStateMu.Unlock()

		if t != MessageTypeComplete {
			p.output <- docResult
			return nil
		}

		p.output <- docResult
		//get document result, force termniate messaging worker
		p.stopChan <- stopTypeTerminate
	default:
		return errors.New("unsupported message type")
	}
	return nil
}

// reportWithheldFailureUnlessShutdownProceeds waits out withheldFailureGrace and then, if this
// process is still running and its backend has not been torn down, reports the failure that was
// withheld when the shutdown was signaled.
//
// Reaching the report means the machine did not go down, so the document did not survive a reboot --
// it simply failed. Restoring the Failed status before reporting matters as much as the report
// itself: the withheld path persisted InProgress, and leaving that on disk would send the resume
// path in initialize() after a document whose failure has just been reported, re-running the patch.
func (p *ExecuterBackend) reportWithheldFailureUnlessShutdownProceeds(docResult contracts.DocumentResult, withheldPlugins map[string]contracts.PluginResult) {
	defer func() {
		if msg := recover(); msg != nil {
			// A send on the closed result channel lands here: the owner tore down in the instant
			// after the released check. Nothing left to report to.
			p.log.Errorf("withheld failure reporter stopped: %v", msg)
		}
	}()

	grace := p.withheldFailureGrace
	if grace <= 0 {
		grace = defaultWithheldFailureGrace
	}
	time.Sleep(grace)

	p.docStateMu.Lock()
	if p.released {
		documentID := p.docState.DocumentInformation.DocumentID
		p.docStateMu.Unlock()
		p.log.Debugf("document %v is no longer running; not reporting the withheld failure", documentID)
		return
	}

	// Undo the reboot-safe rewrite: put back the failure the worker actually reported.
	p.docState.DocumentInformation.DocumentStatus = contracts.ResultStatusFailed
	for i := range p.docState.InstancePluginsInformation {
		if result, ok := withheldPlugins[p.docState.InstancePluginsInformation[i].Id]; ok {
			p.docState.InstancePluginsInformation[i].Result = result
		}
	}
	p.formatDocResult(&docResult)
	stateCopy := p.copyDocStateLocked()
	save := p.saveDocState
	documentID := p.docState.DocumentInformation.DocumentID
	documentName := p.docState.DocumentInformation.DocumentName
	// Terminal now, so another preshutdown arriving while this reports must not rewind it -- whether
	// its snapshot was taken after the registry removal or before it.
	p.terminalReported = true
	unregisterInFlightPatchDoc(documentID)
	p.docStateMu.Unlock()

	if save != nil {
		save(stateCopy)
	}
	p.log.TelemetryWarnf("machine did not shut down within %v of withholding patch document %v's (name=%v) "+
		"failure, so the shutdown did not proceed; reporting the failure instead of leaving it InProgress",
		grace, documentID, documentName)

	p.output <- docResult
	// Non-blocking: if the owner has already stopped consuming, there is nothing to terminate.
	select {
	case p.stopChan <- stopTypeTerminate:
	default:
	}
}

// isRebootInterruptedPatchFailureLocked reports whether a Failed patch result was caused by a
// servicing reboot rather than a genuine failure. Caller must hold docStateMu.
func (p *ExecuterBackend) isRebootInterruptedPatchFailureLocked(docResult *contracts.DocumentResult) bool {
	if docResult.Status != contracts.ResultStatusFailed {
		return false
	}
	if !contracts.IsPatchDocument(p.docState.DocumentInformation.DocumentName) {
		return false
	}
	if !servicingRebootChecker(p.log) {
		return false
	}
	p.log.TelemetryWarnf("patch document %v (name=%v) reported Failed after a CBS servicing reboot; keeping status "+
		"InProgress to resume instead of reporting a false-negative failure",
		p.docState.DocumentInformation.DocumentID,
		p.docState.DocumentInformation.DocumentName)
	return true
}

// resetRebootInterruptedPluginResults rewrites Failed plugin results to InProgress so the resumed
// worker re-runs the step instead of replaying the Failed. InProgress rather than
// SuccessAndReboot: the processor acts on SuccessAndReboot by requesting a reboot, and this gate
// is a heuristic that can false-positive. Caller must hold docStateMu.
func (p *ExecuterBackend) resetRebootInterruptedPluginResults() {
	for i := range p.docState.InstancePluginsInformation {
		if p.docState.InstancePluginsInformation[i].Result.Status == contracts.ResultStatusFailed {
			p.docState.InstancePluginsInformation[i].Result.Status = contracts.ResultStatusInProgress
			p.docState.InstancePluginsInformation[i].Result.Code = 0
			p.docState.InstancePluginsInformation[i].Result.Error = ""
		}
	}
}

func (p *ExecuterBackend) formatDocResult(docResult *contracts.DocumentResult) {
	//fill doc level information that the sub-process wouldn't know
	docResult.MessageID = p.docState.DocumentInformation.MessageID
	docResult.AssociationID = p.docState.DocumentInformation.AssociationID
	docResult.DocumentName = p.docState.DocumentInformation.DocumentName
	docResult.NPlugins = len(p.docState.InstancePluginsInformation)
	docResult.DocumentVersion = p.docState.DocumentInformation.DocumentVersion
	//update current document status
	contracts.UpdateDocState(docResult, p.docState)
}

func NewWorkerBackend(ctx context.T, runner PluginRunner) *WorkerBackend {
	stopChan := make(chan int)
	return &WorkerBackend{
		ctx:        ctx.With("[DataBackend]"),
		input:      make(chan string),
		cancelFlag: task.NewChanneledCancelFlag(),
		runner:     runner,
		stopChan:   stopChan,
		state:      atomic.Int32{},
	}
}

func (p *WorkerBackend) Process(datagram string) error {
	t, content := ParseDatagram(datagram)
	log := p.ctx.Log()
	p.state.Store(BackendStateProc)
	switch t {
	case MessageTypePluginConfig:
		log.Info("received plugin config message")
		var docState contracts.DocumentState
		log.Info(content)
		if err := jsonutil.Unmarshal(content, &docState); err != nil {
			log.Errorf("failed to unmarshal plugin config: %v", err)
			//TODO request messaging to stop
			return err
		}
		log.Debugf("unmarshal plugin config: %+v", docState)
		p.once.Do(func() {
			statusChan := make(chan contracts.PluginResult)
			go p.runner(p.ctx, docState, statusChan, p.cancelFlag)
			go p.pluginListener(statusChan)
		})

	case MessageTypeCancel:
		log.Info("requested cancel the command, setting cancel flag...")
		p.cancelFlag.Set(task.Canceled)
	default:
		//TODO add extra logic to check whether plugin has started, if not, stop IPC, or add timeout
		return errors.New("unsupported message type")
	}
	return nil
}

func (p *WorkerBackend) pluginListener(statusChan chan contracts.PluginResult) {
	log := p.ctx.Log()
	results := make(map[string]*contracts.PluginResult)
	var finalStatus contracts.ResultStatus
	defer func() {
		//if this routine panics, return failed results
		if msg := recover(); msg != nil {
			log.Errorf("plugin listener panic: %v", msg)
			log.Errorf("Stacktrace:\n%s", debug.Stack())
			finalStatus = contracts.ResultStatusFailed
		}

		docResult := contracts.DocumentResult{
			Status:        finalStatus,
			PluginResults: results,
			LastPlugin:    "",
		}
		log.Info("sending document complete response...")
		completeMessage, _ := CreateDatagram(MessageTypeComplete, docResult)
		p.input <- completeMessage
		close(p.input)
		log.Info("stopping ipc worker...")
		//sending stop signal
		p.stopChan <- stopTypeShutdown
		close(p.stopChan)
	}()

	for res := range statusChan {
		var result = res
		results[res.PluginID] = &result
		//TODO move the aggregator under executer package and protect it, there's global lock in this package
		status, _, _, _ := contracts.DocumentResultAggregator(log, res.PluginID, results)
		docResult := contracts.DocumentResult{
			Status:        status,
			PluginResults: results,
			LastPlugin:    res.PluginID,
		}
		replyMessage, _ := CreateDatagram(MessageTypeReply, docResult)
		log.Debugf("plugin: %v done, sending reply message...", res.PluginID)
		p.input <- replyMessage
	}
	log.Info("document execution complete")
	finalStatus, _, _, _ = contracts.DocumentResultAggregator(log, "", results)

}

func (p *WorkerBackend) Accept() <-chan string {
	return p.input
}

func (p *WorkerBackend) Stop() <-chan int {
	return p.stopChan
}

func (p *WorkerBackend) Close() {
	p.input = nil
}

func (p *WorkerBackend) CloseStop() {
	p.stopChan = nil
}

func (p *WorkerBackend) GetBackendState() int32 {
	return p.state.Load()
}

func (p *WorkerBackend) ForceQuit() {
	p.stopChan <- stopTypeShutdown
	close(p.stopChan)
}
