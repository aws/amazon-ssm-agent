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

package cloudwatchlogsqueue

import (
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/aws/amazon-ssm-agent/agent/appconfig"
	"github.com/aws/aws-sdk-go/service/cloudwatchlogs"
	"github.com/cihub/seelog"
	"github.com/stretchr/testify/assert"
)

func TestFacade(t *testing.T) {

	xmlArgs := make(map[string]string)
	xmlArgs["log-group"] = "LogGroup"
	xmlArgs["log-stream"] = "LogStream"

	initArgs := seelog.CustomReceiverInitArgs{
		XmlCustomAttrs: xmlArgs,
	}

	once = new(sync.Once)
	CreateCloudWatchDataInstance(initArgs)

	messages, err := Dequeue(time.Millisecond)
	assert.NoError(t, err, "Unexpected Error in Dequeueing From Queue")
	assert.Len(t, messages, 0, "No Messages should be present")

	message := &cloudwatchlogs.InputLogEvent{}

	Enqueue(message)

	messages, err = Dequeue(time.Millisecond)

	assert.NoError(t, err, "Unexpected Error in Dequeueing From Queue")
	assert.Len(t, messages, 1, "Messages should be of length 1")

	messages, err = Dequeue(time.Millisecond)
	assert.NoError(t, err, "Unexpected Error in Dequeueing From Queue")
	assert.Len(t, messages, 0, "No Messages should be present")

	Enqueue(message)

	messages, err = Dequeue(time.Millisecond)
	assert.NoError(t, err, "Unexpected Error in Dequeueing From Queue")
	assert.NotNil(t, messages, "Messages should be present")

	s := strings.Repeat("A", batchByteSizeMax/2)
	message = &cloudwatchlogs.InputLogEvent{
		Message: &s,
	}
	Enqueue(message)
	Enqueue(message)
	messages, err = Dequeue(time.Millisecond)
	assert.Equal(t, "cw batch byte size exceeded the limit", err.Error())
	assert.Len(t, messages, 1, "No Messages should be present")

	DestroyCloudWatchDataInstance()

	messages, err = Dequeue(time.Millisecond)
	assert.Error(t, err, "No Error in Dequeueing From Destroyed Queue")
	assert.Len(t, messages, 0, "No Messages should be present")

}

func TestParallelAccessOfQueue(t *testing.T) {
	xmlArgs := make(map[string]string)
	xmlArgs["log-group"] = "LogGroup"
	xmlArgs["log-stream"] = "LogStream"

	initArgs := seelog.CustomReceiverInitArgs{
		XmlCustomAttrs: xmlArgs,
	}

	once = new(sync.Once)
	CreateCloudWatchDataInstance(initArgs)

	message := &cloudwatchlogs.InputLogEvent{}

	counter := 0

	dequeued := make(chan bool, 6)
	done := make(chan bool, 3)
	enqueuesComplete := false

	go func() {
		for i := 0; i < 500; i++ {
			Enqueue(message)
			counter++
			if i == 100 || i == 300 {
				// Block on dequeued which unblocks only when something dequeues to ensure dequeuer is running while enqueueing
				<-dequeued
			}
		}
		<-dequeued
		done <- true
	}()

	go func() {
		for i := 0; i < 1000; i++ {
			Enqueue(message)
			counter++
			if i == 100 || i == 500 {
				// Block on dequeued which unblocks only when something dequeues to ensure dequeuer is running while enqueueing
				<-dequeued
			}
		}
		<-dequeued
		done <- true
	}()

	go func() {
		for {
			messages, _ := Dequeue(time.Millisecond)
			counter -= len(messages)
			if len(messages) == 0 {
				// Unblock Enqueuers to continue enqueueing
				dequeued <- true
			}
			if enqueuesComplete {
				break
			}
			time.Sleep(time.Millisecond)
		}
		done <- true
	}()

	<-done
	<-done
	enqueuesComplete = true
	<-done

	assert.Equal(t, 0, counter, "Message loss while enqueueing and dequeueing from go routines")

}

func TestOverflow(t *testing.T) {
	xmlArgs := make(map[string]string)
	xmlArgs["log-group"] = "LogGroup"
	xmlArgs["log-stream"] = "LogStream"

	initArgs := seelog.CustomReceiverInitArgs{
		XmlCustomAttrs: xmlArgs,
	}

	once = new(sync.Once)
	CreateCloudWatchDataInstance(initArgs)

	message := &cloudwatchlogs.InputLogEvent{}

	for i := int64(0); i < (queueLimit + int64(100)); i++ {
		Enqueue(message)
	}

	assert.Equal(t, queueLimit, logDataFacadeInstance.messageQueue.Len(), "No. of messages in Queue do not match queuelimit on enqueueing more than limit")
}

func TestFacadeCWGroupNameTest(t *testing.T) {

	xmlArgs := make(map[string]string)
	xmlArgs["log-stream"] = "LogStream"

	initArgs := seelog.CustomReceiverInitArgs{
		XmlCustomAttrs: xmlArgs,
	}

	var args = []string{appconfig.DefaultDocumentWorker}
	initArgs.XmlCustomAttrs[agentWorkerLogGroupSeelogAttrib] = ""
	err := verifyLogGroupName(initArgs, args)
	assert.Error(t, err)

	verifiedLogGroupName = ""
	args = []string{appconfig.DefaultDocumentWorker}
	initArgs.XmlCustomAttrs[agentWorkerLogGroupSeelogAttrib] = "test"
	err = verifyLogGroupName(initArgs, args)
	assert.Error(t, err)
	assert.Equal(t, "", verifiedLogGroupName)

	verifiedLogGroupName = ""
	args = []string{appconfig.DefaultDocumentWorker}
	initArgs.XmlCustomAttrs[docWorkerLogGroupSeelogAttrib] = "test1"
	err = verifyLogGroupName(initArgs, args)
	assert.NoError(t, err)
	assert.Equal(t, "test1", verifiedLogGroupName)

	verifiedLogGroupName = ""
	args = []string{appconfig.DefaultSessionWorker}
	initArgs.XmlCustomAttrs[docWorkerLogGroupSeelogAttrib] = "test2"
	err = verifyLogGroupName(initArgs, args)
	assert.Error(t, err)
	assert.Equal(t, "", verifiedLogGroupName)

	verifiedLogGroupName = ""
	initArgs.XmlCustomAttrs[sessionWorkerLogGroupSeelogAttrib] = "test3"
	err = verifyLogGroupName(initArgs, args)
	assert.NoError(t, err)
	assert.Equal(t, "test3", verifiedLogGroupName)
}
