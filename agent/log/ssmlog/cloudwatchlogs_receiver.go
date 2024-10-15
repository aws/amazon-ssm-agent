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

// Package ssmlog is used to initialize ssm functional logger
package ssmlog

import (
	"fmt"
	"time"

	"github.com/aws/amazon-ssm-agent/agent/agentlogstocloudwatch/cloudwatchlogsqueue"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/cloudwatchlogs"
	"github.com/cihub/seelog"
)

const (
	cloudWatchLogEventLength = int(262144 / 2) // max CW log event bytes - 262144
)

// CloudWatchCustomReceiver implements seelog.CustomReceiver
type CloudWatchCustomReceiver struct {
}

// ReceiveMessage Enqueues the new message to the queue
func (logReceiver *CloudWatchCustomReceiver) ReceiveMessage(message string, level seelog.LogLevel, context seelog.LogContextInterface) error {

	var result string
	count := 0
	maxIteration := 4
	for i := 0; i < len(message); i += cloudWatchLogEventLength {
		count++
		if count == maxIteration {
			return fmt.Errorf("exceeded max iterations for sending cloudwatch log event")
		}
		if i+cloudWatchLogEventLength > len(message) {
			result = message[i:]
		} else {
			result = message[i : i+cloudWatchLogEventLength]
		}
		// Creating cloudwatchlogs Log Event struct
		newMessage := &cloudwatchlogs.InputLogEvent{
			Message:   aws.String(result),
			Timestamp: aws.Int64(time.Now().UnixNano() / int64(time.Millisecond)),
		}

		// Adding the message to Queue
		err := cloudwatchlogsqueue.Enqueue(newMessage)
		if err != nil {
			return err
		}
	}
	return nil
}

// AfterParse extracts the log group and stream from the XML args and sets them in a new log data facade instance
func (logReceiver *CloudWatchCustomReceiver) AfterParse(initArgs seelog.CustomReceiverInitArgs) error {
	// Creating the facade instance at initialization
	if err := cloudwatchlogsqueue.CreateCloudWatchDataInstance(initArgs); err != nil {
		fmt.Printf("[ERROR] Failed to create cloudwatch log custom receiver. Err: %v\n", err)
	}

	return nil
}

// Flush flush the logs in the queue
func (logReceiver *CloudWatchCustomReceiver) Flush() {
	//TODO: Trigger the publisher to empty queue
}

// Close clears the queue being used.
func (logReceiver *CloudWatchCustomReceiver) Close() error {
	cloudwatchlogsqueue.DestroyCloudWatchDataInstance()
	return nil
}
