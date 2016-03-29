// Copyright 2016 Amazon.com, Inc. or its affiliates. All Rights Reserved.
//
// Licensed under the Amazon Software License (the "License"). You may not
// use this file except in compliance with the License. A copy of the
// License is located at
//
// http://aws.amazon.com/asl/
//
// or in the "license" file accompanying this file. This file is distributed
// on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND,
// express or implied. See the License for the specific language governing
// permissions and limitations under the License.

package processor

import (
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/aws/amazon-ssm-agent/agent/appconfig"
	"github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/aws/amazon-ssm-agent/agent/log"
	"github.com/aws/amazon-ssm-agent/agent/message/service"
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

// NewMockDefault returns an instance of Mock with default expectations set.
func MockContext() *context.Mock {
	ctx := new(context.Mock)
	log := log.NewMockLog()
	config := appconfig.T{}
	ctx.On("Log").Return(log)
	ctx.On("AppConfig").Return(config)
	ctx.On("With", mock.AnythingOfType("string")).Return(ctx)
	return ctx
}

func TestLoop_Once(t *testing.T) {
	// Test loop with valid response
	contextMock := MockContext()
	log := contextMock.Log()

	// create mocked service and set expectations
	mdsMock := new(MockedMDS)
	mdsMock.On("GetMessages", log, sampleInstanceID).Return(&ssmmds.GetMessagesOutput{}, nil)
	newMdsService = func(appconfig.T) service.Service {
		return mdsMock
	}
	called := 0
	job := func() {
		called++
	}
	messagePollJob, _ := scheduler.Every(10).Seconds().NotImmediately().Run(job)

	proc := Processor{
		context:             contextMock,
		service:             mdsMock,
		messagePollJob:      messagePollJob,
		processorStopPolicy: sdkutil.NewStopPolicy(name, stopPolicyThreshold),
	}

	proc.loop()

	time.Sleep(1 * time.Second)
	assert.Equal(t, 1, called)
}

func TestLoop_Multiple_Serial(t *testing.T) {
	// Test loop multiple times with valid response
	contextMock := MockContext()
	log := contextMock.Log()

	// create mocked service and set expectations
	mdsMock := new(MockedMDS)
	mdsMock.On("GetMessages", log, sampleInstanceID).Return(&ssmmds.GetMessagesOutput{}, nil)
	newMdsService = func(appconfig.T) service.Service {
		return mdsMock
	}
	called := 0
	job := func() {
		called++
	}
	messagePollJob, _ := scheduler.Every(10).Seconds().NotImmediately().Run(job)

	proc := Processor{
		context:             contextMock,
		service:             mdsMock,
		messagePollJob:      messagePollJob,
		processorStopPolicy: sdkutil.NewStopPolicy(name, stopPolicyThreshold),
	}

	start := time.Now()

	for i := 0; i < multipleRetryCount; i++ {
		proc.loop()
	}

	// elapsed should be greater than number of polls in seconds as we force a 1 second delay
	elapsed := time.Since(start)

	time.Sleep(1 * time.Second)

	assert.Equal(t, multipleRetryCount, called)
	assert.True(t, multipleRetryCount < elapsed.Seconds())
}

func TestLoop_Multiple_Parallel(t *testing.T) {
	// Test loop multiple times in parallel with valid response
	contextMock := MockContext()
	log := contextMock.Log()

	// create mocked service and set expectations
	mdsMock := new(MockedMDS)
	mdsMock.On("GetMessages", log, sampleInstanceID).Return(&ssmmds.GetMessagesOutput{}, nil)
	newMdsService = func(appconfig.T) service.Service {
		return mdsMock
	}
	called := 0
	job := func() {
		called++
	}
	messagePollJob, _ := scheduler.Every(10).Seconds().NotImmediately().Run(job)

	proc := Processor{
		context:             contextMock,
		service:             mdsMock,
		messagePollJob:      messagePollJob,
		processorStopPolicy: sdkutil.NewStopPolicy(name, stopPolicyThreshold),
	}

	for i := 0; i < multipleRetryCount; i++ {
		go proc.loop()
	}

	time.Sleep(4 * time.Second)
	assert.Equal(t, 1, called)
}

func TestLoop_Once_Error(t *testing.T) {
	// Test loop with simple error
	contextMock := MockContext()
	log := contextMock.Log()

	// create mocked service and set expectations
	mdsMock := new(MockedMDS)
	mdsMock.On("GetMessages", log, sampleInstanceID).Return(&ssmmds.GetMessagesOutput{}, errSample)
	newMdsService = func(appconfig.T) service.Service {
		return mdsMock
	}
	called := 0
	job := func() {
		called++
	}
	messagePollJob, _ := scheduler.Every(10).Seconds().NotImmediately().Run(job)

	proc := Processor{
		context:             contextMock,
		service:             mdsMock,
		messagePollJob:      messagePollJob,
		processorStopPolicy: sdkutil.NewStopPolicy(name, stopPolicyThreshold),
	}

	proc.loop()

	time.Sleep(1 * time.Second)
	assert.Equal(t, 1, called)
}

func TestLoop_Multiple_Serial_Error(t *testing.T) {
	// Test loop multiple times with simple error
	contextMock := MockContext()
	log := contextMock.Log()

	// create mocked service and set expectations
	mdsMock := new(MockedMDS)
	mdsMock.On("GetMessages", log, sampleInstanceID).Return(&ssmmds.GetMessagesOutput{}, errSample)
	newMdsService = func(appconfig.T) service.Service {
		return mdsMock
	}
	called := 0
	job := func() {
		called++
	}
	messagePollJob, _ := scheduler.Every(10).Seconds().NotImmediately().Run(job)

	proc := Processor{
		context:             contextMock,
		service:             mdsMock,
		messagePollJob:      messagePollJob,
		processorStopPolicy: sdkutil.NewStopPolicy(name, stopPolicyThreshold),
	}

	start := time.Now()

	for i := 0; i < multipleRetryCount; i++ {
		proc.loop()
	}

	// elapsed should be greater than number of polls in seconds as we force a 1 second delay
	elapsed := time.Since(start)

	time.Sleep(1 * time.Second)

	// number of tries should be the same as stop threshold +1
	assert.Equal(t, stopPolicyThreshold+1, called)
	assert.True(t, stopPolicyThreshold+1 < elapsed.Seconds())
}

func TestLoop_Multiple_Parallel_Error(t *testing.T) {
	// Test loop multiple times in parallel with simple error
	contextMock := MockContext()
	log := contextMock.Log()

	// create mocked service and set expectations
	mdsMock := new(MockedMDS)
	mdsMock.On("GetMessages", log, sampleInstanceID).Return(&ssmmds.GetMessagesOutput{}, errSample)
	newMdsService = func(appconfig.T) service.Service {
		return mdsMock
	}
	called := 0
	job := func() {
		called++
	}
	messagePollJob, _ := scheduler.Every(10).Seconds().NotImmediately().Run(job)

	proc := Processor{
		context:             contextMock,
		service:             mdsMock,
		messagePollJob:      messagePollJob,
		processorStopPolicy: sdkutil.NewStopPolicy(name, stopPolicyThreshold),
	}

	for i := 0; i < multipleRetryCount; i++ {
		go proc.loop()
	}

	time.Sleep(4 * time.Second)
	assert.Equal(t, 1, called)
}

func TestLoop_Once_AwsError(t *testing.T) {
	// Test loop with aws error
	contextMock := MockContext()
	log := contextMock.Log()

	// create mocked service and set expectations
	mdsMock := new(MockedMDS)
	mdsMock.On("GetMessages", log, sampleInstanceID).Return(&ssmmds.GetMessagesOutput{}, errAwsSample)
	newMdsService = func(appconfig.T) service.Service {
		return mdsMock
	}
	called := 0
	job := func() {
		called++
	}
	messagePollJob, _ := scheduler.Every(10).Seconds().NotImmediately().Run(job)

	proc := Processor{
		context:             contextMock,
		service:             mdsMock,
		messagePollJob:      messagePollJob,
		processorStopPolicy: sdkutil.NewStopPolicy(name, stopPolicyThreshold),
	}

	proc.loop()

	time.Sleep(1 * time.Second)
	assert.Equal(t, 1, called)
}

func TestLoop_Multiple_Serial_AwsError(t *testing.T) {
	// Test loop multiple times with aws error
	contextMock := MockContext()
	log := contextMock.Log()

	// create mocked service and set expectations
	mdsMock := new(MockedMDS)
	mdsMock.On("GetMessages", log, sampleInstanceID).Return(&ssmmds.GetMessagesOutput{}, errAwsSample)
	newMdsService = func(appconfig.T) service.Service {
		return mdsMock
	}
	called := 0
	job := func() {
		called++
	}
	messagePollJob, _ := scheduler.Every(10).Seconds().NotImmediately().Run(job)

	proc := Processor{
		context:             contextMock,
		service:             mdsMock,
		messagePollJob:      messagePollJob,
		processorStopPolicy: sdkutil.NewStopPolicy(name, stopPolicyThreshold),
	}

	start := time.Now()

	for i := 0; i < multipleRetryCount; i++ {
		proc.loop()
	}

	// elapsed should be greater than number of polls in seconds as we force a 1 second delay
	elapsed := time.Since(start)

	time.Sleep(1 * time.Second)

	// number of tries should be the same as stop threshold +1
	assert.Equal(t, stopPolicyThreshold+1, called)
	assert.True(t, stopPolicyThreshold+1 < elapsed.Seconds())
}

func TestLoop_Multiple_Parallel_AwsError(t *testing.T) {
	// Test loop multiple times in parallel with aws error
	contextMock := MockContext()
	contextMock.On("AppConfig").Return(appconfig.T{})
	log := contextMock.Log()

	// create mocked service and set expectations
	mdsMock := new(MockedMDS)
	mdsMock.On("GetMessages", log, sampleInstanceID).Return(&ssmmds.GetMessagesOutput{}, errAwsSample)
	newMdsService = func(appconfig.T) service.Service {
		return mdsMock
	}
	called := 0
	m := &sync.Mutex{}
	job := func() {
		m.Lock()
		called++
		m.Unlock()
	}
	waitTimeSec := 10
	messagePollJob, _ := scheduler.Every(waitTimeSec).Seconds().NotImmediately().Run(job)

	proc := Processor{
		context:             contextMock,
		service:             mdsMock,
		messagePollJob:      messagePollJob,
		processorStopPolicy: sdkutil.NewStopPolicy(name, stopPolicyThreshold),
	}

	for i := 0; i < multipleRetryCount; i++ {
		go proc.loop()
	}

	// add one second time difference to reduce chance of loop() not getting called
	time.Sleep(time.Duration(waitTimeSec+1) * time.Second)
	m.Lock()
	assert.Equal(t, 1, called)
	m.Unlock()
}
