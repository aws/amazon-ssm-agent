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

// Package runpluginutil run plugin utility functions without referencing the actually plugin impl packages
package runpluginutil

import (
	"encoding/json"
	"testing"

	"github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/aws/amazon-ssm-agent/agent/contracts"
	"github.com/aws/amazon-ssm-agent/agent/docmanager/model"
	"github.com/aws/amazon-ssm-agent/agent/docparser"
	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/amazon-ssm-agent/agent/task"
	"github.com/stretchr/testify/assert"
)

const testOrchDir = "test-orchestrationDir"
const testS3Bucket = "test-s3Bucket"
const testS3Prefix = "test-s3KeyPrefix"
const testMessageID = "test-messageID"
const testDocumentID = "test-documentID"
const testWorkingDir = "test-defaultWorkingDirectory"
const validdocument = `{"schemaVersion":"1.2","description":"PowerShell.","runtimeConfig":{"aws:runPowerShellScript":{"properties":[{"id":"0.aws:runPowerShellScript","runCommand":["echo foo"],"timeoutSeconds":"5"}]}}}`

func TestExecuteDocument(t *testing.T) {
	mockLog := log.NewMockLog()
	mockContext := context.NewMockDefault()

	testParserInfo := docparser.DocumentParserInfo{
		OrchestrationDir:  testOrchDir,
		S3Bucket:          testS3Bucket,
		S3Prefix:          testS3Prefix,
		MessageId:         testMessageID,
		DocumentId:        testDocumentID,
		DefaultWorkingDir: testWorkingDir,
	}
	var testDocContent contracts.DocumentContent
	err := json.Unmarshal([]byte(validdocument), &testDocContent)
	if err != nil {
		assert.Error(t, err, "Error occured when trying to unmarshal validDocument")
	}
	pluginsInfo, _ := docparser.ParseDocument(mockLog, &testDocContent, testParserInfo, nil)
	runner := &PluginRunner{RunPlugins: func(
		context context.T,
		documentID string,
		documentCreateData string,
		plugins []model.PluginState,
		pluginRegistry PluginRegistry,
		sendReply SendResponseLegacy,
		updateAssoc UpdateAssociation,
		cancelFlag task.CancelFlag,
	) (pluginOutputs map[string]*contracts.PluginResult) {
		return map[string]*contracts.PluginResult{"foo": {}}
	},
		SendReply:   NoReply,
		UpdateAssoc: NoUpdate,
		CancelFlag:  &task.MockCancelFlag{},
	}
	result := runner.ExecuteDocument(mockContext, pluginsInfo, testDocumentID, "1/1/2000")
	assert.Equal(t, 1, len(result))
	_, exists := result["foo"]
	assert.True(t, exists)
}
