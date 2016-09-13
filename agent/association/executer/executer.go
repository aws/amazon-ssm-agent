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
	"time"

	"github.com/aws/amazon-ssm-agent/agent/appconfig"
	"github.com/aws/amazon-ssm-agent/agent/association/service"
	"github.com/aws/amazon-ssm-agent/agent/association/taskpool"
	"github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/aws/amazon-ssm-agent/agent/contracts"
	"github.com/aws/amazon-ssm-agent/agent/framework/plugin"
	"github.com/aws/amazon-ssm-agent/agent/jsonutil"
	"github.com/aws/amazon-ssm-agent/agent/log"
	messageContracts "github.com/aws/amazon-ssm-agent/agent/message/contracts"
	"github.com/aws/amazon-ssm-agent/agent/reply"
	"github.com/aws/amazon-ssm-agent/agent/task"
	"github.com/aws/aws-sdk-go/service/ssm"
)

// DocumentExecuter represents the interface for running a document
type DocumentExecuter interface {
	ExecutePendingDocument(context context.T, pool taskpool.T, docState *messageContracts.DocumentState) error
	ExecuteInProgressDocument(context context.T, docState *messageContracts.DocumentState, cancelFlag task.CancelFlag)
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
func (r *AssociationExecuter) ExecutePendingDocument(context context.T, pool taskpool.T, docState *messageContracts.DocumentState) error {
	log := context.Log()
	//TODO: check if p.sendDocLevelResponse is needed here
	log.Debugf("Persist document and update association status to pending")
	r.updateAssocStatusWithDocInfo(
		log,
		&docState.DocumentInformation,
		ssm.AssociationStatusNamePending,
		"processing document")

	bookkeepingSvc.MoveCommandState(log,
		docState.DocumentInformation.CommandID,
		docState.DocumentInformation.Destination,
		appconfig.DefaultLocationOfPending,
		appconfig.DefaultLocationOfCurrent)

	if err := pool.Submit(log, docState.DocumentInformation.CommandID, func(cancelFlag task.CancelFlag) {
		r.ExecuteInProgressDocument(context, docState, cancelFlag)
	}); err != nil {
		return fmt.Errorf("failed to process association, %v", err)
	}

	return nil
}

// ExecuteInProgressDocument parses and processes the document
func (r *AssociationExecuter) ExecuteInProgressDocument(context context.T, docState *messageContracts.DocumentState, cancelFlag task.CancelFlag) {
	log := context.Log()
	//TODO: check isManagedInstance
	log.Debug("Running plugins...")

	//TODO: add plugin report for update association state
	outputs := pluginExecution.RunPlugins(context,
		docState.DocumentInformation.CommandID,
		&docState.PluginsInformation,
		plugin.RegisteredWorkerPlugins(context),
		nil,
		cancelFlag)

	pluginOutputContent, err := jsonutil.Marshal(outputs)
	if err != nil {
		log.Error("failed to parse to json string ", err)
		return
	}
	log.Debugf("Plugin outputs %v", jsonutil.Indent(pluginOutputContent))

	r.parseAndPersistReplyContents(log, docState, outputs)
	// Skip sending response when the document requires a reboot
	if docState.IsRebootRequired() {
		log.Debug("skipping sending response of %v since the document requires a reboot", docState.DocumentInformation.CommandID)
		return
	}

	log.Debug("Association execution completion ", outputs)
	if docState.DocumentInformation.DocumentStatus == contracts.ResultStatusFailed {
		r.updateAssocStatusWithDocInfo(
			log,
			&docState.DocumentInformation,
			ssm.AssociationStatusNameFailed,
			"Execution failed")

	} else if docState.DocumentInformation.DocumentStatus == contracts.ResultStatusSuccess {
		r.updateAssocStatusWithDocInfo(
			log,
			&docState.DocumentInformation,
			ssm.AssociationStatusNameSuccess,
			"Execution succeeded")
	}

	//persist : commands execution in completed folder (terminal state folder)
	log.Debugf("execution of %v is over. Moving interimState file from Current to Completed folder", docState.DocumentInformation.CommandID)
	bookkeepingSvc.MoveCommandState(log,
		docState.DocumentInformation.CommandID,
		docState.DocumentInformation.Destination,
		appconfig.DefaultLocationOfCurrent,
		appconfig.DefaultLocationOfCompleted)
}

// parseAndPersistReplyContents reloads interimDocState, updates it with replyPayload and persist it on disk.
func (r *AssociationExecuter) parseAndPersistReplyContents(log log.T,
	docState *messageContracts.DocumentState,
	pluginOutputs map[string]*contracts.PluginResult) {

	//update interim cmd state file
	documentInfo := bookkeepingSvc.GetDocumentInfo(log,
		docState.DocumentInformation.CommandID,
		docState.DocumentInformation.Destination,
		appconfig.DefaultLocationOfCurrent)

	runtimeStatuses := reply.PrepareRuntimeStatuses(log, pluginOutputs)
	replyPayload := reply.PrepareReplyPayload("", runtimeStatuses, time.Now(), *r.agentInfo)

	// set document level information which wasn't set previously
	documentInfo.AdditionalInfo = replyPayload.AdditionalInfo
	documentInfo.DocumentStatus = replyPayload.DocumentStatus
	documentInfo.DocumentTraceOutput = replyPayload.DocumentTraceOutput
	documentInfo.RuntimeStatus = replyPayload.RuntimeStatus

	//persist final documentInfo.
	bookkeepingSvc.PersistDocumentInfo(log,
		documentInfo,
		docState.DocumentInformation.CommandID,
		docState.DocumentInformation.Destination,
		appconfig.DefaultLocationOfCurrent)
}

// updateAssocStatusWithDocInfo provides wrapper for calling update association service
func (r *AssociationExecuter) updateAssocStatusWithDocInfo(
	log log.T,
	assoc *messageContracts.DocumentInfo,
	status string,
	message string) {

	r.assocSvc.UpdateAssociationStatus(
		log,
		assoc.Destination,
		assoc.DocumentName,
		status,
		message,
		r.agentInfo)
}
