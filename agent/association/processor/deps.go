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
	"github.com/aws/amazon-ssm-agent/agent/docmanager"
	docModel "github.com/aws/amazon-ssm-agent/agent/docmanager/model"
	messageContract "github.com/aws/amazon-ssm-agent/agent/framework/service/runcommand/contracts"
	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/amazon-ssm-agent/agent/platform"
)

var assocParser parserService = &assocParserService{}
var assocBookkeeping bookkeepingService = &assocBookkeepingService{}

//TODO in future, platform calls will be stubbed
var sys system = &systemImp{}

// bookkeepingService represents the dependency for docmanager
type bookkeepingService interface {
	DeleteOldDocumentFolderLogs(log log.T, instanceID, orchestrationRootDirName string, retentionDurationHours int, isIntendedFileNameFormat func(string) bool, formOrchestrationFolderName func(string) string)
}

type assocBookkeepingService struct{}

func (assocBookkeepingService) DeleteOldDocumentFolderLogs(log log.T, instanceID, orchestrationRootDirName string, retentionDurationHours int, isIntendedFileNameFormat func(string) bool, formOrchestrationFolderName func(string) string) {
	docmanager.DeleteOldDocumentFolderLogs(log, instanceID, orchestrationRootDirName, retentionDurationHours, isIntendedFileNameFormat, formOrchestrationFolderName)
}

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

// parserService represents the dependency for association parser
type parserService interface {
	ParseDocumentForPayload(log log.T, rawData *model.InstanceAssociation) (*messageContract.SendCommandPayload, error)
	InitializeDocumentState(context context.T,
		payload *messageContract.SendCommandPayload,
		rawData *model.InstanceAssociation) (docModel.DocumentState, error)
}

type assocParserService struct{}

// ParseDocumentWithParams wraps parser ParseDocumentWithParams
func (assocParserService) ParseDocumentForPayload(
	log log.T,
	rawData *model.InstanceAssociation) (*messageContract.SendCommandPayload, error) {

	return parser.ParseDocumentForPayload(log, rawData)
}

// InitializeDocumentState wraps engine InitializeCommandState
func (assocParserService) InitializeDocumentState(context context.T,
	payload *messageContract.SendCommandPayload,
	rawData *model.InstanceAssociation) (docModel.DocumentState, error) {

	return parser.InitializeDocumentState(context, payload, rawData)
}
