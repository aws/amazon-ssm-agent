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
	"sync"
	"time"

	"path/filepath"
	"regexp"

	"path"
	"strings"

	"github.com/aws/amazon-ssm-agent/agent/association/cache"
	"github.com/aws/amazon-ssm-agent/agent/association/model"
	"github.com/aws/amazon-ssm-agent/agent/association/schedulemanager"
	"github.com/aws/amazon-ssm-agent/agent/association/schedulemanager/signal"
	assocScheduler "github.com/aws/amazon-ssm-agent/agent/association/scheduler"
	"github.com/aws/amazon-ssm-agent/agent/association/service"
	complianceUploader "github.com/aws/amazon-ssm-agent/agent/compliance/uploader"
	"github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/aws/amazon-ssm-agent/agent/contracts"
	"github.com/aws/amazon-ssm-agent/agent/docmanager"
	docModel "github.com/aws/amazon-ssm-agent/agent/docmanager/model"
	"github.com/aws/amazon-ssm-agent/agent/framework/processor"
	"github.com/aws/amazon-ssm-agent/agent/jsonutil"
	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/amazon-ssm-agent/agent/platform"
	"github.com/aws/amazon-ssm-agent/agent/plugins/pluginutil"
	"github.com/aws/amazon-ssm-agent/agent/times"
	"github.com/carlescere/scheduler"
)

const (
	name                                    = "Association"
	documentWorkersLimit                    = 1
	cancelWaitDurationMillisecond           = 10000
	documentLevelTimeOutDurationHour        = 2
	outputMessageTemplate            string = "%v out of %v plugin%v processed, %v success, %v failed, %v timedout, %v skipped"
)

// Processor contains the logic for processing association
type Processor struct {
	pollJob            *scheduler.Job
	assocSvc           service.T
	complianceUploader complianceUploader.T
	context            context.T
	agentInfo          *contracts.AgentInfo
	stopSignal         chan bool
	proc               processor.Processor
	resChan            chan contracts.DocumentResult
	defaultPlugin      pluginutil.DefaultPlugin
}

var lock sync.RWMutex

// NewAssociationProcessor returns a new Processor with the given context.
func NewAssociationProcessor(context context.T, instanceID string) *Processor {
	assocContext := context.With("[" + name + "]")

	config := assocContext.AppConfig()

	//TODO this is what service should know
	agentInfo := contracts.AgentInfo{
		Lang:      config.Os.Lang,
		Name:      config.Agent.Name,
		Version:   config.Agent.Version,
		Os:        config.Os.Name,
		OsVersion: config.Os.Version,
	}

	assocSvc := service.NewAssociationService(name)
	uploader := complianceUploader.NewComplianceUploader(context)

	//TODO Rename everything to service and move package to framework
	//association has no cancel worker
	proc := processor.NewEngineProcessor(assocContext, documentWorkersLimit, documentWorkersLimit, []docModel.DocumentType{docModel.Association})
	return &Processor{
		context:            assocContext,
		assocSvc:           assocSvc,
		complianceUploader: uploader,
		agentInfo:          &agentInfo,
		stopSignal:         make(chan bool),
		proc:               proc,
	}
}

// StartAssociationWorker starts worker to process scheduled association
func (p *Processor) InitializeAssociationProcessor() {
	log := p.context.Log()
	if resChan, err := p.proc.Start(); err != nil {
		log.Errorf("starting EngineProcessor encountered error: %v", err)
		return
	} else {
		p.resChan = resChan
	}
	log.Info("Initializing association scheduling service")
	signal.InitializeAssociationSignalService(log, p.runScheduledAssociation)
	log.Info("Association scheduling service initialized")
	log.Info("Launching response handler")
	go p.lisenToResponses()
}

// SetPollJob represents setter for PollJob
func (p *Processor) SetPollJob(job *scheduler.Job) {
	p.pollJob = job
}

// ProcessAssociation poll and process all the associations
func (p *Processor) ProcessAssociation() {
	log := p.context.Log()
	associations := []*model.InstanceAssociation{}

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
	p.complianceUploader.CreateNewServiceIfUnHealthy(log)

	if associations, err = p.assocSvc.ListInstanceAssociations(log, instanceID); err != nil {
		log.Errorf("Unable to load instance associations, %v", err)
		return
	}

	// evict the invalid cache first
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

		//TODO: add retry for load association detail
		if err = p.assocSvc.LoadAssociationDetail(log, assoc); err != nil {
			err = fmt.Errorf("Encountered error while loading association %v contents, %v",
				*assoc.Association.AssociationId,
				err)
			log.Error(err)
			assoc.Errors = append(assoc.Errors, err)
			p.assocSvc.UpdateInstanceAssociationStatus(
				log,
				*assoc.Association.AssociationId,
				*assoc.Association.Name,
				*assoc.Association.InstanceId,
				contracts.AssociationStatusFailed,
				contracts.AssociationErrorCodeListAssociationError,
				times.ToIso8601UTC(time.Now()),
				err.Error(),
				service.NoOutputUrl)

			p.complianceUploader.UpdateAssociationCompliance(
				*assoc.Association.AssociationId,
				*assoc.Association.InstanceId,
				*assoc.Association.Name,
				*assoc.Association.DocumentVersion,
				contracts.AssociationStatusFailed,
				time.Now().UTC())
			continue
		}

		if !assoc.IsRunOnceAssociation() {
			if err = assoc.ParseExpression(log); err != nil {
				message := fmt.Sprintf("Encountered error while parsing expression for association %v", *assoc.Association.AssociationId)
				log.Errorf("%v, %v", message, err)
				assoc.Errors = append(assoc.Errors, err)
				p.assocSvc.UpdateInstanceAssociationStatus(
					log,
					*assoc.Association.AssociationId,
					*assoc.Association.Name,
					*assoc.Association.InstanceId,
					contracts.AssociationStatusFailed,
					contracts.AssociationErrorCodeInvalidExpression,
					times.ToIso8601UTC(time.Now()),
					message,
					service.NoOutputUrl)

				p.complianceUploader.UpdateAssociationCompliance(
					*assoc.Association.AssociationId,
					*assoc.Association.InstanceId,
					*assoc.Association.Name,
					*assoc.Association.DocumentVersion,
					contracts.AssociationStatusFailed,
					time.Now().UTC())
				continue
			}
		}
	}

	schedulemanager.Refresh(log, associations)
	signal.ExecuteAssociation(log)
}

// runScheduledAssociation runs the next scheduled association
func (p *Processor) runScheduledAssociation(log log.T) {
	lock.Lock()
	defer lock.Unlock()

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
		if nextScheduledDate != nil {
			signal.ResetWaitTimerForNextScheduledAssociation(log, *nextScheduledDate)
		} else {
			log.Debug("No association scheduled at this time, system will retry later")
		}
		return
	}

	// stop previous wait timer if there is scheduled association
	signal.StopWaitTimerForNextScheduledAssociation()

	if schedulemanager.IsAssociationInProgress(*scheduledAssociation.Association.AssociationId) {
		if isAssociationTimedOut(scheduledAssociation) {
			err = fmt.Errorf("Association stuck at InProgress for longer than %v hours", documentLevelTimeOutDurationHour)
			log.Error(err)
			p.assocSvc.UpdateInstanceAssociationStatus(
				log,
				*scheduledAssociation.Association.AssociationId,
				*scheduledAssociation.Association.Name,
				*scheduledAssociation.Association.InstanceId,
				contracts.AssociationStatusFailed,
				contracts.AssociationErrorCodeStuckAtInProgressError,
				times.ToIso8601UTC(time.Now()),
				err.Error(),
				service.NoOutputUrl)
			p.complianceUploader.UpdateAssociationCompliance(
				*scheduledAssociation.Association.AssociationId,
				*scheduledAssociation.Association.InstanceId,
				*scheduledAssociation.Association.Name,
				*scheduledAssociation.Association.DocumentVersion,
				contracts.AssociationStatusFailed,
				time.Now().UTC())

		}

		return
	}

	log.Debugf("Update association %v to pending ", *scheduledAssociation.Association.AssociationId)
	// Update association status to pending
	p.assocSvc.UpdateInstanceAssociationStatus(
		log,
		*scheduledAssociation.Association.AssociationId,
		*scheduledAssociation.Association.Name,
		*scheduledAssociation.Association.InstanceId,
		contracts.AssociationStatusPending,
		contracts.AssociationErrorCodeNoError,
		times.ToIso8601UTC(time.Now()),
		contracts.AssociationPendingMessage,
		service.NoOutputUrl)

	var docState *docModel.DocumentState
	if docState, err = p.parseAssociation(scheduledAssociation); err != nil {
		err = fmt.Errorf("Encountered error while parsing association %v, %v",
			docState.DocumentInformation.AssociationID,
			err)
		log.Error(err)
		p.assocSvc.UpdateInstanceAssociationStatus(
			log,
			*scheduledAssociation.Association.AssociationId,
			*scheduledAssociation.Association.Name,
			*scheduledAssociation.Association.InstanceId,
			contracts.AssociationStatusFailed,
			contracts.AssociationErrorCodeInvalidAssociation,
			times.ToIso8601UTC(time.Now()),
			err.Error(),
			service.NoOutputUrl)
		p.complianceUploader.UpdateAssociationCompliance(
			*scheduledAssociation.Association.AssociationId,
			*scheduledAssociation.Association.InstanceId,
			*scheduledAssociation.Association.Name,
			*scheduledAssociation.Association.DocumentVersion,
			contracts.AssociationStatusFailed,
			time.Now().UTC())
		return
	}
	updatePluginAssociationInstances(*scheduledAssociation.Association.AssociationId, docState)
	log = p.context.With("[associationId=" + docState.DocumentInformation.AssociationID + "]").Log()
	instanceID, _ := sys.InstanceID()
	p.assocSvc.UpdateInstanceAssociationStatus(
		log,
		docState.DocumentInformation.AssociationID,
		docState.DocumentInformation.DocumentName,
		instanceID,
		contracts.AssociationStatusInProgress,
		contracts.AssociationErrorCodeNoError,
		times.ToIso8601UTC(time.Now()),
		contracts.AssociationInProgressMessage,
		service.NoOutputUrl)

	p.proc.Submit(*docState)
}

func isAssociationTimedOut(assoc *model.InstanceAssociation) bool {
	if assoc.Association.LastExecutionDate == nil {
		return false
	}

	currentTime := time.Now().UTC()
	return (*assoc.Association.LastExecutionDate).Add(documentLevelTimeOutDurationHour * time.Hour).UTC().Before(currentTime)
}

// ShutdownAndWait Stops the contained processor
func (p *Processor) ShutdownAndWait(stopType contracts.StopType) {
	p.proc.Stop(stopType)
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
func (p *Processor) parseAssociation(rawData *model.InstanceAssociation) (*docModel.DocumentState, error) {
	// create separate logger that includes messageID with every log message
	context := p.context.With("[associationId=" + *rawData.Association.AssociationId + "]")
	log := context.Log()
	docState := docModel.DocumentState{}

	log.Info("Executing association")

	document, err := assocParser.ParseDocumentForPayload(log, rawData)
	if err != nil {
		log.Debugf("Failed to parse association, %v", err)
		return &docState, err
	}

	if docState, err = assocParser.InitializeDocumentState(context, document, rawData); err != nil {
		return &docState, err
	}
	var parsedMessageContent string
	if parsedMessageContent, err = jsonutil.Marshal(document); err != nil {
		errorMsg := "Encountered error while parsing input - internal error"
		log.Debugf("failed to parse document, %v", err)
		return &docState, fmt.Errorf("%v", errorMsg)
	}
	log.Debug("Executing association with document content: \n", jsonutil.Indent(parsedMessageContent))

	isMI, err := sys.IsManagedInstance()
	if err != nil {
		errorMsg := "Error determining type of instance - internal error"
		log.Debugf("error determining managed instance, %v", err)
		return &docState, fmt.Errorf("%v", errorMsg)
	}

	if isMI {
		log.Debugf("Running incompatible AWS SSM Document %v on managed instance", docState.DocumentInformation.DocumentName)
		if err = docModel.RemoveDependencyOnInstanceMetadata(context, &docState); err != nil {
			errorMsg := "Encountered error while parsing input - internal error"
			log.Debug(err)
			return &docState, fmt.Errorf("%v", errorMsg)
		}
	}

	return &docState, nil
}

// pluginExecutionReport allow engine to update progress after every plugin execution
func (r *Processor) pluginExecutionReport(
	log log.T,
	associationID string,
	pluginID string,
	outputs map[string]*contracts.PluginResult,
	totalNumberOfPlugins int) {

	_, _, runtimeStatuses := docmanager.DocumentResultAggregator(log, pluginID, outputs)
	outputContent, err := jsonutil.Marshal(runtimeStatuses)
	if err != nil {
		log.Error("could not marshal plugin outputs! ", err)
		return
	}
	log.Info("Update instance association status with results ", jsonutil.Indent(outputContent))

	// Legacy association api does not support plugin level status update
	// it returns error for multiple update with same status
	if !r.assocSvc.IsInstanceAssociationApiMode() {
		return
	}

	instanceID, err := platform.InstanceID()
	if err != nil {
		log.Error("failed to load instance id ", err)
		return
	}

	executionSummary, outputUrl := buildOutput(runtimeStatuses, totalNumberOfPlugins)

	r.assocSvc.UpdateInstanceAssociationStatus(
		log,
		associationID,
		"",
		instanceID,
		contracts.AssociationStatusInProgress,
		contracts.AssociationErrorCodeNoError,
		times.ToIso8601UTC(time.Now()),
		executionSummary,
		outputUrl)
}

// associationExecutionReport update the status for association
func (r *Processor) associationExecutionReport(
	log log.T,
	associationID string,
	documentName string,
	documentVersion string,
	outputs map[string]*contracts.PluginResult,
	totalNumberOfPlugins int,
	errorCode string,
	associationStatus string) {

	_, _, runtimeStatuses := docmanager.DocumentResultAggregator(log, "", outputs)
	runtimeStatusesContent, err := jsonutil.Marshal(runtimeStatuses)
	if err != nil {
		log.Error("could not marshal plugin outputs ", err)
		return
	}
	log.Info("Update instance association status with results ", jsonutil.Indent(runtimeStatusesContent))

	executionSummary, outputUrl := buildOutput(runtimeStatuses, totalNumberOfPlugins)
	instanceID, _ := sys.InstanceID()
	r.assocSvc.UpdateInstanceAssociationStatus(
		log,
		associationID,
		documentName,
		instanceID,
		associationStatus,
		errorCode,
		times.ToIso8601UTC(time.Now()),
		executionSummary,
		outputUrl)

	r.complianceUploader.UpdateAssociationCompliance(
		associationID,
		instanceID,
		documentName,
		documentVersion,
		associationStatus,
		time.Now().UTC())
}

func (r *Processor) lisenToResponses() {
	log := r.context.Log()
	for res := range r.resChan {
		if res.LastPlugin != "" {
			log.Infof("update association status upon plugin $v completion", res.LastPlugin)
			r.pluginExecutionReport(log, res.AssociationID, res.LastPlugin, res.PluginResults, res.NPlugins)
		}
		if res.Status == contracts.ResultStatusSuccessAndReboot {
			signal.StopExecutionSignal()
			return
		}
		//send asociation completion response
		if res.LastPlugin == "" {
			log.Debug("Association execution completion: ", res.AssociationID)
			log.Debug("Association execution status is ", res.Status)
			if res.Status == contracts.ResultStatusFailed {
				r.associationExecutionReport(
					log,
					res.AssociationID,
					res.DocumentName,
					res.DocumentVersion,
					res.PluginResults,
					res.NPlugins,
					contracts.AssociationErrorCodeExecutionError,
					contracts.AssociationStatusFailed)

			} else if res.Status == contracts.ResultStatusSuccess ||
				res.Status == contracts.AssociationStatusTimedOut ||
				res.Status == contracts.ResultStatusSkipped {
				// Association should only update status when it's Failed, Success, TimedOut, or Skipped as Final status
				r.associationExecutionReport(
					log,
					res.AssociationID,
					res.DocumentName,
					res.DocumentVersion,
					res.PluginResults,
					res.NPlugins,
					contracts.AssociationErrorCodeNoError,
					string(res.Status))
			}
			instanceID, _ := sys.InstanceID()
			//clean association logs once the document state is moved to completed
			//clean completed document state files and orchestration dirs. Takes care of only files generated by association in the folder
			go assocBookkeeping.DeleteOldDocumentFolderLogs(log,
				instanceID,
				r.context.AppConfig().Agent.OrchestrationRootDir,
				r.context.AppConfig().Ssm.AssociationLogsRetentionDurationHours,
				isAssociationLogFile,
				formAssociationOrchestrationFolder)
			//TODO move this part to service
			schedulemanager.UpdateNextScheduledDate(log, res.AssociationID)
			signal.ExecuteAssociation(log)

		}

	}
}

// isAssociationLogFile checks whether the file name passed is of the format of Association Files
func isAssociationLogFile(fileName string) (matched bool) {
	matched, _ = regexp.MatchString("^[a-f0-9]{8}-[a-f0-9]{4}-[a-f0-9]{4}-[a-f0-9]{4}-[a-f0-9]{12}\\.[0-9]{4}-[0-9]{2}-[0-9]{2}.*$", fileName)
	return
}

// formAssociationOrchestrationFolder forms the orchestration dir name from the document state file
func formAssociationOrchestrationFolder(documentStateFileName string) string {
	splitFileName := strings.SplitN(documentStateFileName, ".", 2)
	if len(splitFileName) == 2 {
		assocID := splitFileName[0]
		isoDashUTCFormattedName := splitFileName[1]
		return filepath.Join(assocID, isoDashUTCFormattedName)
	}
	return documentStateFileName
}

// buildOutput build the output message for association update
// TODO: totalNumberOfPlugins is no longer needed, we can get the same value from len(runtimeStatuses)
func buildOutput(runtimeStatuses map[string]*contracts.PluginRuntimeStatus, totalNumberOfPlugins int) (outputSummary, outputUrl string) {
	plural := ""
	if totalNumberOfPlugins > 1 {
		plural = "s"
	}

	completed := len(filterByStatus(runtimeStatuses, func(status contracts.ResultStatus) bool {
		return status != ""
	}))

	success := len(filterByStatus(runtimeStatuses, func(status contracts.ResultStatus) bool {
		return status == contracts.ResultStatusPassedAndReboot ||
			status == contracts.ResultStatusSuccessAndReboot ||
			status == contracts.ResultStatusSuccess
	}))
	failed := len(filterByStatus(runtimeStatuses, func(status contracts.ResultStatus) bool {
		return status == contracts.ResultStatusFailed
	}))
	timedOut := len(filterByStatus(runtimeStatuses, func(status contracts.ResultStatus) bool {
		return status == contracts.ResultStatusTimedOut
	}))
	skipped := len(filterByStatus(runtimeStatuses, func(status contracts.ResultStatus) bool {
		return status == contracts.ResultStatusSkipped
	}))

	for _, value := range runtimeStatuses {
		paths := strings.Split(value.OutputS3KeyPrefix, "/")
		for _, p := range paths[:len(paths)-1] {
			outputUrl = path.Join(outputUrl, p)
		}
		outputUrl = path.Join(value.OutputS3BucketName, outputUrl)
		break
	}

	return fmt.Sprintf(outputMessageTemplate, completed, totalNumberOfPlugins, plural, success, failed, timedOut, skipped), outputUrl
}

// filterByStatus represents the helper method that filter pluginResults base on ResultStatus
func filterByStatus(runtimeStatuses map[string]*contracts.PluginRuntimeStatus, predicate func(contracts.ResultStatus) bool) map[string]*contracts.PluginRuntimeStatus {
	result := make(map[string]*contracts.PluginRuntimeStatus)
	for name, value := range runtimeStatuses {
		if predicate(value.Status) {
			result[name] = value
		}
	}
	return result
}

//This operation is locked by runScheduledAssociation
//lazy update, update only when the document is ready to run, update will validate and invalidate current attached association
func updatePluginAssociationInstances(associationID string, docState *docModel.DocumentState) {
	currentPluginAssociations := getPluginAssociationInstances()
	for i := 0; i < len(docState.InstancePluginsInformation); i++ {

		pluginName := docState.InstancePluginsInformation[i].Name
		//update the associations attached to the given plugin
		if list, ok := currentPluginAssociations[pluginName]; ok {
			newList := AssocList{associationID}
			for _, id := range list {
				if id != associationID && schedulemanager.AssociationExists(id) {
					newList = append(newList, id)
				}
			}
			currentPluginAssociations[pluginName] = newList

		} else {
			currentPluginAssociations[pluginName] = AssocList{associationID}
		}
		//assign the field to pluginconfig in place
		docState.InstancePluginsInformation[i].Configuration.CurrentAssociations = currentPluginAssociations[pluginName]
	}
	return
}
