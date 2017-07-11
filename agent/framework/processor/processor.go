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
	"sync"
	"time"

	"github.com/aws/amazon-ssm-agent/agent/appconfig"
	"github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/aws/amazon-ssm-agent/agent/contracts"
	"github.com/aws/amazon-ssm-agent/agent/docmanager"
	"github.com/aws/amazon-ssm-agent/agent/docmanager/model"
	"github.com/aws/amazon-ssm-agent/agent/fileutil"
	"github.com/aws/amazon-ssm-agent/agent/message/processor/executer"
	"github.com/aws/amazon-ssm-agent/agent/platform"
	"github.com/aws/amazon-ssm-agent/agent/task"
	"github.com/aws/amazon-ssm-agent/agent/times"
)

// TopicPrefix is the prefix of the Topic field in an MDS message.
type TopicPrefix string
type ExecuterCreator func(ctx context.T) executer.Executer

const (

	// hardstopTimeout is the time before the processor will be shutdown during a hardstop
	// TODO:  load this value from config
	hardStopTimeout = time.Second * 4

	// the default stoppolicy error threshold. After 10 consecutive errors the plugin will stop for 15 minutes.
	stopPolicyErrorThreshold = 10
)

type Processor interface {
	//Start activate the Processor and pick up the left over document in the last run, it returns a channel to caller to gather DocumentResult
	Start() (error, chan contracts.DocumentResult)
	//Stop the processor, save the current state to resume later
	Stop(stopType contracts.StopType)
	//submit to the pool a document in form of docState object, results will be streamed back from the central channel returned by Start()
	Submit(docState model.DocumentState)
	//cancel process the cancel document, with no return value since the command is already tracked in a different thread
	Cancel(docState model.DocumentState)
	//TODO do we need to implement CancelAll?
	//CancelAll()
}

type EngineProcessor struct {
	context           context.T
	executerCreator   ExecuterCreator
	sendCommandPool   task.Pool
	cancelCommandPool task.Pool
	//TODO this should be abstract as the Processor's domain
	supportedDocTypes []model.DocumentType
	resChan           chan contracts.DocumentResult
}

//TODO worker pool should be triggered in the Start() function
func NewEngineProcessor(ctx context.T, commandWorkerLimit int, cancelWorkerLimit int, executerCreator ExecuterCreator, supportedDocs []model.DocumentType) *EngineProcessor {
	log := ctx.Log()
	// sendCommand and cancelCommand will be processed by separate worker pools
	// so we can define the number of workers per each
	cancelWaitDuration := 10000 * time.Millisecond
	clock := times.DefaultClock
	sendCommandTaskPool := task.NewPool(log, commandWorkerLimit, cancelWaitDuration, clock)
	cancelCommandTaskPool := task.NewPool(log, cancelWorkerLimit, cancelWaitDuration, clock)
	resChan := make(chan contracts.DocumentResult)
	return &EngineProcessor{
		context:           ctx.With("[EngineProcessor]"),
		executerCreator:   executerCreator,
		sendCommandPool:   sendCommandTaskPool,
		cancelCommandPool: cancelCommandTaskPool,
		supportedDocTypes: supportedDocs,
		resChan:           resChan,
	}
}

func (p *EngineProcessor) Start() (err error, resChan chan contracts.DocumentResult) {
	context := p.context
	if context == nil {
		return fmt.Errorf("EngineProcessor is not initialized"), nil
	}
	log := context.Log()
	//process the older jobs from Current & Pending folder
	instanceID, err := platform.InstanceID()
	if err != nil {
		log.Errorf("no instanceID provided, %v", err)
		return
	}
	resChan = p.resChan
	//prioritie the ongoing document first
	p.processInProgressDocuments(instanceID)
	//deal with the pending jobs that haven't picked up by worker yet
	p.processPendingDocuments(instanceID)
	return
}

func (p *EngineProcessor) Submit(docState model.DocumentState) {
	log := p.context.Log()
	//queue up the pending document
	docmanager.PersistData(log, docState.DocumentInformation.DocumentID, docState.DocumentInformation.InstanceID, appconfig.DefaultLocationOfPending, docState)
	err := p.sendCommandPool.Submit(log, docState.DocumentInformation.MessageID, func(cancelFlag task.CancelFlag) {
		processCommand(
			p.context,
			p.executerCreator,
			cancelFlag,
			p.resChan,
			&docState)
	})
	if err != nil {
		log.Error("Document Submission failed", err)
		return
	}
	return
}

func (p *EngineProcessor) Cancel(docState model.DocumentState) {
	log := p.context.Log()
	err := p.cancelCommandPool.Submit(log, docState.DocumentInformation.MessageID, func(cancelFlag task.CancelFlag) {
		processCancelCommand(p.context, p.sendCommandPool, &docState)
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
		defer wg.Done()
		p.sendCommandPool.ShutdownAndWait(waitTimeout)
	}()

	// shutdown the cancel command pool in a separate go routine
	wg.Add(1)
	go func() {
		defer wg.Done()
		p.cancelCommandPool.ShutdownAndWait(waitTimeout)
	}()

	// wait for everything to shutdown
	wg.Wait()
}

//TODO remove the direct file dependency once we encapsulate docmanager package
func (p *EngineProcessor) processPendingDocuments(instanceID string) {
	log := p.context.Log()
	files := []os.FileInfo{}
	var err error

	//process older documents from PENDING folder
	pendingDocsLocation := docmanager.DocumentStateDir(instanceID, appconfig.DefaultLocationOfPending)

	if isDirectoryEmpty, _ := fileutil.IsDirEmpty(pendingDocsLocation); isDirectoryEmpty {
		log.Debugf("No documents to process from %v", pendingDocsLocation)
		return
	}

	//get all pending messages
	if files, err = fileutil.ReadDir(pendingDocsLocation); err != nil {
		log.Errorf("skipping reading pending documents from %v. unexpected error encountered - %v", pendingDocsLocation, err)
		return
	}

	//iterate through all pending messages
	for _, f := range files {
		log.Debugf("Processing an older document - %v", f.Name())
		//inspect document state
		docState := docmanager.GetDocumentInterimState(log, f.Name(), instanceID, appconfig.DefaultLocationOfPending)

		if p.isSupportedDocumentType(docState.DocumentType) {
			log.Debugf("processor processing pending document %v", docState.DocumentInformation.DocumentID)
			p.Submit(docState)
		}

	}
}

// ProcessInProgressDocuments processes InProgress documents that have been persisted in current folder
func (p *EngineProcessor) processInProgressDocuments(instanceID string) {
	log := p.context.Log()
	config := p.context.AppConfig()
	var err error

	pendingDocsLocation := docmanager.DocumentStateDir(instanceID, appconfig.DefaultLocationOfCurrent)

	if isDirectoryEmpty, _ := fileutil.IsDirEmpty(pendingDocsLocation); isDirectoryEmpty {
		log.Debugf("no older document to process from %v", pendingDocsLocation)
		return

	}

	files := []os.FileInfo{}
	if files, err = ioutil.ReadDir(pendingDocsLocation); err != nil {
		log.Errorf("skipping reading inprogress document from %v. unexpected error encountered - %v", pendingDocsLocation, err)
		return
	}

	//iterate through all InProgress docs
	for _, f := range files {
		log.Debugf("processing previously unexecuted document - %v", f.Name())

		//inspect document state
		docState := docmanager.GetDocumentInterimState(log, f.Name(), instanceID, appconfig.DefaultLocationOfCurrent)

		retryLimit := config.Mds.CommandRetryLimit
		if docState.DocumentInformation.RunCount >= retryLimit {
			docmanager.MoveDocumentState(log, f.Name(), instanceID, appconfig.DefaultLocationOfCurrent, appconfig.DefaultLocationOfCorrupt)
			continue
		}

		// increment the command run count
		docState.DocumentInformation.RunCount++

		docmanager.PersistData(log, docState.DocumentInformation.DocumentID, instanceID, appconfig.DefaultLocationOfCurrent, docState)

		if p.isSupportedDocumentType(docState.DocumentType) {
			log.Debugf("processor processing in-progress document %v", docState.DocumentInformation.DocumentID)
			//Submit the work to Job Pool so that we don't block for processing of new messages
			p.Submit(docState)
		}
	}
}

func (p *EngineProcessor) isSupportedDocumentType(documentType model.DocumentType) bool {
	for _, d := range p.supportedDocTypes {
		if documentType == d {
			return true
		}
	}
	return false
}

func processCommand(context context.T, executerCreator ExecuterCreator, cancelFlag task.CancelFlag, resChan chan contracts.DocumentResult, docState *model.DocumentState) {
	log := context.Log()
	//persist the current running document
	docmanager.MoveDocumentState(log,
		docState.DocumentInformation.DocumentID,
		docState.DocumentInformation.InstanceID,
		appconfig.DefaultLocationOfPending,
		appconfig.DefaultLocationOfCurrent)
	log.Debug("Running executer...")
	documentID := docState.DocumentInformation.DocumentID
	instanceID := docState.DocumentInformation.InstanceID
	messageID := docState.DocumentInformation.MessageID
	e := executerCreator(context)
	docStore := executer.NewDocumentFileStore(context, instanceID, documentID, appconfig.DefaultLocationOfCurrent, docState)
	statusChan := e.Run(
		cancelFlag,
		&docStore,
	)
	// Listen for reboot
	isReboot := false
	for res := range statusChan {
		log.Infof("sending reply for plugin %v update", res.LastPlugin)
		//hand off the message to Service
		resChan <- res
		isReboot = res.Status == contracts.ResultStatusSuccessAndReboot
	}
	//TODO since there's a bug in UpdatePlugin that returns InProgress even if the document is completed, we cannot use InProgress to judge here, we need to fix the bug by the time out-of-proc is done
	// Shutdown/reboot detection
	if isReboot {
		log.Infof("document %v did not finish up execution, need to resume", messageID)
		return
	}

	//persist : commands execution in completed folder (terminal state folder)
	log.Debugf("execution of %v is over. Moving interimState file from Current to Completed folder", messageID)

	docmanager.MoveDocumentState(log,
		documentID,
		instanceID,
		appconfig.DefaultLocationOfCurrent,
		appconfig.DefaultLocationOfCompleted)

}

//TODO CancelCommand is currently treated as a special type of Command by the Processor, but in general Cancel operation should be seen as a probe to existing commands
func processCancelCommand(context context.T, sendCommandPool task.Pool, docState *model.DocumentState) {

	log := context.Log()

	log.Debugf("Canceling job with id %v...", docState.CancelInformation.CancelMessageID)

	if found := sendCommandPool.Cancel(docState.CancelInformation.CancelMessageID); !found {
		log.Debugf("Job with id %v not found (possibly completed)", docState.CancelInformation.CancelMessageID)
		docState.CancelInformation.DebugInfo = fmt.Sprintf("Command %v couldn't be cancelled", docState.CancelInformation.CancelCommandID)
		docState.DocumentInformation.DocumentStatus = contracts.ResultStatusFailed
	} else {
		docState.CancelInformation.DebugInfo = fmt.Sprintf("Command %v cancelled", docState.CancelInformation.CancelCommandID)
		docState.DocumentInformation.DocumentStatus = contracts.ResultStatusSuccess
	}

	//persist the final status of cancel-message in current folder
	docmanager.PersistData(log,
		docState.DocumentInformation.DocumentID,
		docState.DocumentInformation.InstanceID,
		appconfig.DefaultLocationOfCurrent, docState)

	//persist : commands execution in completed folder (terminal state folder)
	log.Debugf("Execution of %v is over. Moving interimState file from Current to Completed folder", docState.DocumentInformation.MessageID)

	docmanager.MoveDocumentState(log,
		docState.DocumentInformation.DocumentID,
		docState.DocumentInformation.InstanceID,
		appconfig.DefaultLocationOfCurrent,
		appconfig.DefaultLocationOfCompleted)

}
