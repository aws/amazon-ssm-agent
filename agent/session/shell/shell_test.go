// Copyright 2022 Amazon.com, Inc. or its affiliates. All Rights Reserved.
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

// Package shell implements session shell plugin.
package shell

import (
	"os"
	"strings"
	"testing"
	"time"

	cloudwatchlogspublisher_mock "github.com/aws/amazon-ssm-agent/agent/agentlogstocloudwatch/cloudwatchlogspublisher/mock"
	"github.com/aws/amazon-ssm-agent/agent/appconfig"
	"github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/aws/amazon-ssm-agent/agent/contracts"
	iohandlermocks "github.com/aws/amazon-ssm-agent/agent/framework/processor/executer/iohandler/mock"
	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/amazon-ssm-agent/agent/s3util"
	mgsContracts "github.com/aws/amazon-ssm-agent/agent/session/contracts"
	dataChannelMock "github.com/aws/amazon-ssm-agent/agent/session/datachannel/mocks"
	execcmdMock "github.com/aws/amazon-ssm-agent/agent/session/shell/execcmd/mocks"
	"github.com/aws/amazon-ssm-agent/agent/task"
	"github.com/aws/aws-sdk-go/service/cloudwatchlogs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
)

var (
	payload            = []byte("testPayload")
	messageId          = "dd01e56b-ff48-483e-a508-b5f073f31b16"
	schemaVersion      = uint32(1)
	createdDate        = uint64(1503434274948)
	mockLog            = log.NewMockLog()
	sessionId          = "sessionId"
	sessionOwner       = "sessionOwner"
	testCwLogGroupName = "testCW"
	testCwlLogGroup    = cloudwatchlogs.LogGroup{
		LogGroupName: &testCwLogGroupName,
	}
)

type ShellTestSuite struct {
	suite.Suite
	mockContext     *context.Mock
	mockLog         log.T
	mockCancelFlag  *task.MockCancelFlag
	mockDataChannel *dataChannelMock.IDataChannel
	mockIohandler   *iohandlermocks.MockIOHandler
	mockCWL         *cloudwatchlogspublisher_mock.CloudWatchLogsServiceMock
	mockS3          *s3util.MockS3Uploader
	stdin           *os.File
	stdout          *os.File
	mockCmd         *execcmdMock.IExecCmd
	plugin          *ShellPlugin
}

func (suite *ShellTestSuite) SetupTest() {
	mockContext := context.NewMockDefault()
	mockCancelFlag := &task.MockCancelFlag{}
	mockDataChannel := &dataChannelMock.IDataChannel{}
	mockCWL := new(cloudwatchlogspublisher_mock.CloudWatchLogsServiceMock)
	mockS3 := new(s3util.MockS3Uploader)
	mockIohandler := new(iohandlermocks.MockIOHandler)
	mockCmd := &execcmdMock.IExecCmd{}

	suite.mockContext = mockContext
	suite.mockCancelFlag = mockCancelFlag
	suite.mockLog = mockLog
	suite.mockDataChannel = mockDataChannel
	suite.mockIohandler = mockIohandler
	suite.mockCWL = mockCWL
	suite.mockS3 = mockS3
	suite.mockCmd = mockCmd
	stdout, stdin, _ := os.Pipe()
	suite.stdin = stdin
	suite.stdout = stdout
	suite.plugin = &ShellPlugin{
		stdin:       stdin,
		stdout:      stdout,
		dataChannel: mockDataChannel,
		context:     mockContext,
		logger: logger{
			cwl:                         mockCWL,
			s3Util:                      mockS3,
			ptyTerminated:               make(chan bool),
			cloudWatchStreamingFinished: make(chan bool),
		},
	}
}

func (suite *ShellTestSuite) TearDownTest() {
	suite.stdin.Close()
	suite.stdout.Close()
}

// Execute the test suite
func TestShellTestSuite(t *testing.T) {
	suite.Run(t, new(ShellTestSuite))
}

// Testing validPrefix
func (suite *ShellTestSuite) TestValidPrefix() {
	plugin := &ShellPlugin{}
	suite.True(plugin.validPrefix("STD_OUT:\r\n"))
	suite.True(plugin.validPrefix("stderr:"))
	suite.True(plugin.validPrefix("STD_OUT\n123"))
	suite.True(plugin.validPrefix("std-OUT:"))

	suite.False(plugin.validPrefix("std@OUT:"))
	suite.False(plugin.validPrefix("stdOUT!\t"))
	suite.False(plugin.validPrefix("(stdOUT1)"))
	suite.False(plugin.validPrefix("abcabcabcabcabcabcabcabcabcabcabcabcabc:"))
}

// Testing setSeparateOutputStreamProperties for NonInteractiveCommands plugin
func (suite *ShellTestSuite) TestSetSeparateOutputStreamProperties() {
	plugin := &ShellPlugin{
		context:     suite.mockContext,
		name:        appconfig.PluginNameNonInteractiveCommands,
		dataChannel: suite.mockDataChannel,
		execCmd:     suite.mockCmd,
	}
	shellConfig := mgsContracts.ShellConfig{
		"ls", false, "true", "STD_OUT:\n", "STD_ERR:\n"}
	shellProperties := mgsContracts.ShellProperties{shellConfig, shellConfig, shellConfig}

	plugin.setSeparateOutputStreamProperties(shellProperties)
	assert.True(suite.T(), plugin.separateOutput)
	assert.Equal(suite.T(), plugin.stdoutPrefix, "STD_OUT:\n")
	assert.Equal(suite.T(), plugin.stderrPrefix, "STD_ERR:\n")
}

// Testing setSeparateOutputStreamProperties with invalid separateOutPutStream
func (suite *ShellTestSuite) TestSetSeparateOutputStreamPropertiesWithInvalidSeparateOutPutStream() {
	plugin := &ShellPlugin{
		context:     suite.mockContext,
		name:        appconfig.PluginNameNonInteractiveCommands,
		dataChannel: suite.mockDataChannel,
		execCmd:     suite.mockCmd,
	}
	shellConfig := mgsContracts.ShellConfig{
		"ls", false, "error", "STD_OUT:\n", "STD$ERR:\n"}
	shellProperties := mgsContracts.ShellProperties{shellConfig, shellConfig, shellConfig}

	err := plugin.setSeparateOutputStreamProperties(shellProperties)

	suite.True(strings.Contains(err.Error(), "fail to get separateOutPutStream property"))
}

// Testing setSeparateOutputStreamProperties with invalid stdout prefix
func (suite *ShellTestSuite) TestSetSeparateOutputStreamPropertiesWithInvalidStdoutPrefix() {
	plugin := &ShellPlugin{
		context:     suite.mockContext,
		name:        appconfig.PluginNameNonInteractiveCommands,
		dataChannel: suite.mockDataChannel,
		execCmd:     suite.mockCmd,
	}
	shellConfig := mgsContracts.ShellConfig{
		"ls", false, "true", "STD@OUT:\n", "STD_ERR:\n"}
	shellProperties := mgsContracts.ShellProperties{shellConfig, shellConfig, shellConfig}

	err := plugin.setSeparateOutputStreamProperties(shellProperties)

	suite.True(strings.Contains(err.Error(), "invalid stdoutSeparatorPrefix"))
	suite.True(plugin.separateOutput)
}

// Testing setSeparateOutputStreamProperties with invalid stderr prefix
func (suite *ShellTestSuite) TestSetSeparateOutputStreamPropertiesWithInvalidStderrPrefix() {
	plugin := &ShellPlugin{
		context:     suite.mockContext,
		name:        appconfig.PluginNameNonInteractiveCommands,
		dataChannel: suite.mockDataChannel,
		execCmd:     suite.mockCmd,
	}
	shellConfig := mgsContracts.ShellConfig{
		"ls", false, "true", "STD_OUT:\n", "STD$ERR:\n"}
	shellProperties := mgsContracts.ShellProperties{shellConfig, shellConfig, shellConfig}

	err := plugin.setSeparateOutputStreamProperties(shellProperties)

	suite.True(strings.Contains(err.Error(), "invalid stderrSeparatorPrefix"))
	suite.True(plugin.separateOutput)
}

// Testing sendExitCode for NonInteractiveCommands plugin
func (suite *ShellTestSuite) TestSendExitCode() {
	plugin := &ShellPlugin{
		context:     suite.mockContext,
		name:        appconfig.PluginNameNonInteractiveCommands,
		dataChannel: suite.mockDataChannel,
		execCmd:     suite.mockCmd,
	}
	suite.plugin.logger = logger{ipcFilePath: "test.log"}
	ipcFile, _ := os.Create(suite.plugin.logger.ipcFilePath)

	// Deleting file
	defer func() {
		ipcFile.Close()
		os.Remove(suite.plugin.logger.ipcFilePath)
	}()
	exitCode := 0

	suite.mockDataChannel.On("IsActive").Return(true)
	suite.mockDataChannel.On("SendStreamDataMessage", mock.Anything, mock.Anything, mock.Anything).Return(nil)

	plugin.sendExitCode(suite.mockLog, ipcFile, exitCode)
	suite.mockDataChannel.AssertExpectations(suite.T())
}

// Testing sendExitCode when data channel inactive
func (suite *ShellTestSuite) TestSendExitCodeWhenDataChannelInactive() {
	suite.mockDataChannel.On("IsActive").Return(false)
	plugin := &ShellPlugin{
		name:        appconfig.PluginNameNonInteractiveCommands,
		dataChannel: suite.mockDataChannel,
	}
	suite.plugin.logger = logger{ipcFilePath: "ipcfile_for_shell_unit_test.log"}
	ipcFile, _ := os.Create(suite.plugin.logger.ipcFilePath)

	// Deleting file
	defer func() {
		ipcFile.Close()
		os.Remove(suite.plugin.logger.ipcFilePath)
	}()
	exitCode := 0
	err := plugin.sendExitCode(suite.mockLog, ipcFile, exitCode)
	suite.True(strings.Contains(err.Error(), "failed to send exit code as data channel closed"))
	suite.mockDataChannel.AssertExpectations(suite.T())
}

// Testing Execute for NonInteractiveCommand with separate output stream enabled
func (suite *ShellTestSuite) TestExecuteForNonInteractiveCommandsWithSeparateOutputStream() {
	suite.mockCancelFlag.On("Canceled").Return(false)
	suite.mockCancelFlag.On("ShutDown").Return(false)
	suite.mockCancelFlag.On("Wait").Return(task.Completed)
	suite.mockIohandler.On("SetStatus", mock.Anything).Return()
	suite.mockIohandler.On("SetOutput", mock.Anything).Return()
	suite.mockIohandler.On("SetExitCode", mock.Anything).Return()
	suite.mockDataChannel.On("IsActive").Return(true)
	suite.mockDataChannel.On("PrepareToCloseChannel", mock.Anything).Return(nil).Times(1)
	suite.mockDataChannel.On("SendAgentSessionStateMessage", mock.Anything, mgsContracts.Terminating).
		Return(nil).Times(1)
	suite.mockDataChannel.On("SendStreamDataMessage", mock.Anything, mock.Anything, mock.Anything).Return(nil)
	suite.mockCmd.On("Start").Return(nil)
	suite.mockCmd.On("Wait").Return(nil)
	suite.mockCmd.On("Pid").Return(234)

	stdoutPipe, stdoutPipeinput, _ := os.Pipe()
	stdoutPipeinput.Write(payload)
	stderrPipe, stderrPipeinput, _ := os.Pipe()
	stderrPipeinput.Write(payload)

	shellConfig := mgsContracts.ShellConfig{
		"ls", false, "true", "STD_OUT:\n", "STD_ERR:\n"}
	shellProperties := mgsContracts.ShellProperties{shellConfig, shellConfig, shellConfig}

	getCommandExecutor = func(log log.T, shellProps mgsContracts.ShellProperties, isSessionLogger bool, config contracts.Configuration, plugin *ShellPlugin) (err error) {
		plugin.execCmd = suite.mockCmd
		plugin.stdoutPipe = stdoutPipe
		plugin.stderrPipe = stderrPipe
		return nil
	}

	plugin := &ShellPlugin{
		name:        appconfig.PluginNameNonInteractiveCommands,
		context:     suite.mockContext,
		dataChannel: suite.mockDataChannel,
		execCmd:     suite.mockCmd,
	}

	go func() {
		time.Sleep(1000 * time.Millisecond)
		stdoutPipeinput.Close()
		stderrPipeinput.Close()
		time.Sleep(500 * time.Millisecond)
		stdoutPipe.Close()
		stderrPipe.Close()
	}()

	plugin.Execute(
		contracts.Configuration{
			CloudWatchLogGroup:         testCwLogGroupName,
			CloudWatchStreamingEnabled: false,
			SessionId:                  sessionId,
			SessionOwner:               sessionOwner,
		},
		suite.mockCancelFlag,
		suite.mockIohandler,
		suite.mockDataChannel,
		shellProperties)

	suite.mockCancelFlag.AssertExpectations(suite.T())
	suite.mockIohandler.AssertExpectations(suite.T())
	suite.mockDataChannel.AssertExpectations(suite.T())
	suite.mockCmd.AssertExpectations(suite.T())
}

// Testing SetupRoutineToWriteCmdPipelineOutput
func (suite *ShellTestSuite) TestSetupRoutineToWriteCmdPipelineOutput() {
	suite.mockDataChannel.On("IsActive").Return(true)
	var payloadType mgsContracts.PayloadType
	var payloadContent []byte
	suite.mockDataChannel.On("SendStreamDataMessage", mock.Anything, mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
		payloadType = args.Get(1).(mgsContracts.PayloadType)
		payloadContent = args.Get(2).([]byte)
	}).Return(nil)

	stdoutPipe, stdoutPipeinput, _ := os.Pipe()
	stdoutPipeinput.Write(payload)
	stderrPipe, stderrPipeinput, _ := os.Pipe()
	stderrPipeinput.Write(payload)

	plugin := &ShellPlugin{
		name:           appconfig.PluginNameNonInteractiveCommands,
		context:        suite.mockContext,
		dataChannel:    suite.mockDataChannel,
		execCmd:        suite.mockCmd,
		stdoutPipe:     stdoutPipe,
		stderrPipe:     stderrPipe,
		separateOutput: true,
		stdoutPrefix:   "STD_OUT:",
		stderrPrefix:   "STD_ERR:",
	}

	go func() {
		time.Sleep(1000 * time.Millisecond)
		stdoutPipeinput.Close()
		stderrPipeinput.Close()
		time.Sleep(500 * time.Millisecond)
		stdoutPipe.Close()
		stderrPipe.Close()
	}()

	ipcFileName := "shell_util_test_file"
	ipcFile, _ := os.Create(ipcFileName)

	// Deleting file
	defer func() {
		ipcFile.Close()
		os.Remove(ipcFileName)
	}()

	result := plugin.setupRoutineToWriteCmdPipelineOutput(
		suite.mockLog,
		ipcFile,
		false)

	suite.Equal(0, <-result)
	suite.Equal(mgsContracts.Output, payloadType)
	suite.Equal("STD_OUT:testPayload", string(payloadContent[:]))
	suite.mockDataChannel.AssertExpectations(suite.T())
	suite.mockCmd.AssertExpectations(suite.T())
}

// Testing SetupRoutineToWriteCmdPipelineOutput with read error
func (suite *ShellTestSuite) TestSetupRoutineToWriteCmdPipelineOutputWithReadPipeError() {
	suite.mockDataChannel.On("IsActive").Return(true)
	var payloadType mgsContracts.PayloadType
	var payloadContent []byte
	suite.mockDataChannel.On("SendStreamDataMessage", mock.Anything, mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
		payloadType = args.Get(1).(mgsContracts.PayloadType)
		payloadContent = args.Get(2).([]byte)
	}).Return(nil)

	stdoutPipe, stdoutPipeinput, _ := os.Pipe()
	stdoutPipeinput.Write(payload)
	stderrPipe, stderrPipeinput, _ := os.Pipe()
	stderrPipeinput.Write(payload)

	plugin := &ShellPlugin{
		name:           appconfig.PluginNameNonInteractiveCommands,
		context:        suite.mockContext,
		dataChannel:    suite.mockDataChannel,
		execCmd:        suite.mockCmd,
		stdoutPipe:     stdoutPipe,
		stderrPipe:     stderrPipe,
		separateOutput: true,
		stdoutPrefix:   "STD_OUT:",
		stderrPrefix:   "STD_ERR:",
	}

	// Close the output side firstly to trigger it
	go func() {
		time.Sleep(1000 * time.Millisecond)
		stdoutPipe.Close()
		stderrPipe.Close()
		time.Sleep(500 * time.Millisecond)
		stdoutPipeinput.Close()
		stderrPipeinput.Close()
	}()

	ipcFileName := "shell_util_test_file"
	ipcFile, _ := os.Create(ipcFileName)

	// Deleting file
	defer func() {
		ipcFile.Close()
		os.Remove(ipcFileName)
	}()

	result := plugin.setupRoutineToWriteCmdPipelineOutput(
		suite.mockLog,
		ipcFile,
		false)

	suite.Equal(1, <-result)
	suite.Equal(mgsContracts.Output, payloadType)
	suite.Equal("STD_OUT:testPayload", string(payloadContent[:]))
	suite.mockDataChannel.AssertExpectations(suite.T())
	suite.mockCmd.AssertExpectations(suite.T())
}

// Testing SetupRoutineToWriteCmdPipelineOutput for stderr type payload
func (suite *ShellTestSuite) TestSetupRoutineToWriteCmdPipelineOutputForStdErr() {
	suite.mockDataChannel.On("IsActive").Return(true)
	var payloadType mgsContracts.PayloadType
	var payloadContent []byte
	suite.mockDataChannel.On("SendStreamDataMessage", mock.Anything, mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
		payloadType = args.Get(1).(mgsContracts.PayloadType)
		payloadContent = args.Get(2).([]byte)
	}).Return(nil)

	stdoutPipe, stdoutPipeinput, _ := os.Pipe()
	stdoutPipeinput.Write(payload)
	stderrPipe, stderrPipeinput, _ := os.Pipe()
	stderrPipeinput.Write([]byte("testPayload"))

	plugin := &ShellPlugin{
		name:           appconfig.PluginNameNonInteractiveCommands,
		context:        suite.mockContext,
		dataChannel:    suite.mockDataChannel,
		execCmd:        suite.mockCmd,
		stdoutPipe:     stdoutPipe,
		stderrPipe:     stderrPipe,
		separateOutput: true,
		stdoutPrefix:   "STD_OUT:",
		stderrPrefix:   "STD_ERR:",
	}

	go func() {
		time.Sleep(1000 * time.Millisecond)
		stdoutPipeinput.Close()
		stderrPipeinput.Close()
		time.Sleep(500 * time.Millisecond)
		stdoutPipe.Close()
		stderrPipe.Close()
	}()

	ipcFileName := "shell_stderr_util_test_file"
	ipcFile, _ := os.Create(ipcFileName)

	// Deleting file
	defer func() {
		ipcFile.Close()
		os.Remove(ipcFileName)
	}()

	result := plugin.setupRoutineToWriteCmdPipelineOutput(
		suite.mockLog,
		ipcFile,
		true)

	suite.Equal(0, <-result)
	suite.Equal(mgsContracts.StdErr, payloadType)
	suite.Equal("STD_ERR:testPayload", string(payloadContent[:]))
	suite.mockDataChannel.AssertExpectations(suite.T())
	suite.mockCmd.AssertExpectations(suite.T())
}

// Testing SetupRoutineToWriteCmdPipelineOutput when data channel not active
func (suite *ShellTestSuite) TestSetupRoutineToWriteCmdPipelineOutputWhenDataChannelInactive() {
	suite.mockDataChannel.On("IsActive").Return(false)

	stdoutPipe, stdoutPipeinput, _ := os.Pipe()
	stderrPipe, stderrPipeinput, _ := os.Pipe()

	plugin := &ShellPlugin{
		name:           appconfig.PluginNameNonInteractiveCommands,
		context:        suite.mockContext,
		dataChannel:    suite.mockDataChannel,
		execCmd:        suite.mockCmd,
		stdoutPipe:     stdoutPipe,
		stderrPipe:     stderrPipe,
		separateOutput: true,
		stdoutPrefix:   "STD_OUT:",
		stderrPrefix:   "STD_ERR:",
	}

	go func() {
		time.Sleep(1000 * time.Millisecond)
		stdoutPipe.Close()
		stdoutPipeinput.Close()
		stderrPipe.Close()
		stderrPipeinput.Close()
	}()

	ipcFileName := "shell_util_test_file"
	ipcFile, _ := os.Create(ipcFileName)

	// Deleting file
	defer func() {
		ipcFile.Close()
		os.Remove(ipcFileName)
	}()

	result := plugin.setupRoutineToWriteCmdPipelineOutput(
		suite.mockLog,
		ipcFile,
		true)

	suite.Equal(<-result, 1)
	suite.mockDataChannel.AssertExpectations(suite.T())
}
