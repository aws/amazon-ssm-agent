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

	"github.com/aws/aws-sdk-go-v2/service/ssm/types"

	processor2 "github.com/aws/amazon-ssm-agent/agent/association/mocks/processor"
	"github.com/aws/amazon-ssm-agent/agent/association/mocks/service"
	complianceUploader "github.com/aws/amazon-ssm-agent/agent/association/mocks/uploader"
	"github.com/aws/amazon-ssm-agent/agent/association/model"
	"github.com/aws/amazon-ssm-agent/agent/association/schedulemanager"
	"github.com/aws/amazon-ssm-agent/agent/contracts"
	processormock "github.com/aws/amazon-ssm-agent/agent/framework/processor/mock"
	"github.com/aws/amazon-ssm-agent/agent/mocks/context"
	"github.com/aws/amazon-ssm-agent/agent/mocks/log"
	messageContracts "github.com/aws/amazon-ssm-agent/agent/runcommand/contracts"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
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
	complianceUploader := complianceUploader.NewMockDefault()

	processor.assocSvc = svcMock
	processor.complianceUploader = complianceUploader

	svcMock.On("CreateNewServiceIfUnHealthy", mock.AnythingOfType("*context.Mock"))
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
	parserMock := processor2.ParserMock{}

	complianceUploader := complianceUploader.NewMockDefault()

	// Arrange
	processor.assocSvc = svcMock
	processor.complianceUploader = complianceUploader
	assocParser = &parserMock

	// Mock service
	svcMock.On("CreateNewServiceIfUnHealthy", mock.AnythingOfType("*context.Mock"))
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
		mock.AnythingOfType("string"),
		mock.AnythingOfType("string"),
		mock.AnythingOfType("string"),
		mock.AnythingOfType("string"),
		mock.AnythingOfType("string"))
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
	complianceUploader := complianceUploader.NewMockDefault()

	parserMock := processor2.ParserMock{}

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
	processorMock.On("InitialProcessing", false).Return(nil)

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
	svcMock.On("CreateNewServiceIfUnHealthy", mock.AnythingOfType("*context.Mock"))
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
		mock.AnythingOfType("string"),
		mock.AnythingOfType("string"),
		mock.AnythingOfType("string"),
		mock.AnythingOfType("string"))
}

func TestProcessAssociationSuccessful(t *testing.T) {
	processor := createProcessor()
	svcMock := service.NewMockDefault()
	assocRawData := createAssociationRawData()
	output := ssm.UpdateInstanceAssociationStatusOutput{}

	payload := messageContracts.SendCommandPayload{}
	docState := contracts.DocumentState{}
	parserMock := processor2.ParserMock{}
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
	processorMock.On("InitialProcessing", false).Return(nil)
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

// make sure this operation is thread safe
func TestUpdatePluginAssociationInstances(t *testing.T) {
	testAssociationID := "testAssociationID"
	testName := "testName"
	testAssociations := []*model.InstanceAssociation{
		&model.InstanceAssociation{
			Association: &types.InstanceAssociationSummary{
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
			Association: &types.InstanceAssociationSummary{
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

func mockParser(parserMock *processor2.ParserMock, payload *messageContracts.SendCommandPayload, docState contracts.DocumentState) {
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
	association := types.InstanceAssociationSummary{
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

func TestIsAssociationTimedOut_TrackedStartTimeNotExpired(t *testing.T) {
	// Association just started (tracked start time < 2h ago) → not timed out
	assocID := "test-assoc-1"
	testAssociations := []*model.InstanceAssociation{
		{
			Association: &types.InstanceAssociationSummary{
				AssociationId: aws.String(assocID),
				Name:          aws.String("Test"),
				InstanceId:    aws.String("i-123"),
			},
		},
	}
	schedulemanager.Refresh(log.NewMockLog(), testAssociations)
	// Mark InProgress (this records the start time as now)
	schedulemanager.UpdateAssociationStatus(assocID, contracts.AssociationStatusInProgress)

	result := isAssociationTimedOut(testAssociations[0])
	assert.False(t, result)
}

func TestIsAssociationTimedOut_NoTrackedStartTimeFallbackToLastExecution(t *testing.T) {
	// No tracked start time, fallback to LastExecutionDate behavior
	assocID := "test-assoc-3"
	oldTime := time.Now().UTC().Add(-3 * time.Hour)
	testAssociations := []*model.InstanceAssociation{
		{
			Association: &types.InstanceAssociationSummary{
				AssociationId:     aws.String(assocID),
				Name:              aws.String("Test"),
				InstanceId:        aws.String("i-123"),
				LastExecutionDate: aws.Time(oldTime),
			},
		},
	}
	// Refresh without marking InProgress — no tracked start time
	schedulemanager.Refresh(log.NewMockLog(), testAssociations)

	result := isAssociationTimedOut(testAssociations[0])
	assert.True(t, result)
}

func TestIsAssociationTimedOut_LastExecutionDateNil(t *testing.T) {
	// LastExecutionDate nil and no tracked start time → not timed out
	assocID := "test-assoc-4"
	testAssociations := []*model.InstanceAssociation{
		{
			Association: &types.InstanceAssociationSummary{
				AssociationId: aws.String(assocID),
				Name:          aws.String("Test"),
				InstanceId:    aws.String("i-123"),
			},
		},
	}
	schedulemanager.Refresh(log.NewMockLog(), testAssociations)

	result := isAssociationTimedOut(testAssociations[0])
	assert.False(t, result)
}

func TestIsAssociationTimedOut_StartTimePreservedAcrossRefresh(t *testing.T) {
	// Tracked start time must survive a Refresh cycle (every 10 min)
	assocID := "test-assoc-5"
	testAssociations := []*model.InstanceAssociation{
		{
			Association: &types.InstanceAssociationSummary{
				AssociationId: aws.String(assocID),
				Name:          aws.String("Test"),
				InstanceId:    aws.String("i-123"),
			},
		},
	}
	schedulemanager.Refresh(log.NewMockLog(), testAssociations)
	schedulemanager.UpdateAssociationStatus(assocID, contracts.AssociationStatusInProgress)

	// Simulate a 10-minute refresh cycle with the same association still present
	schedulemanager.Refresh(log.NewMockLog(), testAssociations)

	// Start time should be preserved — association should not be timed out
	result := isAssociationTimedOut(testAssociations[0])
	assert.False(t, result)

	// Verify the start time still exists
	_, exists := schedulemanager.GetInProgressStartTime(assocID)
	assert.True(t, exists)
}

func TestIsAssociationTimedOut_NewExecutionAtTwoHourBoundary(t *testing.T) {
	// A new execution starts exactly when LastExecutionDate is 2h old.
	// The timeout check must use the tracked start time (now), not LastExecutionDate.
	assocID := "repro-boundary-race"
	twoHoursAgo := time.Now().UTC().Add(-2 * time.Hour)
	testAssociations := []*model.InstanceAssociation{
		{
			Association: &types.InstanceAssociationSummary{
				AssociationId:     aws.String(assocID),
				Name:              aws.String("AWS-RunShellScript"),
				InstanceId:        aws.String("i-test"),
				LastExecutionDate: aws.Time(twoHoursAgo),
			},
		},
	}
	schedulemanager.Refresh(log.NewMockLog(), testAssociations)
	// Simulate: new execution just started → agent marks InProgress
	schedulemanager.UpdateAssociationStatus(assocID, contracts.AssociationStatusInProgress)

	// With the fix: should NOT be timed out (just started seconds ago)
	result := isAssociationTimedOut(testAssociations[0])
	assert.False(t, result, "BUG REPRODUCED: association falsely declared timed out even though it just started")
}

func TestIsAssociationTimedOut_OldBehaviorWouldFalsePositive(t *testing.T) {
	// Demonstrate that WITHOUT tracked start time, the old LastExecutionDate logic
	// would incorrectly declare timeout. This is the fallback path.
	assocID := "old-behavior-demo"
	twoHoursAgo := time.Now().UTC().Add(-2*time.Hour - time.Second)
	testAssociations := []*model.InstanceAssociation{
		{
			Association: &types.InstanceAssociationSummary{
				AssociationId:     aws.String(assocID),
				Name:              aws.String("AWS-RunShellScript"),
				InstanceId:        aws.String("i-test"),
				LastExecutionDate: aws.Time(twoHoursAgo),
			},
		},
	}
	// Refresh but do NOT mark InProgress → no tracked start time → fallback path
	schedulemanager.Refresh(log.NewMockLog(), testAssociations)

	// Fallback to LastExecutionDate: 2h+1s ago + 2h = 1s ago → before now → TIMED OUT
	// This confirms the old behavior still works as a fallback (e.g., after agent restart)
	result := isAssociationTimedOut(testAssociations[0])
	assert.True(t, result, "Fallback path should still detect genuinely stuck associations")
}
