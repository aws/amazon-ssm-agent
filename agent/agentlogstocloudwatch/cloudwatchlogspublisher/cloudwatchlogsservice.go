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
	"errors"
	"os"
	"time"

	"github.com/aws/amazon-ssm-agent/agent/agentlogstocloudwatch/cloudwatchlogspublisher/cloudwatchlogsinterface"
	"github.com/aws/amazon-ssm-agent/agent/appconfig"
	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/amazon-ssm-agent/agent/sdkutil"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/client"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/request"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/cloudwatchlogs"
)

const (
	stopPolicyErrorThreshold = 10
	stopPolicyName           = "CloudWatchLogsService"
	maxRetries               = 5
	UploadFrequency          = 3 * time.Second
	NewLineCharacter         = "\n"
	maxNumberOfEventsPerCall = 4

	// Event size - https://docs.aws.amazon.com/AmazonCloudWatch/latest/logs/cloudwatch_limits_cwl.html
	MessageLengthThresholdInBytes = 200 * 1000
)

// CloudWatchLogsService encapsulates the client and stop policy as a wrapper to call the cloudwatchlogs API
type CloudWatchLogsService struct {
	cloudWatchLogsClient cloudwatchlogsinterface.CloudWatchLogsClient
	stopPolicy           *sdkutil.StopPolicy
	IsFileComplete       bool
	IsUploadComplete     bool
}

// createCloudWatchStopPolicy creates a new policy for cloudwatchlogs
func createCloudWatchStopPolicy() *sdkutil.StopPolicy {
	return sdkutil.NewStopPolicy(stopPolicyName, stopPolicyErrorThreshold)
}

// createCloudWatchClient creates a client to call CloudWatchLogs APIs
func createCloudWatchClient() cloudwatchlogsinterface.CloudWatchLogsClient {
	config := sdkutil.AwsConfig()
	return createCloudWatchClientWithConfig(config)
}

// createCloudWatchClientWithCredentials creates a client to call CloudWatchLogs APIs using credentials from the id and secret passed
func createCloudWatchClientWithCredentials(id, secret string) cloudwatchlogsinterface.CloudWatchLogsClient {
	config := sdkutil.AwsConfig().WithCredentials(credentials.NewStaticCredentials(id, secret, ""))
	return createCloudWatchClientWithConfig(config)
}

// createCloudWatchClientWithConfig creates a client to call CloudWatchLogs APIs using the passed aws config
func createCloudWatchClientWithConfig(config *aws.Config) cloudwatchlogsinterface.CloudWatchLogsClient {
	//Adding the AWS SDK Retrier with Exponential Backoff
	config = request.WithRetryer(config, client.DefaultRetryer{
		NumMaxRetries: maxRetries,
	})

	appConfig, _ := appconfig.Config(false)
	sess := session.New(config)
	sess.Handlers.Build.PushBack(request.MakeAddToUserAgentHandler(appConfig.Agent.Name, appConfig.Agent.Version))
	return cloudwatchlogs.New(sess)
}

// NewCloudWatchLogsService Creates a new instance of the CloudWatchLogsService
func NewCloudWatchLogsService() *CloudWatchLogsService {
	cloudWatchLogsService := CloudWatchLogsService{
		cloudWatchLogsClient: createCloudWatchClient(),
		stopPolicy:           createCloudWatchStopPolicy(),
		IsFileComplete:       false,
		IsUploadComplete:     false,
	}
	return &cloudWatchLogsService
}

// NewCloudWatchLogsServiceWithCredentials Creates a new instance of the CloudWatchLogsService using credentials from the Id and Secret passed
func NewCloudWatchLogsServiceWithCredentials(id, secret string) *CloudWatchLogsService {
	cloudWatchLogsService := CloudWatchLogsService{
		cloudWatchLogsClient: createCloudWatchClientWithCredentials(id, secret),
		stopPolicy:           createCloudWatchStopPolicy(),
		IsFileComplete:       false,
		IsUploadComplete:     false,
	}
	return &cloudWatchLogsService
}

// CreateNewServiceIfUnHealthy checks service healthy and create new service if original is unhealthy
func (service *CloudWatchLogsService) CreateNewServiceIfUnHealthy() {
	if service.stopPolicy == nil {
		service.stopPolicy = createCloudWatchStopPolicy()
	}

	if !service.stopPolicy.IsHealthy() {
		service.stopPolicy.ResetErrorCount()
		service.cloudWatchLogsClient = createCloudWatchClient()
		return
	}
}

// CreateLogGroup calls the CreateLogGroup API to create a log group
func (service *CloudWatchLogsService) CreateLogGroup(log log.T, logGroup string) (err error) {

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
func (service *CloudWatchLogsService) CreateLogStream(log log.T, logGroup, logStream string) (err error) {

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
func (service *CloudWatchLogsService) DescribeLogGroups(log log.T, logGroupPrefix, nextToken string) (response *cloudwatchlogs.DescribeLogGroupsOutput, err error) {

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
func (service *CloudWatchLogsService) DescribeLogStreams(log log.T, logGroup, logStreamPrefix, nextToken string) (response *cloudwatchlogs.DescribeLogStreamsOutput, err error) {

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
func (service *CloudWatchLogsService) getLogGroupDetails(log log.T, logGroup string) (logGroupDetails *cloudwatchlogs.LogGroup) {

	// Keeping the nextToken as empty in the beginning. Might get filled from response for subsequent calls
	nextToken := ""
	// The API implements paginations. The bool if true means more results are present and thus need to call the API again.
	nextBatchPresent := true

	// Continue calling  the API until we find the group or next batch of groups is not present
	for nextBatchPresent {
		describeLogGroupsOutput, err := service.DescribeLogGroups(log, logGroup, nextToken)

		if err != nil {
			log.Errorf("Error in calling DescribeLogGroups:%v", err)
			return
		}

		// Iterate through the log streams and check for the input log stream
		for _, stream := range describeLogGroupsOutput.LogGroups {
			if logGroup == *stream.LogGroupName {
				// Log Group Matched
				logGroupDetails = stream
				return
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

	return
}

// IsLogGroupPresent checks and returns true when the log group is present
func (service *CloudWatchLogsService) IsLogGroupPresent(log log.T, logGroup string) bool {
	return service.getLogGroupDetails(log, logGroup) != nil
}

// IsLogStreamPresent checks and returns true when the log stream is present
func (service *CloudWatchLogsService) IsLogStreamPresent(log log.T, logGroupName, logStreamName string) bool {
	return service.getLogStreamDetails(log, logGroupName, logStreamName) != nil
}

// GetSequenceTokenForStream returns the current sequence token for the stream specified
func (service *CloudWatchLogsService) GetSequenceTokenForStream(log log.T, logGroupName, logStreamName string) (sequenceToken *string) {
	logStream := service.getLogStreamDetails(log, logGroupName, logStreamName)
	if logStream != nil {
		sequenceToken = logStream.UploadSequenceToken
	}
	return
}

// getLogStreamDetails Calls the DescribeLogStreams API to get the details of the Log Stream specified. Returns nil if the stream is not found
func (service *CloudWatchLogsService) getLogStreamDetails(log log.T, logGroupName, logStreamName string) (logStream *cloudwatchlogs.LogStream) {

	// Keeping the nextToken as empty in the beginning. Might get filled from response for subsequent calls
	nextToken := ""
	// Takes note of whether need to call the API again
	nextBatchPresent := true

	// Continue calling  the API until we find the stream or next batch of streams is not present
	for nextBatchPresent {
		describeLogStreamsOutput, err := service.DescribeLogStreams(log, logGroupName, logStreamName, nextToken)

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
func (service *CloudWatchLogsService) PutLogEvents(log log.T, messages []*cloudwatchlogs.InputLogEvent, logGroup, logStream string, sequenceToken *string) (nextSequenceToken *string, err error) {

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
			return service.retryPutWithNewSequenceToken(log, messages, logGroup, logStream)
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
func (service *CloudWatchLogsService) retryPutWithNewSequenceToken(log log.T, messages []*cloudwatchlogs.InputLogEvent, logGroupName, logStreamName string) (*string, error) {
	// Get the sequence token by calling the DescribeLogStreams API
	logStream := service.getLogStreamDetails(log, logGroupName, logStreamName)

	if logStream == nil {
		// Failed to get log stream and hence the sequence token. Log the error
		err := errors.New("Failed to get sequence token")
		return nil, err
	}

	sequenceToken := logStream.UploadSequenceToken

	// Successfully got the new sequence token. Retry the PutLogEvents API
	return service.PutLogEvents(log, messages, logGroupName, logStreamName, sequenceToken)
}

//IsLogGroupEncryptedWithKMS return true if the log group is encrypted with KMS key.
func (service *CloudWatchLogsService) IsLogGroupEncryptedWithKMS(log log.T, logGroupName string) bool {
	logGroup := service.getLogGroupDetails(log, logGroupName)
	if logGroup == nil {
		return false
	}

	if logGroup.KmsKeyId != nil {
		return true
	}

	log.Errorf("CloudWatch log group %s is not encrypted with KMS", logGroupName)
	return false
}

//StreamData streams data from the absoluteFilePath file to cloudwatch logs.
func (service *CloudWatchLogsService) StreamData(log log.T, logGroupName string, logStreamName string, absoluteFilePath string, isFileComplete bool, isLogStreamCreated bool) {
	log.Debugf("Uploading logs at %s to CloudWatch", absoluteFilePath)

	service.IsFileComplete = isFileComplete

	// Keeps track of the last known line number that was successfully uploaded to CloudWatch.
	var lastKnownLineUploadedToCWL int64 = 0

	// Keeps track of the next line number upto which the logs will be uploaded to CloudWatch.
	var currentLineNumber int64 = 0

	IsLogStreamCreated := isLogStreamCreated

	// Initialize timer and set upload frequency.
	ticker := time.NewTicker(UploadFrequency)

	for range ticker.C {
		// Get next message to be uploaded.
		events, eof := service.getNextMessage(log, absoluteFilePath, &lastKnownLineUploadedToCWL, &currentLineNumber)

		// Exit case determining that the file is complete and has been scanned till EOF.
		if eof {
			ticker.Stop()
			service.IsUploadComplete = true
			break
		}

		// If no new messages found then skip uploading.
		if len(events) == 0 {
			log.Debug("No events to upload to CloudWatch")
			continue
		}

		log.Debugf("Uploading message %v to CloudWatch", events)

		if !IsLogStreamCreated {

			// Terminate process if the log group is not present.
			if !service.IsLogGroupPresent(log, logGroupName) {
				log.Errorf("CloudWatch log group resource not created: %s", logGroupName)
				ticker.Stop()
				break
			}

			if err := service.CreateLogStream(log, logGroupName, logStreamName); err != nil {
				log.Errorf("Error Creating Log Stream for CloudWatchLogs output: %v", err)
				currentLineNumber = lastKnownLineUploadedToCWL
				log.Debug("Failed to upload message to CloudWatch")
				continue
			} else {
				IsLogStreamCreated = true
			}
		}

		sequenceToken := service.GetSequenceTokenForStream(log, logGroupName, logStreamName)

		_, err := service.PutLogEvents(log, events, logGroupName, logStreamName, sequenceToken)
		if err == nil {
			// Set the last known line to current since the upload was successful.
			lastKnownLineUploadedToCWL = currentLineNumber
			log.Debug("Successfully uploaded message to CloudWatch")
		} else {
			// Reset the current line to last known line since the upload failed and retry again in the next iteration.
			currentLineNumber = lastKnownLineUploadedToCWL
			log.Debug("Failed to upload message to CloudWatch")
		}
	}
}

//getNextMessage gets the next message to be uploaded to cloudwatch.
func (service *CloudWatchLogsService) getNextMessage(log log.T, absoluteFilePath string, lastKnownLineUploadedToCWL *int64, currentLineNumber *int64) (allEvents []*cloudwatchlogs.InputLogEvent, eof bool) {
	// Open file to read.
	file, err := os.Open(absoluteFilePath)
	if err != nil {
		log.Debugf("Error opening file: %v", err)
		return
	}
	defer file.Close()

	// Initialize scanner.
	scanner := bufio.NewScanner(file)
	scanner.Split(bufio.ScanLines)

	// Skip to the last uploaded line.
	if *lastKnownLineUploadedToCWL > 0 {
		var lastLine int64 = 0
		for scanner.Scan() {
			lastLine++
			if lastLine == *lastKnownLineUploadedToCWL {
				break
			}
		}
	}

	var message []byte
	// Scan the next set of lines to upload.
	for scanner.Scan() {
		if len(message) == 0 {
			message = append(message, scanner.Bytes()...)
		} else if (len(message) + len(scanner.Bytes())) > MessageLengthThresholdInBytes {
			event := &cloudwatchlogs.InputLogEvent{
				Message:   aws.String(string(message)),
				Timestamp: aws.Int64(time.Now().UnixNano() / int64(time.Millisecond)),
			}
			allEvents = append(allEvents, event)
			if len(allEvents) >= maxNumberOfEventsPerCall {
				return
			}

			message = nil
			message = append(message, scanner.Bytes()...)
		} else {
			message = append(append(message, []byte(NewLineCharacter)...), scanner.Bytes()...)
		}

		*currentLineNumber++
	}

	if len(message) > 0 {
		event := &cloudwatchlogs.InputLogEvent{
			Message:   aws.String(string(message)),
			Timestamp: aws.Int64(time.Now().UnixNano() / int64(time.Millisecond)),
		}

		allEvents = append(allEvents, event)
		return
	}

	// This determines the end of session.
	if len(message) == 0 && scanner.Err() == nil && service.IsFileComplete {
		eof = true
	}

	return
}
