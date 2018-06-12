// Copyright 2018 Amazon.com, Inc. or its affiliates. All Rights Reserved.
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

package runcommand

import (
	"fmt"
	"testing"

	"github.com/aws/amazon-ssm-agent/agent/contracts"
	messageContracts "github.com/aws/amazon-ssm-agent/agent/runcommand/contracts"
	"github.com/stretchr/testify/assert"
)

const (
	testLogGroupName = "myLogGroupName"
	testCommandID    = "12345"
	testInstanceID   = "i-12345"
	testDocumentName = "myDocumentName"
)

func TestGenerateCloudWatchConfigWithOutputEnabled(t *testing.T) {
	systemInfo = &systemStub{}
	expectedLogGroupName := fmt.Sprintf("%s%s", CloudWatchLogGroupNamePrefix, testDocumentName)
	expectedLogStreamName := fmt.Sprintf("%s/%s", testCommandID, testInstanceID)
	mockParsedMessage := getSampleParsedMessage("", "true")

	cloudWatchConfig, err := generateCloudWatchConfigFromPayload(mockParsedMessage)
	assert.Nil(t, err)
	assert.Equal(t, expectedLogGroupName, cloudWatchConfig.LogGroupName)
	assert.Equal(t, expectedLogStreamName, cloudWatchConfig.LogStreamPrefix)
}

func TestGenerateCloudWatchConfigWithLogGroupNameAndOutputEnabled(t *testing.T) {
	systemInfo = &systemStub{}
	expectedLogStreamName := fmt.Sprintf("%s/%s", testCommandID, testInstanceID)
	expectedLogGroupName := "myLogGroupName"
	mockParsedMessage := getSampleParsedMessage(expectedLogGroupName, "true")

	cloudWatchConfig, err := generateCloudWatchConfigFromPayload(mockParsedMessage)
	assert.Nil(t, err)
	assert.Equal(t, expectedLogGroupName, cloudWatchConfig.LogGroupName)
	assert.Equal(t, expectedLogStreamName, cloudWatchConfig.LogStreamPrefix)
}

func TestGenerateCloudWatchConfigWithOutputNotEnabled(t *testing.T) {
	mockParsedMessage := getSampleParsedMessage("", "false")
	cloudWatchConfig, err := generateCloudWatchConfigFromPayload(mockParsedMessage)
	assert.Nil(t, err)
	assert.Equal(t, contracts.CloudWatchConfiguration{}, cloudWatchConfig)
}

func TestGenerateCloudWatchConfigWithLogGroupNameAndOutputNotEnabled(t *testing.T) {
	mockParsedMessage := getSampleParsedMessage(testLogGroupName, "false")
	cloudWatchConfig, err := generateCloudWatchConfigFromPayload(mockParsedMessage)
	assert.Nil(t, err)
	assert.Equal(t, contracts.CloudWatchConfiguration{}, cloudWatchConfig)
}

func TestGenerateCloudWatchConfigWithEmptyCloudWatchConfigInPayload(t *testing.T) {
	mockParsedMessage := getSampleParsedMessage("", "")
	cloudWatchConfig, err := generateCloudWatchConfigFromPayload(mockParsedMessage)
	assert.Equal(t, contracts.CloudWatchConfiguration{}, cloudWatchConfig)
	assert.NotNil(t, err)
}

func TestGenerateCloudWatchConfigWithoutEmptyValuesInParsedMessage(t *testing.T) {
	emptyParsedMessage := messageContracts.SendCommandPayload{
		CommandID:    testCommandID,
		DocumentName: testDocumentName,
	}
	cloudWatchConfig, err := generateCloudWatchConfigFromPayload(emptyParsedMessage)
	assert.Equal(t, contracts.CloudWatchConfiguration{}, cloudWatchConfig)
	assert.NotNil(t, err)
}

//getSampleParsedMessage returns a mocked SendCommandPayload
func getSampleParsedMessage(logGroupName string, outputEnabled string) messageContracts.SendCommandPayload {

	return messageContracts.SendCommandPayload{
		CommandID:               testCommandID,
		DocumentName:            testDocumentName,
		CloudWatchLogGroupName:  logGroupName,
		CloudWatchOutputEnabled: outputEnabled,
	}
}
