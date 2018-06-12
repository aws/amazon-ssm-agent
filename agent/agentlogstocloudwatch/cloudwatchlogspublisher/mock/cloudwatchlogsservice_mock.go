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

// cloudwatchlogspublisher_mock implements the mocks required for testing cloudwatchlogspublisher

package cloudwatchlogspublisher_mock

import (
	"github.com/aws/amazon-ssm-agent/agent/agentlogstocloudwatch/cloudwatchlogspublisher/cloudwatchlogsinterface"
	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/aws-sdk-go/service/cloudwatchlogs"
	"github.com/stretchr/testify/mock"
)

// CloudWatchLogsClientMock mocks CloudWatchLogsClient
type CloudWatchLogsClientMock struct {
	mock.Mock
}

// CloudWatchLogsServiceMock mocks CloudWatchLogsService
type CloudWatchLogsServiceMock struct {
	mock.Mock
	cloudWatchLogsClient cloudwatchlogsinterface.CloudWatchLogsClient
}

// NewClientMockDefault returns an instance of CloudWatchLogsClientMock with default expectations set.
func NewClientMockDefault() *CloudWatchLogsClientMock {
	return new(CloudWatchLogsClientMock)
}

// NewServiceMockDefault returns an instance of CloudWatchLogsServiceMock with default expectations set.
func NewServiceMockDefault() *CloudWatchLogsServiceMock {
	mock := new(CloudWatchLogsServiceMock)
	mock.On("CreateNewServiceIfUnHealthy").Return()
	return mock
}

// CreateLogStream mocks CloudWatchLogsClient CreateLogStream method
func (m *CloudWatchLogsClientMock) CreateLogStream(input *cloudwatchlogs.CreateLogStreamInput) (*cloudwatchlogs.CreateLogStreamOutput, error) {
	args := m.Called(input)
	return args.Get(0).(*cloudwatchlogs.CreateLogStreamOutput), args.Error(1)
}

// CreateLogGroup mocks CloudWatchLogsClient CreateLogGroup method
func (m *CloudWatchLogsClientMock) CreateLogGroup(input *cloudwatchlogs.CreateLogGroupInput) (*cloudwatchlogs.CreateLogGroupOutput, error) {
	args := m.Called(input)
	return args.Get(0).(*cloudwatchlogs.CreateLogGroupOutput), args.Error(1)
}

// PutLogEvents mocks CloudWatchLogsClient PutLogEvents method
func (m *CloudWatchLogsClientMock) PutLogEvents(input *cloudwatchlogs.PutLogEventsInput) (*cloudwatchlogs.PutLogEventsOutput, error) {
	args := m.Called(input)
	return args.Get(0).(*cloudwatchlogs.PutLogEventsOutput), args.Error(1)
}

// DescribeLogGroups mocks CloudWatchLogsClient DescribeLogGroups method
func (m *CloudWatchLogsClientMock) DescribeLogGroups(input *cloudwatchlogs.DescribeLogGroupsInput) (*cloudwatchlogs.DescribeLogGroupsOutput, error) {
	args := m.Called(input)
	return args.Get(0).(*cloudwatchlogs.DescribeLogGroupsOutput), args.Error(1)
}

// DescribeLogStreams mocks CloudWatchLogsClient DescribeLogStreams method
func (m *CloudWatchLogsClientMock) DescribeLogStreams(input *cloudwatchlogs.DescribeLogStreamsInput) (*cloudwatchlogs.DescribeLogStreamsOutput, error) {
	args := m.Called(input)
	return args.Get(0).(*cloudwatchlogs.DescribeLogStreamsOutput), args.Error(1)
}

// CreateNewServiceIfUnHealthy mocks CloudWatchLogsService CreateNewServiceIfUnHealthy method
func (m *CloudWatchLogsServiceMock) CreateNewServiceIfUnHealthy() {

}

// CreateLogGroup mocks CloudWatchLogsService CreateLogGroup method
func (m *CloudWatchLogsServiceMock) CreateLogGroup(log log.T, logGroup string) (err error) {
	args := m.Called(log, logGroup)
	return args.Error(0)
}

// CreateLogStream mocks CloudWatchLogsService CreateLogStream method
func (m *CloudWatchLogsServiceMock) CreateLogStream(log log.T, logGroup, logStream string) (err error) {
	args := m.Called(log, logGroup, logStream)
	return args.Error(0)
}

// DescribeLogGroups mocks CloudWatchLogsService DescribeLogGroups method
func (m *CloudWatchLogsServiceMock) DescribeLogGroups(log log.T, logGroupPrefix, nextToken string) (response *cloudwatchlogs.DescribeLogGroupsOutput, err error) {
	args := m.Called(log, logGroupPrefix, nextToken)
	return args.Get(0).(*cloudwatchlogs.DescribeLogGroupsOutput), args.Error(1)
}

// DescribeLogStreams mocks CloudWatchLogsService DescribeLogStreams method
func (m *CloudWatchLogsServiceMock) DescribeLogStreams(log log.T, logGroup, logStreamPrefix, nextToken string) (response *cloudwatchlogs.DescribeLogStreamsOutput, err error) {
	args := m.Called(log, logGroup, logStreamPrefix, nextToken)
	return args.Get(0).(*cloudwatchlogs.DescribeLogStreamsOutput), args.Error(1)
}

// GetLogGroupDetails mocks CloudWatchLogsService getLogGroupDetails method
func (m *CloudWatchLogsServiceMock) GetLogGroupDetails(log log.T, logGroup string) (logGroupDetails *cloudwatchlogs.LogGroup) {
	args := m.Called(log, logGroup)
	return args.Get(0).(*cloudwatchlogs.LogGroup)
}

// IsLogGroupPresent mocks CloudWatchLogsService IsLogGroupPresent method
func (m *CloudWatchLogsServiceMock) IsLogGroupPresent(log log.T, logGroup string) bool {
	args := m.Called(log, logGroup)
	return args.Bool(0)
}

// IsLogStreamPresent mocks CloudWatchLogsService IsLogStreamPresent method
func (m *CloudWatchLogsServiceMock) IsLogStreamPresent(log log.T, logGroupName, logStreamName string) bool {
	args := m.Called(log, logGroupName, logStreamName)
	return args.Bool(0)
}

// GetSequenceTokenForStream mocks CloudWatchLogsService GetSequenceTokenForStream method
func (m *CloudWatchLogsServiceMock) GetSequenceTokenForStream(log log.T, logGroupName, logStreamName string) (sequenceToken *string) {
	args := m.Called(log, logGroupName, logStreamName)
	if args.Get(0) == nil {
		return nil
	}
	return args.Get(0).(*string)
}

// getLogStreamDetails mocks CloudWatchLogsService getLogStreamDetails method
func (m *CloudWatchLogsServiceMock) getLogStreamDetails(log log.T, logGroupName, logStreamName string) (logStream *cloudwatchlogs.LogStream) {
	args := m.Called(log, logGroupName, logStreamName)
	return args.Get(0).(*cloudwatchlogs.LogStream)
}

// PutLogEvents mocks CloudWatchLogsService PutLogEvents method
func (m *CloudWatchLogsServiceMock) PutLogEvents(log log.T, messages []*cloudwatchlogs.InputLogEvent, logGroup, logStream string, sequenceToken *string) (nextSequenceToken *string, err error) {
	args := m.Called(log, messages, logGroup, logStream, sequenceToken)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*string), args.Error(1)
}

// retryPutWithNewSequenceToken mocks CloudWatchLogsService retryPutWithNewSequenceToken method
func (m *CloudWatchLogsServiceMock) retryPutWithNewSequenceToken(log log.T, messages []*cloudwatchlogs.InputLogEvent, logGroupName, logStreamName string) (*string, error) {
	args := m.Called(log, messages, logGroupName, logStreamName)
	return args.Get(0).(*string), args.Error(1)
}

// IsLogGroupEncryptedWithKMS mocks CloudWatchLogsService IsLogGroupEncryptedWithKMS method
func (m *CloudWatchLogsServiceMock) IsLogGroupEncryptedWithKMS(log log.T, logGroupName string) bool {
	args := m.Called(log, logGroupName)
	return args.Get(0).(bool)
}

// StreamData mocks CloudWatchLogsService StreamData method
func (m *CloudWatchLogsServiceMock) StreamData(log log.T, logGroupName string, logStreamName string, absoluteFilePath string, isFileComplete bool, isLogStreamCreated bool) {
	m.Called(log, logGroupName, logStreamName, absoluteFilePath, isFileComplete, isLogStreamCreated)
}
