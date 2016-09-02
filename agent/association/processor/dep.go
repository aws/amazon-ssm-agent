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
	"github.com/aws/amazon-ssm-agent/agent/association/model"
	"github.com/aws/amazon-ssm-agent/agent/association/parser"
	"github.com/aws/amazon-ssm-agent/agent/association/service"
	"github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/aws/amazon-ssm-agent/agent/contracts"
	"github.com/aws/amazon-ssm-agent/agent/framework/engine"
	"github.com/aws/amazon-ssm-agent/agent/framework/plugin"
	"github.com/aws/amazon-ssm-agent/agent/log"
	message "github.com/aws/amazon-ssm-agent/agent/message/contracts"
	bookkeeping "github.com/aws/amazon-ssm-agent/agent/message/statemanager"
	"github.com/aws/amazon-ssm-agent/agent/sdkutil"
	ssmsvc "github.com/aws/amazon-ssm-agent/agent/ssm"
	"github.com/aws/amazon-ssm-agent/agent/task"
	"github.com/aws/aws-sdk-go/service/ssm"
)

var assocSvc assocService = assocSvcImp{}
var bookkeepingSvc = bookkeepingImp{}
var pulginExecution = pluginExecutionImp{}
var assocParser = parserImp{}

// assocService represents the dependency for association service
type assocService interface {
	ListAssociations(log log.T, ssmSvc ssmsvc.Service, instanceID string) (*model.AssociationRawData, error)
	LoadAssociationDetail(log log.T, ssmSvc ssmsvc.Service, assoc *model.AssociationRawData) error
	UpdateAssociationStatus(log log.T,
		ssmSvc ssmsvc.Service,
		instanceID string,
		name string,
		status string,
		message string,
		agentInfo *contracts.AgentInfo,
		processorStopPolicy *sdkutil.StopPolicy) (*ssm.UpdateAssociationStatusOutput, error)
}

type assocSvcImp struct{}

// ListAssociations wraps service ListAssociations
func (assocSvcImp) ListAssociations(log log.T, ssmSvc ssmsvc.Service, instanceID string) (*model.AssociationRawData, error) {
	return service.ListAssociations(log, ssmSvc, instanceID)
}

// LoadAssociationDetail wraps service LoadAssociationDetail
func (assocSvcImp) LoadAssociationDetail(log log.T, ssmSvc ssmsvc.Service, assoc *model.AssociationRawData) error {
	return service.LoadAssociationDetail(log, ssmSvc, assoc)
}

// UpdateAssociationStatus wraps service UpdateAssociationStatus
func (assocSvcImp) UpdateAssociationStatus(log log.T,
	ssmSvc ssmsvc.Service,
	instanceID string,
	name string,
	status string,
	message string,
	agentInfo *contracts.AgentInfo,
	processorStopPolicy *sdkutil.StopPolicy) (*ssm.UpdateAssociationStatusOutput, error) {
	return service.UpdateAssociationStatus(log, ssmSvc, instanceID, name, status, message, agentInfo, processorStopPolicy)
}

// bookkeepingService represents the dependency for statemanager
type bookkeepingService interface {
	PersistData(log log.T, commandID, instanceID, locationFolder string, object interface{})
	RemoveData(log log.T, commandID, instanceID, locationFolder string)
	GetDocumentInfo(log log.T, commandID, instanceID, locationFolder string) message.DocumentInfo
	PersistDocumentInfo(log log.T, docInfo message.DocumentInfo, commandID, instanceID, locationFolder string)
	MoveCommandState(log log.T, commandID, instanceID, srcLocationFolder, dstLocationFolder string)
}

type bookkeepingImp struct{}

// PersistData wraps statemanager PersistData
func (bookkeepingImp) PersistData(log log.T, commandID, instanceID, locationFolder string, object interface{}) {
	bookkeeping.PersistData(log, commandID, instanceID, locationFolder, object)
}

// RemoveData wraps statemanager RemoveData
func (bookkeepingImp) RemoveData(log log.T, commandID, instanceID, locationFolder string) {
	bookkeeping.RemoveData(log, commandID, instanceID, locationFolder)
}

// GetDocumentInfo wraps statemanager GetDocumentInfo
func (bookkeepingImp) GetDocumentInfo(log log.T, commandID, instanceID, locationFolder string) message.DocumentInfo {
	return bookkeeping.GetDocumentInfo(log, commandID, instanceID, locationFolder)
}

// PersistDocumentInfo wraps statemanager PersistDocumentInfo
func (bookkeepingImp) PersistDocumentInfo(log log.T, docInfo message.DocumentInfo, commandID, instanceID, locationFolder string) {
	bookkeeping.PersistDocumentInfo(log, docInfo, commandID, instanceID, locationFolder)
}

// MoveCommandState wraps statemanager MoveCommandState
func (bookkeepingImp) MoveCommandState(log log.T, commandID, instanceID, srcLocationFolder, dstLocationFolder string) {
	bookkeeping.MoveCommandState(log, commandID, instanceID, srcLocationFolder, dstLocationFolder)
}

// pluginExecutionService represents the dependency for engine
type pluginExecutionService interface {
	RunPlugins(
		context context.T,
		documentID string,
		plugins map[string]*contracts.Configuration,
		pluginRegistry plugin.PluginRegistry,
		sendReply engine.SendResponse,
		cancelFlag task.CancelFlag,
	) (pluginOutputs map[string]*contracts.PluginResult)
}

type pluginExecutionImp struct{}

// RunPlugins wraps engine RunPlugins
func (pluginExecutionImp) RunPlugins(
	context context.T,
	documentID string,
	plugins map[string]*contracts.Configuration,
	pluginRegistry plugin.PluginRegistry,
	sendReply engine.SendResponse,
	cancelFlag task.CancelFlag,
) (pluginOutputs map[string]*contracts.PluginResult) {
	return engine.RunPlugins(context, documentID, plugins, pluginRegistry, sendReply, cancelFlag)
}

// parserService represents the dependency for association parser
type parserService interface {
	ParseDocumentWithParams(log log.T, rawData *model.AssociationRawData) (*message.SendCommandPayload, error)
	InitializeCommandState(context context.T,
		payload *message.SendCommandPayload,
		rawData *model.AssociationRawData) (map[string]*contracts.Configuration, message.CommandState)
}

type parserImp struct{}

// ParseDocumentWithParams wraps parser ParseDocumentWithParams
func (parserImp) ParseDocumentWithParams(
	log log.T,
	rawData *model.AssociationRawData) (*message.SendCommandPayload, error) {

	return parser.ParseDocumentWithParams(log, rawData)
}

// InitializeCommandState wraps engine InitializeCommandState
func (parserImp) InitializeCommandState(context context.T,
	payload *message.SendCommandPayload,
	rawData *model.AssociationRawData) (map[string]*contracts.Configuration, message.CommandState) {

	return parser.InitializeCommandState(context, payload, rawData)
}
