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

package runcommandmock

import (
	log "github.com/aws/amazon-ssm-agent/agent/log"
	mdsService "github.com/aws/amazon-ssm-agent/agent/runcommand/mds"
	"github.com/aws/aws-sdk-go/service/ssmmds"
	"github.com/stretchr/testify/mock"
)

// Note: This code is used in the test files. However, this code is not in a _test.go file
// because then we would have to copy it in every test package that needs the mock.

// MockedMDS stands for a mock MDS service.
type MockedMDS struct {
	mock.Mock
}

// GetMessages mocks the service function with the same name.
func (_m *MockedMDS) GetMessages(_a0 log.T, instanceID string) (*ssmmds.GetMessagesOutput, error) {
	ret := _m.Called(_a0, instanceID)

	var r0 *ssmmds.GetMessagesOutput
	if rf, ok := ret.Get(0).(func(log log.T, instanceID string) *ssmmds.GetMessagesOutput); ok {
		r0 = rf(_a0, instanceID)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*ssmmds.GetMessagesOutput)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(log log.T, instanceID string) error); ok {
		r1 = rf(_a0, instanceID)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// AcknowledgeMessage mocks the service function with the same name.
func (mdsMock *MockedMDS) AcknowledgeMessage(log log.T, messageID string) error {
	return mdsMock.Called(log, messageID).Error(0)
}

// SendReply mocks the service function with the same name.
func (mdsMock *MockedMDS) SendReply(_a0 log.T, messageID string, payload string) error {
	ret := mdsMock.Called(_a0, messageID, payload)

	var r0 error
	if rf, ok := ret.Get(0).(func(log log.T, messageID string, payload string) error); ok {
		r0 = rf(_a0, messageID, payload)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// FailMessage mocks the service function with the same name.
func (mdsMock *MockedMDS) FailMessage(log log.T, messageID string, failureType mdsService.FailureType) error {
	return mdsMock.Called(log, messageID, failureType).Error(0)
}

// DeleteMessage mocks the service function with the same name.
func (mdsMock *MockedMDS) DeleteMessage(log log.T, messageID string) error {
	return mdsMock.Called(log, messageID).Error(0)
}

// Stop mocks the service function with the same name.
func (mdsMock *MockedMDS) Stop() {
	mdsMock.Called()
}

func (mdsMock *MockedMDS) LoadFailedReplies(log log.T) []string {
	args := mdsMock.Called(log)
	return args.Get(0).([]string)
}

func (mdsMock *MockedMDS) DeleteFailedReply(log log.T, replyId string) {
	mdsMock.Called(log, replyId)
}

func (mdsMock *MockedMDS) PersistFailedReply(log log.T, sendReply ssmmds.SendReplyInput) error {
	return mdsMock.Called(log, sendReply).Error(0)
}

func (mdsMock *MockedMDS) GetFailedReply(log log.T, replyId string) (*ssmmds.SendReplyInput, error) {
	args := mdsMock.Called(log, replyId)
	return args.Get(0).(*ssmmds.SendReplyInput), args.Error(1)
}

func (mdsMock *MockedMDS) SendReplyWithInput(log log.T, sendReply *ssmmds.SendReplyInput) error {
	return mdsMock.Called(log, sendReply).Error(0)
}
