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

// Package runpluginutil provides interfaces for running plugins that can be referenced by other plugins and a utility method for parsing documents
package runpluginutil

import (
	"path/filepath"
	"testing"

	"github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/aws/amazon-ssm-agent/agent/contracts"
	"github.com/aws/amazon-ssm-agent/agent/docmanager/model"
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

func TestParseDocument_Valid(t *testing.T) {
	mockContext := context.NewMockDefault()

	pluginsInfo, err := ParseDocument(mockContext, []byte(validdocument), testOrchDir, testS3Bucket, testS3Prefix, testMessageID, testDocumentID, testWorkingDir)

	assert.Nil(t, err)
	assert.Equal(t, 1, len(pluginsInfo))

	pluginInfoTest := pluginsInfo[0]
	assert.Nil(t, pluginInfoTest.Result.Error)
	assert.Equal(t, filepath.Join(testOrchDir, "awsrunPowerShellScript"), pluginInfoTest.Configuration.OrchestrationDirectory)
	assert.Equal(t, testS3Bucket, pluginInfoTest.Configuration.OutputS3BucketName)
	assert.Equal(t, filepath.Join(testS3Prefix, "awsrunPowerShellScript"), pluginInfoTest.Configuration.OutputS3KeyPrefix)
	assert.Equal(t, testMessageID, pluginInfoTest.Configuration.MessageId)
	assert.Equal(t, testDocumentID, pluginInfoTest.Configuration.BookKeepingFileName)
	assert.Equal(t, testWorkingDir, pluginInfoTest.Configuration.DefaultWorkingDirectory)
}

func TestParseDocument_Invalid(t *testing.T) {
	mockContext := context.NewMockDefault()
	invaliddocument := `FOO`

	_, err := ParseDocument(mockContext, []byte(invaliddocument), testOrchDir, testS3Bucket, testS3Prefix, testMessageID, testDocumentID, testWorkingDir)

	assert.NotNil(t, err)
}

func TestExecuteDocument(t *testing.T) {
	mockContext := context.NewMockDefault()
	pluginsInfo, _ := ParseDocument(mockContext, []byte(validdocument), testOrchDir, testS3Bucket, testS3Prefix, testMessageID, testDocumentID, testWorkingDir)
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
