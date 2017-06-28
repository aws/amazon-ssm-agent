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
	"testing"

	"github.com/aws/amazon-ssm-agent/agent/cloudwatchlogspublisher/mock"
	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/amazon-ssm-agent/agent/sdkutil"
	"github.com/aws/aws-sdk-go/service/cloudwatchlogs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

var logMock = log.NewMockLog()
var cwLogsClientMock = cloudwatchlogspublisher_mock.NewClientMockDefault()

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
