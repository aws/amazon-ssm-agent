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

	cloudwatchlogspublisher_mock "github.com/aws/amazon-ssm-agent/agent/agentlogstocloudwatch/cloudwatchlogspublisher/mock"
	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/amazon-ssm-agent/agent/sdkutil"
	"github.com/aws/aws-sdk-go/service/cloudwatchlogs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

var logMock = log.NewMockLog()
var cwLogsClientMock = cloudwatchlogspublisher_mock.NewClientMockDefault(logMock)
var input = []string{
	"AWS Systems Manager Agent (SSM Agent) is Amazon software that runs on your Amazon EC2 instances and your hybrid instances that are configured for Systems Manager (hybrid instances).",
	"SSM Agent processes requests from the Systems Manager service in the cloud and configures your machine as specified in the request. SSM Agent sends status and execution information back to the Systems Manager service by using the EC2 Messaging service.",
	"If you monitor traffic, you will see your instances communicating with ec2messages.* endpoints. For more information, see Reference: Ec2messages and Other API Calls.",
	"SSM Agent is installed, by default, on the following Amazon EC2 Amazon Machine Image (AMIs): Windows Server (all SKUs), Amazon Linux, Amazon Linux 2, Ubuntu Server 16.04, Ubuntu Server 18.04",
	"You must manually install the agent on Amazon EC2 instances created from other Linux AMIs and on Linux servers or virtual machines in your on-premises environment.",
	"The SSM Agent download and installation process for hybrid instances is different than Amazon EC2 instances. For more information, see Install SSM Agent on Servers and VMs in a Windows Hybrid Environment.",
	"For information about porting SSM Agent logs to Amazon CloudWatch Logs, see Monitoring Instances with AWS Systems Manager.",
	"Use the following procedures to install, configure, or uninstall SSM Agent. This section also includes information about configuring SSM Agent to use a proxy.",
	"SSM Agent is installed by default on Windows Server 2016 instances. It is also installed by default on instances created from Windows Server 2003-2012 R2 AMIs published in November 2016 or later.",
	"You don't need to install SSM Agent on these instances. If you need to update SSM Agent, we recommend that you use State Manager to automatically update SSM Agent on your instances when new versions become available.",
	"For more information, see Walkthrough: Automatically Update SSM Agent (CLI).",
	"If your instance is a Windows Server 2003-2012 R2 instance created before November 2016, then EC2Config processes Systems Manager requests on your instance. We recommend that you upgrade your existing instances to use the latest version of EC2Config.",
	"By using the latest EC2Config installer, you install SSM Agent side-by-side with EC2Config.",
	"This side-by-side version of SSM Agent is compatible with your instances created from earlier Windows AMIs and enables you to use SSM features published after November 2016.",
	"For information about how to install the latest version of the EC2Config service, see Installing the Latest Version of EC2Config in the Amazon EC2 User Guide for Windows Instances.",
	"SSM Agent writes information about executions, scheduled actions, errors, and health statuses to log files on each instance.",
	"You can view log files by manually connecting to an instance, or you can automatically send logs to Amazon CloudWatch Logs.",
	"For more information about sending logs to CloudWatch, see Monitoring Instances with AWS Systems Manager.",
	"You can view SSM Agent log files on Windows instances in the following locations. %PROGRAMDATA%\\Amazon\\SSM\\Logs\\amazon-ssm-agent.log and %PROGRAMDATA%\\Amazon\\SSM\\Logs\\errors.log",
	"The information in this topic applies to Windows Server instances created in or after November 2016 that do not use the Nano installation option.",
	"If your instance is a Windows Server 2003-2012 R2 instance created before November 2016, then EC2Config processes Systems Manager requests on your instance.",
	"For information about configuring EC2Config to use a proxy, see Configure Proxy Settings for the EC2Config Service.",
	"For Windows Server 2016 instances that use the Nano installation option (Nano Server), you must connect using PowerShell. For more information, see Connect to a Windows Server 2016 Nano Server Instance.",
	"SSM Agent runs on Amazon EC2 instances using root permissions (Linux) or SYSTEM permissions (Windows).",
	"Because these are the highest level of system access privileges, any trusted entity that has been granted permission to send commands to SSM Agent has root or SYSTEM permissions.",
	"In AWS, a trusted entity that can perform actions and access resources in AWS is called a principal. A principal can be an AWS account root user, an IAM user, or a role.)",
	"This level of access is required for a principal to send authorized Systems Manager commands to SSM Agent, but also makes it possible for a principal to run malicious code by exploiting any potential vulnerabilities in SSM Agent.",
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

	service.CreateNewServiceIfUnHealthy(logMock)

	assert.True(t, service.stopPolicy.IsHealthy(), "Service should be healthy")

	service.stopPolicy = sdkutil.NewStopPolicy("Test", 0)

	service.stopPolicy.AddErrorCount(10)

	assert.True(t, service.stopPolicy.IsHealthy(), "Service should be healthy")

	service.CreateNewServiceIfUnHealthy(logMock)

	assert.True(t, service.stopPolicy.IsHealthy(), "Service should be healthy")

}

func TestCloudWatchLogsService_getNextMessage(t *testing.T) {
	service := CloudWatchLogsService{
		cloudWatchLogsClient: cwLogsClientMock,
		stopPolicy:           sdkutil.NewStopPolicy("Test", 0),
		IsFileComplete:       true,
	}

	// Mock long lines
	input[0] = strings.Repeat(input[0], 2210)
	input[6] = strings.Repeat(input[6], 1620)

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

	type messageEnd struct {
		lineNumber   int64
		prefixNumber int64
	}

	// First Run - Get expected result
	var messageEndings []messageEnd
	var runningMessageLength = 0
	var expectedLastKnownLineUploadedToCWL int64 = 0
	var expectedLastKnownPrefixUploadedToCWL int64 = 0
	var expectedCurrentLineNumber int64 = 0
	var expectedCurrentPrefixNumber int64 = 0

	// Iterate through input and mock how messages should be
	// extracted when reading from the cwl file.
	for _, v := range input {
		expectedCurrentPrefixNumber = 0
		IsMaxEventsReached := false
		// Loop until current line can be added to latest message without passing threshold.
		// Each iteration slices out a 'prefix' from current line of size up to the threshold.
		for (runningMessageLength + len(v)) >= MessageLengthThresholdInBytes {
			// Finish with latest message if there is one.
			if runningMessageLength != 0 {
				// Track message's last line and prefix number.
				messageEndings = append(messageEndings, messageEnd{
					expectedCurrentLineNumber,
					expectedCurrentPrefixNumber,
				})
			}
			// Stop iterating through input if we reach max events
			if len(messageEndings) >= maxNumberOfEventsPerCall {
				IsMaxEventsReached = true
				break
			}

			// Move to next message in the current line
			nextMessageStart := MessageLengthThresholdInBytes
			if len(v) < MessageLengthThresholdInBytes {
				// No next message on this line. Will not loop again.
				nextMessageStart = len(v)
			}
			v = v[nextMessageStart:len(v)]
			// Add length of the sliced out prefix to the running total
			runningMessageLength = nextMessageStart
			expectedCurrentPrefixNumber++
		}

		if IsMaxEventsReached {
			break
		}

		// Skip this if line was added completely while 'prefix' slicing
		if expectedCurrentPrefixNumber == 0 || len(v) > 0 {
			// Add length of line of the running total
			if runningMessageLength == 0 {
				runningMessageLength = len(v)
			} else {
				runningMessageLength += len(v) + len(NewLineCharacter)
			}
		}

		expectedCurrentLineNumber++
		expectedCurrentPrefixNumber = 0
	}

	// Include final message if exists and max events hasn't been reached
	if runningMessageLength != 0 && len(messageEndings) < maxNumberOfEventsPerCall {
		messageEndings = append(messageEndings, messageEnd{
			expectedCurrentLineNumber,
			expectedCurrentPrefixNumber,
		})
	}

	// Get actual result
	var actualLastKnownLineUploadedToCWL int64 = 0
	var actualLastKnownPrefixUploadedToCWL int64 = 0
	var actualCurrentLineNumber int64 = 0
	var actualCurrentPrefixNumber int64 = 0
	nextMessages, eof := service.getNextMessage(
		logMock,
		fileName,
		&actualLastKnownLineUploadedToCWL,
		&actualLastKnownPrefixUploadedToCWL,
		&actualCurrentLineNumber,
		&actualCurrentPrefixNumber)

	// Compare results
	assert.Equal(t, expectedLastKnownLineUploadedToCWL, actualLastKnownLineUploadedToCWL)
	assert.Equal(t, expectedLastKnownPrefixUploadedToCWL, actualLastKnownPrefixUploadedToCWL)
	assert.Equal(t, expectedCurrentLineNumber, actualCurrentLineNumber)
	assert.Equal(t, expectedCurrentPrefixNumber, actualCurrentPrefixNumber)
	assert.Equal(t, len(messageEndings), len(nextMessages))
	assert.False(t, eof)

	// Gather actual content of messages from stored line/prefix numbers for comparison
	var lastLineNumber int64 = 0
	var lastPrefixNumber int64 = 0
	for i, v := range messageEndings {
		// Slice the relevant lines
		var inputSlice []string
		if v.lineNumber == int64(len(input)) {
			inputSlice = input[lastLineNumber:]
		} else if v.prefixNumber > 0 {
			inputSlice = input[lastLineNumber : v.lineNumber+1]
		} else {
			inputSlice = input[lastLineNumber:v.lineNumber]
		}

		// Clone elements
		inputSliceCopy := make([]string, len(inputSlice))
		copy(inputSliceCopy, inputSlice)

		// Handle slicing out the end of last line to include only relevant 'prefixes'
		if v.prefixNumber > 0 {
			prefixEndIndex := v.prefixNumber * MessageLengthThresholdInBytes
			sliceLastIndex := len(inputSliceCopy) - 1
			if int64(len(inputSliceCopy[sliceLastIndex])) < prefixEndIndex {
				prefixEndIndex = int64(len(inputSliceCopy[sliceLastIndex]))
			}
			inputSliceCopy[sliceLastIndex] = inputSliceCopy[sliceLastIndex][:prefixEndIndex]
		}
		// Handle slicing beginning of first line to include only relevant 'prefixes'
		if lastPrefixNumber > 0 {
			inputSliceCopy[0] = inputSliceCopy[0][lastPrefixNumber*MessageLengthThresholdInBytes:]
		}

		assert.Equal(t, strings.Join(inputSliceCopy, NewLineCharacter), *nextMessages[i].Message)
		lastLineNumber = v.lineNumber
		lastPrefixNumber = v.prefixNumber
	}

	// Final Run
	// Get expected result
	expectedLastKnownLineUploadedToCWL = expectedCurrentLineNumber
	expectedLastKnownPrefixUploadedToCWL = expectedCurrentPrefixNumber

	// Get actual result
	actualLastKnownLineUploadedToCWL = actualCurrentLineNumber
	actualLastKnownPrefixUploadedToCWL = actualCurrentPrefixNumber
	nextMessages, eof = service.getNextMessage(
		logMock,
		fileName,
		&actualLastKnownLineUploadedToCWL,
		&actualLastKnownPrefixUploadedToCWL,
		&actualCurrentLineNumber,
		&actualCurrentPrefixNumber)

	// Compare results
	assert.Equal(t, expectedLastKnownLineUploadedToCWL, actualLastKnownLineUploadedToCWL)
	assert.Equal(t, expectedCurrentLineNumber, actualCurrentLineNumber)
	assert.Equal(t, 0, len(nextMessages))
	assert.True(t, eof)
	assert.Nil(t, nextMessages)
}

func TestCloudWatchLogsService_IsLogGroupEncryptedWithKMS(t *testing.T) {
	service := CloudWatchLogsService{
		cloudWatchLogsClient: cwLogsClientMock,
		stopPolicy:           sdkutil.NewStopPolicy("Test", 0),
	}

	output := cloudwatchlogs.DescribeLogGroupsOutput{}

	cwLogsClientMock.On("DescribeLogGroups", mock.AnythingOfType("*cloudwatchlogs.DescribeLogGroupsInput")).Return(&output, nil)
	encrypted, _ := service.IsLogGroupEncryptedWithKMS(logMock, "LogGroup")
	assert.False(t, encrypted)
}
