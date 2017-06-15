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

// package parser contains utilities for parsing and encoding MDS/SSM messages.
package reply

import (
	"encoding/json"
	"io/ioutil"

	"testing"

	"github.com/aws/amazon-ssm-agent/agent/contracts"
	"github.com/aws/amazon-ssm-agent/agent/docmanager/model"
	"github.com/aws/amazon-ssm-agent/agent/log"
	messageContracts "github.com/aws/amazon-ssm-agent/agent/message/contracts"
	"github.com/aws/amazon-ssm-agent/agent/times"
	"github.com/stretchr/testify/assert"
)

//TODO once service is moved out, merge all the reply tests here
var sampleMessageReplyFiles = []string{
	"./testdata/sampleReply.json",
}

var logger = log.NewMockLog()

func TestSendReplyBuilder_UpdatePluginResult(t *testing.T) {
	res := contracts.PluginResult{
		PluginName:     "pluginName",
		Output:         "output",
		Status:         contracts.ResultStatusSuccess,
		StandardOutput: "output",
		StandardError:  "error",
	}
	builder := NewSendReplyBuilder()
	builder.UpdatePluginResult(res)
	assert.Equal(t, len(builder.pluginResults), 1)
	assert.Equal(t, builder.pluginResults["pluginName"], &res)
}

func TestSendReplyBuilder_FormatPayload(t *testing.T) {

	for _, fileName := range sampleMessageReplyFiles {
		// load the test data
		sampleReply := loadMessageReplyFromFile(t, fileName)
		builder := NewSendReplyBuilder()
		for _, pluginRuntimeStatus := range sampleReply.RuntimeStatus {
			pluginResult := parsePluginResult(t, *pluginRuntimeStatus)
			builder.UpdatePluginResult(pluginResult)
		}
		// format the payload for document status update
		payload := builder.FormatPayload(logger, "", sampleReply.AdditionalInfo.Agent)
		// fix the date time
		payload.AdditionalInfo.DateTime = sampleReply.AdditionalInfo.DateTime
		assert.Equal(t, payload, sampleReply)
	}

}

func TestPrepareReplyPayload(t *testing.T) {
	type testCase struct {
		DocInfo model.DocumentInfo
		Result  messageContracts.SendReplyPayload
		Agent   contracts.AgentInfo
	}

	// generate test cases
	var testCases []testCase
	for _, fileName := range sampleMessageReplyFiles {
		// parse a test reply to see if we can regenerate it
		expectedReply := loadMessageReplyFromFile(t, fileName)

		testCases = append(testCases, testCase{
			DocInfo: model.DocumentInfo{
				AdditionalInfo:      expectedReply.AdditionalInfo,
				DocumentStatus:      expectedReply.DocumentStatus,
				DocumentTraceOutput: expectedReply.DocumentTraceOutput,
				RuntimeStatus:       expectedReply.RuntimeStatus,
			},
			Result: expectedReply,
			Agent:  expectedReply.AdditionalInfo.Agent,
		})
	}

	// run test cases
	for _, tst := range testCases {
		// call our method under test
		docResult := PrepareReplyPayload(tst.DocInfo, tst.Agent)
		// check result
		assert.Equal(t, tst.Result, docResult)

	}

}

func loadFile(t *testing.T, fileName string) (result []byte) {
	result, err := ioutil.ReadFile(fileName)
	if err != nil {
		t.Fatal(err)
	}
	return
}

func loadMessageReplyFromFile(t *testing.T, fileName string) (message messageContracts.SendReplyPayload) {
	b := loadFile(t, fileName)
	err := json.Unmarshal(b, &message)
	if err != nil {
		t.Fatal(err)
	}
	return message
}

func parsePluginResult(t *testing.T, pluginRuntimeStatus contracts.PluginRuntimeStatus) contracts.PluginResult {
	parsedOutput := pluginRuntimeStatus.Output
	return contracts.PluginResult{
		PluginName:     pluginRuntimeStatus.Name,
		Output:         parsedOutput,
		Status:         pluginRuntimeStatus.Status,
		StartDateTime:  times.ParseIso8601UTC(pluginRuntimeStatus.StartDateTime),
		EndDateTime:    times.ParseIso8601UTC(pluginRuntimeStatus.EndDateTime),
		StandardOutput: pluginRuntimeStatus.StandardOutput,
		StandardError:  pluginRuntimeStatus.StandardError,
	}
}
