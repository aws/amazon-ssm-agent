package utils

import (
	"fmt"
	"testing"

	"github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/aws/amazon-ssm-agent/agent/contracts"
	contracts2 "github.com/aws/amazon-ssm-agent/agent/messageservice/contracts"
	messageContracts "github.com/aws/amazon-ssm-agent/agent/runcommand/contracts"
	"github.com/stretchr/testify/assert"
)

const (
	testLogGroupName = "myLogGroupName"
	testCommandID    = "123123123"
	testInstanceID   = "i-123123123"
	testDocumentName = "myDocumentName"
)

func TestParseCancelCommandMessage(t *testing.T) {
	mockContext := context.NewMockDefault()
	msg := contracts2.InstanceMessage{
		Destination: "destination",
		MessageId:   "MessageID",
		CreatedDate: "2017-06-10T01-23-07.853Z",
		Payload:     "{\"CancelMessageId\":\"aws.ssm.e8b9850d-930a-4366-a5a6-34060e003170.i-0094d85abec5ef507\"}",
	}

	docState, err := ParseCancelCommandMessage(mockContext, msg, contracts.MessageGatewayService)
	assert.Nil(t, err)

	assert.Equal(t, contracts.CancelCommand, docState.DocumentType)
	assert.NotNil(t, docState)

	assert.Equal(t, "e8b9850d-930a-4366-a5a6-34060e003170", docState.CancelInformation.CancelCommandID)
	assert.Equal(t, "aws.ssm.e8b9850d-930a-4366-a5a6-34060e003170.i-0094d85abec5ef507", docState.CancelInformation.CancelMessageID)
	assert.Equal(t, contracts.MessageGatewayService, docState.UpstreamServiceName)
}

func TestParseSendCommandMessage(t *testing.T) {
	mockContext := context.NewMockDefault()
	msg := contracts2.InstanceMessage{
		Destination: "destination",
		MessageId:   "e8b9850d-930a-4366-a5a6-34060e003170",
		CreatedDate: "2017-06-10T01-23-07.853Z",
		Payload:     "{\"Parameters\":{\"workingDirectory\":\"\",\"runCommand\":[\"echo hello; sleep 10\"]},\"DocumentContent\":{\"schemaVersion\":\"1.2\",\"description\":\"This document defines the PowerShell command to run or path to a script which is to be executed.\",\"runtimeConfig\":{\"aws:runScript\":{\"properties\":[{\"workingDirectory\":\"{{ workingDirectory }}\",\"timeoutSeconds\":\"{{ timeoutSeconds }}\",\"runCommand\":\"{{ runCommand }}\",\"id\":\"0.aws:runScript\"}]}},\"parameters\":{\"workingDirectory\":{\"default\":\"\",\"description\":\"Path to the working directory (Optional)\",\"type\":\"String\"},\"timeoutSeconds\":{\"default\":\"\",\"description\":\"Timeout in seconds (Optional)\",\"type\":\"String\"},\"runCommand\":{\"description\":\"List of commands to run (Required)\",\"type\":\"Array\"}}},\"CommandId\":\"55b78ece-7a7f-4198-aaf4-d8c8a3e960e6\",\"DocumentName\":\"AWS-RunPowerShellScript\",\"CloudWatchOutputEnabled\":\"true\"}",
	}

	docState, err := ParseSendCommandMessage(mockContext, msg, "messagesOrchestrationRootDir", contracts.MessageGatewayService)
	pluginInfo := docState.InstancePluginsInformation
	assert.Nil(t, err)
	assert.Equal(t, contracts.SendCommand, docState.DocumentType)
	assert.Equal(t, "e8b9850d-930a-4366-a5a6-34060e003170", docState.DocumentInformation.CommandID)
	assert.Equal(t, "1.2", docState.SchemaVersion)
	assert.Equal(t, 1, len(pluginInfo))
	assert.Equal(t, "e8b9850d-930a-4366-a5a6-34060e003170", pluginInfo[0].Configuration.MessageId)
	assert.Equal(t, contracts.MessageGatewayService, docState.UpstreamServiceName)
}

func TestGenerateCloudWatchConfigWithOutputEnabled(t *testing.T) {
	mockContext := context.NewMockDefault()
	expectedLogGroupName := fmt.Sprintf("%s%s", CloudWatchLogGroupNamePrefix, testDocumentName)
	expectedLogStreamName := fmt.Sprintf("%s/%s", testCommandID, testInstanceID)
	mockParsedMessage := getSampleParsedMessage("", "true")

	cloudWatchConfig, err := generateCloudWatchConfigFromPayload(mockContext, mockParsedMessage)
	assert.Nil(t, err)
	assert.Equal(t, expectedLogGroupName, cloudWatchConfig.LogGroupName)
	assert.Equal(t, expectedLogStreamName, cloudWatchConfig.LogStreamPrefix)
}

func TestGenerateCloudWatchConfigWithLogGroupNameAndOutputEnabled(t *testing.T) {
	mockContext := context.NewMockDefault()
	expectedLogStreamName := fmt.Sprintf("%s/%s", testCommandID, testInstanceID)
	expectedLogGroupName := "myLogGroupName"
	mockParsedMessage := getSampleParsedMessage(expectedLogGroupName, "true")

	cloudWatchConfig, err := generateCloudWatchConfigFromPayload(mockContext, mockParsedMessage)
	assert.Nil(t, err)
	assert.Equal(t, expectedLogGroupName, cloudWatchConfig.LogGroupName)
	assert.Equal(t, expectedLogStreamName, cloudWatchConfig.LogStreamPrefix)
}

func TestGenerateCloudWatchConfigWithOutputNotEnabled(t *testing.T) {
	mockContext := context.NewMockDefault()

	mockParsedMessage := getSampleParsedMessage("", "false")
	cloudWatchConfig, err := generateCloudWatchConfigFromPayload(mockContext, mockParsedMessage)
	assert.Nil(t, err)
	assert.Equal(t, contracts.CloudWatchConfiguration{}, cloudWatchConfig)
}

func TestGenerateCloudWatchConfigWithLogGroupNameAndOutputNotEnabled(t *testing.T) {
	mockContext := context.NewMockDefault()

	mockParsedMessage := getSampleParsedMessage(testLogGroupName, "false")
	cloudWatchConfig, err := generateCloudWatchConfigFromPayload(mockContext, mockParsedMessage)
	assert.Nil(t, err)
	assert.Equal(t, contracts.CloudWatchConfiguration{}, cloudWatchConfig)
}

func TestGenerateCloudWatchConfigWithEmptyCloudWatchConfigInPayload(t *testing.T) {
	mockContext := context.NewMockDefault()

	mockParsedMessage := getSampleParsedMessage("", "")
	cloudWatchConfig, err := generateCloudWatchConfigFromPayload(mockContext, mockParsedMessage)
	assert.Equal(t, contracts.CloudWatchConfiguration{}, cloudWatchConfig)
	assert.NotNil(t, err)
}

func TestGenerateCloudWatchConfigWithoutEmptyValuesInParsedMessage(t *testing.T) {
	mockContext := context.NewMockDefault()

	emptyParsedMessage := messageContracts.SendCommandPayload{
		CommandID:    testCommandID,
		DocumentName: testDocumentName,
	}
	cloudWatchConfig, err := generateCloudWatchConfigFromPayload(mockContext, emptyParsedMessage)
	assert.Equal(t, contracts.CloudWatchConfiguration{}, cloudWatchConfig)
	assert.NotNil(t, err)
}

func TestCloudLogGroupNameCleanup(t *testing.T) {
	inputOutputMap := make(map[string]string)
	inputOutputMap["aws:vendor:account:document-name"] = "aws.vendor.account.document-name"
	inputOutputMap["aws:vendor:account/document-name"] = "aws.vendor.account/document-name"
	inputOutputMap["aws:vendor:account/document/name123"] = "aws.vendor.account/document/name123"
	inputOutputMap["AWS-TestDoc"] = "AWS-TestDoc"
	inputOutputMap["//__****;;\\\\"] = "//__........"
	inputOutputMap["#\n\n  "] = "#...."
	inputOutputMap[""] = ""

	for input, output := range inputOutputMap {
		groupName := cleanupLogGroupName(input)
		assert.Equal(t, groupName, output)
	}
}

// getSampleParsedMessage returns a mocked SendCommandPayload
func getSampleParsedMessage(logGroupName string, outputEnabled string) messageContracts.SendCommandPayload {

	return messageContracts.SendCommandPayload{
		CommandID:               testCommandID,
		DocumentName:            testDocumentName,
		CloudWatchLogGroupName:  logGroupName,
		CloudWatchOutputEnabled: outputEnabled,
	}
}
