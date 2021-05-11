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
	"bufio"
	"encoding/json"
	"errors"
	"io"
	"os"
	"runtime/debug"
	"strings"
	"time"

	"github.com/aws/amazon-ssm-agent/agent/agentlogstocloudwatch/cloudwatchlogspublisher/cloudwatchlogsinterface"
	"github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/amazon-ssm-agent/agent/sdkutil"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/client"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/request"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/cloudwatchlogs"
	"github.com/pborman/ansi"
)

const (
	stopPolicyErrorThreshold = 10
	stopPolicyName           = "CloudWatchLogsService"
	maxRetries               = 5
	UploadFrequency          = 1 * time.Second
	NewLineCharacter         = '\n'
	maxNumberOfEventsPerCall = 4

	// Event size - https://docs.aws.amazon.com/AmazonCloudWatch/latest/logs/cloudwatch_limits_cwl.html
	MessageLengthThresholdInBytes = 200 * 1000
)

// CloudWatchLogsService encapsulates the client and stop policy as a wrapper to call the cloudwatchlogs API
type CloudWatchLogsService struct {
	context              context.T
	cloudWatchLogsClient cloudwatchlogsinterface.CloudWatchLogsClient
	stopPolicy           *sdkutil.StopPolicy
	isFileComplete       bool
	isUploadComplete     bool
	CloudWatchMessage    CloudWatchMessage
}

// CloudWatchMessage captures all the information that are published in an event for streaming logs
type CloudWatchMessage struct {
	_ struct{} `type:"structure"`

	EventVersion *string       `json:"eventVersion"`
	EventTime    *string       `json:"eventTime"`
	AwsRegion    *string       `json:"awsRegion"`
	Target       *Target       `json:"target"`
	UserIdentity *UserIdentity `json:"userIdentity"`
	RunAsUser    *string       `json:"runAsUser"`
	SessionId    *string       `json:"sessionId"`
	SessionData  []*string     `json:"sessionData"`
}

// UserIdentity represents iam arn of the requester
type UserIdentity struct {
	_ struct{} `type:"structure"`

	Arn *string `json:"arn"`
}

// Target represents id of the target
type Target struct {
	_ struct{} `type:"structure"`

	Id *string `json:"id"`
}

// createCloudWatchStopPolicy creates a new policy for cloudwatchlogs
func createCloudWatchStopPolicy() *sdkutil.StopPolicy {
	return sdkutil.NewStopPolicy(stopPolicyName, stopPolicyErrorThreshold)
}

// createCloudWatchClient creates a client to call CloudWatchLogs APIs
func createCloudWatchClient(context context.T) cloudwatchlogsinterface.CloudWatchLogsClient {
	config := sdkutil.AwsConfig(context)
	return createCloudWatchClientWithConfig(context, config)
}

// createCloudWatchClientWithCredentials creates a client to call CloudWatchLogs APIs using credentials from the id and secret passed
func createCloudWatchClientWithCredentials(context context.T, id, secret string) cloudwatchlogsinterface.CloudWatchLogsClient {
	config := sdkutil.AwsConfig(context).WithCredentials(credentials.NewStaticCredentials(id, secret, ""))
	return createCloudWatchClientWithConfig(context, config)
}

// createCloudWatchClientWithConfig creates a client to call CloudWatchLogs APIs using the passed aws config
func createCloudWatchClientWithConfig(context context.T, config *aws.Config) cloudwatchlogsinterface.CloudWatchLogsClient {
	//Adding the AWS SDK Retrier with Exponential Backoff
	config = request.WithRetryer(config, client.DefaultRetryer{
		NumMaxRetries: maxRetries,
	})

	if defaultEndpoint := context.Identity().GetDefaultEndpoint("logs"); defaultEndpoint != "" {
		config.Endpoint = &defaultEndpoint
	}

	appConfig := context.AppConfig()

	sess := session.New(config)
	sess.Handlers.Build.PushBack(request.MakeAddToUserAgentHandler(appConfig.Agent.Name, appConfig.Agent.Version))
	return cloudwatchlogs.New(sess)
}

// NewCloudWatchLogsService Creates a new instance of the CloudWatchLogsService
func NewCloudWatchLogsService(context context.T) *CloudWatchLogsService {
	cloudWatchLogsService := CloudWatchLogsService{
		context:              context,
		cloudWatchLogsClient: createCloudWatchClient(context),
		stopPolicy:           createCloudWatchStopPolicy(),
		isFileComplete:       false,
		isUploadComplete:     false,
		CloudWatchMessage:    CloudWatchMessage{},
	}
	return &cloudWatchLogsService
}

// NewCloudWatchLogsServiceWithCredentials Creates a new instance of the CloudWatchLogsService using credentials from the Id and Secret passed
func NewCloudWatchLogsServiceWithCredentials(context context.T, id, secret string) *CloudWatchLogsService {
	cloudWatchLogsService := CloudWatchLogsService{
		context:              context,
		cloudWatchLogsClient: createCloudWatchClientWithCredentials(context, id, secret),
		stopPolicy:           createCloudWatchStopPolicy(),
		isFileComplete:       false,
		isUploadComplete:     false,
	}
	return &cloudWatchLogsService
}

// SetCloudWatchMessage initializes CloudWatchMessage
func (service *CloudWatchLogsService) SetCloudWatchMessage(
	eventVersion string,
	awsRegion string,
	targetId string,
	runAsUser string,
	sessionId string,
	sessionOwner string) {

	service.CloudWatchMessage = CloudWatchMessage{
		EventVersion: aws.String(eventVersion),
		AwsRegion:    aws.String(awsRegion),
		Target:       &Target{Id: aws.String(targetId)},
		UserIdentity: &UserIdentity{Arn: aws.String(sessionOwner)},
		RunAsUser:    aws.String(runAsUser),
		SessionId:    aws.String(sessionId),
	}
}

// CreateNewServiceIfUnHealthy checks service healthy and create new service if original is unhealthy
func (service *CloudWatchLogsService) CreateNewServiceIfUnHealthy() {
	if service.stopPolicy == nil {
		service.stopPolicy = createCloudWatchStopPolicy()
	}

	if !service.stopPolicy.IsHealthy() {
		service.stopPolicy.ResetErrorCount()
		service.cloudWatchLogsClient = createCloudWatchClient(service.context)
		return
	}
}

// CreateLogGroup calls the CreateLogGroup API to create a log group
func (service *CloudWatchLogsService) CreateLogGroup(logGroup string) (err error) {
	log := service.context.Log()
	service.CreateNewServiceIfUnHealthy()

	//Creating the parameters for the API Call
	params := &cloudwatchlogs.CreateLogGroupInput{
		LogGroupName: aws.String(logGroup),
	}

	//Calling the API
	if _, err = service.cloudWatchLogsClient.CreateLogGroup(params); err != nil {
		// Handle the common AWS errors and update the stop policy accordingly
		sdkutil.HandleAwsError(log, err, service.stopPolicy)

		// Cast err to awserr.Error to get the Code
		errorCode := sdkutil.GetAwsErrorCode(err)

		switch errorCode {
		// Check for error code. Note that the AWS Retrier has already made retries for the 5xx Response Codes
		case resourceAlreadyExistsException:
			// 400 Error, occurs when the LogGroup already exists
			// Ignoring the error
			err = nil
		default:
			// Other 400 Errors, 500 Errors even after retries. Log the error
			log.Errorf("Error Calling CreateLogGroup:%v", err.Error())
		}
	}
	return
}

// CreateLogStream calls the CreateLogStream API to create log stream within the specified log group
func (service *CloudWatchLogsService) CreateLogStream(logGroup, logStream string) (err error) {
	log := service.context.Log()
	service.CreateNewServiceIfUnHealthy()

	//Creating the parameters for the API Call
	params := &cloudwatchlogs.CreateLogStreamInput{
		LogGroupName:  aws.String(logGroup),
		LogStreamName: aws.String(logStream),
	}

	//Calling the API
	if _, err = service.cloudWatchLogsClient.CreateLogStream(params); err != nil {
		// Handle the common AWS errors and update the stop policy accordingly
		sdkutil.HandleAwsError(log, err, service.stopPolicy)

		// Cast err to awserr.Error to get the Code
		errorCode := sdkutil.GetAwsErrorCode(err)

		switch errorCode {
		// Check for error code. Note that the AWS Retrier has already made retries for the 5xx Response Codes
		case resourceAlreadyExistsException:
			// 400 Error, occurs when the LogStream already exists
			// Ignoring the error
			err = nil
		default:
			// Other 400 Errors, 500 Errors even after retries. Log the error
			log.Errorf("Error Calling CreateLogStream:%v", err.Error())
		}
	}
	return

}

// DescribeLogGroups calls the DescribeLogGroups API to get the details of log groups of account
func (service *CloudWatchLogsService) DescribeLogGroups(logGroupPrefix, nextToken string) (response *cloudwatchlogs.DescribeLogGroupsOutput, err error) {
	log := service.context.Log()
	service.CreateNewServiceIfUnHealthy()

	// Creating the parameters for the API Call
	params := &cloudwatchlogs.DescribeLogGroupsInput{}

	if logGroupPrefix != "" {
		params.LogGroupNamePrefix = aws.String(logGroupPrefix)
	}
	if nextToken != "" {
		params.NextToken = aws.String(nextToken)
	}

	// Calling the API
	if response, err = service.cloudWatchLogsClient.DescribeLogGroups(params); err != nil {
		// Handle the common AWS errors and update the stop policy accordingly
		sdkutil.HandleAwsError(log, err, service.stopPolicy)

		// AWS Retrier has already made retries for the 5xx Response Codes. Logging and Returning the error
		log.Errorf("Error Calling DescribeLogGroups:%v", err.Error())

		return
	}

	// Pretty-print the response data.
	log.Debugf("DescribeLogGroups Response:%v", response)

	return

}

// DescribeLogStreams calls the DescribeLogStreams API to get the details of the log streams present
func (service *CloudWatchLogsService) DescribeLogStreams(logGroup, logStreamPrefix, nextToken string) (response *cloudwatchlogs.DescribeLogStreamsOutput, err error) {
	log := service.context.Log()
	service.CreateNewServiceIfUnHealthy()

	// Creating the parameters for the API Call
	params := &cloudwatchlogs.DescribeLogStreamsInput{
		LogGroupName: aws.String(logGroup),
	}

	if logStreamPrefix != "" {
		params.LogStreamNamePrefix = aws.String(logStreamPrefix)
	}
	if nextToken != "" {
		params.NextToken = aws.String(nextToken)
	}

	// Calling the API
	if response, err = service.cloudWatchLogsClient.DescribeLogStreams(params); err != nil {
		// Handle the common AWS errors and update the stop policy accordingly
		sdkutil.HandleAwsError(log, err, service.stopPolicy)

		// AWS Retrier has already made retries for the 5xx Response Codes. Logging and Returning the error
		log.Errorf("Error Calling DescribeLogStreams:%v", err.Error())

		return
	}

	// Pretty-print the response data.
	log.Debugf("DescribeLogStreams Response:%v", response)

	return

}

//getLogGroupDetails Calls the DescribeLogGroups API to get the details of the loggroup specified. Returns nil if not found
func (service *CloudWatchLogsService) getLogGroupDetails(logGroup string) (logGroupDetails *cloudwatchlogs.LogGroup, err error) {
	log := service.context.Log()
	// Keeping the nextToken as empty in the beginning. Might get filled from response for subsequent calls
	nextToken := ""
	// The API implements paginations. The bool if true means more results are present and thus need to call the API again.
	nextBatchPresent := true

	// Continue calling  the API until we find the group or next batch of groups is not present
	for nextBatchPresent {
		describeLogGroupsOutput, err := service.DescribeLogGroups(logGroup, nextToken)

		if err != nil {
			log.Errorf("Error in calling DescribeLogGroups:%v", err)
			return nil, err
		}

		// Iterate through the log streams and check for the input log stream
		for _, stream := range describeLogGroupsOutput.LogGroups {
			if logGroup == *stream.LogGroupName {
				// Log Group Matched
				logGroupDetails = stream
				break
			}
		}

		// Group not found. Check if nextToken is returned. If yes, need to call the API again to get the next set of log groups
		if describeLogGroupsOutput.NextToken == nil {
			// Stream not found and nextToken not present
			nextBatchPresent = false
		} else {
			// There is a NextToken present. Use it to call and continue calling the API
			nextToken = *describeLogGroupsOutput.NextToken
		}
	}

	return logGroupDetails, nil
}

// IsLogGroupPresent checks and returns true when the log group is present
func (service *CloudWatchLogsService) IsLogGroupPresent(logGroup string) (bool, *cloudwatchlogs.LogGroup) {
	logGroupDetails, _ := service.getLogGroupDetails(logGroup)
	return logGroupDetails != nil, logGroupDetails
}

// GetSequenceTokenForStream returns the current sequence token for the stream specified
func (service *CloudWatchLogsService) GetSequenceTokenForStream(logGroupName, logStreamName string) (sequenceToken *string) {
	logStream := service.getLogStreamDetails(logGroupName, logStreamName)
	if logStream != nil {
		sequenceToken = logStream.UploadSequenceToken
	}
	return
}

// getLogStreamDetails Calls the DescribeLogStreams API to get the details of the Log Stream specified. Returns nil if the stream is not found
func (service *CloudWatchLogsService) getLogStreamDetails(logGroupName, logStreamName string) (logStream *cloudwatchlogs.LogStream) {
	log := service.context.Log()
	// Keeping the nextToken as empty in the beginning. Might get filled from response for subsequent calls
	nextToken := ""
	// Takes note of whether need to call the API again
	nextBatchPresent := true

	// Continue calling  the API until we find the stream or next batch of streams is not present
	for nextBatchPresent {
		describeLogStreamsOutput, err := service.DescribeLogStreams(logGroupName, logStreamName, nextToken)

		if err != nil {
			log.Errorf("Error in calling DescribeLogStreams:%v", err)
			return
		}

		// Iterate through the log streams and check for the input log stream
		for _, stream := range describeLogStreamsOutput.LogStreams {
			if logStreamName == *stream.LogStreamName {
				// Log Stream Matched
				logStream = stream
				return
			}
		}

		// Stream not found. Check if nextToken is returned. If yes, need to call the API again to get the next set of log streams
		if describeLogStreamsOutput.NextToken == nil {
			// Stream not found and nextToken not present
			nextBatchPresent = false
		} else {
			// There is a NextToken present. Use it to call and continue calling the API
			nextToken = *describeLogStreamsOutput.NextToken
		}
	}

	return
}

// PutLogEvents calls the PutLogEvents API to push messages to CloudWatchLogs
func (service *CloudWatchLogsService) PutLogEvents(messages []*cloudwatchlogs.InputLogEvent, logGroup, logStream string, sequenceToken *string) (nextSequenceToken *string, err error) {
	log := service.context.Log()
	service.CreateNewServiceIfUnHealthy()

	// Creating the parameters for the API Call
	params := &cloudwatchlogs.PutLogEventsInput{
		LogEvents:     messages,
		LogGroupName:  aws.String(logGroup),
		LogStreamName: aws.String(logStream),
		SequenceToken: sequenceToken,
	}

	// Calling the API
	response, err := service.cloudWatchLogsClient.PutLogEvents(params)

	if err != nil {

		// Handle the common AWS errors and update the stop policy accordingly
		sdkutil.HandleAwsError(log, err, service.stopPolicy)
		// Cast err to awserr.Error to get the Code
		errorCode := sdkutil.GetAwsErrorCode(err)

		switch errorCode {
		// Check for error code. Note that the AWS Retrier has already made retries for the 5xx Response Codes
		case invalidSequenceTokenException:
			// 400 Error, occurs when the SequenceToken is invalid. Create new SequenceToken and use it again
			fallthrough
		case dataAlreadyAcceptedException:
			// 400 Error, occurs when the SequenceToken has been used. Create new SequenceToken and use it again
			// Adding Error Count to StopPolicy before retrying to ensure the retries stop after Stop Policy error counts exceed
			service.stopPolicy.AddErrorCount(1)
			return service.retryPutWithNewSequenceToken(messages, logGroup, logStream)
		default:
			// Other 400 Errors, 500 Errors even after retries. Log the error
			log.Errorf("Error in PutLogEvents:%v", err.Error())
		}

		return
	}

	nextSequenceToken = response.NextSequenceToken
	return
}

// retryPutWithNewSequenceToken gets a new sequence token and retries pushing messages to cloudwatchlogs
func (service *CloudWatchLogsService) retryPutWithNewSequenceToken(messages []*cloudwatchlogs.InputLogEvent, logGroupName, logStreamName string) (*string, error) {
	// Get the sequence token by calling the DescribeLogStreams API
	logStream := service.getLogStreamDetails(logGroupName, logStreamName)

	if logStream == nil {
		// Failed to get log stream and hence the sequence token. Log the error
		err := errors.New("Failed to get sequence token")
		return nil, err
	}

	sequenceToken := logStream.UploadSequenceToken

	// Successfully got the new sequence token. Retry the PutLogEvents API
	return service.PutLogEvents(messages, logGroupName, logStreamName, sequenceToken)
}

//IsLogGroupEncryptedWithKMS return true if the log group is encrypted with KMS key.
func (service *CloudWatchLogsService) IsLogGroupEncryptedWithKMS(logGroup *cloudwatchlogs.LogGroup) (bool, error) {
	if logGroup == nil {
		return false, nil
	}

	if logGroup.KmsKeyId != nil {
		return true, nil
	}

	service.context.Log().Debugf("CloudWatch log group %s is not encrypted with KMS", logGroup.LogGroupName)
	return false, nil
}

//StreamData streams data from the absoluteFilePath file to cloudwatch logs.
func (service *CloudWatchLogsService) StreamData(
	logGroupName string,
	logStreamName string,
	absoluteFilePath string,
	isFileComplete bool,
	isLogStreamCreated bool,
	fileCompleteSignal chan bool,
	cleanupControlCharacters bool,
	structuredLogs bool) (success bool) {
	log := service.context.Log()
	log.Infof("Uploading logs at %s to CloudWatch", absoluteFilePath)
	defer func() {
		if r := recover(); r != nil {
			log.Errorf("CloudWatch service stream data panic: %v", r)
			log.Errorf("Stacktrace:\n%s", debug.Stack())
		}
	}()

	service.isFileComplete = isFileComplete
	go func() {
		service.isFileComplete = <-fileCompleteSignal
		log.Debugf("Received file complete signal %v", service.isFileComplete)
	}()

	// Keeps track of the last known line number that was successfully uploaded to CloudWatch.
	var lastKnownLineUploadedToCWL int64 = 0
	// Keeps track of the next line number upto which the logs will be uploaded to CloudWatch.
	var currentLineNumber int64 = 0
	var sequenceToken *string
	var err error

	IsLogStreamCreated := isLogStreamCreated
	IsFirstTimeLogging := true

	// Initialize timer and set upload frequency.
	ticker := time.NewTicker(UploadFrequency)
	defer ticker.Stop()

	for range ticker.C {
		// Get next message to be uploaded.
		events, eof := service.getNextMessage(
			absoluteFilePath,
			&lastKnownLineUploadedToCWL,
			&currentLineNumber,
			cleanupControlCharacters,
			structuredLogs)

		// Exit case determining that the file is complete and has been scanned till EOF.
		if eof {
			log.Info("Finished uploading events to CloudWatch")
			service.isUploadComplete = true
			success = true
			break
		}

		// If no new messages found then skip uploading.
		if len(events) == 0 {
			log.Trace("No events to upload to CloudWatch")
			continue
		}

		if IsFirstTimeLogging {
			log.Infof("Started CloudWatch upload")
			IsFirstTimeLogging = false
		}
		log.Tracef("Uploading message line %d to CloudWatch", currentLineNumber)

		if !IsLogStreamCreated {
			log.Info("Log stream creation started")
			// Terminate process if the log stream cannot be created
			if err := service.CreateLogStream(logGroupName, logStreamName); err != nil {
				log.Errorf("Error Creating Log Stream for CloudWatchLogs output: %v", err)
				currentLineNumber = lastKnownLineUploadedToCWL
				log.Debug("Failed to upload message to CloudWatch")
				break
			} else {
				log.Info("Log stream already created")
				IsLogStreamCreated = true
			}
			log.Info("Log stream creation ended")
		}

		// Use sequenceToken returned by PutLogEvents if present, else fetch new one
		if sequenceToken == nil {
			log.Info("Calling Get Sequence token")
			sequenceToken = service.GetSequenceTokenForStream(logGroupName, logStreamName)
			log.Info("Received Sequence token")
		}

		sequenceToken, err = service.PutLogEvents(events, logGroupName, logStreamName, sequenceToken)
		if err == nil {
			// Set the last known line to current since the upload was successful.
			lastKnownLineUploadedToCWL = currentLineNumber
			log.Trace("Successfully uploaded message line %d to CloudWatch", currentLineNumber)
		} else {
			if errCode := sdkutil.GetAwsErrorCode(err); errCode == resourceNotFoundException {
				// Log group or log stream not found due to resource change outside of client. Stop log streaming for session
				log.Errorf(
					"Log group \"%s\" or log stream \"%s\" not found. Log stream stopped. Error:%v",
					logGroupName,
					logStreamName,
					err)
				break
			}
			// Upload failed for unknown reason. Reset the current line to last known line and retry upload again in the next iteration
			currentLineNumber = lastKnownLineUploadedToCWL
			log.Warnf("Failed to upload message to CloudWatch, err: %v", err)
		}
	}
	return success
}

//getNextMessage gets the next message to be uploaded to cloudwatch.
func (service *CloudWatchLogsService) getNextMessage(
	absoluteFilePath string,
	lastKnownLineUploadedToCWL *int64,
	currentLineNumber *int64,
	cleanupControlCharacters bool,
	structuredLogs bool) (allEvents []*cloudwatchlogs.InputLogEvent, eof bool) {
	log := service.context.Log()
	// Open file to read.
	file, err := os.Open(absoluteFilePath)
	if err != nil {
		log.Warnf("Error opening file: %v", err)
		return
	}
	defer file.Close()

	// Initialize reader.
	reader := bufio.NewReaderSize(file, MessageLengthThresholdInBytes)

	// Skip to the last uploaded line.
	if *lastKnownLineUploadedToCWL > 0 {
		var lastLine int64 = 0
		_, err := reader.ReadSlice(NewLineCharacter)
		for err == nil || err == bufio.ErrBufferFull {
			lastLine++
			if lastLine == *lastKnownLineUploadedToCWL {
				break
			}
			_, err = reader.ReadSlice(NewLineCharacter)
		}
		if err != nil && err != io.EOF {
			log.Warnf("Error skipping to last uploaded Cloudwatch line: %v", err)
			return
		}
	}

	var message, line []byte
	for {
		// Scan the next set of lines to upload.
		line, err = reader.ReadSlice(NewLineCharacter)
		if err != nil && err != bufio.ErrBufferFull {
			// Breaking out of loop since nothing to upload
			if err != io.EOF || len(line) == 0 || !service.isFileComplete {
				break
			}
		}

		// Process message if needed before uploading to CW
		line = processMessage(log, line, cleanupControlCharacters)

		// Check if message length threshold for the event has reached.
		// If true, then construct event with existing message so that new line will get added to the next event.
		// If false, then continue to append new line to existing message.
		if (len(message) + len(line)) > MessageLengthThresholdInBytes {
			log.Tracef("Appending line to current Cloudwatch event message"+
				" exceeds length limit %v bytes. [Line: %v]",
				MessageLengthThresholdInBytes, *currentLineNumber)

			event := service.buildEventInfo(message, structuredLogs)

			log.Trace("Created CloudWatch event from current event message buffer")
			allEvents = append(allEvents, event)
			if len(allEvents) >= maxNumberOfEventsPerCall {
				return
			}

			log.Trace("Reset Cloudwatch event message buffer")
			message = nil
		}
		message = append(message, line...)
		*currentLineNumber++
	}

	if err != io.EOF && err != nil {
		log.Warnf("Error reading from Cloudwatch logs file:", err)
	}

	// Build event with the message read so far to be uploaded to CW
	if len(message) > 0 {
		event := service.buildEventInfo(message, structuredLogs)
		allEvents = append(allEvents, event)
		return
	}

	// This determines the end of session.
	if len(message) == 0 && (err == nil || err == io.EOF) && service.isFileComplete {
		eof = true
	}

	return
}

// processMessage is used to process message before uploading to CW like cleaning up ANSI control characters
func processMessage(log log.T, line []byte, cleanupANSICharacters bool) (processedLine []byte) {
	defer func() {
		if err := recover(); err != nil {
			log.Tracef("processMessage encountered error: %v", err)
		}
	}()

	// Do nothing if cleanup of ANSI characters not required
	if !cleanupANSICharacters {
		return line
	}

	// Strip ANSI control sequences like color codes
	processedLine = line
	processedLine, err := ansi.Strip(line)
	if err != nil {
		processedLine = line
	}

	return processedLine
}

// buildEventInfo constructs event to be uploaded to CW
func (service *CloudWatchLogsService) buildEventInfo(message []byte, structuredLogs bool) *cloudwatchlogs.InputLogEvent {
	var formattedMessage string
	// Construct CloudWatch event in JSON format if structured logs required
	if structuredLogs {
		currentTime := time.Now().UTC()
		messageString := string(message)
		messageString = strings.ReplaceAll(messageString, "\t", " ")
		messageString = strings.ReplaceAll(messageString, "\r", "")
		messageList := strings.Split(messageString, "\n")
		if messageList[len(messageList)-1] == "" {
			messageList = messageList[:len(messageList)-1]
		}

		service.CloudWatchMessage.EventTime = aws.String(currentTime.Format(time.RFC3339))
		service.CloudWatchMessage.SessionData = aws.StringSlice(messageList)
		formattedMessageBytes, _ := json.Marshal(service.CloudWatchMessage)
		formattedMessage = string(formattedMessageBytes)
	} else {
		formattedMessage = strings.ReplaceAll(string(message), "\r\n", "\n")
		if service.isFileComplete && message[len(message)-1] == byte(NewLineCharacter) {
			formattedMessage = formattedMessage[:len(formattedMessage)-1]
		}
	}

	event := &cloudwatchlogs.InputLogEvent{
		Message:   aws.String(formattedMessage),
		Timestamp: aws.Int64(time.Now().UnixNano() / int64(time.Millisecond)),
	}
	return event
}

func (service *CloudWatchLogsService) SetIsFileComplete(val bool) {
	service.isFileComplete = val
}

func (service *CloudWatchLogsService) GetIsUploadComplete() bool {
	return service.isUploadComplete
}
