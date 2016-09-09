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

// Package processor manage polling of associations, dispatching association to processor
package processor

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"time"

	"github.com/aws/amazon-ssm-agent/agent/appconfig"
	"github.com/aws/amazon-ssm-agent/agent/association/model"
	"github.com/aws/amazon-ssm-agent/agent/association/service"
	"github.com/aws/amazon-ssm-agent/agent/association/taskpool"
	"github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/aws/amazon-ssm-agent/agent/contracts"
	"github.com/aws/amazon-ssm-agent/agent/fileutil"
	"github.com/aws/amazon-ssm-agent/agent/framework/plugin"
	"github.com/aws/amazon-ssm-agent/agent/jsonutil"
	"github.com/aws/amazon-ssm-agent/agent/log"
	messageContracts "github.com/aws/amazon-ssm-agent/agent/message/contracts"
	"github.com/aws/amazon-ssm-agent/agent/platform"
	"github.com/aws/amazon-ssm-agent/agent/reply"
	"github.com/aws/amazon-ssm-agent/agent/sdkutil"
	ssmsvc "github.com/aws/amazon-ssm-agent/agent/ssm"
	"github.com/aws/amazon-ssm-agent/agent/task"
	"github.com/aws/aws-sdk-go/service/ssm"
	"github.com/carlescere/scheduler"
)

const (
	name                          = "Association"
	documentWorkersLimit          = 1
	cancelWaitDurationMillisecond = 10000
	associationRetryLimit         = 3
	stopPolicyErrorThreshold      = 10
)

// Processor contains the logic for processing association
type Processor struct {
	pollJob             *scheduler.Job
	ssmSvc              ssmsvc.Service
	context             context.T
	taskPool            taskpool.T
	agentInfo           *contracts.AgentInfo
	processorStopPolicy *sdkutil.StopPolicy
}

// NewAssociationProcessor returns a new Processor with the given context.
func NewAssociationProcessor(context context.T) *Processor {
	assocContext := context.With("[" + name + "]")

	log := assocContext.Log()
	config := assocContext.AppConfig()

	instanceID, err := platform.InstanceID()
	if instanceID == "" {
		log.Errorf("no instanceID provided, %v", err)
		return nil
	}

	taskPool := taskpool.NewTaskPool(log, documentWorkersLimit, cancelWaitDurationMillisecond)

	agentInfo := contracts.AgentInfo{
		Lang:      config.Os.Lang,
		Name:      config.Agent.Name,
		Version:   config.Agent.Version,
		Os:        config.Os.Name,
		OsVersion: config.Os.Version,
	}

	ssmService := ssmsvc.NewService()
	return &Processor{
		context:             assocContext,
		ssmSvc:              ssmService,
		taskPool:            taskPool,
		agentInfo:           &agentInfo,
		processorStopPolicy: sdkutil.NewStopPolicy(name, stopPolicyErrorThreshold),
	}
}

// SetPollJob represents setter for PollJob
func (p *Processor) SetPollJob(job *scheduler.Job) {
	p.pollJob = job
}

// ProcessPendingDocuments processes pending documents that have been persisted in pending folder
func (p *Processor) ProcessPendingDocuments() {
	log := p.context.Log()

	instanceID, err := platform.InstanceID()
	if err != nil {
		log.Errorf("no instanceID provided, %v", err)
		return
	}

	//process older documents from PENDING folder
	pendingDocsLocation := bookkeepingSvc.DocumentStateDir(instanceID, appconfig.DefaultLocationOfPending)

	if isDirectoryEmpty, _ := fileutil.IsDirEmpty(pendingDocsLocation); isDirectoryEmpty {
		log.Debugf("No documents to process from %v", pendingDocsLocation)
		return
	}

	files := []os.FileInfo{}
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

		interimDocState := messageContracts.DocumentState{}
		//parse the message
		if err := jsonutil.UnmarshalFile(filePath, &interimDocState); err != nil {
			log.Errorf("skipping processsing of pending messages. encountered error %v while reading pending message from file - %v", err, f)
			break
		}

		p.submitDocForExecution(log, &interimDocState)
	}
}

// ProcessInProgressDocuments processes InProgress documents that have been persisted in current folder
func (p *Processor) ProcessInProgressDocuments(instanceID string) {
	log := p.context.Log()
	var err error

	pendingDocsLocation := bookkeepingSvc.DocumentStateDir(instanceID, appconfig.DefaultLocationOfCurrent)

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
		log.Debugf("processing previously unexecuted message - %v", f.Name())

		//construct the absolute path - safely assuming that interim state for older messages are already present in Current folder
		file := filepath.Join(pendingDocsLocation, f.Name())
		var docState messageContracts.DocumentState

		//parse the message
		if err := jsonutil.UnmarshalFile(file, &docState); err != nil {
			log.Errorf("skipping processsing of previously unexecuted messages. encountered error %v while reading unprocessed message from file - %v", err, f)
			//TODO: Move doc to corrupt/failed
			break
		}

		if !docState.IsAssociation() {
			break
		}

		if docState.DocumentInformation.RunCount >= associationRetryLimit {
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

		bookkeepingSvc.PersistData(log, docState.DocumentInformation.CommandID, instanceID, appconfig.DefaultLocationOfCurrent, docState)

		//Submit the work to Job Pool so that we don't block for processing of new association
		if err = p.taskPool.Submit(log, docState.DocumentInformation.CommandID, func(cancelFlag task.CancelFlag) {
			p.processAssociationDocument(p.context, &docState, cancelFlag)
		}); err != nil {
			log.Errorf("Failed to resume previously unexecuted association, %v", err)
		}
	}
}

// ProcessAssociation poll and process all the associations
func (p *Processor) ProcessAssociation() {
	log := p.context.Log()
	var assocRawData *model.AssociationRawData

	instanceID, err := platform.InstanceID()
	if err != nil {
		log.Error("unable to retrieve instance id", err)
		return
	}

	if assocRawData, err = assocSvc.ListAssociations(log, p.ssmSvc, instanceID); err != nil {
		log.Error("unable to retrieve associations", err)
		return
	}

	if err = assocSvc.LoadAssociationDetail(log, p.ssmSvc, assocRawData); err != nil {
		message := fmt.Sprintf("unable to load association details, %v", err)
		log.Error(message)
		p.updateAssocStatus(assocRawData.Association, ssm.AssociationStatusNameFailed, message)
		return
	}

	if err = parseAssociation(p, assocRawData); err != nil {
		log.Error(err)
		p.updateAssocStatus(assocRawData.Association, ssm.AssociationStatusNameFailed, err.Error())
		return
	}
}

// parseAssociation submits the association to the task pool for execution
func parseAssociation(p *Processor, rawData *model.AssociationRawData) error {
	// create separate logger that includes messageID with every log message
	context := p.context.With("[associationName=" + *rawData.Association.Name + "]")
	log := context.Log()
	var interimDocState messageContracts.DocumentState

	log.Debug("Processing association")

	document, err := assocParser.ParseDocumentWithParams(log, rawData)
	if err != nil {
		return fmt.Errorf("failed to parse association, %v", err)
	}

	parsedMessageContent, _ := jsonutil.Marshal(document)
	log.Debug("ParsedAssociation is ", jsonutil.Indent(parsedMessageContent))

	//Data format persisted in Current Folder is defined by the struct - CommandState
	interimDocState = assocParser.InitializeDocumentState(context, document, rawData)

	log.Debug("Persisting interim state in current execution folder")
	bookkeepingSvc.PersistData(log,
		interimDocState.DocumentInformation.CommandID,
		interimDocState.DocumentInformation.Destination,
		appconfig.DefaultLocationOfPending,
		interimDocState)

	if err = p.submitDocForExecution(log, &interimDocState); err != nil {
		return err
	}

	return nil
}

// submitDocForExecution moves doc to current folder and submit it for execution
func (p *Processor) submitDocForExecution(log log.T, interimDocState *messageContracts.DocumentState) (err error) {
	//TODO: check if p.sendDocLevelResponse is needed here
	log.Debugf("Persist document and update association status to pending")
	p.updateAssocStatusWithDocInfo(&interimDocState.DocumentInformation, ssm.AssociationStatusNamePending, "processing document")

	bookkeepingSvc.MoveCommandState(log,
		interimDocState.DocumentInformation.CommandID,
		interimDocState.DocumentInformation.Destination,
		appconfig.DefaultLocationOfPending,
		appconfig.DefaultLocationOfCurrent)

	if err = p.taskPool.Submit(log, interimDocState.DocumentInformation.CommandID, func(cancelFlag task.CancelFlag) {
		p.processAssociationDocument(p.context, interimDocState, cancelFlag)
	}); err != nil {
		return fmt.Errorf("failed to process association, %v", err)
	}

	return nil
}

// processAssociationDocument parses and processes the document
func (p *Processor) processAssociationDocument(context context.T,
	interimDocState *messageContracts.DocumentState,
	cancelFlag task.CancelFlag) {
	log := context.Log()
	//TODO: check isManagedInstance
	log.Debug("Running plugins...")

	//TODO: add sendReply engine.SendResponse
	outputs := pluginExecution.RunPlugins(context,
		interimDocState.DocumentInformation.CommandID,
		&interimDocState.PluginsInformation,
		plugin.RegisteredWorkerPlugins(context),
		nil,
		cancelFlag)

	pluginOutputContent, _ := jsonutil.Marshal(outputs)
	log.Debugf("Plugin outputs %v", jsonutil.Indent(pluginOutputContent))

	p.parseAndPersistReplyContents(log, interimDocState, outputs)
	// Skip sending response when the document requires a reboot
	if interimDocState.IsRebootRequired() {
		log.Debug("skipping sending response of %v since the document requires a reboot", interimDocState.DocumentInformation.CommandID)
		return
	}

	log.Debug("Association execution completion ", outputs)
	if interimDocState.DocumentInformation.DocumentStatus == contracts.ResultStatusFailed {
		p.updateAssocStatusWithDocInfo(&interimDocState.DocumentInformation,
			ssm.AssociationStatusNameFailed,
			"Execution failed")

	} else if interimDocState.DocumentInformation.DocumentStatus == contracts.ResultStatusSuccess {
		p.updateAssocStatusWithDocInfo(&interimDocState.DocumentInformation,
			ssm.AssociationStatusNameSuccess,
			"Execution succeeded")
	}

	//persist : commands execution in completed folder (terminal state folder)
	log.Debugf("execution of %v is over. Moving interimState file from Current to Completed folder", interimDocState.DocumentInformation.CommandID)
	bookkeepingSvc.MoveCommandState(log,
		interimDocState.DocumentInformation.CommandID,
		interimDocState.DocumentInformation.Destination,
		appconfig.DefaultLocationOfCurrent,
		appconfig.DefaultLocationOfCompleted)
}

// parseAndPersistReplyContents reloads interimDocState, updates it with replyPayload and persist it on disk.
func (p *Processor) parseAndPersistReplyContents(log log.T,
	interimDocState *messageContracts.DocumentState,
	pluginOutputs map[string]*contracts.PluginResult) {

	//update interim cmd state file
	documentInfo := bookkeepingSvc.GetDocumentInfo(log,
		interimDocState.DocumentInformation.CommandID,
		interimDocState.DocumentInformation.Destination,
		appconfig.DefaultLocationOfCurrent)

	runtimeStatuses := reply.PrepareRuntimeStatuses(log, pluginOutputs)
	replyPayload := reply.PrepareReplyPayload("", runtimeStatuses, time.Now(), *p.agentInfo)

	// set document level information which wasn't set previously
	documentInfo.AdditionalInfo = replyPayload.AdditionalInfo
	documentInfo.DocumentStatus = replyPayload.DocumentStatus
	documentInfo.DocumentTraceOutput = replyPayload.DocumentTraceOutput
	documentInfo.RuntimeStatus = replyPayload.RuntimeStatus

	//persist final documentInfo.
	bookkeepingSvc.PersistDocumentInfo(log,
		documentInfo,
		interimDocState.DocumentInformation.CommandID,
		interimDocState.DocumentInformation.Destination,
		appconfig.DefaultLocationOfCurrent)
}

// updateAssociationStatus provides wrapper for calling update association service
func (p *Processor) updateAssocStatus(
	assoc *ssm.Association,
	status string,
	message string) {
	log := p.context.Log()

	service.UpdateAssociationStatus(log,
		p.ssmSvc,
		*assoc.InstanceId,
		*assoc.Name,
		status,
		message,
		p.agentInfo,
		p.processorStopPolicy)
}

// updateAssocStatusWithDocInfo provides wrapper for calling update association service
func (p *Processor) updateAssocStatusWithDocInfo(
	assoc *messageContracts.DocumentInfo,
	status string,
	message string) {
	log := p.context.Log()

	service.UpdateAssociationStatus(log,
		p.ssmSvc,
		assoc.Destination,
		assoc.DocumentName,
		status,
		message,
		p.agentInfo,
		p.processorStopPolicy)
}
