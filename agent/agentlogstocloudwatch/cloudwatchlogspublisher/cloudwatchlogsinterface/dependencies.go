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

// cloudwatchlogsserviceinterface defines the interfaces for cloudwatchlogspublisher

package cloudwatchlogsinterface

import (
	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/aws-sdk-go/service/cloudwatchlogs"
)

// CloudWatchLogsClient interface for *cloudwatchlogs.CloudWatchLogs
type CloudWatchLogsClient interface {
	CreateLogGroup(input *cloudwatchlogs.CreateLogGroupInput) (*cloudwatchlogs.CreateLogGroupOutput, error)
	CreateLogStream(input *cloudwatchlogs.CreateLogStreamInput) (*cloudwatchlogs.CreateLogStreamOutput, error)
	DescribeLogGroups(input *cloudwatchlogs.DescribeLogGroupsInput) (*cloudwatchlogs.DescribeLogGroupsOutput, error)
	PutLogEvents(input *cloudwatchlogs.PutLogEventsInput) (*cloudwatchlogs.PutLogEventsOutput, error)
	DescribeLogStreams(input *cloudwatchlogs.DescribeLogStreamsInput) (*cloudwatchlogs.DescribeLogStreamsOutput, error)
}

// ICloudWatchLogsService interface for CloudWatchLogsService
type ICloudWatchLogsService interface {
	CreateNewServiceIfUnHealthy()
	CreateLogGroup(log log.T, logGroup string) (err error)
	CreateLogStream(log log.T, logGroup, logStream string) (err error)
	DescribeLogGroups(log log.T, logGroupPrefix, nextToken string) (response *cloudwatchlogs.DescribeLogGroupsOutput, err error)
	DescribeLogStreams(log log.T, logGroup, logStreamPrefix, nextToken string) (response *cloudwatchlogs.DescribeLogStreamsOutput, err error)
	IsLogGroupPresent(log log.T, logGroup string) bool
	IsLogStreamPresent(log log.T, logGroupName, logStreamName string) bool
	GetSequenceTokenForStream(log log.T, logGroupName, logStreamName string) (sequenceToken *string)
	PutLogEvents(log log.T, messages []*cloudwatchlogs.InputLogEvent, logGroup, logStream string, sequenceToken *string) (nextSequenceToken *string, err error)
	IsLogGroupEncryptedWithKMS(log log.T, logGroupName string) bool
	StreamData(log log.T, logGroupName string, logStreamName string, absoluteFilePath string, isFileComplete bool, isLogStreamCreated bool)
}
