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

// cloudwatchlogsqueue queues up agent's context event log, to be consumed by the CloudWatchLogs publisher
package ssmlog

import (
	"strings"
	"testing"
	"time"

	"github.com/aws/amazon-ssm-agent/agent/agentlogstocloudwatch/cloudwatchlogsqueue"
	"github.com/cihub/seelog"
	"github.com/stretchr/testify/assert"
)

func TestCloudWatchLogsReceiver(t *testing.T) {
	xmlCustomAttrs := make(map[string]string)
	xmlCustomAttrs["log-group"] = "LogGroup"

	initArgs := seelog.CustomReceiverInitArgs{
		XmlCustomAttrs: xmlCustomAttrs,
	}

	cwLogReceiver := CloudWatchCustomReceiver{}

	cwLogReceiver.AfterParse(initArgs)

	messages, _ := cloudwatchlogsqueue.Dequeue(time.Millisecond)

	assert.Len(t, messages, 0, "No Messages should be present")

	assert.Equal(t, "LogGroup", cloudwatchlogsqueue.GetLogGroup(), "LogGroup Name Incorrect")

	err := cwLogReceiver.ReceiveMessage("Message", seelog.DebugLvl, nil)

	messages, _ = cloudwatchlogsqueue.Dequeue(time.Millisecond)

	assert.NotNil(t, messages, "Messages should be present")

	assert.Len(t, messages, 1, "Messages should be of length 1")

	assert.Equal(t, "Message", *messages[0].Message, "Message Incorrect")

	err = cwLogReceiver.ReceiveMessage(strings.Repeat("A", cloudWatchLogEventLength*3), seelog.DebugLvl, nil)
	messages, _ = cloudwatchlogsqueue.Dequeue(time.Millisecond)
	assert.NotNil(t, messages, "No Messages should be present")
	assert.Nil(t, err)
	assert.Len(t, messages, 3, "Messages should be of length 3")

	err = cwLogReceiver.ReceiveMessage(strings.Repeat("A", cloudWatchLogEventLength*4), seelog.DebugLvl, nil)
	messages, _ = cloudwatchlogsqueue.Dequeue(time.Millisecond)
	assert.Equal(t, "exceeded max iterations for sending cloudwatch log event", err.Error())
	assert.NotNil(t, messages, "No Messages should be present")
	assert.Len(t, messages, 3, "Messages should be of length 3")

	cwLogReceiver.Close()

	messages, _ = cloudwatchlogsqueue.Dequeue(time.Millisecond)
	assert.Len(t, messages, 0, "No Messages should be present")
}
