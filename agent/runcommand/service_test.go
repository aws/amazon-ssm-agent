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
	"crypto/sha256"
	"strings"
	"testing"
	"time"

	"encoding/json"
	"path"

	"github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/aws/amazon-ssm-agent/agent/contracts"
	"github.com/aws/amazon-ssm-agent/agent/docparser"
	"github.com/aws/amazon-ssm-agent/agent/fileutil"
	processormock "github.com/aws/amazon-ssm-agent/agent/framework/processor/mock"
	"github.com/aws/amazon-ssm-agent/agent/jsonutil"
	"github.com/aws/amazon-ssm-agent/agent/log"
	messageContracts "github.com/aws/amazon-ssm-agent/agent/runcommand/contracts"
	runcommandmock "github.com/aws/amazon-ssm-agent/agent/runcommand/mock"
	"github.com/aws/amazon-ssm-agent/agent/times"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ssmmds"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

//TODO unittest the parser functions
var testMessageId = "03f44d19-90fe-44d4-bd4c-298b966a1e1a"
var testDestination = "i-1679test"
var testTopicSend = "aws.ssm.sendCommand.test"
var testTopicCancel = "aws.ssm.cancelCommand.test"
var testCreatedDate = "2015-01-01T00:00:00.000Z"
var testEmptyMessage = ""

var loggers = log.NewMockLog()

// TestCaseProcessMessage contains fields to prepare processMessage tests
type TestCaseProcessMessage struct {
	ContextMock *context.Mock

	Message ssmmds.Message

	MdsMock *runcommandmock.MockedMDS

	ProcessMock *processormock.MockedProcessor

	IsDocLevelResponseSent *bool
}

// TestProcessMessageWithSendCommandTopicPrefix tests processMessage with SendCommand topic prefix
func TestProcessMessageWithSendCommandTopicPrefix(t *testing.T) {
	// SendCommand topic prefix
	var topic = testTopicSend
	var fakeDocState = contracts.DocumentState{
		DocumentType: contracts.SendCommand,
	}
	// prepare processor and test case fields
	svc, tc := prepareTestProcessMessage(topic)

	// set the expectations
	tc.MdsMock.On("AcknowledgeMessage", mock.Anything, *tc.Message.MessageId).Return(nil)
	loadDocStateFromSendCommand = func(context context.T,
		msg *ssmmds.Message,
		messagesOrchestrationRootDir string) (*contracts.DocumentState, error) {
		return &fakeDocState, nil
	}

	tc.ProcessMock.On("Submit", fakeDocState).Return(nil)
	// execute processMessage
	svc.processMessage(&tc.Message)

	// check expectations
	tc.ContextMock.AssertCalled(t, "Log")
	tc.MdsMock.AssertExpectations(t)
	tc.ProcessMock.AssertExpectations(t)

	assert.True(t, *tc.IsDocLevelResponseSent)
}

// TestProcessMessageWithCancelCommandTopicPrefix tests processMessage with CancelCommand topic prefix
func TestProcessMessageWithCancelCommandTopicPrefix(t *testing.T) {
	// CancelCommand topic prefix
	var topic = testTopicCancel
	var fakeCancelDocState = contracts.DocumentState{
		DocumentType: contracts.CancelCommand,
	}
	//prepare processor and test case fields
	svc, tc := prepareTestProcessMessage(topic)

	// set the expectations
	tc.MdsMock.On("AcknowledgeMessage", mock.Anything, *tc.Message.MessageId).Return(nil)
	tc.ProcessMock.On("Cancel", fakeCancelDocState).Return(nil)
	loadDocStateFromCancelCommand = func(context context.T, msg *ssmmds.Message, messagesOrchestrationRootDir string) (*contracts.DocumentState, error) {
		return &fakeCancelDocState, nil
	}

	// execute processMessage
	svc.processMessage(&tc.Message)

	// check expectations
	tc.ContextMock.AssertCalled(t, "Log")
	tc.MdsMock.AssertExpectations(t)
	tc.ProcessMock.AssertExpectations(t)

	assert.True(t, *tc.IsDocLevelResponseSent)
}

// TestProcessMessageWithInvalidCommandTopicPrefix tests processMessage with invalid topic prefix
func TestProcessMessageWithInvalidCommandTopicPrefix(t *testing.T) {
	// CancelCommand topic prefix
	var topic = "invalid"

	//prepare processor and test case fields
	svc, tc := prepareTestProcessMessage(topic)

	// set the expectations, do not call Submit() since command parsing failed in the first place
	tc.MdsMock.On("FailMessage", mock.Anything, *tc.Message.MessageId, mock.Anything).Return(nil)

	// execute processMessage
	svc.processMessage(&tc.Message)

	// check expectations
	tc.ContextMock.AssertCalled(t, "Log")
	tc.MdsMock.AssertExpectations(t)

	assert.False(t, *tc.IsDocLevelResponseSent)
}

// TestProcessMessageWithInvalidMessage tests processMessage with invalid message
func TestProcessMessageWithInvalidMessage(t *testing.T) {
	// prepare processor and test case fields
	svc, tc := prepareTestProcessMessage(testTopicSend)

	// exclude some fields from message
	tc.Message = ssmmds.Message{
		CreatedDate: &testEmptyMessage,
		Destination: &testEmptyMessage,
		MessageId:   &testEmptyMessage,
		Topic:       &testEmptyMessage,
	}

	// execute processMessage
	svc.processMessage(&tc.Message)

	// check expectations
	tc.ContextMock.AssertCalled(t, "Log")
	tc.MdsMock.AssertNotCalled(t, "AcknowledgeMessage", mock.AnythingOfType("logger.T"))
	assert.False(t, *tc.IsDocLevelResponseSent)
}

func prepareTestProcessMessage(testTopic string) (svc RunCommandService, testCase TestCaseProcessMessage) {

	// create mock context and log
	contextMock := context.NewMockDefault()

	// create dummy message that would be passed processMessage
	message := ssmmds.Message{
		CreatedDate: &testCreatedDate,
		Destination: &testDestination,
		MessageId:   &testMessageId,
		Topic:       &testTopic,
	}

	// create a agentConfig with dummy instanceID and agentInfo
	agentConfig := contracts.AgentConfiguration{
		AgentInfo: contracts.AgentInfo{
			Name:      "EC2Config",
			Version:   "1",
			Lang:      "en-US",
			Os:        "linux",
			OsVersion: "1",
		},
		InstanceID: *message.Destination,
	}

	// create mocked service and set expectations
	mdsMock := new(runcommandmock.MockedMDS)

	orchestrationRootDir := ""

	// create a mock sendDocLevelResponse function
	isDocLevelResponseSent := false
	sendDocLevelResponse := func(messageID string, resultStatus contracts.ResultStatus, documentTraceOutput string) {
		isDocLevelResponseSent = true
	}

	// create mocked processor
	processorMock := new(processormock.MockedProcessor)

	svc = RunCommandService{
		context:              contextMock,
		config:               agentConfig,
		service:              mdsMock,
		sendDocLevelResponse: sendDocLevelResponse,
		orchestrationRootDir: orchestrationRootDir,
		processor:            processorMock,
	}

	testCase = TestCaseProcessMessage{
		ContextMock:            contextMock,
		Message:                message,
		MdsMock:                mdsMock,
		ProcessMock:            processorMock,
		IsDocLevelResponseSent: &isDocLevelResponseSent,
	}

	return
}

//TODO keep the following functions temporarily before we have processor's integ_test
var sampleMessageFiles = []string{
	"../service/runcommand/testdata/sampleMsg.json",
	"../service/runcommand/testdata/sampleMsgVersion2_0.json",
	"../service/runcommand/testdata/sampleMsgVersion2_2.json",
}

type TestCaseSendCommand struct {
	// Msg stores a parsed MDS message as received from GetMessages.
	Msg ssmmds.Message

	// DocState stores parsed Document State
	DocState contracts.DocumentState

	// PluginStates stores the configurations that the plugins require to run.
	// These configurations hav a slightly different structure from what we receive in the MDS message payload.
	PluginStates map[string]contracts.PluginState

	// PluginStatesArray stores the configurations that the plugins require to run for document version 2.0
	PluginStatesArray []contracts.PluginState

	// PluginResults stores the (unmarshalled) results that the plugins are expected to produce.
	PluginResults map[string]*contracts.PluginResult
}

type TestCaseCancelCommand struct {
	// MsgID is the id of the cancel command Message
	MsgID string

	// MsgToCancelID is the message ID found in the payload of the cancel command message
	MsgToCancelID string

	InstanceID string

	OrchestrationDir string
}

func GenerateDocStateFromFile(t *testing.T, messagePayloadFile string, instanceID string) (testCase TestCaseSendCommand) {
	// load message payload and create MDS message from it
	var payload messageContracts.SendCommandPayload
	err := json.Unmarshal((loadFile(t, messagePayloadFile)), &payload)
	if err != nil {
		t.Fatal(err)
	}
	msgContent, err := jsonutil.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	testCase.Msg = createMDSMessage(payload.CommandID, msgContent, "aws.ssm.sendCommand.us.east.1.1", instanceID)
	s3KeyPrefix := path.Join(payload.OutputS3KeyPrefix, payload.CommandID, *testCase.Msg.Destination)

	//orchestrationRootDir is set to CommandID considering that orchestration root directory name will be empty in the test case.
	orchestrationRootDir, _ := messageContracts.GetCommandID(*testCase.Msg.MessageId)

	//configs := make(map[string]*contracts.Configuration)
	testCase.PluginStates = make(map[string]contracts.PluginState)

	// document 1.0 & 1.2
	if payload.DocumentContent.RuntimeConfig != nil {
		configs := make(map[string]*contracts.Configuration)
		configs = getPluginConfigurationsFromRuntimeConfig(payload.DocumentContent.RuntimeConfig,
			orchestrationRootDir,
			payload.OutputS3BucketName,
			s3KeyPrefix,
			*testCase.Msg.MessageId)

		for pluginName, config := range configs {
			state := contracts.PluginState{}
			state.Configuration = *config
			state.Name = pluginName
			state.Id = pluginName
			testCase.PluginStates[pluginName] = state
		}
	}

	// document 2.0 & 2.2
	if payload.DocumentContent.MainSteps != nil {
		configs := []*contracts.Configuration{}
		configs = getPluginConfigurationsFromMainStep(payload.DocumentContent.MainSteps,
			orchestrationRootDir,
			payload.OutputS3BucketName,
			s3KeyPrefix,
			*testCase.Msg.MessageId,
			payload.DocumentContent.SchemaVersion)

		pluginStatesArrays := make([]contracts.PluginState, len(configs))
		for index, config := range configs {
			state := contracts.PluginState{}
			state.Configuration = *config
			state.Name = config.PluginName
			state.Id = config.PluginID
			pluginStatesArrays[index] = state
		}
		testCase.PluginStatesArray = pluginStatesArrays
	}
	var documentType contracts.DocumentType
	if strings.HasPrefix(*testCase.Msg.Topic, string(SendCommandTopicPrefixOffline)) {
		documentType = contracts.SendCommandOffline
	} else {
		documentType = contracts.SendCommand
	}
	documentInfo := newDocumentInfo(testCase.Msg, payload)
	parserInfo := docparser.DocumentParserInfo{
		OrchestrationDir: orchestrationRootDir,
		S3Bucket:         payload.OutputS3BucketName,
		S3Prefix:         s3KeyPrefix,
		MessageId:        documentInfo.MessageID,
		DocumentId:       documentInfo.DocumentID,
	}

	docContent := &docparser.DocContent{
		SchemaVersion: payload.DocumentContent.SchemaVersion,
		Description:   payload.DocumentContent.Description,
		RuntimeConfig: payload.DocumentContent.RuntimeConfig,
		MainSteps:     payload.DocumentContent.MainSteps,
		Parameters:    payload.DocumentContent.Parameters,
	}
	//Data format persisted in Current Folder is defined by the struct - CommandState
	testCase.DocState, err = docparser.InitializeDocState(loggers, documentType, docContent, documentInfo, parserInfo, payload.Parameters)
	if err != nil {
		t.Fatal(err)
	}

	return
}

func getPluginConfigurationsFromRuntimeConfig(runtimeConfig map[string]*contracts.PluginConfig, orchestrationDir, s3BucketName, s3KeyPrefix, messageID string) (res map[string]*contracts.Configuration) {
	res = make(map[string]*contracts.Configuration)
	commandID, _ := messageContracts.GetCommandID(messageID)
	for pluginName, pluginConfig := range runtimeConfig {
		res[pluginName] = &contracts.Configuration{
			Settings:               pluginConfig.Settings,
			Properties:             pluginConfig.Properties,
			OutputS3BucketName:     s3BucketName,
			OutputS3KeyPrefix:      fileutil.BuildS3Path(s3KeyPrefix, pluginName),
			OrchestrationDirectory: fileutil.BuildPath(orchestrationDir, pluginName),
			MessageId:              messageID,
			BookKeepingFileName:    commandID,
			PluginName:             pluginName,
			PluginID:               pluginName,
		}
	}
	return
}

func getPluginConfigurationsFromMainStep(mainSteps []*contracts.InstancePluginConfig, orchestrationDir, s3BucketName, s3KeyPrefix, messageID string, schemaVersion string) (res []*contracts.Configuration) {
	res = make([]*contracts.Configuration, len(mainSteps))

	// set precondition flag based on document schema version
	isPreconditionEnabled := contracts.IsPreconditionEnabled(schemaVersion)
	commandID, _ := messageContracts.GetCommandID(messageID)
	for index, instancePluginConfig := range mainSteps {
		pluginId := instancePluginConfig.Name
		pluginName := instancePluginConfig.Action
		res[index] = &contracts.Configuration{
			Settings:               instancePluginConfig.Settings,
			Properties:             instancePluginConfig.Inputs,
			OutputS3BucketName:     s3BucketName,
			OutputS3KeyPrefix:      fileutil.BuildS3Path(s3KeyPrefix, pluginName),
			OrchestrationDirectory: fileutil.BuildPath(orchestrationDir, pluginId),
			MessageId:              messageID,
			BookKeepingFileName:    commandID,
			PluginName:             pluginName,
			PluginID:               pluginId,
			Preconditions:          instancePluginConfig.Preconditions,
			IsPreconditionEnabled:  isPreconditionEnabled,
		}
	}
	return
}

func createMDSMessage(commandID string, payload string, topic string, instanceID string) ssmmds.Message {
	messageCreatedDate := time.Date(2015, 7, 9, 23, 22, 39, 19000000, time.UTC)

	c := sha256.New()
	c.Write([]byte(payload))
	payloadDigest := string(c.Sum(nil))

	return ssmmds.Message{
		CreatedDate:   aws.String(times.ToIso8601UTC(messageCreatedDate)),
		Destination:   aws.String(instanceID),
		MessageId:     aws.String("aws.ssm." + commandID + "." + instanceID),
		Payload:       aws.String(payload),
		PayloadDigest: aws.String(payloadDigest),
		Topic:         aws.String(topic),
	}
}
func GenerateCancelDocState(t *testing.T, testCase TestCaseCancelCommand) (docState *contracts.DocumentState) {
	context := context.NewMockDefault()
	cancelMessagePayload := messageContracts.CancelPayload{
		CancelMessageID: "aws.ssm" + testCase.MsgToCancelID + "." + testCase.InstanceID,
	}
	msgContent, err := jsonutil.Marshal(cancelMessagePayload)
	if err != nil {
		t.Fatal(err)
	}
	mdsCancelMessage := createMDSMessage(testCase.MsgID, msgContent, "aws.ssm.cancelCommand.us.east.1.1", testCase.InstanceID)

	docState, _ = parseCancelCommandMessage(context, &mdsCancelMessage, testCase.OrchestrationDir)
	return
}
