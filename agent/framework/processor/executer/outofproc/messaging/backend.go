package messaging

import (
	"errors"
	"runtime/debug"
	"sync/atomic"

	"sync"

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
}

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
	}
	go p.start(log, *docState)
	return &p
}

// shutdownChecker reports whether a machine shutdown has been signaled, gating whether a
// Failed patch result was reboot-caused. Backed by the flag core sets over IPC. Injectable.
var shutdownChecker = func(log.T) bool { return IsMachineShuttingDown() }

// PrepareForShutdown records that core relayed a machine-shutdown notification, so a patch document
// that fails or is cut short from here on is treated as reboot-interrupted rather than failed. Runs
// while core blocks the OS in the SCM preshutdown window, ahead of any worker being killed.
//
// Setting a flag is all it does, and that is the design. The decision belongs where the false
// failure is otherwise produced, and there are exactly two such places -- Process, for a result the
// worker did report before it died, and the Executer's messaging error path, for a run the shutdown
// killed before it could report anything. Both run on the messaging goroutine, so neither needs to
// be reached from here and nothing needs to guard docState against a concurrent writer.
func PrepareForShutdown(logger log.T) {
	SetMachineShuttingDown()
	logger.Info("machine shutdown signaled; a patch document interrupted from here on will resume " +
		"after the reboot instead of reporting a failure")
}

// RewindPatchDocumentForReboot rewinds a patch document whose run a machine shutdown cut short, and
// reports whether it did. Called from the Executer's messaging error path: when the shutdown kills
// the document worker before it can report anything, that path is what would otherwise synthesize a
// terminal failure for a document that has not actually failed.
//
// Nothing is persisted here. The Executer's end-of-run save writes whatever this leaves behind, and
// if the machine goes down before that save runs, the state already on disk is the resumable one
// written at the start of the run -- a run only ever persists at its start and its end, so a Failed
// cannot reach disk while a shutdown is in progress.
func RewindPatchDocumentForReboot(logger log.T, docState *contracts.DocumentState) bool {
	if !contracts.IsPatchDocument(docState.DocumentInformation.DocumentName) {
		return false
	}
	if !shutdownChecker(logger) {
		return false
	}
	rewindToInProgress(docState)
	logger.TelemetryWarnf("patch document %v (name=%v) was interrupted by a machine shutdown before it "+
		"reported a result; keeping it InProgress to resume instead of reporting a false-negative failure",
		docState.DocumentInformation.DocumentID,
		docState.DocumentInformation.DocumentName)
	return true
}

// rewindToInProgress puts a document back into the state a resume will re-run.
//
// Resetting the Failed plugin results is what makes the resume actually re-run: runpluginutil skips
// a plugin that is already Failed and replays its recorded result verbatim, so leaving them would
// re-report the failure on the very next pass. Failed is also the only status to rewind, and
// pluginutil.GetStatus is why: a preemptively stopped plugin maps to Failed when the cancel flag
// reports shutdown, and TimedOut only when neither shutdown nor cancel was signalled. So Failed
// already is the bucket a shutdown-killed step lands in, while TimedOut means the step outlived its
// own timeout and Cancelled means someone stopped it on purpose -- re-running either would override
// a decision the reboot did not make, and the resume skips both anyway. A step the reboot killed
// before it could report anything stays InProgress, which the resume re-runs regardless.
//
// Only the latest Failed step is rewound, not every one. Steps run in order and each result is
// reported upstream as its step finishes, so an earlier Failed is a failure the service already has
// -- re-running it would redo work whose result is already public. Most patch documents cannot have
// more than one Failed step anyway, since only the step matching the target's platform runs and the
// rest are Skipped by precondition, but AWS-RunPatchBaselineWithHooks runs several steps on the same
// platform (scan, verify, the three hooks, install, reboot), and there an earlier genuine failure can
// coexist with the step the reboot cut short. Scoping to the last one is correct either way.
//
// InProgress rather than SuccessAndReboot at the document level: the processor acts on
// SuccessAndReboot by requesting a reboot, and this whole path is a heuristic that can
// false-positive.
func rewindToInProgress(docState *contracts.DocumentState) {
	docState.DocumentInformation.DocumentStatus = contracts.ResultStatusInProgress
	for i := len(docState.InstancePluginsInformation) - 1; i >= 0; i-- {
		if docState.InstancePluginsInformation[i].Result.Status == contracts.ResultStatusFailed {
			docState.InstancePluginsInformation[i].Result.Status = contracts.ResultStatusInProgress
			docState.InstancePluginsInformation[i].Result.Code = 0
			docState.InstancePluginsInformation[i].Result.Error = ""
			return
		}
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
		// A patch document that reports Failed while the machine is going down did not fail -- the
		// reboot cut it short. Reporting is irreversible and the run is not actually over, so
		// withhold the result and leave the document resumable instead. Bounded by
		// CommandRetryLimit and the 48h document expiry.
		if p.isRebootInterruptedPatchFailure(&docResult) {
			rewindToInProgress(p.docState)
			if t == MessageTypeComplete {
				// Same teardown a reported result gets: messaging returns with no error, so the
				// Executer neither synthesizes a failure nor reports one, and its end-of-run save
				// writes the resumable state this left behind.
				p.stopChan <- stopTypeTerminate
			}
			return nil
		}
		p.formatDocResult(&docResult)
		p.output <- docResult
		if t == MessageTypeComplete {
			//get document result, force termniate messaging worker
			p.stopChan <- stopTypeTerminate
		}
	default:
		return errors.New("unsupported message type")
	}
	return nil
}

// isRebootInterruptedPatchFailure reports whether a Failed patch result was caused by a servicing
// reboot rather than a genuine failure.
func (p *ExecuterBackend) isRebootInterruptedPatchFailure(docResult *contracts.DocumentResult) bool {
	if docResult.Status != contracts.ResultStatusFailed {
		return false
	}
	if !contracts.IsPatchDocument(p.docState.DocumentInformation.DocumentName) {
		return false
	}
	if !shutdownChecker(p.log) {
		return false
	}
	p.log.TelemetryWarnf("patch document %v (name=%v) reported Failed while the machine was shutting down; "+
		"keeping it InProgress to resume instead of reporting a false-negative failure",
		p.docState.DocumentInformation.DocumentID,
		p.docState.DocumentInformation.DocumentName)
	return true
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
