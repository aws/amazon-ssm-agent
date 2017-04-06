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

package processor

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"path"
	"testing"
	"time"

	"github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/aws/amazon-ssm-agent/agent/contracts"
	"github.com/aws/amazon-ssm-agent/agent/docmanager/model"
	"github.com/aws/amazon-ssm-agent/agent/fileutil"
	"github.com/aws/amazon-ssm-agent/agent/jsonutil"
	"github.com/aws/amazon-ssm-agent/agent/log"
	messageContracts "github.com/aws/amazon-ssm-agent/agent/message/contracts"
	"github.com/aws/amazon-ssm-agent/agent/message/converter"
	"github.com/aws/amazon-ssm-agent/agent/message/parser"
	"github.com/aws/amazon-ssm-agent/agent/task"
	"github.com/aws/amazon-ssm-agent/agent/times"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ssmmds"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/twinj/uuid"
)

var sampleMessageFiles = []string{
	"../testdata/sampleMsg.json",
	"../testdata/sampleMsgVersion2_0.json",
}

var sampleMessageReplacedParamsFiles = []string{
	"../testdata/sampleMsgReplacedParams.json",
	"../testdata/sampleMsgReplacedParamsVersion2_0.json",
}

var sampleMessageReplyFiles = []string{
	"../testdata/sampleReply.json",
	"../testdata/sampleReplyVersion2_0.json",
}

var testMessageId = "03f44d19-90fe-44d4-bd4c-298b966a1e1a"
var testDestination = "i-1679test"
var testTopicSend = "aws.ssm.sendCommand.test"
var testTopicCancel = "aws.ssm.cancelCommand.test"
var testCreatedDate = "2015-01-01T00:00:00.000Z"
var testEmptyMessage = ""

var loggers = log.NewMockLog()

type TestCaseSendCommand struct {
	// Msg stores a parsed MDS message as received from GetMessages.
	Msg ssmmds.Message

	// DocState stores parsed Document State
	DocState model.DocumentState

	// MsgPayload stores the (parsed) payload of an MDS message.
	MsgPayload messageContracts.SendCommandPayload

	// PluginStates stores the configurations that the plugins require to run.
	// These configurations hav a slightly different structure from what we receive in the MDS message payload.
	PluginStates map[string]model.PluginState

	// PluginStatesArray stores the configurations that the plugins require to run for document version 2.0
	PluginStatesArray []model.PluginState

	// PluginResults stores the (unmarshalled) results that the plugins are expected to produce.
	PluginResults map[string]*contracts.PluginResult

	// ReplyPayload stores the message payload expected to be sent via SendReply (contains marshalled plugin results).
	ReplyPayload messageContracts.SendReplyPayload
}

type TestCaseCancelCommand struct {
	// MsgID is the id of the cancel command Message
	MsgID string

	// MsgToCancelID is the message ID found in the payload of the cancel command message
	MsgToCancelID string

	InstanceID string
}

// TestCaseProcessMessage contains fields to prepare processMessage tests
type TestCaseProcessMessage struct {
	ContextMock *context.Mock

	Message ssmmds.Message

	MdsMock *MockedMDS

	SendCommandTaskPoolMock *task.MockedPool

	CancelCommandTaskPoolMock *task.MockedPool

	IsDocLevelResponseSent *bool

	IsDataPersisted *bool
}

// TestCasePollOnce contains fields to prepare pollOnce tests
type TestCasePollOnce struct {
	ContextMock *context.Mock

	MdsMock *MockedMDS
}

// TestPollOnce tests the pollOnce function with one message
func TestPollOnce(t *testing.T) {
	// prepare test case fields
	proc, tc := prepareTestPollOnce()

	// mock GetMessagesOutput to return one message
	getMessageOutput := ssmmds.GetMessagesOutput{
		Destination:       &testDestination,
		Messages:          make([]*ssmmds.Message, 1),
		MessagesRequestId: &testMessageId,
	}

	// mock GetMessages function to return mocked GetMessagesOutput and no error
	tc.MdsMock.On("GetMessages", mock.AnythingOfType("*log.Mock"), mock.AnythingOfType("string")).Return(&getMessageOutput, nil)

	// set expectations
	countMessageProcessed := 0
	processMessage = func(proc *Processor, msg *ssmmds.Message) {
		countMessageProcessed++
	}

	// execute pollOnce
	proc.pollOnce()

	// check expectations
	tc.MdsMock.AssertExpectations(t)
	assert.Equal(t, countMessageProcessed, 1)
}

// TestPollOnceWithZeroMessage tests the pollOnce function with zero message
func TestPollOnceWithZeroMessage(t *testing.T) {
	// prepare test case fields
	proc, tc := prepareTestPollOnce()

	// mock GetMessagesOutput to return zero message
	getMessageOutput := ssmmds.GetMessagesOutput{
		Destination:       &testDestination,
		Messages:          make([]*ssmmds.Message, 0),
		MessagesRequestId: &testMessageId,
	}

	// mock GetMessages function to return mocked GetMessagesOutput and no error
	tc.MdsMock.On("GetMessages", mock.AnythingOfType("*log.Mock"), mock.AnythingOfType("string")).Return(&getMessageOutput, nil)
	countMessageProcessed := 0
	processMessage = func(proc *Processor, msg *ssmmds.Message) {
		countMessageProcessed++
	}

	// execute pollOnce
	proc.pollOnce()

	// check expectations
	tc.MdsMock.AssertExpectations(t)
	assert.Equal(t, countMessageProcessed, 0)
}

// TestPollOnceMultipleTimes tests the pollOnce function with five messages
func TestPollOnceMultipleTimes(t *testing.T) {
	// prepare test case fields
	proc, tc := prepareTestPollOnce()

	// mock GetMessagesOutput to return five message
	getMessageOutput := ssmmds.GetMessagesOutput{
		Destination:       &testDestination,
		Messages:          make([]*ssmmds.Message, 5),
		MessagesRequestId: &testMessageId,
	}

	// mock GetMessages function to return mocked GetMessagesOutput and no error
	tc.MdsMock.On("GetMessages", mock.AnythingOfType("*log.Mock"), mock.AnythingOfType("string")).Return(&getMessageOutput, nil)
	countMessageProcessed := 0
	processMessage = func(proc *Processor, msg *ssmmds.Message) {
		countMessageProcessed++
	}

	// execute pollOnce
	proc.pollOnce()

	// check expectations
	tc.MdsMock.AssertExpectations(t)
	assert.Equal(t, countMessageProcessed, 5)
}

// TestPollOnceWithGetMessagesReturnError tests the pollOnce function with errors from GetMessages function
func TestPollOnceWithGetMessagesReturnError(t *testing.T) {
	// prepare test case fields
	proc, tc := prepareTestPollOnce()

	// mock GetMessagesOutput to return one message
	getMessageOutput := ssmmds.GetMessagesOutput{
		Destination:       &testDestination,
		Messages:          make([]*ssmmds.Message, 1),
		MessagesRequestId: &testMessageId,
	}

	// mock GetMessages function to return an error
	tc.MdsMock.On("GetMessages", mock.AnythingOfType("*log.Mock"), mock.AnythingOfType("string")).Return(&getMessageOutput, fmt.Errorf("Test"))
	isMessageProcessed := false
	processMessage = func(proc *Processor, msg *ssmmds.Message) {
		isMessageProcessed = true
	}

	// execute pollOnce
	proc.pollOnce()

	// check expectations
	tc.MdsMock.AssertExpectations(t)
	assert.False(t, isMessageProcessed)
}

// TestProcessMessageWithSendCommandTopicPrefix tests processMessage with SendCommand topic prefix
func TestProcessMessageWithSendCommandTopicPrefix(t *testing.T) {
	// SendCommand topic prefix
	var topic = testTopicSend

	// prepare processor and test case fields
	proc, tc := prepareTestProcessMessage(topic)

	// set the expectations
	tc.MdsMock.On("AcknowledgeMessage", mock.Anything, *tc.Message.MessageId).Return(nil)
	tc.SendCommandTaskPoolMock.On("Submit", mock.Anything, mock.AnythingOfType("string"), mock.AnythingOfType("task.Job")).Return(nil)
	loadDocStateFromSendCommand = mockParseSendCommand

	// execute processMessage
	proc.processMessage(&tc.Message)

	// check expectations
	tc.ContextMock.AssertCalled(t, "Log")
	tc.MdsMock.AssertExpectations(t)
	tc.SendCommandTaskPoolMock.AssertExpectations(t)
	tc.CancelCommandTaskPoolMock.AssertNotCalled(t, "Submit")
	assert.True(t, *tc.IsDocLevelResponseSent)
	assert.True(t, *tc.IsDataPersisted)
}

// TestProcessMessageWithCancelCommandTopicPrefix tests processMessage with CancelCommand topic prefix
func TestProcessMessageWithCancelCommandTopicPrefix(t *testing.T) {
	// CancelCommand topic prefix
	var topic = testTopicCancel

	//prepare processor and test case fields
	proc, tc := prepareTestProcessMessage(topic)

	// set the expectations
	tc.MdsMock.On("AcknowledgeMessage", mock.Anything, *tc.Message.MessageId).Return(nil)
	tc.CancelCommandTaskPoolMock.On("Submit", mock.Anything, mock.AnythingOfType("string"), mock.AnythingOfType("task.Job")).Return(nil)
	loadDocStateFromCancelCommand = mockParseCancelCommand

	// execute processMessage
	proc.processMessage(&tc.Message)

	// check expectations
	tc.ContextMock.AssertCalled(t, "Log")
	tc.MdsMock.AssertExpectations(t)
	tc.CancelCommandTaskPoolMock.AssertExpectations(t)
	tc.SendCommandTaskPoolMock.AssertNotCalled(t, "Submit")
	assert.True(t, *tc.IsDocLevelResponseSent)
	assert.True(t, *tc.IsDataPersisted)
}

// TestProcessMessageWithInvalidCommandTopicPrefix tests processMessage with invalid topic prefix
func TestProcessMessageWithInvalidCommandTopicPrefix(t *testing.T) {
	// CancelCommand topic prefix
	var topic = "invalid"

	//prepare processor and test case fields
	proc, tc := prepareTestProcessMessage(topic)

	// set the expectations
	tc.MdsMock.On("FailMessage", mock.Anything, *tc.Message.MessageId, mock.Anything).Return(nil)

	// execute processMessage
	proc.processMessage(&tc.Message)

	// check expectations
	tc.ContextMock.AssertCalled(t, "Log")
	tc.MdsMock.AssertExpectations(t)
	tc.SendCommandTaskPoolMock.AssertNotCalled(t, "Submit")
	tc.CancelCommandTaskPoolMock.AssertNotCalled(t, "Submit")
	assert.False(t, *tc.IsDocLevelResponseSent)
	assert.False(t, *tc.IsDataPersisted)
}

// TestProcessMessageWithInvalidMessage tests processMessage with invalid message
func TestProcessMessageWithInvalidMessage(t *testing.T) {
	// prepare processor and test case fields
	proc, tc := prepareTestProcessMessage(testTopicSend)

	// exclude some fields from message
	tc.Message = ssmmds.Message{
		CreatedDate: &testEmptyMessage,
		Destination: &testEmptyMessage,
		MessageId:   &testEmptyMessage,
		Topic:       &testEmptyMessage,
	}

	// execute processMessage
	proc.processMessage(&tc.Message)

	// check expectations
	tc.ContextMock.AssertCalled(t, "Log")
	tc.MdsMock.AssertNotCalled(t, "AcknowledgeMessage", mock.AnythingOfType("logger.T"))
	tc.SendCommandTaskPoolMock.AssertNotCalled(t, "Submit")
	tc.CancelCommandTaskPoolMock.AssertNotCalled(t, "Submit")
	assert.False(t, *tc.IsDocLevelResponseSent)
	assert.False(t, *tc.IsDataPersisted)
}

// TestProcessMessage tests that processSendCommandMessage calls all the expected APIs
// with the correct response.
func TestProcessSendCommandMessage(t *testing.T) {
	for i, messagePayloadFile := range sampleMessageFiles {
		messageReplyPayloadFile := sampleMessageReplyFiles[i]
		testCase := generateTestCaseFromFiles(t, messagePayloadFile, messageReplyPayloadFile, "i-400e1090")
		testProcessSendCommandMessage(t, testCase)
	}
}

func generateTestCaseFromFiles(t *testing.T, messagePayloadFile string, messageReplyPayloadFile string, instanceID string) (testCase TestCaseSendCommand) {
	// load message payload and create MDS message from it
	payload, err := parser.ParseMessageWithParams(loggers, string(loadFile(t, messagePayloadFile)))
	if err != nil {
		t.Fatal(err)
	}
	msgContent, err := jsonutil.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	testCase.Msg = createMDSMessage(payload.CommandID, msgContent, "aws.ssm.sendCommand.us.east.1.1", instanceID)
	testCase.MsgPayload = payload
	s3KeyPrefix := path.Join(payload.OutputS3KeyPrefix, payload.CommandID, *testCase.Msg.Destination)

	//orchestrationRootDir is set to CommandID considering that orchestration root directory name will be empty in the test case.
	orchestrationRootDir := getCommandID(*testCase.Msg.MessageId)

	//configs := make(map[string]*contracts.Configuration)
	testCase.PluginStates = make(map[string]model.PluginState)

	// document 1.0 & 1.2
	if payload.DocumentContent.RuntimeConfig != nil {
		configs := make(map[string]*contracts.Configuration)
		configs = getPluginConfigurationsFromRuntimeConfig(payload.DocumentContent.RuntimeConfig,
			orchestrationRootDir,
			payload.OutputS3BucketName,
			s3KeyPrefix,
			*testCase.Msg.MessageId)

		for pluginName, config := range configs {
			state := model.PluginState{}
			state.Configuration = *config
			state.Name = pluginName
			state.Id = pluginName
			testCase.PluginStates[pluginName] = state
		}
	}

	// document 2.0
	if payload.DocumentContent.MainSteps != nil {
		configs := []*contracts.Configuration{}
		configs = getPluginConfigurationsFromMainStep(payload.DocumentContent.MainSteps,
			orchestrationRootDir,
			payload.OutputS3BucketName,
			s3KeyPrefix,
			*testCase.Msg.MessageId)

		pluginStatesArrays := make([]model.PluginState, len(configs))
		for index, config := range configs {
			state := model.PluginState{}
			state.Configuration = *config
			state.Name = config.PluginName
			state.Id = config.PluginID
			pluginStatesArrays[index] = state
		}
		testCase.PluginStatesArray = pluginStatesArrays
	}

	testCase.DocState = initializeSendCommandState(payload, orchestrationRootDir, s3KeyPrefix, testCase.Msg)

	return
}

func getPluginConfigurationsFromRuntimeConfig(runtimeConfig map[string]*contracts.PluginConfig, orchestrationDir, s3BucketName, s3KeyPrefix, messageID string) (res map[string]*contracts.Configuration) {
	res = make(map[string]*contracts.Configuration)
	for pluginName, pluginConfig := range runtimeConfig {
		res[pluginName] = &contracts.Configuration{
			Settings:               pluginConfig.Settings,
			Properties:             pluginConfig.Properties,
			OutputS3BucketName:     s3BucketName,
			OutputS3KeyPrefix:      fileutil.BuildS3Path(s3KeyPrefix, pluginName),
			OrchestrationDirectory: fileutil.BuildPath(orchestrationDir, pluginName),
			MessageId:              messageID,
			BookKeepingFileName:    getCommandID(messageID),
			PluginName:             pluginName,
			PluginID:               pluginName,
		}
	}
	return
}

func getPluginConfigurationsFromMainStep(mainSteps []*contracts.InstancePluginConfig, orchestrationDir, s3BucketName, s3KeyPrefix, messageID string) (res []*contracts.Configuration) {
	res = make([]*contracts.Configuration, len(mainSteps))
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
			BookKeepingFileName:    getCommandID(messageID),
			PluginName:             pluginName,
			PluginID:               pluginId,
		}
	}
	return
}

func testProcessSendCommandMessage(t *testing.T, testCase TestCaseSendCommand) {

	cancelFlag := task.NewChanneledCancelFlag()

	// method should call replyBuilder to format the response
	replyBuilderMock := new(MockedReplyBuilder)
	replyBuilderMock.On("BuildReply", mock.Anything, testCase.PluginResults).Return(testCase.ReplyPayload)

	// method should call the proper APIs on the MDS service
	mdsMock := new(MockedMDS)
	var replyPayload string
	mdsMock.On("SendReply", mock.Anything, mock.AnythingOfType("string"), mock.AnythingOfType("string")).Return(nil).Run(func(args mock.Arguments) {
		replyPayload = args.Get(2).(string)
	})
	mdsMock.On("DeleteMessage", mock.Anything, mock.AnythingOfType("string")).Return(nil)

	// create a mock sendResponse function
	sendResponse := func(messageID string, pluginID string, results map[string]*contracts.PluginResult) {
		contextMock := context.NewMockDefault()
		log := contextMock.Log()
		payloadDoc := replyBuilderMock.BuildReply(pluginID, results)
		payloadB, err := json.Marshal(payloadDoc)
		if err != nil {
			return
		}
		payload := string(payloadB)
		// call the mock sendreply so that we can assert the reply sent
		err = mdsMock.SendReply(log, messageID, payload)
	}

	// method should call plugin runner with the given configuration
	pluginRunnerMock := new(MockedPluginRunner)
	// mock.AnythingOfType("func(string, string, map[string]*plugin.Result)")

	pluginStates := []model.PluginState{}
	// For document 2.0
	if testCase.PluginStatesArray != nil {
		pluginStates = testCase.PluginStatesArray
	} else {
		pluginStates = converter.ConvertPluginState(testCase.PluginStates)
	}

	pluginRunnerMock.On("RunPlugins", mock.Anything, *testCase.Msg.MessageId, pluginStates, mock.Anything, cancelFlag).Return(testCase.PluginResults)

	// call method under test
	//orchestrationRootDir is set to empty such that it can meet the test expectation.
	p := Processor{}
	p.processSendCommandMessage(context.NewMockDefault(), mdsMock, pluginRunnerMock.RunPlugins, cancelFlag, replyBuilderMock.BuildReply, sendResponse, &testCase.DocState)

	// assert that the expectations were met
	pluginRunnerMock.AssertExpectations(t)
	replyBuilderMock.AssertExpectations(t)
	mdsMock.AssertExpectations(t)

	// check that the method sent the right reply
	var parsedReply messageContracts.SendReplyPayload
	err := json.Unmarshal([]byte(replyPayload), &parsedReply)
	assert.Nil(t, err)
	assert.Equal(t, testCase.ReplyPayload, parsedReply)
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

// getMessagesOutput wraps an MDS message into a GetMessagesOutput struct.
func getMessagesOutput(m *ssmmds.Message) ssmmds.GetMessagesOutput {
	uuid.SwitchFormat(uuid.CleanHyphen)
	requestID := uuid.NewV4().String()
	return ssmmds.GetMessagesOutput{
		Destination:       m.Destination,
		Messages:          []*ssmmds.Message{m},
		MessagesRequestId: aws.String(requestID),
	}
}

// TestProcessCancelCommandMessage tests that processCancelCommandMessage calls all the expected APIs
// on receiving a cancel message.
func TestProcessCancelCommandMessage(t *testing.T) {
	testCase := TestCaseCancelCommand{
		MsgToCancelID: uuid.NewV4().String(),
		MsgID:         uuid.NewV4().String(),
		InstanceID:    "i-400e1090",
	}

	testProcessCancelCommandMessage(t, testCase)
}

func testProcessCancelCommandMessage(t *testing.T, testCase TestCaseCancelCommand) {
	context := context.NewMockDefault()
	// create a cancel message
	cancelMessagePayload := messageContracts.CancelPayload{
		CancelMessageID: "aws.ssm" + testCase.MsgToCancelID + "." + testCase.InstanceID,
	}
	msgContent, err := jsonutil.Marshal(cancelMessagePayload)
	if err != nil {
		t.Fatal(err)
	}
	mdsCancelMessage := createMDSMessage(testCase.MsgID, msgContent, "aws.ssm.cancelCommand.us.east.1.1", testCase.InstanceID)

	// method should call the proper APIs on the MDS service
	mdsMock := new(MockedMDS)
	mdsMock.On("DeleteMessage", mock.Anything, *mdsCancelMessage.MessageId).Return(nil)

	// method should call cancel command
	sendCommandPoolMock := new(task.MockedPool)
	sendCommandPoolMock.On("Cancel", cancelMessagePayload.CancelMessageID).Return(true)

	docState := initializeCancelCommandState(mdsCancelMessage, cancelMessagePayload)

	p := Processor{}
	// call the code we are testing
	p.processCancelCommandMessage(context, mdsMock, sendCommandPoolMock, &docState)

	// assert that the expectations were met
	mdsMock.AssertExpectations(t)
	sendCommandPoolMock.AssertExpectations(t)
}

func prepareTestPollOnce() (proc Processor, testCase TestCasePollOnce) {

	// create mock context and log
	contextMock := context.NewMockDefault()

	// create mocked service and set expectations
	mdsMock := new(MockedMDS)

	// create a agentConfig with dummy instanceID and agentInfo
	agentConfig := contracts.AgentConfiguration{
		AgentInfo: contracts.AgentInfo{
			Name:      "EC2Config",
			Version:   "1",
			Lang:      "en-US",
			Os:        "linux",
			OsVersion: "1",
		},
		InstanceID: testDestination,
	}

	proc = Processor{
		context: contextMock,
		config:  agentConfig,
		service: mdsMock,
	}

	testCase = TestCasePollOnce{
		ContextMock: contextMock,
		MdsMock:     mdsMock,
	}

	return
}

func prepareTestProcessMessage(testTopic string) (proc Processor, testCase TestCaseProcessMessage) {

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
	mdsMock := new(MockedMDS)

	// sendCommand and cancelCommand will be processed by separate worker pools
	// so we can define the number of workers per each
	sendCommandTaskPool := new(task.MockedPool)
	cancelCommandTaskPool := new(task.MockedPool)

	orchestrationRootDir := ""

	// create a mock sendDocLevelResponse function
	isDocLevelResponseSent := false
	sendDocLevelResponse := func(messageID string, resultStatus contracts.ResultStatus, documentTraceOutput string) {
		isDocLevelResponseSent = true
	}

	// create a mock persistData function
	isDataPersisted := false
	persistData := func(docState *model.DocumentState, bookkeeping string) {
		isDataPersisted = true
	}

	// create a processor with all above
	proc = Processor{
		context:              contextMock,
		config:               agentConfig,
		service:              mdsMock,
		pluginRunner:         pluginRunner,
		sendCommandPool:      sendCommandTaskPool,
		cancelCommandPool:    cancelCommandTaskPool,
		sendDocLevelResponse: sendDocLevelResponse,
		orchestrationRootDir: orchestrationRootDir,
		persistData:          persistData,
	}

	testCase = TestCaseProcessMessage{
		ContextMock:               contextMock,
		Message:                   message,
		MdsMock:                   mdsMock,
		IsDocLevelResponseSent:    &isDocLevelResponseSent,
		IsDataPersisted:           &isDataPersisted,
		SendCommandTaskPoolMock:   sendCommandTaskPool,
		CancelCommandTaskPoolMock: cancelCommandTaskPool,
	}

	return
}

func parsePluginResult(t *testing.T, pluginRuntimeStatus contracts.PluginRuntimeStatus) contracts.PluginResult {
	parsedOutput := pluginRuntimeStatus.Output
	return contracts.PluginResult{
		Output:        parsedOutput,
		Status:        pluginRuntimeStatus.Status,
		StartDateTime: times.ParseIso8601UTC(pluginRuntimeStatus.StartDateTime),
		EndDateTime:   times.ParseIso8601UTC(pluginRuntimeStatus.EndDateTime),
	}
}

func loadFile(t *testing.T, fileName string) (result []byte) {
	result, err := ioutil.ReadFile(fileName)
	if err != nil {
		t.Fatal(err)
	}
	return
}

func loadMessageFromFile(t *testing.T, fileName string) (message messageContracts.SendCommandPayload) {
	b := loadFile(t, fileName)
	err := json.Unmarshal(b, &message)
	if err != nil {
		t.Fatal(err)
	}
	return message
}

func loadMessageReplyFromFile(t *testing.T, fileName string) (message messageContracts.SendReplyPayload) {
	b := loadFile(t, fileName)
	err := json.Unmarshal(b, &message)
	if err != nil {
		t.Fatal(err)
	}
	return message
}

func mockParseSendCommand(context context.T, msg *ssmmds.Message, messagesOrchestrationRootDir string) (*model.DocumentState, error) {
	return &model.DocumentState{
		DocumentType: model.SendCommand,
	}, nil
}

func mockParseCancelCommand(context context.T, msg *ssmmds.Message, messagesOrchestrationRootDir string) (*model.DocumentState, error) {
	return &model.DocumentState{
		DocumentType: model.CancelCommand,
	}, nil
}
