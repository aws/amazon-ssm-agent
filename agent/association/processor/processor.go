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
	"github.com/aws/amazon-ssm-agent/agent/appconfig"
	"github.com/aws/amazon-ssm-agent/agent/association/model"
	"github.com/aws/amazon-ssm-agent/agent/association/taskpool"
	"github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/aws/amazon-ssm-agent/agent/contracts"
	"github.com/aws/amazon-ssm-agent/agent/framework/plugin"
	"github.com/aws/amazon-ssm-agent/agent/jsonutil"
	"github.com/aws/amazon-ssm-agent/agent/platform"
	ssmsvc "github.com/aws/amazon-ssm-agent/agent/ssm"
	"github.com/aws/amazon-ssm-agent/agent/task"
	"github.com/carlescere/scheduler"
)

const (
	name                          = "Association"
	documentWorkersLimit          = 1
	cancelWaitDurationMillisecond = 10000
)

// Processor contains the logic for processing association
type Processor struct {
	PollJob  *scheduler.Job
	SsmSvc   ssmsvc.Service
	Context  context.T
	TaskPool taskpool.T
}

// NewAssociationProcessor returns a new Processor with the given context.
func NewAssociationProcessor(context context.T) *Processor {
	assocContext := context.With("[" + name + "]")
	log := assocContext.Log()
	taskPool := taskpool.NewTaskPool(log, documentWorkersLimit, cancelWaitDurationMillisecond)

	ssmService := ssmsvc.NewService()
	return &Processor{
		Context:  assocContext,
		SsmSvc:   ssmService,
		TaskPool: taskPool,
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
		log.Error("unable to load association details", err)
		return
	}

	if err = submitAssociation(p, assocRawData); err != nil {
		log.Error("failed to process association ", err)
	}
}

// submitAssociation submits the association to the task pool for execution
func submitAssociation(p *Processor, rawData *model.AssociationRawData) (err error) {
	// create separate logger that includes messageID with every log message
	context := p.Context.With("[associationName=" + *rawData.Association.Name + "]")
	log := context.Log()

	log.Debug("Processing association")
	//persisting received association in file-system [pending folder]
	bookkeepingSvc.PersistData(log,
		rawData.ID,
		*rawData.Association.InstanceId,
		appconfig.DefaultLocationOfPending,
		*rawData)

	//TODO: send association level updates
	log.Debugf("Processing to send a reply to update the document status to InProgress")

	if err = p.TaskPool.Submit(log, rawData.ID, func(cancelFlag task.CancelFlag) {
		p.processAssociationDocument(context, rawData, cancelFlag)
	}); err != nil {
		log.Error("sendCommand failed ", err)
		return
	}

	return nil
}

// processAssociationDocument parses and processes the document
func (p *Processor) processAssociationDocument(context context.T, rawData *model.AssociationRawData, cancelFlag task.CancelFlag) {
	log := context.Log()

	document, err := assocParser.ParseDocumentWithParams(log, rawData)
	if err != nil {
		log.Error("failed to parse association ", err)
		return
	}

	parsedMessageContent, _ := jsonutil.Marshal(document)
	log.Debug("ParsedAssociation is ", jsonutil.Indent(parsedMessageContent))

	//persist : all information in current folder
	log.Debug("Persisting message in current execution folder")
	//Data format persisted in Current Folder is defined by the struct - CommandState
	pluginConfigurations, interimCmdState := assocParser.InitializeCommandState(context, document, rawData)

	//TODO: check isManagedInstance

	// persist new interim command state in current folder
	bookkeepingSvc.PersistData(log,
		interimCmdState.DocumentInformation.CommandID,
		interimCmdState.DocumentInformation.Destination,
		appconfig.DefaultLocationOfCurrent,
		interimCmdState)

	//Deleting from pending folder since the command is getting executed
	bookkeepingSvc.RemoveData(log,
		interimCmdState.DocumentInformation.CommandID,
		interimCmdState.DocumentInformation.Destination,
		appconfig.DefaultLocationOfPending)

	log.Debug("Running plugins...")

	outputs := pulginExecution.RunPlugins(context,
		interimCmdState.DocumentInformation.CommandID,
		pluginConfigurations,
		plugin.RegisteredWorkerPlugins(context),
		nil,
		cancelFlag)

	pluginOutputContent, _ := jsonutil.Marshal(outputs)
	log.Debugf("Plugin outputs %v", jsonutil.Indent(pluginOutputContent))

	//TODO: build reply and update association
	//update documentInfo in interim cmd state file
	documentInfo := bookkeepingSvc.GetDocumentInfo(log,
		interimCmdState.DocumentInformation.CommandID,
		interimCmdState.DocumentInformation.Destination,
		appconfig.DefaultLocationOfCurrent)

	// set document level information which wasn't set previously
	documentInfo.AdditionalInfo = contracts.AdditionalInfo{}
	documentInfo.DocumentTraceOutput = ""

	//persist final documentInfo.
	bookkeepingSvc.PersistDocumentInfo(log,
		documentInfo,
		interimCmdState.DocumentInformation.CommandID,
		interimCmdState.DocumentInformation.Destination,
		appconfig.DefaultLocationOfCurrent)

	// Skip sending response when the document requires a reboot
	if documentInfo.DocumentStatus == contracts.ResultStatusSuccessAndReboot {
		log.Debug("skipping sending response of %v since the document requires a reboot", documentInfo.CommandID)
		return
	}

	log.Debug("Association execution completion ", outputs)
	//TODO, update association to completion

	//persist : commands execution in completed folder (terminal state folder)
	log.Debugf("execution of %v is over. Moving interimState file from Current to Completed folder", documentInfo.CommandID)

	bookkeepingSvc.MoveCommandState(log,
		interimCmdState.DocumentInformation.CommandID,
		interimCmdState.DocumentInformation.Destination,
		appconfig.DefaultLocationOfCurrent,
		appconfig.DefaultLocationOfCompleted)
}
