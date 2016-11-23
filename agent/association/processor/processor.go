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
	"github.com/aws/amazon-ssm-agent/agent/association/cache"
	"github.com/aws/amazon-ssm-agent/agent/association/executer"
	"github.com/aws/amazon-ssm-agent/agent/association/model"
	"github.com/aws/amazon-ssm-agent/agent/association/schedulemanager"
	"github.com/aws/amazon-ssm-agent/agent/association/schedulemanager/signal"
	assocScheduler "github.com/aws/amazon-ssm-agent/agent/association/scheduler"
	"github.com/aws/amazon-ssm-agent/agent/association/service"
	"github.com/aws/amazon-ssm-agent/agent/association/taskpool"
	"github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/aws/amazon-ssm-agent/agent/contracts"
	"github.com/aws/amazon-ssm-agent/agent/jsonutil"
	"github.com/aws/amazon-ssm-agent/agent/log"
	stateModel "github.com/aws/amazon-ssm-agent/agent/statemanager/model"
	"github.com/aws/amazon-ssm-agent/agent/task"
	"github.com/aws/amazon-ssm-agent/agent/times"
	"github.com/aws/aws-sdk-go/service/ssm"
	"github.com/carlescere/scheduler"
)

const (
	name                          = "Association"
	documentWorkersLimit          = 1
	cancelWaitDurationMillisecond = 10000
)

// Processor contains the logic for processing association
type Processor struct {
	pollJob    *scheduler.Job
	assocSvc   service.T
	executer   executer.DocumentExecuter
	context    context.T
	taskPool   taskpool.T
	agentInfo  *contracts.AgentInfo
	stopSignal chan bool
}

// NewAssociationProcessor returns a new Processor with the given context.
func NewAssociationProcessor(context context.T, instanceID string) *Processor {
	assocContext := context.With("[" + name + "]")

	log := assocContext.Log()
	config := assocContext.AppConfig()

	taskPool := taskpool.NewTaskPool(log, documentWorkersLimit, cancelWaitDurationMillisecond)

	agentInfo := contracts.AgentInfo{
		Lang:      config.Os.Lang,
		Name:      config.Agent.Name,
		Version:   config.Agent.Version,
		Os:        config.Os.Name,
		OsVersion: config.Os.Version,
	}

	assocSvc := service.NewAssociationService(name)
	executer := executer.NewAssociationExecuter(assocSvc, &agentInfo)

	return &Processor{
		context:    assocContext,
		assocSvc:   assocSvc,
		taskPool:   taskPool,
		executer:   executer,
		agentInfo:  &agentInfo,
		stopSignal: make(chan bool),
	}
}

// StartAssociationWorker starts worker to process scheduled association
func (p *Processor) InitializeAssociationProcessor() {
	log := p.context.Log()

	log.Info("Initializing association scheduling service")
	signal.InitializeAssociationSignalService(log, p.runScheduledAssociation)
	log.Info("Association scheduling service initialized")
}

// SetPollJob represents setter for PollJob
func (p *Processor) SetPollJob(job *scheduler.Job) {
	p.pollJob = job
}

// ProcessAssociation poll and process all the associations
func (p *Processor) ProcessAssociation() {
	log := p.context.Log()
	var associations []*model.InstanceAssociation

	if p.isStopped() {
		log.Debug("Stopping association processor...")
		return
	}

	instanceID, err := sys.InstanceID()
	if err != nil {
		log.Error("Unable to retrieve instance id", err)
		return
	}

	p.assocSvc.CreateNewServiceIfUnHealthy(log)

	if associations, err = p.assocSvc.ListInstanceAssociations(log, instanceID); err != nil {
		log.Errorf("Unable to load instance associations, %v", err)
		return
	}

	// update the cache first
	for _, assoc := range associations {
		cache.ValidateCache(assoc)
	}

	// read from cache or load association details from service
	for _, assoc := range associations {
		var assocContent string
		if assocContent, err = jsonutil.Marshal(assoc); err != nil {
			return
		}
		log.Debug("Association content is \n", jsonutil.Indent(assocContent))

		if err = p.assocSvc.LoadAssociationDetail(log, assoc); err != nil {
			err = fmt.Errorf("Encountered error while loading association %v contents, %v",
				*assoc.Association.AssociationId,
				err)
			log.Error(err)
			p.updateInstanceAssocStatus(
				assoc.Association,
				contracts.AssociationStatusFailed,
				contracts.AssociationErrorCodeListAssociationError,
				times.ToIso8601UTC(time.Now()),
				err.Error())
			assoc.ExcludeFromFutureScheduling = true
		}
	}

	schedulemanager.Refresh(log, associations, p.assocSvc)
	signal.ExecuteAssociation(log)
}

// runScheduledAssociation runs the next scheduled association
func (p *Processor) runScheduledAssociation(log log.T) {
	defer func() {
		// recover in case the job panics
		if msg := recover(); msg != nil {
			log.Errorf("Execute association failed with message, %v", msg)
		}
	}()

	var (
		scheduledAssociation *model.InstanceAssociation
		err                  error
	)

	if scheduledAssociation, err = schedulemanager.LoadNextScheduledAssociation(log); err != nil {
		log.Errorf("Unable to get next scheduled association, %v, system will retry later", err)
		return
	}

	if scheduledAssociation == nil {
		// if no scheduled association found at given time, get the next scheduled time and wait
		nextScheduledDate := schedulemanager.LoadNextScheduledDate(log)
		if !nextScheduledDate.IsZero() {
			signal.ResetWaitTimerForNextScheduledAssociation(log, nextScheduledDate)
		} else {
			log.Debug("No association scheduled at this time, system will retry later")
		}
		return
	}

	// stop previous wait timer if there is scheduled association
	signal.StopWaitTimerForNextScheduledAssociation()

	if assocBookkeeping.IsDocumentCurrentlyExecuting(
		scheduledAssociation.DocumentID,
		*scheduledAssociation.Association.InstanceId) {
		log.Infof("Association %v is executing, system will retry later", *scheduledAssociation.Association.AssociationId)
		return
	}

	var docState *stateModel.DocumentState
	if docState, err = p.parseAssociation(scheduledAssociation); err != nil {
		err = fmt.Errorf("Encountered error while parsing association %v, %v",
			docState.DocumentInformation.AssociationID,
			err)
		log.Error(err)
		p.updateInstanceAssocStatus(
			scheduledAssociation.Association,
			contracts.AssociationStatusFailed,
			contracts.AssociationErrorCodeInvalidAssociation,
			times.ToIso8601UTC(time.Now()),
			err.Error())
		schedulemanager.ExcludeAssocFromFutureScheduling(log, *scheduledAssociation.Association.AssociationId)
		return
	}

	if err = p.persistAssociationForExecution(log, docState); err != nil {
		err = fmt.Errorf("Encountered error while persist association %v for execution, %v",
			docState.DocumentInformation.AssociationID,
			err)
		log.Error(err)
		p.updateInstanceAssocStatus(
			scheduledAssociation.Association,
			contracts.AssociationStatusFailed,
			contracts.AssociationErrorCodeSubmitAssociationError,
			times.ToIso8601UTC(time.Now()),
			err.Error())
		//TODO revisit the logic here
		schedulemanager.UpdateNextScheduledDate(log, *scheduledAssociation.Association.AssociationId)
		return
	}
}

// ExecutePendingDocument wraps ExecutePendingDocument from document executer
func (p *Processor) ExecutePendingDocument(docState *stateModel.DocumentState) {
	log := p.context.Log()
	if err := p.executer.ExecutePendingDocument(p.context, p.taskPool, docState); err != nil {
		log.Error("Failed to execute pending documents ", err)
	}
}

// ExecuteInProgressDocument wraps ExecuteInProgressDocument from document executer
func (p *Processor) ExecuteInProgressDocument(docState *stateModel.DocumentState, cancelFlag task.CancelFlag) {
	p.executer.ExecuteInProgressDocument(p.context, docState, cancelFlag)
}

// SubmitTask wraps SubmitTask for taskpool
func (p *Processor) SubmitTask(log log.T, jobID string, job task.Job) error {
	return p.taskPool.Submit(log, jobID, job)
}

// ShutdownAndWait wraps the ShutdownAndWait for task pool
func (p *Processor) ShutdownAndWait(timeout time.Duration) {
	p.taskPool.ShutdownAndWait(timeout)
}

// Stop stops the association processor and stops the poll job
func (p *Processor) Stop() {
	// close channel; subsequent calls to isDone will return true
	if !p.isStopped() {
		close(p.stopSignal)
	}

	assocScheduler.Stop(p.pollJob)
	signal.Stop()
}

// isStopped returns if the association processor has been stopped
func (p *Processor) isStopped() bool {
	select {
	case <-p.stopSignal:
		// received signal or channel already closed
		return true
	default:
		return false
	}
}

// parseAssociation parses the association to the document state
func (p *Processor) parseAssociation(rawData *model.InstanceAssociation) (*stateModel.DocumentState, error) {
	// create separate logger that includes messageID with every log message
	context := p.context.With("[associationName=" + *rawData.Association.Name + "]")
	log := context.Log()
	docState := stateModel.DocumentState{}

	log.Infof("Executing association %v", *rawData.Association.AssociationId)

	document, err := assocParser.ParseDocumentWithParams(log, rawData)
	if err != nil {
		log.Debugf("failed to parse association, %v", err)
		return &docState, err
	}

	var parsedMessageContent string
	if parsedMessageContent, err = jsonutil.Marshal(document); err != nil {
		errorMsg := "Encountered error while parsing input - internal error"
		log.Debugf("failed to parse document, %v", err)
		return &docState, fmt.Errorf("%v", errorMsg)
	}
	log.Debug("Parsed association content is \n", jsonutil.Indent(parsedMessageContent))

	//Data format persisted in Current Folder is defined by the struct - DocumentState
	docState = assocParser.InitializeDocumentState(context, document, rawData)

	isMI, err := sys.IsManagedInstance()
	if err != nil {
		errorMsg := "Error determining type of instance - internal error"
		log.Debugf("error determining managed instance, %v", err)
		return &docState, fmt.Errorf("%v", errorMsg)
	}

	if isMI {
		log.Debugf("Running incompatible AWS SSM Document %v on managed instance", docState.DocumentInformation.DocumentName)
		if err = stateModel.RemoveDependencyOnInstanceMetadata(context, &docState); err != nil {
			errorMsg := "Encountered error while parsing input - internal error"
			log.Debug(err)
			return &docState, fmt.Errorf("%v", errorMsg)
		}
	}

	return &docState, nil
}

// persistAssociationForExecution saves the document to pending folder and submit it to the task pool
func (p *Processor) persistAssociationForExecution(log log.T, docState *stateModel.DocumentState) error {
	log.Info("Persisting interim state in current execution folder")
	assocBookkeeping.PersistData(log,
		docState.DocumentInformation.DocumentID,
		docState.DocumentInformation.InstanceID,
		appconfig.DefaultLocationOfPending,
		docState)

	return p.executer.ExecutePendingDocument(p.context, p.taskPool, docState)
}

// updateInstanceAssocStatus provides wrapper for calling update association service
// TODO: executionDate is not used, remove it from the method
func (p *Processor) updateInstanceAssocStatus(
	assoc *ssm.InstanceAssociationSummary,
	status string,
	errorCode string,
	executionDate string,
	message string) {
	log := p.context.Log()

	p.assocSvc.UpdateInstanceAssociationStatus(
		log,
		*assoc.AssociationId,
		*assoc.Name,
		*assoc.InstanceId,
		status,
		errorCode,
		times.ToIso8601UTC(time.Now()),
		message)
}
