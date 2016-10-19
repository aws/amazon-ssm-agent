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
	"errors"
	"testing"

	"github.com/aws/amazon-ssm-agent/agent/appconfig"
	"github.com/aws/amazon-ssm-agent/agent/association/executer"
	"github.com/aws/amazon-ssm-agent/agent/association/model"
	"github.com/aws/amazon-ssm-agent/agent/association/service"
	"github.com/aws/amazon-ssm-agent/agent/association/taskpool"
	"github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/aws/amazon-ssm-agent/agent/log"
	messageContracts "github.com/aws/amazon-ssm-agent/agent/message/contracts"
	stateModel "github.com/aws/amazon-ssm-agent/agent/statemanager/model"
	"github.com/aws/amazon-ssm-agent/agent/task"
	"github.com/aws/aws-sdk-go/service/ssm"
	"github.com/carlescere/scheduler"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestNewAssociationProcessor(t *testing.T) {
	log := log.Logger()
	context := context.Default(log, appconfig.SsmagentConfig{})
	process := NewAssociationProcessor(context, "i-test")

	assert.NotNil(t, process)
}

func TestSetJob(t *testing.T) {
	processor := Processor{}
	job := scheduler.Job{}

	processor.SetPollJob(&job)

	assert.NotNil(t, processor.pollJob)
	assert.Equal(t, processor.pollJob, &job)
}

func TestProcessAssociationUnableToGetAssociation(t *testing.T) {
	processor := createProcessor()
	svcMock := service.NewMockDefault()
	assocRawData := createAssociationRawData()
	sys = &systemStub{}

	processor.assocSvc = svcMock

	svcMock.On("CreateNewServiceIfUnHealthy", mock.AnythingOfType("*log.Mock"))
	svcMock.On(
		"ListInstanceAssociations",
		mock.AnythingOfType("*log.Mock"),
		mock.AnythingOfType("string")).Return(assocRawData, errors.New("unable to load association"))
	svcMock.On(
		"LoadAssociationDetail",
		mock.AnythingOfType("*log.Mock"),
		mock.AnythingOfType("*model.AssociationRawData")).Return(nil)

	processor.ProcessAssociation()

	assert.True(t, svcMock.AssertNumberOfCalls(t, "CreateNewServiceIfUnHealthy", 1))
	assert.True(t, svcMock.AssertNumberOfCalls(t, "ListInstanceAssociations", 1))
	assert.True(t, svcMock.AssertNumberOfCalls(t, "LoadAssociationDetail", 0))
}

func TestProcessAssociationExecutePendingDocument(t *testing.T) {
	processor := createProcessor()
	docState := stateModel.DocumentState{}
	executerMock := executer.DocumentExecuterMock{}
	sys = &systemStub{}

	processor.executer = &executerMock

	executerMock.On(
		"ExecutePendingDocument",
		mock.AnythingOfType("*context.Mock"),
		mock.AnythingOfType("taskpool.Manager"),
		mock.AnythingOfType("*model.DocumentState")).Return(nil)

	processor.ExecutePendingDocument(&docState)

	assert.True(t, executerMock.AssertNumberOfCalls(t, "ExecutePendingDocument", 1))
}

func TestProcessAssociationExecuteInProgressDocument(t *testing.T) {
	processor := createProcessor()
	docState := stateModel.DocumentState{}
	cancelFlag := task.ChanneledCancelFlag{}
	executerMock := executer.DocumentExecuterMock{}
	sys = &systemStub{}

	processor.executer = &executerMock

	executerMock.On(
		"ExecuteInProgressDocument",
		mock.AnythingOfType("*context.Mock"),
		mock.AnythingOfType("*model.DocumentState"),
		mock.AnythingOfType("task.ChanneledCancelFlag"))

	processor.ExecuteInProgressDocument(&docState, &cancelFlag)
}

func TestProcessAssociationUnableToLoadAssociationDetail(t *testing.T) {
	processor := createProcessor()
	svcMock := service.NewMockDefault()
	assocRawData := createAssociationRawData()
	output := ssm.UpdateInstanceAssociationStatusOutput{}
	parserMock := parserMock{}
	sys = &systemStub{}

	// Arrange
	processor.assocSvc = svcMock
	assocParser = &parserMock

	// Mock service
	svcMock.On("CreateNewServiceIfUnHealthy", mock.AnythingOfType("*log.Mock"))
	svcMock.On(
		"ListInstanceAssociations",
		mock.AnythingOfType("*log.Mock"),
		mock.AnythingOfType("string")).Return(assocRawData, nil)
	svcMock.On(
		"LoadAssociationDetail",
		mock.AnythingOfType("*log.Mock"),
		mock.AnythingOfType("*model.AssociationRawData")).Return(errors.New("unable to load detail"))
	svcMock.On(
		"UpdateInstanceAssociationStatus",
		mock.AnythingOfType("*log.Mock"),
		mock.AnythingOfType("string"),
		mock.AnythingOfType("string"),
		mock.AnythingOfType("*ssm.InstanceAssociationExecutionResult")).Return(&output, nil)

	// Act
	processor.ProcessAssociation()

	// Assert
	assert.True(t, svcMock.AssertNumberOfCalls(t, "CreateNewServiceIfUnHealthy", 1))
	assert.True(t, svcMock.AssertNumberOfCalls(t, "ListInstanceAssociations", 1))
	assert.True(t, svcMock.AssertNumberOfCalls(t, "LoadAssociationDetail", 1))
	assert.True(t, svcMock.AssertNumberOfCalls(t, "UpdateInstanceAssociationStatus", 1))
}

func TestProcessAssociationUnableToParseAssociation(t *testing.T) {
	processor := createProcessor()
	svcMock := service.NewMockDefault()
	assocRawData := createAssociationRawData()
	output := ssm.UpdateInstanceAssociationStatusOutput{}
	sys = &systemStub{}

	payload := messageContracts.SendCommandPayload{}
	parserMock := parserMock{}

	// Arrange
	processor.assocSvc = svcMock
	assocParser = &parserMock

	// Mock service
	mockService(svcMock, assocRawData, &output)

	// Mock parser
	parserMock.On(
		"ParseDocumentWithParams",
		mock.AnythingOfType("*log.Mock"),
		mock.AnythingOfType("*model.AssociationRawData")).Return(&payload, errors.New("failed to parse data"))

	// Act
	processor.ProcessAssociation()

	// Assert
	assert.True(t, svcMock.AssertNumberOfCalls(t, "CreateNewServiceIfUnHealthy", 1))
	assert.True(t, svcMock.AssertNumberOfCalls(t, "ListInstanceAssociations", 1))
	assert.True(t, svcMock.AssertNumberOfCalls(t, "LoadAssociationDetail", 1))
	assert.True(t, parserMock.AssertNumberOfCalls(t, "ParseDocumentWithParams", 1))
	assert.True(t, svcMock.AssertNumberOfCalls(t, "UpdateInstanceAssociationStatus", 1))
}

func mockService(svcMock *service.AssociationServiceMock, assocRawData []*model.AssociationRawData, output *ssm.UpdateInstanceAssociationStatusOutput) {
	svcMock.On("CreateNewServiceIfUnHealthy", mock.AnythingOfType("*log.Mock"))
	svcMock.On(
		"ListInstanceAssociations",
		mock.AnythingOfType("*log.Mock"),
		mock.AnythingOfType("string")).Return(assocRawData, nil)
	svcMock.On(
		"LoadAssociationDetail",
		mock.AnythingOfType("*log.Mock"),
		mock.AnythingOfType("*model.AssociationRawData")).Return(nil)
	svcMock.On(
		"UpdateInstanceAssociationStatus",
		mock.AnythingOfType("*log.Mock"),
		mock.AnythingOfType("string"),
		mock.AnythingOfType("string"),
		mock.AnythingOfType("*ssm.InstanceAssociationExecutionResult")).Return(output, nil)
}

func TestProcessAssociationUnableToExecutePendingDocument(t *testing.T) {
	processor := createProcessor()
	svcMock := service.NewMockDefault()
	assocRawData := createAssociationRawData()
	output := ssm.UpdateInstanceAssociationStatusOutput{}
	sys = &systemStub{}

	payload := messageContracts.SendCommandPayload{}
	docState := stateModel.DocumentState{}
	parserMock := parserMock{}
	executerMock := executer.DocumentExecuterMock{}

	// Arrange
	processor.assocSvc = svcMock
	processor.executer = &executerMock
	assocParser = &parserMock

	// Mock service
	mockService(svcMock, assocRawData, &output)

	// Mock parser
	mockParser(&parserMock, &payload, docState)

	// Mock executer
	executerMock.On(
		"ExecutePendingDocument",
		mock.AnythingOfType("*context.Mock"),
		mock.AnythingOfType("taskpool.Manager"),
		mock.AnythingOfType("*model.DocumentState")).Return(errors.New("failed to execute document"))

	// Act
	processor.ProcessAssociation()

	// Assert
	assert.True(t, svcMock.AssertNumberOfCalls(t, "CreateNewServiceIfUnHealthy", 1))
	assert.True(t, svcMock.AssertNumberOfCalls(t, "ListInstanceAssociations", 1))
	assert.True(t, svcMock.AssertNumberOfCalls(t, "LoadAssociationDetail", 1))
	assert.True(t, parserMock.AssertNumberOfCalls(t, "ParseDocumentWithParams", 1))
	assert.True(t, parserMock.AssertNumberOfCalls(t, "InitializeDocumentState", 1))
	assert.True(t, svcMock.AssertNumberOfCalls(t, "UpdateInstanceAssociationStatus", 1))
	assert.True(t, executerMock.AssertNumberOfCalls(t, "ExecutePendingDocument", 1))
}

func mockParser(parserMock *parserMock, payload *messageContracts.SendCommandPayload, docState stateModel.DocumentState) {
	parserMock.On(
		"ParseDocumentWithParams",
		mock.AnythingOfType("*log.Mock"),
		mock.AnythingOfType("*model.AssociationRawData")).Return(payload, nil)
	parserMock.On(
		"InitializeDocumentState",
		mock.AnythingOfType("*context.Mock"),
		mock.AnythingOfType("*model.SendCommandPayload"),
		mock.AnythingOfType("*model.AssociationRawData")).Return(docState)
}

func TestProcessAssociationSuccessful(t *testing.T) {
	processor := createProcessor()
	svcMock := service.NewMockDefault()
	assocRawData := createAssociationRawData()
	output := ssm.UpdateInstanceAssociationStatusOutput{}
	sys = &systemStub{}

	payload := messageContracts.SendCommandPayload{}
	docState := stateModel.DocumentState{}
	parserMock := parserMock{}
	executerMock := executer.DocumentExecuterMock{}

	// Arrange
	processor.assocSvc = svcMock
	processor.executer = &executerMock
	assocParser = &parserMock

	// Mock service
	mockService(svcMock, assocRawData, &output)

	// Mock parser
	mockParser(&parserMock, &payload, docState)

	// Mock executer
	executerMock.On(
		"ExecutePendingDocument",
		mock.AnythingOfType("*context.Mock"),
		mock.AnythingOfType("taskpool.Manager"),
		mock.AnythingOfType("*model.DocumentState")).Return(nil)

	// Act
	processor.ProcessAssociation()

	// Assert
	assert.True(t, svcMock.AssertNumberOfCalls(t, "CreateNewServiceIfUnHealthy", 1))
	assert.True(t, svcMock.AssertNumberOfCalls(t, "ListInstanceAssociations", 1))
	assert.True(t, svcMock.AssertNumberOfCalls(t, "LoadAssociationDetail", 1))
	assert.True(t, parserMock.AssertNumberOfCalls(t, "ParseDocumentWithParams", 1))
	assert.True(t, parserMock.AssertNumberOfCalls(t, "InitializeDocumentState", 1))
	assert.True(t, svcMock.AssertNumberOfCalls(t, "UpdateInstanceAssociationStatus", 0))
	assert.True(t, executerMock.AssertNumberOfCalls(t, "ExecutePendingDocument", 1))
}

func createProcessor() *Processor {
	processor := Processor{}
	processor.context = context.NewMockDefault()
	processor.taskPool = taskpool.Manager{}
	processor.stopSignal = make(chan bool)

	return &processor
}

func createAssociationRawData() []*model.AssociationRawData {
	name := "Test-Association"
	istanceID := "Id-Test"
	associationID := "test-association-id"
	association := ssm.InstanceAssociationSummary{
		Name:          &name,
		AssociationId: &associationID,
		InstanceId:    &istanceID,
	}
	assocRawData := model.AssociationRawData{
		Association: &association,
	}

	return []*model.AssociationRawData{&assocRawData}
}
