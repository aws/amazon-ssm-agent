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

//go:build integration
// +build integration

// Package runcommand implements runcommand core processing module
package runcommand

import (
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/aws/amazon-ssm-agent/agent/appconfig"
	"github.com/aws/amazon-ssm-agent/agent/context"
	contextmocks "github.com/aws/amazon-ssm-agent/agent/mocks/context"
	logmocks "github.com/aws/amazon-ssm-agent/agent/mocks/log"
	mds "github.com/aws/amazon-ssm-agent/agent/runcommand/mds"
	runcommandmock "github.com/aws/amazon-ssm-agent/agent/runcommand/mock"
	"github.com/aws/amazon-ssm-agent/agent/sdkutil"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/service/ssmmds"
	"github.com/carlescere/scheduler"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

const (
	sampleInstanceID    = ""
	multipleRetryCount  = 5
	stopPolicyThreshold = 3
)

var (
	errSample         = errors.New("some error")
	errAwsSample      = awserr.New("RequestError", "send request failed", errSample)
	stopPolicyTimeout = time.Second * 2
)

// verifyWaitGroup verifies that the waitgroup counter is zero or until it times out
func verifyWaitGroup(wg *sync.WaitGroup, timeout time.Duration) bool {
	waitCh := make(chan struct{})
	go func() {
		wg.Wait()
		close(waitCh)
	}()

	select {
	case <-waitCh:
		return true
	case <-time.After(timeout):
		return false
	}
}

// NewMockDefault returns an instance of Mock with default expectations set.
func MockContext() *contextmocks.Mock {
	ctx := new(contextmocks.Mock)
	log := logmocks.NewMockLog()
	config := appconfig.SsmagentConfig{}
	ctx.On("Log").Return(log)
	ctx.On("AppConfig").Return(config)
	ctx.On("With", mock.AnythingOfType("string")).Return(ctx)
	return ctx
}

func GetTestFailedReplies() []string {
	t := time.Now().UTC()
	replyFileName := fmt.Sprintf("reply_%v", t.Format("2006-01-02T15-04-05"))
	return []string{"1" + replyFileName, "2" + replyFileName, "3" + replyFileName}
}

func TestLoop_Once(t *testing.T) {
	// Test loop with valid response
	contextMock := MockContext()
	log := contextMock.Log()

	// create mocked service and set expectations
	mdsMock := new(runcommandmock.MockedMDS)
	mdsMock.On("GetMessages", log, sampleInstanceID).Return(&ssmmds.GetMessagesOutput{}, nil)
	newMdsService = func(context.T) mds.Service {
		return mdsMock
	}
	called := 0
	job := func() {
		called++
	}
	messagePollJob, _ := scheduler.Every(10).Seconds().NotImmediately().Run(job)

	wg := &sync.WaitGroup{}
	proc := RunCommandService{
		name:                 mdsName,
		context:              contextMock,
		service:              mdsMock,
		messagePollJob:       messagePollJob,
		messagePollWaitGroup: wg,
		processorStopPolicy:  sdkutil.NewStopPolicy(mdsName, stopPolicyThreshold),
	}

	proc.messagePollLoop()

	success := verifyWaitGroup(wg, 1*time.Second)
	assert.True(t, success, "Message loop failed to return within the expected time")

	time.Sleep(1 * time.Second)
	assert.Equal(t, 1, called)
}

func TestLoop_Multiple_Serial(t *testing.T) {
	// Test loop multiple times with valid response
	contextMock := MockContext()
	log := contextMock.Log()

	// create mocked service and set expectations
	mdsMock := new(runcommandmock.MockedMDS)
	mdsMock.On("GetMessages", log, sampleInstanceID).Return(&ssmmds.GetMessagesOutput{}, nil)
	newMdsService = func(context.T) mds.Service {
		return mdsMock
	}
	called := 0
	job := func() {
		called++
	}
	messagePollJob, _ := scheduler.Every(10).Seconds().NotImmediately().Run(job)

	wg := &sync.WaitGroup{}
	proc := RunCommandService{
		name:                 mdsName,
		context:              contextMock,
		service:              mdsMock,
		messagePollWaitGroup: wg,
		messagePollJob:       messagePollJob,
		processorStopPolicy:  sdkutil.NewStopPolicy(mdsName, stopPolicyThreshold),
	}

	start := time.Now()

	for i := 0; i < multipleRetryCount; i++ {
		proc.messagePollLoop()
	}

	// elapsed should be greater than number of polls in seconds as we force a 1 second delay
	elapsed := time.Since(start)

	success := verifyWaitGroup(wg, 1*time.Second)
	assert.True(t, success, "Message loop failed to return within the expected time")
	time.Sleep(1 * time.Second)

	assert.Equal(t, multipleRetryCount, called)
	assert.True(t, multipleRetryCount < elapsed.Seconds())
}

func TestLoop_Once_Error(t *testing.T) {
	// Test loop with simple error
	contextMock := MockContext()
	log := contextMock.Log()

	// create mocked service and set expectations
	mdsMock := new(runcommandmock.MockedMDS)
	mdsMock.On("GetMessages", log, sampleInstanceID).Return(&ssmmds.GetMessagesOutput{}, errSample)
	newMdsService = func(context.T) mds.Service {
		return mdsMock
	}
	called := 0
	job := func() {
		called++
	}
	messagePollJob, _ := scheduler.Every(10).Seconds().NotImmediately().Run(job)

	wg := &sync.WaitGroup{}
	proc := RunCommandService{
		name:                 mdsName,
		context:              contextMock,
		service:              mdsMock,
		messagePollWaitGroup: wg,
		messagePollJob:       messagePollJob,
		processorStopPolicy:  sdkutil.NewStopPolicy(mdsName, stopPolicyThreshold),
	}

	proc.messagePollLoop()

	success := verifyWaitGroup(wg, 1*time.Second)
	assert.True(t, success, "Message loop failed to return within the expected time")

	time.Sleep(1 * time.Second)
	assert.Equal(t, 1, called)
}

func TestLoop_Multiple_Serial_Error(t *testing.T) {
	// Test loop multiple times with simple error
	contextMock := MockContext()
	log := contextMock.Log()

	// create mocked service and set expectations
	mdsMock := new(runcommandmock.MockedMDS)
	mdsMock.On("GetMessages", log, sampleInstanceID).Return(&ssmmds.GetMessagesOutput{}, errSample)
	newMdsService = func(context.T) mds.Service {
		return mdsMock
	}
	called := 0
	job := func() {
		called++
	}
	messagePollJob, _ := scheduler.Every(10).Seconds().NotImmediately().Run(job)

	wg := &sync.WaitGroup{}
	proc := RunCommandService{
		name:                 mdsName,
		context:              contextMock,
		service:              mdsMock,
		messagePollWaitGroup: wg,
		messagePollJob:       messagePollJob,
		processorStopPolicy:  sdkutil.NewStopPolicy(mdsName, stopPolicyThreshold),
	}

	start := time.Now()

	for i := 0; i < multipleRetryCount; i++ {
		proc.messagePollLoop()
	}

	// elapsed should be greater than number of polls in seconds as we force a 1 second delay
	elapsed := time.Since(start)

	success := verifyWaitGroup(wg, 1*time.Second)
	assert.True(t, success, "Message loop failed to return within the expected time")
	time.Sleep(1 * time.Second)

	// number of tries should be the same as stop threshold +1
	assert.Equal(t, stopPolicyThreshold+1, called)
	assert.True(t, stopPolicyThreshold+1 < elapsed.Seconds())
}
func TestSendReplyLoop_Multiple_Serial_Error(t *testing.T) {
	// Test send reply loop multiple times with simple error
	contextMock := MockContext()
	replies := GetTestFailedReplies()

	// create mocked service and set expectations
	mdsMock := new(runcommandmock.MockedMDS)
	mdsMock.On("SendReplyWithInput", mock.AnythingOfType("*log.Mock"), &ssmmds.SendReplyInput{}).Return(errSample)
	mdsMock.On("LoadFailedReplies", mock.AnythingOfType("*log.Mock")).Return(replies)
	mdsMock.On("GetFailedReply", mock.AnythingOfType("*log.Mock"), mock.AnythingOfType("string")).Return(&ssmmds.SendReplyInput{}, nil)
	newMdsService = func(context.T) mds.Service {
		return mdsMock
	}
	proc := RunCommandService{
		name:                mdsName,
		context:             contextMock,
		service:             mdsMock,
		processorStopPolicy: sdkutil.NewStopPolicy(mdsName, stopPolicyThreshold),
	}

	for i := 0; i < multipleRetryCount; i++ {
		proc.sendReplyLoop()
	}

	time.Sleep(1 * time.Second)

	// number of tries should be the same as stop threshold
	mdsMock.AssertNumberOfCalls(t, "SendReplyWithInput", stopPolicyThreshold+1)
}
