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
	return name
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

	log.Info("Starting association polling")
	var job *scheduler.Job
	if job, err = asocitscheduler.CreateScheduler(
		log,
		p.assocProcessor.ProcessAssociation,
		pollAssociationFrequencyMinutes); err != nil {
		context.Log().Errorf("unable to schedule association processor. %v", err)
	}
	p.assocProcessor.InitializeAssociationProcessor()
	p.assocProcessor.SetPollJob(job)
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
		log.Debugf("Processing an older message with messageID - %v", f.Name())

		//construct the absolute path - safely assuming that interim state for older messages are already present in Pending folder
		filePath := filepath.Join(pendingDocsLocation, f.Name())

		docState := model.DocumentState{}
		//parse the message
		if err := jsonutil.UnmarshalFile(filePath, &docState); err != nil {
			log.Errorf("skipping processsing of pending messages. encountered error %v while reading pending message from file - %v", err, f)
			break
		}

		if docState.IsAssociation() {
			p.assocProcessor.ExecutePendingDocument(&docState)
		} else {
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
		log.Debugf("no older messages to process from %v", pendingDocsLocation)
		return

	}

	files := []os.FileInfo{}
	if files, err = ioutil.ReadDir(pendingDocsLocation); err != nil {
		log.Errorf("skipping reading inprogress messages from %v. unexpected error encountered - %v", pendingDocsLocation, err)
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
			log.Errorf("skipping processsing of previously unexecuted messages. encountered error %v while reading unprocessed message from file - %v", err, f)
			//TODO: Move doc to corrupt/failed
			break
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

		pluginOutputs := make(map[string]*contracts.PluginResult)

		// increment the command run count
		docState.DocumentInformation.RunCount++
		// Update reboot status
		for v := range docState.PluginsInformation {
			plugin := docState.PluginsInformation[v]
			if plugin.HasExecuted && plugin.Result.Status == contracts.ResultStatusSuccessAndReboot {
				log.Debugf("plugin %v has completed a reboot. Setting status to Success.", v)
				plugin.Result.Status = contracts.ResultStatusSuccess
				docState.PluginsInformation[v] = plugin
				pluginOutputs[v] = &plugin.Result
			}
		}

		statemanager.PersistData(log, docState.DocumentInformation.DocumentID, instanceID, appconfig.DefaultLocationOfCurrent, docState)

		if docState.IsAssociation() {
			//Submit the work to Job Pool so that we don't block for processing of new association
			if err = p.assocProcessor.SubmitTask(log, docState.DocumentInformation.DocumentID, func(cancelFlag task.CancelFlag) {
				p.assocProcessor.ExecuteInProgressDocument(&docState, cancelFlag)
			}); err != nil {
				log.Errorf("Association failed to resume previously unexecuted documents, %v", err)
			}
			return
		}

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
		p.assocProcessor.ShutdownAndWait(waitTimeout)
	}()

	// wait for everything to shutdown
	wg.Wait()
	return nil
}
