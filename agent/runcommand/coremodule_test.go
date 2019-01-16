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
	"time"

	runcommandmock "github.com/aws/amazon-ssm-agent/agent/runcommand/mock"
	"github.com/aws/aws-sdk-go/service/ssmmds"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// TestSendFailedReplies tests the sendFailedReplies function with multiple failed replies
func TestSendFailedReplies(t *testing.T) {
	contextMock := MockContext()

	// create mocked service and set expectations
	mdsMock := new(runcommandmock.MockedMDS)
	replies := GetTestFailedReplies()
	mdsMock.On("LoadFailedReplies", mock.AnythingOfType("*log.Mock")).Return(replies)
	mdsMock.On("GetFailedReply", mock.AnythingOfType("*log.Mock"), mock.AnythingOfType("string")).Return(&ssmmds.SendReplyInput{}, nil)
	mdsMock.On("SendReplyWithInput", mock.AnythingOfType("*log.Mock"), &ssmmds.SendReplyInput{}).Return(nil)
	mdsMock.On("DeleteFailedReply", mock.AnythingOfType("*log.Mock"), mock.AnythingOfType("string")).Return()

	proc := RunCommandService{
		name:    mdsName,
		context: contextMock,
		service: mdsMock,
	}

	proc.sendFailedReplies()

	time.Sleep(1 * time.Second)
	mdsMock.AssertNumberOfCalls(t, "SendReplyWithInput", 3)
	mdsMock.AssertNumberOfCalls(t, "DeleteFailedReply", 3)
}

// TestSendFailedRepliesWithZeroReplies tests the sendFailedReplies function with zero replies
func TestSendFailedRepliesWithZeroReplies(t *testing.T) {
	contextMock := MockContext()

	// create mocked service and set expectations
	mdsMock := new(runcommandmock.MockedMDS)
	replies := []string{}
	mdsMock.On("LoadFailedReplies", mock.AnythingOfType("*log.Mock")).Return(replies)

	proc := RunCommandService{
		name:    mdsName,
		context: contextMock,
		service: mdsMock,
	}

	proc.sendFailedReplies()

	time.Sleep(1 * time.Second)
	mdsMock.AssertNumberOfCalls(t, "SendReplyWithInput", 0)
	mdsMock.AssertNumberOfCalls(t, "DeleteFailedReply", 0)
}

// TestSendFailedRepliesWithSendReplyReturnError tests the sendFailedReplies function with errors from SendReplyWithInput function
func TestSendFailedRepliesWithSendReplyReturnError(t *testing.T) {
	contextMock := MockContext()

	// create mocked service and set expectations
	mdsMock := new(runcommandmock.MockedMDS)
	replies := GetTestFailedReplies()
	mdsMock.On("LoadFailedReplies", mock.AnythingOfType("*log.Mock")).Return(replies)
	mdsMock.On("SendReplyWithInput", mock.AnythingOfType("*log.Mock"), &ssmmds.SendReplyInput{}).Return(fmt.Errorf("some error"))
	mdsMock.On("GetFailedReply", mock.AnythingOfType("*log.Mock"), mock.AnythingOfType("string")).Return(&ssmmds.SendReplyInput{}, nil)

	proc := RunCommandService{
		name:    mdsName,
		context: contextMock,
		service: mdsMock,
	}

	proc.sendFailedReplies()

	time.Sleep(1 * time.Second)

	mdsMock.AssertNumberOfCalls(t, "SendReplyWithInput", 1)
	mdsMock.AssertNumberOfCalls(t, "DeleteFailedReply", 0)
}

func TestValidFailedReply(t *testing.T) {
	curT := time.Now().UTC()
	replyFileName := fmt.Sprintf("reply_%v", curT.Format("2006-01-02T15-04-05"))
	valid := isValidReplyRequest(replyFileName)
	assert.Equal(t, valid, true)

	replyFileName = "reply_2006-01-02T15-04-05"
	valid = isValidReplyRequest(replyFileName)
	assert.Equal(t, valid, false)

	replyFileName = "reply"
	valid = isValidReplyRequest(replyFileName)
	assert.Equal(t, valid, false)
}
