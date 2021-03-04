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
	"io/ioutil"
	"os"
	"runtime/debug"
	"sync"
	"time"

	"path/filepath"

	"github.com/aws/amazon-ssm-agent/agent/appconfig"
	"github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/aws/amazon-ssm-agent/agent/contracts"
	"github.com/aws/amazon-ssm-agent/agent/fileutil"
	"github.com/aws/amazon-ssm-agent/agent/framework/docmanager"
	"github.com/aws/amazon-ssm-agent/agent/framework/processor/executer"
	"github.com/aws/amazon-ssm-agent/agent/framework/processor/executer/outofproc"
	"github.com/aws/amazon-ssm-agent/agent/longrunning/manager"
	"github.com/aws/amazon-ssm-agent/agent/rebooter"
	"github.com/aws/amazon-ssm-agent/agent/task"
	"github.com/aws/amazon-ssm-agent/agent/times"
)

type ExecuterCreator func(ctx context.T) executer.Executer

const (

	// hardstopTimeout is the time before the processor will be shutdown during a hardstop
	hardStopTimeout = time.Second * 4

	maxDocumentTimeOutHour = time.Hour * 48
)

type Processor interface {
	//Start activate the Processor and pick up the left over document in the last run, it returns a channel to caller to gather DocumentResult
	Start() (chan contracts.DocumentResult, error)
	//Process any initial documents loaded from file directory. This should be run after Start().
	InitialProcessing(skipDocumentIfExpired bool) error
	//Stop the processor, save the current state to resume later
	Stop(stopType contracts.StopType)
	//submit to the pool a document in form of docState object, results will be streamed back from the central channel returned by Start()
	Submit(docState contracts.DocumentState)
	//cancel process the cancel document, with no return value since the command is already tracked in a different thread
	Cancel(docState contracts.DocumentState)
	//TODO do we need to implement CancelAll?
	//CancelAll()
}

type EngineProcessor struct {
	context           context.T
	executerCreator   ExecuterCreator
	sendCommandPool   task.Pool
	cancelCommandPool task.Pool
	//TODO this should be abstract as the Processor's domain
	supportedDocTypes []contracts.DocumentType
	resChan           chan contracts.DocumentResult
	documentMgr       docmanager.DocumentMgr
}

//TODO worker pool should be triggered in the Start() function
//supported document types indicate the domain of the documentes the Processor with run upon. There'll be race-conditions if there're multiple Processors in a certain domain.
func NewEngineProcessor(ctx context.T, commandWorkerLimit int, cancelWorkerLimit int, supportedDocs []contracts.DocumentType) *EngineProcessor {
	log := ctx.Log()
	// sendCommand and cancelCommand will be processed by separate worker pools
	// so we can define the number of workers per each
	cancelWaitDuration := 10000 * time.Millisecond
	clock := times.DefaultClock
	sendCommandTaskPool := task.NewPool(log, commandWorkerLimit, cancelWaitDuration, clock)
	cancelCommandTaskPool := task.NewPool(log, cancelWorkerLimit, cancelWaitDuration, clock)
	resChan := make(chan contracts.DocumentResult)
	executerCreator := func(ctx context.T) executer.Executer {
		return outofproc.NewOutOfProcExecuter(ctx)
	}
	documentMgr := docmanager.NewDocumentFileMgr(ctx, appconfig.DefaultDataStorePath, appconfig.DefaultDocumentRootDirName, appconfig.DefaultLocationOfState)
	return &EngineProcessor{
		context:           ctx.With("[EngineProcessor]"),
		executerCreator:   executerCreator,
		sendCommandPool:   sendCommandTaskPool,
		cancelCommandPool: cancelCommandTaskPool,
		supportedDocTypes: supportedDocs,
		resChan:           resChan,
		documentMgr:       documentMgr,
	}
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
	//prioritize the ongoing document first
	p.processInProgressDocuments(skipDocumentIfExpired)
	//deal with the pending jobs that haven't picked up by worker yet
	p.processPendingDocuments()
	return
}

//Submit() is the public interface for sending run document request to processor
func (p *EngineProcessor) Submit(docState contracts.DocumentState) {
	log := p.context.Log()
	//queue up the pending document
	p.documentMgr.PersistDocumentState(docState.DocumentInformation.DocumentID, appconfig.DefaultLocationOfPending, docState)
	err := p.submit(&docState)
	if err != nil {
		log.Error("Document Submission failed", err)
		//move the fail-to-submit document to corrupt folder
		p.documentMgr.MoveDocumentState(docState.DocumentInformation.DocumentID, appconfig.DefaultLocationOfPending, appconfig.DefaultLocationOfCorrupt)
		return
	}
	log.Debug("EngineProcessor submit succeeded")
	return
}

func (p *EngineProcessor) submit(docState *contracts.DocumentState) error {
	log := p.context.Log()
	//TODO this is a hack, in future jobID should be managed by Processing engine itself, instead of inferring from job's internal field
	var jobID string
	if docState.IsAssociation() {
		jobID = docState.DocumentInformation.AssociationID
	} else {
		jobID = docState.DocumentInformation.MessageID
	}
	return p.sendCommandPool.Submit(log, jobID, func(cancelFlag task.CancelFlag) {
		processCommand(
			p.context,
			p.executerCreator,
			cancelFlag,
			p.resChan,
			docState,
			p.documentMgr)
	})

}

func (p *EngineProcessor) Cancel(docState contracts.DocumentState) {
	log := p.context.Log()
	//TODO this is a hack, in future jobID should be managed by Processing engine itself, instead of inferring from job's internal field
	var jobID string
	if docState.IsAssociation() {
		jobID = docState.DocumentInformation.AssociationID
	} else {
		jobID = docState.DocumentInformation.MessageID
	}
	//queue up the pending document
	p.documentMgr.PersistDocumentState(docState.DocumentInformation.DocumentID, appconfig.DefaultLocationOfPending, docState)
	err := p.cancelCommandPool.Submit(log, jobID, func(cancelFlag task.CancelFlag) {
		processCancelCommand(p.context, p.sendCommandPool, &docState, p.documentMgr)
	})
	if err != nil {
		log.Error("CancelCommand failed", err)
		return
	}
}

//Stop set the cancel flags of all the running jobs, which are to be captured by the command worker and shutdown gracefully
func (p *EngineProcessor) Stop(stopType contracts.StopType) {
	var waitTimeout time.Duration

	if stopType == contracts.StopTypeSoftStop {
		waitTimeout = time.Duration(p.context.AppConfig().Mds.StopTimeoutMillis) * time.Millisecond
	} else {
		waitTimeout = hardStopTimeout
	}

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

	// wait for everything to shutdown
	wg.Wait()
	// close the receiver channel only after we're sure all the ongoing jobs are stopped and no sender is on this channel
	close(p.resChan)
}

//TODO remove the direct file dependency once we encapsulate docmanager package
func (p *EngineProcessor) processPendingDocuments() {
	log := p.context.Log()
	files := []os.FileInfo{}
	instanceID, err := p.context.Identity().ShortInstanceID()
	if err != nil {
		log.Errorf("Failed to get short instanceID for processPendingDocuments: %v", err)
	}

	//process older documents from PENDING folder
	pendingDocsLocation := docmanager.DocumentStateDir(instanceID, appconfig.DefaultLocationOfPending)

	if isDirectoryEmpty, _ := fileutil.IsDirEmpty(pendingDocsLocation); isDirectoryEmpty {
		log.Debugf("No pending documents to process from %v", pendingDocsLocation)
		return
	}

	//get all pending messages
	if files, err = fileutil.ReadDir(pendingDocsLocation); err != nil {
		log.Errorf("skipping reading pending documents from %v. unexpected error encountered - %v", pendingDocsLocation, err)
		return
	}

	//iterate through all pending messages
	for _, f := range files {
		log.Infof("Found pending document - %v", f.Name())
		//inspect document state
		docState := p.documentMgr.GetDocumentState(f.Name(), appconfig.DefaultLocationOfPending)

		if p.isSupportedDocumentType(docState.DocumentType) {
			log.Infof("Processing pending document %v", docState.DocumentInformation.DocumentID)
			p.Submit(docState)
		}

	}
}

// ProcessInProgressDocuments processes InProgress documents that have already dequeued and entered job pool
func (p *EngineProcessor) processInProgressDocuments(skipDocumentIfExpired bool) {
	log := p.context.Log()
	config := p.context.AppConfig()
	instanceID, err := p.context.Identity().ShortInstanceID()
	if err != nil {
		log.Errorf("Failed to get short instanceID for processInProgressDocuments: %v", err)
	}

	pendingDocsLocation := docmanager.DocumentStateDir(instanceID, appconfig.DefaultLocationOfCurrent)

	if isDirectoryEmpty, _ := fileutil.IsDirEmpty(pendingDocsLocation); isDirectoryEmpty {
		log.Debugf("No in-progress document to process from %v", pendingDocsLocation)
		return

	}

	files := []os.FileInfo{}
	if files, err = ioutil.ReadDir(pendingDocsLocation); err != nil {
		log.Errorf("skipping reading inprogress document from %v. unexpected error encountered - %v", pendingDocsLocation, err)
		return
	}

	//iterate through all InProgress docs
	for _, f := range files {
		log.Infof("Found in-progress document - %v", f.Name())

		//inspect document state
		docState := p.documentMgr.GetDocumentState(f.Name(), appconfig.DefaultLocationOfCurrent)

		retryLimit := config.Mds.CommandRetryLimit
		if docState.DocumentInformation.RunCount >= retryLimit {
			p.documentMgr.MoveDocumentState(f.Name(), appconfig.DefaultLocationOfCurrent, appconfig.DefaultLocationOfCorrupt)
			continue
		}

		// increment the command run count
		docState.DocumentInformation.RunCount++

		p.documentMgr.PersistDocumentState(docState.DocumentInformation.DocumentID, appconfig.DefaultLocationOfCurrent, docState)

		if p.isSupportedDocumentType(docState.DocumentType) {
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
			if err := p.submit(&docState); err != nil {
				log.Errorf("failed to submit in progress document %v : %v", docState.DocumentInformation.DocumentID, err)
				p.documentMgr.MoveDocumentState(f.Name(), appconfig.DefaultLocationOfCurrent, appconfig.DefaultLocationOfCorrupt)
			}
		}
	}
}

func (p *EngineProcessor) isSupportedDocumentType(documentType contracts.DocumentType) bool {
	for _, d := range p.supportedDocTypes {
		if documentType == d {
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
	docStore := executer.NewDocumentFileStore(documentID, appconfig.DefaultLocationOfCurrent, docState, docMgr)
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

//TODO CancelCommand is currently treated as a special type of Command by the Processor, but in general Cancel operation should be seen as a probe to existing commands
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

//TODO remove this once CloudWatch plugin is reworked
//temporary solution on plugins with shared responsibility with agent
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
