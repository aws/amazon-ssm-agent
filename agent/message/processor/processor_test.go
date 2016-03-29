// Copyright 2016 Amazon.com, Inc. or its affiliates. All Rights Reserved.
//
// Licensed under the Amazon Software License (the "License"). You may not
// use this file except in compliance with the License. A copy of the
// License is located at
//
// http://aws.amazon.com/asl/
//
// or in the "license" file accompanying this file. This file is distributed
// on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND,
// express or implied. See the License for the specific language governing
// permissions and limitations under the License.

package processor

import (
	"crypto/sha256"
	"encoding/json"
	"io/ioutil"
	"path"
	"testing"
	"time"

	"github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/aws/amazon-ssm-agent/agent/contracts"
	"github.com/aws/amazon-ssm-agent/agent/framework/engine"
	"github.com/aws/amazon-ssm-agent/agent/framework/plugin"
	"github.com/aws/amazon-ssm-agent/agent/jsonutil"
	"github.com/aws/amazon-ssm-agent/agent/log"
	messageContracts "github.com/aws/amazon-ssm-agent/agent/message/contracts"
	"github.com/aws/amazon-ssm-agent/agent/message/parser"
	"github.com/aws/amazon-ssm-agent/agent/task"
	"github.com/aws/amazon-ssm-agent/agent/taskimpl"
	"github.com/aws/amazon-ssm-agent/agent/times"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ssmmds"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/twinj/uuid"
)

var sampleMessageFiles = []string{
	"../testdata/sampleMsg.json",
}

var sampleMessageReplacedParamsFiles = []string{
	"../testdata/sampleMsgReplacedParams.json",
}

var sampleMessageReplyFiles = []string{
	"../testdata/sampleReply.json",
}

var logger = log.NewMockLog()

type TestCaseSendCommand struct {
	// Msg stores a parsed MDS message as received from GetMessages.
	Msg ssmmds.Message

	// MsgPayload stores the (parsed) payload of an MDS message.
	MsgPayload messageContracts.SendCommandPayload

	// PluginConfigs stores the configurations that the plugins require to run.
	// These configurations hav a slightly different structure from what we receive in the MDS message payload.
	PluginConfigs map[string]*contracts.Configuration

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
	payload, err := parser.ParseMessageWithParams(logger, string(loadFile(t, messagePayloadFile)))
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

	testCase.PluginConfigs = getPluginConfigurations(payload.DocumentContent.RuntimeConfig,
		orchestrationRootDir,
		payload.OutputS3BucketName,
		s3KeyPrefix,
		*testCase.Msg.MessageId)

	testCase.PluginResults = make(map[string]*contracts.PluginResult)
	testCase.ReplyPayload = loadMessageReplyFromFile(t, messageReplyPayloadFile)
	for pluginName, pluginRuntimeStatus := range testCase.ReplyPayload.RuntimeStatus {
		pluginResult := parsePluginResult(t, *pluginRuntimeStatus)
		testCase.PluginResults[pluginName] = &pluginResult
	}
	return
}

func testProcessSendCommandMessage(t *testing.T, testCase TestCaseSendCommand) {

	cancelFlag := taskimpl.NewCancelFlag()

	// method should call replyBuilder to format the response
	replyBuilderMock := new(MockedReplyBuilder)
	replyBuilderMock.On("BuildReply", mock.Anything, testCase.PluginResults).Return(testCase.ReplyPayload)

	// method should call the proper APIs on the MDS service
	mdsMock := new(MockedMDS)
	var replyPayload string
	mdsMock.On("SendReply", mock.Anything, *testCase.Msg.MessageId, mock.AnythingOfType("string")).Return(nil).Run(func(args mock.Arguments) {
		replyPayload = args.Get(2).(string)
	})
	mdsMock.On("DeleteMessage", mock.Anything, *testCase.Msg.MessageId).Return(nil)

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
	pluginRunnerMock.On("RunPlugins", mock.Anything, *testCase.Msg.MessageId, testCase.PluginConfigs, mock.Anything, cancelFlag).Return(testCase.PluginResults)

	// call method under test
	//orchestrationRootDir is set to empty such that it can meet the test expectation.
	orchestrationRootDir := ""
	p := Processor{}
	p.processSendCommandMessage(context.NewMockDefault(), mdsMock, orchestrationRootDir, pluginRunnerMock.RunPlugins, cancelFlag, replyBuilderMock.BuildReply, sendResponse, testCase.Msg)

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

	p := Processor{}
	// call the code we are testing
	p.processCancelCommandMessage(context, mdsMock, sendCommandPoolMock, mdsCancelMessage)

	// assert that the expectations were met
	mdsMock.AssertExpectations(t)
	sendCommandPoolMock.AssertExpectations(t)
}

// TestPollOnce tests the pollOnce method with only service and plugin runner mocked.
// This checks that all the messages get to be processed, all the service APIs are called,
// and all the worker pools close properly. This test is not concerned with the correctness
// of the response sent to the service.
func TestPollOnce(t *testing.T) {
	var messages ssmmds.GetMessagesOutput
	err := jsonutil.UnmarshalFile("../testdata/sampleGetMessagesResp.json", &messages)
	assert.Nil(t, err)

	instanceID := *messages.Destination
	agentInfo := contracts.AgentInfo{
		Name:      "EC2Config",
		Version:   "1",
		Lang:      "en-US",
		Os:        "linux",
		OsVersion: "1",
	}

	agentConfig := contracts.AgentConfiguration{
		AgentInfo:  agentInfo,
		InstanceID: instanceID,
	}

	contextMock := context.NewMockDefault()
	log := contextMock.Log()

	// create mocked service and set expectations
	mdsMock := new(MockedMDS)
	mdsMock.On("GetMessages", log, instanceID).Return(&messages, nil)
	for _, msg := range messages.Messages {
		mdsMock.On("AcknowledgeMessage", log, *msg.MessageId).Return(nil)
		mdsMock.On("SendReply", log, *msg.MessageId, mock.AnythingOfType("string")).Return(nil)
		mdsMock.On("AcknowledgeMessage", log, *msg.MessageId).Return(nil)
		mdsMock.On("DeleteMessage", log, *msg.MessageId).Return(nil)
	}

	var clock = times.DefaultClock
	replyBuilder := func(pluginID string, results map[string]*contracts.PluginResult) messageContracts.SendReplyPayload {
		t := clock.Now()
		runtimeStatuses := parser.PrepareRuntimeStatuses(log, results)
		return parser.PrepareReplyPayload(pluginID, runtimeStatuses, t, agentConfig.AgentInfo)
	}

	// create a mock sendResponse function
	sendResponse := func(messageID string, pluginID string, results map[string]*contracts.PluginResult) {
		payloadDoc := replyBuilder(pluginID, results)
		payloadB, err := json.Marshal(payloadDoc)
		if err != nil {
			return
		}
		payload := string(payloadB)
		// call the mock sendreply so that we can assert the reply sent
		err = mdsMock.SendReply(log, messageID, payload)
	}

	// create mock plugin that returns empty result
	mockedPlugin := new(plugin.Mock)
	mockedPlugin.On("Execute", contextMock, mock.Anything, mock.Anything).Return(contracts.PluginResult{})

	// create plugin runner
	pluginRegistry := plugin.PluginRegistry{}
	pluginRegistry["aws:runScript"] = mockedPlugin
	pluginRunner := func(context context.T, messageID string, plugins map[string]*contracts.Configuration, sendResponse engine.SendResponse, cancelFlag task.CancelFlag) (pluginOutputs map[string]*contracts.PluginResult) {
		return engine.RunPlugins(context, messageID, plugins, pluginRegistry, sendResponse, cancelFlag)
	}

	// sendCommand and cancelCommand will be processed by separate worker pools
	// so we can define the number of workers per each
	cancelWaitDuration := 100 * time.Millisecond
	sendCommandTaskPool := taskimpl.NewPool(log, 1, cancelWaitDuration, clock)
	cancelCommandTaskPool := taskimpl.NewPool(log, 1, cancelWaitDuration, clock)

	// run our method under test
	orchestrationRootDir := ""

	proc := Processor{
		context:              contextMock,
		config:               agentConfig,
		service:              mdsMock,
		pluginRunner:         pluginRunner,
		sendCommandPool:      sendCommandTaskPool,
		cancelCommandPool:    cancelCommandTaskPool,
		buildReply:           replyBuilder,
		sendResponse:         sendResponse,
		orchestrationRootDir: orchestrationRootDir,
	}
	proc.pollOnce()

	sendCommandTaskPool.ShutdownAndWait(time.Second)
	cancelCommandTaskPool.ShutdownAndWait(time.Second)

	mockedPlugin.AssertExpectations(t)
	mdsMock.AssertExpectations(t)
	contextMock.AssertCalled(t, "Log")
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
