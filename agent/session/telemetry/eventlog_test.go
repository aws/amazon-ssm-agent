// Copyright 2020 Amazon.com, Inc. or its affiliates. All Rights Reserved.
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

// Package telemetry is used to schedule and send the audit logs to MGS
package telemetry

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/aws/amazon-ssm-agent/agent/context"
	logger "github.com/aws/amazon-ssm-agent/agent/log"
	commMock "github.com/aws/amazon-ssm-agent/agent/session/communicator/mocks"
	cloudWatchMock "github.com/aws/amazon-ssm-agent/agent/session/telemetry/metrics/mocks"
	"github.com/aws/amazon-ssm-agent/core/workerprovider/longrunningprovider/datastore/filesystem"
	"github.com/aws/aws-sdk-go/service/cloudwatch"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
)

type TelemetrySchedulerTestSuite struct {
	suite.Suite
	EventLog           *logger.EventLog
	TelemetryScheduler *AuditLogTelemetry
	FileSystem         filesystem.IFileSystem
}

// Execute the test suite
func TestTelemetrySchedulerTestSuite(t *testing.T) {
	suite.Run(t, new(TelemetrySchedulerTestSuite))
}

// Setup
func (suite *TelemetrySchedulerTestSuite) SetupTest() {
	contextMock := context.NewMockDefault()
	mockWsChannel := &commMock.IWebSocketChannel{}
	mockWsChannel.On("SendMessage", mock.Anything, mock.Anything, mock.Anything).Return(nil)
	path, _ := os.Getwd()
	suite.EventLog = logger.GetEventLog(path, "testEventLog")

	cwMock := &cloudWatchMock.ICloudWatchService{}
	cwMock.On("GenerateUpdateMetrics", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(&cloudwatch.MetricDatum{})
	cwMock.On("GenerateBasicTelemetryMetrics", mock.Anything, mock.Anything, mock.Anything).Return(&cloudwatch.MetricDatum{})
	cwMock.On("PutMetrics", mock.Anything).Return(nil)
	cwMock.On("IsCloudWatchEnabled").Return(true)

	suite.TelemetryScheduler = GetAuditLogTelemetryInstance(contextMock, mockWsChannel)
	suite.TelemetryScheduler.eventLogDelayFactor = 1
	suite.TelemetryScheduler.eventLogDelayBase = 0
	suite.TelemetryScheduler.isMGSTelemetryTransportEnable = true
	suite.TelemetryScheduler.mgsDelay = 1
	suite.TelemetryScheduler.cloudWatchService = cwMock

	suite.FileSystem = suite.EventLog.GetFileSystem()
}

// Test case for checking event counts in the file
func (suite *TelemetrySchedulerTestSuite) TestWrite_SendEventToMGS() {
	timeStamp := "15:04:05"
	nextTimeStamp := "23:00:00"
	nextFilePath := suite.EventLog.GetAuditFileName() + "-2020-03-01"
	filePath := filepath.Join(suite.EventLog.GetAuditFilePath(), nextFilePath)
	defer os.Remove(filePath)
	fileInput1 := "SchemaVersion=1\nEventType1 Test 2.0.0.0 " + timeStamp + "\nEventType1 Test 2.0.0.0 " + nextTimeStamp + "\n"
	suite.FileSystem.AppendToFile(filePath, fileInput1, 0600)

	suite.TelemetryScheduler.SendAuditMessage()
	time.Sleep(1500 * time.Millisecond)

	byteMarker := fmt.Sprintf("%0"+strconv.Itoa(logger.BytePatternLen)+"d", len(fileInput1))
	output := fileInput1 + logger.AuditSentSuccessFooter + byteMarker
	fileContentBytes, err := suite.FileSystem.ReadFile(filePath)
	assert.Equal(suite.T(), string(fileContentBytes), output)
	assert.Nil(suite.T(), err)
}

// Test case for checking the send event to MGS with "audit sent" file
func (suite *TelemetrySchedulerTestSuite) TestWrite_SendEventToMGSWithAuditSuccessFile() {
	timeStamp := "22:59:59"
	nextTimeStamp := "23:00:00"

	file1 := filepath.Join(suite.EventLog.GetAuditFilePath(), suite.EventLog.GetAuditFileName()+"-2020-03-01")
	file2 := filepath.Join(suite.EventLog.GetAuditFilePath(), suite.EventLog.GetAuditFileName()+"-2020-03-02")
	file3 := filepath.Join(suite.EventLog.GetAuditFilePath(), suite.EventLog.GetTodayAuditFileName())
	defer func() {
		os.Remove(file1)
		os.Remove(file2)
		os.Remove(file3)
	}()
	fileInput1 := "SchemaVersion=1\nagent_telemetry amazon-ssm-agent.start 2.0.0.0 " + timeStamp + "\nagent_telemetry ssm-agent-worker.start 2.0.0.0 " + timeStamp + "\n"
	fileInput2 := "SchemaVersion=1\nagent_telemetry amazon-ssm-agent.start 2.0.0.0 " + timeStamp + "\nagent_telemetry ssm-agent-worker.start 2.0.0.1 " + nextTimeStamp + "\n"
	fileInput3 := "SchemaVersion=1\nagent_telemetry amazon-ssm-agent.start 2.0.0.0 " + timeStamp

	byteMarker := fmt.Sprintf("%0"+strconv.Itoa(logger.BytePatternLen)+"d", len(fileInput1))
	suite.FileSystem.AppendToFile(file1, fileInput1+logger.AuditSentSuccessFooter+byteMarker, 0600)
	suite.FileSystem.AppendToFile(file2, fileInput2, 0600)
	suite.FileSystem.AppendToFile(file3, fileInput3, 0600)

	suite.TelemetryScheduler.SendAuditMessage()
	time.Sleep(2 * time.Second)

	firstFileOutput := fileInput1 + logger.AuditSentSuccessFooter + byteMarker
	byteMarker = fmt.Sprintf("%0"+strconv.Itoa(logger.BytePatternLen)+"d", len(fileInput2))
	SecondFileOutput := fileInput2 + logger.AuditSentSuccessFooter + byteMarker
	thirdFileOutput := fileInput3

	fileContentBytes, err := suite.FileSystem.ReadFile(file1)
	assert.Equal(suite.T(), string(fileContentBytes), firstFileOutput, "2020-03-01 audit file is not matching")
	assert.Nil(suite.T(), err)

	fileContentBytes, err = suite.FileSystem.ReadFile(file2)
	assert.Equal(suite.T(), string(fileContentBytes), SecondFileOutput, "2020-03-02 audit file is not matching")
	assert.Nil(suite.T(), err)

	fileContentBytes, err = suite.FileSystem.ReadFile(file3)
	assert.Contains(suite.T(), string(fileContentBytes), thirdFileOutput)
	assert.Nil(suite.T(), err)
}

// for update send event
func (suite *TelemetrySchedulerTestSuite) TestWrite_SendUpdateEventToMGS() {
	timeStamp := "22:59:59"
	nextTimeStamp := "23:00:00"

	file1 := filepath.Join(suite.EventLog.GetAuditFilePath(), suite.EventLog.GetAuditFileName()+"-2020-03-01")
	file2 := filepath.Join(suite.EventLog.GetAuditFilePath(), suite.EventLog.GetAuditFileName()+"-2020-03-02")
	defer func() {
		os.Remove(file1)
		os.Remove(file2)
	}()

	file1Input := "SchemaVersion=1\nagent_telemetry amazon-ssm-agent.start 2.0.0.0 " + timeStamp + "\nagent_telemetry ssm-agent-worker.start 2.0.0.0 " + timeStamp + "\nagent_update_result UpdateError_2.3.2.2 2.0.3.2 " + nextTimeStamp + "\n"
	file2Input := "SchemaVersion=1\n" +
		"agent_telemetry amazon-ssm-agent.start 2.0.0.0 " + timeStamp +
		"\nagent_telemetry ssm-agent-worker.start 2.0.0.1 " + timeStamp +
		"\nagent_update_result UpdateError1-2.3.2.2 2.0.3.2 " + timeStamp +
		"\nagent_update_result UpdateError2-2.3.2.2 2.0.3.2 " + timeStamp +
		"\nagent_update_result UpdateError3-2.3.2.2 2.0.3.2 " + nextTimeStamp +
		"\n"

	byteMarker := fmt.Sprintf("%0"+strconv.Itoa(logger.BytePatternLen)+"d", len(file1Input))
	suite.FileSystem.AppendToFile(file1, file1Input+logger.AuditSentSuccessFooter+byteMarker, 0600)
	suite.FileSystem.AppendToFile(file2, file2Input, 0600)

	suite.TelemetryScheduler.SendAuditMessage()
	time.Sleep(1500 * time.Millisecond)

	byteMarker = fmt.Sprintf("%0"+strconv.Itoa(logger.BytePatternLen)+"d", len(file1Input))
	firstFileOutput := file1Input + logger.AuditSentSuccessFooter + byteMarker

	byteMarker = fmt.Sprintf("%0"+strconv.Itoa(logger.BytePatternLen)+"d", len(file2Input))
	SecondFileOutput := file2Input + logger.AuditSentSuccessFooter + byteMarker

	fileContentBytes, err := suite.FileSystem.ReadFile(file1)
	assert.Equal(suite.T(), string(fileContentBytes), firstFileOutput, "2020-03-01 audit file is not matching")
	assert.Nil(suite.T(), err)

	fileContentBytes, err = suite.FileSystem.ReadFile(file2)
	assert.Equal(suite.T(), string(fileContentBytes), SecondFileOutput, "2020-03-02 audit file is not matching")
	assert.Nil(suite.T(), err)
}

// for event log with incorrect event for the message type
func (suite *TelemetrySchedulerTestSuite) TestWrite_SendUpdateEventWithInvalidMessageTypes() {
	timeStamp := "22:59:59"
	nextTimeStamp := "23:00:00"

	file1 := filepath.Join(suite.EventLog.GetAuditFilePath(), suite.EventLog.GetAuditFileName()+"-2020-03-02")
	defer func() {
		os.Remove(file1)
	}()

	file1InValidInput := "SchemaVersion=1\n" +
		"agent_telemetry amazon-ssm-agent.start 2.0.0.0 " + timeStamp + "\n" +
		"agent_telemetry ssm-agent-worker.start 2.0.0.1 " + timeStamp + "\n" +
		"invalid_event_type dfsfsdfsdsdsdsdffdsfdds 2.0.3.2 " + timeStamp +
		"\nagent_update_result UpdateError2-2.3.2.2 2.0.3.2 " + timeStamp +
		"\nagent_update_result UpdateError3-2.3.2.2 2.0.3.2 " + nextTimeStamp +
		"\n"

	suite.FileSystem.AppendToFile(file1, file1InValidInput, 0600)

	suite.TelemetryScheduler.SendAuditMessage()
	time.Sleep(1500 * time.Millisecond)

	byteMarker := fmt.Sprintf("%0"+strconv.Itoa(logger.BytePatternLen)+"d", len(file1InValidInput))
	firstFileOutput := file1InValidInput + logger.AuditSentSuccessFooter + byteMarker
	fileContentBytes, err := suite.FileSystem.ReadFile(file1)

	assert.Equal(suite.T(), string(fileContentBytes), firstFileOutput, "2020-03-02 audit file is not matching")
	assert.Nil(suite.T(), err)
}

// for event log with incorrect event for the message type
func (suite *TelemetrySchedulerTestSuite) TestWrite_SendEventCountCheck() {

	file1 := filepath.Join(suite.EventLog.GetAuditFilePath(), suite.EventLog.GetAuditFileName()+"-2020-03-02")
	defer func() {
		os.Remove(file1)
	}()

	file1ValidInput := "SchemaVersion=1\n" +
		"\nagent_telemetry amazon-ssm-agent.start 9.5.0.0 04:38:04" +
		"\nagent_telemetry ssm-agent-worker.start 9.5.0.0 04:38:08" +
		"\nagent_update_result UpdateSucceeded-8.0.0.0 9.5.0.0 04:38:10" +
		"\nagent_telemetry ssm-agent-worker.start 9.5.0.0 04:38:14" +
		"\nagent_update_result UpdateSucceeded-8.0.0.0 9.5.0.0 04:38:15" +
		"\nagent_telemetry amazon-ssm-agent.start 9.5.0.0 05:23:10" +
		"\nagent_telemetry ssm-agent-worker.start 9.5.0.0 05:23:14" + "\n"
	lenVal := len(file1ValidInput)
	file1ValidInput += "agent_update_result UpdateSucceeded-8.0.0.0 9.5.0.0 05:23:16" + "\n"

	suite.FileSystem.AppendToFile(file1, file1ValidInput, 0600)

	suite.TelemetryScheduler.SendAuditMessage()
	time.Sleep(1500 * time.Millisecond)

	byteMarker := fmt.Sprintf("%0"+strconv.Itoa(logger.BytePatternLen)+"d", lenVal)
	firstFileOutput := file1ValidInput + logger.AuditSentSuccessFooter + byteMarker

	fileContentBytes, err := suite.FileSystem.ReadFile(file1)

	assert.Equal(suite.T(), string(fileContentBytes), firstFileOutput, "2020-03-02 audit file is not matching")
	assert.Nil(suite.T(), err)
}
