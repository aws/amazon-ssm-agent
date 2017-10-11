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
	executermock "github.com/aws/amazon-ssm-agent/agent/framework/processor/executer/mock"
	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/amazon-ssm-agent/agent/task"
	"github.com/stretchr/testify/assert"
)

var logger = log.NewMockLog()

type TestCase struct {
	// Msg stores a parsed MDS message as received from GetMessages.
	MsgId string

	// DocState stores parsed Document State
	DocState contracts.DocumentState

	// PluginResults stores the (unmarshalled) results that the plugins are expected to produce.
	PluginResults map[string]*contracts.PluginResult

	//Result
	ResultStatus contracts.ResultStatus
}

// TestBasicExecuter test the execution of a given document
// with the correct response.
func TestBasicExecuter(t *testing.T) {

	docInfo := contracts.DocumentInfo{
		CreatedDate:  "2017-06-10T01-23-07.853Z",
		CommandID:    "13e8e6ad-e195-4ccb-86ee-328153b0dafe",
		MessageID:    "MessageID",
		DocumentName: "AWS-RunPowerShellScript",
		InstanceID:   "i-400e1090",
		RunCount:     0,
	}

	pluginState := contracts.PluginState{
		Name: "aws:runScript",
		Id:   "plugin1",
	}
	docState := contracts.DocumentState{
		DocumentInformation:        docInfo,
		DocumentType:               "SendCommand",
		InstancePluginsInformation: []contracts.PluginState{pluginState},
	}

	result := contracts.PluginResult{
		PluginID:   "plugin1",
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
		ResultStatus:  contracts.ResultStatusSuccess,
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
	dataStoreMock := new(executermock.MockDocumentStore)
	resultState := state
	resultState.DocumentInformation.DocumentStatus = testCase.ResultStatus
	resultState.InstancePluginsInformation[0].Result = *testCase.PluginResults["plugin1"]
	dataStoreMock.On("Load").Return(state)
	dataStoreMock.On("Save", resultState).Return()
	pluginRunner = func(context context.T,
		docState contracts.DocumentState,
		resChan chan contracts.PluginResult,
		cancelFlag task.CancelFlag) map[string]*contracts.PluginResult {
		outputs := make(map[string]*contracts.PluginResult)
		for _, pluginState := range testCase.DocState.InstancePluginsInformation {
			resChan <- *testCase.PluginResults[pluginState.Id]
			outputs[pluginState.Id] = testCase.PluginResults[pluginState.Id]
		}
		return outputs
	}

	resChan := e.Run(cancelFlag, dataStoreMock)
	done := false
	for res := range resChan {
		if done {
			assert.FailNow(t, "response channel is not closed")
		}
		//receive overall results
		if res.LastPlugin == "" {
			//all individual plugin has finished execution
			assert.Equal(t, nStatusReceived, nPlugins)
			assert.Equal(t, res.Status, testCase.ResultStatus)
			assert.Equal(t, res.PluginResults, testCase.PluginResults)
			assert.Equal(t, "MessageID", res.MessageID)
			//assert channel close last
			done = true
			continue
		}
		curPlugin := testCase.DocState.InstancePluginsInformation[nStatusReceived].Id
		//assert plugin execution order
		assert.Equal(t, curPlugin, res.LastPlugin)
		//assert message id
		assert.Equal(t, "MessageID", res.MessageID)
		nStatusReceived++
		//Assert the number of plugins have been updated
		assert.Equal(t, len(res.PluginResults), nStatusReceived)
		//Assert the specific latest plugin is updated
		assert.Equal(t, res.PluginResults[res.LastPlugin], testCase.PluginResults[res.LastPlugin])
	}
	//assert transaction has completed
	assert.True(t, done)
	dataStoreMock.AssertExpectations(t)

}
