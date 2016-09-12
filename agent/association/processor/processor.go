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

	"github.com/aws/amazon-ssm-agent/agent/appconfig"
	"github.com/aws/amazon-ssm-agent/agent/association/executer"
	"github.com/aws/amazon-ssm-agent/agent/association/model"
	"github.com/aws/amazon-ssm-agent/agent/association/service"
	"github.com/aws/amazon-ssm-agent/agent/association/taskpool"
	"github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/aws/amazon-ssm-agent/agent/contracts"
	"github.com/aws/amazon-ssm-agent/agent/jsonutil"
	"github.com/aws/amazon-ssm-agent/agent/log"
	messageContracts "github.com/aws/amazon-ssm-agent/agent/message/contracts"
	"github.com/aws/amazon-ssm-agent/agent/platform"
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
	stopPolicyErrorThreshold      = 10
)

// Processor contains the logic for processing association
type Processor struct {
	pollJob   *scheduler.Job
	assocSvc  service.T
	executer  executer.DocumentExecuter
	context   context.T
	taskPool  taskpool.T
	agentInfo *contracts.AgentInfo
}

// NewAssociationProcessor returns a new Processor with the given context.
func NewAssociationProcessor(context context.T) (*Processor, error) {
	assocContext := context.With("[" + name + "]")

	log := assocContext.Log()
	config := assocContext.AppConfig()

	instanceID, err := platform.InstanceID()
	if instanceID == "" {
		log.Errorf("no instanceID provided, %v", err)
		return nil, err
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
	assocSvc := service.NewAssociationService(ssmService, sdkutil.NewStopPolicy(name, stopPolicyErrorThreshold))
	executer := executer.NewAssociationExecuter(assocSvc, &agentInfo)

	return &Processor{
		context:   assocContext,
		assocSvc:  assocSvc,
		taskPool:  taskPool,
		executer:  executer,
		agentInfo: &agentInfo,
	}, nil
}

// SetPollJob represents setter for PollJob
func (p *Processor) SetPollJob(job *scheduler.Job) {
	p.pollJob = job
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

	if assocRawData, err = p.assocSvc.ListAssociations(log, instanceID); err != nil {
		log.Error("unable to retrieve associations", err)
		return
	}

	if err = p.assocSvc.LoadAssociationDetail(log, assocRawData); err != nil {
		message := fmt.Sprintf("unable to load association details, %v", err)
		log.Error(message)
		p.updateAssocStatus(assocRawData.Association, ssm.AssociationStatusNameFailed, message)
		return
	}
	var docState *messageContracts.DocumentState
	if docState, err = p.parseAssociation(assocRawData); err != nil {
		message := fmt.Sprintf("unable to parse association, %v", err)
		log.Error(message)
		p.updateAssocStatus(assocRawData.Association, ssm.AssociationStatusNameFailed, message)
		return
	}

	if err = p.persistAssociationForExecution(log, docState); err != nil {
		message := fmt.Sprintf("unable to submit association for exectution, %v", err)
		log.Error(message)
		p.updateAssocStatus(assocRawData.Association, ssm.AssociationStatusNameFailed, message)
		return
	}
}

// ExecutePendingDocument wraps ExecutePendingDocument from document executer
func (p *Processor) ExecutePendingDocument(docState *messageContracts.DocumentState) {
	p.executer.ExecutePendingDocument(p.context, p.taskPool, docState)
}

// ExecuteInProgressDocument wraps ExecuteInProgressDocument from document executer
func (p *Processor) ExecuteInProgressDocument(docState *messageContracts.DocumentState, cancelFlag task.CancelFlag) {
	p.executer.ExecuteInProgressDocument(p.context, docState, cancelFlag)
}

// SubmitTask wraps SubmitTask for taskpool
func (p *Processor) SubmitTask(log log.T, jobID string, job task.Job) error {
	return p.taskPool.Submit(log, jobID, job)
}

// parseAssociation parses the association to the document state
func (p *Processor) parseAssociation(rawData *model.AssociationRawData) (*messageContracts.DocumentState, error) {
	// create separate logger that includes messageID with every log message
	context := p.context.With("[associationName=" + *rawData.Association.Name + "]")
	log := context.Log()
	docState := messageContracts.DocumentState{}

	log.Debug("Processing association")

	document, err := assocParser.ParseDocumentWithParams(log, rawData)
	if err != nil {
		return &docState, fmt.Errorf("failed to parse association, %v", err)
	}

	parsedMessageContent, _ := jsonutil.Marshal(document)
	log.Debug("ParsedAssociation is ", jsonutil.Indent(parsedMessageContent))

	//Data format persisted in Current Folder is defined by the struct - DocumentState
	docState = assocParser.InitializeDocumentState(context, document, rawData)

	return &docState, nil
}

// persistAssociationForExecution saves the document to pending folder and submit it to the task pool
func (p *Processor) persistAssociationForExecution(log log.T, docState *messageContracts.DocumentState) error {
	log.Debug("Persisting interim state in current execution folder")
	assocBookkeeping.PersistData(log,
		docState.DocumentInformation.CommandID,
		docState.DocumentInformation.Destination,
		appconfig.DefaultLocationOfPending,
		docState)

	return p.executer.ExecutePendingDocument(p.context, p.taskPool, docState)
}

// updateAssociationStatus provides wrapper for calling update association service
func (p *Processor) updateAssocStatus(
	assoc *ssm.Association,
	status string,
	message string) {
	log := p.context.Log()

	p.assocSvc.UpdateAssociationStatus(
		log,
		*assoc.InstanceId,
		*assoc.Name,
		status,
		message,
		p.agentInfo)
}
