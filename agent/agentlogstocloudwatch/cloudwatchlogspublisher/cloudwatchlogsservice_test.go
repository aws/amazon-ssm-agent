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

// cloudwatchlogspublisher is responsible for pulling logs from the log queue and publishing them to cloudwatch

package cloudwatchlogspublisher

import (
	"os"
	"strings"
	"testing"

	"github.com/aws/amazon-ssm-agent/agent/agentlogstocloudwatch/cloudwatchlogspublisher/mock"
	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/amazon-ssm-agent/agent/sdkutil"
	"github.com/aws/aws-sdk-go/service/cloudwatchlogs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

var logMock = log.NewMockLog()
var cwLogsClientMock = cloudwatchlogspublisher_mock.NewClientMockDefault()
var input = []string{
	"Test input text.",
	"\b5Ὂg̀9! ℃ᾭG",
	"Lorem ipsum dolor sit amet, consectetur adipiscing elit. In fermentum cursus mi, sed placerat tellus condimentum non.",
	"Pellentesque vel volutpat velit. Sed eget varius nibh. Sed quis nisl enim. Nulla faucibus nisl a massa fermentum porttitor. ",
	"Integer at massa blandit, congue ligula ut, vulputate lacus. Morbi tempor tellus a tempus sodales. Nam at placerat odio, ",
	"ut placerat purus. Donec imperdiet venenatis orci eu mollis. Phasellus rhoncus bibendum lacus sit amet cursus. Aliquam erat",
	" volutpat. Phasellus auctor ipsum vel efficitur interdum. Duis sed elit tempor, convallis lacus sed, accumsan mi. Integer",
	" porttitor a nunc in porttitor. Vestibulum felis enim, pretium vel nulla vel, commodo mollis ex. Sed placerat mollis leo, ",
	"at varius eros elementum vitae. Nunc aliquet velit quis dui facilisis elementum. Etiam interdum lobortis nisi, vitae ",
	"convallis libero tincidunt at. Nam eu velit et velit dignissim aliquet facilisis id ipsum. Vestibulum hendrerit, arcu ",
	"id gravida facilisis, felis leo malesuada eros, non dignissim quam turpis a massa. ",
}

// TODO: Adding more tests including negative tests by date: 7/7/2017

func TestCloudWatchLogsService_DescribeLogGroups(t *testing.T) {
	service := CloudWatchLogsService{
		cloudWatchLogsClient: cwLogsClientMock,
		stopPolicy:           sdkutil.NewStopPolicy("Test", 0),
	}

	output := cloudwatchlogs.DescribeLogGroupsOutput{}

	cwLogsClientMock.On("DescribeLogGroups", mock.AnythingOfType("*cloudwatchlogs.DescribeLogGroupsInput")).Return(&output, nil)

	_, err := service.DescribeLogGroups(logMock, "LogGroup", "")

	assert.NoError(t, err, "DescribeLogGroups should be called successfully")

}

func TestCloudWatchLogsService_CreateLogGroup(t *testing.T) {
	service := CloudWatchLogsService{
		cloudWatchLogsClient: cwLogsClientMock,
		stopPolicy:           sdkutil.NewStopPolicy("Test", 0),
	}

	output := cloudwatchlogs.CreateLogGroupOutput{}

	cwLogsClientMock.On("CreateLogGroup", mock.AnythingOfType("*cloudwatchlogs.CreateLogGroupInput")).Return(&output, nil)

	err := service.CreateLogGroup(logMock, "LogGroup")

	assert.NoError(t, err, "CreateLogGroup should be called successfully")

}

func TestCloudWatchLogsService_DescribeLogStreams(t *testing.T) {
	service := CloudWatchLogsService{
		cloudWatchLogsClient: cwLogsClientMock,
		stopPolicy:           sdkutil.NewStopPolicy("Test", 0),
	}

	output := cloudwatchlogs.DescribeLogStreamsOutput{}

	cwLogsClientMock.On("DescribeLogStreams", mock.AnythingOfType("*cloudwatchlogs.DescribeLogStreamsInput")).Return(&output, nil)
	_, err := service.DescribeLogStreams(logMock, "LogGroup", "LogStream", "")

	assert.NoError(t, err, "DescribeLogStreams should be called successfully")

}

func TestCloudWatchLogsService_CreateLogStream(t *testing.T) {
	service := CloudWatchLogsService{
		cloudWatchLogsClient: cwLogsClientMock,
		stopPolicy:           sdkutil.NewStopPolicy("Test", 0),
	}

	output := cloudwatchlogs.CreateLogStreamOutput{}

	cwLogsClientMock.On("CreateLogStream", mock.AnythingOfType("*cloudwatchlogs.CreateLogStreamInput")).Return(&output, nil)
	err := service.CreateLogStream(logMock, "LogGroup", "LogStream")

	assert.NoError(t, err, "CreateLogStream should be called successfully")

}

func TestCloudWatchLogsService_PutLogEvents(t *testing.T) {
	service := CloudWatchLogsService{
		cloudWatchLogsClient: cwLogsClientMock,
		stopPolicy:           sdkutil.NewStopPolicy("Test", 0),
	}

	output := cloudwatchlogs.PutLogEventsOutput{}

	messages := []*cloudwatchlogs.InputLogEvent{}

	sequenceToken := "1234"

	cwLogsClientMock.On("PutLogEvents", mock.AnythingOfType("*cloudwatchlogs.PutLogEventsInput")).Return(&output, nil)
	_, err := service.PutLogEvents(logMock, messages, "LogGroup", "LogStream", &sequenceToken)

	assert.NoError(t, err, "PutLogEvents should be called successfully")

}

func TestCloudWatchLogsService_CreateNewServiceIfUnHealthy(t *testing.T) {
	service := CloudWatchLogsService{
		cloudWatchLogsClient: cwLogsClientMock,
		stopPolicy:           sdkutil.NewStopPolicy("Test", 5),
	}

	service.stopPolicy.AddErrorCount(10)

	assert.False(t, service.stopPolicy.IsHealthy(), "Service should be unhealthy")

	service.CreateNewServiceIfUnHealthy()

	assert.True(t, service.stopPolicy.IsHealthy(), "Service should be healthy")

	service.stopPolicy = sdkutil.NewStopPolicy("Test", 0)

	service.stopPolicy.AddErrorCount(10)

	assert.True(t, service.stopPolicy.IsHealthy(), "Service should be healthy")

	service.CreateNewServiceIfUnHealthy()

	assert.True(t, service.stopPolicy.IsHealthy(), "Service should be healthy")

}

func TestCloudWatchLogsService_getNextMessage(t *testing.T) {
	service := CloudWatchLogsService{
		cloudWatchLogsClient: cwLogsClientMock,
		stopPolicy:           sdkutil.NewStopPolicy("Test", 0),
		IsFileComplete:       true,
	}

	fileName := "cwl_util_test_file"
	file, err := os.Create(fileName)
	assert.Nil(t, err, "Failed to create test file")
	file.Write([]byte(strings.Join(input, NewLineCharacter)))
	file.Close()

	// Deleting file
	defer func() {
		err = os.Remove(fileName)
		assert.Nil(t, err)
	}()

	// First Run
	// Get expected result
	var lengthCount = 0
	var expectedLastKnownLineUploadedToCWL int64 = 0
	var expectedCurrentLineNumber int64 = 0
	for _, v := range input {
		if lengthCount == 0 {
			lengthCount = len(v)
		} else {
			lengthCount = lengthCount + len(v) + len(NewLineCharacter)
		}
		expectedCurrentLineNumber++
		if lengthCount > MessageLengthThresholdInBytes {
			break
		}
	}

	// Get actual result
	var actualLastKnownLineUploadedToCWL int64 = 0
	var actualCurrentLineNumber int64 = 0
	message, eof := service.getNextMessage(logMock, fileName, &actualLastKnownLineUploadedToCWL, &actualCurrentLineNumber)

	// Compare results
	assert.Equal(t, expectedLastKnownLineUploadedToCWL, actualLastKnownLineUploadedToCWL)
	assert.Equal(t, expectedCurrentLineNumber, actualCurrentLineNumber)
	assert.Equal(t, lengthCount, len(message))
	assert.False(t, eof)
	assert.Equal(t, strings.Join(input[:actualCurrentLineNumber], NewLineCharacter), string(message))

	// Final Run
	// Get expected result
	expectedLastKnownLineUploadedToCWL = expectedCurrentLineNumber

	// Get actual result
	actualLastKnownLineUploadedToCWL = actualCurrentLineNumber
	message, eof = service.getNextMessage(logMock, fileName, &actualLastKnownLineUploadedToCWL, &actualCurrentLineNumber)

	// Compare results
	assert.Equal(t, expectedLastKnownLineUploadedToCWL, actualLastKnownLineUploadedToCWL)
	assert.Equal(t, expectedCurrentLineNumber, actualCurrentLineNumber)
	assert.Equal(t, 0, len(message))
	assert.True(t, eof)
	assert.Nil(t, message)
}

func TestCloudWatchLogsService_IsLogGroupEncryptedWithKMS(t *testing.T) {
	service := CloudWatchLogsService{
		cloudWatchLogsClient: cwLogsClientMock,
		stopPolicy:           sdkutil.NewStopPolicy("Test", 0),
	}

	output := cloudwatchlogs.DescribeLogGroupsOutput{}

	cwLogsClientMock.On("DescribeLogGroups", mock.AnythingOfType("*cloudwatchlogs.DescribeLogGroupsInput")).Return(&output, nil)
	encrypted := service.IsLogGroupEncryptedWithKMS(logMock, "LogGroup")
	assert.False(t, encrypted)
}
