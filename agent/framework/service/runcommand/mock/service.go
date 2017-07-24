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
	mdsService "github.com/aws/amazon-ssm-agent/agent/framework/service/runcommand/mds"
	"github.com/aws/amazon-ssm-agent/agent/log"
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
func (mdsMock *MockedMDS) GetMessages(log log.T, instanceID string) (messages *ssmmds.GetMessagesOutput, err error) {
	args := mdsMock.Called(log, instanceID)
	return args.Get(0).(*ssmmds.GetMessagesOutput), args.Error(1)
}

// AcknowledgeMessage mocks the service function with the same name.
func (mdsMock *MockedMDS) AcknowledgeMessage(log log.T, messageID string) error {
	return mdsMock.Called(log, messageID).Error(0)
}

// SendReply mocks the service function with the same name.
func (mdsMock *MockedMDS) SendReply(log log.T, messageID string, payload string) error {
	return mdsMock.Called(log, messageID, payload).Error(0)
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
