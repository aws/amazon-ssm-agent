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
	"time"

	"github.com/aws/amazon-ssm-agent/agent/cloudwatchlogspublisher/mock"
	"github.com/aws/amazon-ssm-agent/agent/cloudwatchlogsqueue"
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

	cwPublisher := CloudWatchPublisher{
		log: logMock,
		cloudWatchLogsService: serviceMock,
		QueuePollingInterval:  time.Millisecond * 100,
		QueuePollingWaitTime:  time.Millisecond,
	}

	xmlArgs := make(map[string]string)
	xmlArgs["log-group"] = "LogGroup"
	xmlArgs["log-stream"] = "LogStream"

	initArgs := seelog.CustomReceiverInitArgs{
		XmlCustomAttrs: xmlArgs,
	}

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
