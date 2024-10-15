//go:build integration && (freebsd || linux || netbsd || openbsd || darwin)
// +build integration
// +build freebsd linux netbsd openbsd darwin

package runcommand

import (
	"sync"
	"testing"
	"time"

	"github.com/aws/amazon-ssm-agent/agent/context"
	mds "github.com/aws/amazon-ssm-agent/agent/runcommand/mds"
	runcommandmock "github.com/aws/amazon-ssm-agent/agent/runcommand/mock"
	"github.com/aws/amazon-ssm-agent/agent/sdkutil"
	"github.com/aws/aws-sdk-go/service/ssmmds"
	"github.com/carlescere/scheduler"
	"github.com/stretchr/testify/assert"
)

func TestLoop_Multiple_Parallel(t *testing.T) {
	// Test loop multiple times in parallel with valid response
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

	for i := 0; i < multipleRetryCount; i++ {
		wg.Add(1) // adding additional wait
		go func() {
			defer wg.Done()
			proc.messagePollLoop()
		}()
	}

	success := verifyWaitGroup(wg, 4*time.Second)
	assert.True(t, success, "Message loop failed to return within the expected time")

	time.Sleep(4 * time.Second)
	assert.Equal(t, 1, called)
}

func TestLoop_Multiple_Parallel_Error(t *testing.T) {
	// Test loop multiple times in parallel with simple error
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
	messagePollJob, _ := scheduler.Every(7).Seconds().NotImmediately().Run(job)

	wg := &sync.WaitGroup{}
	proc := RunCommandService{
		name:                 mdsName,
		context:              contextMock,
		service:              mdsMock,
		messagePollWaitGroup: wg,
		messagePollJob:       messagePollJob,
		processorStopPolicy:  sdkutil.NewStopPolicy(mdsName, stopPolicyThreshold),
	}

	for i := 0; i < multipleRetryCount; i++ {
		wg.Add(1) // adding additional wait
		go func() {
			defer wg.Done()
			proc.messagePollLoop()
		}()
	}

	success := verifyWaitGroup(wg, 5*time.Second)
	assert.True(t, success, "Message loop failed to return within the expected time")

	time.Sleep(5 * time.Second)
	assert.Equal(t, 1, called)
}
