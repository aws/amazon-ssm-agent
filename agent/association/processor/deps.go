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
	"github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/aws/amazon-ssm-agent/agent/log"
	messageContract "github.com/aws/amazon-ssm-agent/agent/message/contracts"
	"github.com/aws/amazon-ssm-agent/agent/platform"
	"github.com/aws/amazon-ssm-agent/agent/statemanager"
	stateModel "github.com/aws/amazon-ssm-agent/agent/statemanager/model"
)

var assocParser parserService = &assocParserService{}
var assocBookkeeping bookkeepingService = &assocBookkeepingService{}
var sys system = &systemImp{}

// system represents the dependency for platform
type system interface {
	InstanceID() (string, error)
	IsManagedInstance() (bool, error)
}

type systemImp struct{}

// InstanceID wraps platform InstanceID
func (systemImp) InstanceID() (string, error) {
	return platform.InstanceID()
}

// IsManagedInstance wraps platform IsManagedInstance
func (systemImp) IsManagedInstance() (bool, error) {
	return platform.IsManagedInstance()
}

// bookkeepingService represents the dependency for statemanager
type bookkeepingService interface {
	PersistData(log log.T, documentID, instanceID, locationFolder string, object interface{})
	IsDocumentCurrentlyExecuting(documentID, instanceID string) bool
}

type assocBookkeepingService struct{}

// PersistData wraps statemanager PersistData
func (assocBookkeepingService) PersistData(log log.T, documentID, instanceID, locationFolder string, object interface{}) {
	statemanager.PersistData(log, documentID, instanceID, locationFolder, object)
}

// IsDocumentExist wraps statemanager IsDocumentExist
func (assocBookkeepingService) IsDocumentCurrentlyExecuting(documentID, instanceID string) bool {
	return statemanager.IsDocumentCurrentlyExecuting(documentID, instanceID)
}

// parserService represents the dependency for association parser
type parserService interface {
	ParseDocumentWithParams(log log.T, rawData *model.InstanceAssociation) (*messageContract.SendCommandPayload, error)
	InitializeDocumentState(context context.T,
		payload *messageContract.SendCommandPayload,
		rawData *model.InstanceAssociation) stateModel.DocumentState
}

type assocParserService struct{}

// ParseDocumentWithParams wraps parser ParseDocumentWithParams
func (assocParserService) ParseDocumentWithParams(
	log log.T,
	rawData *model.InstanceAssociation) (*messageContract.SendCommandPayload, error) {

	return parser.ParseDocumentWithParams(log, rawData)
}

// InitializeDocumentState wraps engine InitializeCommandState
func (assocParserService) InitializeDocumentState(context context.T,
	payload *messageContract.SendCommandPayload,
	rawData *model.InstanceAssociation) stateModel.DocumentState {

	return parser.InitializeDocumentState(context, payload, rawData)
}
