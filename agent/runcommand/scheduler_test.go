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

// Package runcommand implements runcommand core processing module
package runcommand

import (
	"fmt"
	"testing"

	"github.com/aws/amazon-ssm-agent/agent/context"
	"github.com/aws/amazon-ssm-agent/agent/contracts"
	runcommandmock "github.com/aws/amazon-ssm-agent/agent/runcommand/mock"
	"github.com/aws/aws-sdk-go/service/ssmmds"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// TestCasePollOnce contains fields to prepare pollOnce tests
type TestCasePollOnce struct {
	ContextMock *context.Mock

	MdsMock *runcommandmock.MockedMDS
}

func prepareTestPollOnce() (svc RunCommandService, testCase TestCasePollOnce) {

	// create mock context and log
	contextMock := context.NewMockDefault()

	// create mocked service and set expectations
	mdsMock := new(runcommandmock.MockedMDS)

	// create a agentConfig with dummy instanceID and agentInfo
	agentConfig := contracts.AgentConfiguration{
		AgentInfo: contracts.AgentInfo{
			Name:      "EC2Config",
			Version:   "1",
			Lang:      "en-US",
			Os:        "linux",
			OsVersion: "1",
		},
		InstanceID: testDestination,
	}

	svc = RunCommandService{
		context: contextMock,
		config:  agentConfig,
		service: mdsMock,
	}

	testCase = TestCasePollOnce{
		ContextMock: contextMock,
		MdsMock:     mdsMock,
	}

	return
}

// TestPollOnce tests the pollOnce function with one message
func TestPollOnce(t *testing.T) {
	// prepare test case fields
	proc, tc := prepareTestPollOnce()

	// mock GetMessagesOutput to return one message
	getMessageOutput := ssmmds.GetMessagesOutput{
		Destination:       &testDestination,
		Messages:          make([]*ssmmds.Message, 1),
		MessagesRequestId: &testMessageId,
	}

	// mock GetMessages function to return mocked GetMessagesOutput and no error
	tc.MdsMock.On("GetMessages", mock.AnythingOfType("*log.Mock"), mock.AnythingOfType("string")).Return(&getMessageOutput, nil)
	// set expectations
	countMessageProcessed := 0
	processMessage = func(svc *RunCommandService, msg *ssmmds.Message) {
		countMessageProcessed++
	}

	// execute pollOnce
	proc.pollOnce()

	// check expectations
	tc.MdsMock.AssertExpectations(t)
	assert.Equal(t, countMessageProcessed, 1)
}

// TestPollOnceWithZeroMessage tests the pollOnce function with zero message
func TestPollOnceWithZeroMessage(t *testing.T) {
	// prepare test case fields
	proc, tc := prepareTestPollOnce()

	// mock GetMessagesOutput to return zero message
	getMessageOutput := ssmmds.GetMessagesOutput{
		Destination:       &testDestination,
		Messages:          make([]*ssmmds.Message, 0),
		MessagesRequestId: &testMessageId,
	}

	// mock GetMessages function to return mocked GetMessagesOutput and no error
	tc.MdsMock.On("GetMessages", mock.AnythingOfType("*log.Mock"), mock.AnythingOfType("string")).Return(&getMessageOutput, nil)
	countMessageProcessed := 0
	processMessage = func(svc *RunCommandService, msg *ssmmds.Message) {
		countMessageProcessed++
	}

	// execute pollOnce
	proc.pollOnce()

	// check expectations
	tc.MdsMock.AssertExpectations(t)
	assert.Equal(t, countMessageProcessed, 0)
}

// TestPollOnceMultipleTimes tests the pollOnce function with five messages
func TestPollOnceMultipleTimes(t *testing.T) {
	// prepare test case fields
	proc, tc := prepareTestPollOnce()

	// mock GetMessagesOutput to return five message
	getMessageOutput := ssmmds.GetMessagesOutput{
		Destination:       &testDestination,
		Messages:          make([]*ssmmds.Message, 5),
		MessagesRequestId: &testMessageId,
	}

	// mock GetMessages function to return mocked GetMessagesOutput and no error
	tc.MdsMock.On("GetMessages", mock.AnythingOfType("*log.Mock"), mock.AnythingOfType("string")).Return(&getMessageOutput, nil)
	countMessageProcessed := 0
	processMessage = func(svc *RunCommandService, msg *ssmmds.Message) {
		countMessageProcessed++
	}

	// execute pollOnce
	proc.pollOnce()

	// check expectations
	tc.MdsMock.AssertExpectations(t)
	assert.Equal(t, countMessageProcessed, 5)
}

// TestPollOnceWithGetMessagesReturnError tests the pollOnce function with errors from GetMessages function
func TestPollOnceWithGetMessagesReturnError(t *testing.T) {
	// prepare test case fields
	proc, tc := prepareTestPollOnce()

	// mock GetMessagesOutput to return one message
	getMessageOutput := ssmmds.GetMessagesOutput{
		Destination:       &testDestination,
		Messages:          make([]*ssmmds.Message, 1),
		MessagesRequestId: &testMessageId,
	}

	// mock GetMessages function to return an error
	tc.MdsMock.On("GetMessages", mock.AnythingOfType("*log.Mock"), mock.AnythingOfType("string")).Return(&getMessageOutput, fmt.Errorf("Test"))
	isMessageProcessed := false
	processMessage = func(svc *RunCommandService, msg *ssmmds.Message) {
		isMessageProcessed = true
	}

	// execute pollOnce
	proc.pollOnce()

	// check expectations
	tc.MdsMock.AssertExpectations(t)
	assert.False(t, isMessageProcessed)
}
