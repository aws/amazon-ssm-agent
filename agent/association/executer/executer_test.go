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
	"testing"

	"github.com/aws/amazon-ssm-agent/agent/appconfig"
	"github.com/aws/amazon-ssm-agent/agent/association/service"
	"github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/aws/amazon-ssm-agent/agent/contracts"
	"github.com/aws/amazon-ssm-agent/agent/docmanager/model"
	"github.com/aws/amazon-ssm-agent/agent/framework/processor/executer"
	"github.com/aws/amazon-ssm-agent/agent/framework/processor/executer/mock"
	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/amazon-ssm-agent/agent/task"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestOutputBuilderWithMultiplePlugins(t *testing.T) {
	results := make(map[string]*contracts.PluginRuntimeStatus)

	results["pluginA"] = &contracts.PluginRuntimeStatus{
		Status: contracts.ResultStatusPassedAndReboot,
	}
	results["pluginB"] = &contracts.PluginRuntimeStatus{
		Status: contracts.ResultStatusSuccess,
	}
	results["pluginC"] = &contracts.PluginRuntimeStatus{
		Status: contracts.ResultStatusFailed,
	}
	results["pluginD"] = &contracts.PluginRuntimeStatus{
		Status: contracts.ResultStatusSkipped,
	}

	output, _ := buildOutput(results, 5)

	fmt.Println(output)
	assert.NotNil(t, output)
	assert.Equal(t, output, "4 out of 5 plugins processed, 2 success, 1 failed, 0 timedout, 1 skipped")
}

func TestOutputBuilderWithSinglePlugin(t *testing.T) {
	results := make(map[string]*contracts.PluginRuntimeStatus)

	results["pluginA"] = &contracts.PluginRuntimeStatus{
		Status: contracts.ResultStatusFailed,
	}

	output, _ := buildOutput(results, 1)

	fmt.Println(output)
	assert.NotNil(t, output)
	assert.Equal(t, output, "1 out of 1 plugin processed, 0 success, 1 failed, 0 timedout, 0 skipped")
}

func TestOutputBuilderWithSinglePluginWithSkippedStatus(t *testing.T) {
	results := make(map[string]*contracts.PluginRuntimeStatus)

	results["pluginA"] = &contracts.PluginRuntimeStatus{
		Status: contracts.ResultStatusSkipped,
	}

	output, _ := buildOutput(results, 1)

	fmt.Println(output)
	assert.NotNil(t, output)
	assert.Equal(t, output, "1 out of 1 plugin processed, 0 success, 0 failed, 0 timedout, 1 skipped")
}

func TestAssociationExecuter_ExecuteInProgressDocument(t *testing.T) {
	svcMock := service.NewMockDefault()
	ctxMock := context.NewMockDefault()
	assocID := "testAssocID"
	instID := "i-400e1090"
	docID := "testDocID"
	assocName := "AWS-RunPowerShellScript"
	agentInfo := contracts.AgentInfo{
		Name: "test",
	}
	docInfo := model.DocumentInfo{
		CreatedDate:    "2017-06-10T01-23-07.853Z",
		CommandID:      "13e8e6ad-e195-4ccb-86ee-328153b0dafe",
		DocumentName:   assocName,
		InstanceID:     instID,
		AssociationID:  assocID,
		DocumentID:     docID,
		RunCount:       0,
		DocumentStatus: contracts.ResultStatusSuccess,
	}

	pluginState := model.PluginState{
		Name: "aws:runScript",
		Id:   "aws:runScript",
	}
	docState := model.DocumentState{
		DocumentInformation:        docInfo,
		DocumentType:               "SendCommand",
		InstancePluginsInformation: []model.PluginState{pluginState},
	}

	r := NewAssociationExecuter(svcMock, &agentInfo)
	cancelFlag := task.ChanneledCancelFlag{}
	executerMock := executermocks.NewMockExecuter()
	resChan := make(chan contracts.DocumentResult)
	executerMock.On("Run", &cancelFlag, mock.AnythingOfType("*executer.DocumentFileStore")).Return(resChan)
	executerCreator = func(ctx context.T) executer.Executer {
		return executerMock
	}
	bookkeepingMock := NewMockBookkeepingSvc()
	bookkeepingSvc = bookkeepingMock
	bookkeepingMock.On("MoveDocumentState", ctxMock.Log(), docID, instID, appconfig.DefaultLocationOfCurrent, appconfig.DefaultLocationOfCompleted).Return(nil)
	close(resChan)
	r.ExecuteInProgressDocument(ctxMock, &docState, &cancelFlag)
	executerMock.AssertExpectations(t)
	bookkeepingMock.AssertExpectations(t)
	svcMock.AssertExpectations(t)
}

type BookkeepingSvcMock struct {
	mock.Mock
}

func NewMockBookkeepingSvc() *BookkeepingSvcMock {
	return new(BookkeepingSvcMock)
}

func (m *BookkeepingSvcMock) GetDocumentInfo(log log.T, documentID, instanceID, locationFolder string) model.DocumentInfo {
	args := m.Called(log, documentID, instanceID, locationFolder)
	return args.Get(0).(model.DocumentInfo)
}

// PersistDocumentInfo wraps docmanager PersistDocumentInfo
func (m *BookkeepingSvcMock) PersistDocumentInfo(log log.T, docInfo model.DocumentInfo, documentID, instanceID, locationFolder string) {
	m.Called(log, docInfo, documentID, instanceID, locationFolder)
	return
}

// GetDocumentInterimState wraps the docmanager GetDocumentInterimState
func (m *BookkeepingSvcMock) GetDocumentInterimState(log log.T, documentID, instanceID, locationFolder string) model.DocumentState {
	args := m.Called(log, documentID, instanceID, locationFolder)
	return args.Get(0).(model.DocumentState)
}

// MoveDocumentState wraps docmanager MoveDocumentState
func (m *BookkeepingSvcMock) MoveDocumentState(log log.T, documentID, instanceID, srcLocationFolder, dstLocationFolder string) {
	m.Called(log, documentID, instanceID, srcLocationFolder, dstLocationFolder)
	return
}

func (m *BookkeepingSvcMock) DeleteOldDocumentFolderLogs(log log.T, instanceID, orchestrationRootDirName string, retentionDurationHours int, isIntendedFileNameFormat func(string) bool, formOrchestrationFolderName func(string) string) {
	//do not adverize you're called since we don't set expectations on this function
	return
}
