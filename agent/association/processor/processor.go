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
	"bytes"
	"fmt"
	"path"
	"runtime/debug"
	"strings"
	"sync"
	"time"

	"github.com/aws/amazon-ssm-agent/agent/association/cache"
	complianceUploader "github.com/aws/amazon-ssm-agent/agent/association/compliance/uploader"
	"github.com/aws/amazon-ssm-agent/agent/association/frequentcollector"
	"github.com/aws/amazon-ssm-agent/agent/association/model"
	"github.com/aws/amazon-ssm-agent/agent/association/schedulemanager"
	"github.com/aws/amazon-ssm-agent/agent/association/schedulemanager/signal"
	assocScheduler "github.com/aws/amazon-ssm-agent/agent/association/scheduler"
	"github.com/aws/amazon-ssm-agent/agent/association/service"
	"github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/aws/amazon-ssm-agent/agent/contracts"
	"github.com/aws/amazon-ssm-agent/agent/framework/processor"
	"github.com/aws/amazon-ssm-agent/agent/jsonutil"
	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/amazon-ssm-agent/agent/times"
	"github.com/aws/amazon-ssm-agent/common/identity"
	"github.com/carlescere/scheduler"
)

const (
	name                                    = "Association"
	documentWorkersLimit                    = 1
	cancelWaitDurationMillisecond           = 10000
	documentLevelTimeOutDurationHour        = 2
	outputMessageTemplate            string = "%v out of %v plugin%v processed, %v success, %v failed, %v timedout, %v skipped. %v"
	defaultRetryWaitOnBootInSeconds         = 30
)

// Processor contains the logic for processing association
type Processor struct {
	pollJob            *scheduler.Job
	assocSvc           service.T
	complianceUploader complianceUploader.T
	context            context.T
	agentInfo          *contracts.AgentInfo
	proc               processor.Processor
	resChan            chan contracts.DocumentResult
	onBoot             bool
}

var lock sync.RWMutex

// NewAssociationProcessor returns a new Processor with the given context.
func NewAssociationProcessor(context context.T) *Processor {
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

	assocSvc := service.NewAssociationService(context, name)
	uploader := complianceUploader.NewComplianceUploader(context)

	//TODO Rename everything to service and move package to framework
	//association has no cancel worker
	proc := processor.NewEngineProcessor(assocContext, documentWorkersLimit, documentWorkersLimit, []contracts.DocumentType{contracts.Association})
	return &Processor{
		context:            assocContext,
		assocSvc:           assocSvc,
		complianceUploader: uploader,
		agentInfo:          &agentInfo,
		proc:               proc,
		onBoot:             true,
	}
}

func (p *Processor) ModuleExecute() {
	log := p.context.Log()
	associationFrequenceMinutes := p.context.AppConfig().Ssm.AssociationFrequencyMinutes
	log.Info("Starting association polling")
	log.Debugf("Association polling frequency is %v", associationFrequenceMinutes)
	var job *scheduler.Job
	var err error
	if job, err = assocScheduler.CreateScheduler(
		log,
		p.ProcessAssociation,
		associationFrequenceMinutes); err != nil {
		p.context.Log().Errorf("unable to schedule association processor. %v", err)
	}
	p.InitializeAssociationProcessor()
	p.SetPollJob(job)
}
func (p *Processor) ModuleRequestStop(stopType contracts.StopType) (err error) {
	assocScheduler.Stop(p.pollJob)
	signal.Stop()
	p.proc.Stop(stopType)
	return nil
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

	log.Info("Launching response handler")
	go p.listenToResponses()

	if err := p.proc.InitialProcessing(false); err != nil {
		log.Errorf("initial processing in EngineProcessor encountered error: %v", err)
		return
	}

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
	associations := []*model.InstanceAssociation{}

	log.Debug("running ProcessAssociation")

	instanceID, err := p.context.Identity().InstanceID()
	if err != nil {
		log.Error("Unable to retrieve instance id", err)
		return
	}

	p.assocSvc.CreateNewServiceIfUnHealthy(p.context)
	p.complianceUploader.CreateNewServiceIfUnHealthy(log)

	if associations, err = p.assocSvc.ListInstanceAssociations(log, instanceID); err != nil {
		log.Errorf("Unable to load instance associations, %v", err)
		return
	}

	// to account for any tag expansion delays on boot, call list associations again
	if p.onBoot {
		p.onBoot = false
		if len(associations) < 1 {
			log.Info("No associations on boot. Requerying for associations after 30 seconds.")
			time.Sleep(defaultRetryWaitOnBootInSeconds * time.Second)
			if associations, err = p.assocSvc.ListInstanceAssociations(log, instanceID); err != nil {
				log.Errorf("Unable to load instance associations, %v", err)
				return
			}
		}
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

	log.Debug("ProcessAssociation is triggering execution")

	signal.ExecuteAssociation(log)

	log.Debug("ProcessAssociation completed")
}

// runScheduledAssociation runs the next scheduled association
func (p *Processor) runScheduledAssociation(log log.T) {
	log.Debug("runScheduledAssociation starting")

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
		log.Errorf("Unable to get next scheduled association, %v, will retry later", err)
		return
	}

	if scheduledAssociation == nil {
		// if no scheduled association found at given time, get the next scheduled time and wait
		nextScheduledDate := schedulemanager.LoadNextScheduledDate(log)
		if nextScheduledDate != nil {
			signal.ResetWaitTimerForNextScheduledAssociation(log, *nextScheduledDate)
		} else {
			log.Debug("No association scheduled at this time, will retry later")
		}
		return
	}

	// stop previous wait timer if there is scheduled association
	signal.StopWaitTimerForNextScheduledAssociation()

	if schedulemanager.IsAssociationInProgress(*scheduledAssociation.Association.AssociationId) {
		log.Debug("runScheduledAssociation is InProgress")
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

	var docState *contracts.DocumentState
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
	instanceID, _ := p.context.Identity().InstanceID()
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

	log.Debug("runScheduledAssociation submitting document")

	p.proc.Submit(*docState)

	log.Debug("runScheduledAssociation submitted document")

	frequentCollector := frequentcollector.GetFrequentCollector()
	if frequentCollector.IsSoftwareInventoryAssociation(docState) {
		// Start the frequent collector if the association enabled it
		frequentCollector.ClearTicker()
		if frequentCollector.IsFrequentCollectorEnabled(p.context, docState, scheduledAssociation) {
			log.Infof("This software inventory association enabled frequent collector")
			frequentCollector.StartFrequentCollector(p.context, docState, scheduledAssociation)
		}
	}
}

func isAssociationTimedOut(assoc *model.InstanceAssociation) bool {
	if assoc.Association.LastExecutionDate == nil {
		return false
	}

	currentTime := time.Now().UTC()
	return (*assoc.Association.LastExecutionDate).Add(documentLevelTimeOutDurationHour * time.Hour).UTC().Before(currentTime)
}

// parseAssociation parses the association to the document state
func (p *Processor) parseAssociation(rawData *model.InstanceAssociation) (*contracts.DocumentState, error) {
	// create separate logger that includes messageID with every log message
	context := p.context.With("[associationId=" + *rawData.Association.AssociationId + "]")
	log := context.Log()
	docState := contracts.DocumentState{}

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

	isMI := identity.IsOnPremInstance(p.context.Identity())
	if isMI && contracts.IsManagedInstanceIncompatibleAWSSSMDocument(docState.DocumentInformation.DocumentName) {
		log.Debugf("Running incompatible AWS SSM Document %v on managed instance", docState.DocumentInformation.DocumentName)
		if err = contracts.RemoveDependencyOnInstanceMetadata(context, &docState); err != nil {
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

	_, _, runtimeStatuses := contracts.DocumentResultAggregator(log, pluginID, outputs)
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

	instanceID, err := r.context.Identity().InstanceID()
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

	_, _, runtimeStatuses := contracts.DocumentResultAggregator(log, "", outputs)
	runtimeStatusesContent, err := jsonutil.Marshal(runtimeStatuses)
	if err != nil {
		log.Error("could not marshal plugin outputs ", err)
		return
	}
	log.Info("Update instance association status with results ", jsonutil.Indent(runtimeStatusesContent))

	executionSummary, outputUrl := buildOutput(runtimeStatuses, totalNumberOfPlugins)
	instanceID, _ := r.context.Identity().InstanceID()
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

func (r *Processor) listenToResponses() {
	log := r.context.Log()
	defer func() {
		if r := recover(); r != nil {
			log.Errorf("Association processor listen panic: %v", r)
			log.Errorf("Stacktrace:\n%s", debug.Stack())
		}
	}()
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
			} else if res.Status == contracts.ResultStatusInProgress {
				// reset the association to pending if it's still in progress after the command finish
				r.associationExecutionReport(
					log,
					res.AssociationID,
					res.DocumentName,
					res.DocumentVersion,
					res.PluginResults,
					res.NPlugins,
					contracts.AssociationErrorCodeNoError,
					contracts.AssociationStatusPending,
				)
			}
			instanceID, _ := r.context.Identity().ShortInstanceID()
			//clean association logs once the document state is moved to completed
			//clean completed document state files and orchestration dirs. Takes care of only files generated by association in the folder
			go assocBookkeeping.DeleteOldOrchestrationDirectories(log,
				instanceID,
				r.context.AppConfig().Agent.OrchestrationRootDir,
				r.context.AppConfig().Ssm.RunCommandLogsRetentionDurationHours,
				r.context.AppConfig().Ssm.AssociationLogsRetentionDurationHours)
			//TODO move this part to service
			schedulemanager.UpdateNextScheduledDate(log, res.AssociationID)
			signal.ExecuteAssociation(log)

		}

	}
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
	timedOut := len(filterByStatus(runtimeStatuses, func(status contracts.ResultStatus) bool {
		return status == contracts.ResultStatusTimedOut
	}))
	skipped := len(filterByStatus(runtimeStatuses, func(status contracts.ResultStatus) bool {
		return status == contracts.ResultStatusSkipped
	}))
	failedPluginReportMap := filterByStatus(runtimeStatuses, func(status contracts.ResultStatus) bool {
		return status == contracts.ResultStatusFailed
	})
	failed := len(failedPluginReportMap)
	var buffer bytes.Buffer
	for pluginId := range failedPluginReportMap {
		buffer.WriteString(fmt.Sprintf("\nThe operation %v failed because %v.", pluginId, failedPluginReportMap[pluginId].StandardError))
	}
	failedPluginReport := buffer.String()
	for _, value := range runtimeStatuses {
		paths := strings.Split(value.OutputS3KeyPrefix, "/")
		for _, p := range paths[:len(paths)-1] {
			outputUrl = path.Join(outputUrl, p)
		}
		outputUrl = path.Join(value.OutputS3BucketName, outputUrl)
		break
	}

	return fmt.Sprintf(outputMessageTemplate, completed, totalNumberOfPlugins, plural, success, failed, timedOut, skipped, failedPluginReport), outputUrl
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
func updatePluginAssociationInstances(associationID string, docState *contracts.DocumentState) {
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
