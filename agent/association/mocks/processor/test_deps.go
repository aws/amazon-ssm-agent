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
	"github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/aws/amazon-ssm-agent/agent/contracts"
	"github.com/aws/amazon-ssm-agent/agent/log"
	messageContract "github.com/aws/amazon-ssm-agent/agent/runcommand/contracts"
	"github.com/stretchr/testify/mock"
)

type BookkeepingMock struct {
	mock.Mock
}

// PersistData mocks implementation for PersistData
func (m *BookkeepingMock) PersistData(log log.T, commandID, instanceID, locationFolder string, object interface{}) {
	m.Called(log, commandID, instanceID, locationFolder, object)
}

type ParserMock struct {
	mock.Mock
}

// InitializeDocumentState mocks implementation for InitializeDocumentState
func (m *ParserMock) ParseDocumentForPayload(
	log log.T, rawData *model.InstanceAssociation) (*messageContract.SendCommandPayload, error) {

	args := m.Called(log, rawData)

	return args.Get(0).(*messageContract.SendCommandPayload), args.Error(1)
}

// InitializeDocumentState mocks implementation for InitializeDocumentState
func (m *ParserMock) InitializeDocumentState(
	context context.T,
	payload *messageContract.SendCommandPayload,
	rawData *model.InstanceAssociation) (contracts.DocumentState, error) {

	args := m.Called(context, payload, rawData)

	return args.Get(0).(contracts.DocumentState), args.Error(1)
}
