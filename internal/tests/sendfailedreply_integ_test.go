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

// Package tests represents stress and integration tests of the agent
package tests

import (
	"fmt"
	"github.com/stretchr/testify/assert"
	"os"
	"path"
	"runtime/debug"
	"strings"
	"testing"
	"time"

	"github.com/aws/amazon-ssm-agent/agent/agent"
	"github.com/aws/amazon-ssm-agent/agent/appconfig"
	"github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/aws/amazon-ssm-agent/agent/contracts"
	"github.com/aws/amazon-ssm-agent/agent/fileutil"
	"github.com/aws/amazon-ssm-agent/agent/framework/coremanager"
	"github.com/aws/amazon-ssm-agent/agent/jsonutil"
	"github.com/aws/amazon-ssm-agent/agent/log"
	logger "github.com/aws/amazon-ssm-agent/agent/log/ssmlog"
	mds "github.com/aws/amazon-ssm-agent/agent/runcommand/mds"
	"github.com/aws/amazon-ssm-agent/internal/tests/testdata"
	"github.com/aws/amazon-ssm-agent/internal/tests/testutils"
	"github.com/aws/aws-sdk-go/aws/request"
	"github.com/aws/aws-sdk-go/service/ssmmds"
	mdssdkmock "github.com/aws/aws-sdk-go/service/ssmmds/ssmmdsiface/mocks"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
)

// SendFailedReplyTestSuite defines test suite for saving SendReplyInput that failed sending to MDS
type SendFailedReplyTestSuite struct {
	suite.Suite
	ssmAgent   agent.ISSMAgent
	mdsSdkMock *mdssdkmock.SSMMDSAPI
	log        log.T
}

// SetupTest makes sure that all the components referenced in the test case are initialized
// before each test
func (suite *SendFailedReplyTestSuite) SetupTest() {
	log := logger.SSMLogger(true)
	suite.log = log

	config, err := appconfig.Config(true)
	if err != nil {
		log.Debugf("appconfig could not be loaded - %v", err)
		return
	}
	context := context.Default(log, config)

	// Mock mds sdk, sendRequest should return error only in case of sending reply to MDS
	sendMdsSdkRequest := func(req *request.Request) error {
		switch req.Params.(type) {
		case *ssmmds.SendReplyInput:
			return fmt.Errorf("can't send reply")
		default:
			return nil
		}
	}
	mdsSdkMock := testutils.NewMdsSdkMock()
	mdsService := testutils.NewMdsService(mdsSdkMock, sendMdsSdkRequest)
	suite.mdsSdkMock = mdsSdkMock

	// The actual runcommand core module with mocked MDS service injected
	runcommandService := testutils.NewRuncommandService(context, mdsService)
	var modules []contracts.ICoreModule
	modules = append(modules, runcommandService)

	// Create core manager that accepts runcommand core module
	var cpm *coremanager.CoreManager
	if cpm, err = testutils.NewCoreManager(context, &modules, log); err != nil {
		log.Errorf("error occurred when starting core manager: %v", err)
		return
	}
	// Create core ssm agent
	suite.ssmAgent = &agent.SSMAgent{}
	suite.ssmAgent.SetContext(context)
	suite.ssmAgent.SetCoreManager(cpm)
}

func (suite *SendFailedReplyTestSuite) TearDownSuite() {
	// Close the log only after the all tests are done.
	suite.log.Close()
}

func cleanUpTest(suite *SendFailedReplyTestSuite) {
	// recover in case the agent panics
	// this should handle some kind of seg fault errors.
	if msg := recover(); msg != nil {
		suite.T().Errorf("Agent crashed with message %v!", msg)
		suite.T().Errorf("%s: %s", msg, debug.Stack())
	}
	//Empty the replies folder
	repliesDirectory := mds.GetFailedReplyDirectory()
	files, _ := fileutil.GetFileNames(repliesDirectory)
	for _, file := range files {
		fileutil.DeleteFile(path.Join(repliesDirectory, file))
	}
	// flush the log to get full logs after the test is done, don't close the log unless all tests are done
	suite.log.Flush()
}

//TestSaveFailedReply tests the agent saves mds reply to disk if it failed sending it
func (suite *SendFailedReplyTestSuite) TestSaveFailedReply() {

	// Mock MDs service so it returns only one messages, it'll return empty messages after that.
	suite.mdsSdkMock.On("GetMessagesRequest", mock.AnythingOfType("*ssmmds.GetMessagesInput")).Return(&request.Request{}, func(input *ssmmds.GetMessagesInput) *ssmmds.GetMessagesOutput {
		messageOutput, _ := testutils.GenerateMessages(testdata.EchoMDSMessage)
		return messageOutput
	}, nil).Times(1)

	suite.mdsSdkMock.On("GetMessagesRequest", mock.AnythingOfType("*ssmmds.GetMessagesInput")).Return(&request.Request{}, func(input *ssmmds.GetMessagesInput) *ssmmds.GetMessagesOutput {
		emptyMessage, _ := testutils.GenerateEmptyMessage()
		return emptyMessage
	}, nil)

	// Mock sendReplyRequest to capture the first replyid and verify later that it has been saved to disk
	// Explicitly set the input of the http request to SendReplyInput so we can detect it later in sendRequest
	// and fail the request
	httpSendReplyRequest := &request.Request{Params: &ssmmds.SendReplyInput{}}
	var replyId string
	suite.mdsSdkMock.On("SendReplyRequest", mock.AnythingOfType("*ssmmds.SendReplyInput")).Return(httpSendReplyRequest, func(input *ssmmds.SendReplyInput) *ssmmds.SendReplyOutput {
		replyId = *input.ReplyId
		suite.T().Logf("Test is sending reply %v", replyId)
		return &ssmmds.SendReplyOutput{}
	}).Times(1)
	suite.mdsSdkMock.On("SendReplyRequest", mock.AnythingOfType("*ssmmds.SendReplyInput")).Return(httpSendReplyRequest, &ssmmds.SendReplyOutput{})

	defer func() {
		cleanUpTest(suite)
	}()

	// foundReply is a channel that gets set to true if reply was saved locally
	foundReply := make(chan bool)

	suite.ssmAgent.Start()

	// Launch go routine to check if the reply has been sent, sleep 4 seconds so the agent can execute the document
	// and writes the request locally
	go func() {
		found := false
		for i := 0; i < 40; i++ {
			files, _ := fileutil.GetFileNames(mds.GetFailedReplyDirectory())
			for _, file := range files {
				if strings.HasPrefix(file, replyId) {
					found = true
				}
			}
			time.Sleep(100 * time.Millisecond)
		}
		foundReply <- found
	}()

	// block test execution untill the failed SendReply request gets saved locally
	switch <-foundReply {
	case true:
		suite.T().Logf("Found reply %v on disk", replyId)
	case false:
		suite.T().Errorf("Reply wasn't written on disk")
	}

	// stop agent execution
	suite.ssmAgent.Stop()
}

//TestSendFailedReply tests the agent sends back to the service the saved mds reply on disk
func (suite *SendFailedReplyTestSuite) TestSendFailedReply() {
	//Save test send reply input on disk
	t := time.Now().UTC()
	fileName := fmt.Sprintf("%v_%v", testdata.TestReplyId, t.Format("2006-01-02T15-04-05"))
	absoluteFileName := path.Join(mds.GetFailedReplyDirectory(), fileName)
	if s, err := fileutil.WriteIntoFileWithPermissions(absoluteFileName, jsonutil.Indent(testdata.TestSendReplyInput), os.FileMode(int(appconfig.ReadWriteAccess))); s && err == nil {
		suite.T().Logf("successfully persisted reply in %v", absoluteFileName)
	} else {
		suite.T().Errorf("persisting reply in %v failed with error %v", absoluteFileName, err)
		suite.T().FailNow()
	}

	defer func() {
		cleanUpTest(suite)
	}()

	// Mock MDs service to return empty messages.
	suite.mdsSdkMock.On("GetMessagesRequest", mock.AnythingOfType("*ssmmds.GetMessagesInput")).Return(&request.Request{}, func(input *ssmmds.GetMessagesInput) *ssmmds.GetMessagesOutput {
		emptyMessage, _ := testutils.GenerateEmptyMessage()
		return emptyMessage
	}, nil)

	// sentReply is a channel that gets set to true if saved reply was sent bback to MDS
	sentReply := make(chan bool)

	// Mock sendReplyRequest to capture the replyid and verify later that it is equal to the saved reply on disk
	suite.mdsSdkMock.On("SendReplyRequest", mock.AnythingOfType("*ssmmds.SendReplyInput")).Return(&request.Request{}, func(input *ssmmds.SendReplyInput) *ssmmds.SendReplyOutput {
		replyId := *input.ReplyId
		suite.T().Logf("Test is sending reply %v", replyId)
		if replyId == testdata.TestReplyId {
			sentReply <- true
		}
		return &ssmmds.SendReplyOutput{}
	})

	suite.ssmAgent.Start()
	// block test execution
	<-sentReply
	// stop agent execution
	suite.ssmAgent.Stop()
}

//TestSendFailedReply tests the agent sends back to the service the saved mds reply on disk
func (suite *SendFailedReplyTestSuite) TestDeleteOldFailedReply() {
	//Save test send reply input on disk
	fileName := fmt.Sprintf("%v_%v", testdata.TestReplyId, "2006-01-02T15-04-05")
	absoluteFileName := path.Join(mds.GetFailedReplyDirectory(), fileName)
	if s, err := fileutil.WriteIntoFileWithPermissions(absoluteFileName, jsonutil.Indent(testdata.TestSendReplyInput), os.FileMode(int(appconfig.ReadWriteAccess))); s && err == nil {
		suite.T().Logf("successfully persisted reply in %v", absoluteFileName)
	} else {
		suite.T().Errorf("persisting reply in %v failed with error %v", absoluteFileName, err)
		suite.T().FailNow()
	}

	defer func() {
		cleanUpTest(suite)
	}()

	// Mock MDs service to return empty messages.
	suite.mdsSdkMock.On("GetMessagesRequest", mock.AnythingOfType("*ssmmds.GetMessagesInput")).Return(&request.Request{}, func(input *ssmmds.GetMessagesInput) *ssmmds.GetMessagesOutput {
		emptyMessage, _ := testutils.GenerateEmptyMessage()
		return emptyMessage
	}, nil)

	// Mock sendReplyRequest to capture the replyid and verify later that it is equal to the saved reply on disk
	suite.mdsSdkMock.On("SendReplyRequest", mock.AnythingOfType("*ssmmds.SendReplyInput")).Return(&request.Request{}, func(input *ssmmds.SendReplyInput) *ssmmds.SendReplyOutput {
		replyId := *input.ReplyId
		suite.T().Logf("Test is sending reply %v", replyId)
		assert.NotEqual(suite.T(), replyId, testdata.TestReplyId, "Agent should not send old sendReplyInput")
		return &ssmmds.SendReplyOutput{}
	})

	// replyDeleted is a channel that gets set to true if saved reply was deleted
	replyDeleted := make(chan bool)

	suite.ssmAgent.Start()

	// Launch go routine to check if the old sendReplyInput was deleted from disk
	go func() {
		for i := 0; i < 40; i++ {
			files, _ := fileutil.GetFileNames(mds.GetFailedReplyDirectory())
			found := false
			for _, file := range files {
				if strings.HasPrefix(file, testdata.TestReplyId) {
					found = true
				}
			}
			if !found {
				replyDeleted <- true
				break
			}
			time.Sleep(100 * time.Millisecond)
		}
		replyDeleted <- false
	}()

	// block test execution until the failed SendReply request gets saved locally
	switch <-replyDeleted {
	case true:
		suite.T().Logf("Saves reply %v was successfully deleted from disk", testdata.TestReplyId)
	case false:
		suite.T().Errorf("Reply didn't get deleted from on disk")
	}

	// stop agent execution
	suite.ssmAgent.Stop()
}

func TestSendFailedReplyIntegTestSuite(t *testing.T) {
	suite.Run(t, new(SendFailedReplyTestSuite))
}
