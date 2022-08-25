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

// Package log is used to test event logger
package log

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
)

type EventLogTestSuite struct {
	suite.Suite
	EventLog *EventLog
}

// Setup
func (suite *EventLogTestSuite) SetupTest() {
	path, _ := os.Getwd()
	suite.EventLog = GetEventLog(path, "testEventLog")
}

// Execute the test suite
func TestEventLogTestSuite(t *testing.T) {
	suite.Run(t, new(EventLogTestSuite))
}

// Test case for checking the Load function
func (suite *EventLogTestSuite) TestWrite_LoadEvent() {
	logPath := filepath.Join(suite.EventLog.eventLogPath, suite.EventLog.nextFileName)

	defer func() {
		os.Remove(logPath)
		suite.EventLog.close()
	}()
	//Load Event - Creates file
	input := "Sample Event"
	output := "SchemaVersion=1\n" + AgentTelemetryMessage + " " + input
	suite.EventLog.loadEvent(AgentTelemetryMessage, "", input)
	suite.EventLog.loadEvent(AgentTelemetryMessage, "", input)
	suite.EventLog.loadEvent(AgentTelemetryMessage, "", input)
	suite.EventLog.loadEvent(AgentTelemetryMessage, "", input)
	//Check the content
	fileContentBytes, err := suite.EventLog.fileSystem.ReadFile(logPath)
	assert.Contains(suite.T(), string(fileContentBytes), output)
	assert.Nil(suite.T(), err)
	suite.WriteConfigCheck(logPath)
}

// Test case for rolling files
func (suite *EventLogTestSuite) WriteConfigCheck(currentLogFile string) {
	suite.EventLog.init()
	inputFiles := []string{
		suite.EventLog.eventLogName + suite.EventLog.fileDelimiter + "2020-03-01",
		suite.EventLog.eventLogName + suite.EventLog.fileDelimiter + "2020-03-02",
		suite.EventLog.eventLogName + suite.EventLog.fileDelimiter + "2020-03-03",
		suite.EventLog.eventLogName + suite.EventLog.fileDelimiter + "2020-03-04",
		suite.EventLog.eventLogName + suite.EventLog.fileDelimiter + "2020-03-05",
		suite.EventLog.eventLogName + suite.EventLog.fileDelimiter + "2020-03-06",
		suite.EventLog.eventLogName + suite.EventLog.fileDelimiter + "2020-03-07",
		suite.EventLog.eventLogName + suite.EventLog.fileDelimiter + "2020-03-08",
		suite.EventLog.eventLogName + suite.EventLog.fileDelimiter + "2020-03-09",
	}
	defer func() {
		for _, fileName := range inputFiles {
			os.Remove(filepath.Join(suite.EventLog.eventLogPath, fileName))
		}
		os.Remove(currentLogFile)
	}()

	for _, fileName := range inputFiles {
		suite.EventLog.fileSystem.AppendToFile(filepath.Join(suite.EventLog.eventLogPath, fileName), "Test content", 0600)
	}
	content := "test"
	suite.EventLog.loadEvent(AgentUpdateResultMessage, "2.0.0.2", content)
	suite.EventLog.rotateEventLog()
	currentLogCount := len(suite.EventLog.getFilesWithMatchDatePattern())
	assert.Equal(suite.T(), currentLogCount, suite.EventLog.noOfHistoryFiles)
}

// Test case for checking event counts in the file
func (suite *EventLogTestSuite) TestWrite_GetEventCount() {
	timeStamp := "15:04:05"
	inputFiles := []string{
		suite.EventLog.eventLogName + suite.EventLog.fileDelimiter + "2020-03-01",
		suite.EventLog.eventLogName + suite.EventLog.fileDelimiter + "2020-03-02",
	}
	defer func() {
		for _, fileName := range inputFiles {
			os.Remove(filepath.Join(suite.EventLog.eventLogPath, fileName))
		}
	}()
	for _, fileName := range inputFiles {
		suite.EventLog.fileSystem.AppendToFile(filepath.Join(suite.EventLog.eventLogPath, fileName), "SchemaVersion=1\nEventType1 TestEvent1 2.0.0.1 "+timeStamp+"\nEventType1 TestEvent1\nEventType1 TestEvent1 2.0.0.2 "+timeStamp+"\n", 0600)
	}
	eventCounts, err := GetEventCounter()
	assert.Equal(suite.T(), 4, len(eventCounts))
	assert.Nil(suite.T(), err)
	assert.Equal(suite.T(), 1, eventCounts[0].CountMap["TestEvent1"])
	assert.Equal(suite.T(), 1, eventCounts[1].CountMap["TestEvent1"])
}

// Test case for checking event counts in the file with "audit sent" file
func (suite *EventLogTestSuite) TestWrite_GetEventCountWithAuditSuccess() {
	timeStamp := "15:04:05"

	inputFiles := []string{
		suite.EventLog.eventLogName + suite.EventLog.fileDelimiter + "2020-03-01",
		suite.EventLog.eventLogName + suite.EventLog.fileDelimiter + "2020-03-02",
	}
	defer func() {
		for _, fileName := range inputFiles {
			os.Remove(filepath.Join(suite.EventLog.eventLogPath, fileName))
		}
	}()
	input := "SchemaVersion=1\nEventType1 TestEvent1 2.0.0.1 " + timeStamp + "\nEventType1 TestEvent1\nEventType1 TestEvent1 2.0.0.1 " + timeStamp + "\n"
	for _, fileName := range inputFiles {
		suite.EventLog.fileSystem.AppendToFile(filepath.Join(suite.EventLog.eventLogPath, fileName), input, 0600)
	}
	byteMarker := fmt.Sprintf("%0"+strconv.Itoa(BytePatternLen)+"d", len(input))
	suite.EventLog.fileSystem.AppendToFile(filepath.Join(suite.EventLog.eventLogPath, inputFiles[0]), AuditSentSuccessFooter+byteMarker, 0600)
	eventCounts, err := GetEventCounter()
	assert.Equal(suite.T(), 1, len(eventCounts))
	assert.Nil(suite.T(), err)
	assert.Equal(suite.T(), 2, eventCounts[0].CountMap["TestEvent1"])
}

// Test case for checking event counts in the file with incorrect formats
func (suite *EventLogTestSuite) TestWrite_GetEventCountWithIncorrectSchema() {
	inputFiles := []string{
		suite.EventLog.eventLogName + suite.EventLog.fileDelimiter + "2020-03-02",
	}
	defer func() {
		for _, fileName := range inputFiles {
			os.Remove(filepath.Join(suite.EventLog.eventLogPath, fileName))
		}
	}()
	fileInput1 := "SchemaVersion=sdsds" +
		"\nEventType1 TestEvent1 2.0.0.1 dssdsds ddsd" +
		"\nEventType1 TestEvent1" +
		"\nEventType1 TestEvent1 dssdsds sdsdssd\n"
	for _, fileName := range inputFiles {
		suite.EventLog.fileSystem.AppendToFile(filepath.Join(suite.EventLog.eventLogPath, fileName), fileInput1, 0600)
	}
	eventCounts, err := GetEventCounter()
	assert.Nil(suite.T(), err)
	assert.Equal(suite.T(), 1, len(eventCounts))
	val, err := strconv.Atoi(eventCounts[0].SchemaVersion)
	assert.Equal(suite.T(), 0, val)
	assert.NotNil(suite.T(), err)
	assert.Equal(suite.T(), 1, eventCounts[0].CountMap["TestEvent1"])
}

// Test case for checking update events
func (suite *EventLogTestSuite) TestWrite_GetEventCountWithUpdateResult() {
	timeStamp1 := "22:59:59"
	timeStamp2 := "23:00:00"
	//timeStamp3 := "23:05:00"
	inputFiles := []string{
		suite.EventLog.eventLogName + suite.EventLog.fileDelimiter + "2020-03-02",
	}
	defer func() {
		for _, fileName := range inputFiles {
			os.Remove(filepath.Join(suite.EventLog.eventLogPath, fileName))
		}
	}()
	file2Input := "SchemaVersion=1\n" +
		"EventType1 Test 2.3.2.0 " + timeStamp1 +
		"\nEventType1 Test 2.3.3.1 " + timeStamp1 +
		"\nagent_update_result UpdateError1-2.3.2.1 2.0.3.1 " + timeStamp1 +
		"\nagent_update_result UpdateError2-2.3.2.2 2.0.3.2 " + timeStamp1 +
		"\nagent_update_result UpdateError3-2.3.2.3 2.0.3.3 " + timeStamp2 +
		"\n"
	for _, fileName := range inputFiles {
		suite.EventLog.fileSystem.AppendToFile(filepath.Join(suite.EventLog.eventLogPath, fileName), file2Input, 0600)
	}
	eventCounts, err := GetEventCounter()
	assert.Equal(suite.T(), 5, len(eventCounts))
	assert.Nil(suite.T(), err)
	_, ok := eventCounts[0].CountMap["UpdateError3-2.3.2.3"]
	assert.Equal(suite.T(), true, ok)
	val, err := strconv.Atoi(eventCounts[0].SchemaVersion)
	assert.Equal(suite.T(), 1, val)
}

// Test case for checking event counts in the file with invalid update event lines
func (suite *EventLogTestSuite) TestWrite_CheckFooterBytesWithInvalidEventLine() {
	timeStamp1 := "22:59:59"
	timeStamp2 := "23:00:00"
	inputFiles := []string{
		suite.EventLog.eventLogName + suite.EventLog.fileDelimiter + "2020-03-02",
	}
	defer func() {
		for _, fileName := range inputFiles {
			os.Remove(filepath.Join(suite.EventLog.eventLogPath, fileName))
		}
	}()
	file2Input := "SchemaVersion=1\n" +
		"EventType1 Test 2.3.3.0 " + timeStamp1 +
		"\nEventType1 Test 2.3.3.1 " + timeStamp1 +
		"\nagent_update_result UpdateError1-2.3.2.1-2.0.3.1 " + timeStamp1 + // Invalid line
		"\nagent_update_result UpdateError2-2.3.2.2-2.0.3.2 " + timeStamp1 + // Invalid line
		"\nagent_update_result UpdateError3-2.3.2.3-2.0.3.3 " + timeStamp2 + // Invalid line
		"\n"
	for _, fileName := range inputFiles {
		suite.EventLog.fileSystem.AppendToFile(filepath.Join(suite.EventLog.eventLogPath, fileName), file2Input, 0600)
	}
	eventCounts, err := GetEventCounter()
	assert.Equal(suite.T(), 2, len(eventCounts)) // two valid versions
	assert.Nil(suite.T(), err)
	assert.Equal(suite.T(), len(file2Input), eventCounts[0].LastReadByte)
}

func (suite *EventLogTestSuite) TestWrite_GetTopFiveChunks() {
	inputFiles := []string{
		suite.EventLog.eventLogName + suite.EventLog.fileDelimiter + "2020-03-02",
	}
	defer func() {
		for _, fileName := range inputFiles {
			os.Remove(filepath.Join(suite.EventLog.eventLogPath, fileName))
		}
	}()
	file2Input := "SchemaVersion=1\n" +
		"EventType1 amazon-ssm-agent.start 2.0.0.0 05:29:13\n" +
		"EventType1 ssm-agent-worker.start 2.0.0.0 05:29:17\n" +
		"EventType1 amazon-ssm-agent.start 2.0.0.0 05:29:28\n" +
		"agent_update_result UpdateFailed_ErrorTargetPkgDownload-9.5.0.0 2.0.0.0 05:32:05\n" +
		"EventType1 amazon-ssm-agent.start 2.0.0.1 05:33:29\n" +
		"EventType1 ssm-agent-worker.start 2.0.0.1 05:33:33\n" +
		"EventType1 amazon-ssm-agent.start 2.0.0.0 05:36:35\n" +
		"EventType1 ssm-agent-worker.start 2.0.0.0 05:36:39\n" +
		"agent_update_result UpdateSucceeded-8.0.0.0 deeee 05:36:41\n" + // Invalid version
		"EventType1 amazon-ssm-agent.start 2.0.0.1 05:40:03\n" +
		"EventType1 ssm-agent-worker.start 2.0.0.1 05:40:03\n"

	for _, fileName := range inputFiles {
		suite.EventLog.fileSystem.AppendToFile(filepath.Join(suite.EventLog.eventLogPath, fileName), file2Input, 0600)
	}
	eventCounts, err := GetEventCounter()
	assert.Equal(suite.T(), 5, len(eventCounts))
	assert.Nil(suite.T(), err)
	assert.Equal(suite.T(), "2.0.0.1", eventCounts[0].AgentVersion)
	assert.Equal(suite.T(), "EventType1", eventCounts[0].EventChunkType)
	assert.Equal(suite.T(), 2, len(eventCounts[0].CountMap))
	assert.Equal(suite.T(), 2, len(eventCounts[1].CountMap))
	assert.Equal(suite.T(), "2.0.0.1", eventCounts[2].AgentVersion)
	assert.Equal(suite.T(), "agent_update_result", eventCounts[3].EventChunkType)
	assert.Equal(suite.T(), "2.0.0.0", eventCounts[4].AgentVersion)
}
