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

// Package processor defines the document processing unit interface
package processor

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime/debug"
	"sync"
	"time"

	"github.com/aws/amazon-ssm-agent/agent/appconfig"
	"github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/aws/amazon-ssm-agent/agent/contracts"
	"github.com/aws/amazon-ssm-agent/agent/fileutil"
	"github.com/aws/amazon-ssm-agent/agent/framework/docmanager"
	"github.com/aws/amazon-ssm-agent/agent/framework/processor/executer"
	"github.com/aws/amazon-ssm-agent/agent/framework/processor/executer/outofproc"
	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/amazon-ssm-agent/agent/longrunning/manager"
	"github.com/aws/amazon-ssm-agent/agent/rebooter"
	"github.com/aws/amazon-ssm-agent/agent/task"
	"github.com/aws/amazon-ssm-agent/agent/times"
)

type ExecuterCreator func(ctx context.T) executer.Executer

// ErrorCode represents processor related error codes
type ErrorCode string

const (

	// hardstopTimeout is the time before the processor will be shutdown during a hardstop
	hardStopTimeout = time.Second * 4

	maxDocumentTimeOutHour = time.Hour * 48

	// CommandBufferFull denotes that the cancel command buffer is full
	CommandBufferFull ErrorCode = "CommandBufferFull"

	// ClosedProcessor denotes that the processor is closed
	ClosedProcessor ErrorCode = "ClosedProcessor"

	// UnsupportedDocType represents unsupported doc type
	UnsupportedDocType ErrorCode = "UnsupportedDocType"

	// DuplicateCommand represents duplicate command in the buffer
	DuplicateCommand ErrorCode = "DuplicateCommand"

	// InvalidDocumentId represents invalid document id
	InvalidDocumentId ErrorCode = "InvalidDocumentId"

	// ConversionFailed represents conversion from pool to processor error code failed
	ConversionFailed ErrorCode = "ConversionFailed"

	// SubmissionPanic represents panic during submission to the pool
	SubmissionPanic ErrorCode = "SubmissionPanic"
)

type Processor interface {
	//Start activate the Processor and pick up the leftover document in the last run, it returns a channel to caller to gather DocumentResult
	Start() (chan contracts.DocumentResult, error)
	//InitialProcessing processes any initial documents loaded from file directory. This should be run after Start().
	InitialProcessing(skipDocumentIfExpired bool) error
	//Stop the processor, save the current state to resume later
	Stop()
	//Submit to the pool a document in form of docState object, results will be streamed back from the central channel returned by Start()
	Submit(docState contracts.DocumentState) ErrorCode
	//Cancel cancels processing of the given document
	Cancel(docState contracts.DocumentState) ErrorCode
	//TODO do we need to implement CancelAll?
	//CancelAll()
}

// EngineProcessor defines methods to process the incoming document by pushing to the executor using JobPools
type EngineProcessor struct {
	context           context.T
	executerCreator   ExecuterCreator
	sendCommandPool   task.Pool
	cancelCommandPool task.Pool
	//TODO this should be abstract as the Processor's domain
	resChan       chan contracts.DocumentResult
	documentMgr   docmanager.DocumentMgr
	stopFlagMutex sync.RWMutex

	isProcessorStopped          bool
	startWorker                 *workerProcessorSpec
	cancelWorker                *workerProcessorSpec
	poolToProcessorErrorCodeMap map[task.PoolErrorCode]ErrorCode
}

// WorkerProcessorSpec contains properties and methods to specify worker related specifications needed for the processor
type workerProcessorSpec struct {
	workerLimit     int
	assignedDocType contracts.DocumentType
	bufferLimit     int
}

// GetAssignedDocType returns the assigned doc type
func (wps *workerProcessorSpec) GetAssignedDocType() contracts.DocumentType {
	return wps.assignedDocType
}

// GetWorkerLimit returns the worker limit
func (wps *workerProcessorSpec) GetWorkerLimit() int {
	return wps.workerLimit
}

// GetBufferLimit returns the worker buffer limit
func (wps *workerProcessorSpec) GetBufferLimit() int {
	return wps.bufferLimit
}

// NewWorkerProcessorSpec return new worker processor specification object reference
func NewWorkerProcessorSpec(ctx context.T, workerLimit int, assignedDocType contracts.DocumentType, bufferLimit int) *workerProcessorSpec {
	logger := ctx.Log()
	workerProcessorSpecObj := &workerProcessorSpec{
		workerLimit:     workerLimit,
		assignedDocType: assignedDocType,
		bufferLimit:     bufferLimit,
	}
	if workerLimit < 1 {
		logger.Warnf("wrong worker limit format, assigning default value as 5")
		workerProcessorSpecObj.workerLimit = 5
	}
	// 0 as buffer limit blocks the buffer channel
	// 0 is passed by association module and offline processor module
	if bufferLimit < 0 {
		logger.Infof("wrong buffer limit format, assigning default value as 1")
		workerProcessorSpecObj.bufferLimit = 1
	}
	if assignedDocType == "" {
		logger.Infof("empty worker type assigned, assigning random doc type")
		workerProcessorSpecObj.assignedDocType = "nodoctype" // dummy value
	}
	return workerProcessorSpecObj
}

// NewEngineProcessor returns the newly initiated EngineProcessor
// TODO worker pool should be triggered in the Start() function
// supported document types indicate the domain of the documents the Processor with run upon. There'll be race-conditions if there're multiple Processors in a certain domain.
func NewEngineProcessor(ctx context.T, startWorker *workerProcessorSpec, cancelWorker *workerProcessorSpec) *EngineProcessor {
	engineProcessorCtx := ctx.With("[EngineProcessor]")
	log := engineProcessorCtx.Log()
	// sendCommand and cancelCommand will be processed by separate worker pools,
	// so we can define the number of workers per each
	cancelWaitDuration := 10000 * time.Millisecond
	clock := times.DefaultClock
	resChan := make(chan contracts.DocumentResult)
	executerCreator := func(ctx context.T) executer.Executer {
		return outofproc.NewOutOfProcExecuter(ctx)
	}

	documentMgr := docmanager.NewDocumentFileMgr(engineProcessorCtx, appconfig.DefaultDataStorePath, appconfig.DefaultDocumentRootDirName, appconfig.DefaultLocationOfState)
	engineProcessor := &EngineProcessor{
		context:                     engineProcessorCtx,
		executerCreator:             executerCreator,
		resChan:                     resChan,
		documentMgr:                 documentMgr,
		sendCommandPool:             task.NewPool(log, startWorker.workerLimit, startWorker.bufferLimit, cancelWaitDuration, clock),
		cancelCommandPool:           task.NewPool(log, cancelWorker.workerLimit, cancelWorker.bufferLimit, cancelWaitDuration, clock),
		startWorker:                 startWorker,
		cancelWorker:                cancelWorker,
		poolToProcessorErrorCodeMap: make(map[task.PoolErrorCode]ErrorCode),
	}
	engineProcessor.loadProcessorPoolErrorCodes()
	return engineProcessor
}

func (p *EngineProcessor) Start() (resChan chan contracts.DocumentResult, err error) {
	context := p.context
	if context == nil {
		return nil, fmt.Errorf("EngineProcessor is not initialized")
	}
	log := context.Log()
	log.Info("Starting")

	resChan = p.resChan
	return
}

func (p *EngineProcessor) InitialProcessing(skipDocumentIfExpired bool) (err error) {
	context := p.context
	if context == nil {
		return fmt.Errorf("EngineProcessor is not initialized")
	}
	log := context.Log()

	log.Info("Initial processing")

	// preloading pending files is added here to handle the below case:
	// In-progress documents starts submission by pushing it to the pending state.
	// This may lead to load same documents again when calling function processPendingDocuments
	pendingFiles := p.getDocStateFiles(log, appconfig.DefaultLocationOfPending)
	//prioritize the ongoing document first
	p.processInProgressDocuments(skipDocumentIfExpired)
	//deal with the pending jobs that have not picked up by worker yet
	p.processPendingDocuments(pendingFiles)
	return
}

// checkDocSubmissionAllowed checks whether the processor submission is allowed or not
func (p *EngineProcessor) checkDocSubmissionAllowed(docState *contracts.DocumentState, taskPool task.Pool, bufferLimit int) (error ErrorCode) {
	logger := p.context.Log()
	tokenSize := taskPool.BufferTokensIssued()
	logger.Debugf("buffer limit start value for doc type %v with command id %v: tokenSize - %v bufferLimit - %v", docState.DocumentType, docState.DocumentInformation.DocumentID, tokenSize, bufferLimit)
	if bufferLimit == 0 { // No synchronization needed as this value is loaded during processor initialization
		return "" // No check needed when buffer limit is zero. sticking with old behavior
	}
	if p.hasProcessorStopped() { // additional check to drop it at the beginning itself
		return ClosedProcessor
	}
	jobId := p.getJobId(docState)
	errorCode := taskPool.AcquireBufferToken(jobId)
	if errorCode != "" {
		if processorErrorCode, ok := p.poolToProcessorErrorCodeMap[errorCode]; ok {
			return processorErrorCode
		} else {
			return ConversionFailed
		}
	}
	tokenSize = taskPool.BufferTokensIssued()
	// Success condition
	logger.Debugf("buffer limit end value for doc type %v with command id %v: tokenSize - %v bufferLimit - %v", docState.DocumentType, docState.DocumentInformation.DocumentID, tokenSize, bufferLimit)
	return "" // Success
}

// loadProcessorPoolErrorCodes loads processor pool error code mappings
func (p *EngineProcessor) loadProcessorPoolErrorCodes() {
	p.poolToProcessorErrorCodeMap[task.InvalidJobId] = InvalidDocumentId
	p.poolToProcessorErrorCodeMap[task.DuplicateCommand] = DuplicateCommand
	p.poolToProcessorErrorCodeMap[task.JobQueueFull] = CommandBufferFull
}

// cleanUpDocSubmissionOnError is used to clean-up initially acquired tokens
// call this function only after acquiring token successfully
func (p *EngineProcessor) cleanUpDocSubmissionOnError(doc *contracts.DocumentState) {
	if doc.DocumentType == p.startWorker.assignedDocType && p.startWorker.bufferLimit > 0 { // do not call release token when buffer limit is zero
		p.decrementCommandBuffer(doc, p.sendCommandPool)
	} else if doc.DocumentType == p.cancelWorker.assignedDocType && p.cancelWorker.bufferLimit > 0 {
		p.decrementCommandBuffer(doc, p.cancelCommandPool)
	}
}

// decrementCommandBuffer used to delete start worker document from buffer
func (p *EngineProcessor) decrementCommandBuffer(doc *contracts.DocumentState, sendCommandPool task.Pool) {
	logger := p.context.Log()
	// safety check
	if doc == nil {
		logger.Errorf("document is nil")
		return
	}
	jobId := p.getJobId(doc)
	errorCode := sendCommandPool.ReleaseBufferToken(jobId)
	tokenSize := sendCommandPool.BufferTokensIssued()
	logger.Debugf("current buffer size for doc type %v with command id %v: tokenSize: %v", doc.DocumentType, jobId, tokenSize)
	// should not happen at any time
	if errorCode != "" {
		logger.Warnf("clean up failed because of the following error code %v", errorCode)
		return
	}
	logger.Infof("cleaned up command %v with doc type %v", jobId, doc.DocumentType)
}

// Submit submits to the pool a document in form of docState object, results will be streamed back from the channel returned by Start()
func (p *EngineProcessor) Submit(docState contracts.DocumentState) (errorCode ErrorCode) {
	return p.submit(&docState, false)
}

// submit will send job to the sendCommandPool
func (p *EngineProcessor) submit(docState *contracts.DocumentState, isInProgressDocument bool) (errorCode ErrorCode) {
	log := p.context.Log()
	jobID := p.getJobId(docState)
	// checks whether the document submission allowed in send command pool
	// duplicate command check also happens here
	// when buffer limit is zero, we return success("") always which means the pool submit will be blocking if it is full already
	errorCode = p.checkProcessorSubmissionAllowed(docState)
	if errorCode != "" {
		return errorCode
	}
	log.Infof("document %v submission started", jobID)
	defer log.Infof("document %v submission ended", jobID)
	defer func() {
		if r := recover(); r != nil {
			errorCode = SubmissionPanic
			p.cleanUpDocSubmissionOnError(docState) // call this function only after acquiring token successfully
			log.Errorf("document %v submission panicked", jobID)
			log.Errorf("stacktrace:\n%s", debug.Stack())
		}
	}()
	if !isInProgressDocument {
		p.documentMgr.PersistDocumentState(docState.DocumentInformation.DocumentID, appconfig.DefaultLocationOfPending, *docState)
	}
	//TODO this is a hack, in future jobID should be managed by Processing engine itself, instead of inferring from job's internal field
	err := p.sendCommandPool.Submit(log, jobID, func(cancelFlag task.CancelFlag) {
		processCommand(
			p.context,
			p.executerCreator,
			cancelFlag,
			p.resChan,
			docState,
			p.documentMgr)
	})
	if err != nil {
		// currently, we have only Duplicate command error returned by the job pool
		// * When buffer is zero, we don't have issues as we do not acquire/release token in pool
		// * When buffer is > 0, we do acquire/release the token. In this case, the checkProcessorSubmissionAllowed would have been called at the beginning.
		//   Listing all possible combinations of states in Job pool for a document to discuss Duplicate command error:
		//   1) When job is in the job queue buffer and not yet processed - This case is not possible as we do not receive commands already in "job queue buffer".
		//   2) When job is released from job queue buffer and started processing - This case is also not possible as we do not receive commands already in "job store".
		p.cleanUpDocSubmissionOnError(docState)
		log.Error("Document Submission failed: ", err)
		//move the fail-to-submit document to corrupt folder
		p.documentMgr.MoveDocumentState(docState.DocumentInformation.DocumentID, appconfig.DefaultLocationOfPending, appconfig.DefaultLocationOfCorrupt)
		return "" // considered submission successful even though it failed
	}
	return "" // considered submission successful
}

// checkProcessorSubmissionAllowed checks whether the processor submission is allowed or not
func (p *EngineProcessor) checkProcessorSubmissionAllowed(doc *contracts.DocumentState) (error ErrorCode) {
	if doc.DocumentType == p.startWorker.assignedDocType {
		return p.checkDocSubmissionAllowed(doc, p.sendCommandPool, p.startWorker.bufferLimit)
	} else if doc.DocumentType == p.cancelWorker.assignedDocType {
		return p.checkDocSubmissionAllowed(doc, p.cancelCommandPool, p.cancelWorker.bufferLimit)
	}
	return UnsupportedDocType
}

// getJobId returns job id
func (p *EngineProcessor) getJobId(docState *contracts.DocumentState) string {
	var jobID string
	if docState.IsAssociation() {
		jobID = docState.DocumentInformation.AssociationID
	} else {
		jobID = docState.DocumentInformation.MessageID
	}
	return jobID
}

// Cancel pushes the command to CancelThread which is responsible for submitting to cancelCommandPool
func (p *EngineProcessor) Cancel(docState contracts.DocumentState) (errorCode ErrorCode) {
	return p.cancel(&docState, false)
}

func (p *EngineProcessor) cancel(docState *contracts.DocumentState, isInProgressDocument bool) (errorCode ErrorCode) {
	log := p.context.Log()
	jobID := p.getJobId(docState)

	log.Infof("document %v cancellation started", jobID)
	defer log.Infof("document %v cancellation ended", jobID)

	// checks whether the document submission allowed in cancel command pool
	// duplicate command checks also happens here
	// when buffer limit is zero, we return success("") which means the channel will be blocking if buffer is zero
	errorCode = p.checkProcessorSubmissionAllowed(docState)
	if errorCode != "" {
		return errorCode
	}

	defer func() {
		if r := recover(); r != nil {
			errorCode = SubmissionPanic
			p.cleanUpDocSubmissionOnError(docState) // call this function only after acquiring token successfully
			log.Errorf("document %v submission panicked", jobID)
			log.Errorf("stacktrace:\n%s", debug.Stack())
		}
	}()
	if !isInProgressDocument {
		p.documentMgr.PersistDocumentState(docState.DocumentInformation.DocumentID, appconfig.DefaultLocationOfPending, *docState)
	}
	err := p.cancelCommandPool.Submit(log, jobID, func(cancelFlag task.CancelFlag) {
		processCancelCommand(p.context, p.sendCommandPool, docState, p.documentMgr)
	})
	if err != nil {
		// currently, we have only Duplicate command error returned by the job pool
		// * When buffer is zero, we don't have issues as we do not acquire/release token in pool
		// * When buffer is > 0, we do acquire/release the token. In this case, the checkProcessorSubmissionAllowed would have been called at the beginning.
		//   Listing all possible combinations of states in Job pool for a document to discuss Duplicate command error:
		//   1) When job is in the job queue buffer and not yet processed - This case is not possible as we do not receive commands already in "job queue buffer".
		//   2) When job is released from job queue buffer and started processing - This case is also not possible as we do not receive commands already in "job store".
		p.cleanUpDocSubmissionOnError(docState)
		log.Error("CancelCommand failed", err)
	}
	return ""
}

// hasProcessorStopped checks whether the processor has stopped
func (p *EngineProcessor) hasProcessorStopped() bool {
	p.stopFlagMutex.RLock() // change to RWMutex
	defer p.stopFlagMutex.RUnlock()
	return p.isProcessorStopped
}

// hasProcessorStoppedAlready returns whether the processor stop is called once or not
func (p *EngineProcessor) hasProcessorStopCalledAlready() bool {
	p.stopFlagMutex.Lock()
	defer p.stopFlagMutex.Unlock()
	if p.isProcessorStopped {
		return true
	}
	p.isProcessorStopped = true
	return false
}

// Stop set the cancel flags of all the running jobs, which are to be captured by the command worker and shutdown gracefully
func (p *EngineProcessor) Stop() {
	if p.hasProcessorStopCalledAlready() {
		p.context.Log().Info("Processor stop called already")
		return
	}

	waitTimeout := time.Duration(p.context.AppConfig().Mds.StopTimeoutMillis) * time.Millisecond

	var wg sync.WaitGroup

	// shutdown the send command pool in a separate go routine
	wg.Add(1)
	go func() {
		defer func() {
			if r := recover(); r != nil {
				p.context.Log().Errorf("Shutdown send command pool panic: %v", r)
				p.context.Log().Errorf("Stacktrace:\n%s", debug.Stack())
			}
		}()
		defer wg.Done()
		p.sendCommandPool.ShutdownAndWait(waitTimeout)
	}()

	// shutdown the cancel command pool in a separate go routine
	wg.Add(1)
	go func() {
		defer func() {
			if r := recover(); r != nil {
				p.context.Log().Errorf("Shutdown cancel command pool panic: %v", r)
				p.context.Log().Errorf("Stacktrace:\n%s", debug.Stack())
			}
		}()
		defer wg.Done()
		p.cancelCommandPool.ShutdownAndWait(waitTimeout)
	}()

	// wait for everything to shut down
	wg.Wait()
	// close the receiver channel only after we're sure all the ongoing jobs are stopped and no sender is on this channel
	close(p.resChan)
	p.context.Log().Info("processor closed")
}

// TODO remove the direct file dependency once we encapsulate docmanager package
func (p *EngineProcessor) processPendingDocuments(files []os.FileInfo) {
	log := p.context.Log()
	//iterate through all pending messages
	for _, f := range files {
		log.Infof("Found pending document - %v", f.Name())
		//inspect document state
		docState := p.documentMgr.GetDocumentState(f.Name(), appconfig.DefaultLocationOfPending)

		if p.isSupportedDocumentType(docState.DocumentType) {
			p.pushPersistedDocToJobPool(docState, appconfig.DefaultLocationOfPending, false)
		}
	}
}

func (p *EngineProcessor) getDocStateFiles(log log.T, docStateDir string) []os.FileInfo {
	var files []os.FileInfo
	instanceID, err := p.context.Identity().ShortInstanceID()
	if err != nil {
		log.Errorf("Failed to get short instanceID for process %v Documents: %v", docStateDir, err)
		return files
	}

	// process older documents from state folder
	docsLocation := docmanager.DocumentStateDir(instanceID, docStateDir)

	if isDirectoryEmpty, _ := fileutil.IsDirEmpty(docsLocation); isDirectoryEmpty {
		log.Debugf("No %v documents to process from %v", docStateDir, docsLocation)
		return files
	}

	//get all messages
	if files, err = fileutil.ReadDir(docsLocation); err != nil {
		log.Errorf("skipping reading %v documents from %v. unexpected error encountered - %v", docStateDir, docsLocation, err)
	}
	return files
}

// ProcessInProgressDocuments processes InProgress documents that have already dequeued and entered job pool
func (p *EngineProcessor) processInProgressDocuments(skipDocumentIfExpired bool) {
	log := p.context.Log()
	config := p.context.AppConfig()
	files := p.getDocStateFiles(log, appconfig.DefaultLocationOfCurrent)

	//iterate through all InProgress docs
	for _, f := range files {
		log.Infof("Found in-progress document - %v", f.Name())

		//inspect document state
		docState := p.documentMgr.GetDocumentState(f.Name(), appconfig.DefaultLocationOfCurrent)

		if p.isSupportedDocumentType(docState.DocumentType) {
			retryLimit := config.Mds.CommandRetryLimit
			if docState.DocumentInformation.RunCount >= retryLimit {
				p.documentMgr.MoveDocumentState(f.Name(), appconfig.DefaultLocationOfCurrent, appconfig.DefaultLocationOfCorrupt)
				continue
			}

			// increment the command run count
			docState.DocumentInformation.RunCount++

			p.documentMgr.PersistDocumentState(docState.DocumentInformation.DocumentID, appconfig.DefaultLocationOfCurrent, docState)

			log.Infof("Processing in-progress document %v", docState.DocumentInformation.DocumentID)
			if skipDocumentIfExpired && docState.DocumentInformation.CreatedDate != "" {
				createDate := times.ParseIso8601UTC(docState.DocumentInformation.CreatedDate)

				// Do not resume in-progress document is create date is 48 hours ago.
				if createDate.Add(maxDocumentTimeOutHour).Before(time.Now().UTC()) {
					log.Infof("Document %v expired %v, skipping", docState.DocumentInformation.DocumentID, docState.DocumentInformation.CreatedDate)
					p.documentMgr.MoveDocumentState(f.Name(), appconfig.DefaultLocationOfCurrent, appconfig.DefaultLocationOfCorrupt)
					continue
				}
			}

			//Submit the work to Job Pool so that we don't block for processing of new messages
			p.pushPersistedDocToJobPool(docState, appconfig.DefaultLocationOfCurrent, true)
		}
	}
}

// pushPersistedDocToJobPool pushes in-progress and pending documents to job pool during restart
func (p *EngineProcessor) pushPersistedDocToJobPool(docState contracts.DocumentState, docStateDir string, isInProgress bool) {
	logger := p.context.Log()
	// safety check
	defer func() {
		if r := recover(); r != nil {
			p.cleanUpDocSubmissionOnError(&docState)
			logger.Errorf("submitting to processor panicked %v %v", docState.DocumentInformation.DocumentID, r)
			logger.Errorf("stacktrace:\n%s", debug.Stack())
		}
	}()
	logger.Infof("Processing document %v from state dir %v", docState.DocumentInformation.DocumentID, docStateDir)
	for {
		var processorErrorCode ErrorCode
		if docState.DocumentType == p.startWorker.assignedDocType {
			processorErrorCode = p.submit(&docState, isInProgress)
		} else if docState.DocumentType == p.cancelWorker.assignedDocType {
			processorErrorCode = p.cancel(&docState, isInProgress)
		}
		if processorErrorCode == CommandBufferFull { // sleep only for command buffer full
			logger.Debugf("pausing in-progress submission for a second %v because of error code %v", docState.DocumentInformation.DocumentID, processorErrorCode)
			time.Sleep(time.Second)
			continue
		}
		if processorErrorCode != "" { // all errors except CommandBufferFull
			logger.Warnf("skipping in-progress document %v because of error code %v", docState.DocumentInformation.DocumentID, processorErrorCode)
		}
		break // break iteration for success and errors other than CommandBufferFull
	}
}

// isSupportedDocumentType returns whether the processor supports the document
func (p *EngineProcessor) isSupportedDocumentType(documentType contracts.DocumentType) bool {
	if documentType != "" {
		if p.startWorker.assignedDocType == documentType || p.cancelWorker.assignedDocType == documentType {
			return true
		}
	}
	return false
}

func processCommand(context context.T, executerCreator ExecuterCreator, cancelFlag task.CancelFlag, resChan chan contracts.DocumentResult, docState *contracts.DocumentState, docMgr docmanager.DocumentMgr) {
	log := context.Log()
	//persist the current running document
	docMgr.MoveDocumentState(
		docState.DocumentInformation.DocumentID,
		appconfig.DefaultLocationOfPending,
		appconfig.DefaultLocationOfCurrent)
	log.Debug("Running executer...")
	documentID := docState.DocumentInformation.DocumentID
	messageID := docState.DocumentInformation.MessageID
	e := executerCreator(context)
	docStore := executer.NewDocumentFileStore(documentID, appconfig.DefaultLocationOfCurrent, docState, docMgr, true)
	statusChan := e.Run(
		cancelFlag,
		&docStore,
	)
	// Listen for reboot
	var final *contracts.DocumentResult
	for res := range statusChan {
		func() {
			defer func() {
				if err := recover(); err != nil {
					log.Errorf("Failed to process status for document %s with error %v", documentID, err)
					log.Errorf("Stacktrace:\n%s", debug.Stack())
				}
			}()

			if res.LastPlugin == "" {
				log.Infof("sending document: %v complete response", documentID)
			} else {
				log.Infof("sending reply for plugin update: %v", res.LastPlugin)
			}

			final = &res
			handleCloudwatchPlugin(context, res.PluginResults, documentID)
			// when receiving the reply from workers, we do not have UpstreamServiceName populated
			// whenever we receive a response, we populate with the appropriate Upstream service
			// this is added to avoid changes in the workers
			res.UpstreamServiceName = docState.UpstreamServiceName
			// used to add topic to the payload in agent reply message in MGS interactor
			res.RelatedDocumentType = docState.DocumentType
			//hand off the message to Service
			resChan <- res

			log.Info("Done")
		}()
	}
	//TODO add shutdown as API call, move cancelFlag out of task pool; cancelFlag to contracts, nobody else above runplugins needs to create cancelFlag.
	// Shutdown/reboot detection
	if final == nil || final.LastPlugin != "" {
		log.Infof("document %v still in progress, shutting down...", messageID)
		return
	} else if final.Status == contracts.ResultStatusSuccessAndReboot {
		log.Infof("document %v requested reboot, need to resume", messageID)
		rebooter.RequestPendingReboot(context.Log())
		return
	}

	//persist : commands execution in completed folder (terminal state folder)
	log.Infof("execution of %v is over. Removing interimState from current folder", messageID)

	docMgr.RemoveDocumentState(
		documentID,
		appconfig.DefaultLocationOfCurrent)

}

// TODO CancelCommand is currently treated as a special type of Command by the Processor, but in general Cancel operation should be seen as a probe to existing commands
func processCancelCommand(context context.T, sendCommandPool task.Pool, docState *contracts.DocumentState, docMgr docmanager.DocumentMgr) {

	log := context.Log()
	//persist the final status of cancel-message in current folder
	docMgr.MoveDocumentState(
		docState.DocumentInformation.DocumentID,
		appconfig.DefaultLocationOfPending, appconfig.DefaultLocationOfCurrent)
	log.Debugf("Canceling job with id %v...", docState.CancelInformation.CancelMessageID)

	if found := sendCommandPool.Cancel(docState.CancelInformation.CancelMessageID); !found {
		log.Debugf("Job with id %v not found (possibly completed)", docState.CancelInformation.CancelMessageID)
		docState.CancelInformation.DebugInfo = fmt.Sprintf("Command %v couldn't be cancelled", docState.CancelInformation.CancelCommandID)
		docState.DocumentInformation.DocumentStatus = contracts.ResultStatusFailed
	} else {
		docState.CancelInformation.DebugInfo = fmt.Sprintf("Command %v cancelled", docState.CancelInformation.CancelCommandID)
		docState.DocumentInformation.DocumentStatus = contracts.ResultStatusSuccess
	}

	//persist : commands execution in completed folder (terminal state folder)
	log.Debugf("Execution of %v is over. Removing interimState file from Current folder", docState.DocumentInformation.MessageID)

	docMgr.RemoveDocumentState(
		docState.DocumentInformation.DocumentID,
		appconfig.DefaultLocationOfCurrent)

}

// TODO remove this once CloudWatch plugin is reworked
// temporary solution on plugins with shared responsibility with agent
func handleCloudwatchPlugin(context context.T, pluginResults map[string]*contracts.PluginResult, documentID string) {
	log := context.Log()
	instanceID, _ := context.Identity().InstanceID()
	//TODO once association service switches to use RC and CW goes away, remove this block
	for ID, pluginRes := range pluginResults {
		if pluginRes.PluginName == appconfig.PluginNameCloudWatch {
			log.Infof("Found %v to invoke lrpm invoker", pluginRes.PluginName)
			orchestrationRootDir := filepath.Join(
				appconfig.DefaultDataStorePath,
				instanceID,
				appconfig.DefaultDocumentRootDirName,
				context.AppConfig().Agent.OrchestrationRootDir)
			orchestrationDir := fileutil.BuildPath(orchestrationRootDir, documentID)
			manager.Invoke(context, ID, pluginRes, orchestrationDir)
		}
	}

}
