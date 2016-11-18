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

// Package processor implements MDS plugin processor
// processor_coreplugin contains the ICorePlugin implementation
package processor

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/aws/amazon-ssm-agent/agent/appconfig"
	asocitscheduler "github.com/aws/amazon-ssm-agent/agent/association/scheduler"
	"github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/aws/amazon-ssm-agent/agent/contracts"
	"github.com/aws/amazon-ssm-agent/agent/fileutil"
	"github.com/aws/amazon-ssm-agent/agent/jsonutil"
	"github.com/aws/amazon-ssm-agent/agent/platform"
	"github.com/aws/amazon-ssm-agent/agent/statemanager"
	"github.com/aws/amazon-ssm-agent/agent/statemanager/model"
	"github.com/aws/amazon-ssm-agent/agent/task"
	"github.com/carlescere/scheduler"
)

// Name returns the Plugin Name
func (p *Processor) Name() string {
	return p.name
}

func (p *Processor) isSupportedDocumentType(documentType model.DocumentType) bool {
	for _, d := range p.supportedDocTypes {
		if documentType == d {
			return true
		}
	}
	return false
}

// Execute starts the scheduling of the message processor plugin
func (p *Processor) Execute(context context.T) (err error) {

	log := p.context.Log()
	//process the older messages from Current & Pending folder
	instanceID, err := platform.InstanceID()
	if err != nil {
		log.Errorf("no instanceID provided, %v", err)
		return
	}

	// InProgress documents must be resumed before Pending documents
	// to avoid resuming same document twice in both Pending and InProgress
	p.processInProgressDocuments(instanceID)
	p.processPendingDocuments(instanceID)

	log.Info("Starting message processor polling")
	if p.messagePollJob, err = scheduler.Every(pollMessageFrequencyMinutes).Minutes().Run(p.loop); err != nil {
		context.Log().Errorf("unable to schedule message processor. %v", err)
	}

	if p.pollAssociations {
		associationFrequenceMinutes := context.AppConfig().Ssm.AssociationFrequencyMinutes
		log.Info("Starting association polling")
		log.Debugf("Association polling frequencey is %v", associationFrequenceMinutes)
		var job *scheduler.Job
		if job, err = asocitscheduler.CreateScheduler(
			log,
			p.assocProcessor.ProcessAssociation,
			associationFrequenceMinutes); err != nil {
			context.Log().Errorf("unable to schedule association processor. %v", err)
		}
		p.assocProcessor.InitializeAssociationProcessor()
		p.assocProcessor.SetPollJob(job)
	}
	return
}

// ProcessPendingDocuments processes pending documents that have been persisted in pending folder
func (p *Processor) processPendingDocuments(instanceID string) {
	log := p.context.Log()
	files := []os.FileInfo{}
	var err error

	//process older documents from PENDING folder
	pendingDocsLocation := statemanager.DocumentStateDir(instanceID, appconfig.DefaultLocationOfPending)

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

		//construct the absolute path - safely assuming that interim state for older messages are already present in Pending folder
		filePath := filepath.Join(pendingDocsLocation, f.Name())

		docState := model.DocumentState{}

		//parse the message
		if err := jsonutil.UnmarshalFile(filePath, &docState); err != nil {
			log.Errorf("skipping processsing of pending document. encountered error %v while reading pending document from file - %v", err, f)
			break
		}

		if !p.isSupportedDocumentType(docState.DocumentType) && (!docState.IsAssociation() || !p.pollAssociations) {
			continue // This is a document for a different processor to handle
		}

		if docState.IsAssociation() && p.pollAssociations {
			log.Debugf("processing pending association document: %v", docState.DocumentInformation.DocumentID)
			p.assocProcessor.ExecutePendingDocument(&docState)
		} else if p.isSupportedDocumentType(docState.DocumentType) {
			log.Debugf("processor %v processing pending document %v", p.name, docState.DocumentInformation.DocumentID)
			p.ExecutePendingDocument(&docState)
		}

	}
}

// ProcessInProgressDocuments processes InProgress documents that have been persisted in current folder
func (p *Processor) processInProgressDocuments(instanceID string) {
	log := p.context.Log()
	config := p.context.AppConfig()
	var err error

	pendingDocsLocation := statemanager.DocumentStateDir(instanceID, appconfig.DefaultLocationOfCurrent)

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

		//construct the absolute path - safely assuming that interim state for older messages are already present in Current folder
		file := filepath.Join(pendingDocsLocation, f.Name())
		var docState model.DocumentState

		//parse the message
		if err := jsonutil.UnmarshalFile(file, &docState); err != nil {
			log.Errorf("skipping processsing of previously unexecuted documents. encountered error %v while reading unprocessed document from file - %v", err, f)
			//TODO: Move doc to corrupt/failed
			break
		}

		if !p.isSupportedDocumentType(docState.DocumentType) && (!docState.IsAssociation() || !p.pollAssociations) {
			log.Debugf("Skipping document %v type %v isaccoc %v and our pollAssociations is %v", docState.DocumentInformation.DocumentID, docState.DocumentType, docState.IsAssociation(), p.pollAssociations)
			continue // This is a document for a different processor to handle
		}

		retryLimit := config.Mds.CommandRetryLimit
		if docState.IsAssociation() {
			retryLimit = config.Ssm.AssociationRetryLimit
		}

		if docState.DocumentInformation.RunCount >= retryLimit {
			//TODO:  Move doc to corrupt/failed
			// do not process as the command has failed too many times
			break
		}

		// increment the command run count
		docState.DocumentInformation.RunCount++
		// Update reboot status
		for index, plugin := range docState.InstancePluginsInformation {
			if plugin.HasExecuted && plugin.Result.Status == contracts.ResultStatusSuccessAndReboot {
				log.Debugf("plugin %v has completed a reboot. Setting status to InProgress to resume the work.", plugin.Name)
				plugin.Result.Status = contracts.ResultStatusInProgress
				docState.InstancePluginsInformation[index] = plugin
			}
		}

		statemanager.PersistData(log, docState.DocumentInformation.DocumentID, instanceID, appconfig.DefaultLocationOfCurrent, docState)

		if docState.IsAssociation() && p.pollAssociations {
			log.Debugf("processing in-progress association document: %v", docState.DocumentInformation.DocumentID)
			//Submit the work to Job Pool so that we don't block for processing of new association
			if err = p.assocProcessor.SubmitTask(log, docState.DocumentInformation.CommandID, func(cancelFlag task.CancelFlag) {
				p.assocProcessor.ExecuteInProgressDocument(&docState, cancelFlag)
			}); err != nil {
				log.Errorf("Association failed to resume previously unexecuted documents, %v", err)
			}
		} else if p.isSupportedDocumentType(docState.DocumentType) {
			log.Debugf("processor %v processing in-progress document %v", p.name, docState.DocumentInformation.DocumentID)
			//Submit the work to Job Pool so that we don't block for processing of new messages
			err := p.sendCommandPool.Submit(log, docState.DocumentInformation.MessageID, func(cancelFlag task.CancelFlag) {
				p.runCmdsUsingCmdState(p.context.With("[messageID="+docState.DocumentInformation.MessageID+"]"),
					p.service,
					p.pluginRunner,
					cancelFlag,
					p.buildReply,
					p.sendResponse,
					docState)
			})
			if err != nil {
				log.Error("SendCommand failed for previously unexecuted commands", err)
				break
			}
		}
	}
}

// RequestStop handles the termination of the message processor plugin job
func (p *Processor) RequestStop(stopType contracts.StopType) (err error) {
	var waitTimeout time.Duration

	if stopType == contracts.StopTypeSoftStop {
		waitTimeout = time.Duration(p.context.AppConfig().Mds.StopTimeoutMillis) * time.Millisecond
	} else {
		waitTimeout = hardStopTimeout
	}

	var wg sync.WaitGroup

	// ask the message processor to stop
	p.stop()

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

	// shutdown the association task pool in a separate go routine
	wg.Add(1)
	go func() {
		defer wg.Done()
		if p.assocProcessor != nil {
			p.assocProcessor.ShutdownAndWait(waitTimeout)
		}
	}()

	// wait for everything to shutdown
	wg.Wait()
	return nil
}
