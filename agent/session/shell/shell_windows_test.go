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

//go:build windows && e2e
// +build windows,e2e

// Package shell implements session shell plugin.
package shell

import (
	"bytes"
	"errors"
	"io/ioutil"
	"os"
	"time"

	"github.com/aws/amazon-ssm-agent/agent/contracts"
	"github.com/aws/amazon-ssm-agent/agent/log"
	mgsConfig "github.com/aws/amazon-ssm-agent/agent/session/config"
	mgsContracts "github.com/aws/amazon-ssm-agent/agent/session/contracts"
	"github.com/aws/amazon-ssm-agent/agent/task"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/twinj/uuid"
)

// Testing Execute
func (suite *ShellTestSuite) TestExecuteWhenCancelFlagIsShutDown() {
	suite.mockCancelFlag.On("ShutDown").Return(true)
	suite.mockIohandler.On("MarkAsShutdown").Return(nil)

	suite.plugin.Execute(
		contracts.Configuration{},
		suite.mockCancelFlag,
		suite.mockIohandler,
		suite.mockDataChannel,
		mgsContracts.ShellProperties{})

	suite.mockCancelFlag.AssertExpectations(suite.T())
	suite.mockIohandler.AssertExpectations(suite.T())
}

// Testing Execute
func (suite *ShellTestSuite) TestExecuteWhenCancelFlagIsCancelled() {
	suite.mockCancelFlag.On("Canceled").Return(true)
	suite.mockCancelFlag.On("ShutDown").Return(false)
	suite.mockIohandler.On("MarkAsCancelled").Return(nil)

	suite.plugin.Execute(
		contracts.Configuration{},
		suite.mockCancelFlag,
		suite.mockIohandler,
		suite.mockDataChannel,
		mgsContracts.ShellProperties{})

	suite.mockCancelFlag.AssertExpectations(suite.T())
	suite.mockIohandler.AssertExpectations(suite.T())
}

// Testing Execute
func (suite *ShellTestSuite) TestExecute() {
	suite.mockCancelFlag.On("Canceled").Return(false)
	suite.mockCancelFlag.On("ShutDown").Return(false)
	suite.mockCancelFlag.On("Wait").Return(task.Completed)
	suite.mockIohandler.On("SetExitCode", 0).Return(nil)
	suite.mockIohandler.On("SetStatus", contracts.ResultStatusSuccess).Return()
	suite.mockIohandler.On("SetOutput", mock.Anything).Return()
	suite.mockDataChannel.On("IsActive").Return(true)
	suite.mockDataChannel.On("PrepareToCloseChannel", mock.Anything).Return(nil).Times(1)
	suite.mockDataChannel.On("SendAgentSessionStateMessage", mock.Anything, mgsContracts.Terminating).
		Return(nil).Times(1)
	suite.mockCmd.On("Wait").Return(nil)
	suite.mockCmd.On("Pid").Return(234)

	stdout, stdin, _ := os.Pipe()
	stdin.Write(payload)
	getCommandExecutor = func(log log.T, shellProps mgsContracts.ShellProperties, isSessionLogger bool, config contracts.Configuration, plugin *ShellPlugin) (err error) {
		plugin.stdin = stdin
		plugin.stdout = stdout
		plugin.execCmd = suite.mockCmd
		return nil
	}

	plugin := &ShellPlugin{
		context:     suite.mockContext,
		stdout:      stdout,
		dataChannel: suite.mockDataChannel,
		execCmd:     suite.mockCmd,
	}

	plugin.Execute(
		contracts.Configuration{},
		suite.mockCancelFlag,
		suite.mockIohandler,
		suite.mockDataChannel,
		mgsContracts.ShellProperties{})

	suite.mockCancelFlag.AssertExpectations(suite.T())
	suite.mockIohandler.AssertExpectations(suite.T())
	suite.mockDataChannel.AssertExpectations(suite.T())
	suite.mockCmd.AssertExpectations(suite.T())

	stdin.Close()
	stdout.Close()
}

// Testing Execute when CW logging(at the end of the session) is enabled
func (suite *ShellTestSuite) TestExecuteWithCWLoggingEnabled() {
	suite.mockCancelFlag.On("Canceled").Return(false)
	suite.mockCancelFlag.On("ShutDown").Return(false)
	suite.mockCancelFlag.On("Wait").Return(task.Completed)
	suite.mockIohandler.On("SetExitCode", 0).Return(nil)
	suite.mockIohandler.On("SetStatus", contracts.ResultStatusSuccess).Return()
	suite.mockIohandler.On("SetOutput", mock.Anything).Return()
	suite.mockDataChannel.On("PrepareToCloseChannel", mock.Anything).Return(nil).Times(1)
	suite.mockDataChannel.On("SendAgentSessionStateMessage", mock.Anything, mgsContracts.Terminating).
		Return(nil).Times(1)
	suite.mockDataChannel.On("IsActive").Return(true)

	stdout, stdin, _ := os.Pipe()
	stdin.Write(payload)
	getCommandExecutor = func(log log.T, shellProps mgsContracts.ShellProperties, isSessionLogger bool, config contracts.Configuration, plugin *ShellPlugin) (err error) {
		plugin.stdin = stdin
		plugin.stdout = stdout
		return nil
	}

	// When CW logging is enabled with streaming disabled then IsFileComplete is expected to be true since log to CW is uploaded once at the end of the session
	expectedIsFileComplete := true
	suite.mockCWL.On("IsLogGroupPresent", testCwLogGroupName).Return(true, &testCwlLogGroup)
	suite.mockCWL.On("StreamData", testCwLogGroupName, sessionId, sessionId+mgsConfig.LogFileExtension, expectedIsFileComplete, false, mock.Anything, false, false).Return(true)

	suite.plugin.Execute(
		contracts.Configuration{
			CloudWatchLogGroup: testCwLogGroupName,
			SessionId:          sessionId,
			SessionOwner:       sessionOwner,
		},
		suite.mockCancelFlag,
		suite.mockIohandler,
		suite.mockDataChannel,
		mgsContracts.ShellProperties{})

	suite.mockCancelFlag.AssertExpectations(suite.T())
	suite.mockIohandler.AssertExpectations(suite.T())
	suite.mockDataChannel.AssertExpectations(suite.T())
	suite.mockCWL.AssertExpectations(suite.T())
	assert.Equal(suite.T(), false, suite.plugin.logger.streamLogsToCloudWatch)

	stdin.Close()
	stdout.Close()
}

// Testing Execute when near real time CW log streaming is enabled
func (suite *ShellTestSuite) TestExecuteWithCWLogStreamingEnabled() {
	suite.mockCancelFlag.On("Canceled").Return(false)
	suite.mockCancelFlag.On("ShutDown").Return(false)
	suite.mockCancelFlag.On("Wait").Return(task.Completed)
	suite.mockIohandler.On("SetExitCode", 0).Return(nil)
	suite.mockIohandler.On("SetStatus", contracts.ResultStatusSuccess).Return()
	suite.mockIohandler.On("SetOutput", mock.Anything).Return()
	suite.mockDataChannel.On("PrepareToCloseChannel", mock.Anything).Return(nil).Times(1)
	suite.mockDataChannel.On("SendAgentSessionStateMessage", mock.Anything, mgsContracts.Terminating).
		Return(nil).Times(1)
	suite.mockDataChannel.On("IsActive").Return(true)

	stdout, stdin, _ := os.Pipe()
	defer stdin.Close()
	defer stdout.Close()

	stdin.Write(payload)
	getCommandExecutor = func(log log.T, shellProps mgsContracts.ShellProperties, isSessionLogger bool, config contracts.Configuration, plugin *ShellPlugin) (err error) {
		plugin.stdin = stdin
		plugin.stdout = stdout
		return nil
	}

	// When CW log streaming is enabled then IsFileComplete is expected to be false since log to CW will uploaded periodically since the beginning of the session
	suite.mockCWL.On("IsLogGroupPresent", testCwLogGroupName).Return(true, &testCwlLogGroup)
	suite.mockDataChannel.On("GetRegion").Return("region")
	suite.mockDataChannel.On("GetInstanceId").Return("instanceId")

	checkForLoggingInterruption = func(log log.T, ipcFile *os.File, plugin *ShellPlugin) {}
	go func() {
		<-suite.plugin.logger.ptyTerminated
		suite.plugin.logger.cloudWatchStreamingFinished <- true
	}()

	suite.plugin.Execute(
		contracts.Configuration{
			CloudWatchLogGroup:         testCwLogGroupName,
			CloudWatchStreamingEnabled: true,
			SessionId:                  sessionId,
			SessionOwner:               sessionOwner,
		},
		suite.mockCancelFlag,
		suite.mockIohandler,
		suite.mockDataChannel,
		mgsContracts.ShellProperties{})

	suite.mockCancelFlag.AssertExpectations(suite.T())
	suite.mockIohandler.AssertExpectations(suite.T())
	suite.mockDataChannel.AssertExpectations(suite.T())
	suite.mockCWL.AssertExpectations(suite.T())
	assert.Equal(suite.T(), true, suite.plugin.logger.streamLogsToCloudWatch)
}

// Testing Execute when CW logging is disabled but streaming is enabled
func (suite *ShellTestSuite) TestExecuteWithCWLoggingDisabledButStreamingEnabled() {
	suite.mockCancelFlag.On("Canceled").Return(false)
	suite.mockCancelFlag.On("ShutDown").Return(false)
	suite.mockCancelFlag.On("Wait").Return(task.Completed)
	suite.mockIohandler.On("SetExitCode", 0).Return(nil)
	suite.mockIohandler.On("SetStatus", contracts.ResultStatusSuccess).Return()
	suite.mockIohandler.On("SetOutput", mock.Anything).Return()
	suite.mockDataChannel.On("PrepareToCloseChannel", mock.Anything).Return(nil).Times(1)
	suite.mockDataChannel.On("SendAgentSessionStateMessage", mock.Anything, mgsContracts.Terminating).
		Return(nil).Times(1)
	suite.mockDataChannel.On("IsActive").Return(true)

	stdout, stdin, _ := os.Pipe()
	stdin.Write(payload)
	getCommandExecutor = func(log log.T, shellProps mgsContracts.ShellProperties, isSessionLogger bool, config contracts.Configuration, plugin *ShellPlugin) (err error) {
		plugin.stdin = stdin
		plugin.stdout = stdout
		return nil
	}

	suite.plugin.Execute(
		contracts.Configuration{
			CloudWatchLogGroup:         "",
			CloudWatchStreamingEnabled: true,
			SessionId:                  sessionId,
			SessionOwner:               sessionOwner,
		},
		suite.mockCancelFlag,
		suite.mockIohandler,
		suite.mockDataChannel,
		mgsContracts.ShellProperties{})

	suite.mockCancelFlag.AssertExpectations(suite.T())
	suite.mockIohandler.AssertExpectations(suite.T())
	suite.mockDataChannel.AssertExpectations(suite.T())
	assert.Equal(suite.T(), false, suite.plugin.logger.streamLogsToCloudWatch)

	stdin.Close()
	stdout.Close()
}

// Testing Execute when cancel flag is set
func (suite *ShellTestSuite) TestExecuteWithCancelFlag() {
	suite.mockCancelFlag.On("Canceled").Return(false).Once()
	suite.mockCancelFlag.On("Canceled").Return(true).Once()
	suite.mockCancelFlag.On("ShutDown").Return(false)
	suite.mockCancelFlag.On("Wait").Return(task.Completed)
	suite.mockIohandler.On("SetExitCode", 0).Return(nil)
	suite.mockIohandler.On("SetStatus", contracts.ResultStatusSuccess).Return()
	suite.mockIohandler.On("SetOutput", mock.Anything).Return()
	suite.mockDataChannel.On("PrepareToCloseChannel", mock.Anything).Return(nil).Times(0)
	suite.mockDataChannel.On("SendAgentSessionStateMessage", mock.Anything, mgsContracts.Terminating).
		Return(nil).Times(0)
	suite.mockDataChannel.On("IsActive").Return(true)
	suite.mockCmd.On("Wait").Return(nil)
	suite.mockCmd.On("Kill").Return(nil)

	stdout, stdin, _ := os.Pipe()
	stdin.Write(payload)
	getCommandExecutor = func(log log.T, shellProps mgsContracts.ShellProperties, isSessionLogger bool, config contracts.Configuration, plugin *ShellPlugin) (err error) {
		plugin.stdin = stdin
		plugin.stdout = stdout
		plugin.execCmd = suite.mockCmd
		return nil
	}

	plugin := &ShellPlugin{
		context:     suite.mockContext,
		stdout:      stdout,
		dataChannel: suite.mockDataChannel,
		execCmd:     suite.mockCmd,
	}

	plugin.Execute(
		contracts.Configuration{},
		suite.mockCancelFlag,
		suite.mockIohandler,
		suite.mockDataChannel,
		mgsContracts.ShellProperties{})

	suite.mockCancelFlag.AssertExpectations(suite.T())
	suite.mockIohandler.AssertExpectations(suite.T())
	suite.mockCmd.AssertExpectations(suite.T())

	stdin.Close()
	stdout.Close()
}

// Testing writepump separately
func (suite *ShellTestSuite) TestWritePump() {
	stdout, stdin, _ := os.Pipe()
	stdin.Write(payload)

	//suite.mockDataChannel := &dataChannelMock.IDataChannel{}
	suite.mockDataChannel.On("SendStreamDataMessage", mock.Anything, mock.Anything, payload).Return(nil)
	suite.mockDataChannel.On("IsActive").Return(true)

	suite.plugin.stdout = stdout
	suite.plugin.logger = logger{ipcFilePath: "test.log"}

	// Create ipc file
	ipcFile, _ := os.Create(suite.plugin.logger.ipcFilePath)
	defer ipcFile.Close()

	// Spawning a separate go routine to close read and write pipes after a few seconds.
	// This is required as plugin.writePump() has a for loop which will continuosly read data from pipe until it is closed.
	go func() {
		time.Sleep(1800 * time.Millisecond)
		stdin.Close()
		stdout.Close()
	}()
	suite.plugin.writePump(suite.mockLog, ipcFile, 1)

	// Assert if SendStreamDataMessage function was called with same data from stdout
	suite.mockDataChannel.AssertExpectations(suite.T())
}

// Testing writepump for scenario when shell can give non utf8 characters
func (suite *ShellTestSuite) TestWritePumpForInvalidUtf8Character() {
	// invalidUtf8Payload contains 200 which is an invalid utf8 character
	invalidUtf8Payload := []byte{72, 200, 108, 108, 111}

	stdout, stdin, _ := os.Pipe()
	stdin.Write(invalidUtf8Payload)

	//suite.mockDataChannel := &dataChannelMock.IDataChannel{}
	suite.mockDataChannel.On("SendStreamDataMessage", mock.Anything, mock.Anything, invalidUtf8Payload).Return(nil)
	suite.mockDataChannel.On("IsActive").Return(true)

	suite.plugin.stdout = stdout
	suite.plugin.logger = logger{ipcFilePath: "test.log"}

	// Create ipc file
	ipcFile, _ := os.Create(suite.plugin.logger.ipcFilePath)
	defer ipcFile.Close()

	// Spawning a separate go routine to close read and write pipes after a few seconds.
	// This is required as plugin.writePump() has a for loop which will continuosly read data from pipe until it is closed.
	go func() {
		time.Sleep(1800 * time.Millisecond)
		stdin.Close()
		stdout.Close()
	}()
	suite.plugin.writePump(suite.mockLog, ipcFile, 1)

	// Assert if SendStreamDataMessage function was called with same data from stdout
	suite.mockDataChannel.AssertExpectations(suite.T())
}

// TestProcessStdoutData tests stdout bytes containing utf8 encoded characters
func (suite *ShellTestSuite) TestProcessStdoutData() {
	stdoutBytes := []byte("\x80 is a utf8 character.\xc9")
	var unprocessedBuf bytes.Buffer
	unprocessedBuf.Write([]byte("\xc8"))

	tmpFolder, _ := ioutil.TempDir("", "")
	defer os.RemoveAll(tmpFolder)

	file, _ := ioutil.TempFile(tmpFolder, "file")
	defer file.Close()

	suite.mockDataChannel.On("SendStreamDataMessage", suite.mockLog, mgsContracts.Output, []byte("Ȁ is a utf8 character.")).Return(nil)
	outputBuf, err := suite.plugin.processStdoutData(suite.mockLog, stdoutBytes, len(stdoutBytes), unprocessedBuf, file, mgsContracts.Output)

	suite.mockDataChannel.AssertExpectations(suite.T())
	assert.Equal(suite.T(), []byte("\xc9"), outputBuf.Bytes())
	assert.Nil(suite.T(), err)
}

func (suite *ShellTestSuite) TestProcessStreamMessage() {
	tmpFolder, _ := ioutil.TempDir("", "")
	defer os.RemoveAll(tmpFolder)

	stdinFile, _ := ioutil.TempFile(tmpFolder, "stdin")
	defer stdinFile.Close()
	stdoutFile, _ := ioutil.TempFile(tmpFolder, "stdout")
	defer stdoutFile.Close()

	suite.plugin.stdin = stdinFile
	suite.plugin.stdout = stdoutFile
	agentMessage := getAgentMessage(uint32(mgsContracts.Output), payload)
	suite.plugin.InputStreamMessageHandler(mockLog, *agentMessage)

	stdinFileContent, _ := ioutil.ReadFile(stdinFile.Name())
	assert.Equal(suite.T(), "testPayload", string(stdinFileContent))
}

// getAgentMessage constructs and returns AgentMessage with given sequenceNumber, messageType & payload
func getAgentMessage(payloadType uint32, payload []byte) *mgsContracts.AgentMessage {
	messageUUID, _ := uuid.Parse(messageId)
	agentMessage := mgsContracts.AgentMessage{
		MessageType:    mgsContracts.InputStreamDataMessage,
		SchemaVersion:  schemaVersion,
		CreatedDate:    createdDate,
		SequenceNumber: 1,
		Flags:          2,
		MessageId:      messageUUID,
		PayloadType:    payloadType,
		Payload:        payload,
	}
	return &agentMessage
}

func (suite *ShellTestSuite) TestValidateCWLogGroupDoesNotExistWithEncryptionEnabled() {
	configuration := contracts.Configuration{
		OutputS3BucketName:          "",
		S3EncryptionEnabled:         false,
		CloudWatchLogGroup:          testCwLogGroupName,
		CloudWatchEncryptionEnabled: true,
	}

	// When cw log group is missing and encryption is enabled
	suite.mockCWL.On("IsLogGroupPresent", testCwLogGroupName).Return(false, &testCwlLogGroup)
	suite.mockCWL.On("IsLogGroupEncryptedWithKMS", mock.Anything).Return(true, nil)
	err := suite.plugin.validate(configuration)
	assert.Nil(suite.T(), err)
}

func (suite *ShellTestSuite) TestValidateCWLogGroupDoesNotExistWithEncryptionDisabled() {
	configuration := contracts.Configuration{
		OutputS3BucketName:          "",
		S3EncryptionEnabled:         false,
		CloudWatchLogGroup:          testCwLogGroupName,
		CloudWatchEncryptionEnabled: false,
	}

	// When cw log group is missing and encryption is disabled
	suite.mockCWL.On("IsLogGroupPresent", testCwLogGroupName).Return(false, &testCwlLogGroup)
	err := suite.plugin.validate(configuration)
	assert.Nil(suite.T(), err)
}

func (suite *ShellTestSuite) TestValidateCWLogGroupNotEncrypted() {
	configuration := contracts.Configuration{
		OutputS3BucketName:          "",
		S3EncryptionEnabled:         false,
		CloudWatchLogGroup:          testCwLogGroupName,
		CloudWatchEncryptionEnabled: true,
	}

	// When cw log group is not encrypted, validate returns error
	suite.mockCWL.On("IsLogGroupPresent", testCwLogGroupName).Return(true, &testCwlLogGroup)
	suite.mockCWL.On("IsLogGroupEncryptedWithKMS", &testCwlLogGroup).Return(false, nil)
	err := suite.plugin.validate(configuration)
	assert.NotNil(suite.T(), err)
}

func (suite *ShellTestSuite) TestValidateCWLogGroupEncrypted() {
	testCwLogGroupName := "testCW"
	configuration := contracts.Configuration{
		OutputS3BucketName:          "",
		CloudWatchLogGroup:          testCwLogGroupName,
		CloudWatchEncryptionEnabled: true,
	}

	// When cw log group is encrypted and CreateLogStream succeed, validate returns nil
	suite.mockCWL.On("IsLogGroupPresent", testCwLogGroupName).Return(true, &testCwlLogGroup)
	suite.mockCWL.On("IsLogGroupEncryptedWithKMS", &testCwlLogGroup).Return(true, nil)
	suite.mockCWL.On("CreateLogStream", testCwLogGroupName, mock.Anything).Return(nil)
	err := suite.plugin.validate(configuration)
	assert.Nil(suite.T(), err)
}

func (suite *ShellTestSuite) TestValidateBypassCWLogGroupEncryptionCheck() {
	testCwLogGroupName := "testCW"
	configuration := contracts.Configuration{
		OutputS3BucketName:          "",
		S3EncryptionEnabled:         false,
		CloudWatchLogGroup:          testCwLogGroupName,
		CloudWatchEncryptionEnabled: false,
	}

	// When cw log group is not encrypted but we choose to bypass encryption check, validate returns true
	suite.mockCWL.On("IsLogGroupPresent", testCwLogGroupName).Return(true, &testCwlLogGroup)
	suite.mockCWL.On("IsLogGroupEncryptedWithKMS", &testCwlLogGroup).Return(false, nil)
	suite.mockCWL.On("CreateLogStream", testCwLogGroupName, mock.Anything).Return(nil)
	err := suite.plugin.validate(configuration)
	assert.Nil(suite.T(), err)
}

func (suite *ShellTestSuite) TestValidateGetCWLogGroupFailed() {
	testCwLogGroupName := "testCW"
	configuration := contracts.Configuration{
		OutputS3BucketName:          "",
		CloudWatchLogGroup:          testCwLogGroupName,
		CloudWatchEncryptionEnabled: true,
	}

	// When get cw log group is failed, validate returns error
	suite.mockCWL.On("IsLogGroupPresent", testCwLogGroupName).Return(true, &testCwlLogGroup)
	suite.mockCWL.On("IsLogGroupEncryptedWithKMS", &testCwlLogGroup).Return(false, errors.New("unable to get log groups"))
	suite.mockCWL.On("CreateLogStream", testCwLogGroupName, mock.Anything).Return(nil)
	err := suite.plugin.validate(configuration)
	assert.NotNil(suite.T(), err)
}

func (suite *ShellTestSuite) TestValidateS3BucketNotEncrypted() {
	testS3BucketName := "testS3"
	configuration := contracts.Configuration{
		OutputS3BucketName:  testS3BucketName,
		CloudWatchLogGroup:  "",
		S3EncryptionEnabled: true,
	}

	// When s3 bucket is not encrypted, validate returns error
	suite.mockS3.On("IsBucketEncrypted", mock.Anything, testS3BucketName).Return(false, nil)
	err := suite.plugin.validate(configuration)
	assert.NotNil(suite.T(), err)
}

func (suite *ShellTestSuite) TestValidateS3BucketEncrypted() {
	testS3BucketName := "testS3"
	configuration := contracts.Configuration{
		OutputS3BucketName:  testS3BucketName,
		CloudWatchLogGroup:  "",
		S3EncryptionEnabled: true,
	}

	// When s3 bucket is encrypted, validate returns nil
	suite.mockS3.On("IsBucketEncrypted", mock.Anything, testS3BucketName).Return(true, nil)
	err := suite.plugin.validate(configuration)
	assert.Nil(suite.T(), err)
}

func (suite *ShellTestSuite) TestValidateBypassS3BucketEncryptionCheck() {
	testS3BucketName := "testS3"
	configuration := contracts.Configuration{
		OutputS3BucketName:  testS3BucketName,
		CloudWatchLogGroup:  "",
		S3EncryptionEnabled: false,
	}

	// When s3 bucket is not encrypted but choose to bypass encryption check, validate returns nil
	suite.mockS3.On("IsBucketEncrypted", mock.Anything, testS3BucketName).Return(false, nil)
	err := suite.plugin.validate(configuration)
	assert.Nil(suite.T(), err)
}

func (suite *ShellTestSuite) TestValidateGetS3BucketEncryptionFailed() {
	testS3BucketName := "testS3"
	configuration := contracts.Configuration{
		OutputS3BucketName:  testS3BucketName,
		CloudWatchLogGroup:  "",
		S3EncryptionEnabled: true,
	}

	// When agent failed to get s3 bucket encryption, validate returns error
	suite.mockS3.On("IsBucketEncrypted", mock.Anything, testS3BucketName).Return(false, errors.New("get encryption failed"))
	err := suite.plugin.validate(configuration)
	assert.NotNil(suite.T(), err)
}

// buildAgentMessage constructs and returns AgentMessage with payload type and payload
func buildAgentMessage(payloadType uint32, payload []byte) mgsContracts.AgentMessage {
	agentMessage := mgsContracts.AgentMessage{
		MessageType:    mgsContracts.InputStreamDataMessage,
		SequenceNumber: 1,
		PayloadType:    payloadType,
		Payload:        payload,
	}
	return agentMessage
}
