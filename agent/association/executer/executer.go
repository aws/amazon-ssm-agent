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

// Package executer allows execute Pending association and InProgress association
package executer

import (
	"fmt"
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/aws/amazon-ssm-agent/agent/appconfig"
	"github.com/aws/amazon-ssm-agent/agent/association/schedulemanager"
	"github.com/aws/amazon-ssm-agent/agent/association/schedulemanager/signal"
	"github.com/aws/amazon-ssm-agent/agent/association/service"
	"github.com/aws/amazon-ssm-agent/agent/association/taskpool"
	"github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/aws/amazon-ssm-agent/agent/contracts"
	docModel "github.com/aws/amazon-ssm-agent/agent/docmanager/model"

	"github.com/aws/amazon-ssm-agent/agent/docmanager"
	"github.com/aws/amazon-ssm-agent/agent/framework/processor/executer"
	"github.com/aws/amazon-ssm-agent/agent/framework/processor/executer/basicexecuter"
	"github.com/aws/amazon-ssm-agent/agent/jsonutil"
	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/amazon-ssm-agent/agent/platform"
	"github.com/aws/amazon-ssm-agent/agent/task"
	"github.com/aws/amazon-ssm-agent/agent/times"
)

const (
	outputMessageTemplate string = "%v out of %v plugin%v processed, %v success, %v failed, %v timedout, %v skipped"
)

//TODO this should be the dependency injected into the association service later
var executerCreator = func(assocContext context.T) executer.Executer {
	return basicexecuter.NewBasicExecuter(assocContext)
}

// DocumentExecuter represents the interface for running a document
type DocumentExecuter interface {
	ExecutePendingDocument(context context.T, pool taskpool.T, docState *docModel.DocumentState) error
	ExecuteInProgressDocument(context context.T, docState *docModel.DocumentState, cancelFlag task.CancelFlag)
}

// AssociationExecuter represents the implementation of document executer
type AssociationExecuter struct {
	assocSvc  service.T
	agentInfo *contracts.AgentInfo
}

// NewAssociationExecuter returns a new document executer
func NewAssociationExecuter(assocSvc service.T, agentInfo *contracts.AgentInfo) *AssociationExecuter {
	runner := AssociationExecuter{
		assocSvc:  assocSvc,
		agentInfo: agentInfo,
	}

	return &runner
}

// ExecutePendingDocument moves doc to current folder and submit it for execution
func (r *AssociationExecuter) ExecutePendingDocument(context context.T, pool taskpool.T, docState *docModel.DocumentState) error {
	log := context.With("[associationId=" + docState.DocumentInformation.AssociationID + "]").Log()
	log.Debugf("Persist document to the state folder for execution")

	bookkeepingSvc.MoveDocumentState(log,
		docState.DocumentInformation.DocumentID,
		docState.DocumentInformation.InstanceID,
		appconfig.DefaultLocationOfPending,
		appconfig.DefaultLocationOfCurrent)

	if err := pool.Submit(log, docState.DocumentInformation.AssociationID, func(cancelFlag task.CancelFlag) {
		r.ExecuteInProgressDocument(context, docState, cancelFlag)
	}); err != nil {
		return fmt.Errorf("failed to process association, %v", err)
	}

	return nil
}

// ExecuteInProgressDocument parses and processes the document
func (r *AssociationExecuter) ExecuteInProgressDocument(context context.T, docState *docModel.DocumentState, cancelFlag task.CancelFlag) {
	assocContext := context.With("[associationId=" + docState.DocumentInformation.AssociationID + "]")
	log := assocContext.Log()

	totalNumberOfActions := len(docState.InstancePluginsInformation)

	assocID := docState.DocumentInformation.AssociationID
	documentID := docState.DocumentInformation.DocumentID
	instanceID := docState.DocumentInformation.InstanceID

	//TODO we should have a creator for factory construct of Executer
	e := executerCreator(assocContext)
	docStore := executer.NewDocumentFileStore(assocContext, instanceID, documentID, appconfig.DefaultLocationOfCurrent, docState)

	resChan := e.Run(cancelFlag, &docStore)
	for res := range resChan {
		log.Infof("update association status upon plugin $v completion", res.LastPlugin)
		r.pluginExecutionReport(log, assocID, res.LastPlugin, res.PluginResults, totalNumberOfActions)
	}

	//TODO below is processor's responisbility, make sure them parity with Processor
	newCmdState := docStore.Load()
	// Skip sending response when the document requires a reboot
	if newCmdState.IsRebootRequired() {
		log.Debugf("skipping sending response of %v since the document requires a reboot", newCmdState.DocumentInformation.AssociationID)
		// stop execution signal if detects reboot
		signal.StopExecutionSignal()
		return
	}

	log.Debug("Association execution completion ", newCmdState.InstancePluginsInformation)
	log.Debug("Association execution status is ", newCmdState.DocumentInformation.DocumentStatus)
	if newCmdState.DocumentInformation.DocumentStatus == contracts.ResultStatusFailed {
		r.associationExecutionReport(
			log,
			&newCmdState.DocumentInformation,
			newCmdState.DocumentInformation.RuntimeStatus,
			totalNumberOfActions,
			contracts.AssociationErrorCodeExecutionError,
			contracts.AssociationStatusFailed)

	} else if newCmdState.DocumentInformation.DocumentStatus == contracts.ResultStatusSuccess ||
		newCmdState.DocumentInformation.DocumentStatus == contracts.AssociationStatusTimedOut ||
		newCmdState.DocumentInformation.DocumentStatus == contracts.ResultStatusCancelled ||
		newCmdState.DocumentInformation.DocumentStatus == contracts.ResultStatusSkipped {
		// Association should only update status when it's Failed, Success, TimedOut, Cancelled or Skipped as Final status
		r.associationExecutionReport(
			log,
			&newCmdState.DocumentInformation,
			newCmdState.DocumentInformation.RuntimeStatus,
			totalNumberOfActions,
			contracts.AssociationErrorCodeNoError,
			string(newCmdState.DocumentInformation.DocumentStatus))
	}

	//persist : commands execution in completed folder (terminal state folder)
	log.Debugf("execution of %v is over. Moving newCmdState file from Current to Completed folder", newCmdState.DocumentInformation.AssociationID)
	bookkeepingSvc.MoveDocumentState(log,
		documentID,
		instanceID,
		appconfig.DefaultLocationOfCurrent,
		appconfig.DefaultLocationOfCompleted)

	//clean association logs once the document state is moved to completed
	//clean completed document state files and orchestration dirs. Takes care of only files generated by association in the folder
	go bookkeepingSvc.DeleteOldDocumentFolderLogs(log,
		instanceID,
		assocContext.AppConfig().Agent.OrchestrationRootDir,
		context.AppConfig().Ssm.AssociationLogsRetentionDurationHours,
		isAssociationLogFile,
		formAssociationOrchestrationFolder)
	//TODO move this part to service
	schedulemanager.UpdateNextScheduledDate(log, newCmdState.DocumentInformation.AssociationID)
	signal.ExecuteAssociation(log)
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

// pluginExecutionReport allow engine to update progress after every plugin execution
// TODO: add unittest for this function
func (r *AssociationExecuter) pluginExecutionReport(
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
func (r *AssociationExecuter) associationExecutionReport(
	log log.T,
	docInfo *docModel.DocumentInfo,
	runtimeStatuses map[string]*contracts.PluginRuntimeStatus,
	totalNumberOfPlugins int,
	errorCode string,
	associationStatus string) {

	runtimeStatusesContent, err := jsonutil.Marshal(runtimeStatuses)
	if err != nil {
		log.Error("could not marshal plugin outputs ", err)
		return
	}
	log.Info("Update instance association status with results ", jsonutil.Indent(runtimeStatusesContent))

	executionSummary, outputUrl := buildOutput(runtimeStatuses, totalNumberOfPlugins)
	r.assocSvc.UpdateInstanceAssociationStatus(
		log,
		docInfo.AssociationID,
		docInfo.DocumentName,
		docInfo.InstanceID,
		associationStatus,
		errorCode,
		times.ToIso8601UTC(time.Now()),
		executionSummary,
		outputUrl)
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
