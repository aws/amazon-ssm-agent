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
	message "github.com/aws/amazon-ssm-agent/agent/message/contracts"
	"github.com/aws/amazon-ssm-agent/agent/statemanager"
)

var assocParser parserService = &assocParserService{}
var assocBookkeeping bookkeepingService = &assocBookkeepingService{}

// bookkeepingService represents the dependency for statemanager
type bookkeepingService interface {
	PersistData(log log.T, commandID, instanceID, locationFolder string, object interface{})
}

type assocBookkeepingService struct{}

// PersistData wraps statemanager PersistData
func (assocBookkeepingService) PersistData(log log.T, commandID, instanceID, locationFolder string, object interface{}) {
	statemanager.PersistData(log, commandID, instanceID, locationFolder, object)
}

// parserService represents the dependency for association parser
type parserService interface {
	ParseDocumentWithParams(log log.T, rawData *model.AssociationRawData) (*message.SendCommandPayload, error)
	InitializeDocumentState(context context.T,
		payload *message.SendCommandPayload,
		rawData *model.AssociationRawData) message.DocumentState
}

type assocParserService struct{}

// ParseDocumentWithParams wraps parser ParseDocumentWithParams
func (assocParserService) ParseDocumentWithParams(
	log log.T,
	rawData *model.AssociationRawData) (*message.SendCommandPayload, error) {

	return parser.ParseDocumentWithParams(log, rawData)
}

// InitializeDocumentState wraps engine InitializeCommandState
func (assocParserService) InitializeDocumentState(context context.T,
	payload *message.SendCommandPayload,
	rawData *model.AssociationRawData) message.DocumentState {

	return parser.InitializeCommandState(context, payload, rawData)
}
