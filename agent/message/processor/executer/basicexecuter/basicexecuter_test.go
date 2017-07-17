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

// Package executer provides interfaces as document execution logic
package basicexecuter

import (
	"testing"

	"github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/aws/amazon-ssm-agent/agent/contracts"
	docModel "github.com/aws/amazon-ssm-agent/agent/docmanager/model"
	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/amazon-ssm-agent/agent/message/processor/executer"
	executermock "github.com/aws/amazon-ssm-agent/agent/message/processor/executer/mock"
	"github.com/aws/amazon-ssm-agent/agent/task"
	"github.com/stretchr/testify/assert"
)

var logger = log.NewMockLog()

type TestCase struct {
	// Msg stores a parsed MDS message as received from GetMessages.
	MsgId string

	// DocState stores parsed Document State
	DocState docModel.DocumentState

	// PluginResults stores the (unmarshalled) results that the plugins are expected to produce.
	PluginResults map[string]*contracts.PluginResult
}

// TestBasicExecuter test the execution of a given document
// with the correct response.
func TestBasicExecuter(t *testing.T) {

	docInfo := docModel.DocumentInfo{
		CreatedDate:  "2017-06-10T01-23-07.853Z",
		CommandID:    "13e8e6ad-e195-4ccb-86ee-328153b0dafe",
		DocumentName: "AWS-RunPowerShellScript",
		InstanceID:   "i-400e1090",
		RunCount:     0,
	}

	pluginState := docModel.PluginState{
		Name: "aws:runScript",
		Id:   "aws:runScript",
	}
	docState := docModel.DocumentState{
		DocumentInformation:        docInfo,
		DocumentType:               "SendCommand",
		InstancePluginsInformation: []docModel.PluginState{pluginState},
	}

	result := contracts.PluginResult{
		PluginName: "aws:runScript",
		Status:     contracts.ResultStatusSuccess,
	}
	results := make(map[string]*contracts.PluginResult)
	results[pluginState.Id] = &result

	//form test case
	testCase := TestCase{
		MsgId:         "aws.ssm.13e8e6ad-e195-4ccb-86ee-328153b0dafe.i-400e1090",
		DocState:      docState,
		PluginResults: results,
	}
	testBasicExecuter(t, testCase)
}

func testBasicExecuter(t *testing.T, testCase TestCase) {

	cancelFlag := task.NewChanneledCancelFlag()
	nPlugins := len(testCase.DocState.InstancePluginsInformation)

	// call method under test
	//orchestrationRootDir is set to empty such that it can meet the test expectation.
	e := NewBasicExecuter(context.NewMockDefault())

	nStatusReceived := 0

	state := testCase.DocState
	dataStoreMock := executermock.MockDocumentStore{}
	dataStoreMock.On("Load").Return(&state)
	dataStoreMock.On("Save").Return()
	pluginRunner = func(context context.T,
		docStore executer.DocumentStore,
		resChan chan contracts.PluginResult,
		cancelFlag task.CancelFlag) {
		assert.Equal(t, docStore, dataStoreMock)
		for _, pluginState := range testCase.DocState.InstancePluginsInformation {
			resChan <- *testCase.PluginResults[pluginState.Id]
		}
		docStore.Save()
		close(resChan)
	}
	resChan := e.Run(cancelFlag, dataStoreMock)

	for res := range resChan {
		nStatusReceived++
		assert.Equal(t, res, *testCase.PluginResults[res.PluginName])

	}
	assert.Equal(t, nStatusReceived, nPlugins)

	dataStoreMock.AssertExpectations(t)

}
