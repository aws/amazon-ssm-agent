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
	"time"

	"github.com/aws/amazon-ssm-agent/agent/appconfig"
	"github.com/aws/amazon-ssm-agent/agent/association/model"
	"github.com/aws/amazon-ssm-agent/agent/association/service"
	"github.com/aws/amazon-ssm-agent/agent/association/taskpool"
	"github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/aws/amazon-ssm-agent/agent/contracts"
	"github.com/aws/amazon-ssm-agent/agent/framework/plugin"
	"github.com/aws/amazon-ssm-agent/agent/jsonutil"
	"github.com/aws/amazon-ssm-agent/agent/log"
	messageContracts "github.com/aws/amazon-ssm-agent/agent/message/contracts"
	"github.com/aws/amazon-ssm-agent/agent/platform"
	"github.com/aws/amazon-ssm-agent/agent/sdkutil"
	ssmsvc "github.com/aws/amazon-ssm-agent/agent/ssm"
	"github.com/aws/amazon-ssm-agent/agent/task"
	"github.com/aws/amazon-ssm-agent/reply"
	"github.com/aws/aws-sdk-go/service/ssm"
	"github.com/carlescere/scheduler"
)

const (
	name                          = "Association"
	documentWorkersLimit          = 1
	cancelWaitDurationMillisecond = 10000
	stopPolicyErrorThreshold      = 10
)

// Processor contains the logic for processing association
type Processor struct {
	PollJob             *scheduler.Job
	SsmSvc              ssmsvc.Service
	Context             context.T
	TaskPool            taskpool.T
	AgentInfo           *contracts.AgentInfo
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
		Context:             assocContext,
		SsmSvc:              ssmService,
		TaskPool:            taskPool,
		AgentInfo:           &agentInfo,
		processorStopPolicy: sdkutil.NewStopPolicy(name, stopPolicyErrorThreshold),
	}
}

// ProcessAssociation poll and process all the associations
func (p *Processor) ProcessAssociation() {
	log := p.Context.Log()
	var assocRawData *model.AssociationRawData

	instanceID, err := platform.InstanceID()
	if err != nil {
		log.Error("unable to retrieve instance id", err)
		return
	}

	if assocRawData, err = assocSvc.ListAssociations(log, p.SsmSvc, instanceID); err != nil {
		log.Error("unable to retrieve associations", err)
		return
	}

	if err = assocSvc.LoadAssociationDetail(log, p.SsmSvc, assocRawData); err != nil {
		message := fmt.Sprintf("unable to load association details, %v", err)
		log.Error(message)
		p.updateAssocStatus(assocRawData.Association, ssm.AssociationStatusNameFailed, message)
		return
	}

	if err = submitAssociation(p, assocRawData); err != nil {
		message := fmt.Sprintf("failed to process association, %v", err)
		log.Error(message)
		p.updateAssocStatus(assocRawData.Association, ssm.AssociationStatusNameFailed, message)
		return
	}
}

// submitAssociation submits the association to the task pool for execution
func submitAssociation(p *Processor, rawData *model.AssociationRawData) (err error) {
	// create separate logger that includes messageID with every log message
	context := p.Context.With("[associationName=" + *rawData.Association.Name + "]")
	log := context.Log()

	log.Debug("Processing association")

	document, err := assocParser.ParseDocumentWithParams(log, rawData)
	if err != nil {
		message := fmt.Sprintf("failed to parse association, %v", err)
		log.Error(message)
		p.updateAssocStatus(rawData.Association, ssm.AssociationStatusNameFailed, message)
		return
	}

	parsedMessageContent, _ := jsonutil.Marshal(document)
	log.Debug("ParsedAssociation is ", jsonutil.Indent(parsedMessageContent))

	//Data format persisted in Current Folder is defined by the struct - CommandState
	pluginConfigurations, interimDocState := assocParser.InitializeDocumentState(context, document, rawData)

	log.Debug("Persisting interim state in current execution folder")
	bookkeepingSvc.PersistData(log,
		interimDocState.DocumentInformation.CommandID,
		interimDocState.DocumentInformation.Destination,
		appconfig.DefaultLocationOfPending,
		interimDocState)

	log.Debugf("Persist document and update association status to pending")
	p.updateAssocStatus(rawData.Association, ssm.AssociationStatusNamePending, "processing document")

	if err = p.TaskPool.Submit(log, rawData.ID, func(cancelFlag task.CancelFlag) {
		p.processAssociationDocument(context, pluginConfigurations, &interimDocState, cancelFlag)
	}); err != nil {
		message := fmt.Sprintf("process association failed, %v", err)
		log.Error(message)
		p.updateAssocStatus(rawData.Association, ssm.AssociationStatusNameFailed, message)
		return
	}

	return nil
}

// processAssociationDocument parses and processes the document
func (p *Processor) processAssociationDocument(context context.T,
	pluginConfigurations map[string]*contracts.Configuration,
	interimDocState *messageContracts.CommandState,
	cancelFlag task.CancelFlag) {

	log := context.Log()
	//TODO: check isManagedInstance

	bookkeepingSvc.MoveCommandState(log,
		interimDocState.DocumentInformation.CommandID,
		interimDocState.DocumentInformation.Destination,
		appconfig.DefaultLocationOfPending,
		appconfig.DefaultLocationOfCurrent)

	log.Debug("Running plugins...")

	outputs := pulginExecution.RunPlugins(context,
		interimDocState.DocumentInformation.CommandID,
		pluginConfigurations,
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
	interimDocState *messageContracts.CommandState,
	pluginOutputs map[string]*contracts.PluginResult) {

	//update interim cmd state file
	documentInfo := bookkeepingSvc.GetDocumentInfo(log,
		interimDocState.DocumentInformation.CommandID,
		interimDocState.DocumentInformation.Destination,
		appconfig.DefaultLocationOfCurrent)

	runtimeStatuses := reply.PrepareRuntimeStatuses(log, pluginOutputs)
	replyPayload := reply.PrepareReplyPayload("", runtimeStatuses, time.Now(), *p.AgentInfo)

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
	log := p.Context.Log()

	service.UpdateAssociationStatus(log,
		p.SsmSvc,
		*assoc.InstanceId,
		*assoc.Name,
		status,
		message,
		p.AgentInfo,
		p.processorStopPolicy)
}

// updateAssocStatusWithDocInfo provides wrapper for calling update association service
func (p *Processor) updateAssocStatusWithDocInfo(
	assoc *messageContracts.DocumentInfo,
	status string,
	message string) {
	log := p.Context.Log()

	service.UpdateAssociationStatus(log,
		p.SsmSvc,
		assoc.CommandID,
		assoc.DocumentName,
		status,
		message,
		p.AgentInfo,
		p.processorStopPolicy)
}
