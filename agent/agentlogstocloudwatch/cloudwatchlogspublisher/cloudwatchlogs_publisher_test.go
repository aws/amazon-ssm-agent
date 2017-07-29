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
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/aws/amazon-ssm-agent/agent/agentlogstocloudwatch/cloudwatchlogspublisher/mock"
	"github.com/aws/amazon-ssm-agent/agent/agentlogstocloudwatch/cloudwatchlogsqueue"
	"github.com/aws/aws-sdk-go/service/cloudwatchlogs"
	"github.com/cihub/seelog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

var serviceMock = cloudwatchlogspublisher_mock.NewServiceMockDefault()

func TestCWPublisherDequeueMessages(t *testing.T) {

	sequenceToken := "1234"

	serviceMock.On("GetSequenceTokenForStream", mock.AnythingOfType("*log.Mock"), mock.AnythingOfType("string"), mock.AnythingOfType("string")).Return(&sequenceToken)
	serviceMock.On("PutLogEvents",
		mock.AnythingOfType("*log.Mock"),
		mock.AnythingOfType("[]*cloudwatchlogs.InputLogEvent"),
		mock.AnythingOfType("string"),
		mock.AnythingOfType("string"),
		mock.AnythingOfType("*string"),
	).Return(&sequenceToken, nil)
	serviceMock.On("IsLogGroupPresent", mock.AnythingOfType("*log.Mock"), mock.AnythingOfType("string")).Return(true)
	serviceMock.On("IsLogStreamPresent", mock.AnythingOfType("*log.Mock"), mock.AnythingOfType("string"), mock.AnythingOfType("string")).Return(true)

	cwPublisher := CloudWatchPublisher{
		log: logMock,
		cloudWatchLogsService: serviceMock,
		QueuePollingInterval:  time.Millisecond * 100,
		QueuePollingWaitTime:  time.Millisecond,
		instanceID:            "InstanceID",
	}

	xmlArgs := make(map[string]string)
	xmlArgs["log-group"] = "LogGroup"

	initArgs := seelog.CustomReceiverInitArgs{
		XmlCustomAttrs: xmlArgs,
	}

	cloudwatchlogsqueue.DestroyCloudWatchDataInstance()
	cloudwatchlogsqueue.CreateCloudWatchDataInstance(initArgs)

	message := &cloudwatchlogs.InputLogEvent{}

	cloudwatchlogsqueue.Enqueue(message)

	// Start the publisher to test whether it dequeues messages from queue
	cwPublisher.Start()

	// Starting a 200 millisecond timer to let the publisher dequeue messages
	time.Sleep(time.Millisecond * 200)

	cwPublisher.Stop()

	// Testing whether messages are still in the queue
	messages, _ := cloudwatchlogsqueue.Dequeue(time.Millisecond)
	assert.Nil(t, messages, "Publisher failed to dequeue messages")
}

func TestCreateLogGroupError(t *testing.T) {
	serviceMock := cloudwatchlogspublisher_mock.NewServiceMockDefault()
	serviceMock.On("IsLogGroupPresent", mock.AnythingOfType("*log.Mock"), mock.AnythingOfType("string")).Return(false)
	serviceMock.On("CreateLogGroup", mock.AnythingOfType("*log.Mock"), mock.AnythingOfType("string")).Return(errors.New("Log Group Creation Service Error"))

	cwPublisher := CloudWatchPublisher{
		log: logMock,
		cloudWatchLogsService: serviceMock,
		instanceID:            "instanceID",
	}

	err := cwPublisher.createLogGroupAndStream("GroupDoesNotExist", "Stream")
	assert.Error(t, err, "Error Expected When Log Group Creation Errors")

}

func TestCreateLogStreamError(t *testing.T) {
	serviceMock := cloudwatchlogspublisher_mock.NewServiceMockDefault()
	serviceMock.On("IsLogGroupPresent", mock.AnythingOfType("*log.Mock"), mock.AnythingOfType("string")).Return(true)
	serviceMock.On("IsLogStreamPresent", mock.AnythingOfType("*log.Mock"), mock.AnythingOfType("string"), mock.AnythingOfType("string")).Return(false)
	serviceMock.On("CreateLogStream", mock.AnythingOfType("*log.Mock"), mock.AnythingOfType("string"), mock.AnythingOfType("string")).Return(errors.New("Log Stream Creation Service Error"))

	cwPublisher := CloudWatchPublisher{
		log: logMock,
		cloudWatchLogsService: serviceMock,
		instanceID:            "instanceID",
	}

	err := cwPublisher.createLogGroupAndStream("Group", "StreamDoesNotExist")
	assert.Error(t, err, "Error Expected When Log Stream Creation Errors")

}

func TestCloudWatchLogsEventsListener(t *testing.T) {
	serviceMock := cloudwatchlogspublisher_mock.NewServiceMockDefault()
	sequenceToken := "1234"

	serviceMock.On("GetSequenceTokenForStream", mock.AnythingOfType("*log.Mock"), mock.AnythingOfType("string"), mock.AnythingOfType("string")).Return(&sequenceToken)
	serviceMock.On("PutLogEvents",
		mock.AnythingOfType("*log.Mock"),
		mock.AnythingOfType("[]*cloudwatchlogs.InputLogEvent"),
		mock.AnythingOfType("string"),
		mock.AnythingOfType("string"),
		mock.AnythingOfType("*string"),
	).Return(&sequenceToken, nil)
	serviceMock.On("IsLogGroupPresent", mock.AnythingOfType("*log.Mock"), mock.AnythingOfType("string")).Return(true)
	serviceMock.On("IsLogStreamPresent", mock.AnythingOfType("*log.Mock"), mock.AnythingOfType("string"), mock.AnythingOfType("string")).Return(true)

	// Create a queue
	xmlArgs := make(map[string]string)
	xmlArgs["log-group"] = "LogGroup"
	initArgs := seelog.CustomReceiverInitArgs{
		XmlCustomAttrs: xmlArgs,
	}

	cloudwatchlogsqueue.DestroyCloudWatchDataInstance()

	// Create the publisher
	cwPublisher := CloudWatchPublisher{
		log: logMock,
		cloudWatchLogsService: serviceMock,
		QueuePollingInterval:  time.Millisecond * 100,
		QueuePollingWaitTime:  time.Millisecond,
		instanceID:            "InstanceID",
	}
	cwPublisher.Init(logMock)

	message := &cloudwatchlogs.InputLogEvent{}
	cloudwatchlogsqueue.CreateCloudWatchDataInstance(initArgs)
	cloudwatchlogsqueue.Enqueue(message)

	// Send Event on channel. This should start the publisher to dequeue messages
	cloudwatchlogsqueue.CloudWatchLogsEventsChannel <- cloudwatchlogsqueue.QueueActivated

	// Starting a 200 millisecond timer to let the publisher dequeue messages
	time.Sleep(time.Millisecond * 200)

	// Testing whether messages are still in the queue
	messages, _ := cloudwatchlogsqueue.Dequeue(time.Millisecond)
	assert.Nil(t, messages, "Failed to signal publisher to start")

	// Send Event on channel. This should stop the publisher
	cloudwatchlogsqueue.CloudWatchLogsEventsChannel <- cloudwatchlogsqueue.QueueDeactivated
	time.Sleep(time.Millisecond * 200)

	cloudwatchlogsqueue.Enqueue(message)
	// Starting a 200 millisecond timer to let the publisher dequeue messages
	time.Sleep(time.Millisecond * 200)
	messages, _ = cloudwatchlogsqueue.Dequeue(time.Millisecond)
	fmt.Println(messages)
	assert.NotNil(t, messages, "Failed to signal Publisher to Stop")
	cwPublisher.Stop()

}

func TestGetSharingConfigurations(t *testing.T) {

	xmlArgs := make(map[string]string)
	xmlArgs["log-group"] = "LogGroup"
	xmlArgs["log-sharing-enabled"] = "true"
	xmlArgs["sharing-destination"] = "KeyID::Key::Group::Stream"
	initArgs := seelog.CustomReceiverInitArgs{
		XmlCustomAttrs: xmlArgs,
	}
	cloudwatchlogsqueue.DestroyCloudWatchDataInstance()
	cloudwatchlogsqueue.CreateCloudWatchDataInstance(initArgs)
	sharingConfigs := getSharingConfigurations()
	assert.NotNil(t, sharingConfigs, "Parsing Valid Configurations should not result in nil")

	assert.Equal(t, "KeyID", sharingConfigs.accessKeyId, "Access Key Id incorrect")
	assert.Equal(t, "Key", sharingConfigs.secretAccessKey, "Secret Access Key incorrect")
	assert.Equal(t, "Group", sharingConfigs.logGroup, "Log Group incorrect")
	assert.Equal(t, "Stream", sharingConfigs.logStream, "Log Stream incorrect")

}

func TestGetSharingConfigurationsIncorrect(t *testing.T) {
	xmlArgs := make(map[string]string)
	xmlArgs["log-group"] = "LogGroup"
	xmlArgs["log-sharing-enabled"] = "true"
	xmlArgs["sharing-destination"] = "KeyID:Key::Group::Stream"
	initArgs := seelog.CustomReceiverInitArgs{
		XmlCustomAttrs: xmlArgs,
	}
	cloudwatchlogsqueue.DestroyCloudWatchDataInstance()
	cloudwatchlogsqueue.CreateCloudWatchDataInstance(initArgs)
	sharingConfigs := getSharingConfigurations()
	assert.Nil(t, sharingConfigs, "Configurations should be nil as incorrectly formatted")
}

func TestStopSharingOnAccessError(t *testing.T) {
	serviceMock := cloudwatchlogspublisher_mock.NewServiceMockDefault()
	sequenceToken := "1234"
	sharingSequenceToken := "12345"
	serviceMock.On("GetSequenceTokenForStream", mock.AnythingOfType("*log.Mock"), mock.AnythingOfType("string"), mock.AnythingOfType("string")).Return(&sequenceToken)
	serviceMock.On("PutLogEvents",
		mock.AnythingOfType("*log.Mock"),
		mock.AnythingOfType("[]*cloudwatchlogs.InputLogEvent"),
		mock.AnythingOfType("string"),
		mock.AnythingOfType("string"),
		mock.AnythingOfType("*string"),
	).Return(&sequenceToken, nil)

	sharingServiceMock := cloudwatchlogspublisher_mock.NewServiceMockDefault()

	sharingServiceMock.On("GetSequenceTokenForStream", mock.AnythingOfType("*log.Mock"), mock.AnythingOfType("string"), mock.AnythingOfType("string")).Return(nil)
	sharingServiceMock.On("PutLogEvents",
		mock.AnythingOfType("*log.Mock"),
		mock.AnythingOfType("[]*cloudwatchlogs.InputLogEvent"),
		mock.AnythingOfType("string"),
		mock.AnythingOfType("string"),
		mock.AnythingOfType("*string"),
	).Return(nil, errors.New("Access Error"))

	cloudwatchlogsqueue.DestroyCloudWatchDataInstance()
	xmlArgs := make(map[string]string)
	xmlArgs["log-group"] = "LogGroup"

	initArgs := seelog.CustomReceiverInitArgs{
		XmlCustomAttrs: xmlArgs,
	}

	cloudwatchlogsqueue.CreateCloudWatchDataInstance(initArgs)

	// Create the publisher
	cwPublisher := CloudWatchPublisher{
		log: logMock,
		cloudWatchLogsService:        serviceMock,
		cloudWatchLogsServiceSharing: sharingServiceMock,
		QueuePollingInterval:         time.Millisecond * 100,
		QueuePollingWaitTime:         time.Millisecond,
		isSharingEnabled:             true,
		selfDestination: &destinationConfigurations{
			logGroup:  "Group",
			logStream: "Stream",
		},
		sharingDestination: &destinationConfigurations{
			logGroup:  "GroupSharing",
			logStream: "StreamSharing",
		},
		instanceID: "instanceID",
	}
	cwPublisher.startPolling(&sequenceToken, &sharingSequenceToken)

	message := &cloudwatchlogs.InputLogEvent{}

	cloudwatchlogsqueue.Enqueue(message)

	// Starting a 200 millisecond timer to let the publisher dequeue messages
	time.Sleep(time.Millisecond * 200)
	cwPublisher.Stop()

	fmt.Println(cwPublisher)

	assert.False(t, cwPublisher.isSharingEnabled, "Sharing should be disabled on Access Error")

}
