// Copyright 2021 Amazon.com, Inc. or its affiliates. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License"). You may not
// use this file except in compliance with the License. A copy of the
// License is located at
//
// http://aws.amazon.com/apache2.0/
//
// or in the "license" file accompanying this file. This file is distributed
// on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND,
// either express or implied. See the License for the specific language governing`
// permissions and limitations under the License.

// Package mdsinteractor will be responsible for communicating with MDS
package mdsinteractor

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/aws/amazon-ssm-agent/agent/contracts"
	"github.com/aws/amazon-ssm-agent/agent/messageservice/messagehandler"
	"github.com/aws/amazon-ssm-agent/agent/messageservice/messagehandler/mocks"
	mdsService "github.com/aws/amazon-ssm-agent/agent/runcommand/mds"
	runcommandmock "github.com/aws/amazon-ssm-agent/agent/runcommand/mock"
	"github.com/aws/amazon-ssm-agent/agent/sdkutil"
	"github.com/aws/aws-sdk-go/service/ssmmds"
	"github.com/carlescere/scheduler"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
)

const (
	testStopPolicyThreshold = 3
)

var (
	testMessageId    = "03f44d19-90fe-44d4-bd4c-298b966a1e1a"
	testDestination  = "i-1679test"
	testTopicSend    = "aws.ssm.sendCommand.test"
	testTopicCancel  = "aws.ssm.cancelCommand.test"
	testCreatedDate  = "2015-01-01T00:00:00.000Z"
	testEmptyMessage = ""
)

type MDSInteractorTestSuite struct {
	suite.Suite
	contextMock   *context.Mock
	mdsMock       *runcommandmock.MockedMDS
	mdsInteractor MDSInteractor
}

func TestMDSInteractorTestSuite(t *testing.T) {
	suite.Run(t, new(MDSInteractorTestSuite))
}

func (suite *MDSInteractorTestSuite) SetupTest() {
	contextMock := context.NewMockDefault()
	mdsMock := new(runcommandmock.MockedMDS)
	newMdsService = func(context context.T) mdsService.Service {
		return mdsMock
	}
	interactor := MDSInteractor{
		context:              contextMock,
		service:              mdsMock,
		messagePollWaitGroup: &sync.WaitGroup{},
		processorStopPolicy:  sdkutil.NewStopPolicy(Name, testStopPolicyThreshold),
	}

	suite.contextMock = contextMock
	suite.mdsMock = mdsMock
	suite.mdsInteractor = interactor
}

func (suite *MDSInteractorTestSuite) TestMDSInteractor_Initialize() {
	contextMock := suite.contextMock
	incomingChan := make(chan contracts.DocumentState)
	mdsServiceMock := suite.mdsMock
	mdsServiceMock.On("GetMessages", contextMock.Log(), mock.AnythingOfType("string")).Return(&ssmmds.GetMessagesOutput{}, nil)
	mdsServiceMock.On("LoadFailedReplies", contextMock.Log()).Return([]string{})

	mdsInteractor := suite.mdsInteractor
	mdsInteractor.replyChan = make(chan contracts.DocumentResult, 1)
	err := mdsInteractor.Initialize()

	assert.NoError(suite.T(), err)
	// message polling should not be loaded during this time
	assert.Nil(suite.T(), mdsInteractor.messagePollJob)
	assert.NotNil(suite.T(), mdsInteractor.sendReplyJob)
	close(mdsInteractor.replyChan)
	close(incomingChan)
}

func (suite *MDSInteractorTestSuite) TestMDSInteractor_ListenReply() {
	contextMock := suite.contextMock
	mdsInteractor := suite.mdsInteractor
	mdsInteractor.replyChan = make(chan contracts.DocumentResult, 1)
	mdsServiceMock := suite.mdsMock
	mdsServiceMock.On("SendReply", contextMock.Log(), mock.AnythingOfType("string"), mock.AnythingOfType("string")).Return(nil)

	pluginRes := contracts.PluginResult{
		PluginID:   "aws:runScript",
		PluginName: "aws:runScript",
		Status:     contracts.ResultStatusSuccess,
		Code:       0,
	}
	pluginResults := make(map[string]*contracts.PluginResult)
	pluginResults[pluginRes.PluginID] = &pluginRes
	result := contracts.DocumentResult{
		MessageID:     "1234",
		PluginResults: pluginResults,
		Status:        contracts.ResultStatusSuccess,
		LastPlugin:    "",
	}
	mdsInteractor.replyChan <- result
	close(mdsInteractor.replyChan)
	mdsInteractor.listenReply()
	mdsServiceMock.AssertNumberOfCalls(suite.T(), "SendReply", 1)
	mdsServiceMock.AssertCalled(suite.T(), "SendReply", contextMock.Log(), result.MessageID, mock.AnythingOfType("string"))
}

func (suite *MDSInteractorTestSuite) TestMDSInteractor_sendFailedReplies() {
	contextMock := suite.contextMock
	mdsServiceMock := suite.mdsMock

	now := time.Now().UTC()
	timeLayout := "2006-01-02T15-04-05"
	reply := fmt.Sprintf("%v_%v", now.Format(timeLayout), now.Add(time.Minute*10).Format(timeLayout))
	mdsServiceMock.On("LoadFailedReplies", contextMock.Log()).Return([]string{reply})
	mdsServiceMock.On("GetFailedReply", contextMock.Log(), reply).Return(&ssmmds.SendReplyInput{}, nil)
	mdsServiceMock.On("SendReplyWithInput", contextMock.Log(), &ssmmds.SendReplyInput{}).Return(nil)
	mdsServiceMock.On("DeleteFailedReply", contextMock.Log(), reply).Return()

	suite.mdsInteractor.sendFailedReplies()
	time.Sleep(1 * time.Second)

	mdsServiceMock.AssertNumberOfCalls(suite.T(), "SendReplyWithInput", 1)
	mdsServiceMock.AssertNumberOfCalls(suite.T(), "DeleteFailedReply", 1)
}

func (suite *MDSInteractorTestSuite) TestMDSInteractor_sendFailedRepliesWithZeroReplies() {
	contextMock := suite.contextMock
	mdsServiceMock := suite.mdsMock

	mdsServiceMock.On("LoadFailedReplies", contextMock.Log()).Return([]string{})

	suite.mdsInteractor.sendFailedReplies()
	time.Sleep(1 * time.Second)

	mdsServiceMock.AssertNumberOfCalls(suite.T(), "SendReplyWithInput", 0)
	mdsServiceMock.AssertNumberOfCalls(suite.T(), "DeleteFailedReply", 0)
}

func (suite *MDSInteractorTestSuite) TestMDSInteractor_sendFailedRepliesWithExpiredReply() {
	contextMock := suite.contextMock
	mdsServiceMock := suite.mdsMock

	expiredDate := "2006-01-02T15-04-05"
	reply := fmt.Sprintf("%v_%v", expiredDate, expiredDate)
	mdsServiceMock.On("LoadFailedReplies", contextMock.Log()).Return([]string{reply})
	mdsServiceMock.On("GetFailedReply", contextMock.Log(), reply).Return(&ssmmds.SendReplyInput{}, nil)
	mdsServiceMock.On("SendReplyWithInput", contextMock.Log(), &ssmmds.SendReplyInput{}).Return(nil)
	mdsServiceMock.On("DeleteFailedReply", contextMock.Log(), reply).Return()

	suite.mdsInteractor.sendFailedReplies()
	time.Sleep(1 * time.Second)

	mdsServiceMock.AssertNumberOfCalls(suite.T(), "SendReplyWithInput", 0)
	mdsServiceMock.AssertNumberOfCalls(suite.T(), "DeleteFailedReply", 1)
}

func (suite *MDSInteractorTestSuite) TestMDSInteractor_sendFailedRepliesWithSendReplyReturnError() {
	contextMock := suite.contextMock
	mdsServiceMock := suite.mdsMock

	now := time.Now().UTC()
	timeLayout := "2006-01-02T15-04-05"
	reply := fmt.Sprintf("%v_%v", now.Format(timeLayout), now.Add(time.Hour*3).Format(timeLayout))
	mdsServiceMock.On("LoadFailedReplies", contextMock.Log()).Return([]string{reply})
	mdsServiceMock.On("GetFailedReply", contextMock.Log(), reply).Return(&ssmmds.SendReplyInput{}, nil)
	mdsServiceMock.On("SendReplyWithInput", contextMock.Log(), &ssmmds.SendReplyInput{}).Return(fmt.Errorf("some error"))
	mdsServiceMock.On("DeleteFailedReply", contextMock.Log(), reply).Return()

	suite.mdsInteractor.sendFailedReplies()
	time.Sleep(1 * time.Second)

	mdsServiceMock.AssertNumberOfCalls(suite.T(), "SendReplyWithInput", 1)
	mdsServiceMock.AssertNumberOfCalls(suite.T(), "DeleteFailedReply", 0)
}

func (suite *MDSInteractorTestSuite) TestMDSInteractor_processMessageWithInvalidMessage() {
	message := ssmmds.Message{
		CreatedDate: &testEmptyMessage,
		Destination: &testEmptyMessage,
		MessageId:   &testEmptyMessage,
		Topic:       &testEmptyMessage,
	}

	suite.mdsInteractor.processMessage(&message)

	suite.mdsMock.AssertNotCalled(suite.T(), "AcknowledgeMessage", mock.Anything)
}

func (suite *MDSInteractorTestSuite) TestMDSInteractor_processMessageWithInvalidCommandTopic() {
	var topic = "invalid"
	message := ssmmds.Message{
		CreatedDate: &testCreatedDate,
		Destination: &testDestination,
		MessageId:   &testMessageId,
		Topic:       &topic,
	}
	contextMock := suite.contextMock
	mdsServiceMock := suite.mdsMock

	// set the expectations, do not call Submit() since command parsing failed in the first place
	mdsServiceMock.On("FailMessage", contextMock.Log(), *message.MessageId, mock.Anything).Return(nil)

	suite.mdsInteractor.processMessage(&message)

	mdsServiceMock.AssertExpectations(suite.T())
	mdsServiceMock.AssertNumberOfCalls(suite.T(), "FailMessage", 1)
	mdsServiceMock.AssertNumberOfCalls(suite.T(), "SendReply", 0)
	mdsServiceMock.AssertCalled(suite.T(), "FailMessage", contextMock.Log(), *message.MessageId, mock.Anything)
}

func (suite *MDSInteractorTestSuite) TestMDSInteractor_pollOnceWithGetMessagesReturnError() {
	contextMock := suite.contextMock
	mdsServiceMock := suite.mdsMock

	getMessageOutput := ssmmds.GetMessagesOutput{
		Destination:       &testDestination,
		Messages:          make([]*ssmmds.Message, 1),
		MessagesRequestId: &testMessageId,
	}
	mdsServiceMock.On("GetMessages", contextMock.Log(), mock.AnythingOfType("string")).Return(&getMessageOutput, fmt.Errorf("test"))

	suite.mdsInteractor.pollOnce()

	mdsServiceMock.AssertExpectations(suite.T())
	mdsServiceMock.AssertNumberOfCalls(suite.T(), "SendReply", 0)
	mdsServiceMock.AssertNumberOfCalls(suite.T(), "AcknowledgeMessage", 0)
	mdsServiceMock.AssertNumberOfCalls(suite.T(), "FailMessage", 0)
}

func (suite *MDSInteractorTestSuite) TestMDSInteractor_processMessageWithSendCommandTopicPrefix() {
	message := getSendCommandMessage()
	contextMock := suite.contextMock
	mdsServiceMock := suite.mdsMock

	messageHandlerMock := &mocks.IMessageHandler{}
	suite.mdsInteractor.messageHandler = messageHandlerMock
	messageHandlerMock.On("CheckProcessorPushAllowed", mock.Anything).Return(messagehandler.ErrorCode(""))
	messageHandlerMock.On("Submit", mock.Anything).Return(messagehandler.ErrorCode(""))

	// set the expectations, do not call Submit() since command parsing failed in the first place
	mdsServiceMock.On("AcknowledgeMessage", contextMock.Log(), *message.MessageId).Return(nil)
	mdsServiceMock.On("SendReply", contextMock.Log(), mock.AnythingOfType("string"), mock.AnythingOfType("string")).Return(nil)

	suite.mdsInteractor.processMessage(&message)
	// wait for message to be received by incomingChan

	mdsServiceMock.AssertExpectations(suite.T())
	mdsServiceMock.AssertNumberOfCalls(suite.T(), "SendReply", 1)
	mdsServiceMock.AssertNumberOfCalls(suite.T(), "AcknowledgeMessage", 1)
	mdsServiceMock.AssertNumberOfCalls(suite.T(), "FailMessage", 0)
	messageHandlerMock.AssertNumberOfCalls(suite.T(), "Submit", 1)
	mdsServiceMock.AssertCalled(suite.T(), "SendReply", contextMock.Log(), *message.MessageId, mock.AnythingOfType("string"))
}

func (suite *MDSInteractorTestSuite) TestMDSInteractor_pollOnceWithZeroMessage() {
	contextMock := suite.contextMock
	mdsServiceMock := suite.mdsMock

	getMessageOutput := ssmmds.GetMessagesOutput{
		Destination:       &testDestination,
		Messages:          make([]*ssmmds.Message, 0),
		MessagesRequestId: &testMessageId,
	}
	mdsServiceMock.On("GetMessages", contextMock.Log(), mock.AnythingOfType("string")).Return(&getMessageOutput, nil)

	suite.mdsInteractor.pollOnce()

	mdsServiceMock.AssertExpectations(suite.T())
	mdsServiceMock.AssertNumberOfCalls(suite.T(), "SendReply", 0)
	mdsServiceMock.AssertNumberOfCalls(suite.T(), "AcknowledgeMessage", 0)
	mdsServiceMock.AssertNumberOfCalls(suite.T(), "FailMessage", 0)
}

func (suite *MDSInteractorTestSuite) TestMDSInteractor_processMessageWithSendCommandTopicPrefixAndInvalidPayload() {
	var payload = "#invalid_json#"
	message := ssmmds.Message{
		CreatedDate: &testCreatedDate,
		Destination: &testDestination,
		MessageId:   &testMessageId,
		Topic:       &testTopicSend,
		Payload:     &payload,
	}
	contextMock := suite.contextMock
	mdsServiceMock := suite.mdsMock
	mdsServiceMock.On("SendReply", contextMock.Log(), mock.AnythingOfType("string"), mock.AnythingOfType("string")).Return(nil)

	suite.mdsInteractor.processMessage(&message)

	mdsServiceMock.AssertExpectations(suite.T())
	mdsServiceMock.AssertNumberOfCalls(suite.T(), "SendReply", 1)
	mdsServiceMock.AssertNumberOfCalls(suite.T(), "AcknowledgeMessage", 0)
	mdsServiceMock.AssertNumberOfCalls(suite.T(), "FailMessage", 0)
	mdsServiceMock.AssertCalled(suite.T(), "SendReply", contextMock.Log(), *message.MessageId, mock.AnythingOfType("string"))
}

func (suite *MDSInteractorTestSuite) TestMDSInteractor_pollOnceMultipleMessages() {
	contextMock := suite.contextMock
	mdsServiceMock := suite.mdsMock

	messageHandlerMock := &mocks.IMessageHandler{}
	suite.mdsInteractor.messageHandler = messageHandlerMock
	messageHandlerMock.On("CheckProcessorPushAllowed", mock.Anything).Return(messagehandler.ErrorCode(""))
	messageHandlerMock.On("Submit", mock.Anything).Return(messagehandler.ErrorCode(""))

	message1 := getCancelCommandMessage()
	message2 := getSendCommandMessage()
	message3 := getCancelCommandMessage()
	getMessageOutput := ssmmds.GetMessagesOutput{
		Destination:       &testDestination,
		Messages:          []*ssmmds.Message{&message1, &message2, &message3},
		MessagesRequestId: &testMessageId,
	}
	mdsServiceMock.On("GetMessages", contextMock.Log(), mock.AnythingOfType("string")).Return(&getMessageOutput, nil)
	mdsServiceMock.On("AcknowledgeMessage", contextMock.Log(), mock.AnythingOfType("string")).Return(nil)
	mdsServiceMock.On("SendReply", contextMock.Log(), mock.AnythingOfType("string"), mock.AnythingOfType("string")).Return(nil)

	suite.mdsInteractor.pollOnce()

	mdsServiceMock.AssertExpectations(suite.T())
	messageHandlerMock.AssertNumberOfCalls(suite.T(), "Submit", 3)
}

func (suite *MDSInteractorTestSuite) TestMDSInteractor_processMessageWithCancelCommandTopicPrefix() {
	message := getCancelCommandMessage()
	contextMock := suite.contextMock
	mdsServiceMock := suite.mdsMock

	// override incomingChan as multiple tests will be writing to single chan will make it difficult to verify the message
	messageHandlerMock := &mocks.IMessageHandler{}
	suite.mdsInteractor.messageHandler = messageHandlerMock
	messageHandlerMock.On("CheckProcessorPushAllowed", mock.Anything).Return(messagehandler.ErrorCode(""))
	messageHandlerMock.On("Submit", mock.Anything).Return(messagehandler.ErrorCode(""))

	// set the expectations, do not call Submit() since command parsing failed in the first place
	mdsServiceMock.On("AcknowledgeMessage", contextMock.Log(), *message.MessageId).Return(nil)
	mdsServiceMock.On("SendReply", contextMock.Log(), mock.AnythingOfType("string"), mock.AnythingOfType("string")).Return(nil)

	suite.mdsInteractor.processMessage(&message)
	// wait for message to be received by incomingChan

	mdsServiceMock.AssertExpectations(suite.T())
	mdsServiceMock.AssertNumberOfCalls(suite.T(), "SendReply", 1)
	mdsServiceMock.AssertNumberOfCalls(suite.T(), "AcknowledgeMessage", 1)
	mdsServiceMock.AssertNumberOfCalls(suite.T(), "FailMessage", 0)
	messageHandlerMock.AssertNumberOfCalls(suite.T(), "Submit", 1)
	mdsServiceMock.AssertCalled(suite.T(), "SendReply", contextMock.Log(), *message.MessageId, mock.AnythingOfType("string"))
}

func (suite *MDSInteractorTestSuite) TestMDSInteractor_pollOnce() {
	contextMock := suite.contextMock
	mdsServiceMock := suite.mdsMock
	message := getCancelCommandMessage()
	getMessageOutput := ssmmds.GetMessagesOutput{
		Destination:       &testDestination,
		Messages:          []*ssmmds.Message{&message},
		MessagesRequestId: &testMessageId,
	}
	messageHandlerMock := &mocks.IMessageHandler{}
	suite.mdsInteractor.messageHandler = messageHandlerMock
	messageHandlerMock.On("CheckProcessorPushAllowed", mock.Anything).Return(messagehandler.ErrorCode(""))
	messageHandlerMock.On("Submit", mock.Anything).Return(messagehandler.ErrorCode(""))

	mdsServiceMock.On("GetMessages", contextMock.Log(), mock.AnythingOfType("string")).Return(&getMessageOutput, nil)
	mdsServiceMock.On("AcknowledgeMessage", contextMock.Log(), *message.MessageId).Return(nil)
	mdsServiceMock.On("SendReply", contextMock.Log(), mock.AnythingOfType("string"), mock.AnythingOfType("string")).Return(nil)
	suite.mdsInteractor.pollOnce()
	mdsServiceMock.AssertExpectations(suite.T())
	mdsServiceMock.AssertNumberOfCalls(suite.T(), "SendReply", 1)
	mdsServiceMock.AssertNumberOfCalls(suite.T(), "AcknowledgeMessage", 1)
	mdsServiceMock.AssertNumberOfCalls(suite.T(), "FailMessage", 0)
	messageHandlerMock.AssertNumberOfCalls(suite.T(), "Submit", 1)
	mdsServiceMock.AssertCalled(suite.T(), "SendReply", contextMock.Log(), *message.MessageId, mock.AnythingOfType("string"))
}

func (suite *MDSInteractorTestSuite) TestMDSInteractor_Close() {
	mdsServiceMock := suite.mdsMock
	mdsServiceMock.On("Stop")
	mdsInteractor := MDSInteractor{
		context:              suite.contextMock,
		service:              suite.mdsMock,
		messagePollWaitGroup: &sync.WaitGroup{},
		processorStopPolicy:  sdkutil.NewStopPolicy(Name, testStopPolicyThreshold),
	}
	var sendReplyRetryNumber, messagePollRetryNumber *int
	mdsInteractor.sendReplyJob, sendReplyRetryNumber = setupTestScheduledJob()
	mdsInteractor.messagePollJob, messagePollRetryNumber = setupTestScheduledJob()
	time.Sleep(500 * time.Millisecond)
	mdsInteractor.PreProcessorClose()
	mdsServiceMock.AssertCalled(suite.T(), "Stop")
	assert.Equal(suite.T(), 1, *sendReplyRetryNumber)
	assert.Equal(suite.T(), 1, *messagePollRetryNumber)
}

func setupTestScheduledJob() (*scheduler.Job, *int) {
	called := 0
	job := func() {
		called++
	}
	scheduledJob, _ := scheduler.Every(1).Seconds().Run(job)
	return scheduledJob, &called
}

func getSendCommandMessage() ssmmds.Message {
	var sendCommandPayload = `{
	    "Parameters": null,
	    "DocumentContent": {
		    "schemaVersion": "2.2",
		    "description": "doc",
		    "runtimeConfig": null,
		    "mainSteps": [{"action": "aws:runShellScript", "name": "pluginLinux"}],
		    "parameters": null
	    },
	    "CommandId": "be8d9d4b-da53-4d2f-a96b-60aec17739af",
	    "DocumentName": "test",
	    "OutputS3KeyPrefix": "",
	    "OutputS3BucketName": "",
	    "CloudWatchOutputEnabled": "false"
    }`

	message := ssmmds.Message{
		CreatedDate: &testCreatedDate,
		Destination: &testDestination,
		MessageId:   &testMessageId,
		Topic:       &testTopicSend,
		Payload:     &sendCommandPayload,
	}
	return message
}

func getCancelCommandMessage() ssmmds.Message {
	var cancelCommandPayload = "{\"CancelMessageId\": \"be8d9d4b-da53-4d2f-a96b-60aec17739af\"}"
	message := ssmmds.Message{
		CreatedDate: &testCreatedDate,
		Destination: &testDestination,
		MessageId:   &testMessageId,
		Topic:       &testTopicCancel,
		Payload:     &cancelCommandPayload,
	}
	return message
}
