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

// Package runcommand implements runcommand core processing module
package runcommand

import (
	"encoding/json"
	"io/ioutil"

	"testing"

	"github.com/aws/amazon-ssm-agent/agent/contracts"
	"github.com/aws/amazon-ssm-agent/agent/log"
	messageContracts "github.com/aws/amazon-ssm-agent/agent/runcommand/contracts"
	"github.com/aws/amazon-ssm-agent/agent/times"
	"github.com/stretchr/testify/assert"
)

//TODO once service is moved out, merge all the reply tests here
var sampleMessageReplyFiles = []string{
	"./testdata/sampleReply.json",
	"./testdata/sampleReplyVersion2_0.json",
	"./testdata/sampleReplyVersion2_2.json",
}

func TestFormatPayload(t *testing.T) {
	logger := log.NewMockLog()
	for _, fileName := range sampleMessageReplyFiles {
		// load the test data
		logger.Infof("loading test file %v", fileName)
		sampleReply := loadMessageReplyFromFile(t, fileName)
		outputs := make(map[string]*contracts.PluginResult)
		//PluginID is the Runtimestatus map key
		for pluginID, pluginRuntimeStatus := range sampleReply.RuntimeStatus {
			pluginResult := parsePluginResult(t, *pluginRuntimeStatus)
			outputs[pluginID] = &pluginResult
		}
		// format the payload for document status update
		payload := FormatPayload(logger, "", sampleReply.AdditionalInfo.Agent, outputs)
		// fix the date time
		payload.AdditionalInfo.DateTime = sampleReply.AdditionalInfo.DateTime
		assert.Equal(t, payload, sampleReply)
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
