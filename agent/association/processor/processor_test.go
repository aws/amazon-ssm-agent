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
	"time"

	"github.com/aws/amazon-ssm-agent/agent/association/model"
	"github.com/aws/amazon-ssm-agent/agent/association/schedulemanager"
	"github.com/aws/amazon-ssm-agent/agent/association/service"
	complianceUploader "github.com/aws/amazon-ssm-agent/agent/compliance/uploader"
	"github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/aws/amazon-ssm-agent/agent/contracts"
	processormock "github.com/aws/amazon-ssm-agent/agent/framework/processor/mock"
	"github.com/aws/amazon-ssm-agent/agent/log"
	messageContracts "github.com/aws/amazon-ssm-agent/agent/runcommand/contracts"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ssm"
	"github.com/carlescere/scheduler"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

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
	complianceUploader := complianceUploader.NewMockDefault()

	processor.assocSvc = svcMock
	processor.complianceUploader = complianceUploader

	svcMock.On("CreateNewServiceIfUnHealthy", mock.AnythingOfType("*log.Mock"))
	svcMock.On(
		"ListInstanceAssociations",
		mock.AnythingOfType("*log.Mock"),
		mock.AnythingOfType("string")).Return(assocRawData, errors.New("unable to load association"))
	svcMock.On(
		"LoadAssociationDetail",
		mock.AnythingOfType("*log.Mock"),
		mock.AnythingOfType("*model.InstanceAssociation")).Return(nil)
	complianceUploader.On("CreateNewServiceIfUnHealthy", mock.AnythingOfType("*log.Mock"))

	processor.ProcessAssociation()

	assert.True(t, complianceUploader.AssertNumberOfCalls(t, "CreateNewServiceIfUnHealthy", 1))
	assert.True(t, svcMock.AssertNumberOfCalls(t, "CreateNewServiceIfUnHealthy", 1))
	assert.True(t, svcMock.AssertNumberOfCalls(t, "ListInstanceAssociations", 1))
	assert.True(t, svcMock.AssertNumberOfCalls(t, "LoadAssociationDetail", 0))
}

func TestProcessAssociationUnableToLoadAssociationDetail(t *testing.T) {
	processor := createProcessor()
	svcMock := service.NewMockDefault()
	assocRawData := createAssociationRawData()
	parserMock := parserMock{}
	sys = &systemStub{}

	complianceUploader := complianceUploader.NewMockDefault()

	// Arrange
	processor.assocSvc = svcMock
	processor.complianceUploader = complianceUploader
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
		mock.AnythingOfType("*model.InstanceAssociation")).Return(errors.New("unable to load detail"))
	svcMock.On(
		"UpdateInstanceAssociationStatus",
		mock.AnythingOfType("*log.Mock"),
		mock.AnythingOfType("string"),
		mock.AnythingOfType("string"),
		mock.AnythingOfType("string"),
		mock.AnythingOfType("*ssm.InstanceAssociationExecutionResult"))
	complianceUploader.On("CreateNewServiceIfUnHealthy", mock.AnythingOfType("*log.Mock"))
	complianceUploader.On(
		"UpdateAssociationCompliance",
		mock.AnythingOfType("string"),
		mock.AnythingOfType("string"),
		mock.AnythingOfType("string"),
		mock.AnythingOfType("string"),
		mock.AnythingOfType("string"),
		mock.AnythingOfType("time.Time")).Return(nil)

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
	complianceUploader := complianceUploader.NewMockDefault()

	parserMock := parserMock{}

	// Arrange
	processor.assocSvc = svcMock
	assocParser = &parserMock
	processor.complianceUploader = complianceUploader

	// Mock service
	mockService(svcMock, assocRawData, &output)

	// Mock processor
	processorMock := &processormock.MockedProcessor{}
	processor.proc = processorMock
	ch := make(chan contracts.DocumentResult)
	processorMock.On("Start").Return(ch, nil)
	processorMock.On("InitialProcessing").Return(nil)

	complianceUploader.On("CreateNewServiceIfUnHealthy", mock.AnythingOfType("*log.Mock"))

	// Act
	processor.InitializeAssociationProcessor()
	processor.ProcessAssociation()
	close(ch)
	// Assert
	assert.True(t, svcMock.AssertNumberOfCalls(t, "CreateNewServiceIfUnHealthy", 1))
	assert.True(t, svcMock.AssertNumberOfCalls(t, "ListInstanceAssociations", 1))
	assert.True(t, svcMock.AssertNumberOfCalls(t, "LoadAssociationDetail", 1))
}

func mockService(svcMock *service.AssociationServiceMock, assocRawData []*model.InstanceAssociation, output *ssm.UpdateInstanceAssociationStatusOutput) {
	svcMock.On("CreateNewServiceIfUnHealthy", mock.AnythingOfType("*log.Mock"))
	svcMock.On(
		"ListInstanceAssociations",
		mock.AnythingOfType("*log.Mock"),
		mock.AnythingOfType("string")).Return(assocRawData, nil)
	svcMock.On(
		"LoadAssociationDetail",
		mock.AnythingOfType("*log.Mock"),
		mock.AnythingOfType("*model.InstanceAssociation")).Return(nil)
	svcMock.On(
		"UpdateInstanceAssociationStatus",
		mock.AnythingOfType("*log.Mock"),
		mock.AnythingOfType("string"),
		mock.AnythingOfType("string"),
		mock.AnythingOfType("string"),
		mock.AnythingOfType("string"),
		mock.AnythingOfType("*ssm.InstanceAssociationExecutionResult"))
}

func TestProcessAssociationSuccessful(t *testing.T) {
	processor := createProcessor()
	svcMock := service.NewMockDefault()
	assocRawData := createAssociationRawData()
	output := ssm.UpdateInstanceAssociationStatusOutput{}
	sys = &systemStub{}

	payload := messageContracts.SendCommandPayload{}
	docState := contracts.DocumentState{}
	parserMock := parserMock{}
	complianceUploader := complianceUploader.NewMockDefault()

	processorMock := &processormock.MockedProcessor{}
	// Arrange
	processor.assocSvc = svcMock
	processor.proc = processorMock
	assocParser = &parserMock
	processor.complianceUploader = complianceUploader

	// Mock service
	mockService(svcMock, assocRawData, &output)

	// Mock parser
	mockParser(&parserMock, &payload, docState)

	// Mock processor
	ch := make(chan contracts.DocumentResult)
	processorMock.On("Start").Return(ch, nil)
	processorMock.On("InitialProcessing").Return(nil)
	complianceUploader.On("CreateNewServiceIfUnHealthy", mock.AnythingOfType("*log.Mock"))

	// Act
	processor.InitializeAssociationProcessor()
	processor.ProcessAssociation()
	//make sure the processor is invoked as expected
	close(ch)
	processorMock.AssertExpectations(t)
	// Assert
	assert.True(t, svcMock.AssertNumberOfCalls(t, "CreateNewServiceIfUnHealthy", 1))
	assert.True(t, svcMock.AssertNumberOfCalls(t, "ListInstanceAssociations", 1))
	assert.True(t, svcMock.AssertNumberOfCalls(t, "LoadAssociationDetail", 1))
	assert.True(t, svcMock.AssertNumberOfCalls(t, "UpdateInstanceAssociationStatus", 0))
	assert.True(t, complianceUploader.AssertNumberOfCalls(t, "UpdateAssociationCompliance", 0))
}

//make sure this operation is thread safe
func TestUpdatePluginAssociationInstances(t *testing.T) {
	testAssociationID := "testAssociationID"
	testName := "testName"
	testAssociations := []*model.InstanceAssociation{
		&model.InstanceAssociation{
			Association: &ssm.InstanceAssociationSummary{
				AssociationId: &testAssociationID,
				Name:          &testName,
			},
		},
	}
	schedulemanager.Refresh(log.NewMockLog(), testAssociations)
	assert.Equal(t, len(pluginAssociationInstances), 0)
	testDocState := contracts.DocumentState{
		InstancePluginsInformation: []contracts.PluginState{
			contracts.PluginState{
				Name: "pluginName",
			},
		},
	}
	updatePluginAssociationInstances("testAssociationID", &testDocState)
	assert.EqualValues(t, AssocList{testAssociationID}, testDocState.InstancePluginsInformation[0].Configuration.CurrentAssociations)
	resultMap := make(map[string]AssocList)
	resultMap["pluginName"] = []string{testAssociationID}
	assert.Equal(t, resultMap, pluginAssociationInstances)
}

func TestRemovePluginAssociationInstances(t *testing.T) {
	testAssociationID := "testAssociationID"
	testRemovedAssociationID := "removedID"
	testName := "testName"
	testAssociations := []*model.InstanceAssociation{
		&model.InstanceAssociation{
			Association: &ssm.InstanceAssociationSummary{
				AssociationId: &testAssociationID,
				Name:          &testName,
			},
		},
	}
	schedulemanager.Refresh(log.NewMockLog(), testAssociations)
	pluginAssociationInstances["pluginName"] = AssocList{testAssociationID, testRemovedAssociationID}
	assert.Equal(t, len(pluginAssociationInstances), 1)
	testDocState := contracts.DocumentState{
		InstancePluginsInformation: []contracts.PluginState{
			contracts.PluginState{
				Name: "pluginName",
			},
		},
	}
	updatePluginAssociationInstances("testAssociationID", &testDocState)
	assert.EqualValues(t, AssocList{testAssociationID}, testDocState.InstancePluginsInformation[0].Configuration.CurrentAssociations)
	resultMap := make(map[string]AssocList)
	resultMap["pluginName"] = []string{testAssociationID}
	assert.Equal(t, resultMap, pluginAssociationInstances)
}

func mockParser(parserMock *parserMock, payload *messageContracts.SendCommandPayload, docState contracts.DocumentState) {
	parserMock.On(
		"InitializeDocumentState",
		mock.AnythingOfType("*context.Mock"),
		mock.AnythingOfType("*contracts.SendCommandPayload"),
		mock.AnythingOfType("*model.InstanceAssociation")).Return(docState)
}

func createProcessor() *Processor {
	processor := Processor{}
	processor.context = context.NewMockDefault()
	return &processor
}

func createAssociationRawData() []*model.InstanceAssociation {
	association := ssm.InstanceAssociationSummary{
		Name:               aws.String("Test-Association"),
		DocumentVersion:    aws.String("1"),
		AssociationId:      aws.String("Id-Test"),
		InstanceId:         aws.String("test-association-id"),
		Checksum:           aws.String("checksum"),
		LastExecutionDate:  aws.Time(time.Now().UTC()),
		ScheduleExpression: aws.String("cron(0 0/5 * 1/1 * ? *)"),
	}
	assocRawData := model.InstanceAssociation{
		Association: &association,
	}

	return []*model.InstanceAssociation{&assocRawData}
}
