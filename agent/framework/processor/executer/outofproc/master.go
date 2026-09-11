package outofproc

import (
	"fmt"
	"path/filepath"
	"runtime/debug"
	"strings"
	"time"

	"github.com/aws/amazon-ssm-agent/agent/appconfig"
	"github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/aws/amazon-ssm-agent/agent/contracts"
	"github.com/aws/amazon-ssm-agent/agent/framework/processor/executer"
	"github.com/aws/amazon-ssm-agent/agent/framework/processor/executer/basicexecuter"
	"github.com/aws/amazon-ssm-agent/agent/framework/processor/executer/outofproc/messaging"
	"github.com/aws/amazon-ssm-agent/agent/framework/processor/executer/outofproc/proc"
	"github.com/aws/amazon-ssm-agent/agent/log"
	logpkg "github.com/aws/amazon-ssm-agent/agent/log/logger"
	"github.com/aws/amazon-ssm-agent/agent/task"

	"github.com/aws/amazon-ssm-agent/common/filewatcherbasedipc"
	"github.com/aws/amazon-ssm-agent/common/identity"

	"github.com/aws/amazon-ssm-agent/core/executor"
)

type Backend messaging.MessagingBackend

// see differences between zombie and orphan: https://www.gmarik.info/blog/2012/orphan-vs-zombie-vs-daemon-processes/
const (
	// Give ample time for messaging block to successfully finish processing by itself.
	// Before sending the force shutdown signal in case it does not correctly exit.
	defaultZombieProcessTimeout = 60 * time.Second
	// If a process is cancelled, we give additional time for the cancellation process
	// to finish itself, and then send the force shutdown signal.
	defaultCancelledProcessTimeout = 300 * time.Second
	// command maximum timeout
	defaultOrphanProcessTimeout = 172800 * time.Second
)

type OutOfProcExecuter struct {
	basicexecuter.BasicExecuter
	docState   *contracts.DocumentState
	ctx        context.T
	cancelFlag task.CancelFlag
	executor   executor.IExecutor
}

var channelCreator = func(log log.T, identity identity.IAgentIdentity, mode filewatcherbasedipc.Mode, documentID string) (filewatcherbasedipc.IPCChannel, error, bool) {
	return filewatcherbasedipc.CreateFileWatcherChannel(log, identity, mode, documentID, false)
}

var channelRemover = filewatcherbasedipc.RemoveFileWatcherChannel

var processFinder = findProcess

// rewindPatchDocumentForReboot is a seam for tests, like processFinder and channelCreator above. The
// predicate it wraps reads process-global shutdown state that a test in this package cannot reset.
var rewindPatchDocumentForReboot = messaging.RewindPatchDocumentForReboot

// workerExecutables are the only binaries a document's ProcInfo can legitimately name. Derived from
// appconfig rather than spelled out, so renaming either worker there cannot silently desync this
// check and make every orphan lookup fail. Matching on an "ssm-" prefix instead would also accept the
// agent worker, letting the agent adopt its own parent as the document worker.
var workerExecutables = []string{
	workerBaseName(appconfig.DefaultDocumentWorker),
	workerBaseName(appconfig.DefaultSessionWorker),
}

// workerBaseName reduces a configured worker path to the lowercase, extension-free binary name that
// findProcess compares against. The basename is stable across every branch of appconfig's init, which
// varies the directory but never the file name. Lowercase before trimming, so a ".EXE" spelling is
// still recognized as the extension rather than kept as part of the name.
func workerBaseName(path string) string {
	return strings.TrimSuffix(strings.ToLower(filepath.Base(path)), ".exe")
}

// isWorkerExecutable matches on the worker name as a prefix so any suffix the platform adds counts --
// ".exe" on Windows today, and whatever a future packaging change appends -- because a name this
// rejects orphans a worker that is genuinely still running and lets the resume start a second one.
func isWorkerExecutable(name string) bool {
	for _, worker := range workerExecutables {
		if strings.HasPrefix(name, worker) {
			return true
		}
	}
	return false
}

func findProcess(log log.T, procinfo contracts.OSProcInfo, processExecutor executor.IExecutor) bool {
	if procinfo.Pid == 0 {
		return false
	}

	isRunning, err := processExecutor.IsPidRunning(procinfo.Pid)
	if err != nil {
		log.Errorf("failed to check if PID %d is running: %v", procinfo.Pid, err)
		return false
	}
	if !isRunning {
		log.Infof("PID %d is not running", procinfo.Pid)
		return false
	}

	processes, err := processExecutor.Processes()
	if err != nil {
		log.Warnf("failed to enumerate processes to verify PID %d, assuming valid: %v", procinfo.Pid, err)
		return true
	}

	for _, p := range processes {
		if p.Pid == procinfo.Pid {
			name := strings.ToLower(filepath.Base(p.Executable))
			if !isWorkerExecutable(name) {
				log.Warnf("PID %d belongs to %q, not a document/session worker; treating as not found", procinfo.Pid, name)
				return false
			}
			log.Debugf("PID %d confirmed as worker %q", procinfo.Pid, name)
			return true
		}
	}

	log.Infof("PID %d exited between running check and process enumeration", procinfo.Pid)
	return false
}

var processCreator = func(name string, argv []string) (proc.OSProcess, error) {
	return proc.StartProcess(name, argv)
}

func NewOutOfProcExecuter(ctx context.T) *OutOfProcExecuter {
	newContext := ctx.With("[OutOfProcExecuter]")
	return &OutOfProcExecuter{
		BasicExecuter: *basicexecuter.NewBasicExecuter(ctx),
		ctx:           newContext,
		executor:      executor.NewProcessExecutor(newContext.Log()),
	}
}

// Run() prepare the ipc channel, create a data processing backend and start messaging with docment worker
func (e *OutOfProcExecuter) Run(
	cancelFlag task.CancelFlag,
	docStore executer.DocumentStore,
) chan contracts.DocumentResult {
	docState := docStore.Load()
	e.docState = &docState
	e.cancelFlag = cancelFlag
	documentID := docState.DocumentInformation.DocumentID

	// update context with the document id
	e.ctx = e.ctx.With("[" + documentID + "]")
	log := e.ctx.Log()

	// stopTimer signals messaging routine to stop, it's buffered because it needs to exit if messaging is already stopped and not receiving anymore
	stopTimer := make(chan bool, 1)
	// start prepare messaging
	// if anything fails during the prep stage, use in-proc Runner
	ipc, err := e.initialize(stopTimer)
	// save doc store immediately in case agent restarts.
	docStore.Save(*e.docState)

	if err != nil {
		log.Errorf("failed to prepare outofproc executer, falling back to InProc Executer")
		log.WriteEvent(logpkg.AgentTelemetryMessage, "", logpkg.AmazonAgentInProcExecuterStartEvent)

		return e.BasicExecuter.Run(cancelFlag, docStore)
	} else {
		// create reply channel
		resChan := make(chan contracts.DocumentResult, len(e.docState.InstancePluginsInformation)+1)
		// launch the messaging go-routine
		go func(store executer.DocumentStore) {
			defer func() {
				if msg := recover(); msg != nil {
					log.Errorf("Executer go-routine panic: %v", msg)
					log.Errorf("Stacktrace:\n%s", debug.Stack())
				}
				// save the overall result and signal called that Executer is done
				store.Save(*e.docState)
				log.Debug("Executer closed")
				close(resChan)
			}()

			e.messaging(log, ipc, resChan, cancelFlag, stopTimer)
		}(docStore)

		return resChan
	}
}

// Executer spins up an ipc transmission worker, it creates a Data processing backend and hands off the backend to the ipc worker
// ipc worker and data backend act as 2 threads exchange raw json messages, and messaging protocol happened in data backend, data backend is self-contained and exit when command finishes accordingly
// Executer however does hold a timer to the worker to forcefully termniate both of them
func (e *OutOfProcExecuter) messaging(log log.T, ipc filewatcherbasedipc.IPCChannel, resChan chan contracts.DocumentResult, cancelFlag task.CancelFlag, stopTimer chan bool) {
	// backup current time in case outofproc execution failed and cannot correctly return PluginResult
	backupStartTime := time.Now()

	// handoff reply functionalities to data backend.
	backend := messaging.NewExecuterBackend(log, resChan, e.docState, cancelFlag)

	// handoff the data backend to messaging worker
	if err := messaging.Messaging(log, ipc, backend, stopTimer); err != nil {
		// the messaging worker encountered error, either ipc run into error or data backend throws error
		log.TelemetryErrorf("messaging worker encountered error: %v", err)
		log.TelemetryErrorf("document state during messaging worker error: %v", e.docState.DocumentInformation.DocumentStatus)
		reachedTerminalStatus := e.docState.DocumentInformation.DocumentStatus != contracts.ResultStatusInProgress &&
			e.docState.DocumentInformation.DocumentStatus != "" &&
			e.docState.DocumentInformation.DocumentStatus != contracts.ResultStatusNotStarted &&
			e.docState.DocumentInformation.DocumentStatus != contracts.ResultStatusSuccessAndReboot

		// A patch document the machine shutdown cut short has not failed, so rewind it to a resumable
		// state rather than stamping the failure below. Gated behind the terminal-status check: a
		// document that already reached a terminal status reported it upstream, and re-running it
		// would re-run work whose result is already public.
		if !reachedTerminalStatus {
			if rewindPatchDocumentForReboot(log, e.docState) {
				// Leave the channel on disk. The resume recreates it from scratch in initialize(),
				// and tearing it down here would race the worker's own shutdown for no benefit --
				// this process is about to go down with the machine.
				return
			}
			e.docState.DocumentInformation.DocumentStatus = contracts.ResultStatusFailed
			log.Info("document failed half way, sending fail message...")
			errMsg := fmt.Sprintf("document process failed unexpectedly: %s , check [ssm-document-worker]/[ssm-session-worker] log for crash reason", err)
			resChan <- e.generateUnexpectedFailResult(errMsg, backupStartTime)
		}
		// destroy the channel
		ipc.Destroy()
	}
}

func (e *OutOfProcExecuter) generateUnexpectedFailResult(errMsg string, startTime time.Time) contracts.DocumentResult {
	endTime := time.Now()
	var docResult contracts.DocumentResult
	docResult.MessageID = e.docState.DocumentInformation.MessageID
	docResult.AssociationID = e.docState.DocumentInformation.AssociationID
	docResult.DocumentName = e.docState.DocumentInformation.DocumentName
	docResult.NPlugins = len(e.docState.InstancePluginsInformation)
	docResult.DocumentVersion = e.docState.DocumentInformation.DocumentVersion
	docResult.Status = contracts.ResultStatusFailed
	docResult.PluginResults = make(map[string]*contracts.PluginResult)
	res := e.docState.InstancePluginsInformation[0].Result
	res.Output = errMsg
	res.Status = contracts.ResultStatusFailed
	res.StartDateTime = startTime
	res.EndDateTime = endTime
	docResult.PluginResults[e.docState.InstancePluginsInformation[0].Id] = &res
	return docResult
}

// prepare the channel for messaging as well as launching the document worker process, if the channel already exists, re-open it.
// launch timeout timer based off the discovered process status
//
// This initialize block handles both start and continuation of document executions from state on filesystem
// For continuation, it is assumed that agent worker process can have interruption (e.g. UpdateSSMAgent) and
// that the document worker process responsible for command execution are still alive and can be reconnected
// In the case where a reboot or global cessation of the SSM Agent, both processes are terminated, and we
// cannot establish correct state to restart unless it is explicitly handled by the command document
func (e *OutOfProcExecuter) initialize(stopTimer chan bool) (ipc filewatcherbasedipc.IPCChannel, err error) {
	log := e.ctx.Log()
	var found bool
	documentID := e.docState.DocumentInformation.DocumentID

	// Due to non-agent scheduled additional reboots during Windows update process caused by automated process
	// recently added by Microsoft, we are allowing Patch Documents to continue from any reboot as the
	// patch documents are state safe with dedicated state tracking and this allow for correct patch visibility
	isPatchDocument := contracts.IsPatchDocument(e.docState.DocumentInformation.DocumentName)

	// this fix is added to delete channels which were not deleted in previous execution
	if e.docState.DocumentInformation.DocumentStatus == contracts.ResultStatusSuccessAndReboot {
		log.Info("deleting channel for reboot document")
		if channelErr := channelRemover(e.ctx.Identity(), documentID); channelErr != nil {
			log.Warnf("failed to remove channel directory: %v", channelErr)
		}
		// change document status to in progress now instead of waiting for worker to avoid
		// the chance agent is restarted before worker updates, leaving status as SuccessAndReboot,
		// and resulting in agent deleting an in use file channel when it comes back up
		e.docState.DocumentInformation.DocumentStatus = contracts.ResultStatusInProgress
	}
	ipc, err, found = channelCreator(log, e.ctx.Identity(), filewatcherbasedipc.ModeMaster, documentID)

	if err != nil {
		log.TelemetryErrorf("failed to create ipc channel: %v", err)
		return
	}
	if found {
		log.Info("discovered old channel object, trying to find detached process...")
		procInfo := e.docState.DocumentInformation.ProcInfo
		if processFinder(log, procInfo, e.executor) {
			log.Infof("found orphan process: %v, start time: %v", procInfo.Pid, procInfo.StartTime)
			go timeoutOrCancel(stopTimer, defaultOrphanProcessTimeout, e.cancelFlag)
		} else if isPatchDocument && e.docState.DocumentInformation.DocumentStatus == contracts.ResultStatusInProgress {
			log.TelemetryWarnf("Continue patch execution from presumed reboot")
			// A worker whose terminal result was never consumed may have left a Failed here, whether
			// killed mid-write or exited cleanly with no reader left. The channel is a directory that
			// survives the reboot and is drained on reopen, so reusing it would replay that Failed as
			// a real failure. Recreate it clean.
			//
			// Close then channelRemover rather than ipc.Destroy(): Destroy does the same two steps but
			// swallows the removal error, and on Windows the removal can genuinely fail while a handle
			// is still open. Close first anyway -- releasing our own watcher is what lets the removal
			// succeed, and it stops the wrapper's goroutine and fsnotify handle from leaking for the
			// life of the agent worker, since Run discards ipc once err is set.
			ipc.Close()
			if removeErr := channelRemover(e.ctx.Identity(), documentID); removeErr != nil {
				// Proceed anyway. A removal that failed leaves whatever the previous worker never
				// consumed in place, so the resume can still be cut short by that stale result -- but
				// refusing to resume at all strands the patch run, and the reopened channel is drained
				// as the run proceeds, so proceeding is the better failure mode.
				log.TelemetryWarnf("failed to remove stale ipc channel for patch resume, resuming against "+
					"the existing channel: %v", removeErr)
			}
			if ipc, err, _ = channelCreator(log, e.ctx.Identity(), filewatcherbasedipc.ModeMaster, documentID); err != nil {
				log.TelemetryErrorf("failed to recreate ipc channel for patch resume: %v", err)
				return
			}
			err = e.startNewWorkerProcess(stopTimer, documentID, ipc)
		} else {
			log.TelemetryWarnf("process: %v not found, treat as exited", procInfo.Pid)
			go timeoutOrCancel(stopTimer, defaultZombieProcessTimeout, e.cancelFlag)
		}
	} else {
		err = e.startNewWorkerProcess(stopTimer, documentID, ipc)
	}

	return
}

// startNewWorkerProcess starts a new worker process for document execution
func (e *OutOfProcExecuter) startNewWorkerProcess(stopTimer chan bool, documentID string, ipc filewatcherbasedipc.IPCChannel) error {
	log := e.ctx.Log()
	log.Debug("channel not found, starting a new process...")

	var workerName string
	if e.docState.DocumentType == contracts.StartSession {
		workerName = appconfig.DefaultSessionWorker
	} else {
		workerName = appconfig.DefaultDocumentWorker
	}

	var process proc.OSProcess
	var err error
	if process, err = processCreator(workerName, []string{documentID}); err != nil {
		log.Errorf("start process: %v error: %v", workerName, err)
		// make sure close the channel
		ipc.Destroy()
		return err
	} else {
		log.Debugf("successfully launched new process: %v", process.Pid())
	}

	e.docState.DocumentInformation.ProcInfo = contracts.OSProcInfo{
		Pid:       process.Pid(),
		StartTime: process.StartTime(),
	}

	// TODO add command timeout as well, in case process get stuck
	go e.WaitForProcess(stopTimer, process)
	return nil
}

func (e *OutOfProcExecuter) WaitForProcess(stopTimer chan bool, process proc.OSProcess) {
	log := e.ctx.Log()
	//TODO revisit this feature, it has done sides of killing the document worker too fast -- the worker might busy doing s3 upload
	//waitReturned := false
	//go func() {
	//	//if job complete but process still hangs, kill it.
	//	e.cancelFlag.Wait()
	//	if !waitReturned && e.cancelFlag.State() == task.Completed {
	//		//do not kill it immediately, should be grace-period to allow s3 upload to finish
	//		<-time.After(defaultZombieProcessTimeout)
	//		log.Info("killing process...")
	//		process.Kill()
	//	}
	//}()
	defer func() {
		if r := recover(); r != nil {
			log.Errorf("Wait for process panic: %v", r)
			log.Errorf("Stacktrace:\n%s", debug.Stack())
		}
	}()
	if err := process.Wait(); err != nil {
		log.Errorf("process: %v exited unsuccessfully, error message: %v", process.Pid(), err)
	} else {
		log.Debugf("process: %v exited successfully, trying to stop messaging worker", process.Pid())
	}
	// waitReturned = true
	timeout(stopTimer, defaultZombieProcessTimeout)
}

func timeoutOrCancel(stopTimer chan bool, duration time.Duration, cancelFlag task.CancelFlag) {
	stopChan := make(chan bool, 1)
	// TODO refactor cancelFlag.Wait() to return channel instead of blocking call
	go func() {
		cancelFlag.Wait()
		stopChan <- true
	}()
	select {
	case <-time.After(duration):
		stopTimer <- true
	case <-stopChan:
		go timeout(stopTimer, defaultCancelledProcessTimeout)
	}
}

func timeout(stopTimer chan bool, duration time.Duration) {
	<-time.After(duration)
	stopTimer <- true
}
