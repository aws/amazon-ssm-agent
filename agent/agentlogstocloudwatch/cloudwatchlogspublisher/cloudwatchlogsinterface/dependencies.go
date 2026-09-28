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
	cont "context"

	"github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs/types"
)

// CloudWatchLogsClient interface for *cloudwatchlogs.CloudWatchLogs
type CloudWatchLogsClient interface {
	CreateLogGroup(ctx cont.Context, params *cloudwatchlogs.CreateLogGroupInput, optFns ...func(*cloudwatchlogs.Options)) (*cloudwatchlogs.CreateLogGroupOutput, error)
	CreateLogStream(ctx cont.Context, params *cloudwatchlogs.CreateLogStreamInput, optFns ...func(*cloudwatchlogs.Options)) (*cloudwatchlogs.CreateLogStreamOutput, error)
	DescribeLogGroups(ctx cont.Context, params *cloudwatchlogs.DescribeLogGroupsInput, optFns ...func(*cloudwatchlogs.Options)) (*cloudwatchlogs.DescribeLogGroupsOutput, error)
	PutLogEvents(ctx cont.Context, params *cloudwatchlogs.PutLogEventsInput, optFns ...func(*cloudwatchlogs.Options)) (*cloudwatchlogs.PutLogEventsOutput, error)
	DescribeLogStreams(ctx cont.Context, params *cloudwatchlogs.DescribeLogStreamsInput, optFns ...func(*cloudwatchlogs.Options)) (*cloudwatchlogs.DescribeLogStreamsOutput, error)
}

// ICloudWatchLogsService interface for CloudWatchLogsService
type ICloudWatchLogsService interface {
	CreateNewServiceIfUnHealthy()
	CreateLogGroup(logGroup string) (err error)
	CreateLogStream(logGroup, logStream string) (err error)
	DescribeLogGroups(logGroupPrefix, nextToken string) (response *cloudwatchlogs.DescribeLogGroupsOutput, err error)
	DescribeLogStreams(logGroup, logStreamPrefix, nextToken string) (response *cloudwatchlogs.DescribeLogStreamsOutput, err error)
	IsLogGroupPresent(logGroup string) (bool, *types.LogGroup)
	IsLogStreamPresent(logGroupName, logStreamName string) bool
	GetSequenceTokenForStream(logGroupName, logStreamName string) (sequenceToken *string)
	PutLogEvents(messages []types.InputLogEvent, logGroup, logStream string, sequenceToken *string) (nextSequenceToken *string, err error)
	IsLogGroupEncryptedWithKMS(logGroup *types.LogGroup) (bool, error)
	StreamData(logGroupName string, logStreamName string, absoluteFilePath string, isFileComplete bool, isLogStreamCreated bool, fileCompleteSignal chan bool, cleanupControlCharacters bool, structuredLogs bool) (success bool)
	SetCloudWatchMessage(eventVersion string, awsRegion string, targetId string, runAsUser string, sessionId string, sessionOwner string)
}
