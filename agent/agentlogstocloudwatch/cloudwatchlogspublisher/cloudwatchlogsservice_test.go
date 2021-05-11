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
	"encoding/json"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	cloudwatchlogspublisher_mock "github.com/aws/amazon-ssm-agent/agent/agentlogstocloudwatch/cloudwatchlogspublisher/mock"
	"github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/aws/amazon-ssm-agent/agent/sdkutil"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/service/cloudwatchlogs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

var (
	contextMock      = context.NewMockDefault()
	logMock          = contextMock.Log()
	cwLogsClientMock = cloudwatchlogspublisher_mock.NewClientMockDefault(logMock)
	logGroupName     = "logGroupName"
	logStreamName    = "logStreamName"
	sequenceToken    = "sequenceToken"
	eventVersion     = "1.0"
	awsRegion        = "awsRegion"
	targetId         = "targetId"
	runAsUser        = "runAsUser"
	sessionId        = "sessionId"
	sessionOwner     = "sessionOwner"
	input            = []string{
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
)

// TODO: Adding more tests including negative tests by date: 7/7/2017

func TestCloudWatchLogsService_DescribeLogGroups(t *testing.T) {
	service := CloudWatchLogsService{
		context:              contextMock,
		cloudWatchLogsClient: cwLogsClientMock,
		stopPolicy:           sdkutil.NewStopPolicy("Test", 0),
	}

	output := cloudwatchlogs.DescribeLogGroupsOutput{}

	cwLogsClientMock.On("DescribeLogGroups", mock.AnythingOfType("*cloudwatchlogs.DescribeLogGroupsInput")).Return(&output, nil)

	_, err := service.DescribeLogGroups("LogGroup", "")

	assert.NoError(t, err, "DescribeLogGroups should be called successfully")

}

func TestCloudWatchLogsService_CreateLogGroup(t *testing.T) {
	service := CloudWatchLogsService{
		context:              contextMock,
		cloudWatchLogsClient: cwLogsClientMock,
		stopPolicy:           sdkutil.NewStopPolicy("Test", 0),
	}

	output := cloudwatchlogs.CreateLogGroupOutput{}

	cwLogsClientMock.On("CreateLogGroup", mock.AnythingOfType("*cloudwatchlogs.CreateLogGroupInput")).Return(&output, nil)

	err := service.CreateLogGroup("LogGroup")

	assert.NoError(t, err, "CreateLogGroup should be called successfully")

}

func TestCloudWatchLogsService_DescribeLogStreams(t *testing.T) {
	service := CloudWatchLogsService{
		context:              contextMock,
		cloudWatchLogsClient: cwLogsClientMock,
		stopPolicy:           sdkutil.NewStopPolicy("Test", 0),
	}

	output := cloudwatchlogs.DescribeLogStreamsOutput{}

	cwLogsClientMock.On("DescribeLogStreams", mock.AnythingOfType("*cloudwatchlogs.DescribeLogStreamsInput")).Return(&output, nil)
	_, err := service.DescribeLogStreams("LogGroup", "LogStream", "")

	assert.NoError(t, err, "DescribeLogStreams should be called successfully")

}

func TestCloudWatchLogsService_CreateLogStream(t *testing.T) {
	service := CloudWatchLogsService{
		context:              contextMock,
		cloudWatchLogsClient: cwLogsClientMock,
		stopPolicy:           sdkutil.NewStopPolicy("Test", 0),
	}

	output := cloudwatchlogs.CreateLogStreamOutput{}

	cwLogsClientMock.On("CreateLogStream", mock.AnythingOfType("*cloudwatchlogs.CreateLogStreamInput")).Return(&output, nil)
	err := service.CreateLogStream("LogGroup", "LogStream")

	assert.NoError(t, err, "CreateLogStream should be called successfully")

}

func TestCloudWatchLogsService_PutLogEvents(t *testing.T) {
	service := CloudWatchLogsService{
		context:              contextMock,
		cloudWatchLogsClient: cwLogsClientMock,
		stopPolicy:           sdkutil.NewStopPolicy("Test", 0),
	}

	output := cloudwatchlogs.PutLogEventsOutput{}

	messages := []*cloudwatchlogs.InputLogEvent{}

	sequenceToken := "1234"

	cwLogsClientMock.On("PutLogEvents", mock.AnythingOfType("*cloudwatchlogs.PutLogEventsInput")).Return(&output, nil)
	_, err := service.PutLogEvents(messages, "LogGroup", "LogStream", &sequenceToken)

	assert.NoError(t, err, "PutLogEvents should be called successfully")

}

func TestCloudWatchLogsService_CreateNewServiceIfUnHealthy(t *testing.T) {
	service := CloudWatchLogsService{
		context:              context.NewMockDefault(),
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
		context:              contextMock,
		cloudWatchLogsClient: cwLogsClientMock,
		stopPolicy:           sdkutil.NewStopPolicy("Test", 0),
		isFileComplete:       true,
	}

	fileName := "cwl_util_test_file"
	file, err := os.Create(fileName)
	assert.Nil(t, err, "Failed to create test file")
	file.Write([]byte(strings.Join(input, "\n")))
	file.Close()

	// Deleting file
	defer func() {
		err = os.Remove(fileName)
		assert.Nil(t, err)
	}()

	// First Run
	// Get expected result
	var totalMessages []int64
	var lengthCount = 0
	var expectedLastKnownLineUploadedToCWL int64 = 0
	var expectedCurrentLineNumber int64 = 0
	for _, v := range input {
		if lengthCount == 0 {
			lengthCount = len(v)
		} else if (lengthCount + len(v)) > MessageLengthThresholdInBytes {
			totalMessages = append(totalMessages, expectedCurrentLineNumber)
			if len(totalMessages) >= maxNumberOfEventsPerCall {
				break
			}

			lengthCount = len(v)
		} else {
			lengthCount = lengthCount + len(v) + len("\n")
		}
		expectedCurrentLineNumber++
	}

	if lengthCount != 0 {
		totalMessages = append(totalMessages, expectedCurrentLineNumber)
	}

	// Get actual result
	var actualLastKnownLineUploadedToCWL int64 = 0
	var actualCurrentLineNumber int64 = 0
	message, eof := service.getNextMessage(fileName, &actualLastKnownLineUploadedToCWL, &actualCurrentLineNumber, false, false)

	// Compare results
	assert.Equal(t, expectedLastKnownLineUploadedToCWL, actualLastKnownLineUploadedToCWL)
	assert.Equal(t, expectedCurrentLineNumber, actualCurrentLineNumber)
	assert.Equal(t, len(totalMessages), len(message))
	assert.False(t, eof)

	for i, v := range totalMessages {
		assert.Equal(t, strings.Join(input[:v], "\n"), *message[i].Message)
	}

	// Final Run
	// Get expected result
	expectedLastKnownLineUploadedToCWL = expectedCurrentLineNumber

	// Get actual result
	actualLastKnownLineUploadedToCWL = actualCurrentLineNumber
	message, eof = service.getNextMessage(fileName, &actualLastKnownLineUploadedToCWL, &actualCurrentLineNumber, false, false)

	// Compare results
	assert.Equal(t, expectedLastKnownLineUploadedToCWL, actualLastKnownLineUploadedToCWL)
	assert.Equal(t, expectedCurrentLineNumber, actualCurrentLineNumber)
	assert.Equal(t, 0, len(message))
	assert.True(t, eof)
	assert.Nil(t, message)
}

func TestCloudWatchLogsService_getNextMessage_largeline(t *testing.T) {
	service := CloudWatchLogsService{
		context:              contextMock,
		cloudWatchLogsClient: cwLogsClientMock,
		stopPolicy:           sdkutil.NewStopPolicy("Test", 0),
		isFileComplete:       true,
	}

	fileName := "cwl_util_test_file"
	file, err := os.Create(fileName)
	assert.Nil(t, err, "Failed to create test file")
	// writing more than 200K bytes
	var largeLine string
	for i := 0; i < 50; i++ {
		largeLine += strings.Join(input, " ")
	}
	file.Write([]byte(largeLine))
	file.Close()

	// Deleting file
	defer func() {
		err = os.Remove(fileName)
		assert.Nil(t, err)
	}()

	// Get actual result
	var actualLastKnownLineUploadedToCWL int64 = 0
	var actualCurrentLineNumber int64 = 0
	events, eof := service.getNextMessage(fileName, &actualLastKnownLineUploadedToCWL, &actualCurrentLineNumber, false, false)

	// Compare results
	// Since length of line exceeds max limit of 200K, messages will be sent in 2 separate events
	assert.Equal(t, 2, len(events))
	assert.False(t, eof)
	assert.Equal(t, largeLine[:200000], *events[0].Message)
	assert.Equal(t, largeLine[200000:], *events[1].Message)
}

func TestCloudWatchLogsService_getNextMessage_ending_with_newlinecharacter(t *testing.T) {
	service := CloudWatchLogsService{
		context:              contextMock,
		cloudWatchLogsClient: cwLogsClientMock,
		stopPolicy:           sdkutil.NewStopPolicy("Test", 0),
		isFileComplete:       true,
	}

	fileName := "cwl_util_test_file"
	file, err := os.Create(fileName)
	assert.Nil(t, err, "Failed to create test file")
	// writing more than 200K bytes
	file.Write([]byte("Test CloudWatch\n"))
	file.Close()

	// Deleting file
	defer func() {
		err = os.Remove(fileName)
		assert.Nil(t, err)
	}()

	// Get actual result
	var actualLastKnownLineUploadedToCWL int64 = 0
	var actualCurrentLineNumber int64 = 0
	events, eof := service.getNextMessage(fileName, &actualLastKnownLineUploadedToCWL, &actualCurrentLineNumber, false, false)

	// Compare results where output does not contain \n at the end of the line
	assert.Equal(t, 1, len(events))
	assert.False(t, eof)
	assert.Equal(t, "Test CloudWatch", *events[0].Message)
}

func TestCloudWatchLogsService_getNextMessage_structuredLogs(t *testing.T) {
	service := CloudWatchLogsService{
		context:              contextMock,
		cloudWatchLogsClient: cwLogsClientMock,
		stopPolicy:           sdkutil.NewStopPolicy("Test", 0),
		isFileComplete:       true,
	}

	fileName := "cwl_util_test_file"
	file, err := os.Create(fileName)
	assert.Nil(t, err, "Failed to create test file")
	file.Write([]byte("Test for log upload to CloudWatch\n"))
	file.Close()

	// Deleting file
	defer func() {
		err = os.Remove(fileName)
		assert.Nil(t, err)
	}()

	service.SetCloudWatchMessage(
		eventVersion,
		awsRegion,
		targetId,
		runAsUser,
		sessionId,
		sessionOwner,
	)

	// Get actual result
	var actualLastKnownLineUploadedToCWL int64 = 0
	var actualCurrentLineNumber int64 = 0
	events, eof := service.getNextMessage(
		fileName,
		&actualLastKnownLineUploadedToCWL,
		&actualCurrentLineNumber,
		false,
		true)

	// Compare results
	assert.Equal(t, int64(0), actualLastKnownLineUploadedToCWL)
	assert.Equal(t, int64(1), actualCurrentLineNumber)
	assert.Equal(t, 1, len(events))
	assert.False(t, eof)

	var cloudWatchMessage *CloudWatchMessage
	err = json.Unmarshal([]byte(*events[0].Message), &cloudWatchMessage)
	assert.Nil(t, err)
	assert.Equal(t, "Test for log upload to CloudWatch", *cloudWatchMessage.SessionData[0])
	assert.Equal(t, sessionId, *cloudWatchMessage.SessionId)
	assert.Equal(t, sessionOwner, *cloudWatchMessage.UserIdentity.Arn)
	assert.Equal(t, runAsUser, *cloudWatchMessage.RunAsUser)
	assert.Equal(t, targetId, *cloudWatchMessage.Target.Id)
	assert.Equal(t, awsRegion, *cloudWatchMessage.AwsRegion)
	assert.Equal(t, eventVersion, *cloudWatchMessage.EventVersion)
}

func TestCloudWatchLogsService_getNextMessage_cleanupControlCharacters(t *testing.T) {
	service := CloudWatchLogsService{
		context:              contextMock,
		cloudWatchLogsClient: cwLogsClientMock,
		stopPolicy:           sdkutil.NewStopPolicy("Test", 0),
		isFileComplete:       true,
	}

	fileName := "cwl_util_test_file"
	file, err := os.Create(fileName)
	assert.Nil(t, err, "Failed to create test file")
	// string representation of below byte array containing control characters is "[?1034hsh-4.2$"
	file.Write([]byte{27, 91, 63, 49, 48, 51, 52, 104, 115, 104, 45, 52, 46, 50, 36})
	file.Close()

	// Deleting file
	defer func() {
		err = os.Remove(fileName)
		assert.Nil(t, err)
	}()

	service.SetCloudWatchMessage(
		eventVersion,
		awsRegion,
		targetId,
		runAsUser,
		sessionId,
		sessionOwner,
	)

	// Get actual result
	var actualLastKnownLineUploadedToCWL int64 = 0
	var actualCurrentLineNumber int64 = 0
	events, eof := service.getNextMessage(
		fileName,
		&actualLastKnownLineUploadedToCWL,
		&actualCurrentLineNumber,
		true,
		true)

	// Compare results
	assert.Equal(t, int64(0), actualLastKnownLineUploadedToCWL)
	assert.Equal(t, int64(1), actualCurrentLineNumber)
	assert.Equal(t, 1, len(events))
	assert.False(t, eof)

	var cloudWatchMessage *CloudWatchMessage
	err = json.Unmarshal([]byte(*events[0].Message), &cloudWatchMessage)
	assert.Nil(t, err)
	assert.Equal(t, "sh-4.2$", *cloudWatchMessage.SessionData[0])
	assert.Equal(t, sessionId, *cloudWatchMessage.SessionId)
	assert.Equal(t, sessionOwner, *cloudWatchMessage.UserIdentity.Arn)
	assert.Equal(t, runAsUser, *cloudWatchMessage.RunAsUser)
	assert.Equal(t, targetId, *cloudWatchMessage.Target.Id)
	assert.Equal(t, awsRegion, *cloudWatchMessage.AwsRegion)
	assert.Equal(t, eventVersion, *cloudWatchMessage.EventVersion)
}

func TestCloudWatchLogsService_StreamData(t *testing.T) {
	cwLogsClientMock = cloudwatchlogspublisher_mock.NewClientMockDefault(logMock)
	service := CloudWatchLogsService{
		context:              contextMock,
		cloudWatchLogsClient: cwLogsClientMock,
		stopPolicy:           sdkutil.NewStopPolicy("Test", 0),
	}

	fileName := "cwl_util_test_file"
	file, err := os.Create(fileName)
	assert.Nil(t, err, "Failed to create test file")
	// string representation of below byte array containing control characters is "[?1034hsh-4.2$"
	file.Write([]byte("Test Line 1\n"))

	// Deleting file
	defer func() {
		file.Close()
		err = os.Remove(fileName)
		assert.Nil(t, err)
	}()

	service.SetCloudWatchMessage(
		eventVersion,
		awsRegion,
		targetId,
		runAsUser,
		sessionId,
		sessionOwner,
	)
	fileCompleteSignal := make(chan bool)
	service.SetCloudWatchMessage(
		eventVersion,
		awsRegion,
		targetId,
		runAsUser,
		sessionId,
		sessionOwner,
	)
	cloudWatchMessage := service.CloudWatchMessage
	cloudWatchMessage.SessionData = aws.StringSlice([]string{"Test Line 1"})
	formattedMessageBytes, _ := json.Marshal(service.CloudWatchMessage)
	formattedMessage := string(formattedMessageBytes)

	var events []*cloudwatchlogs.InputLogEvent
	event := &cloudwatchlogs.InputLogEvent{
		Message:   aws.String(formattedMessage),
		Timestamp: aws.Int64(time.Now().UnixNano() / int64(time.Millisecond)),
	}
	events = append(events, event)

	var logGroups []*cloudwatchlogs.LogGroup
	logGroup := &cloudwatchlogs.LogGroup{LogGroupName: &logGroupName}
	logGroups = append(logGroups, logGroup)

	cwLogsClientMock.On("CreateLogStream", mock.Anything).Return(&cloudwatchlogs.CreateLogStreamOutput{}, nil)
	cwLogsClientMock.On("DescribeLogStreams", mock.Anything).Return(&cloudwatchlogs.DescribeLogStreamsOutput{}, nil)
	// PutLogEvents called once indicates logs was uploaded all at once
	cwLogsClientMock.On("PutLogEvents", mock.Anything).Return(&cloudwatchlogs.PutLogEventsOutput{}, nil).Once()

	// isFileComplete set to true to disable streaming of logs
	success := service.StreamData(
		logGroupName,
		logStreamName,
		fileName,
		true,
		false,
		fileCompleteSignal,
		true,
		true)

	assert.True(t, success)
	cwLogsClientMock.AssertExpectations(t)
}

func TestCloudWatchLogsService_StreamData_StreamingEnabled(t *testing.T) {
	cwLogsClientMock = cloudwatchlogspublisher_mock.NewClientMockDefault(logMock)
	service := CloudWatchLogsService{
		context:              contextMock,
		cloudWatchLogsClient: cwLogsClientMock,
		stopPolicy:           sdkutil.NewStopPolicy("Test", 0),
	}

	fileName := "cwl_util_test_file"
	file, err := os.Create(fileName)
	assert.Nil(t, err, "Failed to create test file")
	// string representation of below byte array containing control characters is "[?1034hsh-4.2$"
	file.Write([]byte("Test Line 1\n"))

	// Deleting file
	defer func() {
		file.Close()
		err = os.Remove(fileName)
		assert.Nil(t, err)
	}()

	service.SetCloudWatchMessage(
		eventVersion,
		awsRegion,
		targetId,
		runAsUser,
		sessionId,
		sessionOwner,
	)
	fileCompleteSignal := make(chan bool)
	service.SetCloudWatchMessage(
		eventVersion,
		awsRegion,
		targetId,
		runAsUser,
		sessionId,
		sessionOwner,
	)
	cloudWatchMessage := service.CloudWatchMessage
	cloudWatchMessage.SessionData = aws.StringSlice([]string{"Test Line 1"})
	formattedMessageBytes, _ := json.Marshal(service.CloudWatchMessage)
	formattedMessage := string(formattedMessageBytes)

	var events []*cloudwatchlogs.InputLogEvent
	event := &cloudwatchlogs.InputLogEvent{
		Message:   aws.String(formattedMessage),
		Timestamp: aws.Int64(time.Now().UnixNano() / int64(time.Millisecond)),
	}
	events = append(events, event)

	var logGroups []*cloudwatchlogs.LogGroup
	logGroup := &cloudwatchlogs.LogGroup{LogGroupName: &logGroupName}
	logGroups = append(logGroups, logGroup)

	cwLogsClientMock.On("CreateLogStream", mock.Anything).Return(&cloudwatchlogs.CreateLogStreamOutput{}, nil)
	cwLogsClientMock.On("DescribeLogStreams", mock.Anything).Return(&cloudwatchlogs.DescribeLogStreamsOutput{}, nil)
	// PutLogEvents calls twice indicates streaming of logs was done
	cwLogsClientMock.On("PutLogEvents", mock.Anything).Return(&cloudwatchlogs.PutLogEventsOutput{}, nil).Twice()

	go func() {
		time.Sleep(1800 * time.Millisecond)
		file.Write([]byte("Test Line 2"))
		service.isFileComplete = true
	}()

	// isFileComplete set to false is to enable streaming of logs
	success := service.StreamData(
		logGroupName,
		logStreamName,
		fileName,
		false,
		false,
		fileCompleteSignal,
		true,
		true)

	assert.True(t, success)
	cwLogsClientMock.AssertExpectations(t)
}

func TestCloudWatchLogsService_StreamData_MissingStreamPermissions(t *testing.T) {
	cwLogsClientMock = cloudwatchlogspublisher_mock.NewClientMockDefault(logMock)
	service := CloudWatchLogsService{
		context:              contextMock,
		cloudWatchLogsClient: cwLogsClientMock,
		stopPolicy:           sdkutil.NewStopPolicy("Test", 0),
	}

	fileName := "cwl_util_test_file"
	file, err := os.Create(fileName)
	assert.Nil(t, err, "Failed to create test file")
	// string representation of below byte array containing control characters is "[?1034hsh-4.2$"
	file.Write([]byte("Test Line 1\n"))

	// Deleting file
	defer func() {
		file.Close()
		err = os.Remove(fileName)
		assert.Nil(t, err)
	}()

	service.SetCloudWatchMessage(
		eventVersion,
		awsRegion,
		targetId,
		runAsUser,
		sessionId,
		sessionOwner,
	)
	fileCompleteSignal := make(chan bool)
	service.SetCloudWatchMessage(
		eventVersion,
		awsRegion,
		targetId,
		runAsUser,
		sessionId,
		sessionOwner,
	)
	cloudWatchMessage := service.CloudWatchMessage
	cloudWatchMessage.SessionData = aws.StringSlice([]string{"Test Line 1"})
	formattedMessageBytes, _ := json.Marshal(service.CloudWatchMessage)
	formattedMessage := string(formattedMessageBytes)

	var events []*cloudwatchlogs.InputLogEvent
	event := &cloudwatchlogs.InputLogEvent{
		Message:   aws.String(formattedMessage),
		Timestamp: aws.Int64(time.Now().UnixNano() / int64(time.Millisecond)),
	}
	events = append(events, event)

	var logGroups []*cloudwatchlogs.LogGroup
	logGroup := &cloudwatchlogs.LogGroup{LogGroupName: &logGroupName}
	logGroups = append(logGroups, logGroup)

	cwLogsClientMock.On("CreateLogStream", mock.Anything).Return(&cloudwatchlogs.CreateLogStreamOutput{}, errors.New("error"))

	go func() {
		time.Sleep(1800 * time.Millisecond)
		file.Write([]byte("Test Line 2"))
		service.isFileComplete = true
	}()

	// isFileComplete set to false is to enable streaming of logs
	success := service.StreamData(
		logGroupName,
		logStreamName,
		fileName,
		false,
		false,
		fileCompleteSignal,
		true,
		true)

	assert.False(t, success)
	cwLogsClientMock.AssertExpectations(t)
}

func TestCloudWatchLogsService_StreamData_InvalidLogStream(t *testing.T) {
	cwLogsClientMock = cloudwatchlogspublisher_mock.NewClientMockDefault(logMock)
	service := CloudWatchLogsService{
		context:              contextMock,
		cloudWatchLogsClient: cwLogsClientMock,
		stopPolicy:           sdkutil.NewStopPolicy("Test", 0),
	}

	fileName := "cwl_util_test_file"
	file, err := os.Create(fileName)
	assert.Nil(t, err, "Failed to create test file")
	file.Write([]byte("Test Line 1\n"))

	// Deleting file
	defer func() {
		file.Close()
		err = os.Remove(fileName)
		assert.Nil(t, err)
	}()
	fileCompleteSignal := make(chan bool)
	service.SetCloudWatchMessage(
		eventVersion,
		awsRegion,
		targetId,
		runAsUser,
		sessionId,
		sessionOwner,
	)
	cloudWatchMessage := service.CloudWatchMessage
	cloudWatchMessage.SessionData = aws.StringSlice([]string{"Test Line 1"})
	formattedMessageBytes, _ := json.Marshal(service.CloudWatchMessage)
	formattedMessage := string(formattedMessageBytes)

	var events []*cloudwatchlogs.InputLogEvent
	event := &cloudwatchlogs.InputLogEvent{
		Message:   aws.String(formattedMessage),
		Timestamp: aws.Int64(time.Now().UnixNano() / int64(time.Millisecond)),
	}
	events = append(events, event)

	var logGroups []*cloudwatchlogs.LogGroup
	logGroup := &cloudwatchlogs.LogGroup{LogGroupName: &logGroupName}
	logGroups = append(logGroups, logGroup)

	cwLogsClientMock.On("CreateLogStream", mock.Anything).Return(&cloudwatchlogs.CreateLogStreamOutput{}, nil)
	cwLogsClientMock.On("DescribeLogStreams", mock.Anything).Return(&cloudwatchlogs.DescribeLogStreamsOutput{}, nil)
	// Returns a ResourceNotFoundException error when PutLogEvents is called
	cwLogsClientMock.On("PutLogEvents", mock.Anything).Return(
		&cloudwatchlogs.PutLogEventsOutput{},
		awserr.New("ResourceNotFoundException",
			"Mocked ResourceNotFound Response from AWS API",
			nil),
	)

	go func() {
		time.Sleep(1800 * time.Millisecond)
		file.Write([]byte("Test Line 2"))
		service.isFileComplete = true
	}()

	// isFileComplete set to false is to enable streaming of logs
	success := service.StreamData(
		logGroupName,
		logStreamName,
		fileName,
		false,
		false,
		fileCompleteSignal,
		true,
		true)

	assert.False(t, success)
	cwLogsClientMock.AssertExpectations(t)
}

func TestCloudWatchLogsService_IsLogGroupEncryptedWithKMSWithEncryptionDisabled(t *testing.T) {
	service := CloudWatchLogsService{
		context:              contextMock,
		cloudWatchLogsClient: cwLogsClientMock,
		stopPolicy:           sdkutil.NewStopPolicy("Test", 0),
	}

	logGroupName := "logGroup"
	testCwlLogGroup := cloudwatchlogs.LogGroup{
		LogGroupName: &logGroupName,
	}

	encrypted, _ := service.IsLogGroupEncryptedWithKMS(&testCwlLogGroup)
	assert.False(t, encrypted)
}

func TestCloudWatchLogsService_IsLogGroupEncryptedWithKMSWithEncryptionEnabled(t *testing.T) {
	service := CloudWatchLogsService{
		context:              contextMock,
		cloudWatchLogsClient: cwLogsClientMock,
		stopPolicy:           sdkutil.NewStopPolicy("Test", 0),
	}

	logGroupName := "logGroup"
	kmsKeyId := "kmsKeyId"
	testCwlLogGroup := cloudwatchlogs.LogGroup{
		LogGroupName: &logGroupName,
		KmsKeyId:     &kmsKeyId,
	}

	encrypted, _ := service.IsLogGroupEncryptedWithKMS(&testCwlLogGroup)
	assert.True(t, encrypted)
}
