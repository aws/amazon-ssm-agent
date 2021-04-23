package outofproc

import (
	"fmt"
	"runtime/debug"
	"time"

	"github.com/aws/amazon-ssm-agent/agent/appconfig"
	"github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/aws/amazon-ssm-agent/agent/contracts"
	"github.com/aws/amazon-ssm-agent/agent/framework/processor/executer"
	"github.com/aws/amazon-ssm-agent/agent/framework/processor/executer/basicexecuter"
	"github.com/aws/amazon-ssm-agent/agent/framework/processor/executer/outofproc/messaging"
	"github.com/aws/amazon-ssm-agent/agent/framework/processor/executer/outofproc/proc"
	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/amazon-ssm-agent/agent/task"
	"github.com/aws/amazon-ssm-agent/common/filewatcherbasedipc"
	"github.com/aws/amazon-ssm-agent/common/identity"
	"github.com/aws/amazon-ssm-agent/core/executor"
)

type Backend messaging.MessagingBackend

//see differences between zombie and orphan: https://www.gmarik.info/blog/2012/orphan-vs-zombie-vs-daemon-processes/
const (
	//TODO prolong this value once we go to production
	defaultZombieProcessTimeout = 3 * time.Second
	//command maximum timeout
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

var processFinder = func(log log.T, procinfo contracts.OSProcInfo, executor executor.IExecutor) bool {
	//If ProcInfo is not initailized
	//pid 0 is reserved for kernel on both linux and windows, so the assumption is safe here
	if procinfo.Pid == 0 {
		return false
	}

	isRunning, err := executor.IsPidRunning(procinfo.Pid)

	if err != nil {
		log.Errorf("Failed to query for running process: %v", err)
		return false
	}

	return isRunning
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

//Run() prepare the ipc channel, create a data processing backend and start messaging with docment worker
func (e *OutOfProcExecuter) Run(
	cancelFlag task.CancelFlag,
	docStore executer.DocumentStore) chan contracts.DocumentResult {
	docState := docStore.Load()
	e.docState = &docState
	e.cancelFlag = cancelFlag
	documentID := docState.DocumentInformation.DocumentID

	//update context with the document id
	e.ctx = e.ctx.With("[" + documentID + "]")
	log := e.ctx.Log()

	//stopTimer signals messaging routine to stop, it's buffered because it needs to exit if messaging is already stopped and not receiving anymore
	stopTimer := make(chan bool, 1)
	//start prepare messaging
	//if anything fails during the prep stage, use in-proc Runner
	ipc, err := e.initialize(stopTimer)
	//save doc store immediately in case agent restarts.
	docStore.Save(*e.docState)

	if err != nil {
		log.Errorf("failed to prepare outofproc executer, falling back to InProc Executer")
		return e.BasicExecuter.Run(cancelFlag, docStore)
	} else {
		//create reply channel
		resChan := make(chan contracts.DocumentResult, len(e.docState.InstancePluginsInformation)+1)
		//launch the messaging go-routine
		go func(store executer.DocumentStore) {
			defer func() {
				if msg := recover(); msg != nil {
					log.Errorf("Executer go-routine panic: %v", msg)
					log.Errorf("Stacktrace:\n%s", debug.Stack())
				}
				//save the overall result and signal called that Executer is done
				store.Save(*e.docState)
				log.Info("Executer closed")
				close(resChan)
			}()
			e.messaging(log, ipc, resChan, cancelFlag, stopTimer)
		}(docStore)

		return resChan
	}
}

//Executer spins up an ipc transmission worker, it creates a Data processing backend and hands off the backend to the ipc worker
//ipc worker and data backend act as 2 threads exchange raw json messages, and messaging protocol happened in data backend, data backend is self-contained and exit when command finishes accordingly
//Executer however does hold a timer to the worker to forcefully termniate both of them
func (e *OutOfProcExecuter) messaging(log log.T, ipc filewatcherbasedipc.IPCChannel, resChan chan contracts.DocumentResult, cancelFlag task.CancelFlag, stopTimer chan bool) {

	//handoff reply functionalities to data backend.
	backend := messaging.NewExecuterBackend(log, resChan, e.docState, cancelFlag)
	//handoff the data backend to messaging worker
	if err := messaging.Messaging(log, ipc, backend, stopTimer); err != nil {
		//the messaging worker encountered error, either ipc run into error or data backend throws error
		log.Errorf("messaging worker encountered error: %v", err)
		log.Debugf("document state during messaging worker error: %v", e.docState.DocumentInformation.DocumentStatus)
		if e.docState.DocumentInformation.DocumentStatus == contracts.ResultStatusInProgress ||
			e.docState.DocumentInformation.DocumentStatus == "" ||
			e.docState.DocumentInformation.DocumentStatus == contracts.ResultStatusNotStarted ||
			e.docState.DocumentInformation.DocumentStatus == contracts.ResultStatusSuccessAndReboot {
			e.docState.DocumentInformation.DocumentStatus = contracts.ResultStatusFailed
			log.Info("document failed half way, sending fail message...")
			resChan <- e.generateUnexpectedFailResult(fmt.Sprintf("document process failed unexpectedly: %s , check [ssm-document-worker]/[ssm-session-worker] log for crash reason", err))
		}
		//destroy the channel
		ipc.Destroy()
	}
}

func (e *OutOfProcExecuter) generateUnexpectedFailResult(errMsg string) contracts.DocumentResult {
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
	docResult.PluginResults[e.docState.InstancePluginsInformation[0].Id] = &res
	return docResult
}

//prepare the channel for messaging as well as launching the document worker process, if the channel already exists, re-open it.
//launch timeout timer based off the discovered process status
func (e *OutOfProcExecuter) initialize(stopTimer chan bool) (ipc filewatcherbasedipc.IPCChannel, err error) {
	log := e.ctx.Log()
	var found bool
	documentID := e.docState.DocumentInformation.DocumentID
	instanceID := e.docState.DocumentInformation.InstanceID

	// this fix is added to delete channels which were not deleted in previous execution
	if e.docState.DocumentInformation.DocumentStatus == contracts.ResultStatusSuccessAndReboot {
		log.Info("deleting channel for reboot document")
		if channelErr := filewatcherbasedipc.RemoveFileWatcherChannel(e.ctx.Identity(), documentID); channelErr != nil {
			log.Warnf("failed to remove channel directory: %v", channelErr)
		}
	}
	ipc, err, found = channelCreator(log, e.ctx.Identity(), filewatcherbasedipc.ModeMaster, documentID)

	if err != nil {
		log.Errorf("failed to create ipc channel: %v", err)
		return
	}
	if found {
		log.Info("discovered old channel object, trying to find detached process...")
		var stopTime time.Duration
		procInfo := e.docState.DocumentInformation.ProcInfo
		if processFinder(log, procInfo, e.executor) {
			log.Infof("found orphan process: %v, start time: %v", procInfo.Pid, procInfo.StartTime)
			stopTime = defaultOrphanProcessTimeout
		} else {
			log.Infof("process: %v not found, treat as exited", procInfo.Pid)
			stopTime = defaultZombieProcessTimeout
		}
		go timeout(stopTimer, stopTime, e.cancelFlag)
	} else {
		log.Debug("channel not found, starting a new process...")
		var workerName string
		if e.docState.DocumentType == contracts.StartSession {
			workerName = appconfig.DefaultSessionWorker
		} else {
			workerName = appconfig.DefaultDocumentWorker
		}
		var process proc.OSProcess
		if process, err = processCreator(workerName, proc.FormArgv(documentID, instanceID)); err != nil {
			log.Errorf("start process: %v error: %v", workerName, err)
			//make sure close the channel
			ipc.Destroy()
			return
		} else {
			log.Debugf("successfully launched new process: %v", process.Pid())
		}
		e.docState.DocumentInformation.ProcInfo = contracts.OSProcInfo{
			Pid:       process.Pid(),
			StartTime: process.StartTime(),
		}
		//TODO add command timeout as well, in case process get stuck
		go e.WaitForProcess(stopTimer, process)

	}

	return
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
	//waitReturned = true
	timeout(stopTimer, defaultZombieProcessTimeout, e.cancelFlag)
}

func timeout(stopTimer chan bool, duration time.Duration, cancelFlag task.CancelFlag) {
	stopChan := make(chan bool, 1)
	//TODO refactor cancelFlag.Wait() to return channel instead of blocking call
	go func() {
		cancelFlag.Wait()
		stopChan <- true
	}()
	select {
	case <-time.After(duration):
		stopTimer <- true
	case <-stopChan:
	}

}
