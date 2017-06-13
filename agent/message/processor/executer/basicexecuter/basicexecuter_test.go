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
	"github.com/aws/amazon-ssm-agent/agent/framework/runpluginutil"
	"github.com/aws/amazon-ssm-agent/agent/log"
	messageContracts "github.com/aws/amazon-ssm-agent/agent/message/contracts"
	"github.com/aws/amazon-ssm-agent/agent/task"
	"github.com/stretchr/testify/assert"
)

var loggers = log.NewMockLog()

type TestCase struct {
	// Msg stores a parsed MDS message as received from GetMessages.
	MsgId string

	// DocState stores parsed Document State
	DocState docModel.DocumentState

	// PluginResults stores the (unmarshalled) results that the plugins are expected to produce.
	PluginResults map[string]*contracts.PluginResult

	// ReplyPayload stores the message payload expected to be sent via SendReply (contains marshalled plugin results).
	ReplyPayload messageContracts.SendReplyPayload
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
		Status: contracts.ResultStatusSuccess,
	}
	results := make(map[string]*contracts.PluginResult)
	results[pluginState.Id] = &result
	payload := messageContracts.SendReplyPayload{
		DocumentStatus:      contracts.ResultStatusSuccess,
		DocumentTraceOutput: "output",
	}
	//form test case
	testCase := TestCase{
		MsgId:         "aws.ssm.13e8e6ad-e195-4ccb-86ee-328153b0dafe.i-400e1090",
		DocState:      docState,
		ReplyPayload:  payload,
		PluginResults: results,
	}
	testBasicExecuter(t, testCase)
}

func testBasicExecuter(t *testing.T, testCase TestCase) {

	cancelFlag := task.NewChanneledCancelFlag()

	//TODO replace this callback with go channel
	sendResponseCalled := false
	sendResponse := func(messageID string, pluginID string, results map[string]*contracts.PluginResult) {
		sendResponseCalled = true
	}
	buildReply := func(pluginID string, results map[string]*contracts.PluginResult) messageContracts.SendReplyPayload {
		assert.Equal(t, results, testCase.PluginResults)
		return testCase.ReplyPayload
	}
	pluginRunner = func(context context.T,
		documentID string,
		plugins []docModel.PluginState,
		updateAssoc runpluginutil.UpdateAssociation,
		sendResponse runpluginutil.SendResponse,
		cancelFlag task.CancelFlag) (pluginOutputs map[string]*contracts.PluginResult) {
		return testCase.PluginResults
	}

	// call method under test
	//orchestrationRootDir is set to empty such that it can meet the test expectation.
	e := NewBasicExecuter()
	state := testCase.DocState
	e.Run(context.NewMockDefault(), cancelFlag, buildReply, nil, sendResponse, &state)

	// assert docState matched the testCase's reply payload
	assert.Equal(t, testCase.ReplyPayload.DocumentStatus, state.DocumentInformation.DocumentStatus)
	assert.Equal(t, testCase.ReplyPayload.RuntimeStatus, state.DocumentInformation.RuntimeStatus)
	assert.Equal(t, testCase.ReplyPayload.DocumentTraceOutput, state.DocumentInformation.DocumentTraceOutput)

	// assert sendReponse is called
	assert.True(t, sendResponseCalled)
}
